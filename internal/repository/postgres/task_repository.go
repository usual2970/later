package postgres

import (
	"context"
	"fmt"

	"later/internal/domain/models"
	"later/internal/domain/repositories"

	"github.com/jackc/pgx/v5/pgxpool"
)

// taskRepository implements repositories.TaskRepository
type taskRepository struct {
	db *pgxpool.Pool
}

// NewTaskRepository creates a new PostgreSQL task repository
func NewTaskRepository(db *pgxpool.Pool) repositories.TaskRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) Create(ctx context.Context, task *models.Task) error {
	query := `
		INSERT INTO task_queue (
			id, name, payload, callback_url, status,
			created_at, scheduled_at, max_retries, retry_count,
			retry_backoff_seconds, callback_timeout_seconds, priority, tags
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err := r.db.Exec(ctx, query,
		task.ID, task.Name, task.Payload, task.CallbackURL, task.Status,
		task.CreatedAt, task.ScheduledAt, task.MaxRetries, task.RetryCount,
		task.RetryBackoffSeconds, task.CallbackTimeoutSecs, task.Priority, task.Tags,
	)

	return err
}

func (r *taskRepository) FindByID(ctx context.Context, id string) (*models.Task, error) {
	query := `
		SELECT id, name, payload, callback_url, status,
			   created_at, scheduled_at, started_at, completed_at,
			   max_retries, retry_count, retry_backoff_seconds, next_retry_at,
			   callback_attempts, callback_timeout_seconds, last_callback_at,
			   last_callback_status, last_callback_error, priority, tags, error_message
		FROM task_queue
		WHERE id = $1
	`

	var task models.Task
	err := r.db.QueryRow(ctx, query, id).Scan(
		&task.ID, &task.Name, &task.Payload, &task.CallbackURL, &task.Status,
		&task.CreatedAt, &task.ScheduledAt, &task.StartedAt, &task.CompletedAt,
		&task.MaxRetries, &task.RetryCount, &task.RetryBackoffSeconds, &task.NextRetryAt,
		&task.CallbackAttempts, &task.CallbackTimeoutSecs, &task.LastCallbackAt,
		&task.LastCallbackStatus, &task.LastCallbackError, &task.Priority, &task.Tags, &task.ErrorMessage,
	)

	if err != nil {
		return nil, err
	}

	return &task, nil
}

func (r *taskRepository) FindDueTasks(ctx context.Context, minPriority int, limit int) ([]*models.Task, error) {
	query := `
		SELECT id, name, payload, callback_url, status,
			   created_at, scheduled_at, started_at, completed_at,
			   max_retries, retry_count, retry_backoff_seconds, next_retry_at,
			   callback_attempts, callback_timeout_seconds, last_callback_at,
			   last_callback_status, last_callback_error, priority, tags, error_message
		FROM task_queue
		WHERE status = 'pending'
		  AND scheduled_at <= NOW()
		  AND ($1 = -1 OR priority > $1)
		ORDER BY priority DESC, scheduled_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`

	rows, err := r.db.Query(ctx, query, minPriority, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		err := rows.Scan(
			&task.ID, &task.Name, &task.Payload, &task.CallbackURL, &task.Status,
			&task.CreatedAt, &task.ScheduledAt, &task.StartedAt, &task.CompletedAt,
			&task.MaxRetries, &task.RetryCount, &task.RetryBackoffSeconds, &task.NextRetryAt,
			&task.CallbackAttempts, &task.CallbackTimeoutSecs, &task.LastCallbackAt,
			&task.LastCallbackStatus, &task.LastCallbackError, &task.Priority, &task.Tags, &task.ErrorMessage,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &task)
	}

	return tasks, rows.Err()
}

func (r *taskRepository) FindPendingTasks(ctx context.Context, limit int) ([]*models.Task, error) {
	return r.FindDueTasks(ctx, -1, limit)
}

func (r *taskRepository) FindFailedTasks(ctx context.Context, limit int) ([]*models.Task, error) {
	query := `
		SELECT id, name, payload, callback_url, status,
			   created_at, scheduled_at, started_at, completed_at,
			   max_retries, retry_count, retry_backoff_seconds, next_retry_at,
			   callback_attempts, callback_timeout_seconds, last_callback_at,
			   last_callback_status, last_callback_error, priority, tags, error_message
		FROM task_queue
		WHERE status = 'failed'
		  AND next_retry_at <= NOW()
		ORDER BY next_retry_at ASC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		err := rows.Scan(
			&task.ID, &task.Name, &task.Payload, &task.CallbackURL, &task.Status,
			&task.CreatedAt, &task.ScheduledAt, &task.StartedAt, &task.CompletedAt,
			&task.MaxRetries, &task.RetryCount, &task.RetryBackoffSeconds, &task.NextRetryAt,
			&task.CallbackAttempts, &task.CallbackTimeoutSecs, &task.LastCallbackAt,
			&task.LastCallbackStatus, &task.LastCallbackError, &task.Priority, &task.Tags, &task.ErrorMessage,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &task)
	}

	return tasks, rows.Err()
}

