package models

import (
	"database/sql/driver"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// TaskStatus represents the current state of a task
type TaskStatus string

const (
	TaskStatusPending      TaskStatus = "pending"
	TaskStatusProcessing   TaskStatus = "processing"
	TaskStatusCompleted    TaskStatus = "completed"
	TaskStatusFailed       TaskStatus = "failed"
	TaskStatusDeadLettered TaskStatus = "dead_lettered"
)

// Task represents an asynchronous task with callback delivery
type Task struct {
	ID        string     `json:"id" db:"id"`
	Name      string     `json:"name" db:"name"`
	Payload   JSONBytes  `json:"payload" db:"payload"`
	CallbackURL string   `json:"callback_url" db:"callback_url"`
	Status    TaskStatus `json:"status" db:"status"`

	// Timing
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	ScheduledAt time.Time      `json:"scheduled_at" db:"scheduled_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty" db:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty" db:"completed_at"`

	// Retry configuration
	MaxRetries          int        `json:"max_retries" db:"max_retries"`
	RetryCount          int        `json:"retry_count" db:"retry_count"`
	RetryBackoffSeconds int        `json:"retry_backoff_seconds" db:"retry_backoff_seconds"`
	NextRetryAt         *time.Time `json:"next_retry_at,omitempty" db:"next_retry_at"`

	// Callback tracking
	CallbackAttempts    int        `json:"callback_attempts" db:"callback_attempts"`
	CallbackTimeoutSecs int        `json:"callback_timeout_seconds" db:"callback_timeout_seconds"`
	LastCallbackAt      *time.Time `json:"last_callback_at,omitempty" db:"last_callback_at"`
	LastCallbackStatus  *int       `json:"last_callback_status,omitempty" db:"last_callback_status"`
	LastCallbackError   *string    `json:"last_callback_error,omitempty" db:"last_callback_error"`

	// Metadata
	Priority      int      `json:"priority" db:"priority"` // 0-10, higher is more urgent
	Tags          []string `json:"tags,omitempty" db:"tags"`
	ErrorMessage  *string  `json:"error_message,omitempty" db:"error_message"`
	WorkerID      string   `json:"worker_id,omitempty" db:"worker_id"`
}

// JSONBytes is a custom type for handling JSONB in PostgreSQL
type JSONBytes []byte

// Scan implements sql.Scanner for JSONBytes
func (j *JSONBytes) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*j = v
	case string:
		*j = []byte(v)
	default:
		return fmt.Errorf("unsupported type for JSONBytes: %T", value)
	}

	return nil
}

// Value implements driver.Valuer for JSONBytes
func (j JSONBytes) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

// NewTask creates a new task with default values
func NewTask(name string, payload []byte, callbackURL string, scheduledAt time.Time, priority int) *Task {
	return &Task{
		ID:                   uuid.New().String(),
		Name:                 name,
		Payload:              payload,
		CallbackURL:          callbackURL,
		Status:               TaskStatusPending,
		CreatedAt:            time.Now(),
		ScheduledAt:           scheduledAt,
		MaxRetries:           5,
		RetryCount:           0,
		RetryBackoffSeconds:  60,
		CallbackTimeoutSecs:  30,
		Priority:             priority,
	}
}

// CanRetry returns true if the task can be retried
func (t *Task) CanRetry() bool {
	return t.RetryCount < t.MaxRetries && t.Status == TaskStatusFailed
}

// ShouldExecuteNow returns true if the task is scheduled for immediate execution
func (t *Task) ShouldExecuteNow() bool {
	return t.ScheduledAt.Before(time.Now().Add(1 * time.Second))
}

// CalculateNextRetry calculates the next retry time with exponential backoff
func (t *Task) CalculateNextRetry() time.Time {
	backoff := t.RetryBackoffSeconds * (1 << t.RetryCount) // Exponential: 60, 120, 240, 480...
	maxBackoff := 24 * 60 * 60 // 24 hours max

	if backoff > maxBackoff {
		backoff = maxBackoff
	}

	// Add jitter (±25%)
	jitter := int(float64(backoff) * 0.25 * (rand.Float64()*2 - 1))
	backoff += jitter

	return time.Now().Add(time.Duration(backoff) * time.Second)
}

// MarkAsProcessing transitions task to processing status
func (t *Task) MarkAsProcessing(workerID string) {
	t.Status = TaskStatusProcessing
	now := time.Now()
	t.StartedAt = &now
}

// MarkAsCompleted transitions task to completed status
func (t *Task) MarkAsCompleted() {
	t.Status = TaskStatusCompleted
	now := time.Now()
	t.CompletedAt = &now
}

// MarkAsFailed transitions task to failed status with error message
func (t *Task) MarkAsFailed(err error) {
	t.Status = TaskStatusFailed
	t.RetryCount++
	if err != nil {
		errMsg := err.Error()
		t.ErrorMessage = &errMsg
	}

	nextRetry := t.CalculateNextRetry()
	t.NextRetryAt = &nextRetry
}

// MarkAsDeadLettered transitions task to dead_lettered status
func (t *Task) MarkAsDeadLettered() {
	t.Status = TaskStatusDeadLettered
}

// IsHighPriority returns true if task priority is greater than 5
func (t *Task) IsHighPriority() bool {
	return t.Priority > 5
}
