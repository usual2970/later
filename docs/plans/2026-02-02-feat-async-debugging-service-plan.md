---
title: feat: Asynchronous Debugging Service with HTTP Callbacks
type: feat
date: 2026-02-02
tags: [go, gin, react, shadcn, async, scheduling, webhooks]
status: draft
---

# ✨ Asynchronous Debugging Service with HTTP Callbacks

## Overview

Build a Go-based asynchronous task scheduling and debugging service that provides HTTP callback capabilities for businesses without message queue infrastructure. The service enables delayed message processing, automatic retry with exponential backoff, and includes a React dashboard with shadcn components for real-time task monitoring.

**Target Users:** Development teams and businesses that need async task processing but don't have existing message queue infrastructure (RabbitMQ, Kafka, Redis, etc.).

## Problem Statement

Many small-to-medium businesses and development teams need asynchronous task processing capabilities but face barriers:

1. **Infrastructure Complexity**: Setting up and maintaining message queue infrastructure (RabbitMQ, Kafka, Redis) requires DevOps expertise
2. **Resource Constraints**: Limited resources to operate and scale distributed message systems
3. **Development Speed**: Need quick integration for async operations without complex setup
4. **Callback Delivery**: Difficulty implementing reliable HTTP callbacks with retry logic
5. **Visibility**: Lack of debugging tools to view scheduled tasks and their execution status

**Current Solutions Are Painful:**
- Self-hosted MQ: Complex setup, maintenance overhead
- Cloud MQ services: Expensive, vendor lock-in
- Custom cron jobs: No retry logic, poor observability
- Background workers: No delayed execution, limited callback support

## Proposed Solution

A standalone HTTP-based async task service that provides:

1. **Simple HTTP API**: Submit tasks via REST endpoints
2. **Delayed Execution**: Schedule tasks for future execution
3. **Reliable Callbacks**: HTTP webhook delivery with automatic retry
4. **Web Dashboard**: React UI with shadcn for task monitoring
5. **Self-Contained**: No external MQ dependencies, uses PostgreSQL for persistence

### Architecture

```
┌─────────────┐      HTTP POST      ┌─────────────────────────────────┐
│   Client    │ ──────────────────> │  Gin API Server (Go)           │
│  (Any App)  │  /api/v1/tasks      │  - Task validation             │
└─────────────┘                     │  - Persistence (PostgreSQL)    │
                                    │  - Tiered scheduling (Ticker)  │
                                    └─────────────────────────────────┘
                                                │
                                                ▼
                                    ┌─────────────────────────────────┐
                                    │  Task Scheduler                │
                                    │  - High-priority: 2s ticker    │
                                    │  - Normal-priority: 3s ticker  │
                                    │  - Worker pool (20 workers)    │
                                    └─────────────────────────────────┘
                                                │
                                                ▼
                                    ┌─────────────────────────────────┐
                                    │  Task Executor                 │
                                    │  - Execute business logic      │
                                    │  - Callback delivery           │
                                    │  - Retry with backoff          │
                                    └─────────────────────────────────┘
                                                │
                                                ▼
                                    ┌─────────────────────────────────┐
                                    │  Callback Service              │
                                    │  - HTTP POST to callback_url   │
                                    │  - Circuit breaker             │
                                    │  - Signature verification      │
                                    └─────────────────────────────────┘

┌─────────────┐  WebSocket/HTTP   ┌─────────────────────────────────┐
│  React UI   │ ──────────────────> │  API Server                    │
│  (shadcn)   │                     │  - Real-time updates           │
└─────────────┘                     │  - Task list/details           │
                                    │  - Metrics & stats             │
                                    └─────────────────────────────────┘
```

## Technical Stack

### Backend (Go)
- **Framework**: Gin (high-performance HTTP router)
- **Architecture**: go-clean-arch pattern
  - Reference: https://github.com/bxcodec/go-clean-arch
- **Database**: PostgreSQL with GORM
- **Scheduling**: `time.Ticker` with tiered polling strategy
  - High-priority tasks: 2-second interval
  - Normal tasks: 3-second interval
  - Low-priority/cleanup: 30-second interval
- **Callback HTTP**: Standard library with circuit breaker pattern
- **Authentication**: JWT tokens (future enhancement)

### Frontend (React)
- **Framework**: React with TypeScript
- **UI Components**: shadcn/ui (Radix UI primitives)
- **Styling**: Tailwind CSS
- **State Management**: React Context + hooks
- **Real-time**: WebSocket connection for live updates

### Database
- **Primary Storage**: PostgreSQL 15+
- **Features**:
  - ACID compliance for task state
  - JSONB for flexible payloads
  - Partial indexes for performance
  - Partitioning for large datasets (future)

## Technical Approach

### Phase 1: Foundation (Week 1)

#### 1.1 Project Setup
- [ ] Initialize Go module: `go mod init later`
- [ ] Set up go-clean-arch directory structure:
  ```
  cmd/
    server/
      main.go
  internal/
    domain/
      models/
      repositories/
    usecase/
    repository/
      postgres/
    delivery/
      gin/
  pkg/
  configs/
  ```
- [ ] Initialize React project with Vite
- [ ] Install shadcn/ui and configure Tailwind CSS
- [ ] Set up PostgreSQL database with Docker Compose

