package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"later/configs"
	"later/internal/server"
	"later/internal/handler"
	"later/internal/repository/postgres"
	"later/internal/usecase"
	"later/internal/infrastructure/circuitbreaker"

	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg := configs.LoadConfig()

	// Initialize logger
	logger := zap.NewNop() // TODO: Use proper logger based on config

	// Initialize database
	db, err := postgres.NewConnection(cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer postgres.Close(db)

	// Run migrations
	if err := postgres.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	taskRepo := postgres.NewTaskRepository(db)

	// Initialize circuit breaker
	cb := circuitbreaker.NewCircuitBreaker(
		5,                             // maxFailures
		60*time.Second,                // resetTimeout
	)

	// Initialize callback service
	callbackService := usecase.NewCallbackService(
		cfg.Callback.DefaultTimeout,
		cb,
		cfg.Callback.Secret,
		logger,
	)

	// Initialize use cases
	taskService := usecase.NewTaskService(taskRepo)

	// Initialize worker pool
	workerPool := usecase.NewWorkerPool(
		cfg.Worker.PoolSize,
		taskService,
		callbackService,
		logger,
	)
	workerPool.Start(cfg.Worker.PoolSize)

	// Convert configs.Scheduler to usecase.SchedulerConfig
	schedulerCfg := usecase.SchedulerConfig{
		HighPriorityInterval:   cfg.Scheduler.HighPriorityInterval,
		NormalPriorityInterval: cfg.Scheduler.NormalPriorityInterval,
		CleanupInterval:        cfg.Scheduler.CleanupInterval,
	}
	scheduler := usecase.NewScheduler(taskRepo, workerPool, schedulerCfg)

	// Initialize HTTP handler
	h := handler.NewHandler(taskService, scheduler)

	// Start scheduler in background
	go scheduler.Start()

	// Start HTTP server
	srv := server.NewServer(cfg.Server, h)

	// Wait for interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	log.Printf("Server started on %s", cfg.Server.Address)
	log.Printf("Worker pool started with %d workers", cfg.Worker.PoolSize)

	// Graceful shutdown
	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	// Stop scheduler
	scheduler.Stop()

	// Stop worker pool
	workerPool.Stop()

	log.Println("Server stopped")
}
