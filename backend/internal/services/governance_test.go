package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestParseEnvMappingIgnoresMalformedEntriesAndTrimsSlashes(t *testing.T) {
	mapping := parseEnvMapping(" foo = https://a.example.com/ , invalid , bar=https://b.example.com///, =missing, baz= ")

	require.Equal(t, map[string]string{
		"foo": "https://a.example.com",
		"bar": "https://b.example.com",
	}, mapping)
}

func TestBuildExternalResourceURLUsesConfiguredMapping(t *testing.T) {
	t.Setenv("INTEGRATION_UI_BASE_URLS", "fakecmdb=https://ui.example.com/")

	url := buildExternalResourceURL("fakecmdb", "table", "tbl_123")
	require.Equal(t, "https://ui.example.com/tables/tbl_123", url)
}

func TestBuildExternalResourceURLFallsBackToSearchForUnknownResourceType(t *testing.T) {
	t.Setenv("INTEGRATION_UI_BASE_URLS", "fakecmdb=https://ui.example.com/")

	url := buildExternalResourceURL("fakecmdb", "unknown_type", "abc_123")
	require.Equal(t, "https://ui.example.com/search?q=abc_123", url)
}

func TestBuildExternalResourceURLUsesFuckCMDBFallbackEnv(t *testing.T) {
	t.Setenv("INTEGRATION_UI_BASE_URLS", "")
	t.Setenv("FUCKCMDB_UI_BASE_URL", "https://cmdb.example.com/")

	url := buildExternalResourceURL("fuckcmdb", "column", "col_001")
	require.Equal(t, "https://cmdb.example.com/columns/col_001", url)
}

func TestBuildExternalResourceURLEscapesPathAndQueryParts(t *testing.T) {
	t.Setenv("INTEGRATION_UI_BASE_URLS", "fakecmdb=https://ui.example.com/")

	pathURL := buildExternalResourceURL("fakecmdb", "table", "folder/a b?")
	searchURL := buildExternalResourceURL("fakecmdb", "unknown_type", "folder/a b?")

	require.Equal(t, "https://ui.example.com/tables/folder%2Fa%20b%3F", pathURL)
	require.Equal(t, "https://ui.example.com/search?q=folder%2Fa+b%3F", searchURL)
}

func TestOutboundTimeoutSupportsDurationAndIntegerSeconds(t *testing.T) {
	t.Setenv("OUTBOUND_INTEGRATION_TIMEOUT_SEC", "1500ms")
	require.Equal(t, 1500*time.Millisecond, outboundTimeout())

	t.Setenv("OUTBOUND_INTEGRATION_TIMEOUT_SEC", "7")
	require.Equal(t, 7*time.Second, outboundTimeout())
}

func TestOutboxRetryDelayCapsBackoffMultiplier(t *testing.T) {
	t.Setenv("GOVERNANCE_OUTBOX_RETRY_INTERVAL_SEC", "10")

	require.Equal(t, 10*time.Second, outboxRetryDelay(0))
	require.Equal(t, 30*time.Second, outboxRetryDelay(2))
	require.Equal(t, 50*time.Second, outboxRetryDelay(10))
}

func setupGovernanceTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.User{},
		&models.ActivityLog{},
		&models.GovernanceTask{},
		&models.GovernanceReview{},
		&models.GovernanceEvidence{},
		&models.GovernanceExternalLink{},
		&models.GovernanceComment{},
		&models.GovernanceOutboxEvent{},
	)
	require.NoError(t, err)

	return db
}

func seedGovernanceUsers(t *testing.T, db *gorm.DB) (models.User, models.User) {
	creator := models.User{
		Username: "governance_creator",
		Email:    "creator@example.com",
		Password: "hashed_password",
	}
	reviewer := models.User{
		Username: "governance_reviewer",
		Email:    "reviewer@example.com",
		Password: "hashed_password",
	}

	require.NoError(t, db.Create(&creator).Error)
	require.NoError(t, db.Create(&reviewer).Error)

	return creator, reviewer
}

