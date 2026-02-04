package dto

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"later/domain/entity"
	"later/domain/repository"
)

// CreateTaskRequest represents a request to create a new task
type CreateTaskRequest struct {
	Name           string          `json:"name" binding:"required"`
	Payload        entity.JSONBytes   `json:"payload" binding:"required"`
	CallbackURL    string      `json:"callback_url" binding:"required,url"`
	ScheduledFor   *CustomTime `json:"scheduled_for"`
	TimeoutSeconds *int        `json:"timeout_seconds"`
	MaxRetries     *int        `json:"max_retries"`
	Priority       int         `json:"priority"`
	Tags           []string    `json:"tags"`
}

// Validate validates the request and returns an error if invalid
func (r *CreateTaskRequest) Validate() error {
	// Validate payload size (max 1MB)
	if len(r.Payload) > 1024*1024 {
		return fmt.Errorf("payload size exceeds 1MB limit")
	}

	// Validate timeout_seconds (5-300 range)
	if r.TimeoutSeconds != nil && (*r.TimeoutSeconds < 5 || *r.TimeoutSeconds > 300) {
		return fmt.Errorf("timeout_seconds must be between 5 and 300 seconds")
	}

	// Validate max_retries (0-20 range)
	if r.MaxRetries != nil && (*r.MaxRetries < 0 || *r.MaxRetries > 20) {
		return fmt.Errorf("max_retries must be between 0 and 20")
	}

	// Validate priority (0-10 range)
	if r.Priority < 0 || r.Priority > 10 {
		return fmt.Errorf("priority must be between 0 and 10")
	}

	// Validate scheduled_for (must be future or within 1 year)
	if r.ScheduledFor != nil && !r.ScheduledFor.IsZero() {
		now := time.Now()
		scheduledTime := r.ScheduledFor.Time
		if scheduledTime.Before(now.AddDate(0, 0, -1)) {
			// Allow tasks scheduled in the past - they'll execute immediately
			return nil
		}
		if scheduledTime.After(now.AddDate(1, 0, 0)) {
			return fmt.Errorf("scheduled_for must be within 1 year from now")
		}
	}

	return nil
}

// TaskResponse represents a task response
type TaskResponse struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Payload            json.RawMessage   `json:"payload"`
	CallbackURL        string            `json:"callback_url"`
	Status             entity.TaskStatus `json:"status"`
	CreatedAt          time.Time         `json:"created_at"`
	ScheduledFor       time.Time         `json:"scheduled_at"`
	StartedAt          *time.Time        `json:"started_at,omitempty"`
	CompletedAt        *time.Time        `json:"completed_at,omitempty"`
	MaxRetries         int               `json:"max_retries"`
	RetryCount         int               `json:"retry_count"`
	CallbackAttempts   int               `json:"callback_attempts"`
	Priority           int               `json:"priority"`
	Tags               []string          `json:"tags,omitempty"`
	ErrorMessage       *string           `json:"error_message,omitempty"`
	EstimatedExecution string            `json:"estimated_execution,omitempty"`
}

// MarshalJSON implements json.Marshaler to ensure all times are in UTC
// TEMPORARILY DISABLED TO DEBUG PERFORMANCE ISSUE
func (tr TaskResponse) MarshalJSON() ([]byte, error) {
	// Create a new type to avoid infinite recursion
	type Alias TaskResponse

	aux := &struct {
		Alias
		CreatedAt   string `json:"created_at"`
		ScheduledFor string `json:"scheduled_at"`
		StartedAt    *string `json:"started_at,omitempty"`
		CompletedAt  *string `json:"completed_at,omitempty"`
	}{
		Alias:        (Alias)(tr),
		CreatedAt:    tr.CreatedAt.UTC().Format(time.RFC3339),
		ScheduledFor: tr.ScheduledFor.UTC().Format(time.RFC3339),
	}

	if tr.StartedAt != nil {
		s := tr.StartedAt.UTC().Format(time.RFC3339)
		aux.StartedAt = &s
	}

	if tr.CompletedAt != nil {
		s := tr.CompletedAt.UTC().Format(time.RFC3339)
		aux.CompletedAt = &s
	}

	return json.Marshal(aux)
}

