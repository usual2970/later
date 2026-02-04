package later

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"later/domain/entity"
	"later/infrastructure/logger"
)

// RegisterRoutes registers Later's HTTP routes with the provided Gin engine
// The routes will be mounted under the configured RoutePrefix
func (l *Later) RegisterRoutes(engine *gin.Engine) error {
	if engine == nil {
		return fmt.Errorf("engine cannot be nil")
	}

	// Create route group with prefix
	group := engine.Group(l.config.RoutePrefix)

	// Apply Later's middleware
	group.Use(l.loggerMiddleware())
	group.Use(l.recoveryMiddleware())

	// Health check endpoint
	group.GET("/health", l.healthCheckHandler)

	// Task routes
	tasks := group.Group("/tasks")
	{
		tasks.POST("", l.createTaskHandler)
		tasks.GET("", l.listTasksHandler)
		tasks.GET("/:id", l.getTaskHandler)
		tasks.DELETE("/:id", l.deleteTaskHandler)
		tasks.POST("/:id/retry", l.retryTaskHandler)
		tasks.POST("/:id/resurrect", l.resurrectTaskHandler)
		tasks.GET("/stats", l.getStatsHandler)
	}

	l.logger.Info("Routes registered successfully",
		zap.String("prefix", l.config.RoutePrefix),
		zap.Int("endpoints", 7),
	)

	return nil
}

// loggerMiddleware logs HTTP requests
func (l *Later) loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log request details
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		logger.Info("HTTP request",
			logger.String("method", method),
			logger.String("path", path),
			logger.String("query", query),
			logger.Int("status", statusCode),
			logger.String("latency", latency.String()),
			logger.String("client_ip", clientIP),
			logger.Int("response_size", c.Writer.Size()),
		)

		// Log errors if any
		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				logger.Error("HTTP request error",
					logger.String("type", e.Error()),
					logger.String("error", e.Error()),
				)
			}
		}
	}
}

// recoveryMiddleware recovers from panics
func (l *Later) recoveryMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			logger.Error("Panic recovered",
				logger.String("error", err),
				logger.String("path", c.Request.URL.Path),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal_error",
				"message": "Internal server error",
			})
		}
	})
}

// healthCheckHandler returns the health status of Later
func (l *Later) healthCheckHandler(c *gin.Context) {
	status := l.HealthCheck()

	httpStatus := http.StatusOK
	if status.Status == "unhealthy" {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, status)
}

// createTaskHandler handles POST /tasks
func (l *Later) createTaskHandler(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	// Validate request
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "name is required",
		})
		return
	}

	if req.CallbackURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "callback_url is required",
		})
		return
	}

	// Set defaults
	if req.ScheduledAt.IsZero() {
		req.ScheduledAt = time.Now()
	}

	if req.MaxRetries == 0 {
		req.MaxRetries = 5
	}

	// Create task
	task, err := l.CreateTask(c.Request.Context(), &req)
	if err != nil {
		logger.Error("Failed to create task",
			logger.String("handler", "createTaskHandler"),
			logger.Any("error", err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Failed to create task",
		})
		return
	}

	// Build response
	estimatedExec := "scheduled"
	if task.ShouldExecuteNow() {
		estimatedExec = "immediate"
	}

	// Convert JSONBytes to string for JSON response
	var payloadStr string
	if len(task.Payload) > 0 {
		payloadStr = string(task.Payload)
	}

	c.JSON(http.StatusAccepted, gin.H{
		"id":                 task.ID,
		"name":               task.Name,
		"payload":            payloadStr,
		"callback_url":       task.CallbackURL,
		"status":             task.Status,
		"created_at":         task.CreatedAt,
		"scheduled_for":      task.ScheduledAt,
		"max_retries":         task.MaxRetries,
		"retry_count":         task.RetryCount,
		"callback_attempts":   task.CallbackAttempts,
		"priority":           task.Priority,
		"tags":               task.Tags,
		"estimated_execution": estimatedExec,
	})
}

