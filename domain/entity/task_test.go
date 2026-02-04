package entity

import (
	"testing"
	"time"
)

func TestCanBeDeleted(t *testing.T) {
	tests := []struct {
		name     string
		task     *Task
		expected bool
	}{
		{
			name:     "Pending task can be deleted",
			task:     &Task{Status: TaskStatusPending, DeletedAt: nil},
			expected: true,
		},
		{
			name:     "Failed task can be deleted",
			task:     &Task{Status: TaskStatusFailed, DeletedAt: nil},
			expected: true,
		},
		{
			name:     "Processing task cannot be deleted",
			task:     &Task{Status: TaskStatusProcessing, DeletedAt: nil},
			expected: false,
		},
		{
			name:     "Completed task cannot be deleted",
			task:     &Task{Status: TaskStatusCompleted, DeletedAt: nil},
			expected: false,
		},
		{
			name:     "Dead lettered task cannot be deleted",
			task:     &Task{Status: TaskStatusDeadLettered, DeletedAt: nil},
			expected: false,
		},
		{
			name:     "Already deleted task cannot be deleted again",
			task:     &Task{Status: TaskStatusPending, DeletedAt: &[]time.Time{time.Now()}[0]},
			expected: false,
		},
		{
			name:     "Deleted failed task cannot be deleted again",
			task:     &Task{Status: TaskStatusFailed, DeletedAt: &[]time.Time{time.Now()}[0]},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.task.CanBeDeleted()
			if result != tt.expected {
				t.Errorf("CanBeDeleted() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestIsDeleted(t *testing.T) {
	tests := []struct {
		name     string
		task     *Task
		expected bool
	}{
		{
			name:     "Task with nil DeletedAt is not deleted",
			task:     &Task{DeletedAt: nil},
			expected: false,
		},
		{
			name:     "Task with DeletedAt set is deleted",
			task:     &Task{DeletedAt: &[]time.Time{time.Now()}[0]},
			expected: true,
		},
		{
			name:     "Pending task with no DeletedAt is not deleted",
			task:     &Task{Status: TaskStatusPending, DeletedAt: nil},
			expected: false,
		},
		{
			name:     "Failed task with DeletedAt is deleted",
			task:     &Task{Status: TaskStatusFailed, DeletedAt: &[]time.Time{time.Now()}[0]},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.task.IsDeleted()
			if result != tt.expected {
				t.Errorf("IsDeleted() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestCanRetry(t *testing.T) {
	tests := []struct {
		name     string
		task     *Task
		expected bool
	}{
		{
			name:     "Failed task with retries available can retry",
			task:     &Task{Status: TaskStatusFailed, RetryCount: 2, MaxRetries: 5},
			expected: true,
		},
		{
			name:     "Failed task with no retries left cannot retry",
			task:     &Task{Status: TaskStatusFailed, RetryCount: 5, MaxRetries: 5},
			expected: false,
		},
		{
			name:     "Pending task cannot retry",
			task:     &Task{Status: TaskStatusPending, RetryCount: 0, MaxRetries: 5},
			expected: false,
		},
		{
			name:     "Completed task cannot retry",
			task:     &Task{Status: TaskStatusCompleted, RetryCount: 0, MaxRetries: 5},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.task.CanRetry()
			if result != tt.expected {
				t.Errorf("CanRetry() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestIsHighPriority(t *testing.T) {
	tests := []struct {
		name     string
		task     *Task
		expected bool
	}{
		{
			name:     "Priority 6 is high priority",
			task:     &Task{Priority: 6},
			expected: true,
		},
		{
			name:     "Priority 10 is high priority",
			task:     &Task{Priority: 10},
			expected: true,
		},
		{
			name:     "Priority 5 is not high priority",
			task:     &Task{Priority: 5},
			expected: false,
		},
		{
			name:     "Priority 0 is not high priority",
			task:     &Task{Priority: 0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.task.IsHighPriority()
			if result != tt.expected {
				t.Errorf("IsHighPriority() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
