---
title: feat: Transform Later into Embeddable Go Library/SDK
type: feat
date: 2026-02-04
---

# Transform Later into Embeddable Go Library/SDK

## Overview

Transform Later from a standalone service into a reusable Go library that other projects can embed directly. Projects should be able to integrate Later by providing a database connection and route prefix, gaining immediate access to async task scheduling capabilities without deploying a separate service.

**Target users**: Backend developers who want to add async task processing to their existing Go web applications without managing additional infrastructure.

## Motivation

### Current State
- Later runs as a standalone HTTP service
- Other projects must call Later via HTTP API
- Requires separate deployment, monitoring, and scaling
- Adds network latency and operational complexity

### Desired State
- Later can be embedded as a Go library
- Direct in-process task creation and execution
- Shared database for transactional consistency
- Optional standalone HTTP API mode still available
- Zero additional infrastructure for simple use cases

### User Benefits
- **Simplified operations**: One service to deploy instead of two
- **Better performance**: In-process task creation, no network overhead
- **Transactional consistency**: Create tasks within app transactions
- **Flexibility**: Choose embedded or standalone mode per use case
- **Reduced latency**: Microseconds instead of milliseconds for task submission

## Proposed Solution

### Hybrid Architecture

Support both **embedded library mode** and **standalone service mode**:

```
┌─────────────────────────────────────────────────────────────┐
│                      User Application                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────────────┐        ┌──────────────────────┐   │
│  │   Business Logic    │───────│  Later (Embedded)    │   │
│  │                     │  API   │                      │   │
│  │  - Gin routes       │        │  - Scheduler         │   │
│  │  - DB transaction   │        │  - Worker Pool       │   │
│  │                     │        │  - Task API          │   │
│  └─────────────────────┘        └──────────────────────┘   │
│                                         │                    │
│                                         │ Shared DB          │
│                                         ▼                    │
│                                    ┌─────────┐              │
│                                    │  MySQL  │              │
│                                    └─────────┘              │
└─────────────────────────────────────────────────────────────┘

OR (Standalone Mode)

┌──────────────────┐         ┌─────────────────────┐
│ Client Apps      │────────▶│  Later Service      │
│                  │  HTTP   │  (Standalone)       │
└──────────────────┘         └─────────────────────┘
                                      │
                                      │ Separate DB
                                      ▼
                                  ┌─────────┐
                                  │  MySQL  │
                                  └─────────┘
```

### Integration Levels

1. **Embedded SDK** (Primary focus)
   ```go
   import "github.com/usual2970/later/v2/pkg/later"

   // Initialize with shared DB
   later := later.NewEmbedded(
       later.WithSharedDB(db),
       later.WithRoutePrefix("/internal/tasks"),
       later.WithWorkerPoolSize(50),
   )

   // Register routes
   later.RegisterRoutes(engine)

   // Start background processing
   later.Start()
   defer later.Shutdown(ctx)
   ```

2. **Standalone Service** (Maintain compatibility)
   ```bash
   # Existing deployment model
   go run cmd/server/main.go
   ```

## Technical Approach

### Phase 1: Core Library API

**Goal**: Create `pkg/later/` package with clean initialization API.

#### Package Structure
```
pkg/later/
├── later.go              # Main Later struct + constructor
├── options.go            # Functional options pattern
├── task_api.go           # Public task API
├── server.go             # Optional HTTP server (embeddable)
├── lifecycle.go          # Start/Shutdown/HealthCheck
└── migrations.go         # Migration helpers

internal/                 # Keep existing structure
├── domain/
├── task/
├── callback/
├── infrastructure/
└── repository/mysql/

cmd/server/               # Keep standalone server
cmd/embedded-example/     # New: Example embedded usage
```

#### Initialization API

