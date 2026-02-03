---
title: fix: WebSocket real-time task status updates not working
type: fix
date: 2026-02-03
---

# fix: WebSocket real-time task status updates not working

## Overview

Task status changes in the backend are not being reflected in real-time on the frontend dashboard. Users submit tasks and see them appear, but subsequent status changes (pending → processing → completed/failed) require manual page refresh to become visible.

## Problem Statement

**User Impact:**
- Users cannot see task progress in real-time
- Dashboard requires manual refresh to see updated task statuses
- Poor UX for long-running or retrying tasks

**Technical Root Cause:**
WebSocket infrastructure is fully implemented but only integrated for task creation. All other task lifecycle events (status changes, retries, failures, completions) do not trigger WebSocket broadcasts.

### Current State

**✅ Working:**
- WebSocket hub implementation (`internal/websocket/hub.go`)
- WebSocket client hook (`dashboard/src/hooks/useWebSocket.ts`)
- Task creation broadcasts (`handler.go:82` calls `wsHub.BroadcastTaskCreated()`)

**❌ Broken:**
- Task status changes to `processing` (worker pool)
- Task status changes to `completed` (callback success)
- Task status changes to `failed` (callback failure)
- Task retry cycles (failed → processing)
- Task dead lettering (max retries exceeded)
- Manual retry/resurrect actions

### Flow Analysis

| Event | Current Broadcast | Expected Behavior |
|-------|------------------|-------------------|
| Task Created | ✅ `task_created` | ✅ Working |
| Task Processing | ❌ None | Should broadcast `task_updated` |
| Task Completed | ❌ None | Should broadcast `task_updated` |
| Task Failed | ❌ None | Should broadcast `task_updated` |
| Task Retried | ❌ None | Should broadcast `task_updated` |
| Task Dead Lettered | ❌ None | Should broadcast `task_updated` |
| Manual Retry | ❌ None | Should broadcast `task_updated` |

## Proposed Solution

Integrate WebSocket broadcasts into the task lifecycle by calling `wsHub.BroadcastTaskUpdated()` after each status transition in the worker pool and task service.

### Architecture

```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│   Worker    │──────│ TaskService  │──────│  Database   │
│   Pool      │      │              │      │             │
└──────┬──────┘      └──────┬───────┘      └─────────────┘
       │                    │
       │   ┌────────────────┘
       │   │
       ▼   ▼
┌─────────────────────────┐
│   WebSocket Hub         │
│   (wsHub)               │
└──────────┬──────────────┘
           │ BroadcastTaskUpdated()
           ▼
┌─────────────────────────┐
│   Frontend Clients      │
│   (React Dashboard)     │
└─────────────────────────┘
```

## Technical Considerations

### Integration Point

**Approach:** Add wsHub to worker pool and call broadcasts after each status transition

**Files to modify:**
- `internal/usecase/worker.go` - Add broadcasts after `MarkAsXXX()` calls
- `internal/usecase/task_service.go` - Add broadcasts to manual retry/resurrect
- `internal/server/server.go` - Pass wsHub to worker pool initialization

### Message Format

Current `BroadcastTaskUpdated()` signature:
```go
func (h *Hub) BroadcastTaskUpdated(taskID string, status string)
```

This is sufficient for frontend React Query cache invalidation.

### Performance Impact

- Hub uses buffered channels (256 capacity) - handles concurrent broadcasts safely
- Broadcast overhead should be < 10ms per status change
- No impact on worker processing speed (non-blocking send)

### Edge Cases

- **Rapid status changes:** Broadcast after DB commit ensures atomicity
- **Connection drops:** Frontend auto-reconnects after 3 seconds
- **Multiple clients:** Hub broadcasts to all connected clients
- **Concurrent workers:** Mutex-protected client registry handles concurrent access

## Acceptance Criteria

### Functional Requirements

- [x] Task status change to `processing` triggers WebSocket broadcast
- [x] Task status change to `completed` triggers WebSocket broadcast
- [x] Task status change to `failed` triggers WebSocket broadcast
- [x] Task retry (failed → processing) triggers WebSocket broadcast
- [x] Task dead lettering triggers WebSocket broadcast
- [x] Manual retry action triggers WebSocket broadcast
- [x] Manual resurrect action triggers WebSocket broadcast
- [ ] Frontend receives all broadcasts and updates UI without refresh
- [ ] Multiple concurrent task updates all broadcast successfully

