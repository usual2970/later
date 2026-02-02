# Later - Asynchronous Task Scheduling Service with HTTP Callbacks

A Go-based asynchronous task scheduling service that provides HTTP callback capabilities for businesses without message queue infrastructure.

## Overview

**Later** enables you to:
- ✅ Submit tasks via simple HTTP API
- ✅ Schedule delayed execution (from immediate to 1 year)
- ✅ Automatic HTTP callback delivery with retry logic
- ✅ Exponential backoff for failed callbacks
- ✅ Circuit breaker to prevent hammering failing endpoints
- ✅ Real-time task monitoring via dashboard (coming in Phase 4)
- ✅ No external message queue dependencies - just PostgreSQL

## Architecture

This service follows the **go-clean-arch** pattern with clear separation of concerns:

```
┌─────────────┐      HTTP POST      ┌─────────────────────────────────┐
│   Client    │ ──────────────────> │  Gin API Server (Go)           │
└─────────────┘                     │  - Tiered scheduling (Ticker)  │
                                    │  - Worker pool (20 workers)    │
                                    └─────────────────────────────────┘
                                                │
                                                ▼
                                    ┌─────────────────────────────────┐
                                    │  PostgreSQL Database           │
                                    │  - Task queue (JSONB)         │
                                    │  - Retry tracking             │
                                    └─────────────────────────────────┘
```

### Tiered Polling Strategy

| Tier | Interval | Use Case |
|------|----------|----------|
| **Immediate** | N/A | Tasks without delay (submitted directly to worker pool) |
| **High** | 2 seconds | Priority > 5 (urgent tasks) |
| **Normal** | 3 seconds | Priority 0-5 (standard tasks) |
| **Low** | 30 seconds | Cleanup and maintenance |

## Tech Stack

- **Go 1.21+** with clean architecture
- **Gin** - High-performance HTTP framework
- **PostgreSQL 15+** - Task persistence with JSONB
- **pgx/v5** - PostgreSQL driver
- **time.Ticker** - Standard library scheduling (no cron dependencies!)

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 15+
- Make (for migrations, optional)

### 1. Clone and Setup

```bash
git clone <repo-url>
cd later
go mod download
```

### 2. Configure Environment

Create a `.env` file or set environment variables:

```bash
# Database
DATABASE_URL=postgres://later:password@localhost:5432/later?sslmode=disable
DATABASE_MAX_CONNECTIONS=100

# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Scheduler (tiered polling)
SCHEDULER_HIGH_INTERVAL=2     # seconds
SCHEDULER_NORMAL_INTERVAL=3   # seconds
SCHEDULER_CLEANUP_INTERVAL=30 # seconds

# Worker Pool
WORKER_POOL_SIZE=20

# Callback Security (generate your own!)
CALLBACK_SECRET=your-secret-key-here
CALLBACK_TIMEOUT=30
CALLBACK_MAX_RETRIES=5

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

### 3. Run Database Migrations

The service will automatically run migrations on startup, or you can run them manually:

```bash
psql -h localhost -U later -d later -f migrations/001_init_schema.up.sql
```

### 4. Run the Service

```bash
go run cmd/server/main.go
```

Or build and run:

```bash
go build -o bin/server cmd/server/main.go
./bin/server
```

The server will start on `http://localhost:8080`

## API Usage

### Submit a Task (Immediate Execution)

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "process_order",
    "payload": {
      "order_id": 12345,
      "customer_id": "abc-123"
    },
    "callback_url": "https://api.example.com/webhooks/order-processed"
  }'
```

**Response:**
```json
{
  "task_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "scheduled_for": "2026-02-02T10:00:05Z",
  "created_at": "2026-02-02T10:00:05Z",
  "estimated_execution": "immediate"
}
```

### Submit a Task (Delayed Execution)

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "send_reminder_email",
    "payload": {
      "user_id": "user-123",
      "email": "user@example.com"
    },
    "callback_url": "https://api.example.com/webhooks/email-sent",
    "scheduled_for": "2026-02-02T15:30:00Z",
    "priority": 5
  }'
```