func createGovernanceTaskAndReviewForApply(
	t *testing.T,
	service *GovernanceService,
	creator models.User,
	reviewer models.User,
	sourceSystem string,
	reviewType string,
) (*models.GovernanceTask, *models.GovernanceReview) {
	t.Helper()

	task, err := service.CreateTask(CreateGovernanceTaskRequest{
		Title:        "待回写审核",
		Description:  "用于测试 outbox 状态流转",
		TaskType:     "classification_review",
		Priority:     "high",
		SourceSystem: sourceSystem,
		ResourceType: "table",
		ResourceID:   "resource_apply_001",
		AssigneeID:   creator.ID,
	}, creator.ID)
	require.NoError(t, err)

	review, err := service.CreateReview(CreateGovernanceReviewRequest{
		TaskID:          task.ID,
		ReviewType:      reviewType,
		ReviewerID:      reviewer.ID,
		ProposalSource:  "manual",
		ProposalPayload: `{"classification":"pii"}`,
	}, creator.ID)
	require.NoError(t, err)

	return task, review
}

func markReviewApprovedForApply(t *testing.T, db *gorm.DB, reviewID string) {
	t.Helper()

	now := time.Now()
	require.NoError(t, db.Model(&models.GovernanceReview{}).Where("id = ?", reviewID).Updates(map[string]interface{}{
		"status":           "approved",
		"decision_payload": `{"decision":"accept"}`,
		"reviewed_at":      &now,
		"apply_status":     applyStatusNotRequested,
	}).Error)
}

func TestGovernanceService_TaskLifecycle(t *testing.T) {
	db := setupGovernanceTestDB(t)
	service := NewGovernanceService(db)
	creator, assignee := seedGovernanceUsers(t, db)

	dueAt := time.Now().Add(24 * time.Hour).UTC()
	task, err := service.CreateTask(CreateGovernanceTaskRequest{
		Title:        "处理 panel_id 术语归一",
		Description:  "需要确认 panel_id 与 玻璃编号 的标准术语绑定",
		TaskType:     "term_review",
		Priority:     "high",
		AssigneeID:   assignee.ID,
		SourceSystem: "fuckcmdb",
		ResourceType: "column",
		ResourceID:   "col_001",
		DueAt:        &dueAt,
		ExternalLinks: []GovernanceExternalLinkInput{
			{
				SourceSystem: "fuckcmdb",
				ResourceType: "column",
				ResourceID:   "col_001",
				DisplayName:  "panel_id",
			},
		},
	}, creator.ID)
	require.NoError(t, err)
	require.Equal(t, "open", task.Status)
	require.Equal(t, "high", task.Priority)

	detail, err := service.GetTask(task.ID, assignee.ID)
	require.NoError(t, err)
	require.Len(t, detail.ExternalLinks, 1)
	require.Equal(t, "panel_id", detail.ExternalLinks[0].DisplayName)

	updated, err := service.UpdateTask(task.ID, UpdateGovernanceTaskRequest{
		Title:       task.Title,
		Description: task.Description,
		Status:      "done",
		Priority:    "critical",
		AssigneeID:  assignee.ID,
		DueAt:       &dueAt,
	}, assignee.ID)
	require.NoError(t, err)
	require.Equal(t, "done", updated.Status)
	require.NotNil(t, updated.CompletedAt)
}