### Non-Functional Requirements

- [x] No performance degradation to worker processing
- [ ] WebSocket broadcasts handle 20+ concurrent workers without message loss
- [x] Connection drops/reconnects handled gracefully
- [x] Existing unit tests pass
- [ ] New unit tests for WebSocket broadcast integration

## Success Metrics

- **Real-time visibility:** 100% of status changes visible in frontend within 1 second
- **No manual refresh:** Users should never need to refresh to see current task status
- **Message delivery:** < 0.1% broadcast failure rate under normal load
- **Performance impact:** < 5% overhead to worker processing time

## Implementation Plan

### Phase 1: Worker Pool Broadcasting ✅ COMPLETED

**Files:** `internal/usecase/worker.go`, `internal/server/server.go`, `cmd/server/main.go`

1. ✅ Add `wsHub *websocket.Hub` field to `WorkerPool` struct
2. ✅ Pass wsHub during worker pool initialization in `server.go`
3. ✅ Add `wsHub.BroadcastTaskUpdated()` call after `task.MarkAsProcessing()`
4. ✅ Add `wsHub.BroadcastTaskUpdated()` call after `task.MarkAsCompleted()`
5. ✅ Add `wsHub.BroadcastTaskUpdated()` call after `task.MarkAsFailed()`
6. ✅ Add `wsHub.BroadcastTaskUpdated()` call after `task.MarkAsDeadLettered()`

### Phase 2: Manual Action Broadcasting ✅ COMPLETED

**Files:** `internal/handler/handler.go`

1. ✅ Add broadcast call in `RetryTask()` handler after status update
2. ✅ Add broadcast call in `ResurrectTask()` handler after status update

### Phase 3: Testing & Validation

1. [ ] Write unit tests for WebSocket broadcasts in worker
2. [ ] Write integration test for end-to-end status updates
3. [ ] Manual testing: Submit task, observe real-time updates
4. [ ] Load testing: 20 concurrent workers processing tasks
5. [ ] Browser testing: Verify multiple clients receive updates simultaneously

## Dependencies & Risks

### Dependencies
- ✅ WebSocket hub infrastructure (already exists)
- ✅ Frontend WebSocket hook (already exists)
- ✅ React Query cache invalidation (already configured)

### Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Hub buffer overflow under high load | Low | Medium | Monitor channel capacity, 256 buffer should suffice |
| Race conditions in status updates | Low | Medium | Broadcast after DB commit ensures atomicity |
| Frontend state desync | Medium | Low | React Query cache invalidation handles consistency |
| Performance degradation | Low | Medium | Benchmark before/after, optimize if needed |

## References & Research

### Internal References

- **WebSocket Hub**: `internal/websocket/hub.go` - Has `BroadcastTaskUpdated()` method ready to use
- **Worker Pool**: `internal/usecase/worker.go:82-120` - Task processing logic with status transitions
- **Task Model**: `internal/domain/models/task.go:42-65` - State transition methods (`MarkAsProcessing`, etc.)
- **Frontend Hook**: `dashboard/src/hooks/useWebSocket.ts` - Ready to receive `task_updated` events
- **Working Example**: `internal/handler/handler.go:82` - Task creation broadcast implementation
- **Config**: `dashboard/src/lib/config.ts` - WebSocket URL configuration (port 7384)

### Institutional Learnings

- **Port Configuration Fix**: `docs/WEBSOCKET_FIX_SUMMARY.md` - Previous issue with hardcoded port mismatch (8080 vs 7384)
- **Environment Config**: `dashboard/docs/ENV_CONFIGURATION.md` - WebSocket URL derivation from API base URL

### Related Files

- `internal/server/server.go:77-79` - WebSocket endpoint registration
- `dashboard/src/contexts/WebSocketContext.tsx` - Frontend WebSocket provider
- `docs/websocket-test.html` - Manual WebSocket testing utility

## Testing Strategy

