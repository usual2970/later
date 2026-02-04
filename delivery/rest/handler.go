package rest

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"later/domain"
	"later/domain/entity"
	"later/delivery/rest/dto"
	tasksvc "later/task"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests
type Handler struct {
	taskService *tasksvc.Service
	scheduler   *tasksvc.Scheduler
}

// NewHandler creates a new HTTP handler
func NewHandler(taskService *tasksvc.Service, scheduler *tasksvc.Scheduler) *Handler {
	return &Handler{
		taskService: taskService,
		scheduler:   scheduler,
	}
}

// CreateTask handles POST /api/v1/tasks
func (h *Handler) CreateTask(c *gin.Context) {
	var req dto.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "validation_error",
			Message: err.Error(),
		})
		return
	}

	// Convert to domain model
	task := req.ToModel()

	// Save to database
	ctx := c.Request.Context()
	if err := h.taskService.CreateTask(ctx, task); err != nil {
		log.Printf("Failed to create task: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal_error",
			Message: "Failed to create task",
		})
		return
	}

	// If immediate execution, submit directly to worker pool
	if task.ShouldExecuteNow() {
		h.scheduler.SubmitTaskImmediately(task)
	}

	// Build response
	estimatedExec := "scheduled"
	if task.ShouldExecuteNow() {
		estimatedExec = "immediate"
	}

	// Convert JSONBytes to json.RawMessage for response
	var payload json.RawMessage
	if len(task.Payload) > 0 {
		payload = json.RawMessage(task.Payload)
	}

	response := dto.TaskResponse{
		ID:                task.ID,
		Name:              task.Name,
		Payload:           payload,
		CallbackURL:       task.CallbackURL,
		Status:            task.Status,
		CreatedAt:         task.CreatedAt,
		ScheduledFor:      task.ScheduledAt,
		MaxRetries:       task.MaxRetries,
		RetryCount:       task.RetryCount,
		CallbackAttempts: task.CallbackAttempts,
		Priority:         task.Priority,
		Tags:             task.Tags,
		EstimatedExecution: estimatedExec,
	}

	c.JSON(http.StatusAccepted, response)
}

// ListTasks handles GET /api/v1/tasks
func (h *Handler) ListTasks(c *gin.Context) {
	var query dto.ListTasksQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid_query",
			Message: err.Error(),
		})
		return
	}

	// Validate and normalize
	if err := query.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "validation_error",
			Message: err.Error(),
		})
		return
	}

	// Convert to repository filter
	filter, err := query.ToRepositoryFilter()
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid_filter",
			Message: err.Error(),
		})
		return
	}

	log.Printf("[ListTasks] Fetching tasks with filter: page=%d, limit=%d, status=%v, sort=%s %s",
		query.Page, query.Limit, query.Status, query.SortBy, query.SortOrder)

	// Fetch tasks
	ctx := c.Request.Context()
	tasks, total, err := h.taskService.List(ctx, filter)
	if err != nil {
		log.Printf("[ListTasks] Failed to list tasks: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal_error",
			Message: "Failed to list tasks",
		})
		return
	}

	log.Printf("[ListTasks] Successfully fetched %d tasks (total: %d)", len(tasks), total)

	// Convert to response format
	taskResponses := make([]*dto.TaskResponse, len(tasks))
	for i, task := range tasks {
		// Convert JSONBytes to json.RawMessage for response
		var payload json.RawMessage
		if len(task.Payload) > 0 {
			payload = json.RawMessage(task.Payload)
		}

		taskResponses[i] = &dto.TaskResponse{
			ID:                task.ID,
			Name:              task.Name,
			Payload:           payload,
			CallbackURL:       task.CallbackURL,
			Status:            task.Status,
			CreatedAt:         task.CreatedAt,
			ScheduledFor:      task.ScheduledAt,
			StartedAt:         task.StartedAt,
			CompletedAt:       task.CompletedAt,
			MaxRetries:       task.MaxRetries,
			RetryCount:       task.RetryCount,
			CallbackAttempts: task.CallbackAttempts,
			Priority:         task.Priority,
			Tags:             task.Tags,
			ErrorMessage:     task.ErrorMessage,
		}
	}

	// Calculate pagination
	totalPages := int(total) / query.Limit
	if int(total)%query.Limit != 0 {
		totalPages++
	}

	response := dto.TaskListResponse{
		Tasks: taskResponses,
		Pagination: dto.PaginationInfo{
			Page:       query.Page,
			Limit:      query.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	c.JSON(http.StatusOK, response)
}

// GetTask handles GET /api/v1/tasks/:id
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
	task, err := h.taskService.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{
				Error: "task_not_found",
				Message: "Task not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal_error",
			Message: "Failed to get task",
		})
		return
	}

	// Convert JSONBytes to json.RawMessage for response
	var payload json.RawMessage
	if len(task.Payload) > 0 {
		payload = json.RawMessage(task.Payload)
	}

	response := dto.TaskResponse{
		ID:                task.ID,
		Name:              task.Name,
		Payload:           payload,
		CallbackURL:       task.CallbackURL,
		Status:            task.Status,
		CreatedAt:         task.CreatedAt,
		ScheduledFor:      task.ScheduledAt,
		StartedAt:         task.StartedAt,
		CompletedAt:       task.CompletedAt,
		MaxRetries:       task.MaxRetries,
		RetryCount:       task.RetryCount,
		CallbackAttempts: task.CallbackAttempts,
		Priority:         task.Priority,
		Tags:             task.Tags,
		ErrorMessage:     task.ErrorMessage,
	}

	c.JSON(http.StatusOK, response)
}