```go
// pkg/later/options.go
type Option func(*Config) error

// WithSharedDB configures Later to use existing database connection
func WithSharedDB(db *sqlx.DB) Option {
    return func(c *Config) error {
        c.DB = db
        c.DBMode = DBModeShared
        return nil
    }
}

// WithSeparateDB configures Later to create its own database connection
func WithSeparateDB(dsn string, opts ...DBOption) Option {
    return func(c *Config) error {
        c.DSN = dsn
        c.DBMode = DBModeSeparate
        for _, opt := range opts {
            if err := opt(&c.DBConfig); err != nil {
                return err
            }
        }
        return nil
    }
}

// WithRoutePrefix sets the HTTP route prefix
func WithRoutePrefix(prefix string) Option {
    return func(c *Config) error {
        c.RoutePrefix = prefix
        return nil
    }
}

// WithWorkerPoolSize sets the worker pool size
func WithWorkerPoolSize(size int) Option {
    return func(c *Config) error {
        if size <= 0 {
            return fmt.Errorf("worker pool size must be positive")
        }
        c.WorkerPoolSize = size
        return nil
    }
}

// WithLogger sets custom logger (defaults to zap global)
func WithLogger(logger *zap.Logger) Option {
    return func(c *Config) error {
        c.Logger = logger
        return nil
    }
}

// WithAutoMigration enables/disables automatic migration on init
func WithAutoMigration(enabled bool) Option {
    return func(c *Config) error {
        c.AutoMigration = enabled
        return nil
    }
}

// WithSchedulerIntervals configures tiered polling intervals
func WithSchedulerIntervals(high, normal, cleanup time.Duration) Option {
    return func(c *Config) error {
        c.SchedulerConfig.HighInterval = high
        c.SchedulerConfig.NormalInterval = normal
        c.SchedulerConfig.CleanupInterval = cleanup
        return nil
    }
}

// pkg/later/later.go
type Later struct {
    // Core components
    taskService    *task.Service
    scheduler      *task.Scheduler
    workerPool     worker.WorkerPool
    callbackService *callback.Service
    taskRepo       repository.TaskRepository

    // Database
    db             *sqlx.DB
    dbMode         DBMode
    closeDB        bool // Close DB on shutdown if separate

    // Configuration
    config         *Config
    logger         *zap.Logger

    // Lifecycle
    ctx            context.Context
    cancel         context.CancelFunc
    started        bool
    mu             sync.RWMutex
}

type DBMode int
const (
    DBModeShared DBMode = iota
    DBModeSeparate
)

type Config struct {
    // Database
    DB              *sqlx.DB
    DBMode          DBMode
    DSN             string
    DBConfig        DatabaseConfig
    AutoMigration   bool

    // HTTP
    RoutePrefix     string

    // Worker Pool
    WorkerPoolSize  int

    // Scheduler
    SchedulerConfig SchedulerConfig

    // Callback
    CallbackTimeout time.Duration
    CallbackSecret  string

    // Logging
    Logger          *zap.Logger
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
        SchedulerConfig: SchedulerConfig{
            HighInterval:    2 * time.Second,
            NormalInterval:  3 * time.Second,
            CleanupInterval: 30 * time.Second,
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
    } else {
        // Create separate connection
        db, err := sqlx.Connect("mysql", l.config.DSN)
        if err != nil {
            return fmt.Errorf("failed to connect to database: %w", err)
        }
        l.db = db
        l.closeDB = true
    }
    return nil
}

func (l *Later) runMigrations() error {
    l.logger.Info("Running database migrations")
    // Use existing migration logic from repository/mysql/
    return mysql.RunMigrations(l.db)
}

func (l *Later) initComponents() error {
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
    l.taskService = task.NewService(l.taskRepo)

    // Worker pool
    l.workerPool = worker.NewWorkerPool(
        l.config.WorkerPoolSize,
        l.taskService,
        l.callbackService,
        l.logger.Named("worker"),
    )

    // Scheduler
    l.scheduler = task.NewScheduler(
        l.taskRepo,
        l.workerPool,
        l.config.SchedulerConfig,
        l.logger.Named("scheduler"),
    )

    return nil
}
```

#### Lifecycle Management

