package usecase

import (
	"context"
	"fmt"
	"sync"
	"time"

	"later/internal/domain/models"
	"later/internal/websocket"

	"go.uber.org/zap"
)

// Worker represents a task worker
type Worker struct {
	id         int
	taskChan   <-chan *models.Task
	taskService *TaskService
	callbackService *CallbackService
	wsHub      *websocket.Hub
	wg         *sync.WaitGroup
	quit       chan bool
	logger     *zap.Logger
}

// NewWorker creates a new worker
func NewWorker(
	id int,
	taskChan <-chan *models.Task,
	taskService *TaskService,
	callbackService *CallbackService,
	wsHub *websocket.Hub,
	wg *sync.WaitGroup,
	logger *zap.Logger,
) *Worker {
	return &Worker{
		id:              id,
		taskChan:        taskChan,
		taskService:     taskService,
		callbackService: callbackService,
		wsHub:           wsHub,
		wg:              wg,
		quit:            make(chan bool),
		logger:          logger,
	}
}

// Start begins the worker's main loop
func (w *Worker) Start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.logger.Info("Worker started", zap.Int("worker_id", w.id))

		for {
			select {
			case task := <-w.taskChan:
				if task == nil {
					// Channel closed
					return
				}
				w.processTask(task)

			case <-w.quit:
				w.logger.Info("Worker stopping", zap.Int("worker_id", w.id))
				return
			}
		}
	}()
}

// Stop signals the worker to stop
func (w *Worker) Stop() {
	close(w.quit)
}

// processTask handles the execution of a single task
func (w *Worker) processTask(task *models.Task) {
	ctx := context.Background()

	w.logger.Info("Processing task",
		zap.Int("worker_id", w.id),
		zap.String("task_id", task.ID),
		zap.String("task_name", task.Name))

	// Mark task as processing
	workerID := fmt.Sprintf("worker-%d", w.id)
	task.MarkAsProcessing(workerID)
	task.WorkerID = workerID

	if err := w.taskService.UpdateTask(ctx, task); err != nil {
		w.logger.Error("Failed to mark task as processing",
			zap.Int("worker_id", w.id),
			zap.String("task_id", task.ID),
			zap.Error(err))
		return
	}

	// Broadcast status change to WebSocket clients
	if w.wsHub != nil {
		w.wsHub.BroadcastTaskUpdated(task.ID, string(task.Status))
	}

	// Deliver callback
	callbackErr := w.callbackService.DeliverCallback(ctx, task)

	if callbackErr != nil {
		w.logger.Error("Task callback failed",
			zap.Int("worker_id", w.id),
			zap.String("task_id", task.ID),
			zap.Error(callbackErr))

		// Handle failure with retry logic
		if task.CanRetry() {
			w.handleRetry(task)
		} else {
			w.handleFailure(task, callbackErr)
		}
	} else {
		// Mark task as completed
		task.MarkAsCompleted()
		if err := w.taskService.UpdateTask(ctx, task); err != nil {
			w.logger.Error("Failed to mark task as completed",
				zap.Int("worker_id", w.id),
				zap.String("task_id", task.ID),
				zap.Error(err))
			return
		}

		// Broadcast status change to WebSocket clients
		if w.wsHub != nil {
			w.wsHub.BroadcastTaskUpdated(task.ID, string(task.Status))
		}

		w.logger.Info("Task completed successfully",
			zap.Int("worker_id", w.id),
			zap.String("task_id", task.ID))
	}
}

// handleRetry handles task retry with exponential backoff
func (w *Worker) handleRetry(task *models.Task) {
	ctx := context.Background()

	// Calculate next retry time
	nextRetryAt := task.CalculateNextRetry()
	task.NextRetryAt = &nextRetryAt
	task.RetryCount++

	// Update task in database
	if err := w.taskService.UpdateTask(ctx, task); err != nil {
		w.logger.Error("Failed to update task for retry",
			zap.Int("worker_id", w.id),
			zap.String("task_id", task.ID),
			zap.Error(err))
		return
	}

	w.logger.Info("Task scheduled for retry",
		zap.Int("worker_id", w.id),
		zap.String("task_id", task.ID),
		zap.Int("retry_count", task.RetryCount),
		zap.Time("next_retry_at", nextRetryAt))
}