#### 1.2 Database Schema
- [ ] Create task_queue table with schema:

```sql
CREATE TABLE task_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,  -- Future: for multi-tenancy
    name VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    callback_url TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'dead_lettered')),

    -- Timing
    created_at TIMESTAMPTZ DEFAULT NOW(),
    scheduled_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    -- Retry configuration
    max_retries INTEGER DEFAULT 5,
    retry_count INTEGER DEFAULT 0,
    retry_backoff_seconds INTEGER DEFAULT 60,
    next_retry_at TIMESTAMPTZ,

    -- Callback tracking
    callback_attempts INTEGER DEFAULT 0,
    callback_timeout_seconds INTEGER DEFAULT 30,
    last_callback_at TIMESTAMPTZ,
    last_callback_status INTEGER,
    last_callback_error TEXT,

    -- Metadata
    priority INTEGER DEFAULT 0 CHECK (priority >= 0),
    tags TEXT[],
    error_message TEXT,

    CHECK (retry_count <= max_retries)
);

-- Performance indexes
CREATE INDEX idx_tasks_status_scheduled
ON task_queue(status, scheduled_at)
WHERE status IN ('pending', 'processing', 'failed');

CREATE INDEX idx_tasks_next_retry
ON task_queue(next_retry_at)
WHERE next_retry_at IS NOT NULL AND status = 'failed';

CREATE INDEX idx_tasks_created_at
ON task_queue(created_at DESC);

CREATE INDEX idx_tasks_tags
ON task_queue USING GIN(tags);

-- Worker coordination
CREATE INDEX idx_tasks_worker
ON task_queue(worker_id, status)
WHERE worker_id IS NOT NULL;
```

- [ ] Add migrations tool (golang-migrate or goose)
- [ ] Set up database connection pooling

### Phase 2: Core API (Week 2)

#### 2.1 Task Submission API
- [ ] `POST /api/v1/tasks` - Submit new task

**Request (Immediate Execution):**
```http
POST /api/v1/tasks
Content-Type: application/json

{
  "name": "process_order",
  "payload": {
    "order_id": 12345,
    "customer_id": "abc-123",
    "items": [...]
  },
  "callback_url": "https://api.example.com/webhooks/order-processed"
  // 注意：没有 scheduled_for 字段 = 立即执行
}
```

**Request (Delayed Execution):**
```http
POST /api/v1/tasks
Content-Type: application/json

{
  "name": "send_reminder_email",
  "payload": {
    "user_id": "user-123",
    "email": "user@example.com"
  },
  "callback_url": "https://api.example.com/webhooks/email-sent",
  "scheduled_for": "2026-02-02T15:30:00Z",  // 延迟到指定时间执行
  "timeout_seconds": 30,
  "max_retries": 5,
  "priority": 1,
  "tags": ["email", "reminder"]
}
```

**Response - Immediate Execution:**
```json
{
  "task_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "scheduled_for": "2026-02-02T10:00:05Z",
  "created_at": "2026-02-02T10:00:05Z",
  "estimated_execution": "immediate"
}
```

**Response - Delayed Execution:**
```json
{
  "task_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "scheduled_for": "2026-02-02T15:30:00Z",
  "created_at": "2026-02-02T10:00:00Z",
  "estimated_execution": "2026-02-02T15:30:00Z"
}
```

**Validation:**
- `callback_url`: Must be valid HTTPS URL, SSRF protection (block private IPs)
- `payload`: Max 1MB, valid JSON
- `scheduled_for`: Optional. If omitted or <= now, execute **immediately**
- `timeout_seconds`: 5-300 seconds range
- `max_retries`: 0-20 range

**Immediate Execution Flow:**

When `scheduled_for` is omitted or in the past:

```go
// internal/delivery/gin/handler.go
func (h *Handler) CreateTask(c *gin.Context) {
    var req CreateTaskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // 设置默认调度时间（立即执行）
    if req.ScheduledFor == nil || req.ScheduledFor.Before(time.Now()) {
        now := time.Now()
        req.ScheduledFor = &now
    }

    task := &domain.Task{
        ID:           uuid.New().String(),
        Name:         req.Name,
        Payload:      req.Payload,
        CallbackURL:  req.CallbackURL,
        ScheduledFor: *req.ScheduledFor,
        Status:       domain.TaskStatusPending,
        // ... 其他字段
    }

    // 保存到数据库
    if err := h.taskService.CreateTask(c.Request.Context(), task); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    // 如果是立即执行的任务，直接提交到 worker pool
    if task.ScheduledFor.Before(time.Now().Add(1 * time.Second)) {
        h.scheduler.SubmitTaskImmediately(task)
    }

    c.JSON(202, gin.H{
        "task_id":             task.ID,
        "status":              task.Status,
        "scheduled_for":       task.ScheduledFor,
        "created_at":          task.CreatedAt,
        "estimated_execution": "immediate",
    })
}
```

**Scheduler with Immediate Task Support:**