func TestGovernanceService_EvidenceAndComment(t *testing.T) {
	db := setupGovernanceTestDB(t)
	service := NewGovernanceService(db)
	creator, assignee := seedGovernanceUsers(t, db)

	task, err := service.CreateTask(CreateGovernanceTaskRequest{
		Title:       "修复 DQ 异常",
		Description: "处理空值率超标",
		TaskType:    "dq_issue",
		AssigneeID:  assignee.ID,
	}, creator.ID)
	require.NoError(t, err)

	evidence, err := service.AddEvidence(task.ID, CreateGovernanceEvidenceRequest{
		EvidenceType: "sql",
		Content:      "UPDATE records SET panel_id = 'PANEL-001' WHERE panel_id IS NULL",
	}, assignee.ID)
	require.NoError(t, err)
	require.Equal(t, "sql", evidence.EvidenceType)

	comment, err := service.AddComment(task.ID, CreateGovernanceCommentRequest{
		Content: "已修复历史空值，待复检。",
	}, creator.ID)
	require.NoError(t, err)
	require.Equal(t, creator.ID, comment.CreatedBy)

	detail, err := service.GetTask(task.ID, creator.ID)
	require.NoError(t, err)
	require.Len(t, detail.Evidences, 1)
	require.Len(t, detail.Comments, 1)
}

func TestGovernanceService_ReviewDecision(t *testing.T) {
	db := setupGovernanceTestDB(t)
	service := NewGovernanceService(db)
	creator, reviewer := seedGovernanceUsers(t, db)

	task, err := service.CreateTask(CreateGovernanceTaskRequest{
		Title:       "审核字段分类建议",
		Description: "确认 customer_phone 是否属于 PII",
		TaskType:    "classification_review",
		AssigneeID:  creator.ID,
	}, creator.ID)
	require.NoError(t, err)

	review, err := service.CreateReview(CreateGovernanceReviewRequest{
		TaskID:          task.ID,
		ReviewType:      "classification",
		ReviewerID:      reviewer.ID,
		ProposalSource:  "llm-governor",
		ProposalPayload: `{"classification":"pii","confidence":0.93}`,
	}, creator.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", review.Status)

	review, err = service.DecideReview(review.ID, reviewer.ID, "approved", `{"decision":"accept"}`)
	require.NoError(t, err)
	require.Equal(t, "approved", review.Status)
	require.NotNil(t, review.ReviewedAt)

	detail, err := service.GetTask(task.ID, creator.ID)
	require.NoError(t, err)
	require.Equal(t, "open", detail.Task.Status)
	require.Len(t, detail.Reviews, 1)
}

func TestGovernanceService_SystemCreatedTaskAccessible(t *testing.T) {
	db := setupGovernanceTestDB(t)
	service := NewGovernanceService(db)
	user, _ := seedGovernanceUsers(t, db)

	task := models.GovernanceTask{
		Title:        "自动生成治理任务",
		Description:  "来自集成事件",
		TaskType:     "dq_issue",
		Status:       "open",
		Priority:     "high",
		SourceSystem: "fuckcmdb",
		ResourceType: "dq_result",
		ResourceID:   "dqr_100",
		CreatedBy:    "system:integration:fuckcmdb",
		AssigneeID:   user.ID,
	}
	require.NoError(t, db.Create(&task).Error)

	tasks, err := service.ListTasks(user.ID, GovernanceTaskListFilter{})
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, task.ID, tasks[0].ID)

	updated, err := service.UpdateTask(task.ID, UpdateGovernanceTaskRequest{
		Title:       task.Title,
		Description: task.Description,
		Status:      "done",
		Priority:    "critical",
		AssigneeID:  user.ID,
	}, user.ID)
	require.NoError(t, err)
	require.Equal(t, "done", updated.Status)
}

