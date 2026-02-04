# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

**Later** is an asynchronous task scheduling service with HTTP callback delivery, built with Go and MySQL. It provides a simple HTTP API for task submission without requiring external message queue infrastructure.

## Development Commands

### Go Backend

```bash
# Run the server
make run
# or
go run cmd/server/main.go

# Build
make build

# Run tests
make test
# or
go test -v ./...

# Run specific test
go test -v ./task -run TestWorkerPool

# Format code
make fmt
# or
go fmt ./...

# Lint code
make lint
# or
golangci-lint run ./...

# Run database migrations
make migrate
# or
mysql -h localhost -u later -p later < migrations/001_init_schema_mysql.up.sql

# Connect to MySQL database
mysql -h localhost -u later -p later
```

### Dashboard (React + TypeScript)

```bash
cd dashboard

# Install dependencies
npm install

# Dev server
npm run dev

# Build
npm run build

# Lint
npm run lint

# Preview production build
npm run preview
```

## Architecture

The codebase follows **go-clean-arch** principles (based on [bxcodec/go-clean-arch](https://github.com/bxcodec/go-clean-arch)) with clear layer separation:

### Directory Structure

```
.
├── domain/                    # Domain layer (entities + repository interfaces)
│   ├── entity/               # Business entities
│   │   ├── task.go           # Task entity with business logic
│   │   └── json_bytes.go     # JSONBytes custom type
│   ├── repository/           # Repository interfaces
│   │   └── task_repository.go
│   └── errors.go             # Domain-specific errors
├── task/                     # Task use case layer
│   ├── service.go            # Task business logic
│   └── scheduler.go          # Task scheduling logic
├── callback/                 # Callback use case layer
│   └── service.go            # Callback delivery logic
├── delivery/                 # Delivery layer
│   └── rest/                 # HTTP delivery
│       ├── handler.go        # HTTP handlers
│       ├── middleware/       # HTTP middleware
│       └── dto/              # Request/response DTOs
├── repository/              # Repository implementations
│   └── mysql/
│       ├── connection.go
│       └── task_repository.go
├── infrastructure/          # Infrastructure services
│   ├── worker/
│   │   └── pool.go          # Worker pool implementation
│   ├── circuitbreaker/
│   │   └── circuit_breaker.go
│   └── logger/
│       └── logger.go
└── server/                  # HTTP server
    └── server.go
```

### Core Components

1. **Domain Layer** (`domain/`)
   - `entity/task.go` - Task entity with business logic (state transitions, retry calculation)
   - `entity/json_bytes.go` - JSONBytes custom type for database JSON handling
   - `repository/task_repository.go` - Repository interface
   - `errors.go` - Domain-specific errors

2. **Use Case Layer** (`task/`, `callback/`)
   - `task/service.go` - Task CRUD operations
   - `task/scheduler.go` - Tiered polling with time.Ticker (high/normal/cleanup)
   - `callback/service.go` - HTTP callback delivery with retry logic

3. **Delivery Layer** (`delivery/`)
   - `rest/handler.go` - Gin-based HTTP handlers for task API endpoints
   - `rest/middleware/` - CORS, recovery, and logging middleware
   - `rest/dto/` - Request/response data transfer objects

4. **Infrastructure Layer** (`infrastructure/`, `repository/`)
   - `worker/pool.go` - Worker pool (default 20 workers) for task execution
   - `circuitbreaker/circuit_breaker.go` - Per-URL circuit breaker (closed/open/half-open states)
   - `repository/mysql/` - MySQL repository implementations

5. **Server** (`server/`)
   - `server.go` - HTTP server

### Tiered Polling Strategy

The scheduler uses three tiers for optimal task processing:

| Tier | Interval | Priority | Description |
|------|----------|----------|-------------|
| High | 2s | > 5 | Urgent tasks |
| Normal | 3s | 0-5 | Standard tasks |
| Low | 30s | - | Cleanup jobs |
| Immediate | N/A | Any | Submitted directly to worker pool |

### Task Lifecycle

```
pending → processing → completed
                    ↓
                   failed → (retry with exponential backoff) → processing
                    ↓
           dead_lettered (after max_retries exceeded)
```

### Circuit Breaker Pattern

- Prevents hammering failing callback endpoints
- States: Closed → Open → Half-Open → Closed
- Triggers after `maxFailures` consecutive failures
- Auto-transitions to half-open after `resetTimeout`

## Key Implementation Details

### Task Model (domain/entity/task.go)

- **JSONBytes**: Custom type for MySQL JSON with `Scan/Value` methods
- **State transitions**: `MarkAsProcessing()`, `MarkAsCompleted()`, `MarkAsFailed()`, `MarkAsDeadLettered()`
- **Exponential backoff**: `CalculateNextRetry()` with jitter (±25%)
- **Priority classification**: `IsHighPriority()` returns true if priority > 5

### Worker Pool (infrastructure/worker/pool.go)

- Buffered channel size: `workerCount * 2`
- Non-blocking submission: `SubmitTask()` returns false if pool is full
- Graceful shutdown: 30-second timeout waiting for workers to finish
- Interface-based design to avoid circular dependencies

### Callback Delivery (callback/service.go)

- HMAC-SHA256 signature header (`X-Signature`) if `CALLBACK_SECRET` is set
- Response classification:
  - 2xx → Success (mark completed)
  - 5xx/429 → Retry (exponential backoff)
  - 4xx → Permanent failure

### Database Schema (migrations/001_init_schema_mysql.up.sql)

- `task_queue` table with JSON payload
- Indexes on `(status, scheduled_at, priority)` for active queries
- `next_retry_at` index for retry scheduling
- Tags stored as JSON array (MySQL 8.0+)

## Environment Configuration

Key environment variables (see `configs/config.go`):

```bash
# Database (MySQL)
DATABASE_URL=mysql://later:password@localhost:3306/later?parseTime=true&loc=UTC&charset=utf8mb4
DATABASE_MAX_CONNECTIONS=100
DATABASE_MAX_OPEN_CONNS=100
DATABASE_MAX_IDLE_CONNS=20
DATABASE_CONN_MAX_LIFETIME=1h
DATABASE_CONN_MAX_IDLE_TIME=10m

# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Scheduler (tiered polling intervals in seconds)
SCHEDULER_HIGH_INTERVAL=2
SCHEDULER_NORMAL_INTERVAL=3
SCHEDULER_CLEANUP_INTERVAL=30

# Worker Pool
WORKER_POOL_SIZE=20

# Callback Security
CALLBACK_SECRET=your-secret-key-here
CALLBACK_TIMEOUT=30
CALLBACK_MAX_RETRIES=5

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

## Dashboard

- **React 19** with TypeScript
- **Vite** for build tooling
- **shadcn/ui** components in `dashboard/src/components/ui/`
- Task list, stats, and detail views
