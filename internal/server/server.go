package server

import (
	"context"
	"log"
	"net/http"
	"later/configs"
	"later/internal/handler"

	"github.com/gin-gonic/gin"
)

// Server wraps the gin engine
type Server struct {
	engine *gin.Engine
	config configs.ServerConfig
	handler *handler.Handler
	httpServer *http.Server
}

// NewServer creates a new HTTP server
func NewServer(cfg configs.ServerConfig, h *handler.Handler) *Server {
	engine := gin.Default()

	// TODO: Register routes here

	return &Server{
		engine: engine,
		config: cfg,
		handler: h,
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
