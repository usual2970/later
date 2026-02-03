-- Create task_queue table (MySQL 8.0+ equivalent)
CREATE TABLE IF NOT EXISTS task_queue (
    id CHAR(36) PRIMARY KEY DEFAULT (UUID()),
    name VARCHAR(255) NOT NULL,
    payload JSON NOT NULL COMMENT 'Flexible JSON payload for task data',
    callback_url TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'dead_lettered')),

    -- Timing
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    scheduled_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,

    -- Retry configuration
    max_retries INTEGER NOT NULL DEFAULT 5,
    retry_count INTEGER NOT NULL DEFAULT 0,
    retry_backoff_seconds INTEGER NOT NULL DEFAULT 60,
    next_retry_at TIMESTAMP NULL COMMENT 'When to retry failed tasks',

    -- Callback tracking
    callback_attempts INTEGER NOT NULL DEFAULT 0,
    callback_timeout_seconds INTEGER NOT NULL DEFAULT 30,
    last_callback_at TIMESTAMP NULL,
    last_callback_status INTEGER NULL,
    last_callback_error TEXT,

    -- Metadata
    priority INTEGER NOT NULL DEFAULT 0 CHECK (priority >= 0 AND priority <= 10) COMMENT 'Task priority 0-10, higher is more urgent',
    tags JSON,
    error_message TEXT,
    worker_id VARCHAR(50),

    CHECK (retry_count <= max_retries)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Asynchronous task queue with HTTP callback support';

-- Performance indexes
CREATE INDEX idx_tasks_status_scheduled_priority
ON task_queue(status, scheduled_at, priority DESC);

CREATE INDEX idx_tasks_next_retry
ON task_queue(next_retry_at);

CREATE INDEX idx_tasks_created_at
ON task_queue(created_at DESC);

CREATE INDEX idx_tasks_status_priority
ON task_queue(status, priority DESC);

-- Note: MySQL doesn't support GIN indexes. For tag queries:
-- - Option 1: Use JSON functional index (MySQL 8.0.17+)
--   CREATE INDEX idx_tasks_tags ON task_queue((CAST(tags AS CHAR(200) ARRAY)));
-- - Option 2: Normalize to separate task_tags table for better performance
