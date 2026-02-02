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
	"later/internal/domain/models"
)

func main() {
	// Load configuration
	cfg := configs.LoadConfig()

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

	// Initialize worker pool
	workerPool := make(chan *models.Task, cfg.Worker.PoolSize)

	// Initialize use cases
	taskService := usecase.NewTaskService(taskRepo)

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

	log.Println("Server stopped")
}
