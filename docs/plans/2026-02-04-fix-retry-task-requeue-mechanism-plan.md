---
title: fix: Retry task requeue mechanism
type: fix
date: 2026-02-04
---

# Fix: Retry Task Requeue Mechanism

## Overview

Tasks that fail and are scheduled for retry are never picked up again by the scheduler because they remain stuck in the `processing` state instead of being transitioned back to a state that the scheduler polls.

## Problem Statement

### Current Behavior (BROKEN)

```
1. Task created with status='pending' ✓
2. Scheduler polls FindDueTasks (WHERE status='pending') ✓
3. Worker marks task as 'processing' ✓
4. Callback fails → handleRetry called ✓
5. handleRetry sets NextRetryAt but KEEPS status='processing' ✗ BUG
6. Scheduler's FindDueTasks only looks for status='pending' ✗
7. Task never gets picked up again ✗
```

### Root Cause

**File:** `infrastructure/worker/pool.go:148-171`

The `handleRetry` method:
- Sets `NextRetryAt` ✓
- Increments `RetryCount` ✓
- **Does NOT transition task status** ✗

The task remains in `processing` state forever, and since the scheduler only queries `WHERE status = 'pending'` (repository/mysql/task_repository.go:93), retry tasks are never discovered.

### Impact

- **Tasks requiring retry are permanently stuck** after first failure
- **No automatic recovery** - manual database intervention required
- **Retry logic is non-functional** despite being implemented
- **Dead letter queue never gets tasks** that exceed max retries

## Proposed Solution

### Approach: Proper State Transition + Scheduler Enhancement

Transition failed tasks to `failed` status and update the scheduler to poll for retry-ready tasks.

#### Why This Approach?

1. **Follows existing patterns**: `MarkAsFailed()` already implements correct transition logic
2. **Clear semantics**: Failed tasks are explicitly marked, not ambiguously "pending"
3. **Uses existing code**: `FindFailedTasks()` repository method already exists but is unused
4. **Separation of concerns**: Worker handles execution results, scheduler handles polling
5. **Architecture alignment**: Keeps retry logic in use case layer, not infrastructure

## Technical Approach

### Phase 1: Fix Worker State Transition

**File:** `infrastructure/worker/pool.go:148-171`

Replace manual retry logic with entity's `MarkAsFailed()`:

```go
// handleRetry handles task retry with exponential backoff
func (w *Worker) handleRetry(task *entity.Task, callbackErr error) {
    ctx := context.Background()

    // Use entity method for proper state transition
    task.MarkAsFailed(callbackErr)

    // Update task in database
    if err := w.taskService.UpdateTask(ctx, task); err != nil {
        w.logger.Error("Failed to update task for retry",
            zap.Int("worker_id", w.id),
            zap.String("task_id", task.ID),
            zap.Error(err))
        return
    }

    w.logger.Info("Task marked as failed for retry",
        zap.Int("worker_id", w.id),
        zap.String("task_id", task.ID),
        zap.Int("retry_count", task.RetryCount),
        zap.Time("next_retry_at", *task.NextRetryAt))
}
```

**Changes:**
- Call `task.MarkAsFailed(callbackErr)` instead of manual state manipulation
- Add `callbackErr` parameter to `handleRetry` signature
- MarkAsFailed already:
  - Transitions status to `failed`
  - Increments `RetryCount`
  - Calculates `NextRetryAt` with exponential backoff
  - Sets `ErrorMessage`

### Phase 2: Update Scheduler to Poll Retry Tasks

**File:** `task/scheduler.go:94-120`

Add polling for failed tasks ready for retry:

