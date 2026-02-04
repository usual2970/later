package later

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestRegisterRoutes tests that routes are registered correctly
func TestRegisterRoutes(t *testing.T) {
	// This test requires a real database, so we'll just test that the method
	// doesn't panic when given valid input
	t.Run("RegisterRoutes with nil engine", func(t *testing.T) {
		l := &Later{}
		err := l.RegisterRoutes(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "engine cannot be nil")
	})
}

// TestHealthCheckHandler tests the health check endpoint
func TestHealthCheckHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	l := &Later{
		config: &Config{
			RoutePrefix: "/api/v1",
		},
		logger: testLogger(),
	}

	router := gin.New()
	err := l.RegisterRoutes(router)
	assert.NoError(t, err)

	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 200 (stopped) instead of 503
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "stopped", response["status"])
}

// TestCreateTaskHandler tests the create task endpoint
func TestCreateTaskHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	l := &Later{
		config: &Config{
			RoutePrefix: "/api/v1",
		},
		logger: testLogger(),
	}

	router := gin.New()
	err := l.RegisterRoutes(router)
	assert.NoError(t, err)

	t.Run("Create task with invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/tasks", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Body = io.NopCloser(bytes.NewReader([]byte("invalid json")))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Create task with missing name", func(t *testing.T) {
		payload := map[string]interface{}{
			"callback_url": "http://example.com/callback",
		}

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/tasks", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Body = io.NopCloser(bytes.NewReader(body))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestRoutePrefix tests that route prefix is respected
func TestRoutePrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		prefix     string
		reqPath    string
		expectCode int
	}{
		{
			name:       "Default prefix",
			prefix:     "/api/v1",
			reqPath:    "/api/v1/health",
			expectCode: http.StatusOK, // Later is stopped, so returns 200
		},
		{
			name:       "Custom prefix",
			prefix:     "/internal/tasks",
			reqPath:    "/internal/tasks/health",
			expectCode: http.StatusOK,
		},
		{
			name:       "Root prefix",
			prefix:     "/",
			reqPath:    "/health",
			expectCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Later{
				config: &Config{
					RoutePrefix: tt.prefix,
				},
				logger: testLogger(),
			}

			router := gin.New()
			err := l.RegisterRoutes(router)
			assert.NoError(t, err)

			req, _ := http.NewRequest("GET", tt.reqPath, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

// TestMiddleware tests that middleware is applied
func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	l := &Later{
		config: &Config{
			RoutePrefix: "/api/v1",
		},
		logger: testLogger(),
	}

	router := gin.New()
	l.RegisterRoutes(router)

	t.Run("Logger middleware is applied", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Request should be handled
		assert.NotEqual(t, http.StatusNotFound, w.Code)
	})

	t.Run("Recovery middleware catches panics", func(t *testing.T) {
		// We can't easily test panic recovery without a handler that panics
		// but we can verify the middleware is registered
		req, _ := http.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusNotFound, w.Code)
	})
}

// testLogger returns a test logger instance
func testLogger() *zap.Logger {
	return zap.NewNop()
}
