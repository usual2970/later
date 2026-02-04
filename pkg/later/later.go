package later

import (
	"context"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/usual2970/later/callback"
	"github.com/usual2970/later/domain/repository"
	"github.com/usual2970/later/infrastructure/circuitbreaker"
	"github.com/usual2970/later/infrastructure/worker"
	"github.com/usual2970/later/repository/mysql"
	tasksvc "github.com/usual2970/later/task"
)

// Later is the main struct that manages the task queue system
type Later struct {
	// Core components
	taskService     *tasksvc.Service
	scheduler       *tasksvc.Scheduler
	workerPool      worker.WorkerPool
	callbackService *callback.Service
	taskRepo        repository.TaskRepository

	// Database
	db      *sqlx.DB
	dbMode  DBMode
	closeDB bool // Close DB on shutdown if separate

	// Configuration
	config *Config
	logger *zap.Logger

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
	mu      sync.RWMutex
}

// New creates a new Later instance with functional options
func New(opts ...Option) (*Later, error) {
	// Apply default config
	cfg := &Config{
		DBMode:          DBModeSeparate,
		WorkerPoolSize:  20,
		AutoMigration:   true,
		RoutePrefix:     "/api/v1",
		CallbackTimeout: 30 * time.Second,
		Logger:          zap.L(), // Use global logger
		SchedulerConfig: tasksvc.SchedulerConfig{
			HighPriorityInterval:   2 * time.Second,
			NormalPriorityInterval: 3 * time.Second,
			CleanupInterval:        30 * time.Second,
		},
	}

	// Apply user options
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("invalid option: %w", err)
		}
	}

	// Validate configuration
	if cfg.DBMode == DBModeShared && cfg.DB == nil {
		return nil, fmt.Errorf("shared DB mode requires DB connection")
	}
	if cfg.DBMode == DBModeSeparate && cfg.DSN == "" {
		return nil, fmt.Errorf("separate DB mode requires DSN")
	}

	// Initialize Later instance
	l := &Later{
		config: cfg,
		logger: cfg.Logger,
		dbMode: cfg.DBMode,
	}
	l.ctx, l.cancel = context.WithCancel(context.Background())

	// Setup database
	if err := l.setupDatabase(); err != nil {
		return nil, fmt.Errorf("database setup failed: %w", err)
	}

	// Run migrations
	if cfg.AutoMigration {
		if err := l.runMigrations(); err != nil {
			return nil, fmt.Errorf("migration failed: %w", err)
		}
	}

	// Initialize components
	if err := l.initComponents(); err != nil {
		return nil, fmt.Errorf("component initialization failed: %w", err)
	}

	l.logger.Info("Later initialized",
		zap.String("db_mode", modeToString(cfg.DBMode)),
		zap.Int("worker_pool_size", cfg.WorkerPoolSize),
		zap.String("route_prefix", cfg.RoutePrefix),
	)

	return l, nil
}

func (l *Later) setupDatabase() error {
	if l.config.DBMode == DBModeShared {
		// Use existing connection
		l.db = l.config.DB
		l.closeDB = false
		l.logger.Info("Using shared database connection")
	} else {
		// Create separate connection
		l.logger.Info("Creating separate database connection",
			zap.String("dsn", maskDSN(l.config.DSN)),
		)

		db, err := sqlx.Connect("mysql", l.config.DSN)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}

		// Configure connection pool if options provided
		if l.config.DBConfig.MaxOpenConns > 0 {
			db.SetMaxOpenConns(l.config.DBConfig.MaxOpenConns)
		}
		if l.config.DBConfig.MaxIdleConns > 0 {
			db.SetMaxIdleConns(l.config.DBConfig.MaxIdleConns)
		}
		if l.config.DBConfig.MaxLifetime > 0 {
			db.SetConnMaxLifetime(l.config.DBConfig.MaxLifetime)
		}
		if l.config.DBConfig.MaxIdleTime > 0 {
			db.SetConnMaxIdleTime(l.config.DBConfig.MaxIdleTime)
		}

		l.db = db
		l.closeDB = true

		// Verify connection
		if err := l.db.Ping(); err != nil {
			return fmt.Errorf("failed to ping database: %w", err)
		}

		l.logger.Info("Separate database connection established")
	}
	return nil
}

func (l *Later) runMigrations() error {
	l.logger.Info("Running database migrations")
	// Use existing migration logic from repository/mysql/
	// Note: RunMigrations expects a migrationsDir parameter
	// For now, we'll use the default migrations directory
	err := mysql.RunMigrations(l.db, "migrations")
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	l.logger.Info("Database migrations completed successfully")
	return nil
}

func (l *Later) initComponents() error {
	l.logger.Info("Initializing components")

	// Circuit breaker
	cb := circuitbreaker.NewCircuitBreaker(5, 60*time.Second)

	// Callback service
	l.callbackService = callback.NewService(
		l.config.CallbackTimeout,
		cb,
		l.config.CallbackSecret,
		l.logger.Named("callback"),
	)

	// Repository
	l.taskRepo = mysql.NewTaskRepository(l.db)

	// Task service
	l.taskService = tasksvc.NewService(l.taskRepo)

	// Worker pool
	l.workerPool = worker.NewWorkerPool(
		l.config.WorkerPoolSize,
		l.taskService,
		l.callbackService,
		l.logger.Named("worker"),
	)

	// Scheduler
	l.scheduler = tasksvc.NewScheduler(
		l.taskRepo,
		l.workerPool,
		l.config.SchedulerConfig,
	)

	l.logger.Info("Components initialized successfully")
	return nil
}

// modeToString converts DBMode to string for logging
func modeToString(mode DBMode) string {
	switch mode {
	case DBModeShared:
		return "shared"
	case DBModeSeparate:
		return "separate"
	default:
		return "unknown"
	}
}

// maskDSN masks sensitive information in DSN for logging
func maskDSN(dsn string) string {
	// Simple masking - just show first part
	if len(dsn) > 20 {
		return dsn[:20] + "***"
	}
	return "***"
}
