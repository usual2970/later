package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"later/configs"
	"later/server"
	"later/delivery/rest"
	"later/repository/mysql"
	"later/task"
	"later/callback"
	"later/infrastructure/worker"
	"later/infrastructure/circuitbreaker"
	"later/infrastructure/logger"

	"go.uber.org/zap"
)

func main() {
	// Initialize logger from environment
	if err := logger.InitFromEnv(); err != nil {
		panic(err)
	}
	defer logger.Sync()

	log := logger.Named("main")

	// Load configuration
	cfg, err := configs.LoadConfig("")
	if err != nil {
		log.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Initialize database
	db, err := mysql.NewConnection(&cfg.Database)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer mysql.Close(db)

	// Run migrations
	if err := mysql.RunMigrations(db, "migrations"); err != nil {
		log.Fatal("Failed to run migrations", zap.Error(err))
	}

	// Initialize repositories
	taskRepo := mysql.NewTaskRepository(db)

	// Initialize circuit breaker
	cb := circuitbreaker.NewCircuitBreaker(
		5,                             // maxFailures
		60*time.Second,                // resetTimeout
	)

	// Initialize callback service
	callbackService := callback.NewService(
		cfg.Callback.DefaultTimeout,
		cb,
		cfg.Callback.Secret,
		logger.Named("callback"),
	)

	// Initialize task service
	taskService := task.NewService(taskRepo)

	// Initialize worker pool
	workerPool := worker.NewWorkerPool(
		cfg.Worker.PoolSize,
		taskService,
		callbackService,
		logger.Named("worker"),
	)
	workerPool.Start(cfg.Worker.PoolSize)

	// Convert configs.Scheduler to task.SchedulerConfig
	schedulerCfg := task.SchedulerConfig{
		HighPriorityInterval:   cfg.Scheduler.HighPriorityInterval,
		NormalPriorityInterval: cfg.Scheduler.NormalPriorityInterval,
		CleanupInterval:        cfg.Scheduler.CleanupInterval,
	}
	scheduler := task.NewScheduler(taskRepo, workerPool, schedulerCfg)

	// Initialize HTTP handler
	h := rest.NewHandler(taskService, scheduler)

	// Start HTTP server
	srv := server.NewServer(cfg.Server, h)

	// Start scheduler in background
	go scheduler.Start()

	// Wait for interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed", zap.Error(err))
		}
	}()

	log.Info("Server started",
		zap.String("address", cfg.Server.Address()),
		zap.Int("workers", cfg.Worker.PoolSize),
	)

	// Graceful shutdown
	<-ctx.Done()
	log.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server shutdown failed", zap.Error(err))
	}

	// Stop scheduler
	scheduler.Stop()

	// Stop worker pool
	workerPool.Stop()

	log.Info("Server stopped")
}
