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

// ReceiveIntegrationEventRequest 入站集成事件请求
type ReceiveIntegrationEventRequest struct {
	EventID      string                 `json:"event_id" binding:"required,max=100"`
	EventType    string                 `json:"event_type" binding:"required,max=100"`
	OccurredAt   string                 `json:"occurred_at" binding:"max=50"`
	ResourceType string                 `json:"resource_type" binding:"max=50"`
	ResourceID   string                 `json:"resource_id" binding:"max=100"`
	ActorID      string                 `json:"actor_id" binding:"max=100"`
	TraceID      string                 `json:"trace_id" binding:"max=100"`
	Payload      map[string]interface{} `json:"payload"`
}

// ReceiveIntegrationEventResult 入站集成事件处理结果
type ReceiveIntegrationEventResult struct {
	Event    models.IntegrationInboundEvent `json:"event"`
	Task     *models.GovernanceTask         `json:"task,omitempty"`
	Duplicate bool                          `json:"duplicate"`
}

// IntegrationEventService 入站集成事件服务
type IntegrationEventService struct {
	db *gorm.DB
}

// NewIntegrationEventService 创建入站集成事件服务
func NewIntegrationEventService(db *gorm.DB) *IntegrationEventService {
	return &IntegrationEventService{db: db}
}

func stringifyPayload(payload map[string]interface{}) (string, error) {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化 payload 失败: %w", err)
	}
	return string(data), nil
}

func parsePriorityFromPayload(payload map[string]interface{}, fallback string) string {
	if payload == nil {
		return fallback
	}

	candidates := []string{
		fmt.Sprintf("%v", payload["priority"]),
		fmt.Sprintf("%v", payload["severity"]),
		fmt.Sprintf("%v", payload["risk_level"]),
	}

	for _, candidate := range candidates {
		value := strings.ToLower(strings.TrimSpace(candidate))
		switch value {
		case "critical", "fatal", "p0":
			return "critical"
		case "high", "error", "severe", "p1":
			return "high"
		case "medium", "warn", "warning", "p2":
			return "medium"
		case "low", "info", "p3":
			return "low"
		}
	}

	return fallback
}

func parseString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func taskFromEvent(sourceSystem string, req ReceiveIntegrationEventRequest) (*models.GovernanceTask, []models.GovernanceExternalLink, bool) {
	payload := req.Payload
	systemActor := "system:integration:" + sourceSystem

	title := ""
	description := ""
	taskType := "manual"
	priority := "medium"

	switch req.EventType {
	case "dq.alert.triggered", "dq.rule.failed":
		taskType = "dq_issue"
		priority = parsePriorityFromPayload(payload, "high")
		title = parseString(payload, "title")
		if title == "" {
			title = fmt.Sprintf("处理数据质量异常：%s", req.ResourceID)
		}
		description = parseString(payload, "summary")
		if description == "" {
			description = "来自 fuckcmdb 的数据质量异常事件，请确认规则、影响范围与整改动作。"
		}
	case "metadata.schema.changed":
		taskType = "schema_change"
		priority = parsePriorityFromPayload(payload, "medium")
		changeType := parseString(payload, "change_type")
		if changeType == "" {
			changeType = "schema_changed"
		}
		title = fmt.Sprintf("处理结构变更：%s", changeType)
		description = parseString(payload, "summary")
		if description == "" {
			description = "来自 fuckcmdb 的结构变更事件，请评估影响并安排整改。"
		}
	case "ai.recommendation.generated":
		recommendationType := parseString(payload, "recommendation_type")
		switch recommendationType {
		case "term_binding":
			taskType = "term_review"
			title = "审核术语绑定建议"
		case "classification":
			taskType = "classification_review"
			title = "审核分类分级建议"
		default:
			taskType = "manual"
			title = "审核 AI 治理建议"
		}
		priority = parsePriorityFromPayload(payload, "medium")
		description = parseString(payload, "reasoning_summary")
		if description == "" {
			description = "来自 AI 治理服务的推荐，请进行人工审核。"
		}
	default:
		return nil, nil, false
	}

	task := &models.GovernanceTask{
		Title:        title,
		Description:  description,
		TaskType:     taskType,
		Status:       "open",
		Priority:     priority,
		SourceSystem: sourceSystem,
		ResourceType: strings.TrimSpace(req.ResourceType),
		ResourceID:   strings.TrimSpace(req.ResourceID),
		CreatedBy:    systemActor,
		AssigneeID:   strings.TrimSpace(parseString(payload, "assignee_id")),
	}

	links := []models.GovernanceExternalLink{
		{
			SourceSystem: sourceSystem,
			ResourceType: strings.TrimSpace(req.ResourceType),
			ResourceID:   strings.TrimSpace(req.ResourceID),
			DisplayName:  strings.TrimSpace(parseString(payload, "display_name")),
		},
	}

	if links[0].DisplayName == "" {
		links[0].DisplayName = strings.TrimSpace(parseString(payload, "title"))
	}

	if links[0].ResourceID == "" || links[0].ResourceType == "" {
		links = nil
	}

	return task, links, true
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}

	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unique constraint") ||
		strings.Contains(lower, "duplicate key") ||
		strings.Contains(lower, "duplicated key") ||
		strings.Contains(lower, "unique failed")
}