// getTaskHandler handles GET /tasks/:id
func (l *Later) getTaskHandler(c *gin.Context) {
	id := c.Param("id")

	task, err := l.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "task_not_found",
			"message": "Task not found",
		})
		return
	}

	// Convert JSONBytes to string for JSON response
	var payloadStr string
	if len(task.Payload) > 0 {
		payloadStr = string(task.Payload)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":               task.ID,
		"name":             task.Name,
		"payload":          payloadStr,
		"callback_url":      task.CallbackURL,
		"status":           task.Status,
		"created_at":        task.CreatedAt,
		"scheduled_for":     task.ScheduledAt,
		"started_at":        task.StartedAt,
		"completed_at":      task.CompletedAt,
		"max_retries":       task.MaxRetries,
		"retry_count":       task.RetryCount,
		"callback_attempts": task.CallbackAttempts,
		"priority":         task.Priority,
		"tags":             task.Tags,
		"error_message":     task.ErrorMessage,
	})
}

// listTasksHandler handles GET /tasks
func (l *Later) listTasksHandler(c *gin.Context) {
	// Parse query parameters
	var filter TaskFilter

	// Set defaults
	filter.Page = 1
	filter.Limit = 10
	filter.SortBy = "created_at"
	filter.SortOrder = "DESC"

	// Parse pagination
	if page := c.Query("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil && p > 0 {
			filter.Page = p
		}
	}

	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 && l <= 100 {
			filter.Limit = l
		}
	}

	// Parse filters
	if status := c.Query("status"); status != "" {
		filter.Status = status
	}

	if sortBy := c.Query("sort_by"); sortBy != "" {
		filter.SortBy = sortBy
	}

	if sortOrder := c.Query("sort_order"); sortOrder != "" {
		filter.SortOrder = sortOrder
	}

	logger.Info("Listing tasks",
		logger.String("handler", "listTasksHandler"),
		logger.Int("page", filter.Page),
		logger.Int("limit", filter.Limit),
		logger.String("status", filter.Status),
		logger.String("sort_by", filter.SortBy),
		logger.String("sort_order", filter.SortOrder),
	)

	// Fetch tasks
	tasks, total, err := l.ListTasks(c.Request.Context(), &filter)
	if err != nil {
		logger.Error("Failed to list tasks",
			logger.String("handler", "listTasksHandler"),
			logger.Any("error", err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Failed to list tasks",
		})
		return
	}

	logger.Info("Successfully fetched tasks",
		logger.String("handler", "listTasksHandler"),
		logger.Int("count", len(tasks)),
		logger.Int64("total", total),
	)

	// Convert to response format
	taskResponses := make([]gin.H, len(tasks))
	for i, task := range tasks {
		// Convert JSONBytes to string
		var payloadStr string
		if len(task.Payload) > 0 {
			payloadStr = string(task.Payload)
		}

		taskResponses[i] = gin.H{
			"id":                 task.ID,
			"name":               task.Name,
			"payload":            payloadStr,
			"callback_url":       task.CallbackURL,
			"status":             task.Status,
			"created_at":         task.CreatedAt,
			"scheduled_for":      task.ScheduledAt,
			"started_at":         task.StartedAt,
			"completed_at":       task.CompletedAt,
			"max_retries":        task.MaxRetries,
			"retry_count":        task.RetryCount,
			"callback_attempts":  task.CallbackAttempts,
			"priority":           task.Priority,
			"tags":               task.Tags,
			"error_message":      task.ErrorMessage,
		}
	}

	// Calculate pagination
	totalPages := int(total) / filter.Limit
	if int(total)%filter.Limit != 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": taskResponses,
		"pagination": gin.H{
			"page":        filter.Page,
			"limit":       filter.Limit,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// deleteTaskHandler handles DELETE /tasks/:id
func (l *Later) deleteTaskHandler(c *gin.Context) {
	id := c.Param("id")
	deletedBy := "system"

	// Try to get user from header if available
	if userID := c.GetHeader("X-User-ID"); userID != "" {
		deletedBy = userID
	}

	// Get task first to validate
	task, err := l.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "task_not_found",
			"message": "Task not found",
		})
		return
	}

	// Validate task can be deleted
	if !task.CanBeDeleted() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_status",
			"message": "Can only delete pending or failed tasks",
		})
		return
	}

	// Perform soft delete
	if err := l.DeleteTask(c.Request.Context(), id, deletedBy); err != nil {
		logger.Error("Failed to delete task",
			logger.String("handler", "deleteTaskHandler"),
			logger.String("task_id", id),
			logger.Any("error", err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Failed to delete task",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// retryTaskHandler handles POST /tasks/:id/retry
func (l *Later) retryTaskHandler(c *gin.Context) {
	id := c.Param("id")

	// Get task
	task, err := l.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "task_not_found",
			"message": "Task not found",
		})
		return
	}

	// Can only retry failed tasks
	if task.Status != entity.TaskStatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_status",
			"message": "Can only retry failed tasks",
		})
		return
	}

	// Retry task
	retriedTask, err := l.RetryTask(c.Request.Context(), id)
	if err != nil {
		logger.Error("Failed to retry task",
			logger.String("handler", "retryTaskHandler"),
			logger.String("task_id", id),
			logger.Any("error", err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Failed to retry task",
		})
		return
	}

	// Convert JSONBytes to string
	var payloadStr string
	if len(retriedTask.Payload) > 0 {
		payloadStr = string(retriedTask.Payload)
	}

	c.JSON(http.StatusAccepted, gin.H{
		"id":                 retriedTask.ID,
		"name":               retriedTask.Name,
		"payload":            payloadStr,
		"callback_url":       retriedTask.CallbackURL,
		"status":             retriedTask.Status,
		"created_at":         retriedTask.CreatedAt,
		"scheduled_for":      retriedTask.ScheduledAt,
		"max_retries":        retriedTask.MaxRetries,
		"retry_count":        retriedTask.RetryCount,
		"callback_attempts":  retriedTask.CallbackAttempts,
		"priority":           retriedTask.Priority,
		"tags":               retriedTask.Tags,
		"estimated_execution": "immediate",
	})
}

