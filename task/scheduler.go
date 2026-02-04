package task

import (
	"context"
	"later/domain/entity"
	"later/domain/repository"
	"later/infrastructure/worker"
	"log"
	"time"

	"go.uber.org/zap"
)

// Scheduler handles tiered polling for task scheduling
type Scheduler struct {
	highPriorityTicker   *time.Ticker
	normalPriorityTicker *time.Ticker
	cleanupTicker        *time.Ticker

	taskRepo   repository.TaskRepository
	workerPool worker.WorkerPool
	logger     *zap.Logger
	quit       chan struct{}
}

// NewScheduler creates a new scheduler with tiered polling
func NewScheduler(
	repo repository.TaskRepository,
	workerPool worker.WorkerPool,
	cfg SchedulerConfig,
) *Scheduler {
	return &Scheduler{
		highPriorityTicker:   time.NewTicker(cfg.HighPriorityInterval),
		normalPriorityTicker: time.NewTicker(cfg.NormalPriorityInterval),
		cleanupTicker:        time.NewTicker(cfg.CleanupInterval),
		taskRepo:             repo,
		workerPool:           workerPool,
		logger:               zap.NewNop(), // TODO: Use proper logger
		quit:                 make(chan struct{}),
	}
}

type SchedulerConfig struct {
	HighPriorityInterval   time.Duration
	NormalPriorityInterval time.Duration
	CleanupInterval        time.Duration
}

// Start begins the tiered polling scheduler
func (s *Scheduler) Start() {
	defer s.highPriorityTicker.Stop()
	defer s.normalPriorityTicker.Stop()
	defer s.cleanupTicker.Stop()

	log.Println("Scheduler started with tiered polling")

	// Initial poll
	s.pollDueTasks("high", 5, 50)
	s.pollDueTasks("normal", 0, 100)

	for {
		select {
		case <-s.highPriorityTicker.C:
			s.pollDueTasks("high", 5, 50)

		case <-s.normalPriorityTicker.C:
			s.pollDueTasks("normal", 0, 100)

		case <-s.cleanupTicker.C:
			s.pollDueTasks("low", -1, 200)
			s.cleanupExpiredTasks()

		case <-s.quit:
			log.Println("Scheduler stopping...")
			return
		}
	}
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	close(s.quit)
}

// SubmitTaskImmediately submits a task directly to the worker pool
func (s *Scheduler) SubmitTaskImmediately(task *entity.Task) {
	if s.workerPool.SubmitTask(task) {
		log.Printf("Task submitted immediately: %s (priority: %d)", task.ID, task.Priority)
	} else {
		log.Printf("Worker pool full, task will be picked up by next poll: %s", task.ID)
	}
}

func (s *Scheduler) pollDueTasks(tier string, minPriority int, limit int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tasks, err := s.taskRepo.FindDueTasks(ctx, minPriority, limit)
	if err != nil {
		log.Printf("Failed to fetch due tasks (tier=%s): %v", tier, err)
		return
	}

	if len(tasks) == 0 {
		// Only poll for retries if no new pending tasks
		s.pollRetryTasks(tier, limit)
		return
	}

	log.Printf("Found %d due tasks (tier=%s)", len(tasks), tier)

	submitted := 0
	for _, task := range tasks {
		if s.workerPool.SubmitTask(task) {
			submitted++
		} else {
			log.Printf("Worker pool full, task will be retried next cycle: %s", task.ID)
		}
	}

	log.Printf("Tasks submitted to workers (tier=%s): %d/%d", tier, submitted, len(tasks))
}

func (s *Scheduler) pollRetryTasks(tier string, limit int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Poll for failed tasks ready for retry
	retryTasks, err := s.taskRepo.FindFailedTasks(ctx, limit)
	if err != nil {
		log.Printf("Failed to fetch retry tasks (tier=%s): %v", tier, err)
		return
	}

	if len(retryTasks) == 0 {
		return
	}

	log.Printf("Found %d retry tasks (tier=%s)", len(retryTasks), tier)

	submitted := 0
	for _, task := range retryTasks {
		// Reset task to pending before resubmitting
		task.Status = entity.TaskStatusPending

		if s.workerPool.SubmitTask(task) {
			submitted++
		} else {
			log.Printf("Worker pool full, retry task will be retried next cycle: %s", task.ID)
		}
	}

	log.Printf("Retry tasks submitted to workers (tier=%s): %d/%d", tier, submitted, len(retryTasks))
}

func (s *Scheduler) cleanupExpiredTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := s.taskRepo.CleanupExpiredData(ctx)
	if err != nil {
		log.Printf("Failed to cleanup expired data: %v", err)
		return
	}

	if count > 0 {
		log.Printf("Cleaned up %d expired tasks", count)
	}
}
