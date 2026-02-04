package response

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AppError defines the interface for application errors
type AppError interface {
	error
	Code() string
	HTTPStatus() int
}

// HTTPError implements AppError interface for HTTP errors
type HTTPError struct {
	code       string
	message    string
	httpStatus int
}

// NewError creates a new HTTPError
func NewError(code string, message string, httpStatus int) *HTTPError {
	return &HTTPError{
		code:       code,
		message:    message,
		httpStatus: httpStatus,
	}
}

func (e *HTTPError) Error() string {
	return e.message
}

func (e *HTTPError) Code() string {
	return e.code
}

func (e *HTTPError) HTTPStatus() int {
	return e.httpStatus
}

// Common errors
var (
	ErrBadRequest = &HTTPError{"bad_request", "Bad request", http.StatusBadRequest}
	ErrNotFound   = &HTTPError{"not_found", "Resource not found", http.StatusNotFound}
	ErrInternal   = &HTTPError{"internal_error", "Internal server error", http.StatusInternalServerError}
)

// Success sends a successful JSON response with status 200
func Success(c *gin.Context, data interface{}) {
	log.Printf("[DEBUG] Success: sending %T", data)
	c.JSON(http.StatusOK, data)
}

// Error sends an error response based on the error type
func Error(c *gin.Context, err error) {
	var httpErr AppError
	if e, ok := err.(AppError); ok {
		httpErr = e
	} else {
		// Convert regular error to internal server error
		httpErr = &HTTPError{
			code:       "internal_error",
			message:    err.Error(),
			httpStatus: http.StatusInternalServerError,
		}
	}

	log.Printf("[ERROR] %s: %s - %s", httpErr.Code(), httpErr.Error(), c.Request.URL.Path)

	c.JSON(httpErr.HTTPStatus(), gin.H{
		"error":   httpErr.Code(),
		"message": httpErr.Error(),
	})
}

// ErrorWithMessage sends an error response with a custom message
func ErrorWithMessage(c *gin.Context, httpStatus int, code string, message string) {
	log.Printf("[ERROR] %s: %s - %s", code, message, c.Request.URL.Path)

	c.JSON(httpStatus, gin.H{
		"error":   code,
		"message": message,
	})
}

// NoContent sends a 204 No Content response
func NoContent(c *gin.Context) {
	c.AbortWithStatus(http.StatusNoContent)
}

// Created sends a 201 Created response
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, data)
}

// Accepted sends a 202 Accepted response
func Accepted(c *gin.Context, data interface{}) {
	c.JSON(http.StatusAccepted, data)
}
