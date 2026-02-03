package mysql

import (
	"context"
	"encoding/json"
	"fmt"

	"later/internal/domain/models"
	"later/internal/domain/repositories"

	"github.com/jmoiron/sqlx"
)

// taskRepository implements repositories.TaskRepository
type taskRepository struct {
	db *sqlx.DB
}

// NewTaskRepository creates a new MySQL task repository
func NewTaskRepository(db *sqlx.DB) repositories.TaskRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) Create(ctx context.Context, task *models.Task) error {
	query := `
		INSERT INTO task_queue (
			id, name, payload, callback_url, status,
			created_at, scheduled_at, max_retries, retry_count,
			retry_backoff_seconds, callback_timeout_seconds, priority, tags
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Convert tags to JSON for MySQL
	tagsJSON, err := json.Marshal(task.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		task.ID, task.Name, task.Payload, task.CallbackURL, task.Status,
		task.CreatedAt, task.ScheduledAt, task.MaxRetries, task.RetryCount,
		task.RetryBackoffSeconds, task.CallbackTimeoutSecs, task.Priority, tagsJSON,
	)

	return err
}

func (r *taskRepository) FindByID(ctx context.Context, id string) (*models.Task, error) {
	query := `
		SELECT id, name, payload, callback_url, status,
			   created_at, scheduled_at, started_at, completed_at,
			   max_retries, retry_count, retry_backoff_seconds, next_retry_at,
			   callback_attempts, callback_timeout_seconds, last_callback_at,
			   last_callback_status, last_callback_error, priority, tags, error_message,
			   deleted_at, deleted_by
		FROM task_queue
		WHERE id = ? AND deleted_at IS NULL
	`

	var task models.Task
	var tagsJSON []byte
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID, &task.Name, &task.Payload, &task.CallbackURL, &task.Status,
		&task.CreatedAt, &task.ScheduledAt, &task.StartedAt, &task.CompletedAt,
		&task.MaxRetries, &task.RetryCount, &task.RetryBackoffSeconds, &task.NextRetryAt,
		&task.CallbackAttempts, &task.CallbackTimeoutSecs, &task.LastCallbackAt,
		&task.LastCallbackStatus, &task.LastCallbackError, &task.Priority, &tagsJSON, &task.ErrorMessage,
		&task.DeletedAt, &task.DeletedBy,
	)
	if err != nil {
		return nil, err
	}

	// Unmarshal tags from JSON
	if tagsJSON != nil {
		if err := json.Unmarshal(tagsJSON, &task.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}
	}

	return &task, nil
}

func (r *taskRepository) FindDueTasks(ctx context.Context, minPriority int, limit int) ([]*models.Task, error) {
	query := `
		SELECT id, name, payload, callback_url, status,
			   created_at, scheduled_at, started_at, completed_at,
			   max_retries, retry_count, retry_backoff_seconds, next_retry_at,
			   callback_attempts, callback_timeout_seconds, last_callback_at,
			   last_callback_status, last_callback_error, priority, tags, error_message,
			   deleted_at, deleted_by
		FROM task_queue
		WHERE status = 'pending'
		  AND scheduled_at <= UTC_TIMESTAMP()
		  AND deleted_at IS NULL
		  AND (? = -1 OR priority > ?)
		ORDER BY priority DESC, scheduled_at ASC
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`

	rows, err := r.db.QueryContext(ctx, query, minPriority, minPriority, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		var tagsJSON []byte
		err := rows.Scan(
			&task.ID, &task.Name, &task.Payload, &task.CallbackURL, &task.Status,
			&task.CreatedAt, &task.ScheduledAt, &task.StartedAt, &task.CompletedAt,
			&task.MaxRetries, &task.RetryCount, &task.RetryBackoffSeconds, &task.NextRetryAt,
			&task.CallbackAttempts, &task.CallbackTimeoutSecs, &task.LastCallbackAt,
			&task.LastCallbackStatus, &task.LastCallbackError, &task.Priority, &tagsJSON, &task.ErrorMessage,
			&task.DeletedAt, &task.DeletedBy,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal tags from JSON
		if tagsJSON != nil {
			if err := json.Unmarshal(tagsJSON, &task.Tags); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
			}
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
			   last_callback_status, last_callback_error, priority, tags, error_message,
			   deleted_at, deleted_by
		FROM task_queue
		WHERE status = 'failed'
		  AND next_retry_at <= UTC_TIMESTAMP()
		  AND deleted_at IS NULL
		ORDER BY next_retry_at ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		var tagsJSON []byte
		err := rows.Scan(
			&task.ID, &task.Name, &task.Payload, &task.CallbackURL, &task.Status,
			&task.CreatedAt, &task.ScheduledAt, &task.StartedAt, &task.CompletedAt,
			&task.MaxRetries, &task.RetryCount, &task.RetryBackoffSeconds, &task.NextRetryAt,
			&task.CallbackAttempts, &task.CallbackTimeoutSecs, &task.LastCallbackAt,
			&task.LastCallbackStatus, &task.LastCallbackError, &task.Priority, &tagsJSON, &task.ErrorMessage,
			&task.DeletedAt, &task.DeletedBy,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal tags from JSON
		if tagsJSON != nil {
			if err := json.Unmarshal(tagsJSON, &task.Tags); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
			}
		}

		tasks = append(tasks, &task)
	}

	return tasks, rows.Err()
}

func (r *taskRepository) Update(ctx context.Context, task *models.Task) error {
	query := `
		UPDATE task_queue SET
			status = ?,
			started_at = ?,
			completed_at = ?,
			retry_count = ?,
			next_retry_at = ?,
			callback_attempts = ?,
			last_callback_at = ?,
			last_callback_status = ?,
			last_callback_error = ?,
			error_message = ?
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query,
		task.Status, task.StartedAt, task.CompletedAt,
		task.RetryCount, task.NextRetryAt,
		task.CallbackAttempts, task.LastCallbackAt,
		task.LastCallbackStatus, task.LastCallbackError,
		task.ErrorMessage,
		task.ID,
	)

	return err
}

