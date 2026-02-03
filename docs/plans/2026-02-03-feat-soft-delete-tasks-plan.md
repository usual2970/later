---
title: Add Soft Delete Functionality to Tasks
type: feat
date: 2026-02-03
---

# Add Soft Delete Functionality to Tasks

## Overview

Implement soft delete functionality for tasks with user confirmation and state-based validation. Tasks in `pending` or `failed` status can be soft deleted (marked as deleted without removing from database), while tasks in `processing` or `completed` status are protected from deletion. This provides a safety net for accidental deletions and maintains audit history.

## Problem Statement / Motivation

**Current State:**
- The "Cancel Task" feature (internal/handler/handler.go:253-295) is a placeholder that doesn't actually delete tasks
- No deletion mechanism exists for removing failed tasks that are no longer needed
- No protection against accidental deletion of running or completed tasks
- No audit trail for deleted tasks

**Why This Matters:**
- Users need to clean up failed tasks that won't be retried
- Failed tasks clutter the UI and affect statistics
- Running tasks should be protected from deletion to prevent system inconsistencies
- Audit trail is important for debugging and compliance

## Proposed Solution

Implement soft delete functionality with the following characteristics:

1. **Soft Delete Mechanism**: Add `deleted_at` timestamp to mark tasks as deleted without removing them from the database
2. **State-Based Validation**: Only allow deletion of tasks in `pending` or `failed` status
3. **User Confirmation**: Require explicit confirmation before deletion
4. **No Restore UI**: Keep deleted tasks for audit purposes but don't provide restore functionality
5. **Permanent Retention**: Soft-deleted tasks are kept forever in the database

## Technical Considerations

### Architecture

**Schema Changes:**
```sql
ALTER TABLE task_queue ADD COLUMN deleted_at TIMESTAMP NULL;
ALTER TABLE task_queue ADD COLUMN deleted_by VARCHAR(255) NULL;
CREATE INDEX idx_tasks_deleted_at ON task_queue(deleted_at);
```

**Model Updates:**
- Add `DeletedAt *time.Time` field to `Task` struct in internal/domain/models/task.go
- Add `DeletedBy string` field for audit tracking
- Add `CanBeDeleted() bool` method to check if task can be deleted

**Repository Layer:**
- Add `SoftDelete(ctx, taskID, deletedBy) error` method to task_repository.go
- Update all SELECT queries to filter `WHERE deleted_at IS NULL`
- Add `ListDeleted(ctx, filter)` method to retrieve deleted tasks if needed

**Service Layer:**
- Update `DeleteTask` in task_service.go to call repository's `SoftDelete`
- Add validation to ensure task status allows deletion
- Broadcast deletion event via WebSocket

**Handler Layer:**
- Update `CancelTask` handler to perform soft delete instead of no-op
- Add proper confirmation in UI before calling delete endpoint
- Return appropriate error if task cannot be deleted

### Performance Implications

- **Query Performance**: Adding `deleted_at IS NULL` filter to all queries may slightly impact performance
- **Index**: The `idx_tasks_deleted_at` index will help optimize filtering
- **Database Size**: Soft-deleted tasks will accumulate over time (permanent retention policy)
- **Mitigation**: The permanent retention policy means no additional cleanup jobs needed

### Security Considerations

- **Audit Trail**: Track who deleted each task via `deleted_by` field
- **Authorization**: Ensure only authenticated users can delete tasks
- **State Protection**: Prevent deletion of processing tasks to avoid orphaned workers
- **Validation**: Server-side validation prevents bypassing UI restrictions

## Acceptance Criteria

### Functional Requirements

