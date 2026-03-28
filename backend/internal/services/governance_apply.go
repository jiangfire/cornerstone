package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

const (
	applyStatusNotRequested = "not_requested"
	applyStatusPending      = "pending"
	applyStatusProcessing   = "processing"
	applyStatusSucceeded    = "succeeded"
	applyStatusFailed       = "failed"
	applyStatusDead         = "dead"
)

type applyRequestEnvelope struct {
	ReviewID        string          `json:"review_id"`
	TaskID          string          `json:"task_id"`
	ReviewType      string          `json:"review_type"`
	TaskType        string          `json:"task_type"`
	ResourceType    string          `json:"resource_type"`
	ResourceID      string          `json:"resource_id"`
	DisplayName     string          `json:"display_name,omitempty"`
	ProposalPayload json.RawMessage `json:"proposal_payload"`
	DecisionPayload json.RawMessage `json:"decision_payload"`
	ApprovedBy      string          `json:"approved_by"`
	ApprovedAt      string          `json:"approved_at"`
	SourceSystem    string          `json:"source_system"`
}

func parseEnvMapping(raw string) map[string]string {
	result := map[string]string{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			continue
		}
		result[key] = strings.TrimRight(value, "/")
	}
	return result
}

func buildExternalResourceURL(sourceSystem, resourceType, resourceID string) string {
	sourceSystem = strings.TrimSpace(sourceSystem)
	resourceType = strings.TrimSpace(resourceType)
	resourceID = strings.TrimSpace(resourceID)
	if sourceSystem == "" || resourceType == "" || resourceID == "" {
		return ""
	}

	baseURLs := parseEnvMapping(os.Getenv("INTEGRATION_UI_BASE_URLS"))
	baseURL := baseURLs[sourceSystem]
	if baseURL == "" && sourceSystem == "fuckcmdb" {
		baseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("FUCKCMDB_UI_BASE_URL")), "/")
	}
	if baseURL == "" {
		return ""
	}

	switch resourceType {
	case "column":
		return fmt.Sprintf("%s/columns/%s", baseURL, url.PathEscape(resourceID))
	case "table":
		return fmt.Sprintf("%s/tables/%s", baseURL, url.PathEscape(resourceID))
	case "dataset":
		return fmt.Sprintf("%s/datasets/%s", baseURL, url.PathEscape(resourceID))
	case "dq_rule":
		return fmt.Sprintf("%s/dq/rules/%s", baseURL, url.PathEscape(resourceID))
	case "dq_result":
		return fmt.Sprintf("%s/dq/results/%s", baseURL, url.PathEscape(resourceID))
	default:
		return fmt.Sprintf("%s/search?q=%s", baseURL, url.QueryEscape(resourceID))
	}
}

func (s *GovernanceService) resolveApplyTargetSystem(review *models.GovernanceReview) string {
	if review == nil || strings.TrimSpace(review.TaskID) == "" {
		return ""
	}

	var task models.GovernanceTask
	if err := s.db.Select("source_system").Where("id = ?", review.TaskID).First(&task).Error; err != nil {
		return ""
	}

	return strings.TrimSpace(task.SourceSystem)
}

func (s *GovernanceService) shouldEnqueueApply(review *models.GovernanceReview) bool {
	if review == nil {
		return false
	}

	switch review.ReviewType {
	case "term_binding", "classification", "dq_rule":
	default:
		return false
	}

	return strings.TrimSpace(s.resolveApplyTargetSystem(review)) != ""
}

func applyEndpointPath(reviewType string) string {
	switch strings.TrimSpace(reviewType) {
	case "term_binding":
		return "/api/integration/v1/recommendations/term-bindings"
	case "classification":
		return "/api/integration/v1/recommendations/classifications"
	case "dq_rule":
		return "/api/integration/v1/recommendations/dq-rules"
	default:
		return ""
	}
}

func outboundAuthTokenForTarget(target string) string {
	tokens := parseEnvMapping(os.Getenv("OUTBOUND_INTEGRATION_TOKENS"))
	if token := strings.TrimSpace(tokens[target]); token != "" {
		return token
	}
	return strings.TrimSpace(os.Getenv("INTEGRATION_SHARED_TOKEN"))
}

func outboundBaseURLForTarget(target string) string {
	baseURLs := parseEnvMapping(os.Getenv("INTEGRATION_BASE_URLS"))
	if baseURL := strings.TrimSpace(baseURLs[target]); baseURL != "" {
		return strings.TrimRight(baseURL, "/")
	}
	if target == "fuckcmdb" {
		return strings.TrimRight(strings.TrimSpace(os.Getenv("FUCKCMDB_BASE_URL")), "/")
	}
	return ""
}

