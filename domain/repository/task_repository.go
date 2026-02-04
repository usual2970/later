package repository

import (
	"context"
	"time"

	"github.com/usual2970/later/domain/entity"
)

// Note: pgxpool import removed - this is interface definition only

// TaskRepository defines the interface for task data operations
type TaskRepository interface {
	Create(ctx context.Context, task *entity.Task) error

	FindByID(ctx context.Context, id string) (*entity.Task, error)

	FindDueTasks(ctx context.Context, minPriority int, limit int) ([]*entity.Task, error)

	FindPendingTasks(ctx context.Context, limit int) ([]*entity.Task, error)

	FindFailedTasks(ctx context.Context, limit int) ([]*entity.Task, error)

	Update(ctx context.Context, task *entity.Task) error

	SoftDelete(ctx context.Context, taskID string, deletedBy string) error

	List(ctx context.Context, filter TaskFilter) ([]*entity.Task, int64, error)

	CountByStatus(ctx context.Context) (map[entity.TaskStatus]int64, error)

	CleanupExpiredData(ctx context.Context) (int64, error)
}

// TaskFilter defines filtering options for listing tasks
type TaskFilter struct {
	Status    *entity.TaskStatus
	Priority  *int
	Tags      []string
	DateFrom  *time.Time
	DateTo    *time.Time
	Page      int
	Limit     int
	SortBy    string // "created_at", "scheduled_at", "priority"
	SortOrder string // "asc", "desc"
}