// resurrectTaskHandler handles POST /tasks/:id/resurrect
func (l *Later) resurrectTaskHandler(c *gin.Context) {
	id := c.Param("id")

	// Get task
	task, err := l.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "task_not_found",
			"message": "Task not found",
		})
		return
	}

	// Can only resurrect dead_lettered tasks
	if task.Status != entity.TaskStatusDeadLettered {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_status",
			"message": "Can only resurrect dead_lettered tasks",
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

	if err := l.taskService.UpdateTask(c.Request.Context(), task); err != nil {
		logger.Error("Failed to resurrect task",
			logger.String("handler", "resurrectTaskHandler"),
			logger.String("task_id", id),
			logger.Any("error", err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Failed to resurrect task",
		})
		return
	}

	logger.Info("Task resurrected",
		logger.String("handler", "resurrectTaskHandler"),
		logger.String("task_id", id),
	)

	// Submit immediately if due now
	if task.ShouldExecuteNow() {
		l.scheduler.SubmitTaskImmediately(task)
	}

	// Convert JSONBytes to string
	var payloadStr string
	if len(task.Payload) > 0 {
		payloadStr = string(task.Payload)
	}

	c.JSON(http.StatusAccepted, gin.H{
		"id":                 task.ID,
		"name":               task.Name,
		"payload":            payloadStr,
		"callback_url":       task.CallbackURL,
		"status":             task.Status,
		"created_at":         task.CreatedAt,
		"scheduled_for":      task.ScheduledAt,
		"max_retries":        task.MaxRetries,
		"retry_count":        task.RetryCount,
		"callback_attempts":  task.CallbackAttempts,
		"priority":           task.Priority,
		"tags":               task.Tags,
		"estimated_execution": "immediate",
	})
}

// getStatsHandler handles GET /tasks/stats
func (l *Later) getStatsHandler(c *gin.Context) {
	stats, err := l.GetStats(c.Request.Context())
	if err != nil {
		logger.Error("Failed to get stats",
			logger.String("handler", "getStatsHandler"),
			logger.Any("error", err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Failed to get statistics",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":                  stats.Total,
		"by_status":              stats.ByStatus,
		"last_24h":                stats.Last24h,
		"callback_success_rate":  stats.CallbackSuccessRate,
	})
}