// CancelTask handles DELETE /api/v1/tasks/:id
func (h *Handler) CancelTask(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
	task, err := h.taskService.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{
				Error: "task_not_found",
				Message: "Task not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal_error",
			Message: "Failed to get task",
		})
		return
	}

	// Validate task can be deleted (only pending or failed tasks)
	if !task.CanBeDeleted() {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid_status",
			Message: "Can only delete pending or failed tasks",
		})
		return
	}

	// Get deleted_by from context or use a default identifier
	// In a real application, this would come from authentication
	deletedBy := "system"
	if userID := c.GetHeader("X-User-ID"); userID != "" {
		deletedBy = userID
	}

	// Perform soft delete
	if err := h.taskService.DeleteTask(ctx, id, deletedBy); err != nil {
		log.Printf("Failed to delete task: %v", err)
		if err.Error() == "task cannot be deleted: invalid status or already deleted" {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error: "invalid_status",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal_error",
			Message: "Failed to delete task",
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// RetryTask handles POST /api/v1/tasks/:id/retry
func (h *Handler) RetryTask(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
	task, err := h.taskService.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{
				Error: "task_not_found",
				Message: "Task not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal_error",
			Message: "Failed to get task",
		})
		return
	}

	// Can only retry failed tasks
	if task.Status != entity.TaskStatusFailed {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid_status",
			Message: "Can only retry failed tasks",
		})
		return
	}

	// Reset task for retry
	task.Status = entity.TaskStatusPending
	task.RetryCount = 0
	task.NextRetryAt = nil

	if err := h.taskService.UpdateTask(ctx, task); err != nil {
		log.Printf("Failed to retry task: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal_error",
			Message: "Failed to retry task",
		})
		return
	}

	// If immediate execution, submit to worker pool
	if task.ShouldExecuteNow() {
		h.scheduler.SubmitTaskImmediately(task)
	}

	// Convert JSONBytes to json.RawMessage for response
	var payload json.RawMessage
	if len(task.Payload) > 0 {
		payload = json.RawMessage(task.Payload)
	}

	response := dto.TaskResponse{
		ID:                task.ID,
		Name:              task.Name,
		Payload:           payload,
		CallbackURL:       task.CallbackURL,
		Status:            task.Status,
		CreatedAt:         task.CreatedAt,
		ScheduledFor:      task.ScheduledAt,
		MaxRetries:       task.MaxRetries,
		RetryCount:       task.RetryCount,
		CallbackAttempts: task.CallbackAttempts,
		Priority:         task.Priority,
		Tags:             task.Tags,
		EstimatedExecution: "immediate",
	}

	c.JSON(http.StatusAccepted, response)
}

// GetStats handles GET /api/v1/tasks/stats
func (h *Handler) GetStats(c *gin.Context) {
	ctx := c.Request.Context()

	stats, err := h.taskService.GetStats(ctx)
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal_error",
			Message: "Failed to get statistics",
		})
		return
	}

	// Convert tasksvc.Last24hStats to dto.Last24hStats
	last24h := dto.Last24hStats{
		Submitted: stats.Last24h.Submitted,
		Completed: stats.Last24h.Completed,
		Failed:    stats.Last24h.Failed,
	}

	response := dto.StatsResponse{
		Total:                stats.Total,
		ByStatus:            stats.ByStatus,
		Last24h:              last24h,
		CallbackSuccessRate: stats.CallbackSuccessRate,
	}

	c.JSON(http.StatusOK, response)
}

// ResurrectTask handles POST /api/v1/tasks/:id/resurrect
func (h *Handler) ResurrectTask(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
	task, err := h.taskService.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{
				Error: "task_not_found",
				Message: "Task not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal_error",
			Message: "Failed to get task",
		})
		return
	}

	// Can only resurrect dead_lettered tasks
	if task.Status != entity.TaskStatusDeadLettered {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid_status",
			Message: "Can only resurrect dead_lettered tasks",
		})
		return
	}

	// Reset task for resurrection
	task.Status = entity.TaskStatusPending
	task.RetryCount = 0
	task.NextRetryAt = nil
	task.ErrorMessage = nil
	task.StartedAt = nil
	task.CompletedAt = nil

	if err := h.taskService.UpdateTask(ctx, task); err != nil {
		log.Printf("Failed to resurrect task: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal_error",
			Message: "Failed to resurrect task",
		})
		return
	}

	// If immediate execution, submit to worker pool
	if task.ShouldExecuteNow() {
		h.scheduler.SubmitTaskImmediately(task)
	}

	// Convert JSONBytes to json.RawMessage for response
	var payload json.RawMessage
	if len(task.Payload) > 0 {
		payload = json.RawMessage(task.Payload)
	}

	response := dto.TaskResponse{
		ID:                task.ID,
		Name:              task.Name,
		Payload:           payload,
		CallbackURL:       task.CallbackURL,
		Status:            task.Status,
		CreatedAt:         task.CreatedAt,
		ScheduledFor:      task.ScheduledAt,
		MaxRetries:       task.MaxRetries,
		RetryCount:       task.RetryCount,
		CallbackAttempts: task.CallbackAttempts,
		Priority:         task.Priority,
		Tags:             task.Tags,
		EstimatedExecution: "immediate",
	}

	c.JSON(http.StatusAccepted, response)
}

// getStatusCode maps domain errors to HTTP status codes
func getStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, domain.ErrBadParamInput):
		return http.StatusBadRequest
	case errors.Is(err, domain.ErrTaskCannotDelete):
		return http.StatusBadRequest
	case errors.Is(err, domain.ErrTaskCannotRetry):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