func TestGovernanceService_SystemCreatedTaskNotVisibleToUnassignedUser(t *testing.T) {
	db := setupGovernanceTestDB(t)
	service := NewGovernanceService(db)
	user, _ := seedGovernanceUsers(t, db)

	other := models.User{
		Username: "other_user",
		Email:    "other@example.com",
		Password: "hashed_password",
	}
	require.NoError(t, db.Create(&other).Error)

	task := models.GovernanceTask{
		Title:        "自动生成治理任务",
		Description:  "来自集成事件",
		TaskType:     "dq_issue",
		Status:       "open",
		Priority:     "high",
		SourceSystem: "fuckcmdb",
		ResourceType: "dq_result",
		ResourceID:   "dqr_101",
		CreatedBy:    "system:integration:fuckcmdb",
		AssigneeID:   user.ID,
	}
	require.NoError(t, db.Create(&task).Error)

	tasks, err := service.ListTasks(other.ID, GovernanceTaskListFilter{})
	require.NoError(t, err)
	require.Len(t, tasks, 0)

	_, err = service.UpdateTask(task.ID, UpdateGovernanceTaskRequest{
		Title:       task.Title,
		Description: task.Description,
		Status:      "done",
		Priority:    "critical",
		AssigneeID:  user.ID,
	}, other.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权访问")
}

func TestGovernanceService_ApproveReviewAutoApplies(t *testing.T) {
	db := setupGovernanceTestDB(t)
	service := NewGovernanceService(db)
	creator, reviewer := seedGovernanceUsers(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/integration/v1/recommendations/term-bindings", r.URL.Path)
		require.Equal(t, "Bearer apply-token", r.Header.Get("Authorization"))
		require.Equal(t, "cornerstone", r.Header.Get("X-Source-System"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","applied":true}`))
	}))
	defer server.Close()

	t.Setenv("INTEGRATION_BASE_URLS", "fuckcmdb="+server.URL)
	t.Setenv("OUTBOUND_INTEGRATION_TOKENS", "fuckcmdb=apply-token")

	task, err := service.CreateTask(CreateGovernanceTaskRequest{
		Title:        "审核术语建议",
		Description:  "确认字段术语",
		TaskType:     "term_review",
		SourceSystem: "fuckcmdb",
		ResourceType: "column",
		ResourceID:   "col_200",
		AssigneeID:   creator.ID,
		ExternalLinks: []GovernanceExternalLinkInput{
			{
				SourceSystem: "fuckcmdb",
				ResourceType: "column",
				ResourceID:   "col_200",
				DisplayName:  "panel_id",
			},
		},
	}, creator.ID)
	require.NoError(t, err)

	review, err := service.CreateReview(CreateGovernanceReviewRequest{
		TaskID:          task.ID,
		ReviewType:      "term_binding",
		ReviewerID:      reviewer.ID,
		ProposalSource:  "manual",
		ProposalPayload: `{"candidate_term":"panel_id","reason":"标准统一"}`,
	}, creator.ID)
	require.NoError(t, err)

	review, err = service.DecideReview(review.ID, reviewer.ID, "approved", `{"decision":"accept","note":"approved"}`)
	require.NoError(t, err)
	require.Equal(t, applyStatusSucceeded, review.ApplyStatus)

	detail, err := service.GetTask(task.ID, creator.ID)
	require.NoError(t, err)
	require.Equal(t, "done", detail.Task.Status)
	require.Len(t, detail.Comments, 1)
	require.Contains(t, detail.Comments[0].Content, "已成功回写")

	var outbox models.GovernanceOutboxEvent
	require.NoError(t, db.Where("review_id = ?", review.ID).First(&outbox).Error)
	require.Equal(t, applyStatusSucceeded, outbox.Status)
}

func TestGovernanceService_ApproveReviewApplyFailureTransitionsToDead(t *testing.T) {
	db := setupGovernanceTestDB(t)
	service := NewGovernanceService(db)
	creator, reviewer := seedGovernanceUsers(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"downstream failure"}`))
	}))
	defer server.Close()

	t.Setenv("INTEGRATION_BASE_URLS", "fakecmdb="+server.URL)
	t.Setenv("GOVERNANCE_OUTBOX_MAX_RETRIES", "1")

	task, review := createGovernanceTaskAndReviewForApply(t, service, creator, reviewer, "fakecmdb", "classification")

	_, err := service.DecideReview(review.ID, reviewer.ID, "approved", `{"decision":"accept"}`)
	require.Error(t, err)

	var updatedReview models.GovernanceReview
	require.NoError(t, db.Where("id = ?", review.ID).First(&updatedReview).Error)
	require.Equal(t, "approved", updatedReview.Status)
	require.Equal(t, applyStatusDead, updatedReview.ApplyStatus)
	require.Contains(t, updatedReview.ApplyError, "downstream failure")

	var outbox models.GovernanceOutboxEvent
	require.NoError(t, db.Where("review_id = ?", review.ID).First(&outbox).Error)
	require.Equal(t, applyStatusDead, outbox.Status)
	require.Equal(t, 1, outbox.RetryCount)
	require.Equal(t, http.StatusInternalServerError, outbox.LastResponseCode)
	require.Contains(t, outbox.LastError, "downstream failure")

	var taskCommentCount int64
	require.NoError(t, db.Model(&models.GovernanceComment{}).Where("task_id = ?", task.ID).Count(&taskCommentCount).Error)
	require.EqualValues(t, 1, taskCommentCount)
}

func TestGovernanceService_ProcessPendingOutboxOnlyHandlesDueEntries(t *testing.T) {
	db := setupGovernanceTestDB(t)
	service := NewGovernanceService(db)
	creator, reviewer := seedGovernanceUsers(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	t.Setenv("INTEGRATION_BASE_URLS", "fakecmdb="+server.URL)

	dueTask, dueReview := createGovernanceTaskAndReviewForApply(t, service, creator, reviewer, "fakecmdb", "classification")
	futureTask, futureReview := createGovernanceTaskAndReviewForApply(t, service, creator, reviewer, "fakecmdb", "classification")
	deadTask, deadReview := createGovernanceTaskAndReviewForApply(t, service, creator, reviewer, "fakecmdb", "classification")

	markReviewApprovedForApply(t, db, dueReview.ID)
	markReviewApprovedForApply(t, db, futureReview.ID)
	markReviewApprovedForApply(t, db, deadReview.ID)

	dueOutbox, err := service.EnqueueReviewApply(dueReview.ID, reviewer.ID, false)
	require.NoError(t, err)
	futureOutbox, err := service.EnqueueReviewApply(futureReview.ID, reviewer.ID, false)
	require.NoError(t, err)
	deadOutbox, err := service.EnqueueReviewApply(deadReview.ID, reviewer.ID, false)
	require.NoError(t, err)

	past := time.Now().Add(-time.Minute)
	future := time.Now().Add(time.Hour)
	require.NoError(t, db.Model(&models.GovernanceOutboxEvent{}).Where("id = ?", dueOutbox.ID).Updates(map[string]interface{}{
		"status":          applyStatusFailed,
		"retry_count":     1,
		"next_attempt_at": &past,
	}).Error)
	require.NoError(t, db.Model(&models.GovernanceReview{}).Where("id = ?", dueReview.ID).Update("apply_status", applyStatusFailed).Error)

	require.NoError(t, db.Model(&models.GovernanceOutboxEvent{}).Where("id = ?", futureOutbox.ID).Updates(map[string]interface{}{
		"status":          applyStatusFailed,
		"retry_count":     1,
		"next_attempt_at": &future,
	}).Error)
	require.NoError(t, db.Model(&models.GovernanceReview{}).Where("id = ?", futureReview.ID).Update("apply_status", applyStatusFailed).Error)

	require.NoError(t, db.Model(&models.GovernanceOutboxEvent{}).Where("id = ?", deadOutbox.ID).Updates(map[string]interface{}{
		"status": applyStatusDead,
	}).Error)
	require.NoError(t, db.Model(&models.GovernanceReview{}).Where("id = ?", deadReview.ID).Update("apply_status", applyStatusDead).Error)

	require.NoError(t, service.ProcessPendingOutbox(10))

	var refreshedDue models.GovernanceOutboxEvent
	var refreshedFuture models.GovernanceOutboxEvent
	var refreshedDead models.GovernanceOutboxEvent
	require.NoError(t, db.Where("id = ?", dueOutbox.ID).First(&refreshedDue).Error)
	require.NoError(t, db.Where("id = ?", futureOutbox.ID).First(&refreshedFuture).Error)
	require.NoError(t, db.Where("id = ?", deadOutbox.ID).First(&refreshedDead).Error)

	require.Equal(t, applyStatusSucceeded, refreshedDue.Status)
	require.Equal(t, applyStatusFailed, refreshedFuture.Status)
	require.Equal(t, applyStatusDead, refreshedDead.Status)

	var dueTaskState models.GovernanceTask
	var futureTaskState models.GovernanceTask
	var deadTaskState models.GovernanceTask
	require.NoError(t, db.Where("id = ?", dueTask.ID).First(&dueTaskState).Error)
	require.NoError(t, db.Where("id = ?", futureTask.ID).First(&futureTaskState).Error)
	require.NoError(t, db.Where("id = ?", deadTask.ID).First(&deadTaskState).Error)

	require.Equal(t, "done", dueTaskState.Status)
	require.Equal(t, "in_review", futureTaskState.Status)
	require.Equal(t, "in_review", deadTaskState.Status)
}

func TestGovernanceService_ApproveReviewApplyFailureThenRetrySucceeds(t *testing.T) {
	db := setupGovernanceTestDB(t)
	service := NewGovernanceService(db)
	creator, reviewer := seedGovernanceUsers(t, db)

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"temporary downstream failure"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","retry":"recovered"}`))
	}))
	defer server.Close()

	t.Setenv("INTEGRATION_BASE_URLS", "fakecmdb="+server.URL)
	t.Setenv("GOVERNANCE_OUTBOX_MAX_RETRIES", "3")

	task, review := createGovernanceTaskAndReviewForApply(t, service, creator, reviewer, "fakecmdb", "classification")

	_, err := service.DecideReview(review.ID, reviewer.ID, "approved", `{"decision":"accept"}`)
	require.Error(t, err)

	var failedReview models.GovernanceReview
	var failedOutbox models.GovernanceOutboxEvent
	require.NoError(t, db.Where("id = ?", review.ID).First(&failedReview).Error)
	require.NoError(t, db.Where("review_id = ?", review.ID).First(&failedOutbox).Error)
	require.Equal(t, applyStatusFailed, failedReview.ApplyStatus)
	require.Equal(t, applyStatusFailed, failedOutbox.Status)
	require.Equal(t, 1, failedOutbox.RetryCount)

	past := time.Now().Add(-time.Minute)
	require.NoError(t, db.Model(&models.GovernanceOutboxEvent{}).Where("id = ?", failedOutbox.ID).Update("next_attempt_at", &past).Error)

	require.NoError(t, service.ProcessPendingOutbox(10))

	var recoveredReview models.GovernanceReview
	var recoveredOutbox models.GovernanceOutboxEvent
	var recoveredTask models.GovernanceTask
	require.NoError(t, db.Where("id = ?", review.ID).First(&recoveredReview).Error)
	require.NoError(t, db.Where("id = ?", failedOutbox.ID).First(&recoveredOutbox).Error)
	require.NoError(t, db.Where("id = ?", task.ID).First(&recoveredTask).Error)

	require.Equal(t, 2, requests)
	require.Equal(t, applyStatusSucceeded, recoveredReview.ApplyStatus)
	require.Equal(t, applyStatusSucceeded, recoveredOutbox.Status)
	require.Equal(t, 1, recoveredOutbox.RetryCount)
	require.Equal(t, http.StatusOK, recoveredOutbox.LastResponseCode)
	require.Contains(t, recoveredOutbox.ResultPayload, "recovered")
	require.Equal(t, "done", recoveredTask.Status)
}

