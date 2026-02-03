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
go test -v ./internal/usecase -run TestWorkerPool

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

The codebase follows **go-clean-arch** principles with clear layer separation:

### Core Components

1. **Domain Layer** (`internal/domain/`)
   - `models/task.go` - Task entity with business logic (state transitions, retry calculation)
   - `repositories/task_repository.go` - Repository interface

2. **Repository Layer** (`internal/repository/mysql/`)
   - MySQL implementation using go-sql-driver/mysql and sqlx
   - Connection pooling and context management

3. **Use Case Layer** (`internal/usecase/`)
   - `task_service.go` - Task CRUD operations
   - `scheduler.go` - Tiered polling with time.Ticker (high/normal/cleanup)
   - `worker.go` - Worker pool (default 20 workers) for task execution
   - `callback_service.go` - HTTP callback delivery with retry logic

4. **Handler Layer** (`internal/handler/`)
   - Gin-based HTTP handlers for task API endpoints

5. **Infrastructure** (`internal/infrastructure/`)
   - `circuitbreaker/circuit_breaker.go` - Per-URL circuit breaker (closed/open/half-open states)

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

### Task Model (internal/domain/models/task.go)

- **JSONBytes**: Custom type for MySQL JSON with `Scan/Value` methods
- **State transitions**: `MarkAsProcessing()`, `MarkAsCompleted()`, `MarkAsFailed()`, `MarkAsDeadLettered()`
- **Exponential backoff**: `CalculateNextRetry()` with jitter (±25%)
- **Priority classification**: `IsHighPriority()` returns true if priority > 5

### Worker Pool (internal/usecase/worker.go)

- Buffered channel size: `workerCount * 2`
- Non-blocking submission: `SubmitTask()` returns false if pool is full
- Graceful shutdown: 30-second timeout waiting for workers to finish

### Callback Delivery (internal/usecase/callback_service.go)

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
- Real-time updates via WebSocket (Phase 4 feature)
- Task list, stats, and detail views