func outboundTimeout() time.Duration {
	timeoutSec := 5
	if raw := strings.TrimSpace(os.Getenv("OUTBOUND_INTEGRATION_TIMEOUT_SEC")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			return parsed
		}
		var seconds int
		if _, err := fmt.Sscanf(raw, "%d", &seconds); err == nil && seconds > 0 {
			timeoutSec = seconds
		}
	}
	return time.Duration(timeoutSec) * time.Second
}

func outboxMaxRetries() int {
	maxRetries := 5
	if raw := strings.TrimSpace(os.Getenv("GOVERNANCE_OUTBOX_MAX_RETRIES")); raw != "" {
		var parsed int
		if _, err := fmt.Sscanf(raw, "%d", &parsed); err == nil && parsed > 0 {
			maxRetries = parsed
		}
	}
	return maxRetries
}

func outboxRetryDelay(retryCount int) time.Duration {
	baseSec := 60
	if raw := strings.TrimSpace(os.Getenv("GOVERNANCE_OUTBOX_RETRY_INTERVAL_SEC")); raw != "" {
		var parsed int
		if _, err := fmt.Sscanf(raw, "%d", &parsed); err == nil && parsed > 0 {
			baseSec = parsed
		}
	}

	multiplier := retryCount + 1
	if multiplier > 5 {
		multiplier = 5
	}
	return time.Duration(baseSec*multiplier) * time.Second
}

func decodeStoredJSON(payload string, field string) (json.RawMessage, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, fmt.Errorf("%s 不能为空", field)
	}

	var raw json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return nil, fmt.Errorf("%s 不是合法 JSON: %w", field, err)
	}
	return raw, nil
}

func (s *GovernanceService) buildApplyEnvelope(review *models.GovernanceReview, task *models.GovernanceTask) ([]byte, string, string, error) {
	if review == nil || task == nil {
		return nil, "", "", errors.New("治理审核或任务不存在")
	}

	targetSystem := strings.TrimSpace(task.SourceSystem)
	if targetSystem == "" {
		return nil, "", "", errors.New("任务未关联可回写的外部系统")
	}

	baseURL := outboundBaseURLForTarget(targetSystem)
	if baseURL == "" {
		return nil, "", "", fmt.Errorf("未配置 %s 的回写地址", targetSystem)
	}

	endpointPath := applyEndpointPath(review.ReviewType)
	if endpointPath == "" {
		return nil, "", "", fmt.Errorf("审核类型 %s 暂不支持自动回写", review.ReviewType)
	}

	proposalPayload, err := decodeStoredJSON(review.ProposalPayload, "proposal_payload")
	if err != nil {
		return nil, "", "", err
	}
	decisionPayload, err := decodeStoredJSON(review.DecisionPayload, "decision_payload")
	if err != nil {
		return nil, "", "", err
	}

	displayName := ""
	var link models.GovernanceExternalLink
	if err := s.db.Where("task_id = ?", task.ID).Order("created_at ASC").First(&link).Error; err == nil {
		displayName = strings.TrimSpace(link.DisplayName)
	}

	envelope := applyRequestEnvelope{
		ReviewID:        review.ID,
		TaskID:          task.ID,
		ReviewType:      review.ReviewType,
		TaskType:        task.TaskType,
		ResourceType:    task.ResourceType,
		ResourceID:      task.ResourceID,
		DisplayName:     displayName,
		ProposalPayload: proposalPayload,
		DecisionPayload: decisionPayload,
		ApprovedBy:      review.ReviewerID,
		ApprovedAt:      time.Now().UTC().Format(time.RFC3339),
		SourceSystem:    "cornerstone",
	}
	if review.ReviewedAt != nil {
		envelope.ApprovedAt = review.ReviewedAt.UTC().Format(time.RFC3339)
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		return nil, "", "", fmt.Errorf("序列化回写请求失败: %w", err)
	}

	return payload, targetSystem, baseURL + endpointPath, nil
}

func (s *GovernanceService) canApplyReview(reviewID, userID string) (*models.GovernanceReview, *models.GovernanceTask, error) {
	review, err := s.getReviewAccessibleByUser(reviewID, userID)
	if err != nil {
		return nil, nil, err
	}

	var task models.GovernanceTask
	if err := s.db.Where("id = ?", review.TaskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("治理任务不存在")
		}
		return nil, nil, fmt.Errorf("查询治理任务失败: %w", err)
	}

	return review, &task, nil
}