// ToModel converts CreateTaskRequest to a Task entity
func (r *CreateTaskRequest) ToModel() *entity.Task {
	now := time.Now()
	scheduledAt := now

	if r.ScheduledFor != nil && !r.ScheduledFor.IsZero() {
		scheduledAt = r.ScheduledFor.Time
	}

	// Set defaults
	maxRetries := 5
	if r.MaxRetries != nil {
		maxRetries = *r.MaxRetries
	}

	timeoutSeconds := 30
	if r.TimeoutSeconds != nil {
		timeoutSeconds = *r.TimeoutSeconds
	}

	priority := r.Priority
	if priority == 0 {
		priority = 0
	}

	task := entity.NewTask(r.Name, r.Payload, r.CallbackURL, scheduledAt, priority)

	// Override defaults with request values
	task.MaxRetries = maxRetries
	task.CallbackTimeoutSecs = timeoutSeconds
	task.Tags = r.Tags

	return task
}

// ListTasksQuery represents query parameters for listing tasks
type ListTasksQuery struct {
	Status    *entity.TaskStatus `form:"status"`
	Priority  *int               `form:"priority"`
	Tags      string             `form:"tags"` // comma-separated
	DateFrom  *string            `form:"date_from"`
	DateTo    *string            `form:"date_to"`
	Page      int                `form:"page" binding:"required,min=1"`
	Limit     int                `form:"limit" binding:"required,min=1,max=100"`
	SortBy    string             `form:"sort_by"`
	SortOrder string             `form:"sort_order"`
}

// Validate validates and normalizes the query parameters
func (q *ListTasksQuery) Validate() error {
	// Set defaults
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 50
	}

	// Validate sort_by
	validSortBy := map[string]bool{
		"created_at":   true,
		"scheduled_at": true,
		"priority":     true,
	}
	if !validSortBy[q.SortBy] {
		q.SortBy = "created_at"
	}

	// Validate sort_order
	if q.SortOrder != "asc" && q.SortOrder != "desc" {
		q.SortOrder = "desc"
	}

	return nil
}

// ToRepositoryFilter converts ListTasksQuery to repository filter
func (q *ListTasksQuery) ToRepositoryFilter() (*repository.TaskFilter, error) {
	filter := &repository.TaskFilter{
		Status:    q.Status,
		Priority:  q.Priority,
		Page:      q.Page,
		Limit:     q.Limit,
		SortBy:    q.SortBy,
		SortOrder: q.SortOrder,
	}

	// Parse tags
	if q.Tags != "" {
		filter.Tags = strings.Split(q.Tags, ",")
	}

	// Parse dates
	if q.DateFrom != nil {
		dateFrom, err := time.Parse(time.RFC3339, *q.DateFrom)
		if err != nil {
			return nil, fmt.Errorf("invalid date_from format: %w", err)
		}
		filter.DateFrom = &dateFrom
	}

	if q.DateTo != nil {
		dateTo, err := time.Parse(time.RFC3339, *q.DateTo)
		if err != nil {
			return nil, fmt.Errorf("invalid date_to format: %w", err)
		}
		filter.DateTo = &dateTo
	}

	return filter, nil
}

// TaskListResponse represents a paginated list of tasks
type TaskListResponse struct {
	Tasks      []*TaskResponse `json:"tasks"`
	Pagination PaginationInfo  `json:"pagination"`
}

// PaginationInfo represents pagination metadata
type PaginationInfo struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// StatsResponse represents statistics about tasks
type StatsResponse struct {
	Total               int64                       `json:"total"`
	ByStatus            map[entity.TaskStatus]int64 `json:"by_status"`
	Last24h             Last24hStats                `json:"last_24h"`
	CallbackSuccessRate float64                     `json:"callback_success_rate"`
}

// Last24hStats represents statistics for the last 24 hours
type Last24hStats struct {
	Submitted int64 `json:"submitted"`
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