```go
// internal/usecase/scheduler.go
func (s *Scheduler) SubmitTaskImmediately(task *Task) {
    select {
    case s.workerPool <- task:
        s.logger.Info("Task submitted immediately",
            zap.String("task_id", task.ID),
            zap.Int("priority", task.Priority))
    default:
        // Worker pool 满，记录到数据库等待下次轮询
        s.logger.Warn("Worker pool full, task will be picked up by next poll",
            zap.String("task_id", task.ID))
    }
}
```

#### 2.2 Task Query APIs
- [ ] `GET /api/v1/tasks` - List tasks (paginated, filterable)
- [ ] `GET /api/v1/tasks/:id` - Get task details
- [ ] `DELETE /api/v1/tasks/:id` - Cancel pending task
- [ ] `POST /api/v1/tasks/:id/retry` - Retry failed task

**List Tasks Query Parameters:**
```
status: pending|processing|completed|failed|dead_lettered
page: integer (default 1)
limit: integer (default 50, max 100)
sort: created_at|scheduled_at|priority
order: asc|desc
tags: string (comma-separated)
```

#### 2.3 Statistics API
- [ ] `GET /api/v1/tasks/stats` - Aggregated statistics

```json
{
  "total": 15234,
  "by_status": {
    "pending": 234,
    "processing": 12,
    "completed": 14800,
    "failed": 150,
    "dead_lettered": 38
  },
  "last_24h": {
    "submitted": 1234,
    "completed": 1198,
    "failed": 36
  },
  "callback_success_rate": 0.94
}
```

#### 2.4 WebSocket Stream
- [ ] `WS /api/v1/tasks/stream` - Real-time task updates

**WebSocket Message Format:**
```json
{
  "type": "task_updated",
  "data": {
    "task_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "completed",
    "updated_at": "2026-02-02T15:30:05Z"
  }
}
```

### Phase 3: Scheduling & Execution (Week 3)

#### 3.1 Task Scheduler with Tiered Polling
- [ ] Implement `time.Ticker` based scheduler (no external dependencies)
- [ ] Tiered polling strategy:
  - **High-priority tasks** (priority > 5): Poll every 2 seconds
  - **Normal tasks** (priority 0-5): Poll every 3 seconds
  - **Low-priority/cleanup**: Poll every 30 seconds
- [ ] Worker pool pattern (configurable, default 20 workers)
- [ ] SELECT FOR UPDATE SKIP LOCKED for distributed workers

**Implementation:**

```go
// internal/usecase/scheduler.go
package usecase

import (
    "context"
    "time"
    "go.uber.org/zap"
)

type Scheduler struct {
    highPriorityTicker  *time.Ticker // 2 seconds
    normalPriorityTicker *time.Ticker // 3 seconds
    cleanupTicker        *time.Ticker // 30 seconds

    taskRepo   TaskRepository
    workerPool chan *Task
    logger     *zap.Logger
    quit       chan struct{}
}

func NewScheduler(
    repo TaskRepository,
    workerPool chan *Task,
    logger *zap.Logger,
) *Scheduler {
    return &Scheduler{
        highPriorityTicker:  time.NewTicker(2 * time.Second),
        normalPriorityTicker: time.NewTicker(3 * time.Second),
        cleanupTicker:        time.NewTicker(30 * time.Second),
        taskRepo:             repo,
        workerPool:           workerPool,
        logger:               logger,
        quit:                 make(chan struct{}),
    }
}

func (s *Scheduler) Start() {
    defer s.highPriorityTicker.Stop()
    defer s.normalPriorityTicker.Stop()
    defer s.cleanupTicker.Stop()

    s.logger.Info("Scheduler started with tiered polling",
        zap.Duration("high_priority_interval", 2*time.Second),
        zap.Duration("normal_priority_interval", 3*time.Second),
        zap.Duration("cleanup_interval", 30*time.Second),
    )

    // 初始轮询
    s.pollDueTasks("high", 5, 50)
    s.pollDueTasks("normal", 0, 100)

    for {
        select {
        case <-s.highPriorityTicker.C:
            s.pollDueTasks("high", 5, 50)

        case <-s.normalPriorityTicker.C:
            s.pollDueTasks("normal", 0, 100)

        case <-s.cleanupTicker.C:
            s.pollDueTasks("low", -1, 200)
            s.cleanupExpiredTasks()

        case <-s.quit:
            s.logger.Info("Scheduler stopping...")
            return
        }
    }
}

func (s *Scheduler) Stop() {
    close(s.quit)
}

// pollDueTasks 查找并提交到期任务到 worker pool
// tier: "high", "normal", "low"
// minPriority: 最小优先级阈值
// limit: 一次查询的最大任务数
func (s *Scheduler) pollDueTasks(tier string, minPriority int, limit int) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    tasks, err := s.taskRepo.FindDueTasks(ctx, minPriority, limit)
    if err != nil {
        s.logger.Error("Failed to fetch due tasks",
            zap.String("tier", tier),
            zap.Error(err))
        return
    }

    if len(tasks) == 0 {
        return
    }

    s.logger.Debug("Found due tasks",
        zap.String("tier", tier),
        zap.Int("count", len(tasks)))

    // 提交到 worker pool
    submitted := 0
    for _, task := range tasks {
        select {
        case s.workerPool <- task:
            submitted++
        default:
            // worker pool 满
            s.logger.Warn("Worker pool full, task will be retried next cycle",
                zap.String("task_id", task.ID),
                zap.String("tier", tier))
        }
    }

    s.logger.Debug("Tasks submitted to workers",
        zap.String("tier", tier),
        zap.Int("submitted", submitted),
        zap.Int("skipped", len(tasks)-submitted))
}

func (s *Scheduler) cleanupExpiredTasks() {
    // 清理过期的幂等性键等
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    count, err := s.taskRepo.CleanupExpiredData(ctx)
    if err != nil {
        s.logger.Error("Failed to cleanup expired data", zap.Error(err))
        return
    }

    if count > 0 {
        s.logger.Info("Cleaned up expired data", zap.Int64("count", count))
    }
}
```

