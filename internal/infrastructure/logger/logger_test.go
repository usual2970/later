package logger

import (
	"os"
	"sync"
	"testing"

	"go.uber.org/zap"
)

// TestLoggerInitialization tests logger initialization with different configurations
func TestLoggerInitialization(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		level       string
	}{
		{
			name:        "Development environment",
			environment: "development",
			level:       "debug",
		},
		{
			name:        "Testing environment",
			environment: "testing",
			level:       "debug",
		},
		{
			name:        "Production environment",
			environment: "production",
			level:       "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Environment: tt.environment,
				Level:       tt.level,
				Filename:    "/tmp/test.log",
				MaxSize:     1,
				MaxBackups:  1,
				MaxAge:     1,
				Compress:   false,
			}

			err := Init(cfg)
			if err != nil {
				t.Fatalf("Failed to initialize logger: %v", err)
			}

			// Test logging at different levels
			Debug("Debug message", zap.String("test", "value"))
			Info("Info message", zap.String("test", "value"))
			Warn("Warning message", zap.String("test", "value"))
			Error("Error message", zap.String("test", "value"))

			// Clean up
			if globalLogger != nil {
				globalLogger.Sync()
			}
			globalLogger = nil
			once = *new(sync.Once)
		})
	}
}

// TestNamedLogger tests named logger functionality
func TestNamedLogger(t *testing.T) {
	cfg := DefaultConfig("testing")
	if err := Init(cfg); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if globalLogger != nil {
			globalLogger.Sync()
		}
		globalLogger = nil
		once = *new(sync.Once)
	}()

	serviceLog := Named("TestService")
	serviceLog.Info("Service started")

	componentLog := serviceLog.Named("Component")
	componentLog.Info("Component started")
}

// TestLoggerWithFields tests logger with additional fields
func TestLoggerWithFields(t *testing.T) {
	cfg := DefaultConfig("testing")
	if err := Init(cfg); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if globalLogger != nil {
			globalLogger.Sync()
		}
		globalLogger = nil
		once = *new(sync.Once)
	}()

	log := With(
		zap.String("request_id", "test-123"),
		zap.String("user_id", "user-456"),
	)

	log.Info("Processing request", zap.Int("attempt", 1))
	log.Info("Request completed")
}

// TestInitFromEnv tests initialization from environment variables
func TestInitFromEnv(t *testing.T) {
	// Save original environment values
	originalEnv := os.Getenv("APP_ENV")
	originalLevel := os.Getenv("LOG_LEVEL")

	// Test with environment variables
	tests := []struct {
		name     string
		env      string
		level    string
		expected string
	}{
		{
			name:     "Development",
			env:      "development",
			level:    "debug",
			expected: "development",
		},
		{
			name:     "Production",
			env:      "production",
			level:    "info",
			expected: "production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("APP_ENV", tt.env)
			os.Setenv("LOG_LEVEL", tt.level)

			err := InitFromEnv()
			if err != nil {
				t.Fatalf("InitFromEnv failed: %v", err)
			}

			// Verify logger is initialized
			if globalLogger == nil {
				t.Error("Global logger should not be nil after InitFromEnv")
			}

			// Clean up
			if globalLogger != nil {
				globalLogger.Sync()
			}
			globalLogger = nil
			once = *new(sync.Once)
		})
	}

	// Restore original environment values
	if originalEnv != "" {
		os.Setenv("APP_ENV", originalEnv)
	} else {
		os.Unsetenv("APP_ENV")
	}
	if originalLevel != "" {
		os.Setenv("LOG_LEVEL", originalLevel)
	} else {
		os.Unsetenv("LOG_LEVEL")
	}
}

// Example_basicUsage demonstrates basic logger usage
func Example_basicUsage() {
	// Initialize logger from environment
	if err := InitFromEnv(); err != nil {
		panic(err)
	}
	defer Sync()

	// Simple logging
	Info("Application started")
	Error("An error occurred", zap.Error(nil))
}

// Example_namedLogger demonstrates named logger usage
func Example_namedLogger() {
	cfg := DefaultConfig("development")
	if err := Init(cfg); err != nil {
		panic(err)
	}
	defer Sync()

	// Create named logger
	serviceLog := Named("UserService")
	serviceLog.Info("User created", zap.String("user_id", "123"))
}

// Example_withFields demonstrates logger with structured fields
func Example_withFields() {
	cfg := DefaultConfig("development")
	if err := Init(cfg); err != nil {
		panic(err)
	}
	defer Sync()

	// Create logger with fixed fields
	requestLog := With(
		zap.String("request_id", "abc-123"),
		zap.String("user_id", "user-456"),
	)

	requestLog.Info("Processing request")
	requestLog.Info("Request completed", zap.Int("duration_ms", 150))
}
