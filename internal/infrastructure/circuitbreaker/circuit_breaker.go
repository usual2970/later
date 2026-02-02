package circuitbreaker

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State string

const (
	StateClosed    State = "closed"
	StateOpen      State = "open"
	StateHalfOpen  State = "half-open"
)

// CircuitBreaker implements the circuit breaker pattern for callback URLs
type CircuitBreaker struct {
	maxFailures     int
	resetTimeout    time.Duration
	halfOpenTimeout time.Duration

	failures    map[string]int
	lastFailure map[string]time.Time
	state       map[string]State
	mu          sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:     maxFailures,
		resetTimeout:    resetTimeout,
		halfOpenTimeout: resetTimeout / 2, // Half-open timeout is half of reset timeout
		failures:        make(map[string]int),
		lastFailure:     make(map[string]time.Time),
		state:           make(map[string]State),
	}
}

// IsOpen checks if the circuit is open for a given URL
func (cb *CircuitBreaker) IsOpen(url string) bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	state, exists := cb.state[url]
	if !exists {
		return false // Circuit is closed by default
	}

	if state == StateClosed {
		return false
	}

	if state == StateOpen {
		// Check if we should transition to half-open
		lastFail, ok := cb.lastFailure[url]
		if ok && time.Since(lastFail) > cb.resetTimeout {
			// Transition to half-open
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state[url] = StateHalfOpen
			cb.mu.Unlock()
			cb.mu.RLock()
			log.Printf("Circuit breaker transitioning to half-open for URL: %s", url)
			return false // Allow request in half-open state
		}
		return true
	}

	// Half-open state - allow request
	return false
}

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(url string, fn func() error) error {
	// Check if circuit is open
	if cb.IsOpen(url) {
		return fmt.Errorf("circuit breaker is open for URL: %s", url)
	}

	// Execute the function
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure(url)
	} else {
		cb.recordSuccess(url)
	}

	return err
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess(url string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.recordSuccess(url)
}

// recordSuccess (internal, must be called with lock held)
func (cb *CircuitBreaker) recordSuccess(url string) {
	// Reset failure count
	delete(cb.failures, url)
	delete(cb.lastFailure, url)

	// If in half-open, transition to closed
	if cb.state[url] == StateHalfOpen {
		cb.state[url] = StateClosed
		log.Printf("Circuit breaker closed for URL: %s (half-open test successful)", url)
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure(url string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.recordFailure(url)
}

// recordFailure (internal, must be called with lock held)
func (cb *CircuitBreaker) recordFailure(url string) {
	cb.failures[url]++
	now := time.Now()
	cb.lastFailure[url] = now

	// Check if we should open the circuit
	if cb.failures[url] >= cb.maxFailures {
		// If in half-open or closed, open the circuit
		cb.state[url] = StateOpen
		log.Printf("Circuit breaker opened for URL: %s (failures: %d)", url, cb.failures[url])
	}
}

// GetState returns the current state for a URL
func (cb *CircuitBreaker) GetState(url string) State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if state, ok := cb.state[url]; ok {
		return state
	}
	return StateClosed
}

// GetFailureCount returns the current failure count for a URL
func (cb *CircuitBreaker) GetFailureCount(url string) int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return cb.failures[url]
}

// Reset resets the circuit breaker for a URL
func (cb *CircuitBreaker) Reset(url string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	delete(cb.failures, url)
	delete(cb.lastFailure, url)
	delete(cb.state, url)

	log.Printf("Circuit breaker reset for URL: %s", url)
}
