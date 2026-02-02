package usecase

import (
	"context"
	"errors"

	"later/internal/domain/models"
	"later/internal/domain/repositories"
)

var (
	ErrTaskNotFound = errors.New("task not found")
)

// Stats represents statistics
type Stats struct {
	Total                int64                        `json:"total"`
	ByStatus            map[models.TaskStatus]int64 `json:"by_status"`
	Last24h              Last24hStats                 `json:"last_24h"`
	CallbackSuccessRate float64                      `json:"callback_success_rate"`
}

// Last24hStats represents statistics for the last 24 hours
type Last24hStats struct {
	Submitted int64 `json:"submitted"`
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
}

// TaskService handles business logic for tasks
type TaskService struct {
	repo repositories.TaskRepository
}

// NewTaskService creates a new task service
func NewTaskService(repo repositories.TaskRepository) *TaskService {
	return &TaskService{repo: repo}
}

// CreateTask creates a new task and saves it to the database
func (s *TaskService) CreateTask(ctx context.Context, task *models.Task) error {
	return s.repo.Create(ctx, task)
}

// GetTask retrieves a task by ID
func (s *TaskService) GetTask(ctx context.Context, id string) (*models.Task, error) {
	task, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// DeleteTask deletes a task by ID
func (s *TaskService) DeleteTask(ctx context.Context, id string) error {
	// For MVP, just return success
	// In production, would mark as cancelled
	return nil
}

// UpdateTask updates a task
func (s *TaskService) UpdateTask(ctx context.Context, task *models.Task) error {
	return s.repo.Update(ctx, task)
}

// List retrieves tasks with filters and pagination
func (s *TaskService) List(ctx context.Context, filter *repositories.TaskFilter) ([]*models.Task, int64, error) {
	return s.repo.List(ctx, *filter)
}

// GetStats retrieves task statistics
func (s *TaskService) GetStats(ctx context.Context) (*Stats, error) {
	byStatus, err := s.repo.CountByStatus(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate total
	total := byStatus[models.TaskStatusPending] + byStatus[models.TaskStatusProcessing] +
		byStatus[models.TaskStatusCompleted] + byStatus[models.TaskStatusFailed] +
		byStatus[models.TaskStatusDeadLettered]

	// Calculate last 24h stats
	last24h := Last24hStats{
		Submitted: 0,  // TODO: Query from database
		Completed: byStatus[models.TaskStatusCompleted],
		Failed:    byStatus[models.TaskStatusFailed],
	}

	// Calculate callback success rate
	successRate := 0.0
	totalCompletedAndFailed := byStatus[models.TaskStatusCompleted] + byStatus[models.TaskStatusFailed]
	if totalCompletedAndFailed > 0 {
		successRate = float64(byStatus[models.TaskStatusCompleted]) / float64(totalCompletedAndFailed)
	}

	return &Stats{
		Total:                total,
		ByStatus:            byStatus,
		Last24h:              last24h,
		CallbackSuccessRate: successRate,
	}, nil
}

// ProcessTask executes a task and delivers callback
func (s *TaskService) ProcessTask(ctx context.Context, task *models.Task) error {
	// TODO: Implement callback delivery
	// For now, just mark as completed
	task.MarkAsCompleted()
	return s.repo.Update(ctx, task)
}