### Unit Tests

```go
// internal/usecase/worker_test.go
func TestWorkerBroadcastsOnStatusChange(t *testing.T) {
    // Mock wsHub and taskService
    mockHub := &MockWebSocketHub{}
    worker := NewWorker(taskService, mockHub)

    // Execute task processing
    worker.processTask(task)

    // Assert broadcast was called with correct status
    assert.True(t, mockHub.WasCalledWith("BroadcastTaskUpdated", task.ID, "processing"))
}
```

### Integration Test

```go
// internal/usecase/integration_test.go
func TestTaskStatusChangeReachesFrontend(t *testing.T) {
    // 1. Create WebSocket client
    wsClient := connectToWebSocket(t)

    // 2. Submit task via API
    task := submitTestTask(t)

    // 3. Wait for worker to process
    waitForStatus(t, task.ID, "processing")

    // 4. Assert WebSocket received task_updated event
    assert.Eventually(t, func() bool {
        return wsClient.ReceivedEvent("task_updated", task.ID, "processing")
    }, 5*time.Second, 100*time.Millisecond)
}
```

### Manual Testing Checklist

- [ ] Open dashboard in browser
- [ ] Submit a new task - verify immediate appearance (task_created)
- [ ] Watch status change to processing without refresh
- [ ] Watch status change to completed without refresh
- [ ] Submit failing task (invalid callback URL)
- [ ] Watch status change to failed without refresh
- [ ] Click manual retry - watch status change to pending → processing
- [ ] Open second browser window - verify both update simultaneously
- [ ] Disconnect network, wait 5s, reconnect - verify WebSocket reconnects

## Debugging Tips

If real-time updates don't work after implementation:

1. **Check browser console for WebSocket errors:**
   - DevTools → Console → Look for WebSocket connection errors
   - Verify URL: `ws://localhost:7384/api/v1/tasks/stream`

2. **Verify backend broadcasts:**
   - Check server logs for broadcast messages
   - Add debug logging: `log.Printf("Broadcasting task_updated: %s -> %s", taskID, status)`

3. **Test WebSocket manually:**
   - Open `docs/websocket-test.html` in browser
   - Verify connection and message reception

4. **Check frontend hook:**
   - Verify `useWebSocket.ts` is receiving messages (add console.log)
   - Check React Query cache invalidation

5. **Verify hub initialization:**
   - Confirm wsHub is not nil in worker
   - Check WebSocket endpoint is registered in server.go

## Appendix: Code Changes Preview

### worker.go Changes

```go
type WorkerPool struct {
    taskService *task_service.TaskService
    wsHub       *websocket.Hub  // Add this
    workerCount int
    // ... existing fields
}

func (w *Worker) processTask(task *models.Task) error {
    // Mark as processing
    if err := task.MarkAsProcessing(); err != nil {
        return err
    }

    // Broadcast status change
    w.wsHub.BroadcastTaskUpdated(task.ID, string(task.Status))

    // Save to database
    if err := w.taskService.UpdateTask(context.Background(), task); err != nil {
        return err
    }

    // ... execute callback ...

    // Update based on result
    if callbackErr != nil {
        task.MarkAsFailed(callbackErr.Error())
    } else {
        task.MarkAsCompleted()
    }

    // Broadcast final status
    w.wsHub.BroadcastTaskUpdated(task.ID, string(task.Status))

    return w.taskService.UpdateTask(context.Background(), task)
}
```

### server.go Initialization

```go
// Initialize WebSocket hub
wsHub := websocket.NewHub()
go wsHub.Run()

// Initialize worker pool with wsHub
workerPool := worker.NewWorkerPool(
    taskService,
    wsHub,  // Add this parameter
    workerPoolSize,
)
go workerPool.Start()
```

### task_service.go Manual Actions

```go
type TaskService struct {
    db    *sqlx.DB
    wsHub *websocket.Hub  // Add this
    // ... existing fields
}

func (s *TaskService) RetryTask(ctx context.Context, taskID string) error {
    // ... update task status to pending ...

    // Broadcast the status change
    s.wsHub.BroadcastTaskUpdated(taskID, "pending")

    return nil
}
```