func TestGovernanceService_ProcessPendingOutboxHonorsLimitBoundary(t *testing.T) {
	db := setupGovernanceTestDB(t)
	service := NewGovernanceService(db)
	creator, reviewer := seedGovernanceUsers(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	t.Setenv("INTEGRATION_BASE_URLS", "fakecmdb="+server.URL)

	tasks := make([]models.GovernanceTask, 0, 3)
	reviews := make([]models.GovernanceReview, 0, 3)
	outboxes := make([]*models.GovernanceOutboxEvent, 0, 3)
	base := time.Now().Add(-time.Hour)

	for i := 0; i < 3; i++ {
		task, review := createGovernanceTaskAndReviewForApply(t, service, creator, reviewer, "fakecmdb", "classification")
		markReviewApprovedForApply(t, db, review.ID)
		outbox, err := service.EnqueueReviewApply(review.ID, reviewer.ID, false)
		require.NoError(t, err)

		createdAt := base.Add(time.Duration(i) * time.Minute)
		require.NoError(t, db.Model(&models.GovernanceOutboxEvent{}).Where("id = ?", outbox.ID).Updates(map[string]interface{}{
			"created_at":      createdAt,
			"updated_at":      createdAt,
			"next_attempt_at": nil,
			"status":          applyStatusPending,
		}).Error)

		tasks = append(tasks, *task)
		reviews = append(reviews, *review)
		outboxes = append(outboxes, outbox)
	}

	require.NoError(t, service.ProcessPendingOutbox(2))

	statuses := make([]string, 0, 3)
	taskStatuses := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		var refreshedOutbox models.GovernanceOutboxEvent
		var refreshedTask models.GovernanceTask
		var refreshedReview models.GovernanceReview
		require.NoError(t, db.Where("id = ?", outboxes[i].ID).First(&refreshedOutbox).Error)
		require.NoError(t, db.Where("id = ?", tasks[i].ID).First(&refreshedTask).Error)
		require.NoError(t, db.Where("id = ?", reviews[i].ID).First(&refreshedReview).Error)

		statuses = append(statuses, refreshedOutbox.Status)
		taskStatuses = append(taskStatuses, refreshedTask.Status)

		if i < 2 {
			require.Equal(t, applyStatusSucceeded, refreshedOutbox.Status)
			require.Equal(t, applyStatusSucceeded, refreshedReview.ApplyStatus)
			require.Equal(t, "done", refreshedTask.Status)
		} else {
			require.Equal(t, applyStatusPending, refreshedOutbox.Status)
			require.Equal(t, applyStatusPending, refreshedReview.ApplyStatus)
			require.Equal(t, "in_review", refreshedTask.Status)
		}
	}

	require.Equal(t, []string{applyStatusSucceeded, applyStatusSucceeded, applyStatusPending}, statuses)
	require.Equal(t, []string{"done", "done", "in_review"}, taskStatuses)
}