**Database Query with Priority Support:**

```go
// internal/repository/postgres/task_repository.go
func (r *PostgresTaskRepository) FindDueTasks(
    ctx context.Context,
    minPriority int,
    limit int,
) ([]*domain.Task, error) {
    query := `
        SELECT id, name, payload, callback_url, status, priority,
               scheduled_at, max_retries, retry_count, callback_timeout_seconds,
               created_at, updated_at
        FROM task_queue
        WHERE status = 'pending'
          AND scheduled_at <= NOW()
          AND ($1 = -1 OR priority > $1)
        ORDER BY priority DESC, scheduled_at ASC
        LIMIT $2
        FOR UPDATE SKIP LOCKED
    `

    var tasks []*domain.Task
    err := r.db.SelectContext(ctx, &tasks, query, minPriority, limit)
    return tasks, err
}
```

**Benefits of Tiered Polling:**

| Tier | Interval | Use Case | Priority Range |
|------|----------|----------|----------------|
| **Immediate** | N/A | Tasks submitted without delay | All priorities |
| **High** | 2 seconds | Urgent callbacks, time-sensitive tasks | priority > 5 |
| **Normal** | 3 seconds | Standard async processing | priority 0-5 |
| **Low** | 30 seconds | Cleanup, batch jobs, low-priority | All priorities |

**Task Execution Flow:**

```
Task Submission
       │
       ▼
Is scheduled_for <= now?
       │
   ┌───┴───┐
   │       │
  YES      NO
   │       │
   ▼       ▼
Submit to  Save to DB,
Worker     Wait for ticker
Pool       Poll
   │       │
   └───┬───┘
       ▼
Execute Task
```

**Advantages:**
- ✅ No external dependencies (uses `time.Ticker` from standard library)
- ✅ **Immediate tasks bypass polling delay** - submitted directly to worker pool
- ✅ High-priority scheduled tasks processed faster (2s vs 3s)
- ✅ Reduced database load (low-priority cleanup only every 30s)
- ✅ Configurable intervals via environment variables
- ✅ Simple and maintainable code

#### 3.2 Task Executor
- [ ] Update task status to "processing"
- [ ] Record worker_id and started_at
- [ ] Execute business logic (future: user-defined hooks)
- [ ] Initiate callback delivery
- [ ] Handle completion/failure

#### 3.3 Callback Delivery Service
- [ ] HTTP POST with timeout
- [ ] Response status classification:
  - Success: 200-299 → mark task "completed"
  - Retry: 500-599, 429 → schedule retry
  - Failure: 400-499 (except 429) → mark task "failed"
- [ ] Exponential backoff calculation:
  ```
  delay = base_backoff * (2 ^ retry_count) + jitter
  max_delay = 24 hours
  ```

```go
// internal/usecase/callback_service.go
func (s *callbackService) deliverCallback(ctx context.Context, task *Task) error {
    client := &http.Client{Timeout: time.Duration(task.CallbackTimeout) * time.Second}

    req, _ := http.NewRequestWithContext(ctx, "POST", task.CallbackURL, bytes.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Task-ID", task.ID)
    req.Header.Set("X-Signature", s.generateSignature(task))

    resp, err := client.Do(req)
    if err != nil {
        return s.handleRetry(task, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        return s.markCompleted(task)
    } else if resp.StatusCode >= 500 || resp.StatusCode == 429 {
        return s.handleRetry(task, fmt.Errorf("callback returned %d", resp.StatusCode))
    } else {
        return s.markFailed(task, fmt.Errorf("callback returned %d", resp.StatusCode))
    }
}
```

#### 3.4 Circuit Breaker
- [ ] Implement circuit breaker for unreliable callback URLs
- [ ] Threshold: 5 consecutive failures → circuit opens
- [ ] Open timeout: 60 seconds before half-open
- [ ] Half-open: Allow 1 test request

```go
// internal/infrastructure/circuitbreaker/circuit_breaker.go
type CircuitBreaker struct {
    maxFailures  int
    resetTimeout time.Duration
    failures     map[string]int
    lastFailure  map[string]time.Time
    state        map[string]string // closed, open, half-open
    mu           sync.RWMutex
}

func (cb *CircuitBreaker) Execute(url string, fn func() error) error {
    if cb.isOpen(url) {
        return ErrCircuitOpen
    }

    err := fn()
    if err != nil {
        cb.recordFailure(url)
    } else {
        cb.recordSuccess(url)
    }
    return err
}
```