```go
func (s *Scheduler) pollDueTasks(tier string, minPriority int, limit int) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Poll for pending tasks
    tasks, err := s.taskRepo.FindDueTasks(ctx, minPriority, limit)
    if err != nil {
        log.Printf("Failed to fetch due tasks (tier=%s): %v", tier, err)
        return
    }

    if len(tasks) == 0 {
        // Only poll for retries if no new pending tasks
        s.pollRetryTasks(tier, limit)
        return
    }

    log.Printf("Found %d due tasks (tier=%s)", len(tasks), tier)

    submitted := 0
    for _, task := range tasks {
        if s.workerPool.SubmitTask(task) {
            submitted++
        } else {
            log.Printf("Worker pool full, task will be retried next cycle: %s", task.ID)
        }
    }

    log.Printf("Tasks submitted to workers (tier=%s): %d/%d", tier, submitted, len(tasks))
}

func (s *Scheduler) pollRetryTasks(tier string, limit int) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Poll for failed tasks ready for retry
    retryTasks, err := s.taskRepo.FindFailedTasks(ctx, limit)
    if err != nil {
        log.Printf("Failed to fetch retry tasks (tier=%s): %v", tier, err)
        return
    }

    if len(retryTasks) == 0 {
        return
    }

    log.Printf("Found %d retry tasks (tier=%s)", len(retryTasks), tier)

    submitted := 0
    for _, task := range retryTasks {
        // Reset task to pending before resubmitting
        task.Status = entity.TaskStatusPending

        if s.workerPool.SubmitTask(task) {
            submitted++
        } else {
            log.Printf("Worker pool full, retry task will be retried next cycle: %s", task.ID)
        }
    }

    log.Printf("Retry tasks submitted to workers (tier=%s): %d/%d", tier, submitted, len(retryTasks))
}
```

**Changes:**
- Add new `pollRetryTasks()` method
- Call it when no pending tasks found
- Reset failed tasks to `pending` before worker submission
- Separate log messages for retry tasks vs new tasks

### Phase 3: Update Worker Call Site

**File:** `infrastructure/worker/pool.go:119-130`

Update the callback error handling to pass error to `handleRetry`:

```go
if callbackErr != nil {
    w.logger.Error("Task callback failed",
        zap.Int("worker_id", w.id),
        zap.String("task_id", task.ID),
        zap.Error(callbackErr))

    // Handle failure with retry logic
    if task.CanRetry() {
        w.handleRetry(task, callbackErr)  // Add callbackErr parameter
    } else {
        w.handleFailure(task, callbackErr)
    }
}
```

## Alternative Approaches Considered

### Approach A: Worker Transitions Back to Pending

Instead of using `MarkAsFailed()`, directly set status to `pending` in `handleRetry()`:

```go
task.Status = entity.TaskStatusPending
task.NextRetryAt = &nextRetryAt
task.RetryCount++
```

**Rejected because:**
- Loses semantic distinction between "new pending" and "failed retry"
- Harder to track retry metrics
- Doesn't use existing `MarkAsFailed()` pattern
- Doesn't utilize existing `FindFailedTasks()` repository method

### Approach B: Modify FindDueTasks Query

Add `OR status = 'failed' AND next_retry_at <= NOW()` to the existing query.

**Rejected because:**
- Mixing new and retry tasks in same query
- Loses retry-specific logging and metrics
- Harder to reason about query behavior
- Less flexible for different retry polling intervals

## Acceptance Criteria

### Functional Requirements

- [x] **Retry tasks are re-queued**: Tasks that fail with retryable errors are automatically picked up by the scheduler after `NextRetryAt`
- [x] **Proper state transitions**: Failed tasks transition from `processing` → `failed` → `pending` → `processing`
- [x] **Exponential backoff works**: Retry delay increases with each retry (60s, 120s, 240s, 480s...)
- [x] **Max retries respected**: Tasks exceeding `MaxRetries` are marked as `dead_lettered`
- [x] **Worker pool capacity respected**: Retry tasks are queued only when worker pool has capacity

### Non-Functional Requirements

- [x] **No duplicate processing**: `FOR UPDATE SKIP LOCKED` prevents multiple workers from picking up the same retry task
- [x] **Database query efficiency**: Separate queries for pending vs retry tasks maintain query performance
- [x] **Observability**: Distinct log messages for new tasks vs retry tasks
- [x] **Graceful degradation**: If worker pool is full, retry tasks are picked up in next poll cycle

### Quality Gates

- [x] **Unit tests**: Test state transitions in `handleRetry()`
- [x] **Integration tests**: Test end-to-end retry flow with scheduler polling
- [x] **Manual testing**: Verify retry tasks appear in dashboard
- [x] **Load testing**: Verify retry mechanism works under load with 20 concurrent workers

## Success Metrics

- **Retry success rate**: % of retry tasks that eventually complete successfully
- **Retry latency**: Average time from failure to retry execution
- **Dead letter rate**: % of tasks that exceed max retries
- **Worker pool utilization**: % of workers processing retry tasks vs new tasks

## Dependencies & Risks

### Dependencies

