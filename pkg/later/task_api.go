package later

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"later/domain/entity"
	"later/domain/repository"
	tasksvc "later/task"
)

// CreateTask creates a new task
func (l *Later) CreateTask(ctx context.Context, req *CreateTaskRequest) (*entity.Task, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	task := &entity.Task{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Payload:      entity.JSONBytes(req.Payload),
		CallbackURL:  req.CallbackURL,
		ScheduledAt:  req.ScheduledAt,
		Priority:     req.Priority,
		MaxRetries:   req.MaxRetries,
		Tags:         req.Tags,
		Status:       entity.TaskStatusPending,
	}

	if err := l.taskService.CreateTask(ctx, task); err != nil {
		l.logger.Error("Failed to create task",
			zap.String("task_name", req.Name),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	l.logger.Info("Task created",
		zap.String("task_id", task.ID),
		zap.String("task_name", task.Name),
		zap.Time("scheduled_at", task.ScheduledAt),
	)

	// Submit immediately if due now
	if task.ShouldExecuteNow() {
		l.scheduler.SubmitTaskImmediately(task)
		l.logger.Debug("Task submitted for immediate execution",
			zap.String("task_id", task.ID),
		)
	}

	return task, nil
}

// GetTask retrieves a task by ID
func (l *Later) GetTask(ctx context.Context, id string) (*entity.Task, error) {
	if id == "" {
		return nil, fmt.Errorf("task ID cannot be empty")
	}

	task, err := l.taskService.GetTask(ctx, id)
	if err != nil {
		l.logger.Error("Failed to get task",
			zap.String("task_id", id),
			zap.Error(err),
		)
		return nil, err
	}

	return task, nil
}

// ListTasks lists tasks with pagination and filters
func (l *Later) ListTasks(ctx context.Context, filter *TaskFilter) ([]*entity.Task, int64, error) {
	if filter == nil {
		filter = &TaskFilter{
			Page:      1,
			Limit:     10,
			SortBy:    "created_at",
			SortOrder: "DESC",
		}
	}

	// Convert TaskFilter to repository.TaskFilter
	repoFilter := filter.toRepositoryFilter()

	tasks, total, err := l.taskService.List(ctx, &repoFilter)
	if err != nil {
		l.logger.Error("Failed to list tasks",
			zap.Error(err),
		)
		return nil, 0, err
	}

	return tasks, total, nil
}

// DeleteTask soft-deletes a task
func (l *Later) DeleteTask(ctx context.Context, id, deletedBy string) error {
	if id == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	if err := l.taskService.DeleteTask(ctx, id, deletedBy); err != nil {
		l.logger.Error("Failed to delete task",
			zap.String("task_id", id),
			zap.String("deleted_by", deletedBy),
			zap.Error(err),
		)
		return err
	}

	l.logger.Info("Task deleted",
		zap.String("task_id", id),
		zap.String("deleted_by", deletedBy),
	)

	return nil
}

// RetryTask resets a failed task for retry
func (l *Later) RetryTask(ctx context.Context, id string) (*entity.Task, error) {
	if id == "" {
		return nil, fmt.Errorf("task ID cannot be empty")
	}

	task, err := l.taskService.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}

	if task.Status != entity.TaskStatusFailed {
		return nil, fmt.Errorf("can only retry failed tasks, current status: %s", task.Status)
	}

	task.Status = entity.TaskStatusPending
	task.RetryCount = 0
	task.NextRetryAt = nil

	if err := l.taskService.UpdateTask(ctx, task); err != nil {
		l.logger.Error("Failed to retry task",
			zap.String("task_id", id),
			zap.Error(err),
		)
		return nil, err
	}

	l.logger.Info("Task retried",
		zap.String("task_id", id),
	)

	// Submit immediately if due now
	if task.ShouldExecuteNow() {
		l.scheduler.SubmitTaskImmediately(task)
	}

	return task, nil
}

// GetStats returns task statistics
func (l *Later) GetStats(ctx context.Context) (*tasksvc.Stats, error) {
	stats, err := l.taskService.GetStats(ctx)
	if err != nil {
		l.logger.Error("Failed to get stats",
			zap.Error(err),
		)
		return nil, err
	}

	return stats, nil
}

// GetMetrics returns real-time metrics
// Note: This is a simplified version using available APIs
// In the future, we can add more detailed metrics
func (l *Later) GetMetrics() Metrics {
	health := l.HealthCheck()

	metrics := Metrics{
		QueueDepth:      0, // Not directly available from current API
		ActiveWorkers:   0,
		CallbackSuccessRate: 0.0,
	}

	if health.Workers != nil {
		metrics.ActiveWorkers = health.Workers.Active
	}

	// Try to get stats for success rate
	stats, err := l.GetStats(context.Background())
	if err == nil && stats != nil {
		metrics.CallbackSuccessRate = stats.CallbackSuccessRate
		metrics.QueueDepth = stats.ByStatus[entity.TaskStatusPending]
	}

	return metrics
}

// CreateTaskRequest represents a request to create a task
type CreateTaskRequest struct {
	Name         string            `json:"name"`
	Payload      []byte            `json:"payload"`
	CallbackURL  string            `json:"callback_url"`
	ScheduledAt  time.Time         `json:"scheduled_at"`
	Priority     int               `json:"priority"`
	MaxRetries   int               `json:"max_retries"`
	Tags         []string          `json:"tags"`
}

// TaskFilter represents filters for listing tasks
type TaskFilter struct {
	Status       string     `json:"status"`
	Priority     *int       `json:"priority"`
	CreatedAfter *time.Time `json:"created_after"`
	CreatedBefore *time.Time `json:"created_before"`
	Page         int        `json:"page"`
	Limit        int        `json:"limit"`
	SortBy       string     `json:"sort_by"`
	SortOrder    string     `json:"sort_order"`
}

// toRepositoryFilter converts TaskFilter to repository.TaskFilter
func (f *TaskFilter) toRepositoryFilter() repository.TaskFilter {
	repoFilter := repository.TaskFilter{
		Page:      f.Page,
		Limit:     f.Limit,
		SortBy:    f.SortBy,
		SortOrder: f.SortOrder,
	}

	// Convert status string to TaskStatus pointer
	if f.Status != "" {
		status := entity.TaskStatus(f.Status)
		repoFilter.Status = &status
	}

	// Set priority
	repoFilter.Priority = f.Priority

	// Set date filters (map created_after/before to date_from/date_to)
	repoFilter.DateFrom = f.CreatedAfter
	repoFilter.DateTo = f.CreatedBefore

	return repoFilter
}

// Metrics represents real-time metrics about the task system
type Metrics struct {
	QueueDepth          int64   `json:"queue_depth"`
	ActiveWorkers       int     `json:"active_workers"`
	CallbackSuccessRate float64 `json:"callback_success_rate"`
}