// ReceiveEvent 接收入站集成事件
func (s *IntegrationEventService) ReceiveEvent(sourceSystem string, req ReceiveIntegrationEventRequest) (*ReceiveIntegrationEventResult, error) {
	sourceSystem = strings.TrimSpace(sourceSystem)
	if sourceSystem == "" {
		return nil, errors.New("sourceSystem 不能为空")
	}

	payload, err := stringifyPayload(req.Payload)
	if err != nil {
		return nil, err
	}

	var existing models.IntegrationInboundEvent
	err = s.db.Where("event_id = ? AND source_system = ?", req.EventID, sourceSystem).First(&existing).Error
	if err == nil {
		result := &ReceiveIntegrationEventResult{
			Event:     existing,
			Duplicate: true,
		}

		if existing.ResultTaskID != "" {
			var task models.GovernanceTask
			if taskErr := s.db.Where("id = ?", existing.ResultTaskID).First(&task).Error; taskErr == nil {
				result.Task = &task
			}
		}
		return result, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("查询入站事件失败: %w", err)
	}

	inbound := models.IntegrationInboundEvent{
		EventID:      strings.TrimSpace(req.EventID),
		EventType:    strings.TrimSpace(req.EventType),
		SourceSystem: sourceSystem,
		ResourceType: strings.TrimSpace(req.ResourceType),
		ResourceID:   strings.TrimSpace(req.ResourceID),
		ActorID:      strings.TrimSpace(req.ActorID),
		TraceID:      strings.TrimSpace(req.TraceID),
		Payload:      payload,
		Status:       "received",
	}

	result := &ReceiveIntegrationEventResult{}
	now := time.Now()
	var task *models.GovernanceTask

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&inbound).Error; err != nil {
			if isUniqueConstraintError(err) {
				var duplicate models.IntegrationInboundEvent
				if fetchErr := tx.Where("event_id = ? AND source_system = ?", req.EventID, sourceSystem).First(&duplicate).Error; fetchErr != nil {
					return fmt.Errorf("查询重复入站事件失败: %w", fetchErr)
				}
				result.Event = duplicate
				result.Duplicate = true
				if duplicate.ResultTaskID != "" {
					var duplicatedTask models.GovernanceTask
					if taskErr := tx.Where("id = ?", duplicate.ResultTaskID).First(&duplicatedTask).Error; taskErr == nil {
						result.Task = &duplicatedTask
					}
				}
				return nil
			}
			return fmt.Errorf("保存入站事件失败: %w", err)
		}

		taskCandidate, links, shouldCreateTask := taskFromEvent(sourceSystem, req)
		if !shouldCreateTask {
			inbound.Status = "ignored"
			inbound.ProcessedAt = &now
			if err := tx.Save(&inbound).Error; err != nil {
				return fmt.Errorf("更新入站事件状态失败: %w", err)
			}
			return nil
		}

		task = taskCandidate
		if err := tx.Create(task).Error; err != nil {
			inbound.Status = "failed"
			inbound.Error = err.Error()
			inbound.ProcessedAt = &now
			_ = tx.Save(&inbound).Error
			return fmt.Errorf("创建治理任务失败: %w", err)
		}

		for _, link := range links {
			link.TaskID = task.ID
			if err := tx.Create(&link).Error; err != nil {
				return fmt.Errorf("创建治理任务外链失败: %w", err)
			}
		}

		if err := tx.Create(&models.ActivityLog{
			UserID:       "system:integration:" + sourceSystem,
			Action:       "receive_event",
			ResourceType: "integration_event",
			ResourceID:   inbound.EventID,
			Description:  fmt.Sprintf("接收入站事件 %s 并创建治理任务 %s", inbound.EventType, task.ID),
		}).Error; err != nil {
			return fmt.Errorf("记录活动日志失败: %w", err)
		}

		inbound.Status = "processed"
		inbound.ResultTaskID = task.ID
		inbound.ProcessedAt = &now
		if err := tx.Save(&inbound).Error; err != nil {
			return fmt.Errorf("更新入站事件结果失败: %w", err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if result.Duplicate {
		return result, nil
	}

	result.Event = inbound
	result.Task = task
	return result, nil
}
