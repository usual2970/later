package callback

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"later/domain/entity"
	"later/infrastructure/circuitbreaker"

	"go.uber.org/zap"
)

// Service handles HTTP callback delivery
type Service struct {
	client           *http.Client
	circuitBreaker   *circuitbreaker.CircuitBreaker
	signingSecret    string
	logger           *zap.Logger
}

// NewService creates a new callback service
func NewService(
	timeout time.Duration,
	circuitBreaker *circuitbreaker.CircuitBreaker,
	signingSecret string,
	logger *zap.Logger,
) *Service {
	return &Service{
		client:         &http.Client{Timeout: timeout},
		circuitBreaker: circuitBreaker,
		signingSecret:  signingSecret,
		logger:         logger,
	}
}

// DeliverCallback delivers a callback to the task's callback URL
func (s *Service) DeliverCallback(ctx context.Context, task *entity.Task) error {
	// Check circuit breaker
	if s.circuitBreaker != nil && s.circuitBreaker.IsOpen(task.CallbackURL) {
		return fmt.Errorf("circuit breaker is open for URL: %s", task.CallbackURL)
	}

	// Execute callback via circuit breaker
	if s.circuitBreaker != nil {
		return s.circuitBreaker.Execute(task.CallbackURL, func() error {
			return s.deliverHTTPCallback(ctx, task)
		})
	}

	return s.deliverHTTPCallback(ctx, task)
}

// deliverHTTPCallback performs the actual HTTP POST
func (s *Service) deliverHTTPCallback(ctx context.Context, task *entity.Task) error {
	// Create request
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		task.CallbackURL,
		bytes.NewReader(task.Payload),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Task-ID", task.ID)
	req.Header.Set("X-Task-Name", task.Name)
	req.Header.Set("X-Retry-Count", fmt.Sprintf("%d", task.RetryCount))

	// Add signature if secret is configured
	if s.signingSecret != "" {
		signature := s.generateSignature(task.Payload)
		req.Header.Set("X-Signature", signature)
	}

	// Execute request
	startTime := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)

	// Log callback attempt
	s.logger.Info("Callback delivered",
		zap.String("task_id", task.ID),
		zap.String("callback_url", task.CallbackURL),
		zap.Int("status_code", resp.StatusCode),
		zap.Duration("duration", duration),
	)

	// Classify response
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Success
		return s.handleSuccess(task)
	} else if resp.StatusCode >= 500 || resp.StatusCode == 429 {
		// Server error or rate limit - retry
		return s.handleRetry(task, fmt.Errorf("callback returned status %d", resp.StatusCode))
	} else {
		// Client error - don't retry
		return s.handleFailure(task, fmt.Errorf("callback returned status %d", resp.StatusCode))
	}
}

// handleSuccess marks task as completed
func (s *Service) handleSuccess(task *entity.Task) error {
	task.MarkAsCompleted()
	task.CallbackAttempts++
	status := 200
	task.LastCallbackStatus = &status
	now := time.Now()
	task.LastCallbackAt = &now

	s.logger.Info("Task completed successfully",
		zap.String("task_id", task.ID),
		zap.Int("callback_attempts", task.CallbackAttempts))

	return nil
}

// handleRetry returns an error to trigger retry logic
func (s *Service) handleRetry(task *entity.Task, err error) error {
	task.CallbackAttempts++
	status := 500
	task.LastCallbackStatus = &status
	now := time.Now()
	task.LastCallbackAt = &now
	errMsg := err.Error()
	task.LastCallbackError = &errMsg

	s.logger.Warn("Task callback failed, will retry",
		zap.String("task_id", task.ID),
		zap.Int("callback_attempts", task.CallbackAttempts),
		zap.Error(err))

	return err
}

// handleFailure marks task as permanently failed
func (s *Service) handleFailure(task *entity.Task, err error) error {
	task.MarkAsFailed(err)
	task.CallbackAttempts++
	status := 400
	task.LastCallbackStatus = &status
	now := time.Now()
	task.LastCallbackAt = &now
	errMsg := err.Error()
	task.LastCallbackError = &errMsg

	s.logger.Error("Task failed permanently",
		zap.String("task_id", task.ID),
		zap.Int("callback_attempts", task.CallbackAttempts),
		zap.Error(err))

	return err
}

// generateSignature creates an HMAC signature for the payload
func (s *Service) generateSignature(payload []byte) string {
	h := hmac.New(sha256.New, []byte(s.signingSecret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}