func TestGovernanceService_DispatchOutboxEventUsesStoredHTTPMethod(t *testing.T) {
	db := setupGovernanceTestDB(t)
	service := NewGovernanceService(db)
	creator, reviewer := seedGovernanceUsers(t, db)

	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","method":"accepted"}`))
	}))
	defer server.Close()

	task, review := createGovernanceTaskAndReviewForApply(t, service, creator, reviewer, "fakecmdb", "classification")
	markReviewApprovedForApply(t, db, review.ID)

	outbox := models.GovernanceOutboxEvent{
		EventType:    "governance.review.approved",
		SourceSystem: "cornerstone",
		TargetSystem: "fakecmdb",
		HTTPMethod:   http.MethodPut,
		Endpoint:     server.URL,
		Payload:      `{"review_id":"` + review.ID + `"}`,
		Status:       applyStatusPending,
		MaxRetries:   3,
		TaskID:       task.ID,
		ReviewID:     review.ID,
	}
	require.NoError(t, db.Create(&outbox).Error)

	require.NoError(t, service.DispatchOutboxEvent(outbox.ID))
	require.Equal(t, http.MethodPut, receivedMethod)

	var refreshedOutbox models.GovernanceOutboxEvent
	require.NoError(t, db.Where("id = ?", outbox.ID).First(&refreshedOutbox).Error)
	require.Equal(t, applyStatusSucceeded, refreshedOutbox.Status)
	require.Equal(t, http.StatusOK, refreshedOutbox.LastResponseCode)
}
