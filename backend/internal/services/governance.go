package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

var (
	allowedTaskTypes = map[string]struct{}{
		"schema_change":         {},
		"dq_issue":              {},
		"term_review":           {},
		"classification_review": {},
		"design_review":         {},
		"remediation":           {},
		"manual":                {},
	}
	allowedTaskStatuses = map[string]struct{}{
		"open":      {},
		"in_review": {},
		"blocked":   {},
		"done":      {},
		"cancelled": {},
	}
	allowedPriority = map[string]struct{}{
		"low":      {},
		"medium":   {},
		"high":     {},
		"critical": {},
	}
	allowedReviewTypes = map[string]struct{}{
		"term_binding":       {},
		"classification":     {},
		"dq_rule":            {},
		"design_validation":  {},
		"remediation_result": {},
		"generic":            {},
	}
	allowedReviewStatus = map[string]struct{}{
		"pending":   {},
		"approved":  {},
		"rejected":  {},
		"cancelled": {},
	}
	allowedEvidenceTypes = map[string]struct{}{
		"note":       {},
		"file":       {},
		"link":       {},
		"sql":        {},
		"screenshot": {},
	}
)

// GovernanceService 治理域服务
type GovernanceService struct {
	db *gorm.DB
}

// NewGovernanceService 创建治理域服务
func NewGovernanceService(db *gorm.DB) *GovernanceService {
	return &GovernanceService{db: db}
}

// GovernanceExternalLinkInput 外部资源引用
type GovernanceExternalLinkInput struct {
	SourceSystem string `json:"source_system" binding:"required,max=50"`
	ResourceType string `json:"resource_type" binding:"required,max=50"`
	ResourceID   string `json:"resource_id" binding:"required,max=100"`
	DisplayName  string `json:"display_name" binding:"max=255"`
}

// CreateGovernanceTaskRequest 创建治理任务请求
type CreateGovernanceTaskRequest struct {
	Title         string                        `json:"title" binding:"required,min=2,max=255"`
	Description   string                        `json:"description" binding:"max=5000"`
	TaskType      string                        `json:"task_type" binding:"required"`
	Priority      string                        `json:"priority" binding:"omitempty"`
	SourceSystem  string                        `json:"source_system" binding:"max=50"`
	ResourceType  string                        `json:"resource_type" binding:"max=50"`
	ResourceID    string                        `json:"resource_id" binding:"max=100"`
	AssigneeID    string                        `json:"assignee_id" binding:"max=50"`
	DueAt         *time.Time                    `json:"due_at"`
	ExternalLinks []GovernanceExternalLinkInput `json:"external_links"`
}

// UpdateGovernanceTaskRequest 更新治理任务请求
type UpdateGovernanceTaskRequest struct {
	Title       string     `json:"title" binding:"required,min=2,max=255"`
	Description string     `json:"description" binding:"max=5000"`
	Status      string     `json:"status" binding:"required"`
	Priority    string     `json:"priority" binding:"required"`
	AssigneeID  string     `json:"assignee_id" binding:"max=50"`
	DueAt       *time.Time `json:"due_at"`
}

// GovernanceTaskListFilter 治理任务查询条件
type GovernanceTaskListFilter struct {
	Status       string
	TaskType     string
	Priority     string
	SourceSystem string
	ResourceType string
	ResourceID   string
}

// CreateGovernanceEvidenceRequest 创建证据请求
type CreateGovernanceEvidenceRequest struct {
	EvidenceType string `json:"evidence_type" binding:"required"`
	Content      string `json:"content" binding:"max=5000"`
	FileID       string `json:"file_id" binding:"max=50"`
}

// CreateGovernanceCommentRequest 创建评论请求
type CreateGovernanceCommentRequest struct {
	Content string `json:"content" binding:"required,min=1,max=5000"`
}

// CreateGovernanceReviewRequest 创建审核请求
type CreateGovernanceReviewRequest struct {
	TaskID          string `json:"task_id" binding:"required,max=50"`
	ReviewType      string `json:"review_type" binding:"required"`
	ReviewerID      string `json:"reviewer_id" binding:"required,max=50"`
	ProposalSource  string `json:"proposal_source" binding:"max=50"`
	ProposalPayload string `json:"proposal_payload" binding:"required"`
}

