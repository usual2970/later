package later

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	tasksvc "github.com/usual2970/later/task"
)

// Option is a function that configures a Later instance
type Option func(*Config) error

// DBOption is a function that configures database settings
type DBOption func(*DatabaseConfig) error

// Config holds all configuration for a Later instance
type Config struct {
	// Database
	DB            *sqlx.DB
	DBMode        DBMode
	DSN           string
	DBConfig      DatabaseConfig
	AutoMigration bool

	// HTTP
	RoutePrefix string

	// Worker Pool
	WorkerPoolSize int

	// Scheduler
	SchedulerConfig tasksvc.SchedulerConfig

	// Callback
	CallbackTimeout time.Duration
	CallbackSecret  string

	// Logging
	Logger *zap.Logger
}

// DatabaseConfig holds database-specific configuration
type DatabaseConfig struct {
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
	MaxIdleTime  time.Duration
}

// DBMode represents the database connection mode
type DBMode int

const (
	// DBModeShared means Later uses an existing database connection provided by the app
	DBModeShared DBMode = iota

	// DBModeSeparate means Later creates and manages its own database connection
	DBModeSeparate
)

// WithSharedDB configures Later to use an existing database connection
// The connection will not be closed by Later.Shutdown()
func WithSharedDB(db *sqlx.DB) Option {
	return func(c *Config) error {
		if db == nil {
			return fmt.Errorf("database connection cannot be nil")
		}
		c.DB = db
		c.DBMode = DBModeShared
		return nil
	}
}

// WithSeparateDB configures Later to create its own database connection
// The connection will be closed by Later.Shutdown()
func WithSeparateDB(dsn string, opts ...DBOption) Option {
	return func(c *Config) error {
		if dsn == "" {
			return fmt.Errorf("DSN cannot be empty")
		}
		c.DSN = dsn
		c.DBMode = DBModeSeparate

		// Apply database options
		for _, opt := range opts {
			if err := opt(&c.DBConfig); err != nil {
				return fmt.Errorf("database option error: %w", err)
			}
		}

		return nil
	}
}

// WithMaxConnections sets the maximum number of open database connections
func WithMaxConnections(max int) DBOption {
	return func(c *DatabaseConfig) error {
		if max <= 0 {
			return fmt.Errorf("max connections must be positive")
		}
		c.MaxOpenConns = max
		return nil
	}
}

// WithMaxIdleConnections sets the maximum number of idle database connections
func WithMaxIdleConnections(max int) DBOption {
	return func(c *DatabaseConfig) error {
		if max < 0 {
			return fmt.Errorf("max idle connections cannot be negative")
		}
		c.MaxIdleConns = max
		return nil
	}
}

// WithConnectionMaxLifetime sets the maximum lifetime of a database connection
func WithConnectionMaxLifetime(lifetime time.Duration) DBOption {
	return func(c *DatabaseConfig) error {
		if lifetime < 0 {
			return fmt.Errorf("connection max lifetime cannot be negative")
		}
		c.MaxLifetime = lifetime
		return nil
	}
}

// WithConnectionMaxIdleTime sets the maximum idle time of a database connection
func WithConnectionMaxIdleTime(idleTime time.Duration) DBOption {
	return func(c *DatabaseConfig) error {
		if idleTime < 0 {
			return fmt.Errorf("connection max idle time cannot be negative")
		}
		c.MaxIdleTime = idleTime
		return nil
	}
}

// WithRoutePrefix sets the HTTP route prefix for Later's endpoints
// Defaults to "/api/v1"
func WithRoutePrefix(prefix string) Option {
	return func(c *Config) error {
		if prefix == "" {
			return fmt.Errorf("route prefix cannot be empty")
		}
		c.RoutePrefix = prefix
		return nil
	}
}

// WithWorkerPoolSize sets the number of worker pool workers
// Defaults to 20
func WithWorkerPoolSize(size int) Option {
	return func(c *Config) error {
		if size <= 0 {
			return fmt.Errorf("worker pool size must be positive")
		}
		c.WorkerPoolSize = size
		return nil
	}
}

// WithLogger sets a custom logger for Later
// Defaults to global zap logger
func WithLogger(logger *zap.Logger) Option {
	return func(c *Config) error {
		if logger == nil {
			return fmt.Errorf("logger cannot be nil")
		}
		c.Logger = logger
		return nil
	}
}

// WithAutoMigration enables or disables automatic database migration on initialization
// Defaults to true
func WithAutoMigration(enabled bool) Option {
	return func(c *Config) error {
		c.AutoMigration = enabled
		return nil
	}
}

// WithSchedulerIntervals configures the tiered polling intervals
// high: interval for high-priority tasks (priority > 5)
// normal: interval for normal-priority tasks (priority >= 0)
// cleanup: interval for cleanup jobs
func WithSchedulerIntervals(high, normal, cleanup time.Duration) Option {
	return func(c *Config) error {
		if high <= 0 || normal <= 0 || cleanup <= 0 {
			return fmt.Errorf("scheduler intervals must be positive")
		}
		c.SchedulerConfig.HighPriorityInterval = high
		c.SchedulerConfig.NormalPriorityInterval = normal
		c.SchedulerConfig.CleanupInterval = cleanup
		return nil
	}
}

// WithCallbackTimeout sets the HTTP timeout for callback delivery
// Defaults to 30 seconds
func WithCallbackTimeout(timeout time.Duration) Option {
	return func(c *Config) error {
		if timeout <= 0 {
			return fmt.Errorf("callback timeout must be positive")
		}
		c.CallbackTimeout = timeout
		return nil
	}
}

// WithCallbackSecret sets the HMAC secret for callback signature validation
// If set, callbacks will include X-Signature header
func WithCallbackSecret(secret string) Option {
	return func(c *Config) error {
		c.CallbackSecret = secret
		return nil
	}
}