#### 3.5 Dead Letter Handling
- [ ] After max retries exhausted → mark status "dead_lettered"
- [ ] Retain in database for 30 days
- [ ] Provide API endpoint to query dead lettered tasks
- [ ] Manual resurrection: `POST /api/v1/tasks/:id/resurrect`

### Phase 4: Frontend Dashboard (Week 4)

#### 4.1 Setup & Configuration
- [ ] Create React app with Vite: `npm create vite@latest dashboard -- --template react-ts`
- [ ] Install shadcn/ui: `npx shadcn-ui@latest init`
- [ ] Configure Tailwind CSS with custom theme
- [ ] Set up React Query for data fetching
- [ ] Configure WebSocket connection

#### 4.2 Core Components
- [ ] TaskList component (shadcn Table)
- [ ] TaskDetail component (shadcn Card, Dialog)
- [ ] TaskFilters component (shadcn Input, Select)
- [ ] TaskStats component (shadcn Card, Badge)
- [ ] RetryButton component (shadcn Button)
- [ ] CancelButton component (shadcn AlertDialog)
- [ ] StatusBadge component (shadcn Badge)

**Example: TaskList Component**
```tsx
// dashboard/src/components/TaskList.tsx
import { useTasks } from '@/hooks/useTasks'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { StatusBadge } from './StatusBadge'
import { TaskActions } from './TaskActions'

export function TaskList() {
  const { tasks, isLoading } = useTasks()

  if (isLoading) return <div>Loading...</div>

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Task ID</TableHead>
          <TableHead>Name</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Scheduled</TableHead>
          <TableHead>Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {tasks.map((task) => (
          <TableRow key={task.id}>
            <TableCell className="font-mono">{task.id.slice(0, 8)}</TableCell>
            <TableCell>{task.name}</TableCell>
            <TableCell><StatusBadge status={task.status} /></TableCell>
            <TableCell>{formatDate(task.scheduled_at)}</TableCell>
            <TableCell><TaskActions task={task} /></TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
```

#### 4.3 Pages
- [ ] Dashboard page (`/`) - Stats + recent tasks
- [ ] Tasks list page (`/tasks`) - Filterable, paginated list
- [ ] Task detail page (`/tasks/:id`) - Full task details
- [ ] Dead letter page (`/dead-letter`) - Failed tasks view

#### 4.4 Real-time Updates
- [ ] WebSocket hook for live updates
- [ ] Auto-refresh task list on status change
- [ ] Toast notifications for task completion/failure

```tsx
// dashboard/src/hooks/useTaskUpdates.ts
import { useEffect } from 'react'
import { useWebSocket } from './useWebSocket'

export function useTaskUpdates() {
  const { lastMessage } = useWebSocket('ws://localhost:8080/api/v1/tasks/stream')

  useEffect(() => {
    if (lastMessage) {
      const message = JSON.parse(lastMessage.data)
      if (message.type === 'task_updated') {
        // Trigger refetch or update cache
        queryClient.invalidateQueries(['tasks'])
      }
    }
  }, [lastMessage])
}
```

### Phase 5: Security & Reliability (Week 5)

#### 5.1 Input Validation
- [ ] JSON schema validation for task payloads
- [ ] URL validation with SSRF protection
- [ ] Payload size limit (1MB)
- [ ] Rate limiting (100 requests/minute per IP)

#### 5.2 Callback Security
- [ ] HMAC-SHA256 signature on callbacks
- [ ] Include timestamp for replay attack prevention
- [ ] Document signature verification for receivers

**Callback Signature:**
```
X-Signature: sha256=<base64(hmac_sha256(shared_secret + timestamp + body))>
X-Timestamp: <unix_timestamp>
```

**Verification Guide (for receivers):**
```go
func verifyCallback(signature, timestamp, body, secret string) bool {
    // Reject if timestamp > 5 minutes old
    if time.Now().Unix() - parseInt(timestamp) > 300 {
        return false
    }

    expected := hmac_sha256(secret + timestamp + body)
    return constantTimeCompare(signature, expected)
}
```

#### 5.3 Idempotency
- [ ] Support optional `Idempotency-Key` header
- [ ] Return existing task ID if key matches
- [ ] Keys expire after 24 hours

#### 5.4 Observability
- [ ] Structured logging (JSON format)
- [ ] Request ID tracking (correlation IDs)
- [ ] Metrics: task submission rate, execution latency, callback success rate
- [ ] Prometheus metrics endpoint
- [ ] Health check endpoint

### Phase 6: Testing & Documentation (Week 6)

#### 6.1 Testing
- [ ] Unit tests for business logic (Go)
- [ ] Integration tests for API endpoints
- [ ] Callback delivery tests with mock server
- [ ] E2E tests for critical flows
- [ ] Load testing (1000 tasks/second)

#### 6.2 Documentation
- [ ] API documentation (OpenAPI/Swagger)
- [ ] Deployment guide (Docker, Kubernetes)
- [ ] Architecture diagram
- [ ] Callback receiver integration guide
- [ ] Troubleshooting guide

## Alternative Approaches Considered

### 1. Full Message Queue (RabbitMQ/Redis)
**Rejected because:**
- Adds operational complexity
- Requires external infrastructure
- Overkill for simple use cases

