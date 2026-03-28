package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// CreateGovernanceTask 创建治理任务
func CreateGovernanceTask(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.CreateGovernanceTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	governanceService := services.NewGovernanceService(db.DB())
	task, err := governanceService.CreateTask(req, userID)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	publishGovernanceTaskChanged(governanceTaskAudience(task, userID), "created", task)

	types.Success(c, task)
}

// ListGovernanceTasks 查询治理任务
func ListGovernanceTasks(c *gin.Context) {
	userID := middleware.GetUserID(c)

	filter := services.GovernanceTaskListFilter{
		Status:       c.Query("status"),
		TaskType:     c.Query("task_type"),
		Priority:     c.Query("priority"),
		SourceSystem: c.Query("source_system"),
		ResourceType: c.Query("resource_type"),
		ResourceID:   c.Query("resource_id"),
	}

	governanceService := services.NewGovernanceService(db.DB())
	tasks, err := governanceService.ListTasks(userID, filter)
	if err != nil {
		types.Error(c, 500, err.Error())
		return
	}

	types.Success(c, gin.H{
		"tasks": tasks,
		"total": len(tasks),
	})
}

// GetGovernanceTask 获取治理任务详情
func GetGovernanceTask(c *gin.Context) {
	userID := middleware.GetUserID(c)
	taskID := c.Param("id")

	governanceService := services.NewGovernanceService(db.DB())
	detail, err := governanceService.GetTask(taskID, userID)
	if err != nil {
		types.Error(c, 404, err.Error())
		return
	}

	types.Success(c, detail)
}

// UpdateGovernanceTask 更新治理任务
func UpdateGovernanceTask(c *gin.Context) {
	userID := middleware.GetUserID(c)
	taskID := c.Param("id")

	var req services.UpdateGovernanceTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	governanceService := services.NewGovernanceService(db.DB())
	task, err := governanceService.UpdateTask(taskID, req, userID)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	publishGovernanceTaskChanged(governanceTaskAudience(task, userID), "updated", task)

	types.Success(c, task)
}

// CreateGovernanceEvidence 创建治理证据
func CreateGovernanceEvidence(c *gin.Context) {
	userID := middleware.GetUserID(c)
	taskID := c.Param("id")

	var req services.CreateGovernanceEvidenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	governanceService := services.NewGovernanceService(db.DB())
	evidence, err := governanceService.AddEvidence(taskID, req, userID)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, evidence)
}

// CreateGovernanceComment 创建治理评论
func CreateGovernanceComment(c *gin.Context) {
	userID := middleware.GetUserID(c)
	taskID := c.Param("id")

	var req services.CreateGovernanceCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	governanceService := services.NewGovernanceService(db.DB())
	comment, err := governanceService.AddComment(taskID, req, userID)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, comment)
}

// CreateGovernanceReview 创建治理审核
func CreateGovernanceReview(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.CreateGovernanceReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	governanceService := services.NewGovernanceService(db.DB())
	review, err := governanceService.CreateReview(req, userID)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	task := loadGovernanceTask(review.TaskID)
	audience := governanceReviewAudience(review, task, userID)
	publishGovernanceReviewChanged(audience, "created", review)
	publishGovernanceTaskChanged(audience, "entered_review", task)

	types.Success(c, review)
}

// GetGovernanceReview 获取治理审核详情
func GetGovernanceReview(c *gin.Context) {
	userID := middleware.GetUserID(c)
	reviewID := c.Param("id")

	governanceService := services.NewGovernanceService(db.DB())
	review, err := governanceService.GetReview(reviewID, userID)
	if err != nil {
		types.Error(c, 404, err.Error())
		return
	}

	types.Success(c, review)
}

// ApproveGovernanceReview 审核通过
func ApproveGovernanceReview(c *gin.Context) {
	userID := middleware.GetUserID(c)
	reviewID := c.Param("id")

	var req services.ReviewDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	governanceService := services.NewGovernanceService(db.DB())
	review, err := governanceService.DecideReview(reviewID, userID, "approved", req.DecisionPayload)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	task := loadGovernanceTask(review.TaskID)
	audience := governanceReviewAudience(review, task, userID)
	publishGovernanceReviewChanged(audience, "approved", review)
	publishGovernanceTaskChanged(audience, "review_decided", task)

	types.Success(c, review)
}

// RejectGovernanceReview 审核拒绝
func RejectGovernanceReview(c *gin.Context) {
	userID := middleware.GetUserID(c)
	reviewID := c.Param("id")

	var req services.ReviewDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	governanceService := services.NewGovernanceService(db.DB())
	review, err := governanceService.DecideReview(reviewID, userID, "rejected", req.DecisionPayload)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	task := loadGovernanceTask(review.TaskID)
	audience := governanceReviewAudience(review, task, userID)
	publishGovernanceReviewChanged(audience, "rejected", review)
	publishGovernanceTaskChanged(audience, "review_decided", task)

	types.Success(c, review)
}

// ApplyGovernanceReview 执行或重试审核回写
func ApplyGovernanceReview(c *gin.Context) {
	userID := middleware.GetUserID(c)
	reviewID := c.Param("id")

	governanceService := services.NewGovernanceService(db.DB())
	outbox, err := governanceService.EnqueueReviewApply(reviewID, userID, true)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	review := loadGovernanceReview(outbox.ReviewID)
	task := loadGovernanceTask(outbox.TaskID)
	audience := governanceReviewAudience(review, task, userID)
	publishGovernanceReviewChanged(audience, "apply_requested", review)
	publishGovernanceTaskChanged(audience, "apply_requested", task)

	types.Success(c, outbox)
}
