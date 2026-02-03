# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

**Later** is an asynchronous task scheduling service with HTTP callback delivery, built with Go and PostgreSQL. It provides a simple HTTP API for task submission without requiring external message queue infrastructure.

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
psql -h localhost -U later -d later -f migrations/001_init_schema.up.sql
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

2. **Repository Layer** (`internal/repository/postgres/`)
   - PostgreSQL implementation using pgx/v5
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

- **JSONBytes**: Custom type for PostgreSQL JSONB with `Scan/Value` methods
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

### Database Schema (migrations/001_init_schema.up.sql)

- `task_queue` table with JSONB payload
- Partial indexes on `(status, scheduled_at, priority)` for active queries
- `next_retry_at` index for retry scheduling
- GIN index on `tags` array

## Environment Configuration

Key environment variables (see `configs/config.go`):

```bash
# Database
DATABASE_URL=postgres://later:password@localhost:5432/later?sslmode=disable
DATABASE_MAX_CONNECTIONS=100

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
