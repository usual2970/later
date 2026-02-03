package server

import (
	"context"
	"log"
	"net/http"
	"time"
	"later/configs"
	"later/internal/handler"
	"later/internal/middleware"
	"later/internal/websocket"

	"github.com/gin-gonic/gin"
)

// Server wraps the gin engine
type Server struct {
	engine *gin.Engine
	config configs.ServerConfig
	handler *handler.Handler
	wsHub   *websocket.Hub
	httpServer *http.Server
}

// NewServer creates a new HTTP server
func NewServer(cfg configs.ServerConfig, h *handler.Handler) *Server {
	engine := gin.New()

	// Add middleware
	engine.Use(middleware.Logger())
	engine.Use(middleware.Recovery())
	engine.Use(middleware.CORS())

	// Create WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()

	s := &Server{
		engine: engine,
		config: cfg,
		handler: h,
		wsHub:   wsHub,
	}

	// Register routes
	s.registerRoutes(engine, h, wsHub)

	return s
}

// registerRoutes sets up all API routes
func (s *Server) registerRoutes(engine *gin.Engine, h *handler.Handler, wsHub *websocket.Hub) {
	// Health check
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"timestamp": time.Now().Format(time.RFC3339),
			"websocket_clients": wsHub.GetClientCount(),
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

		// WebSocket stream
		v1.GET("/tasks/stream", func(c *gin.Context) {
			wsHub.HandleWebSocket(c)
		})
	}
}

// ListenAndServe starts the HTTP server
func (s *Server) ListenAndServe() error {
	s.httpServer = &http.Server{
		Addr:    s.config.Address(),
		Handler: s.engine,
	}

	log.Printf("Starting HTTP server on %s", s.config.Address)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down HTTP server...")
	return s.httpServer.Shutdown(ctx)
}

// GetWSHub returns the WebSocket hub (for use by other components)
func (s *Server) GetWSHub() *websocket.Hub {
	return s.wsHub
}

// SetWSHub updates the WebSocket hub in the handler (used after server creation)
func (s *Server) SetWSHub(wsHub *websocket.Hub) {
	s.wsHub = wsHub
	s.handler.SetWSHub(wsHub)
}