- None - this is a self-contained bug fix

### Risks

#### Risk 1: Scheduler Performance Impact

**Risk**: Additional query for retry tasks on every poll cycle could increase database load.

**Mitigation**:
- Only poll for retry tasks when no pending tasks found
- Use existing indexes on `(status, next_retry_at)`
- Monitor query performance with `EXPLAIN`

#### Risk 2: Retry Task Storm

**Risk**: Many retry tasks becoming ready simultaneously could overwhelm workers.

**Mitigation**:
- Tiered polling intervals (2s/3s/30s) naturally rate-limit
- Worker pool non-blocking submission provides backpressure
- Exponential backoff spreads retry tasks over time

#### Risk 3: Race Condition in State Reset

**Risk**: Setting task to `pending` in scheduler could race with worker marking it as `processing`.

**Mitigation**:
- `FOR UPDATE SKIP LOCKED` in `FindFailedTasks` prevents concurrent access
- Worker updates task to `processing` immediately upon receipt
- Transaction isolation prevents race conditions

## Implementation Phases

### Phase 1: Fix Worker (30 min)

- [x] Update `handleRetry()` to call `MarkAsFailed()`
- [x] Add `callbackErr` parameter to `handleRetry()`
- [x] Update call site in `processTask()`
- [x] Add unit tests for state transitions

### Phase 2: Update Scheduler (45 min)

- [x] Add `pollRetryTasks()` method
- [x] Update `pollDueTasks()` to call `pollRetryTasks()`
- [x] Reset failed tasks to `pending` before submission
- [x] Add integration tests for retry polling

### Phase 3: Testing & Validation (30 min)

- [x] Manual testing with failing callbacks
- [x] Verify dashboard shows retry tasks
- [x] Load testing with concurrent failures
- [x] Monitor database query performance

## References & Research

### Internal References

- **Bug location**: `infrastructure/worker/pool.go:148-171` - `handleRetry()` method
- **Entity state transitions**: `domain/entity/task.go:117-128` - `MarkAsFailed()` implementation
- **Scheduler polling**: `task/scheduler.go:94-120` - `pollDueTasks()` method
- **Repository queries**: `repository/mysql/task_repository.go:84-190` - `FindDueTasks()` and `FindFailedTasks()`
- **State machine**: `domain/entity/task.go:77-80` - `CanRetry()` logic

### External References

- **Exponential backoff with jitter**: `domain/entity/task.go:87-101` - `CalculateNextRetry()` implementation
- **Database locking pattern**: `repository/mysql/task_repository.go:99` - `FOR UPDATE SKIP LOCKED`
- **Worker pool pattern**: `infrastructure/worker/pool.go:284-292` - Non-blocking task submission

### Related Code

- **Callback error classification**: `callback/service.go:102-112` - Determines which errors are retryable
- **Dead letter handling**: `infrastructure/worker/pool.go:173-208` - `handleFailure()` implementation
- **Tiered polling config**: `task/scheduler.go:43-47` - Scheduler interval configuration

## Documentation Plan

### Update CLAUDE.md

Add to "Task Lifecycle" section:

```markdown
### Task Retry Flow

```
pending → processing → failed → pending → processing → completed
                            ↓
                   dead_lettered (after max_retries)
```

**Retry Process:**
1. Task fails in worker → marked as `failed` with `NextRetryAt`
2. Scheduler polls `FindFailedTasks()` for tasks with `next_retry_at <= NOW()`
3. Failed task reset to `pending` and submitted to worker
4. Retry delay follows exponential backoff: 60s, 120s, 240s, 480s...
```

### Database Schema Documentation

Document the retry-related fields:

- `next_retry_at` - Timestamp when task should be retried
- `retry_count` - Number of retry attempts
- `max_retries` - Maximum retry attempts before dead letter
- `status='failed'` - Task awaiting retry

## Open Questions

1. **Should retry polling use different intervals than pending tasks?**
   - Current plan: Same intervals, but only when no pending tasks
   - Alternative: Separate retry poll with configurable interval

2. **Should we add a dedicated `retrying` status?**
   - Current plan: Reuse `pending` after resetting from `failed`
   - Alternative: Add new `retrying` status for better observability

3. **How should we handle tasks that fail consistently?**
   - Current plan: Dead letter after max retries
   - Enhancement: Circuit breaker per callback URL (already implemented)