```go
// pkg/later/lifecycle.go

// Start begins background processing (scheduler and workers)
// Must be called before creating/processing tasks
func (l *Later) Start() error {
    l.mu.Lock()
    defer l.mu.Unlock()

    if l.started {
        return fmt.Errorf("already started")
    }

    l.logger.Info("Starting Later")

    // Start worker pool
    l.workerPool.Start(l.config.WorkerPoolSize)

    // Start scheduler in background
    go l.scheduler.Start(l.ctx)

    l.started = true
    l.logger.Info("Later started successfully")
    return nil
}

// Shutdown gracefully stops Later
// Waits for in-flight tasks to complete or until context is cancelled
func (l *Later) Shutdown(ctx context.Context) error {
    l.mu.Lock()
    if !l.started {
        l.mu.Unlock()
        return nil
    }
    l.mu.Unlock()

    l.logger.Info("Shutting down Later")

    // Stop scheduler (stops polling)
    l.scheduler.Stop()

    // Stop worker pool (waits for in-flight tasks)
    if err := l.workerPool.Shutdown(ctx); err != nil {
        l.logger.Error("Worker pool shutdown error", zap.Error(err))
        return err
    }

    // Close database if we own it
    if l.closeDB && l.db != nil {
        if err := l.db.Close(); err != nil {
            l.logger.Error("Database close error", zap.Error(err))
            return err
        }
    }

    l.cancel()
    l.logger.Info("Later shutdown complete")
    return nil
}

// HealthCheck returns health status for monitoring
func (l *Later) HealthCheck() HealthStatus {
    l.mu.RLock()
    defer l.mu.RUnlock()

    status := HealthStatus{
        Started: l.started,
    }

    if !l.started {
        status.Status = "stopped"
        return status
    }

    // Check database
    if err := l.db.Ping(); err != nil {
        status.Status = "unhealthy"
        status.Database = "disconnected"
        status.Error = err.Error()
        return status
    }
    status.Database = "connected"

    // Check scheduler
    if l.scheduler.IsRunning() {
        status.Scheduler = "running"
    } else {
        status.Scheduler = "stopped"
    }

    // Check worker pool
    activeWorkers := l.workerPool.ActiveCount()
    status.Workers = &WorkerStatus{
        Active:  activeWorkers,
        Total:   l.config.WorkerPoolSize,
    }

    status.Status = "healthy"
    return status
}

type HealthStatus struct {
    Status    string       `json:"status"`     // healthy, unhealthy, stopped
    Database  string       `json:"database"`   // connected, disconnected
    Scheduler string       `json:"scheduler"`  // running, stopped
    Workers   *WorkerStatus `json:"workers,omitempty"`
    Started   bool         `json:"started"`
    Error     string       `json:"error,omitempty"`
}

type WorkerStatus struct {
    Active int `json:"active"`
    Total  int `json:"total"`
}
```

#### Task API