- [x] **Database Migration**: Add `deleted_at` and `deleted_by` columns to task_queue table
- [x] **Model Updates**: Task struct includes soft delete fields with proper JSON tags
- [x] **Can Delete Check**: Task.CanBeDeleted() returns true only for pending/failed tasks
- [x] **Soft Delete Method**: Repository.SoftDelete() sets deleted_at and deleted_by
- [x] **Query Filtering**: All task queries exclude soft-deleted tasks by default
- [x] **API Endpoint**: DELETE /api/tasks/:id performs soft delete with validation
- [x] **State Validation**: Returns 400 error if attempting to delete processing/completed task
- [x] **UI Confirmation**: Dashboard shows confirmation dialog before deletion
- [x] **Delete Button**: Only visible for pending and failed tasks in task list
- [x] **WebSocket Broadcast**: Task deletion broadcasts to all connected clients
- [x] **Audit Trail**: deleted_by field stores user/identifier who performed deletion
- [x] **Error Handling**: Proper error messages for invalid states and not found tasks

### Non-Functional Requirements

- [ ] **Response Time**: Delete operation completes within 100ms
- [ ] **UI Responsiveness**: Delete button state updates immediately after confirmation
- [ ] **Database Integrity**: Foreign key relationships are maintained
- [ ] **Backward Compatibility**: API responses include deleted_at field (null for existing tasks)

### Quality Gates

- [x] **Test Coverage**: Unit tests for CanBeDeleted() with all task states
- [ ] **Integration Tests**: Test soft delete API endpoint with various task states (deferred - requires integration test setup)
- [ ] **UI Tests**: Test delete button visibility and confirmation dialog (deferred - requires UI test setup)
- [ ] **Migration Testing**: Verify migration works on existing data (deferred - requires database setup)
- [ ] **Code Review**: Review for consistency with existing patterns

## Success Metrics

- **Adoption**: Number of tasks successfully soft deleted
- **Error Rate**: Percentage of delete attempts rejected due to invalid state (target: <5%)
- **User Safety**: Zero reports of accidental deletion of processing/completed tasks
- **Performance**: API response time p95 < 100ms for delete operations

## Dependencies & Risks

### Dependencies
- Existing task state management system
- WebSocket infrastructure for real-time updates
- React Query for state management in dashboard
- Radix UI Dialog component for confirmation

### Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Query performance degradation | Medium | Add index on deleted_at column; monitor query performance |
| Database size growth | Low | Permanent retention policy is acceptable for audit purposes |
| Race condition (task starts processing while deleting) | Medium | Check task status within database transaction |
| UI inconsistency across clients | Low | WebSocket broadcast ensures all clients update |
| Migration failure on production | High | Test migration on staging; add rollback migration |

## Implementation Approach

### Phase 1: Database & Model Layer

**Tasks:**
1. Create migration file: migrations/002_add_soft_delete_mysql.up.sql
2. Add down migration: migrations/002_add_soft_delete_mysql.down.sql
3. Update Task model in internal/domain/models/task.go
4. Add CanBeDeleted() method
5. Add IsDeleted() method

**Success Criteria:**
- Migration runs successfully without errors
- Model compiles with new fields
- Unit tests pass for new methods

### Phase 2: Repository Layer

**Tasks:**
1. Add SoftDelete() method to task_repository.go
2. Update FindByID to filter deleted tasks
3. Update List to filter deleted tasks
4. Update FindDueTasks to filter deleted tasks
5. Add ListDeleted() method (for admin/debug views)

**Files:**
- internal/repository/mysql/task_repository.go

**Success Criteria:**
- All repository methods exclude deleted tasks
- Unit tests for soft delete operation
- Integration tests verify query filtering

### Phase 3: Service & Handler Layer

**Tasks:**
1. Update task_service.go DeleteTask() to call SoftDelete
2. Add validation to check task.CanBeDeleted()
3. Update handler.go CancelTask endpoint
4. Add proper error responses for invalid states
5. Add WebSocket broadcast for deletion
6. Add unit tests for service validation

**Files:**
- internal/usecase/task_service.go
- internal/handler/handler.go

**Success Criteria:**
- Service validates task state before deletion
- Handler returns appropriate HTTP status codes
- WebSocket event is broadcast on deletion
- Tests cover all task states