### 2. Cloud-Only Service (AWS SQS, Azure Queues)
**Rejected because:**
- Vendor lock-in
- Can be expensive for high volume
- Harder to run on-premise

### 3. In-Memory Only Queue
**Rejected because:**
- Data loss on restart
- No durability guarantees
- Can't inspect/debug tasks

### 4. Kubernetes CronJobs
**Rejected because:**
- No callback delivery
- Complex setup for dynamic tasks
- Limited observability

## Acceptance Criteria

### Functional Requirements

#### Task Submission
- [ ] Client can submit task via HTTP POST with callback URL
- [ ] Task is persisted to database with "pending" status
- [ ] Response includes task_id and scheduled timestamp
- [ ] Validation rejects invalid URLs, oversized payloads
- [ ] Idempotency key prevents duplicate submissions

#### Scheduling & Execution
- [ ] Tasks are executed at scheduled time (±30 seconds precision)
- [ ] Minimum scheduling delay: 1 minute
- [ ] Maximum scheduling delay: 365 days
- [ ] Worker pool processes concurrent tasks (default 20 workers)
- [ ] Task status transitions: pending → processing → completed/failed

#### Callback Delivery
- [ ] HTTP POST sent to callback_url on task completion
- [ ] Callback includes original payload + execution metadata
- [ ] Retry with exponential backoff on failures (max 5 retries)
- [ ] Circuit breaker opens after 5 consecutive failures to same URL
- [ ] HMAC signature prevents callback spoofing
- [ ] Timeout after 30 seconds (configurable)

#### Dashboard
- [ ] View list of all tasks with filters (status, date range, tags)
- [ ] Real-time updates via WebSocket
- [ ] Cancel pending tasks
- [ ] Retry failed tasks
- [ ] View task details with execution history
- [ ] See aggregated statistics

#### Dead Letter Handling
- [ ] Failed tasks marked "dead_lettered" after max retries
- [ ] Dead lettered tasks retained for 30 days
- [ ] Manual resurrection API available
- [ ] Dead letter view in dashboard

### Non-Functional Requirements

#### Performance
- [ ] Handle 1000 task submissions/second
- [ ] Execute scheduled tasks within 30 seconds of due time
- [ ] Callback delivery latency < 100ms (p50), < 1s (p95)
- [ ] Database query time < 50ms for indexed queries
- [ ] WebSocket message delivery < 100ms

#### Security
- [ ] SSRF protection prevents callbacks to internal IPs
- [ ] HMAC signature verification on callbacks
- [ ] Request size limits prevent DoS (1MB max payload)
- [ ] Rate limiting: 100 requests/minute per IP
- [ ] Input validation on all endpoints

#### Reliability
- [ ] No task loss on service restart (durable persistence)
- [ ] Automatic retry with exponential backoff
- [ ] Circuit breaker prevents cascading failures
- [ ] Graceful shutdown completes in-flight tasks
- [ ] Health check endpoint for orchestration

#### Usability
- [ ] API responses include clear error messages
- [ ] Dashboard loads within 2 seconds
- [ ] Real-time updates within 500ms of status change
- [ ] Mobile-responsive dashboard design

#### Maintainability
- [ ] Clean architecture separation (domain, usecase, repository, delivery)
- [ ] Test coverage > 80% for business logic
- [ ] Structured logging with correlation IDs
- [ ] API documentation auto-generated from OpenAPI spec

### Quality Gates

- [ ] All unit tests pass
- [ ] Integration tests cover critical paths
- [ ] Load test passes (1000 tasks/second sustained for 5 minutes)
- [ ] Security review completed (SSRF, DoS, callback spoofing)
- [ ] Code review by senior engineer
- [ ] Documentation complete (API, deployment, operations)

## Success Metrics

### Technical Metrics
- **Task Throughput**: > 1000 tasks/second submission rate
- **Execution Precision**: 95% of tasks executed within 30s of scheduled time
- **Callback Success Rate**: > 95% of callbacks succeed (excluding dead_lettered)
- **API Latency**: p50 < 50ms, p95 < 200ms for task submission
- **Uptime**: > 99.9% availability

### Business Metrics
- **Time to First Task**: < 5 minutes from service start
- **Dashboard Load Time**: < 2 seconds initial load
- **Setup Time**: < 15 minutes from clone to running service

### User Satisfaction
- **API Clarity**: < 5 minutes to submit first task (reading docs)
- **Debuggability**: Can view task execution trace in < 3 clicks
- **Error Recovery**: Can retry failed task in < 10 seconds

## Dependencies & Risks

### Dependencies

#### External Libraries
- **gin-gonic/gin**: MIT license, actively maintained
- **gorm.io/gorm**: MIT license, widely adopted
- **shadcn/ui**: MIT license, based on Radix UI

**Note**: Scheduling uses Go standard library `time.Ticker` - no external cron dependencies needed!

#### Infrastructure
- **PostgreSQL 15+**: Required for JSONB and partial indexes
- **Docker**: For development and deployment
- **Go 1.21+**: Language version requirement

### Risks & Mitigation