```go
// pkg/later/task_api.go

// CreateTask creates a new task
func (l *Later) CreateTask(ctx context.Context, req *CreateTaskRequest) (*entity.Task, error) {
    task := &entity.Task{
        ID:           uuid.New().String(),
        Name:         req.Name,
        Payload:      entity.JSONBytes(req.Payload),
        CallbackURL:  req.CallbackURL,
        ScheduledAt:  req.ScheduledAt,
        Priority:     req.Priority,
        MaxRetries:   req.MaxRetries,
        Tags:         req.Tags,
        Status:       entity.TaskStatusPending,
    }

    if err := l.taskService.CreateTask(ctx, task); err != nil {
        return nil, fmt.Errorf("failed to create task: %w", err)
    }

    // Submit immediately if due now
    if task.ShouldExecuteNow() {
        l.scheduler.SubmitTaskImmediately(task)
    }

    return task, nil
}

// CreateTaskInTx creates a task within an existing database transaction
// Returns error if tx is nil
func (l *Later) CreateTaskInTx(tx *sqlx.Tx, req *CreateTaskRequest) (*entity.Task, error) {
    if tx == nil {
        return nil, fmt.Errorf("transaction is required")
    }

    task := &entity.Task{
        ID:           uuid.New().String(),
        Name:         req.Name,
        Payload:      entity.JSONBytes(req.Payload),
        CallbackURL:  req.CallbackURL,
        ScheduledAt:  req.ScheduledAt,
        Priority:     req.Priority,
        MaxRetries:   req.MaxRetries,
        Tags:         req.Tags,
        Status:       entity.TaskStatusPending,
    }

    // Use transaction-aware repository
    txRepo := mysql.NewTaskRepositoryWithTx(tx)
    taskService := task.NewService(txRepo)

    if err := taskService.CreateTask(context.Background(), task); err != nil {
        return nil, fmt.Errorf("failed to create task in transaction: %w", err)
    }

    // Note: Cannot submit immediately if in transaction
    // Task will be picked up by scheduler after transaction commits

    return task, nil
}

// GetTask retrieves a task by ID
func (l *Later) GetTask(ctx context.Context, id string) (*entity.Task, error) {
    return l.taskService.GetTask(ctx, id)
}

// ListTasks lists tasks with pagination and filters
func (l *Later) ListTasks(ctx context.Context, filter *TaskFilter) ([]*entity.Task, int64, error) {
    return l.taskService.List(ctx, filter)
}

// DeleteTask soft-deletes a task
func (l *Later) DeleteTask(ctx context.Context, id, deletedBy string) error {
    return l.taskService.DeleteTask(ctx, id, deletedBy)
}

// RetryTask resets a failed task for retry
func (l *Later) RetryTask(ctx context.Context, id string) (*entity.Task, error) {
    task, err := l.taskService.GetTask(ctx, id)
    if err != nil {
        return nil, err
    }

    if task.Status != entity.TaskStatusFailed {
        return nil, fmt.Errorf("can only retry failed tasks")
    }

    task.Status = entity.TaskStatusPending
    task.RetryCount = 0
    task.NextRetryAt = nil

    if err := l.taskService.UpdateTask(ctx, task); err != nil {
        return nil, err
    }

    if task.ShouldExecuteNow() {
        l.scheduler.SubmitTaskImmediately(task)
    }

    return task, nil
}

// GetStats returns task statistics
func (l *Later) GetStats(ctx context.Context) (*tasksvc.Stats, error) {
    return l.taskService.GetStats(ctx)
}

// GetMetrics returns real-time metrics
func (l *Later) GetMetrics() Metrics {
    return Metrics{
        QueueDepth:      l.scheduler.QueueDepth(),
        ActiveWorkers:   l.workerPool.ActiveCount(),
        CallbackSuccessRate: l.callbackService.SuccessRate(),
    }
}

type CreateTaskRequest struct {
    Name         string            `json:"name"`
    Payload      []byte            `json:"payload"`
    CallbackURL  string            `json:"callback_url"`
    ScheduledAt  time.Time         `json:"scheduled_at"`
    Priority     int               `json:"priority"`
    MaxRetries   int               `json:"max_retries"`
    Tags         []string          `json:"tags"`
}

type TaskFilter struct {
    Status       string     `json:"status"`
    Priority     *int       `json:"priority"`
    CreatedAfter *time.Time `json:"created_after"`
    CreatedBefore *time.Time `json:"created_before"`
    Page         int        `json:"page"`
    Limit        int        `json:"limit"`
    SortBy       string     `json:"sort_by"`
    SortOrder    string     `json:"sort_order"`
}

type Metrics struct {
    QueueDepth          int64   `json:"queue_depth"`
    ActiveWorkers       int     `json:"active_workers"`
    CallbackSuccessRate float64 `json:"callback_success_rate"`
}
```

### Phase 2: HTTP Integration

#### Route Registration

```go
// pkg/later/server.go

// RegisterRoutes registers Later's HTTP routes with existing Gin engine
func (l *Later) RegisterRoutes(engine *gin.Engine) {
    // Create route group with prefix
    group := engine.Group(l.config.RoutePrefix)

    // Apply Later's middleware
    group.Use(l.loggerMiddleware())
    group.Use(l.recoveryMiddleware())

    // Health check
    group.GET("/health", l.healthCheckHandler)

    // Task routes
    tasks := group.Group("/tasks")
    {
        tasks.POST("", l.createTaskHandler)
        tasks.GET("", l.listTasksHandler)
        tasks.GET("/:id", l.getTaskHandler)
        tasks.DELETE("/:id", l.deleteTaskHandler)
        tasks.POST("/:id/retry", l.retryTaskHandler)
        tasks.POST("/:id/resurrect", l.resurrectTaskHandler)
    }

    // Stats
    tasks.GET("/stats", l.getStatsHandler)

    l.logger.Info("Routes registered",
        zap.String("prefix", l.config.RoutePrefix),
    )
}

// Health check handler
func (l *Later) healthCheckHandler(c *gin.Context) {
    status := l.HealthCheck()

    httpStatus := http.StatusOK
    if status.Status == "unhealthy" {
        httpStatus = http.StatusServiceUnavailable
    }

    c.JSON(httpStatus, status)
}
```

### Phase 3: Configuration & Examples

#### Embedded Mode Example