// ReviewDecisionRequest 审核结论请求
type ReviewDecisionRequest struct {
	DecisionPayload string `json:"decision_payload" binding:"required"`
}

// GovernanceTaskDetail 治理任务详情响应
type GovernanceTaskDetail struct {
	Task          models.GovernanceTask           `json:"task"`
	ExternalLinks []models.GovernanceExternalLink `json:"external_links"`
	Evidences     []models.GovernanceEvidence     `json:"evidences"`
	Comments      []models.GovernanceComment      `json:"comments"`
	Reviews       []models.GovernanceReview       `json:"reviews"`
}

func validateEnum(value string, allowed map[string]struct{}, field, fallback string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		value = fallback
	}
	if _, ok := allowed[value]; !ok {
		return "", fmt.Errorf("%s 无效: %s", field, value)
	}
	return value, nil
}

func sanitizeText(input string) string {
	input = strings.TrimSpace(input)
	input = strings.ReplaceAll(input, "\x00", "")
	return input
}

func isSystemActor(actorID string) bool {
	return strings.HasPrefix(strings.TrimSpace(actorID), "system:")
}

func validateJSONPayload(payload string, field string) (string, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return "", fmt.Errorf("%s 不能为空", field)
	}

	var raw json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return "", fmt.Errorf("%s 必须是合法 JSON: %w", field, err)
	}

	return payload, nil
}

func (s *GovernanceService) ensureUserExists(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return nil
	}

	var count int64
	if err := s.db.Model(&models.User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
		return fmt.Errorf("查询用户失败: %w", err)
	}
	if count == 0 {
		return errors.New("用户不存在")
	}
	return nil
}

func (s *GovernanceService) getTaskAccessibleByUser(taskID, userID string) (*models.GovernanceTask, error) {
	var task models.GovernanceTask
	err := s.db.Where(
		"id = ? AND (created_by = ? OR assignee_id = ?)",
		taskID, userID, userID,
	).First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("治理任务不存在或无权访问")
		}
		return nil, fmt.Errorf("查询治理任务失败: %w", err)
	}
	return &task, nil
}

func (s *GovernanceService) getTaskManageableByUser(taskID, userID string) (*models.GovernanceTask, error) {
	task, err := s.getTaskAccessibleByUser(taskID, userID)
	if err != nil {
		return nil, err
	}
	if task.CreatedBy != userID && task.AssigneeID != userID {
		return nil, errors.New("无权修改治理任务")
	}
	return task, nil
}

func (s *GovernanceService) logActivity(tx *gorm.DB, userID, action, resourceType, resourceID, description string) error {
	log := models.ActivityLog{
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Description:  description,
	}
	return tx.Create(&log).Error
}