#### Risk 1: Database Performance at Scale
**Impact**: High
**Probability**: Medium
**Mitigation**:
- Use partial indexes for active queries
- Implement table partitioning for large datasets
- Add database connection pooling
- Regular query performance monitoring

#### Risk 2: Callback Timeout Exhausting Workers
**Impact**: High
**Probability**: Medium
**Mitigation**:
- Enforce strict timeout (default 30s)
- Circuit breaker for slow endpoints
- Separate callback delivery pool from execution pool
- Async callback with queue if needed

#### Risk 3: Security Vulnerabilities in Callbacks
**Impact**: Critical
**Probability**: Low
**Mitigation**:
- SSRF protection (allowlist or block private IPs)
- HMAC signature verification
- Rate limiting per callback URL
- Security audit before v1.0

#### Risk 4: Race Conditions in Distributed Workers
**Impact**: Medium
**Probability**: Low (MVP has single scheduler)
**Mitigation**:
- SELECT FOR UPDATE SKIP LOCKED for database-level locking
- Worker heartbeat and lease timeout
- Future: distributed lock (Redis etcd)

#### Risk 5: Large Payloads Causing Memory Issues
**Impact**: Medium
**Probability**: Low
**Mitigation**:
- Enforce 1MB payload limit
- Stream payloads for database operations
- Monitor memory usage and set alerts
- Document best practices for payload design

## Open Questions & Decisions Needed

### Priority 1: CRITICAL (blocks implementation)

#### Q1. Authentication Model
**Current Status**: Not specified for MVP
**Decision Required**:
- Option A: No authentication (MVP, single-user)
- Option B: API key based authentication (simple multi-tenant)
- Option C: JWT with user registration (full multi-user)

**Recommendation**: Start with Option A (no auth) for MVP, add API keys in v1.1

#### Q2. Callback Authentication Secret Management
**Current Status**: Need shared secret for HMAC
**Decision Required**:
- How are secrets generated and distributed?
- Per-user secret or global secret?
- Secret rotation strategy?

**Recommendation**: Global secret for MVP, per-user secrets with API key authentication

#### Q3. Task Execution Logic
**Current Status**: Vague - what does "execute" mean?
**Decision Required**:
- Option A: Tasks are just callbacks (no business logic in service)
- Option B: Support user-defined hook functions
- Option C: Built-in task types (HTTP request, email, etc.)

**Recommendation**: Option A for MVP - service only delivers callbacks, no business logic

### Priority 2: IMPORTANT (affects design)

#### Q4. Exact Callback Payload Format
**Decision Required**: What fields are included in callback?

**Proposed Format**:
```json
{
  "task_id": "uuid",
  "status": "completed",
  "scheduled_at": "2026-02-02T15:00:00Z",
  "executed_at": "2026-02-02T15:00:05Z",
  "duration_ms": 5123,
  "retry_count": 2,
  "callback_attempts": 1,
  "original_payload": { ... }
}
```

#### Q5. Data Retention Policy
**Decision Required**:
- How long to retain completed tasks? (Recommend: 7 days)
- How long to retain dead_lettered tasks? (Recommend: 30 days)
- Auto-cleanup or manual only?

#### Q6. WebSocket vs Polling for Dashboard
**Decision Required**:
- Primary: WebSocket for real-time updates
- Fallback: Polling every 5 seconds if WebSocket unavailable
- Mobile devices: Use polling only

## Implementation Tips (AI-Era Considerations)

### Prompts That Worked Well

#### Backend Architecture
```
"Implement a Go service using go-clean-arch pattern with Gin framework. Include domain models, repository layer with PostgreSQL, usecase layer for business logic, and delivery layer with Gin handlers. Use time.Ticker with tiered polling (2s for high-priority, 3s for normal) for task scheduling."
```

#### Database Schema Design
```
"Design a PostgreSQL schema for a task queue table that supports: JSONB payloads, task status tracking (pending, processing, completed, failed, dead_lettered), retry configuration with exponential backoff, callback tracking with attempts and timeouts. Include partial indexes for performance on active tasks."
```

#### Callback Retry Logic
```
"Implement exponential backoff retry logic in Go with these requirements: base backoff 60 seconds, multiplier 2, max backoff 24 hours, jitter to avoid thundering herd. Use circuit breaker pattern that opens after 5 consecutive failures and closes after 60 seconds."
```

#### React Component with shadcn
```
"Create a React TypeScript component using shadcn/ui components (Table, Badge, Card, Button) to display a list of tasks. Use React Query for data fetching, implement WebSocket connection for real-time updates, and support filtering by status."
```

### AI Tools Used
- **Claude Sonnet 4.5**: Architecture design, code implementation, documentation
- **GitHub Copilot**: Code completion, unit test generation
- **GPT-4**: Documentation review, alternative analysis

### Review Focus Areas
Given rapid AI-assisted development, prioritize review on:
1. **Security**: SSRF protection, callback authentication, rate limiting
2. **Error Handling**: Edge cases in retry logic, circuit breaker correctness
3. **Performance**: Database query optimization, connection pool sizing
4. **Testing**: Integration tests for callback delivery, load testing

## References & Research

### Internal References
- Project README: `docs/mvp/readme.md`
- Architecture reference: https://github.com/bxcodec/go-clean-arch

### External References

