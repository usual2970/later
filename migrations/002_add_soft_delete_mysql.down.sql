-- Remove index
DROP INDEX idx_tasks_deleted_at ON task_queue;

-- Remove soft delete columns
ALTER TABLE task_queue
DROP COLUMN deleted_at,
DROP COLUMN deleted_by;