// CreateTask 创建治理任务
func (s *GovernanceService) CreateTask(req CreateGovernanceTaskRequest, creatorID string) (*models.GovernanceTask, error) {
	title := sanitizeText(req.Title)
	description := sanitizeText(req.Description)
	sourceSystem := sanitizeText(req.SourceSystem)
	resourceType := sanitizeText(req.ResourceType)
	resourceID := sanitizeText(req.ResourceID)

	taskType, err := validateEnum(req.TaskType, allowedTaskTypes, "task_type", "")
	if err != nil {
		return nil, err
	}

	priority, err := validateEnum(req.Priority, allowedPriority, "priority", "medium")
	if err != nil {
		return nil, err
	}

	if err := s.ensureUserExists(creatorID); err != nil {
		return nil, err
	}
	if err := s.ensureUserExists(req.AssigneeID); err != nil {
		return nil, err
	}

	task := models.GovernanceTask{
		Title:        title,
		Description:  description,
		TaskType:     taskType,
		Status:       "open",
		Priority:     priority,
		SourceSystem: sourceSystem,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		AssigneeID:   strings.TrimSpace(req.AssigneeID),
		CreatedBy:    creatorID,
		DueAt:        req.DueAt,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&task).Error; err != nil {
			return fmt.Errorf("创建治理任务失败: %w", err)
		}

		for _, link := range req.ExternalLinks {
			record := models.GovernanceExternalLink{
				TaskID:       task.ID,
				SourceSystem: sanitizeText(link.SourceSystem),
				ResourceType: sanitizeText(link.ResourceType),
				ResourceID:   sanitizeText(link.ResourceID),
				DisplayName:  sanitizeText(link.DisplayName),
			}
			if err := tx.Create(&record).Error; err != nil {
				return fmt.Errorf("创建外部资源引用失败: %w", err)
			}
		}

		desc := fmt.Sprintf("创建治理任务 '%s'", task.Title)
		if err := s.logActivity(tx, creatorID, "create", "governance_task", task.ID, desc); err != nil {
			return fmt.Errorf("记录活动日志失败: %w", err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &task, nil
}

// ListTasks 获取当前用户可见的治理任务
func (s *GovernanceService) ListTasks(userID string, filter GovernanceTaskListFilter) ([]models.GovernanceTask, error) {
	query := s.db.Model(&models.GovernanceTask{}).
		Where("created_by = ? OR assignee_id = ?", userID, userID)

	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.TaskType) != "" {
		query = query.Where("task_type = ?", strings.TrimSpace(filter.TaskType))
	}
	if strings.TrimSpace(filter.Priority) != "" {
		query = query.Where("priority = ?", strings.TrimSpace(filter.Priority))
	}
	if strings.TrimSpace(filter.SourceSystem) != "" {
		query = query.Where("source_system = ?", strings.TrimSpace(filter.SourceSystem))
	}
	if strings.TrimSpace(filter.ResourceType) != "" {
		query = query.Where("resource_type = ?", strings.TrimSpace(filter.ResourceType))
	}
	if strings.TrimSpace(filter.ResourceID) != "" {
		query = query.Where("resource_id = ?", strings.TrimSpace(filter.ResourceID))
	}

	var tasks []models.GovernanceTask
	if err := query.Order("created_at DESC").Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("查询治理任务失败: %w", err)
	}
	return tasks, nil
}

// GetTask 获取治理任务详情
func (s *GovernanceService) GetTask(taskID, userID string) (*GovernanceTaskDetail, error) {
	task, err := s.getTaskAccessibleByUser(taskID, userID)
	if err != nil {
		return nil, err
	}

	var (
		links     []models.GovernanceExternalLink
		evidences []models.GovernanceEvidence
		comments  []models.GovernanceComment
		reviews   []models.GovernanceReview
	)

	if err := s.db.Where("task_id = ?", task.ID).Order("created_at ASC").Find(&links).Error; err != nil {
		return nil, fmt.Errorf("查询治理外链失败: %w", err)
	}
	for i := range links {
		links[i].TargetURL = buildExternalResourceURL(links[i].SourceSystem, links[i].ResourceType, links[i].ResourceID)
	}
	if err := s.db.Where("task_id = ?", task.ID).Order("created_at DESC").Find(&evidences).Error; err != nil {
		return nil, fmt.Errorf("查询治理证据失败: %w", err)
	}
	if err := s.db.Where("task_id = ?", task.ID).Order("created_at ASC").Find(&comments).Error; err != nil {
		return nil, fmt.Errorf("查询治理评论失败: %w", err)
	}
	if err := s.db.Where("task_id = ?", task.ID).Order("created_at DESC").Find(&reviews).Error; err != nil {
		return nil, fmt.Errorf("查询治理审核失败: %w", err)
	}

	return &GovernanceTaskDetail{
		Task:          *task,
		ExternalLinks: links,
		Evidences:     evidences,
		Comments:      comments,
		Reviews:       reviews,
	}, nil
}

