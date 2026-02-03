-- Create task_queue table
CREATE TABLE IF NOT EXISTS task_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
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
    max_retries INTEGER NOT NULL DEFAULT 5,
    retry_count INTEGER NOT NULL DEFAULT 0,
    retry_backoff_seconds INTEGER NOT NULL DEFAULT 60,
    next_retry_at TIMESTAMPTZ,

    -- Callback tracking
    callback_attempts INTEGER NOT NULL DEFAULT 0,
    callback_timeout_seconds INTEGER NOT NULL DEFAULT 30,
    last_callback_at TIMESTAMPTZ,
    last_callback_status INTEGER,
    last_callback_error TEXT,

    -- Metadata
    priority INTEGER NOT NULL DEFAULT 0 CHECK (priority >= 0 AND priority <= 10),
    tags TEXT[],
    error_message TEXT,
    worker_id VARCHAR(50),

    CHECK (retry_count <= max_retries)
);

-- Performance indexes for active queries
CREATE INDEX IF NOT EXISTS idx_tasks_status_scheduled_priority
ON task_queue(status, scheduled_at, priority DESC)
WHERE status IN ('pending', 'processing', 'failed');

CREATE INDEX IF NOT EXISTS idx_tasks_next_retry
ON task_queue(next_retry_at)
WHERE next_retry_at IS NOT NULL AND status = 'failed';

CREATE INDEX IF NOT EXISTS idx_tasks_created_at
ON task_queue(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_tasks_tags
ON task_queue USING GIN(tags);

-- Index for queries by status and user_id (future multi-tenancy)
CREATE INDEX IF NOT EXISTS idx_tasks_status_priority
ON task_queue(status, priority DESC)
WHERE status IN ('pending');

-- Comment the table
COMMENT ON TABLE task_queue IS 'Asynchronous task queue with HTTP callback support';
COMMENT ON COLUMN task_queue.payload IS 'Flexible JSON payload for task data';
COMMENT ON COLUMN task_queue.priority IS 'Task priority 0-10, higher is more urgent';
COMMENT ON COLUMN task_queue.next_retry_at IS 'When to retry failed tasks';