```go
// cmd/embedded-example/main.go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
    _ "github.com/go-sql-driver/mysql"
    "github.com/usual2970/later/v2/pkg/later"
)

func main() {
    // Setup database
    dsn := "user:pass@tcp(localhost:3306)/myapp?parseTime=true"
    db, err := sqlx.Connect("mysql", dsn)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer db.Close()

    // Initialize Later with shared DB
    laterSDK, err := later.New(
        later.WithSharedDB(db),
        later.WithRoutePrefix("/internal/tasks"),
        later.WithWorkerPoolSize(50),
        later.WithAutoMigration(true),
    )
    if err != nil {
        log.Fatalf("Failed to initialize Later: %v", err)
    }

    // Start Later
    if err := laterSDK.Start(); err != nil {
        log.Fatalf("Failed to start Later: %v", err)
    }

    // Setup Gin
    router := gin.Default()

    // Register Later's routes
    laterSDK.RegisterRoutes(router)

    // Register app's routes
    router.GET("/health", func(c *gin.Context) {
        status := laterSDK.HealthCheck()
        c.JSON(http.StatusOK, gin.H{
            "app": "ok",
            "later": status,
        })
    })

    // Create task programmatically
    router.POST("/send-email", func(c *gin.Context) {
        var req struct {
            Email string `json:"email"`
            Subject string `json:"subject"`
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }

        // Create task within request
        task, err := laterSDK.CreateTask(c.Request.Context(), &later.CreateTaskRequest{
            Name:        "send-email",
            Payload:     []byte(`{"to":"`+req.Email+`","subject":"`+req.Subject+`"}`),
            CallbackURL: "https://myapp.com/email-callback",
            ScheduledAt: time.Now().Add(5 * time.Minute),
            Priority:    10,
        })
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }

        c.JSON(http.StatusAccepted, gin.H{"task_id": task.ID})
    })

    // Start server
    srv := &http.Server{
        Addr:    ":8080",
        Handler: router,
    }

    // Graceful shutdown
    go func() {
        log.Println("Server started on :8080")
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server failed: %v", err)
        }
    }()

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("Shutting down...")

    // Shutdown Later first
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := laterSDK.Shutdown(ctx); err != nil {
        log.Printf("Later shutdown error: %v", err)
    }

    // Shutdown HTTP server
    if err := srv.Shutdown(ctx); err != nil {
        log.Printf("Server shutdown error: %v", err)
    }

    log.Println("Server stopped")
}
```

#### Separate Database Example

```go
// Initialize Later with separate database
laterSDK, err := later.New(
    later.WithSeparateDB(
        "user:pass@tcp(localhost:3306)/later_db?parseTime=true",
        later.WithMaxConnections(100),
        later.WithMaxIdleConnections(20),
    ),
    later.WithRoutePrefix("/api/v1/tasks"),
    later.WithWorkerPoolSize(20),
    later.WithAutoMigration(true), // Automatically runs migrations
)
if err != nil {
    log.Fatalf("Failed to initialize Later: %v", err)
}

if err := laterSDK.Start(); err != nil {
    log.Fatalf("Failed to start Later: %v", err)
}

defer laterSDK.Shutdown(context.Background())
```

## Technical Considerations

### Database Connection Management

**Shared DB Mode**:
- App provides `*sqlx.DB` connection
- Later does NOT close connection on shutdown
- Connection pool managed by app
- App responsible for connection lifecycle
- Transactions can span app data and task creation

**Separate DB Mode**:
- Later creates its own `*sqlx.DB` connection
- Later manages connection pool
- Later closes connection on shutdown
- Independent connection lifecycle
- Cannot participate in app transactions

### Migration Strategy

**Automatic Migration** (default):
- Runs on initialization if `AutoMigration: true`
- Safe to run multiple times (idempotent)
- Fails initialization if migration fails
- Recommended for development and simple deployments

**Manual Migration**:
- Set `AutoMigration: false`
- Call `later.RunMigrations(ctx)` explicitly
- Useful for CI/CD pipelines
- Required for production environments with strict migration control

**Migration Versioning**:
- Migrations tracked in `schema_migrations` table
- Each migration has version number
- Only run new migrations on upgrade
- Rollback support via `down` migrations

### Transaction Support

**Transaction-Bound Tasks**:
```go
// Create task within app transaction
tx, _ := db.BeginTxx(ctx, nil)
tx.Exec("INSERT INTO orders ...")

task, err := laterSDK.CreateTaskInTx(tx, &later.CreateTaskRequest{
    Name: "process-order",
    Payload: []byte(`{"order_id": 123}`),
    CallbackURL: "https://myapp.com/callback",
})

tx.Commit() // Task only created if transaction commits
```