// UpdateTask 更新治理任务
func (s *GovernanceService) UpdateTask(taskID string, req UpdateGovernanceTaskRequest, userID string) (*models.GovernanceTask, error) {
	task, err := s.getTaskManageableByUser(taskID, userID)
	if err != nil {
		return nil, err
	}

	status, err := validateEnum(req.Status, allowedTaskStatuses, "status", "")
	if err != nil {
		return nil, err
	}
	priority, err := validateEnum(req.Priority, allowedPriority, "priority", "medium")
	if err != nil {
		return nil, err
	}
	if err := s.ensureUserExists(req.AssigneeID); err != nil {
		return nil, err
	}

	task.Title = sanitizeText(req.Title)
	task.Description = sanitizeText(req.Description)
	task.Status = status
	task.Priority = priority
	task.AssigneeID = strings.TrimSpace(req.AssigneeID)
	task.DueAt = req.DueAt

	now := time.Now()
	if status == "done" {
		task.CompletedAt = &now
	} else {
		task.CompletedAt = nil
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(task).Error; err != nil {
			return fmt.Errorf("更新治理任务失败: %w", err)
		}

		desc := fmt.Sprintf("更新治理任务 '%s'，状态为 %s", task.Title, task.Status)
		if err := s.logActivity(tx, userID, "update", "governance_task", task.ID, desc); err != nil {
			return fmt.Errorf("记录活动日志失败: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return task, nil
}

// AddEvidence 添加治理证据
func (s *GovernanceService) AddEvidence(taskID string, req CreateGovernanceEvidenceRequest, userID string) (*models.GovernanceEvidence, error) {
	task, err := s.getTaskManageableByUser(taskID, userID)
	if err != nil {
		return nil, err
	}

	evidenceType, err := validateEnum(req.EvidenceType, allowedEvidenceTypes, "evidence_type", "")
	if err != nil {
		return nil, err
	}

	evidence := models.GovernanceEvidence{
		TaskID:       task.ID,
		EvidenceType: evidenceType,
		Content:      sanitizeText(req.Content),
		FileID:       strings.TrimSpace(req.FileID),
		CreatedBy:    userID,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&evidence).Error; err != nil {
			return fmt.Errorf("创建治理证据失败: %w", err)
		}

		desc := fmt.Sprintf("为治理任务 '%s' 添加证据", task.Title)
		if err := s.logActivity(tx, userID, "create", "governance_evidence", evidence.ID, desc); err != nil {
			return fmt.Errorf("记录活动日志失败: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &evidence, nil
}

// AddComment 添加治理评论
func (s *GovernanceService) AddComment(taskID string, req CreateGovernanceCommentRequest, userID string) (*models.GovernanceComment, error) {
	task, err := s.getTaskAccessibleByUser(taskID, userID)
	if err != nil {
		return nil, err
	}

	comment := models.GovernanceComment{
		TaskID:    task.ID,
		Content:   sanitizeText(req.Content),
		CreatedBy: userID,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&comment).Error; err != nil {
			return fmt.Errorf("创建治理评论失败: %w", err)
		}

		now := time.Now()
		if err := tx.Model(&models.GovernanceTask{}).
			Where("id = ?", task.ID).
			Updates(map[string]interface{}{
				"last_comment_at": now,
				"updated_at":      now,
			}).Error; err != nil {
			return fmt.Errorf("更新任务评论时间失败: %w", err)
		}

		desc := fmt.Sprintf("为治理任务 '%s' 添加评论", task.Title)
		if err := s.logActivity(tx, userID, "comment", "governance_task", task.ID, desc); err != nil {
			return fmt.Errorf("记录活动日志失败: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &comment, nil
}

func (s *GovernanceService) getReviewAccessibleByUser(reviewID, userID string) (*models.GovernanceReview, error) {
	var review models.GovernanceReview
	err := s.db.Where("id = ?", reviewID).First(&review).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("治理审核不存在或无权访问")
		}
		return nil, fmt.Errorf("查询治理审核失败: %w", err)
	}

	if review.CreatedBy == userID || review.ReviewerID == userID {
		return &review, nil
	}

	if _, err := s.getTaskAccessibleByUser(review.TaskID, userID); err != nil {
		return nil, errors.New("治理审核不存在或无权访问")
	}
	return &review, nil
}

// CreateReview 创建治理审核
func (s *GovernanceService) CreateReview(req CreateGovernanceReviewRequest, userID string) (*models.GovernanceReview, error) {
	reviewType, err := validateEnum(req.ReviewType, allowedReviewTypes, "review_type", "")
	if err != nil {
		return nil, err
	}
	if err := s.ensureUserExists(req.ReviewerID); err != nil {
		return nil, err
	}
	if _, err := s.getTaskAccessibleByUser(req.TaskID, userID); err != nil {
		return nil, err
	}

	payload, err := validateJSONPayload(req.ProposalPayload, "proposal_payload")
	if err != nil {
		return nil, err
	}

	review := models.GovernanceReview{
		TaskID:          req.TaskID,
		ReviewType:      reviewType,
		Status:          "pending",
		ProposalSource:  sanitizeText(req.ProposalSource),
		ProposalPayload: payload,
		ReviewerID:      strings.TrimSpace(req.ReviewerID),
		CreatedBy:       userID,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&review).Error; err != nil {
			return fmt.Errorf("创建治理审核失败: %w", err)
		}

		if err := tx.Model(&models.GovernanceTask{}).
			Where("id = ?", req.TaskID).
			Update("status", "in_review").Error; err != nil {
			return fmt.Errorf("更新治理任务审核状态失败: %w", err)
		}

		desc := fmt.Sprintf("创建治理审核 '%s'", review.ReviewType)
		if err := s.logActivity(tx, userID, "create", "governance_review", review.ID, desc); err != nil {
			return fmt.Errorf("记录活动日志失败: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &review, nil
}

// GetReview 获取治理审核详情
func (s *GovernanceService) GetReview(reviewID, userID string) (*models.GovernanceReview, error) {
	return s.getReviewAccessibleByUser(reviewID, userID)
}

// DecideReview 审核通过或拒绝
func (s *GovernanceService) DecideReview(reviewID, userID, targetStatus, decisionPayload string) (*models.GovernanceReview, error) {
	review, err := s.getReviewAccessibleByUser(reviewID, userID)
	if err != nil {
		return nil, err
	}
	if review.ReviewerID != userID {
		return nil, errors.New("只有指定审核人可以提交审核结论")
	}
	if review.Status != "pending" {
		return nil, errors.New("该治理审核已处理")
	}

	targetStatus, err = validateEnum(targetStatus, allowedReviewStatus, "status", "")
	if err != nil {
		return nil, err
	}
	if targetStatus != "approved" && targetStatus != "rejected" {
		return nil, errors.New("治理审核结论只允许 approved 或 rejected")
	}

	decisionPayload, err = validateJSONPayload(decisionPayload, "decision_payload")
	if err != nil {
		return nil, err
	}

	now := time.Now()
	review.Status = targetStatus
	review.DecisionPayload = decisionPayload
	review.ReviewedAt = &now
	review.ApplyStatus = "not_requested"
	review.ApplyError = ""
	review.ApplyResult = ""
	review.AppliedAt = nil

	taskStatus := "blocked"
	if targetStatus == "approved" {
		taskStatus = "open"
		if s.shouldEnqueueApply(review) {
			review.ApplyStatus = "pending"
			review.ApplyTarget = s.resolveApplyTargetSystem(review)
		}
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(review).Error; err != nil {
			return fmt.Errorf("更新治理审核失败: %w", err)
		}

		if err := tx.Model(&models.GovernanceTask{}).
			Where("id = ?", review.TaskID).
			Updates(map[string]interface{}{
				"status":     taskStatus,
				"updated_at": now,
			}).Error; err != nil {
			return fmt.Errorf("更新治理任务状态失败: %w", err)
		}

		desc := fmt.Sprintf("治理审核已%s", targetStatus)
		if err := s.logActivity(tx, userID, targetStatus, "governance_review", review.ID, desc); err != nil {
			return fmt.Errorf("记录活动日志失败: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if targetStatus == "approved" && review.ApplyStatus == "pending" {
		if _, err := s.EnqueueReviewApply(review.ID, userID, true); err != nil {
			return nil, err
		}
		if err := s.db.Where("id = ?", review.ID).First(review).Error; err != nil {
			return nil, fmt.Errorf("查询治理审核最新状态失败: %w", err)
		}
	}

	return review, nil
}