func (s *GovernanceService) EnqueueReviewApply(reviewID, userID string, dispatchNow bool) (*models.GovernanceOutboxEvent, error) {
	review, task, err := s.canApplyReview(reviewID, userID)
	if err != nil {
		return nil, err
	}
	if review.Status != "approved" {
		return nil, errors.New("只有已通过的治理审核才能执行回写")
	}

	payload, targetSystem, endpoint, err := s.buildApplyEnvelope(review, task)
	if err != nil {
		now := time.Now()
		updateErr := s.db.Model(&models.GovernanceReview{}).
			Where("id = ?", review.ID).
			Updates(map[string]interface{}{
				"apply_status": applyStatusFailed,
				"apply_error":  err.Error(),
				"updated_at":   now,
			}).Error
		if updateErr != nil {
			return nil, fmt.Errorf("%v；同时更新回写状态失败: %w", err, updateErr)
		}
		return nil, err
	}

	outbox := &models.GovernanceOutboxEvent{}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var existing models.GovernanceOutboxEvent
		findErr := tx.Where("review_id = ? AND status IN ?", review.ID, []string{
			applyStatusPending, applyStatusProcessing, applyStatusFailed, applyStatusDead,
		}).Order("created_at DESC").First(&existing).Error

		now := time.Now()
		if findErr == nil {
			existing.EventType = "governance.review.approved"
			existing.SourceSystem = "cornerstone"
			existing.TargetSystem = targetSystem
			existing.HTTPMethod = http.MethodPost
			existing.Endpoint = endpoint
			existing.Payload = string(payload)
			existing.Status = applyStatusPending
			existing.NextAttemptAt = &now
			existing.LastError = ""
			existing.ResultPayload = ""
			existing.ProcessedAt = nil
			if err := tx.Save(&existing).Error; err != nil {
				return fmt.Errorf("更新治理回写任务失败: %w", err)
			}
			*outbox = existing
		} else if errors.Is(findErr, gorm.ErrRecordNotFound) {
			nextAttemptAt := now
			record := models.GovernanceOutboxEvent{
				EventType:     "governance.review.approved",
				SourceSystem:  "cornerstone",
				TargetSystem:  targetSystem,
				HTTPMethod:    http.MethodPost,
				Endpoint:      endpoint,
				Payload:       string(payload),
				Status:        applyStatusPending,
				MaxRetries:    outboxMaxRetries(),
				NextAttemptAt: &nextAttemptAt,
				TaskID:        task.ID,
				ReviewID:      review.ID,
			}
			if err := tx.Create(&record).Error; err != nil {
				return fmt.Errorf("创建治理回写任务失败: %w", err)
			}
			*outbox = record
		} else {
			return fmt.Errorf("查询治理回写任务失败: %w", findErr)
		}

		if err := tx.Model(&models.GovernanceReview{}).
			Where("id = ?", review.ID).
			Updates(map[string]interface{}{
				"apply_status": applyStatusPending,
				"apply_error":  "",
				"apply_target": targetSystem,
				"updated_at":   now,
			}).Error; err != nil {
			return fmt.Errorf("更新治理审核回写状态失败: %w", err)
		}

		desc := fmt.Sprintf("为治理审核 %s 创建回写任务", review.ID)
		if err := s.logActivity(tx, userID, "queue_apply", "governance_review", review.ID, desc); err != nil {
			return fmt.Errorf("记录活动日志失败: %w", err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if dispatchNow {
		if err := s.DispatchOutboxEvent(outbox.ID); err != nil {
			return outbox, err
		}
	}

	return outbox, nil
}

func (s *GovernanceService) DispatchOutboxEvent(outboxID string) error {
	var outbox models.GovernanceOutboxEvent
	if err := s.db.Where("id = ?", outboxID).First(&outbox).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("治理回写任务不存在")
		}
		return fmt.Errorf("查询治理回写任务失败: %w", err)
	}

	if outbox.Status == applyStatusSucceeded {
		return nil
	}

	if err := s.db.Model(&models.GovernanceOutboxEvent{}).
		Where("id = ?", outbox.ID).
		Updates(map[string]interface{}{
			"status":     applyStatusProcessing,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("更新治理回写任务状态失败: %w", err)
	}

	client := &http.Client{Timeout: outboundTimeout()}
	method := strings.TrimSpace(outbox.HTTPMethod)
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequest(method, outbox.Endpoint, bytes.NewReader([]byte(outbox.Payload)))
	if err != nil {
		if markErr := s.markOutboxFailure(outbox, 0, err.Error()); markErr != nil {
			return markErr
		}
		return fmt.Errorf("治理回写请求构建失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Source-System", "cornerstone")
	if token := outboundAuthTokenForTarget(outbox.TargetSystem); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		if markErr := s.markOutboxFailure(outbox, 0, err.Error()); markErr != nil {
			return markErr
		}
		return fmt.Errorf("治理回写请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	bodyText := strings.TrimSpace(string(body))
	if len(bodyText) == 0 {
		bodyText = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if markErr := s.markOutboxFailure(outbox, resp.StatusCode, bodyText); markErr != nil {
			return markErr
		}
		return fmt.Errorf("治理回写返回非成功状态: %s", bodyText)
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&models.GovernanceOutboxEvent{}).
			Where("id = ?", outbox.ID).
			Updates(map[string]interface{}{
				"status":             applyStatusSucceeded,
				"processed_at":       now,
				"last_error":         "",
				"last_response_code": resp.StatusCode,
				"result_payload":     bodyText,
				"updated_at":         now,
			}).Error; err != nil {
			return fmt.Errorf("更新治理回写任务成功状态失败: %w", err)
		}

		if err := tx.Model(&models.GovernanceReview{}).
			Where("id = ?", outbox.ReviewID).
			Updates(map[string]interface{}{
				"apply_status": applyStatusSucceeded,
				"apply_error":  "",
				"apply_result": bodyText,
				"applied_at":   now,
				"updated_at":   now,
			}).Error; err != nil {
			return fmt.Errorf("更新治理审核回写成功状态失败: %w", err)
		}

		if err := tx.Model(&models.GovernanceTask{}).
			Where("id = ?", outbox.TaskID).
			Updates(map[string]interface{}{
				"status":     "done",
				"updated_at": now,
			}).Error; err != nil {
			return fmt.Errorf("更新治理任务应用状态失败: %w", err)
		}

		comment := models.GovernanceComment{
			TaskID:    outbox.TaskID,
			Content:   fmt.Sprintf("治理审核已成功回写到 %s，响应：%s", outbox.TargetSystem, bodyText),
			CreatedBy: "system:outbox",
		}
		if err := tx.Create(&comment).Error; err != nil {
			return fmt.Errorf("写入治理回写评论失败: %w", err)
		}

		if err := s.logActivity(tx, "system:outbox", "apply", "governance_review", outbox.ReviewID, fmt.Sprintf("治理审核已回写到 %s", outbox.TargetSystem)); err != nil {
			return fmt.Errorf("记录治理回写活动日志失败: %w", err)
		}

		return nil
	})
}

func (s *GovernanceService) markOutboxFailure(outbox models.GovernanceOutboxEvent, responseCode int, message string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		retryCount := outbox.RetryCount + 1
		status := applyStatusFailed
		var nextAttemptAt *time.Time
		if retryCount >= outbox.MaxRetries {
			status = applyStatusDead
		} else {
			nextRun := time.Now().Add(outboxRetryDelay(retryCount))
			nextAttemptAt = &nextRun
		}

		if err := tx.Model(&models.GovernanceOutboxEvent{}).
			Where("id = ?", outbox.ID).
			Updates(map[string]interface{}{
				"status":             status,
				"retry_count":        retryCount,
				"next_attempt_at":    nextAttemptAt,
				"last_error":         sanitizeText(message),
				"last_response_code": responseCode,
				"updated_at":         time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("更新治理回写失败状态失败: %w", err)
		}

		if err := tx.Model(&models.GovernanceReview{}).
			Where("id = ?", outbox.ReviewID).
			Updates(map[string]interface{}{
				"apply_status": status,
				"apply_error":  sanitizeText(message),
				"updated_at":   time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("更新治理审核失败状态失败: %w", err)
		}

		if status == applyStatusDead {
			comment := models.GovernanceComment{
				TaskID:    outbox.TaskID,
				Content:   fmt.Sprintf("治理审核回写失败且已达到最大重试次数：%s", sanitizeText(message)),
				CreatedBy: "system:outbox",
			}
			if err := tx.Create(&comment).Error; err != nil {
				return fmt.Errorf("写入治理回写失败评论失败: %w", err)
			}
		}

		if err := s.logActivity(tx, "system:outbox", "apply_failed", "governance_review", outbox.ReviewID, fmt.Sprintf("治理审核回写失败：%s", sanitizeText(message))); err != nil {
			return fmt.Errorf("记录治理回写失败日志失败: %w", err)
		}

		return nil
	})
}

func (s *GovernanceService) ProcessPendingOutbox(limit int) error {
	if limit <= 0 {
		limit = 20
	}

	var outboxes []models.GovernanceOutboxEvent
	if err := s.db.Where(
		"status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)",
		[]string{applyStatusPending, applyStatusFailed},
		time.Now(),
	).Order("created_at ASC").Limit(limit).Find(&outboxes).Error; err != nil {
		return fmt.Errorf("查询待处理治理回写任务失败: %w", err)
	}

	var dispatchErrors []string
	for _, item := range outboxes {
		if err := s.DispatchOutboxEvent(item.ID); err != nil {
			dispatchErrors = append(dispatchErrors, err.Error())
		}
	}

	if len(dispatchErrors) > 0 {
		return fmt.Errorf("部分治理回写任务处理失败: %s", strings.Join(dispatchErrors, "; "))
	}
	return nil
}