## Callback Format

When a task completes, the service will POST to your `callback_url`:

```http
POST {callback_url}
X-Task-ID: 550e8400-e29b-41d4-a716-446655440000
X-Signature: sha256=<hmac_signature>
Content-Type: application/json

{
  "task_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "original_payload": {
    "order_id": 12345,
    "customer_id": "abc-123"
  },
  "execution_metadata": {
    "scheduled_at": "2026-02-02T15:00:00Z",
    "executed_at": "2026-02-02T15:00:05Z",
    "completed_at": "2026-02-02T15:00:08Z",
    "retry_count": 2,
    "callback_attempts": 3,
    "duration_ms": 3124
  }
}
```

## Development

### Project Structure

```
later/
├── cmd/
│   └── server/
│       └── main.go                    # Application entry point
├── internal/
│   ├── domain/
│   │   ├── models/
│   │   │   └── task.go                # Task entity
│   │   └── repositories/
│   │       └── task_repository.go      # Repository interface
│   ├── usecase/
│   │   ├── task_service.go            # Business logic
│   │   └── scheduler.go               # Tiered scheduler
│   ├── repository/
│   │   └── postgres/
│   │       ├── connection.go           # DB connection
│   │       └── task_repository.go      # PostgreSQL impl
│   ├── handler/
│   │   └── handler.go                 # HTTP handlers
│   └── server/
│       └── server.go                  # HTTP server
├── migrations/
│   └── 001_init_schema.up.sql        # Database schema
├── configs/
│   └── config.go                      # Configuration
└── go.mod
```

### Database Schema

The service uses a single table `task_queue` with:

- **UUID** primary key
- **JSONB** payload for flexible task data
- **Priority** field (0-10) for tiered processing
- **Retry configuration** (max_retries, retry_count, next_retry_at)
- **Callback tracking** (attempts, last status, error messages)
- **Performance indexes** for efficient querying

### Testing

```bash
go test ./...
```

## Configuration

All configuration is done via environment variables. See `.env` example above for all available options.

Key configuration options:

- `SCHEDULER_HIGH_INTERVAL` - High-priority polling (default: 2s)
- `SCHEDULER_NORMAL_INTERVAL` - Normal-priority polling (default: 3s)
- `SCHEDULER_CLEANUP_INTERVAL` - Cleanup job interval (default: 30s)
- `WORKER_POOL_SIZE` - Number of concurrent workers (default: 20)
- `CALLBACK_SECRET` - Secret key for HMAC signatures on callbacks

## Roadmap

### Phase 1: Foundation ✅ (Current)
- ✅ Go project setup with go-clean-arch
- ✅ PostgreSQL database schema
- ✅ Domain models (Task entity)
- ✅ Repository layer with GORM
- ✅ Configuration management
- ✅ Tiered scheduler with time.Ticker

### Phase 2: HTTP API (Next)
- ⏳ Task submission API
- ⏳ Task query APIs (list, get details)
- ⏳ Statistics API
- ⏳ WebSocket stream for real-time updates

### Phase 3: Scheduling & Execution
- ⏳ Worker pool implementation
- ⏳ Task executor
- ⏳ Callback delivery with retry
- ⏳ Circuit breaker for failing endpoints
- ⏳ Dead letter queue

### Phase 4: Frontend Dashboard
- ⏳ React with TypeScript
- ⏳ shadcn/ui components
- ⏳ Real-time task updates via WebSocket
- ⏳ Task list and detail views

## Contributing

This project follows clean architecture principles. Please:

1. Follow the existing code structure
2. Add tests for new functionality
3. Update documentation for API changes
4. Use conventional commit messages

## License

MIT License - see LICENSE file for details

## Acknowledgments

- Architecture inspired by [go-clean-arch](https://github.com/bxcodec/go-clean-arch)
- Built with [Gin](https://gin-gonic.com/) web framework
- Database powered by [PostgreSQL](https://www.postgresql.org/) and [pgx](https://github.com/jackc/pgx)