// handleFailure handles task failure when max retries exceeded
func (w *Worker) handleFailure(task *models.Task, err error) {
	ctx := context.Background()

	// Check if max retries exceeded
	if task.RetryCount >= task.MaxRetries {
		// Mark as dead lettered
		task.MarkAsDeadLettered()
		errMsg := fmt.Sprintf("Max retries (%d) exceeded: %v", task.MaxRetries, err)
		task.ErrorMessage = &errMsg

		if updateErr := w.taskService.UpdateTask(ctx, task); updateErr != nil {
			w.logger.Error("Failed to mark task as dead_lettered",
				zap.Int("worker_id", w.id),
				zap.String("task_id", task.ID),
				zap.Error(updateErr))
			return
		}

		// Broadcast status change to WebSocket clients
		if w.wsHub != nil {
			w.wsHub.BroadcastTaskUpdated(task.ID, string(task.Status))
		}

		w.logger.Error("Task moved to dead letter queue",
			zap.Int("worker_id", w.id),
			zap.String("task_id", task.ID),
			zap.Int("retry_count", task.RetryCount),
			zap.Int("max_retries", task.MaxRetries))
	} else {
		// Just mark as failed
		task.MarkAsFailed(err)
		if updateErr := w.taskService.UpdateTask(ctx, task); updateErr != nil {
			w.logger.Error("Failed to mark task as failed",
				zap.Int("worker_id", w.id),
				zap.String("task_id", task.ID),
				zap.Error(updateErr))
			return
		}

		// Broadcast status change to WebSocket clients
		if w.wsHub != nil {
			w.wsHub.BroadcastTaskUpdated(task.ID, string(task.Status))
		}
	}
}

// WorkerPool manages a pool of workers
type WorkerPool struct {
	workers    []*Worker
	taskChan   chan *models.Task
	taskService *TaskService
	callbackService *CallbackService
	wsHub      *websocket.Hub
	wg         *sync.WaitGroup
	logger     *zap.Logger
	quit       chan bool
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(
	workerCount int,
	taskService *TaskService,
	callbackService *CallbackService,
	wsHub *websocket.Hub,
	logger *zap.Logger,
) *WorkerPool {
	return &WorkerPool{
		taskChan:        make(chan *models.Task, workerCount*2),
		taskService:     taskService,
		callbackService: callbackService,
		wsHub:           wsHub,
		wg:              &sync.WaitGroup{},
		logger:          logger,
		quit:            make(chan bool),
	}
}

// Start initializes and starts all workers
func (p *WorkerPool) Start(workerCount int) {
	p.workers = make([]*Worker, workerCount)
	for i := 0; i < workerCount; i++ {
		p.workers[i] = NewWorker(
			i+1,
			p.taskChan,
			p.taskService,
			p.callbackService,
			p.wsHub,
			p.wg,
			p.logger,
		)
		p.workers[i].Start()
	}

	p.logger.Info("Worker pool started",
		zap.Int("worker_count", workerCount),
	)
}

// Stop gracefully shuts down all workers
func (p *WorkerPool) Stop() {
	p.logger.Info("Stopping worker pool")

	// Stop all workers
	for _, worker := range p.workers {
		worker.Stop()
	}

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("All workers stopped")
	case <-time.After(30 * time.Second):
		p.logger.Warn("Timeout waiting for workers to stop")
	}

	close(p.taskChan)
}

// SubmitTask submits a task to the worker pool
func (p *WorkerPool) SubmitTask(task *models.Task) bool {
	select {
	case p.taskChan <- task:
		return true
	default:
		return false
	}
}

// WorkerCount returns the number of active workers
func (p *WorkerPool) WorkerCount() int {
	return len(p.workers)
}

// SetWSHub sets or updates the WebSocket hub for the worker pool
func (p *WorkerPool) SetWSHub(wsHub *websocket.Hub) {
	p.wsHub = wsHub
	// Update all existing workers
	for _, worker := range p.workers {
		worker.SetWSHub(wsHub)
	}
}

// SetWSHub sets or updates the WebSocket hub for a single worker
func (w *Worker) SetWSHub(wsHub *websocket.Hub) {
	w.wsHub = wsHub
}
