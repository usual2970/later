-- Add soft delete columns
ALTER TABLE task_queue
ADD COLUMN deleted_at TIMESTAMP NULL DEFAULT NULL,
ADD COLUMN deleted_by VARCHAR(255) NULL DEFAULT NULL;

-- Add index for querying soft-deleted tasks
CREATE INDEX idx_tasks_deleted_at ON task_queue(deleted_at);

-- Add comment for documentation
ALTER TABLE task_queue COMMENT = 'Task queue with soft delete support - deleted_at marks tasks as deleted without removing them';