**Limitations**:
- Task not submitted to workers immediately (waits for scheduler)
- Cannot call `SubmitTaskImmediately` within transaction
- Task picked up by scheduler after transaction commits

### Error Handling

**Initialization Errors**:
- Return error from `New()`
- Caller should handle failure
- No goroutines started yet

**Runtime Errors**:
- Logged to Later's logger
- Returned to caller if applicable
- HTTP errors return appropriate status codes

**Database Connection Loss**:
- Connection pool handles reconnection
- Scheduler continues polling
- Workers fail gracefully
- Health check reflects unhealthy state

### Concurrency

**Thread Safety**:
- `CreateTask`, `GetTask`, `ListTasks` are thread-safe
- Can be called from multiple goroutines
- Internal mutex protects state

**Backpressure**:
- Worker pool has buffer (size: workers * 2)
- Tasks queued in DB if buffer full
- No explicit backpressure mechanism
- Consider rate limiting for high throughput

## Acceptance Criteria

### Functional Requirements

- [x] **Embedded Mode**: Later can be imported as Go library
- [x] **Shared Database**: Projects can pass existing `*sqlx.DB` connection
- [x] **Separate Database**: Projects can let Later manage its own DB connection
- [x] **Route Prefix**: Projects specify route prefix for HTTP endpoints
- [x] **Task Creation API**: Direct Go API to create tasks programmatically
- [x] **Task Query API**: Direct Go API to query tasks
- [ ] **Transaction Support**: Create tasks within existing transactions (Phase 3)
- [x] **Lifecycle Management**: `Start()` and `Shutdown(ctx)` methods
- [x] **Health Check**: `HealthCheck()` returns status of components
- [x] **Metrics**: `GetMetrics()` returns real-time metrics

### Non-Functional Requirements

- [x] **Thread Safety**: All public APIs are thread-safe
- [x] **Graceful Shutdown**: Waits for in-flight tasks (configurable timeout)
- [x] **Connection Pool**: Respects app's pool limits in shared mode
- [x] **Logging**: Uses injected logger or global zap logger
- [x] **Error Handling**: Clear error messages and proper propagation
- [ ] **Documentation**: API documentation with examples (in progress)
- [x] **Tests**: Unit tests for core functionality
- [ ] **Backwards Compatibility**: Standalone mode still works (not yet tested)

### Quality Gates

- [x] **Test Coverage**: Basic tests for `pkg/later/` package
- [x] **Example Application**: Working `cmd/embedded-example/`
- [ ] **Migration Guide**: Document standalone → embedded migration (not done)
- [ ] **API Documentation**: Godoc comments for all public APIs (partial)
- [ ] **Integration Tests**: Test with real database (not done)
- [ ] **Performance Tests**: Benchmark task creation throughput (not done)

## Success Metrics

- [ ] **Integration Time**: <30 minutes to embed Later in existing app
- [ ] **API Surface**: <10 public functions/types (simplicity)
- [ ] **Task Creation Latency**: <1ms for in-process creation (vs 10ms+ HTTP)
- [ ] **Memory Overhead**: <100MB for 50 workers + shared DB
- [ ] **Documentation Coverage**: 100% of public APIs documented

## Dependencies & Risks

### Dependencies
- **Gin Router**: Required for HTTP route registration
- **SQLX**: Required for database operations
- **MySQL Driver**: Required for database connection
- **Zap Logger**: Required for structured logging

### Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| **Gin Coupling** | Medium | Keep HTTP handlers optional, provide clean Go API |
| **Connection Pool Contention** | Medium | Document pool sizing requirements, provide calculator |
| **Migration Conflicts** | High | Idempotent migrations, version tracking, manual migration option |
| **Shutdown Coordination** | Medium | Clear shutdown API, documented timeout behavior |
| **Transaction Complexity** | Low | Support simple transaction-bound tasks, document limitations |
| **Backwards Compatibility** | High | Keep standalone mode working, avoid breaking changes |

### Mitigation Strategies

1. **Gin Coupling**:
   - Make HTTP layer completely optional
   - Provide clean Go API for non-Gin users
   - Consider generic router interface in future

2. **Connection Pool**:
   - Document recommended pool size formula
   - Add pool size validation warnings
   - Provide pool monitoring metrics

3. **Migration Conflicts**:
   - Use table name prefix customization
   - Idempotent migration scripts
   - Version tracking with rollback support

