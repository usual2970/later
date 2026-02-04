package task

import (
	"context"
	"errors"

	"later/domain/entity"
	"later/domain/repository"
	"later/domain"
)

// Stats represents statistics
type Stats struct {
	Total                int64                        `json:"total"`
	ByStatus            map[entity.TaskStatus]int64 `json:"by_status"`
	Last24h              Last24hStats                 `json:"last_24h"`
	CallbackSuccessRate float64                      `json:"callback_success_rate"`
}

// Last24hStats represents statistics for the last 24 hours
type Last24hStats struct {
	Submitted int64 `json:"submitted"`
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
}

// Service handles business logic for tasks
type Service struct {
	repo repository.TaskRepository
}

// NewService creates a new task service
func NewService(repo repository.TaskRepository) *Service {
	return &Service{repo: repo}
}

// CreateTask creates a new task and saves it to the database
func (s *Service) CreateTask(ctx context.Context, task *entity.Task) error {
	return s.repo.Create(ctx, task)
}

// GetTask retrieves a task by ID
func (s *Service) GetTask(ctx context.Context, id string) (*entity.Task, error) {
	task, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	return task, nil
}

// DeleteTask soft deletes a task by ID
// Only pending and failed tasks can be deleted
func (s *Service) DeleteTask(ctx context.Context, id string, deletedBy string) error {
	// Get the task first
	task, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.ErrNotFound
	}

	// Validate task can be deleted
	if !task.CanBeDeleted() {
		return errors.New("task cannot be deleted: invalid status or already deleted")
	}

	// Perform soft delete
	return s.repo.SoftDelete(ctx, id, deletedBy)
}

// UpdateTask updates a task
func (s *Service) UpdateTask(ctx context.Context, task *entity.Task) error {
	return s.repo.Update(ctx, task)
}

// List retrieves tasks with filters and pagination
func (s *Service) List(ctx context.Context, filter *repository.TaskFilter) ([]*entity.Task, int64, error) {
	return s.repo.List(ctx, *filter)
}

// GetStats retrieves task statistics
func (s *Service) GetStats(ctx context.Context) (*Stats, error) {
	byStatus, err := s.repo.CountByStatus(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate total
	total := byStatus[entity.TaskStatusPending] + byStatus[entity.TaskStatusProcessing] +
		byStatus[entity.TaskStatusCompleted] + byStatus[entity.TaskStatusFailed] +
		byStatus[entity.TaskStatusDeadLettered]

	// Calculate last 24h stats
	last24h := Last24hStats{
		Submitted: 0,  // TODO: Query from database
		Completed: byStatus[entity.TaskStatusCompleted],
		Failed:    byStatus[entity.TaskStatusFailed],
	}

	// Calculate callback success rate
	successRate := 0.0
	totalCompletedAndFailed := byStatus[entity.TaskStatusCompleted] + byStatus[entity.TaskStatusFailed]
	if totalCompletedAndFailed > 0 {
		successRate = float64(byStatus[entity.TaskStatusCompleted]) / float64(totalCompletedAndFailed)
	}

	return &Stats{
		Total:                total,
		ByStatus:            byStatus,
		Last24h:              last24h,
		CallbackSuccessRate: successRate,
	}, nil
}

// ProcessTask executes a task and delivers callback
func (s *Service) ProcessTask(ctx context.Context, task *entity.Task) error {
	// TODO: Implement callback delivery
	// For now, just mark as completed
	task.MarkAsCompleted()
	return s.repo.Update(ctx, task)
}
