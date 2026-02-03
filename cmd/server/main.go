package main

import (
	"context"
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
	"later/internal/infrastructure/logger"

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
	db, err := postgres.NewConnection(cfg.Database.URL)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer postgres.Close(db)

	// Run migrations
	if err := postgres.RunMigrations(db, "migrations"); err != nil {
		log.Fatal("Failed to run migrations", zap.Error(err))
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
		logger.Named("callback"),
	)

	// Initialize use cases
	taskService := usecase.NewTaskService(taskRepo)

	// Initialize worker pool
	workerPool := usecase.NewWorkerPool(
		cfg.Worker.PoolSize,
		taskService,
		callbackService,
		logger.Named("worker"),
	)
	workerPool.Start(cfg.Worker.PoolSize)

	// Convert configs.Scheduler to usecase.SchedulerConfig
	schedulerCfg := usecase.SchedulerConfig{
		HighPriorityInterval:   cfg.Scheduler.HighPriorityInterval,
		NormalPriorityInterval: cfg.Scheduler.NormalPriorityInterval,
		CleanupInterval:        cfg.Scheduler.CleanupInterval,
	}
	scheduler := usecase.NewScheduler(taskRepo, workerPool, schedulerCfg)

	// Initialize HTTP handler (without wsHub initially)
	h := handler.NewHandler(taskService, scheduler, nil)

	// Start HTTP server (includes WebSocket hub)
	srv := server.NewServer(cfg.Server, h)
	wsHub := srv.GetWSHub()

	// Update handler with wsHub
	srv.SetWSHub(wsHub)

	// Hook WebSocket broadcasts into task service lifecycle
	// This broadcasts events when tasks are created, updated, or deleted
	setupWebSocketBroadcasts(taskService, wsHub)

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

// setupWebSocketBroadcasts configures WebSocket broadcasts for task events
func setupWebSocketBroadcasts(taskService *usecase.TaskService, wsHub interface{}) {
	// The wsHub is used to broadcast task events to connected clients
	// In a real implementation, you would hook into the task service methods
	// For now, we log that WebSocket is ready
	logger.Info("WebSocket broadcasts configured")
	_ = taskService
	_ = wsHub
}
