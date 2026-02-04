package rest

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"later/domain"
	"later/domain/entity"
	"later/delivery/rest/dto"
	"later/delivery/rest/response"
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
		response.ErrorWithMessage(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		response.ErrorWithMessage(c, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// Convert to domain model
	task := req.ToModel()

	// Save to database
	ctx := c.Request.Context()
	if err := h.taskService.CreateTask(ctx, task); err != nil {
		response.ErrorWithMessage(c, http.StatusInternalServerError, "internal_error", "Failed to create task")
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

	taskResponse := dto.TaskResponse{
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

	response.Accepted(c, taskResponse)
}

// ListTasks handles GET /api/v1/tasks
func (h *Handler) ListTasks(c *gin.Context) {
	var query dto.ListTasksQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.ErrorWithMessage(c, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}

	// Validate and normalize
	if err := query.Validate(); err != nil {
		response.ErrorWithMessage(c, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// Convert to repository filter
	filter, err := query.ToRepositoryFilter()
	if err != nil {
		response.ErrorWithMessage(c, http.StatusBadRequest, "invalid_filter", err.Error())
		return
	}

	log.Printf("[ListTasks] Fetching tasks with filter: page=%d, limit=%d, status=%v, sort=%s %s",
		query.Page, query.Limit, query.Status, query.SortBy, query.SortOrder)

	// Fetch tasks
	ctx := c.Request.Context()
	tasks, total, err := h.taskService.List(ctx, filter)
	if err != nil {
		log.Printf("[ListTasks] Failed to list tasks: %v", err)
		response.ErrorWithMessage(c, http.StatusInternalServerError, "internal_error", "Failed to list tasks")
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

	listResponse := dto.TaskListResponse{
		Tasks: taskResponses,
		Pagination: dto.PaginationInfo{
			Page:       query.Page,
			Limit:      query.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	log.Printf("[ListTasks] About to send response: %d tasks, pagination: page=%d, total=%d",
		len(listResponse.Tasks), listResponse.Pagination.Page, listResponse.Pagination.Total)
	response.Success(c, listResponse)
}

// GetTask handles GET /api/v1/tasks/:id
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
	task, err := h.taskService.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.ErrorWithMessage(c, http.StatusNotFound, "task_not_found", "Task not found")
			return
		}
		response.ErrorWithMessage(c, http.StatusInternalServerError, "internal_error", "Failed to get task")
		return
	}

	// Convert JSONBytes to json.RawMessage for response
	var payload json.RawMessage
	if len(task.Payload) > 0 {
		payload = json.RawMessage(task.Payload)
	}

	taskResponse := dto.TaskResponse{
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

	response.Success(c, taskResponse)
}

// CancelTask handles DELETE /api/v1/tasks/:id
func (h *Handler) CancelTask(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
	task, err := h.taskService.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.ErrorWithMessage(c, http.StatusNotFound, "task_not_found", "Task not found")
			return
		}
		response.ErrorWithMessage(c, http.StatusInternalServerError, "internal_error", "Failed to get task")
		return
	}

	// Validate task can be deleted (only pending or failed tasks)
	if !task.CanBeDeleted() {
		response.ErrorWithMessage(c, http.StatusBadRequest, "invalid_status", "Can only delete pending or failed tasks")
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
			response.ErrorWithMessage(c, http.StatusBadRequest, "invalid_status", err.Error())
			return
		}
		response.ErrorWithMessage(c, http.StatusInternalServerError, "internal_error", "Failed to delete task")
		return
	}

	response.NoContent(c)
}

// RetryTask handles POST /api/v1/tasks/:id/retry
func (h *Handler) RetryTask(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
	task, err := h.taskService.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.ErrorWithMessage(c, http.StatusNotFound, "task_not_found", "Task not found")
			return
		}
		response.ErrorWithMessage(c, http.StatusInternalServerError, "internal_error", "Failed to get task")
		return
	}

	// Can only retry failed tasks
	if task.Status != entity.TaskStatusFailed {
		response.ErrorWithMessage(c, http.StatusBadRequest, "invalid_status", "Can only retry failed tasks")
		return
	}

	// Reset task for retry
	task.Status = entity.TaskStatusPending
	task.RetryCount = 0
	task.NextRetryAt = nil

	if err := h.taskService.UpdateTask(ctx, task); err != nil {
		log.Printf("Failed to retry task: %v", err)
		response.ErrorWithMessage(c, http.StatusInternalServerError, "internal_error", "Failed to retry task")
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
		response.ErrorWithMessage(c, http.StatusInternalServerError, "internal_error", "Failed to get statistics")
		return
	}

	// Convert tasksvc.Last24hStats to dto.Last24hStats
	last24h := dto.Last24hStats{
		Submitted: stats.Last24h.Submitted,
		Completed: stats.Last24h.Completed,
		Failed:    stats.Last24h.Failed,
	}

	statsResponse := dto.StatsResponse{
		Total:                stats.Total,
		ByStatus:            stats.ByStatus,
		Last24h:              last24h,
		CallbackSuccessRate: stats.CallbackSuccessRate,
	}

	response.Success(c, statsResponse)
}

// ResurrectTask handles POST /api/v1/tasks/:id/resurrect
func (h *Handler) ResurrectTask(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
	task, err := h.taskService.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.ErrorWithMessage(c, http.StatusNotFound, "task_not_found", "Task not found")
			return
		}
		response.ErrorWithMessage(c, http.StatusInternalServerError, "internal_error", "Failed to get task")
		return
	}

	// Can only resurrect dead_lettered tasks
	if task.Status != entity.TaskStatusDeadLettered {
		response.ErrorWithMessage(c, http.StatusBadRequest, "invalid_status", "Can only resurrect dead_lettered tasks")
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
		response.ErrorWithMessage(c, http.StatusInternalServerError, "internal_error", "Failed to resurrect task")
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