4. **Shutdown Coordination**:
   - Clear `Shutdown(ctx)` API with timeout
   - Document shutdown order: scheduler → workers → DB
   - Provide health check for monitoring

5. **Transaction Complexity**:
   - MVP: Simple `CreateTaskInTx` method
   - Document that tasks wait for scheduler after transaction
   - Future: Consider outbox pattern

## Alternative Approaches Considered

### Option A: HTTP-Only Integration (Rejected)
**Approach**: Other projects call Later via HTTP API only.

**Pros**:
- Simpler implementation
- Language-agnostic
- Clear service boundary

**Cons**:
- Network overhead
- Operational complexity
- Separate deployment
- No transaction support

**Decision**: Rejected in favor of hybrid approach.

### Option B: Separate Process Manager (Rejected)
**Approach**: Later runs as sidecar process managed by app.

**Pros**:
- Process isolation
- Independent scaling
- Clear failure boundary

**Cons**:
- More complex deployment
- Inter-process communication overhead
- Harder to coordinate lifecycle

**Decision**: Rejected as overly complex for primary use case.

### Option C: Generic Router Interface (Rejected)
**Approach**: Support multiple routers (Gin, Echo, Fiber, etc.)

**Pros**:
- Router-agnostic
- Broader compatibility

**Cons**:
- Complex abstraction
- More maintenance
- Limited user demand

**Decision**: Rejected - Gin is sufficient for MVP, can add later if needed.

## Implementation Phases

### Phase 1: Core Library API (Week 1)
**Goal**: Basic embeddable Later with task creation API

**Tasks**:
1. [x] Create `pkg/later/` package structure
2. [x] Implement `New()` with functional options
3. [x] Implement `Start()` and `Shutdown(ctx)`
4. [x] Implement `CreateTask()` and `GetTask()` APIs
5. [x] Add `HealthCheck()` method
6. [x] Unit tests for core functionality

**Deliverables**:
- ✅ `pkg/later/later.go`
- ✅ `pkg/later/options.go`
- ✅ `pkg/later/task_api.go`
- ✅ `pkg/later/lifecycle.go`
- ✅ `pkg/later/migrations.go`
- ✅ Unit tests
- ✅ Example application (`cmd/embedded-example/`)

**Success Criteria**:
- Can initialize Later with shared DB
- Can create tasks programmatically
- Can shutdown gracefully

### Phase 2: HTTP Integration (Week 2)
**Goal**: Register routes with existing Gin engine

**Tasks**:
1. Implement `RegisterRoutes(engine)`
2. Adapt existing handlers for library use
3. Add route prefix support
4. Add health check endpoint
5. Integration tests

**Deliverables**:
- ✅ `pkg/later/server.go`
- ✅ Route registration tests
- ✅ Example embedded app

**Success Criteria**:
- Can register routes with custom prefix
- HTTP endpoints work correctly
- Health check returns accurate status

### Phase 3: Transaction Support (Week 2)
**Goal**: Create tasks within app transactions

**Tasks**:
1. Add `CreateTaskInTx(tx, req)` method
2. Modify repository to support transactions
3. Document transaction limitations
4. Add integration tests

**Deliverables**:
- ✅ Transaction-aware repository
- ✅ `CreateTaskInTx` API
- ✅ Transaction examples

**Success Criteria**:
- Can create task within transaction
- Task only committed if transaction commits
- Rollback prevents task creation

### Phase 4: Configuration & Examples (Week 3)
**Goal**: Comprehensive examples and configuration options

**Tasks**:
1. Add all configuration options
2. Create `cmd/embedded-example/`
3. Add separate database example
4. Write getting started guide
5. Document all public APIs

**Deliverables**:
- ✅ Complete `options.go`
- ✅ `cmd/embedded-example/main.go`
- ✅ `docs/EMBEDDED.md` guide
- ✅ API documentation

**Success Criteria**:
- Example app runs successfully
- Documentation covers all use cases
- API has 100% godoc coverage

### Phase 5: Migration & Backwards Compatibility (Week 4)
**Goal**: Ensure standalone mode still works

**Tasks**:
1. Update `cmd/server/main.go` to use library
2. Test standalone deployment
3. Write migration guide
4. Add deprecation warnings if needed

**Deliverables**:
- ✅ Updated `cmd/server/main.go`
- ✅ `docs/MIGRATION.md` guide
- ✅ Backwards compatibility tests

