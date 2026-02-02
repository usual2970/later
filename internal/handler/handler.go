package handler

import (
	"later/internal/usecase"
)

// Handler handles HTTP requests
type Handler struct {
	taskService *usecase.TaskService
	scheduler   *usecase.Scheduler
}

// NewHandler creates a new HTTP handler
func NewHandler(taskService *usecase.TaskService, scheduler *usecase.Scheduler) *Handler {
	return &Handler{
		taskService: taskService,
		scheduler:   scheduler,
	}
}
