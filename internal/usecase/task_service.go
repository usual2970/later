package usecase

import (
	"context"
	"later/internal/domain/repositories"
	"later/internal/domain/models"
)

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
	return s.repo.FindByID(ctx, id)
}

// ProcessTask executes a task and delivers callback
func (s *TaskService) ProcessTask(ctx context.Context, task *models.Task) error {
	// TODO: Implement task execution and callback delivery
	// For now, just mark as completed
	task.MarkAsCompleted()
	return s.repo.Update(ctx, task)
}