func (r *taskRepository) Update(ctx context.Context, task *models.Task) error {
	query := `
		UPDATE task_queue SET
			status = $2,
			started_at = $3,
			completed_at = $4,
			retry_count = $5,
			next_retry_at = $6,
			callback_attempts = $7,
			last_callback_at = $8,
			last_callback_status = $9,
			last_callback_error = $10,
			error_message = $11
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query,
		task.ID, task.Status, task.StartedAt, task.CompletedAt,
		task.RetryCount, task.NextRetryAt,
		task.CallbackAttempts, task.LastCallbackAt,
		task.LastCallbackStatus, task.LastCallbackError,
		task.ErrorMessage,
	)

	return err
}

func (r *taskRepository) List(ctx context.Context, filter repositories.TaskFilter) ([]*models.Task, int64, error) {
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argNum := 1

	if filter.Status != nil {
		whereClause += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, *filter.Status)
		argNum++
	}

	if filter.Priority != nil {
		whereClause += fmt.Sprintf(" AND priority >= $%d", argNum)
		args = append(args, *filter.Priority)
		argNum++
	}

	if len(filter.Tags) > 0 {
		whereClause += fmt.Sprintf(" AND tags && $%d", argNum)
		args = append(args, filter.Tags)
		argNum++
	}

	if filter.DateFrom != nil {
		whereClause += fmt.Sprintf(" AND created_at >= $%d", argNum)
		args = append(args, *filter.DateFrom)
		argNum++
	}

	if filter.DateTo != nil {
		whereClause += fmt.Sprintf(" AND created_at <= $%d", argNum)
		args = append(args, *filter.DateTo)
		argNum++
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM task_queue " + whereClause
	var total int64
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Build ORDER BY
	orderBy := "created_at DESC"
	if filter.SortBy != "" {
		orderBy = filter.SortBy + " " + filter.SortOrder
	}

	// Add pagination
	offset := (filter.Page - 1) * filter.Limit
	whereClause += fmt.Sprintf(" ORDER BY %s LIMIT $%d OFFSET $%d", orderBy, argNum, argNum+1)
	args = append(args, filter.Limit, offset)

	// Fetch tasks
	query := `
		SELECT id, name, payload, callback_url, status,
			   created_at, scheduled_at, started_at, completed_at,
			   max_retries, retry_count, retry_backoff_seconds, next_retry_at,
			   callback_attempts, callback_timeout_seconds, last_callback_at,
			   last_callback_status, last_callback_error, priority, tags, error_message
		FROM task_queue
	` + whereClause

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		err := rows.Scan(
			&task.ID, &task.Name, &task.Payload, &task.CallbackURL, &task.Status,
			&task.CreatedAt, &task.ScheduledAt, &task.StartedAt, &task.CompletedAt,
			&task.MaxRetries, &task.RetryCount, &task.RetryBackoffSeconds, &task.NextRetryAt,
			&task.CallbackAttempts, &task.CallbackTimeoutSecs, &task.LastCallbackAt,
			&task.LastCallbackStatus, &task.LastCallbackError, &task.Priority, &task.Tags, &task.ErrorMessage,
		)
		if err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, &task)
	}

	return tasks, total, rows.Err()
}

func (r *taskRepository) CountByStatus(ctx context.Context) (map[models.TaskStatus]int64, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM task_queue
		GROUP BY status
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[models.TaskStatus]int64)
	for rows.Next() {
		var status models.TaskStatus
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		result[status] = count
	}

	return result, rows.Err()
}

func (r *taskRepository) CleanupExpiredData(ctx context.Context) (int64, error) {
	// Clean up tasks completed or dead_lettered more than 30 days ago
	// Delete in batches to avoid long-running transactions
	batchSize := 1000
	var totalDeleted int64

	for {
		query := `
			DELETE FROM task_queue
			WHERE ctid IN (
				SELECT ctid FROM task_queue
				WHERE status IN ('completed', 'dead_lettered')
				  AND (
					(status = 'completed' AND completed_at < NOW() - INTERVAL '30 days')
					OR
					(status = 'dead_lettered' AND completed_at < NOW() - INTERVAL '30 days')
				  )
				LIMIT $1
			)
		`

		result, err := r.db.Exec(ctx, query, batchSize)
		if err != nil {
			return totalDeleted, err
		}

		rowsAffected := result.RowsAffected()
		totalDeleted += rowsAffected

		// If we deleted fewer than the batch size, we're done
		if rowsAffected < int64(batchSize) {
			break
		}
	}

	return totalDeleted, nil
}
