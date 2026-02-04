package server

import (
	"context"
	"log"
	"net/http"
	"time"
	"later/configs"
	"later/delivery/rest"
	"later/delivery/rest/middleware"

	"github.com/gin-gonic/gin"
)

// Server wraps the gin engine
type Server struct {
	engine *gin.Engine
	config configs.ServerConfig
	handler *rest.Handler
	httpServer *http.Server
}

// NewServer creates a new HTTP server
func NewServer(cfg configs.ServerConfig, h *rest.Handler) *Server {
	engine := gin.New()

	// Add middleware
	engine.Use(middleware.Logger())
	engine.Use(middleware.Recovery())
	engine.Use(middleware.CORS())

	s := &Server{
		engine: engine,
		config: cfg,
		handler: h,
	}

	// Register routes
	s.registerRoutes(engine, h)

	return s
}

// registerRoutes sets up all API routes
func (s *Server) registerRoutes(engine *gin.Engine, h *rest.Handler) {
	// Health check
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	// API v1 routes
	v1 := engine.Group("/api/v1")
	{
		// Task routes
		v1.POST("/tasks", h.CreateTask)
		v1.GET("/tasks", h.ListTasks)
		v1.GET("/tasks/:id", h.GetTask)
		v1.DELETE("/tasks/:id", h.CancelTask)
		v1.POST("/tasks/:id/retry", h.RetryTask)
		v1.POST("/tasks/:id/resurrect", h.ResurrectTask)

		// Statistics
		v1.GET("/tasks/stats", h.GetStats)
	}
}

// ListenAndServe starts the HTTP server
func (s *Server) ListenAndServe() error {
	s.httpServer = &http.Server{
		Addr:    s.config.Address(),
		Handler: s.engine,
	}

	log.Printf("Starting HTTP server on %s", s.config.Address())
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down HTTP server...")
	return s.httpServer.Shutdown(ctx)
}