#### Go Clean Architecture
- [go-clean-arch GitHub](https://github.com/bxcodec/go-clean-arch)
- [Clean Architecture in Go: A Practical Guide](https://dev.to/leapcell/clean-architecture-in-go-a-practical-guide-with-go-clean-arch-51h7)

#### Task Scheduling
- [Go time.Ticker Documentation](https://pkg.go.dev/time#Ticker)
- [Tiered polling pattern explained](https://blog.cloudflare.com/using-go-s-ticker-for-timed-tasks/)
- **Why Ticker over Cron?**: Simpler, no dependencies, perfect for fixed-interval polling

#### Web Frameworks
- [Gin Web Framework](https://gin-gonic.com/docs/)
- [Goneed: Best Go frameworks 2026](https://dev.to/leapcell/best-go-frameworks-in-2026-top-picks-and-comparison-4ae6)

#### Database Design
- [Building a Scalable Job Queue in Golang with PostgreSQL](https://medium.com/@diasnour0395/building-a-scalable-job-queue-in-golang-with-postgresql-6ea352ad9540)
- [PostgreSQL Index Best Practices](https://www.postgresql.org/docs/15/indexes-types.html)

#### Callback & Webhook Patterns
- [GoLang HTTP Client with Circuit Breaker and Retry Backoff](https://medium.com/@diasnour0395/golang-http-client-with-circuit-breaker-and-retry-backoff-mechanism-d4def7029de8)
- [Webhook Best Practices](https://docs.sendgrid.com/for-developers/sending-events/event-webhook-security)

#### Frontend
- [shadcn/ui Documentation](https://ui.shadcn.com/)
- [TanStack Query (React Query)](https://tanstack.com/query/latest)
- [WebSocket in React](https://www.npmjs.com/package/react-use-websocket)

### Related Work
- Similar implementations: AWS SQS, Azure Queues, Iron.io
- Design inspiration: Sidekiq (Ruby), Celery (Python)

## File Structure Reference

### Backend Go Project
```
later/
├── cmd/
│   └── server/
│       └── main.go                    # Application entry point
├── internal/
│   ├── domain/
│   │   ├── models/
│   │   │   └── task.go                # Task entity and enums
│   │   └── repositories/
│   │       └── task_repository.go      # Repository interface
│   ├── usecase/
│   │   ├── task_service.go            # Business logic
│   │   ├── scheduler.go               # Task scheduling
│   │   ├── executor.go                # Task execution
│   │   └── callback_service.go        # Callback delivery
│   ├── repository/
│   │   └── postgres/
│   │       ├── task_repository.go      # PostgreSQL implementation
│   │       └── migrations/
│   │           └── 001_init_schema.up.sql
│   └── delivery/
│       └── gin/
│           ├── handler.go             # HTTP handlers
│           ├── middleware.go          # Logging, recovery, CORS
│           └── router.go              # Route setup
├── pkg/
│   ├── circuitbreaker/
│   │   └── circuit_breaker.go         # Circuit breaker implementation
│   ├── signature/
│   │   └── hmac.go                    # HMAC signature utilities
│   └── validator/
│       └── validator.go               # Request validation
├── configs/
│   ├── config.go                      # Configuration struct
│   └── config.yaml                    # Default config
├── migrations/
│   └── ...
├── go.mod
├── go.sum
└── README.md
```

### Frontend React Project
```
dashboard/
├── src/
│   ├── components/
│   │   ├── ui/                        # shadcn components
│   │   │   ├── button.tsx
│   │   │   ├── table.tsx
│   │   │   ├── badge.tsx
│   │   │   └── ...
│   │   ├── TaskList.tsx
│   │   ├── TaskDetail.tsx
│   │   ├── TaskFilters.tsx
│   │   ├── TaskStats.tsx
│   │   ├── StatusBadge.tsx
│   │   └── TaskActions.tsx
│   ├── hooks/
│   │   ├── useTasks.ts                # React Query hooks
│   │   ├── useWebSocket.ts            # WebSocket connection
│   │   └── useTaskUpdates.ts          # Real-time updates
│   ├── pages/
│   │   ├── Dashboard.tsx
│   │   ├── TaskList.tsx
│   │   ├── TaskDetail.tsx
│   │   └── DeadLetter.tsx
│   ├── lib/
│   │   ├── api.ts                     # API client
│   │   └── websocket.ts               # WebSocket client
│   ├── types/
│   │   └── task.ts                    # TypeScript interfaces
│   ├── App.tsx
│   └── main.tsx
├── public/
├── package.json
├── tailwind.config.js
├── tsconfig.json
└── vite.config.ts
```

## Next Steps

1. **Review this plan** and confirm technical approach
2. **Answer open questions** (authentication, callback payload, retention policy)
3. **Set up development environment** (PostgreSQL, Go, React)
4. **Begin Phase 1 implementation** (project setup, database schema)
5. **Schedule weekly review** to track progress

---

**Estimated Timeline**: 6 weeks for MVP
**Team Size**: 1-2 developers
**Complexity**: Medium (well-defined patterns, greenfield project)

**Ready to implement?** Let me know if you'd like to start with `/workflows:work` or run `/plan_review` for feedback from architecture reviewers.
