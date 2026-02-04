package later

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// Start begins background processing (scheduler and workers)
// Must be called before creating/processing tasks
func (l *Later) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.started {
		return fmt.Errorf("already started")
	}

	l.logger.Info("Starting Later")

	// Start worker pool
	l.workerPool.Start(l.config.WorkerPoolSize)

	// Start scheduler in background goroutine
	go l.scheduler.Start()

	l.started = true
	l.logger.Info("Later started successfully")
	return nil
}

// Shutdown gracefully stops Later
// Waits for in-flight tasks to complete or until context is cancelled
func (l *Later) Shutdown(ctx context.Context) error {
	l.mu.Lock()
	if !l.started {
		l.mu.Unlock()
		return nil
	}
	l.mu.Unlock()

	l.logger.Info("Shutting down Later")

	// Stop scheduler (stops polling)
	l.scheduler.Stop()

	// Stop worker pool (waits for in-flight tasks)
	// Note: Current worker pool implementation doesn't accept context
	// It has a fixed 30-second timeout internally
	l.workerPool.Stop()

	// Wait for context cancellation or immediate return
	select {
	case <-ctx.Done():
		l.logger.Warn("Shutdown context cancelled", zap.Error(ctx.Err()))
		return ctx.Err()
	default:
		// Worker pool stopped successfully
	}

	// Close database if we own it
	if l.closeDB && l.db != nil {
		if err := l.db.Close(); err != nil {
			l.logger.Error("Database close error", zap.Error(err))
			return err
		}
		l.logger.Info("Database connection closed")
	}

	l.cancel()
	l.logger.Info("Later shutdown complete")
	return nil
}

// HealthCheck returns health status for monitoring
func (l *Later) HealthCheck() HealthStatus {
	l.mu.RLock()
	defer l.mu.RUnlock()

	status := HealthStatus{
		Started: l.started,
	}

	if !l.started {
		status.Status = "stopped"
		return status
	}

	// Check database
	if err := l.db.Ping(); err != nil {
		status.Status = "unhealthy"
		status.Database = "disconnected"
		status.Error = err.Error()
		return status
	}
	status.Database = "connected"

	// Check scheduler status
	// Note: Current scheduler doesn't expose IsRunning()
	// We'll assume it's running if Later is started
	status.Scheduler = "running"

	// Check worker pool status
	// Note: Current worker pool exposes WorkerCount() but not ActiveCount()
	// We'll use what's available
	activeWorkers := 0
	if wp, ok := l.workerPool.(interface{ WorkerCount() int }); ok {
		activeWorkers = wp.WorkerCount()
	}

	status.Workers = &WorkerStatus{
		Active: activeWorkers,
		Total:  l.config.WorkerPoolSize,
	}

	status.Status = "healthy"
	return status
}

// HealthStatus represents the health status of Later
type HealthStatus struct {
	Status    string       `json:"status"`     // healthy, unhealthy, stopped
	Database  string       `json:"database"`   // connected, disconnected
	Scheduler string       `json:"scheduler"`  // running, stopped
	Workers   *WorkerStatus `json:"workers,omitempty"`
	Started   bool         `json:"started"`
	Error     string       `json:"error,omitempty"`
}

// WorkerStatus represents the status of the worker pool
type WorkerStatus struct {
	Active int `json:"active"`
	Total  int `json:"total"`
}
