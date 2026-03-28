package handlers

import (
	"strings"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

func publishMCPNotificationToUsers(userIDs []string, method string, params interface{}) {
	seen := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		mcpHub.PublishToUser(userID, method, params)
	}
}

func publishDatabaseChanged(userIDs []string, action string, database *models.Database) {
	if database == nil {
		return
	}
	publishMCPNotificationToUsers(userIDs, "notifications/databases/changed", map[string]interface{}{
		"action":   strings.TrimSpace(action),
		"database": database,
	})
}

func publishTableChanged(userIDs []string, action string, table *models.Table) {
	if table == nil {
		return
	}
	publishMCPNotificationToUsers(userIDs, "notifications/tables/changed", map[string]interface{}{
		"action": strings.TrimSpace(action),
		"table":  table,
	})
}

func publishFieldChanged(userIDs []string, action string, field *models.Field) {
	if field == nil {
		return
	}
	publishMCPNotificationToUsers(userIDs, "notifications/fields/changed", map[string]interface{}{
		"action": strings.TrimSpace(action),
		"field":  field,
	})
}

func publishGovernanceTaskChanged(userIDs []string, action string, task *models.GovernanceTask) {
	if task == nil {
		return
	}
	publishMCPNotificationToUsers(userIDs, "notifications/governance/tasks/changed", map[string]interface{}{
		"action": strings.TrimSpace(action),
		"task":   task,
	})
}

func publishGovernanceReviewChanged(userIDs []string, action string, review *models.GovernanceReview) {
	if review == nil {
		return
	}
	publishMCPNotificationToUsers(userIDs, "notifications/governance/reviews/changed", map[string]interface{}{
		"action": strings.TrimSpace(action),
		"review": review,
	})
}

func governanceTaskAudience(task *models.GovernanceTask, fallbackUserID string) []string {
	if task == nil {
		return []string{fallbackUserID}
	}
	return []string{fallbackUserID, task.CreatedBy, task.AssigneeID}
}

func governanceReviewAudience(review *models.GovernanceReview, task *models.GovernanceTask, fallbackUserID string) []string {
	userIDs := []string{fallbackUserID}
	if review != nil {
		userIDs = append(userIDs, review.CreatedBy, review.ReviewerID)
	}
	if task != nil {
		userIDs = append(userIDs, task.CreatedBy, task.AssigneeID)
	}
	return userIDs
}

func loadGovernanceTask(taskID string) *models.GovernanceTask {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}

	var task models.GovernanceTask
	if err := db.DB().Where("id = ?", taskID).First(&task).Error; err != nil {
		return nil
	}
	return &task
}

func loadGovernanceReview(reviewID string) *models.GovernanceReview {
	reviewID = strings.TrimSpace(reviewID)
	if reviewID == "" {
		return nil
	}

	var review models.GovernanceReview
	if err := db.DB().Where("id = ?", reviewID).First(&review).Error; err != nil {
		return nil
	}
	return &review
}