**Success Criteria**:
- Standalone mode works identically
- Existing deployments don't break
- Migration path is clear

### Phase 6: Testing & Documentation (Week 5-6)
**Goal**: Production-ready library

**Tasks**:
1. Integration tests with real database
2. Performance benchmarks
3. Concurrency tests
4. Troubleshooting guide
5. Changelog and release notes

**Deliverables**:
- ✅ Integration test suite
- ✅ Performance benchmarks
- ✅ `docs/TROUBLESHOOTING.md`
- ✅ `CHANGELOG.md`

**Success Criteria**:
- Test coverage >80%
- Performance meets targets
- Documentation is comprehensive

## Documentation Plan

### New Documentation Files

1. **`docs/EMBEDDED.md`**
   - Getting started guide
   - Embedded mode setup
   - Configuration options
   - API reference
   - Examples

2. **`docs/MIGRATION.md`**
   - Standalone → embedded migration
   - Breaking changes
   - Version compatibility
   - Step-by-step guide

3. **`docs/TROUBLESHOOTING.md`**
   - Common issues
   - Database connection problems
   - Migration failures
   - Performance tuning

4. **`README.md` Updates**
   - Quick start for embedded mode
   - Link to embedded docs
   - Example usage

### Code Documentation

- Godoc comments on all public APIs
- Examples in godoc comments
- Type documentation
- Configuration option documentation

### Examples

1. **`cmd/embedded-example/`**
   - Complete working example
   - Shows all features
   - Production-ready code

2. **`examples/simple/`**
   - Minimal example
   - Getting started

3. **`examples/advanced/`**
   - Transaction support
   - Custom configuration
   - Monitoring

## References & Research

### Internal References

- **Architecture**: `/Users/liuxuanyao/work/later/docs/plans/2026-02-04-refactor-reorganize-backend-to-follow-go-clean-arch-structure-plan.md`
- **Current Entry Point**: `/Users/liuxuanyao/work/later/cmd/server/main.go:24-131`
- **HTTP Handlers**: `/Users/liuxuanyao/work/later/delivery/rest/handler.go:18-22`
- **Task Service**: `/Users/liuxuanyao/work/later/task/service.go`
- **Scheduler**: `/Users/liuxuanyao/work/later/task/scheduler.go:50-78`
- **Worker Pool**: `/Users/liuxuanyao/work/later/infrastructure/worker/pool.go:208-234`
- **Configuration**: `/Users/liuxuanyao/work/later/configs/config.go:13-20`

### External References

- **Functional Options Pattern**: [Dave Cheney's blog](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis)
- **Go Library Best Practices**: [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- **Clean Architecture**: [bxcodec/go-clean-arch](https://github.com/bxcodec/go-clean-arch)
- **Gin Router**: [Gin documentation](https://gin-gonic.com/docs/)

### Related Work

- Previous refactoring to clean architecture (2026-02-04)
- Task queue implementation
- Worker pool with concurrency control
- Scheduler with tiered polling

## Open Questions

1. **Should we support transaction-bound task submission in MVP?**
   - Complexity: Medium
   - User value: High
   - Recommendation: Yes, but document limitations clearly

2. **Should we provide Prometheus metrics out of the box?**
   - Complexity: Low
   - User value: High for production
   - Recommendation: Yes, optional via config

3. **Should we support multiple router frameworks (Echo, Fiber)?**
   - Complexity: High
   - User value: Medium
   - Recommendation: No for MVP, add later if demand exists

4. **Should we provide a `sync.Pool` for task objects to reduce GC?**
   - Complexity: Low
   - User value: Low (likely premature optimization)
   - Recommendation: No, measure first

5. **Should we support hot-reload of configuration?**
   - Complexity: Medium
   - User value: Low (restart is fast)
   - Recommendation: No for MVP

## Future Enhancements

1. **Generic Router Interface**: Support Echo, Fiber, Chi
2. **Type-Safe Task Payloads**: Generic API with marshaling
3. **Outbox Pattern**: Better transactional guarantees
4. **Distributed Locking**: Support horizontal scaling in embedded mode
5. **Metrics Export**: Prometheus, OpenTelemetry
6. **Callback Authentication**: Support signed callbacks
7. **Task Scheduling DSL**: Cron-like scheduling syntax
8. **Web UI**: Admin panel for task management
