package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"

	"later/pkg/later"
)

func main() {
	// This example demonstrates how to embed Later in your application
	// For demonstration purposes, we're using a mock database connection
	// In a real application, you would provide your actual database connection

	log.Println("Later Embedded Example")
	log.Println("=======================")

	// For this example to work, you need a MySQL database running
	// Example DSN: "user:password@tcp(localhost:3306)/dbname?parseTime=true"

	// Check if DATABASE_URL is set
	dsn := os.Getenv("DATABASE_URL")
	dsn = "clipmate:123456!Aa@tcp(rm-bp16l0io7uman87025o.mysql.rds.aliyuncs.com:3306)/later?parseTime=true&loc=UTC&charset=utf8mb4"
	if dsn == "" {
		log.Println("DATABASE_URL not set, using default for demo")
		log.Println("To run this example:")
		log.Println("  export DATABASE_URL='user:pass@tcp(localhost:3306)/later?parseTime=true'")
		log.Println("  go run cmd/embedded-example/main.go")
		os.Exit(1)
	}

	// Initialize Later with separate database
	// This will create its own database connection
	laterSDK, err := later.New(
		later.WithSeparateDB(dsn),
		later.WithRoutePrefix("/internal/tasks"),
		later.WithWorkerPoolSize(10),   // Smaller pool for demo
		later.WithAutoMigration(false), // Automatically run migrations
	)
	if err != nil {
		log.Fatalf("Failed to initialize Later: %v", err)
	}

	log.Println("Later initialized successfully")

	// Start Later (starts scheduler and workers)
	if err := laterSDK.Start(); err != nil {
		log.Fatalf("Failed to start Later: %v", err)
	}

	log.Println("Later started successfully")

	// Setup Gin router
	router := gin.Default()

	// Add a simple health check endpoint that includes Later's status
	router.GET("/health", func(c *gin.Context) {
		status := laterSDK.HealthCheck()
		c.JSON(http.StatusOK, gin.H{
			"app":   "ok",
			"later": status,
		})
	})

	// Add a simple endpoint to create tasks
	router.POST("/tasks", func(c *gin.Context) {
		var req struct {
			Name        string `json:"name"`
			Payload     string `json:"payload"`
			CallbackURL string `json:"callback_url"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Create task using Later's API
		task, err := laterSDK.CreateTask(c.Request.Context(), &later.CreateTaskRequest{
			Name:        req.Name,
			Payload:     []byte(req.Payload),
			CallbackURL: req.CallbackURL,
			ScheduledAt: time.Now().Add(1 * time.Minute),
			Priority:    5,
			MaxRetries:  3,
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"task_id":      task.ID,
			"status":       task.Status,
			"scheduled_at": task.ScheduledAt,
		})
	})

	// Add endpoint to get task status
	router.GET("/tasks/:id", func(c *gin.Context) {
		id := c.Param("id")

		task, err := laterSDK.GetTask(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":           task.ID,
			"name":         task.Name,
			"status":       task.Status,
			"created_at":   task.CreatedAt,
			"scheduled_at": task.ScheduledAt,
		})
	})

	// Add endpoint to get metrics
	router.GET("/metrics", func(c *gin.Context) {
		metrics := laterSDK.GetMetrics()
		c.JSON(http.StatusOK, metrics)
	})

	// Add endpoint to get stats
	router.GET("/stats", func(c *gin.Context) {
		stats, err := laterSDK.GetStats(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stats)
	})

	// Start HTTP server
	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Start server in background
	go func() {
		log.Println("Server started on :8080")
		log.Println("Endpoints:")
		log.Println("  GET    /health              - Health check")
		log.Println("  POST   /tasks               - Create task")
		log.Println("  GET    /tasks/:id           - Get task status")
		log.Println("  GET    /metrics             - Get metrics")
		log.Println("  GET    /stats               - Get statistics")
		log.Println("")
		log.Println("Example: curl -X POST http://localhost:8080/tasks \\")
		log.Println("  -H 'Content-Type: application/json' \\")
		log.Println("  -d '{\"name\":\"test-task\",\"payload\":\"{\\\"test\\\":true}\",\"callback_url\":\"http://example.com/callback\"}'")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("")
	log.Println("Shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown Later first (stops scheduler and workers)
	if err := laterSDK.Shutdown(ctx); err != nil {
		log.Printf("Later shutdown error: %v", err)
	}

	// Shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