func (r *taskRepository) SoftDelete(ctx context.Context, taskID string, deletedBy string) error {
	query := `
		UPDATE task_queue
		SET deleted_at = UTC_TIMESTAMP(), deleted_by = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, deletedBy, taskID)
	if err != nil {
		return err
	}

	// Check if a row was actually updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		// Task either doesn't exist or is already deleted
		return fmt.Errorf("task not found or already deleted")
	}

	return nil
}

func (r *taskRepository) List(ctx context.Context, filter repositories.TaskFilter) ([]*models.Task, int64, error) {
	whereClause := "WHERE deleted_at IS NULL"
	args := []interface{}{}

	if filter.Status != nil {
		whereClause += " AND status = ?"
		args = append(args, *filter.Status)
	}

	if filter.Priority != nil {
		whereClause += " AND priority >= ?"
		args = append(args, *filter.Priority)
	}

	if len(filter.Tags) > 0 {
		// MySQL JSON array search
		whereClause += " AND JSON_CONTAINS(tags, JSON_QUOTE(?))"
		args = append(args, filter.Tags[0])
	}

	if filter.DateFrom != nil {
		whereClause += " AND created_at >= ?"
		args = append(args, *filter.DateFrom)
	}

	if filter.DateTo != nil {
		whereClause += " AND created_at <= ?"
		args = append(args, *filter.DateTo)
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM task_queue " + whereClause
	var total int64
	err := r.db.GetContext(ctx, &total, countQuery, args...)
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
	whereClause += fmt.Sprintf(" ORDER BY %s LIMIT ? OFFSET ?", orderBy)
	args = append(args, filter.Limit, offset)

	// Fetch tasks
	query := `
		SELECT id, name, payload, callback_url, status,
			   created_at, scheduled_at, started_at, completed_at,
			   max_retries, retry_count, retry_backoff_seconds, next_retry_at,
			   callback_attempts, callback_timeout_seconds, last_callback_at,
			   last_callback_status, last_callback_error, priority, tags, error_message,
			   deleted_at, deleted_by
		FROM task_queue
	` + whereClause

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		var tagsJSON []byte
		err := rows.Scan(
			&task.ID, &task.Name, &task.Payload, &task.CallbackURL, &task.Status,
			&task.CreatedAt, &task.ScheduledAt, &task.StartedAt, &task.CompletedAt,
			&task.MaxRetries, &task.RetryCount, &task.RetryBackoffSeconds, &task.NextRetryAt,
			&task.CallbackAttempts, &task.CallbackTimeoutSecs, &task.LastCallbackAt,
			&task.LastCallbackStatus, &task.LastCallbackError, &task.Priority, &tagsJSON, &task.ErrorMessage,
			&task.DeletedAt, &task.DeletedBy,
		)
		if err != nil {
			return nil, 0, err
		}

		// Unmarshal tags from JSON
		if tagsJSON != nil {
			if err := json.Unmarshal(tagsJSON, &task.Tags); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal tags: %w", err)
			}
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

	rows, err := r.db.QueryContext(ctx, query)
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
			DELETE tq
			FROM task_queue tq
			INNER JOIN (
				SELECT id FROM task_queue
				WHERE status IN ('completed', 'dead_lettered')
				  AND completed_at < DATE_SUB(UTC_TIMESTAMP(), INTERVAL 30 DAY)
				LIMIT ?
			) AS tmp ON tq.id = tmp.id
		`

		result, err := r.db.ExecContext(ctx, query, batchSize)
		if err != nil {
			return totalDeleted, err
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return totalDeleted, err
		}

		totalDeleted += rowsAffected

		// If we deleted fewer than the batch size, we're done
		if rowsAffected < int64(batchSize) {
			break
		}
	}

	return totalDeleted, nil
}