### Phase 4: Dashboard UI

**Tasks:**
1. Update TaskList.tsx to show delete button for pending/failed tasks
2. Replace window.confirm() with proper Dialog component
3. Add delete confirmation dialog with task details
4. Update API client to call DELETE endpoint
5. Add optimistic UI updates
6. Handle WebSocket delete event to update UI

**Files:**
- dashboard/src/components/TaskList.tsx
- dashboard/src/api/tasks.ts (if exists, or similar)
- dashboard/src/hooks/useTasks.ts (if exists, or similar)

**Success Criteria:**
- Delete button only shows for eligible tasks
- Confirmation dialog displays task details
- UI updates immediately on successful deletion
- WebSocket events update all connected clients

### Phase 5: Testing & Documentation

**Tasks:**
1. Add unit tests for CanBeDeleted() with all states
2. Add integration tests for delete API endpoint
3. Add UI tests for delete button and confirmation
4. Update API documentation
5. Update CLAUDE.md with soft delete information

**Success Criteria:**
- Test coverage >90% for new code
- All tests pass
- Documentation is accurate and complete

## Database Migration

### Up Migration (migrations/002_add_soft_delete_mysql.up.sql)

```sql
-- Add soft delete columns
ALTER TABLE task_queue
ADD COLUMN deleted_at TIMESTAMP NULL DEFAULT NULL,
ADD COLUMN deleted_by VARCHAR(255) NULL DEFAULT NULL;

-- Add index for querying soft-deleted tasks
CREATE INDEX idx_tasks_deleted_at ON task_queue(deleted_at);

-- Add comment for documentation
ALTER TABLE task_queue COMMENT = 'Task queue with soft delete support - deleted_at marks tasks as deleted without removing them';
```

### Down Migration (migrations/002_add_soft_delete_mysql.down.sql)

```sql
-- Remove index
DROP INDEX idx_tasks_deleted_at ON task_queue;

-- Remove soft delete columns
ALTER TABLE task_queue
DROP COLUMN deleted_at,
DROP COLUMN deleted_by;
```

## References & Research

### Internal References

- **Current Cancel Handler**: internal/handler/handler.go:253-295
- **Task Model**: internal/domain/models/task.go
- **Task Repository**: internal/repository/mysql/task_repository.go
- **Current UI Pattern**: dashboard/src/components/TaskList.tsx:31-35
- **Database Schema**: migrations/001_init_schema_mysql.up.sql
- **WebSocket Hub**: internal/infrastructure/websocket/hub.go:130 (BroadcastTaskDeleted)

### External References

- MySQL Soft Delete Patterns: https://www.mysqltutorial.org/mysql-soft-delete/
- Go Time: Best practices for handling timestamps: https://go.dev/doc/effective_go#commentary
- Radix UI Dialog: https://www.radix-ui.com/docs/primitives/components/dialog/

## Related Work

- **WebSocket Real-time Updates**: docs/plans/2026-02-03-fix-websocket-realtime-task-updates-plan.md
- **Current Task Cleanup**: internal/repository/mysql/task_repository.go:331-368 (hard delete of old tasks)

## Future Considerations

### Potential Enhancements

1. **Restore Functionality**: If users need to restore deleted tasks, add UI for viewing and restoring soft-deleted tasks
2. **Bulk Delete**: Add ability to delete multiple tasks at once
3. **Retention Policy**: If database size becomes an issue, implement cleanup job for old soft-deleted tasks
4. **Soft Delete Cascade**: If other tables reference tasks, implement soft delete cascade
5. **Audit Log**: Add separate audit log table for all deletion events with timestamps and user info

### Extensibility

- The soft delete pattern can be extended to other entities in the future (e.g., callbacks, workers)
- The audit trail can be enhanced to include deletion reason
- The confirmation dialog pattern can be reused for other destructive operations
