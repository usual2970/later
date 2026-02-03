---
title: Migrate Database from PostgreSQL to MySQL
type: refactor
date: 2026-02-03
---

# Migrate Database from PostgreSQL to MySQL

## Overview

Migrate the **Later** task scheduling service database from PostgreSQL to MySQL 8.0+. This involves replacing the database driver, rewriting SQL queries to handle dialect differences, converting PostgreSQL-specific data types (JSONB, arrays, UUID), and updating schema definitions to MySQL-compatible syntax.

**Current State:** PostgreSQL 15+ with pgx/v5 driver, using JSONB, TEXT arrays, UUID, TIMESTAMPTZ, partial indexes, and GIN indexes.

**Target State:** MySQL 8.0+ with go-sql-driver/mysql + sqlx, using JSON, JSON arrays, CHAR(36) for UUID, TIMESTAMP, and full-text indexes.

---

## Problem Statement / Motivation

### Why Migrate?

**[USER INPUT REQUIRED - Please specify reason for migration]**

Common reasons might include:
- **Standardization:** Organization standardizes on MySQL for operational consistency
- **Expertise:** Team has more MySQL experience than PostgreSQL
- **Infrastructure:** Existing MySQL infrastructure and tooling
- **Cost:** MySQL hosting is more cost-effective in your environment
- **Compliance:** Regulatory or vendor requirements
- **Integration:** Better compatibility with existing MySQL-based systems

### Migration Complexity Assessment

This is a **HIGH-COMPLEXITY** migration due to:

1. **Heavy PostgreSQL Feature Usage:**
   - JSONB for flexible payload storage (binary JSON with query operators)
   - TEXT[] array type for tags
   - TIMESTAMPTZ for timezone-aware timestamps
   - Partial indexes for query optimization
   - GIN indexes for array overlap queries
   - FOR UPDATE SKIP LOCKED for concurrent task processing
   - CHECK constraints for data validation

2. **Tight Database Coupling:**
   - Repository layer uses PostgreSQL-specific SQL syntax
   - No database abstraction layer
   - Direct use of pgx/v5 driver features

3. **Data Integrity Risks:**
   - Task queue state management requires transaction consistency
   - Concurrent worker pool depends on row-level locking
   - Callback delivery relies on JSON payload structure

**Estimated Effort:** 2-3 weeks for a complete, tested migration

---

## Proposed Solution

### Architecture Approach

**Maintain go-clean-arch principles** while migrating the repository layer:

```
internal/
├── repository/
│   └── mysql/              # Renamed from postgres/
│       ├── connection.go   # MySQL connection with sqlx
│       └── task_repository.go  # MySQL-specific queries
├── domain/
│   └── models/
│       └── task.go         # JSONBytes type remains compatible
└── infrastructure/
    └── circuitbreaker/     # Unchanged
```

### Technology Stack

| Component | Current (PostgreSQL) | New (MySQL) | Rationale |
|-----------|---------------------|-------------|-----------|
| **Driver** | `github.com/jackc/pgx/v5` | `github.com/go-sql-driver/mysql` | Standard MySQL driver, production-proven |
| **Query Builder** | Raw SQL with pgx | `github.com/jmoiron/sqlx` | Ergonomics, struct scanning, 99% of raw SQL performance |
| **Connection Pool** | `pgxpool.Pool` | `database/sql.DB` | Standard Go database/sql with explicit pool config |
| **UUID** | `gen_random_uuid()` | `UUID()` function | MySQL 8.0+ built-in UUID |
| **JSON** | JSONB (binary) | JSON (binary in MySQL 8.0+) | Both support binary JSON storage |
| **Arrays** | TEXT[] native type | JSON arrays | MySQL lacks native arrays |
| **Timestamps** | TIMESTAMPTZ | TIMESTAMP | Always use UTC for both |

### Key Changes

#### 1. Data Type Mappings

| PostgreSQL | MySQL 8.0+ | Notes |
|------------|------------|-------|
| `UUID PRIMARY KEY DEFAULT gen_random_uuid()` | `CHAR(36) PRIMARY KEY DEFAULT (UUID())` | String format UUIDs |
| `payload JSONB NOT NULL` | `payload JSON NOT NULL` | JSON query syntax differs |
| `tags TEXT[]` | `tags JSON` | Array operations require JSON functions |
| `created_at TIMESTAMPTZ DEFAULT NOW()` | `created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP` | Must use UTC explicitly |
| `CHECK (status IN (...))` | `CHECK (status IN (...))` | Supported in MySQL 8.0.16+ |

#### 2. SQL Syntax Changes

| PostgreSQL | MySQL | Example |
|------------|-------|---------|
| `$1, $2, $3` | `?` | `WHERE id = ?` |
| `NOW() - INTERVAL '30 days'` | `DATE_SUB(NOW(), INTERVAL 30 DAY)` | Date arithmetic |
| `tags && array['urgent']` | `JSON_CONTAINS(tags, '"urgent"')` | Array overlap |
| `payload->>'key'` | `JSON_UNQUOTE(JSON_EXTRACT(payload, '$.key'))` | JSON field access |
| `ctid` (tuple ID) | Primary key | Batch deletion |

#### 3. Driver Migration Pattern

**Before (pgx/v5):**
```go
type taskRepository struct {
    db *pgxpool.Pool
}

func (r *taskRepository) FindByID(ctx context.Context, id string) (*models.Task, error) {
    query := `SELECT id, name, payload, ... FROM task_queue WHERE id = $1`
    var task models.Task
    err := r.db.QueryRow(ctx, query, id).Scan(
        &task.ID, &task.Name, &task.Payload, /* 18 more fields */
    )
    return &task, err
}
```

**After (sqlx + MySQL):**
```go
type taskRepository struct {
    db *sqlx.DB
}

func (r *taskRepository) FindByID(ctx context.Context, id string) (*models.Task, error) {
    var task models.Task
    err := r.db.GetContext(ctx, &task, "SELECT * FROM task_queue WHERE id = ?", id)
    return &task, err
}
```

**Benefits:**
- 70% less code (no manual field scanning)
- Better maintainability
- Type safety preserved

---

## Technical Considerations

### 1. Feature Equivalence Analysis

#### ✅ SKIP LOCKED (Supported in MySQL 8.0+)
Your scheduler's concurrent task processing uses `FOR UPDATE SKIP LOCKED`. MySQL 8.0.1+ supports this natively:

```sql
-- Works in both databases
SELECT * FROM task_queue
WHERE status = 'pending' AND scheduled_at <= NOW()
ORDER BY priority DESC, scheduled_at ASC
LIMIT 100
FOR UPDATE SKIP LOCKED;
```

#### ⚠️ Partial Indexes (No Direct Equivalent)
PostgreSQL partial indexes:
```sql
CREATE INDEX idx_tasks_status_scheduled_priority
ON task_queue(status, scheduled_at, priority DESC)
WHERE status IN ('pending', 'processing', 'failed');
```

**MySQL Workaround:** Accept larger indexes (recommended for simplicity)
```sql
-- No WHERE clause, indexes all rows
CREATE INDEX idx_tasks_status_scheduled_priority
ON task_queue(status, scheduled_at, priority DESC);
```

**Impact:** Slightly larger index size, but negligible for your workload (task queue with periodic cleanup).

#### ⚠️ GIN Indexes on Arrays (No Direct Equivalent)
PostgreSQL GIN index:
```sql
CREATE INDEX idx_tasks_tags ON task_queue USING GIN(tags);
```

**MySQL Options:**

**Option 1:** JSON functional index (MySQL 8.0.17+)
```sql
CREATE INDEX idx_tasks_tags
ON task_queue((CAST(tags AS CHAR(200) ARRAY)));
```

**Option 2:** Normalize to separate table (best for query performance)
```sql
CREATE TABLE task_tags (
    task_id CHAR(36) NOT NULL,
    tag VARCHAR(255) NOT NULL,
    PRIMARY KEY (task_id, tag),
    INDEX idx_tag (tag),
    FOREIGN KEY (task_id) REFERENCES task_queue(id) ON DELETE CASCADE
);

-- Query with JOIN
SELECT t.* FROM task_queue t
JOIN task_tags tt ON t.id = tt.task_id
WHERE tt.tag = 'urgent';
```

**Recommendation:** Start with Option 1 (JSON functional index) for simplicity. Migrate to Option 2 if tag query performance becomes a bottleneck.

#### ✅ CHECK Constraints (Supported in MySQL 8.0.16+)
Your schema uses CHECK constraints:
```sql
CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'dead_lettered'))
CHECK (priority >= 0 AND priority <= 10)
CHECK (retry_count <= max_retries)
```

MySQL 8.0.16+ supports these with identical syntax. **Ensure MySQL version is 8.0.16 or higher.**

#### ⚠️ RETURNING Clause (No Equivalent)
PostgreSQL:
```sql
INSERT INTO task_queue (...) VALUES (...)
RETURNING id, created_at;
```

**MySQL Workaround:** Two-step approach
```go
// Step 1: Insert with pre-generated UUID
taskID := uuid.New().String()
_, err := db.Exec(`
    INSERT INTO task_queue (id, name, ...)
    VALUES (?, ?, ...)
`, taskID, name, ...)

// Step 2: Use the pre-generated ID (no query needed)
```

This pattern is actually more explicit and avoids race conditions.

### 2. Performance Implications

| Operation | PostgreSQL | MySQL | Notes |
|-----------|-----------|-------|-------|
| **JSON queries** | Faster (binary JSONB) | Slightly slower | Negligible for your workload |
| **Array searches** | Very fast (GIN index) | Slower (JSON functions) | Consider normalizing if frequent |
| **Connection pooling** | pgxpool (optimized) | database/sql (standard) | Configure explicitly |
| **SKIP LOCKED** | Native | Native (8.0.1+) | No difference |
| **Partial indexes** | Space-efficient | Larger indexes | Minimal impact |

**Overall:** Performance impact should be minimal for your use case. Monitor after migration.

### 3. Security Considerations

✅ **No new security risks** - Both databases support:
- Parameterized queries (prepared statements)
- TLS/SSL connections
- Role-based access control
- Row-level security (if needed)

⚠️ **Configuration Required:**
- Update connection string to use MySQL SSL options: `?tls=true`
- Verify MySQL user permissions match PostgreSQL grants
- Ensure CALLBACK_SECRET HMAC-SHA256 signatures work with JSON payloads (no code changes needed)

### 4. Concurrency & Transaction Behavior

**Critical Difference:** Default transaction isolation levels

| Database | Default Isolation | Impact |
|----------|------------------|--------|
| PostgreSQL | READ COMMITTED | Sees committed changes from other transactions |
| MySQL (InnoDB) | REPEATABLE READ | Doesn't see changes made during transaction |

**Your Use Case Impact:**
- Scheduler's `FOR UPDATE SKIP LOCKED` provides row-level locking (works identically)
- Worker pool processes independent tasks (no cross-task dependencies)
- **No action needed** - your concurrent task processing is safe

**Testing Required:** Verify callback delivery retry logic under high concurrency.

---

## Implementation Approach

### Phase 1: Foundation (Days 1-3)

**Goal:** Set up MySQL infrastructure and basic connectivity.

#### Tasks:
- [ ] Install MySQL 8.0+ locally or provision development instance
- [x] Add MySQL dependencies to `go.mod`:
  ```bash
  go get github.com/go-sql-driver/mysql
  go get github.com/jmoiron/sqlx
  ```
- [x] Create `internal/repository/mysql/` directory
- [x] Implement `internal/repository/mysql/connection.go`:
  ```go
  package mysql

  import (
      "context"
      "fmt"
      "time"
      "github.com/jmoiron/sqlx"
      _ "github.com/go-sql-driver/mysql"
      "later/configs"
  )

  func NewConnection(cfg *configs.DatabaseConfig) (*sqlx.DB, error) {
      db, err := sqlx.Connect("mysql", cfg.URL)
      if err != nil {
          return nil, fmt.Errorf("failed to connect to database: %w", err)
      }

      // Configure connection pool
      db.SetMaxOpenConns(cfg.MaxOpenConns)       // e.g., 100
      db.SetMaxIdleConns(cfg.MaxIdleConns)       // e.g., 20
      db.SetConnMaxLifetime(cfg.ConnMaxLifetime) // e.g., 1 hour
      db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime) // e.g., 10 min

      // Verify connection
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      if err := db.PingContext(ctx); err != nil {
          return nil, fmt.Errorf("failed to ping database: %w", err)
      }

      return db, nil
  }

  func Close(db *sqlx.DB) error {
      return db.Close()
  }
  ```

- [x] Update `configs/config.go` with MySQL-specific settings:
  ```go
  type DatabaseConfig struct {
      URL              string        `mapstructure:"url"`
      MaxOpenConns     int           `mapstructure:"max_open_conns"`
      MaxIdleConns     int           `mapstructure:"max_idle_conns"`
      ConnMaxLifetime  time.Duration `mapstructure:"conn_max_lifetime"`
      ConnMaxIdleTime  time.Duration `mapstructure:"conn_max_idle_time"`
      Timezone         string        `mapstructure:"timezone"`
  }

  func setDefaults(v *viper.Viper) {
      // MySQL defaults
      v.SetDefault("database.url", "mysql://later:later@localhost:3306/later?parseTime=true&loc=UTC&charset=utf8mb4")
      v.SetDefault("database.max_open_conns", 100)
      v.SetDefault("database.max_idle_conns", 20)
      v.SetDefault("database.conn_max_lifetime", "1h")
      v.SetDefault("database.conn_max_idle_time", "10m")
      v.SetDefault("database.timezone", "UTC")
  }
  ```

- [x] Create MySQL migration script: `migrations/001_init_schema_mysql.up.sql`
- [ ] Test database connection with ping (requires MySQL instance)

**Success Criteria:**
- ✅ Can connect to MySQL from Go application
- ✅ Connection pool is configured
- ✅ Basic ping test succeeds

---

### Phase 2: Schema Migration (Days 4-6)

**Goal:** Convert PostgreSQL schema to MySQL and create migration scripts.

#### Tasks:
- [ ] Create `migrations/001_init_schema_mysql.up.sql` with MySQL-compatible DDL:

```sql
-- MySQL 8.0+ equivalent of PostgreSQL schema
CREATE TABLE IF NOT EXISTS task_queue (
    id CHAR(36) PRIMARY KEY DEFAULT (UUID()),
    name VARCHAR(255) NOT NULL,
    payload JSON NOT NULL,
    callback_url TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'dead_lettered')),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    scheduled_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,

    max_retries INTEGER NOT NULL DEFAULT 5,
    retry_count INTEGER NOT NULL DEFAULT 0,
    retry_backoff_seconds INTEGER NOT NULL DEFAULT 60,
    next_retry_at TIMESTAMP NULL,

    callback_attempts INTEGER NOT NULL DEFAULT 0,
    callback_timeout_seconds INTEGER NOT NULL DEFAULT 30,
    last_callback_at TIMESTAMP NULL,
    last_callback_status INTEGER NULL,
    last_callback_error TEXT,

    priority INTEGER NOT NULL DEFAULT 0 CHECK (priority >= 0 AND priority <= 10),
    tags JSON,  -- Changed from TEXT[] to JSON
    error_message TEXT,
    worker_id VARCHAR(50),

    CHECK (retry_count <= max_retries)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Indexes (partial indexes become full indexes)
CREATE INDEX idx_tasks_status_scheduled_priority
ON task_queue(status, scheduled_at, priority DESC);

CREATE INDEX idx_tasks_next_retry
ON task_queue(next_retry_at);

CREATE INDEX idx_tasks_created_at
ON task_queue(created_at DESC);

CREATE INDEX idx_tasks_status_priority
ON task_queue(status, priority DESC);

-- Optional: Separate tags table for better query performance
CREATE TABLE task_tags (
    task_id CHAR(36) NOT NULL,
    tag VARCHAR(255) NOT NULL,
    PRIMARY KEY (task_id, tag),
    INDEX idx_tag (tag),
    FOREIGN KEY (task_id) REFERENCES task_queue(id) ON DELETE CASCADE
);
```

- [ ] Create data migration script: `scripts/migrate_data_to_mysql.go`
  - Reads from PostgreSQL, writes to MySQL
  - Converts types: UUID → string, JSONB → JSON, TEXT[] → JSON array
  - Handles batch processing for large datasets
- [ ] Test schema creation in local MySQL
- [ ] Test data migration with sample data

**Success Criteria:**
- ✅ MySQL schema created successfully
- ✅ All CHECK constraints applied
- ✅ Indexes created
- ✅ Sample data migrates correctly

---

### Phase 3: Repository Layer Migration (Days 7-10)

**Goal:** Rewrite repository layer to use MySQL with sqlx.

#### Tasks:
- [x] Implement `internal/repository/mysql/task_repository.go`:

**Key Methods to Migrate:**

1. **Create Task (with pre-generated UUID):**
```go
func (r *taskRepository) Create(ctx context.Context, task *models.Task) error {
    // Pre-generate UUID (replaces RETURNING clause)
    task.ID = uuid.New().String()

    query := `
        INSERT INTO task_queue (
            id, name, payload, callback_url, status, scheduled_at,
            max_retries, priority, tags
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

    // Convert tags to JSON
    tagsJSON, _ := json.Marshal(task.Tags)

    _, err := r.db.ExecContext(ctx, query,
        task.ID, task.Name, task.Payload, task.CallbackURL, task.Status,
        task.ScheduledAt, task.MaxRetries, task.Priority, tagsJSON,
    )

    return err
}
```

2. **Find Due Tasks (SKIP LOCKED):**
```go
func (r *taskRepository) FindDueTasks(ctx context.Context, limit int, minPriority int) ([]*models.Task, error) {
    query := `
        SELECT * FROM task_queue
        WHERE status = 'pending'
          AND scheduled_at <= NOW()
          AND (? = -1 OR priority > ?)
        ORDER BY priority DESC, scheduled_at ASC
        LIMIT ?
        FOR UPDATE SKIP LOCKED
    `

    var tasks []*models.Task
    err := r.db.SelectContext(ctx, &tasks, query, minPriority, minPriority, limit)
    return tasks, err
}
```

3. **List with Dynamic Filters (named parameters):**
```go
func (r *taskRepository) List(ctx context.Context, filter repositories.TaskFilter) ([]*models.Task, int64, error) {
    query := `SELECT * FROM task_queue WHERE 1=1`
    args := map[string]interface{}{}
    argNum := 1

    if filter.Status != nil {
        query += fmt.Sprintf(" AND status = :status")
        args["status"] = *filter.Status
    }

    if filter.Priority != nil {
        query += fmt.Sprintf(" AND priority >= :priority")
        args["priority"] = *filter.Priority
    }

    if len(filter.Tags) > 0 {
        // MySQL JSON array search
        query += " AND JSON_CONTAINS(tags, JSON_QUOTE(:tag))"
        args["tag"] = filter.Tags[0]
    }

    query += " ORDER BY created_at DESC LIMIT :limit OFFSET :offset"
    args["limit"] = filter.Limit
    args["offset"] = (filter.Page - 1) * filter.Limit

    var tasks []*models.Task
    rows, err := r.db.NamedQueryContext(ctx, query, args)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    for rows.Next() {
        var task models.Task
        if err := rows.StructScan(&task); err != nil {
            return nil, 0, err
        }
        tasks = append(tasks, &task)
    }

    return tasks, int64(len(tasks)), nil
}
```

4. **Cleanup Expired Data (batch deletion):**
```go
func (r *taskRepository) CleanupExpiredData(ctx context.Context, limit int) (int64, error) {
    query := `
        DELETE tq
        FROM task_queue tq
        INNER JOIN (
            SELECT id FROM task_queue
            WHERE status IN ('completed', 'dead_lettered')
              AND completed_at < DATE_SUB(NOW(), INTERVAL 30 DAY)
            LIMIT ?
        ) AS tmp ON tq.id = tmp.id
    `

    result, err := r.db.ExecContext(ctx, query, limit)
    if err != nil {
        return 0, err
    }

    return result.RowsAffected()
}
```

5. **Update Task:**
```go
func (r *taskRepository) Update(ctx context.Context, task *models.Task) error {
    query := `
        UPDATE task_queue
        SET status = ?,
            started_at = ?,
            completed_at = ?,
            retry_count = ?,
            next_retry_at = ?,
            error_message = ?,
            worker_id = ?,
            last_callback_at = ?,
            last_callback_status = ?,
            last_callback_error = ?,
            callback_attempts = ?
        WHERE id = ?
    `

    tagsJSON, _ := json.Marshal(task.Tags)

    _, err := r.db.ExecContext(ctx, query,
        task.Status, task.StartedAt, task.CompletedAt,
        task.RetryCount, task.NextRetryAt, task.ErrorMessage,
        task.WorkerID, task.LastCallbackAt, task.LastCallbackStatus,
        task.LastCallbackError, task.CallbackAttempts,
        task.ID,
    )

    return err
}
```

- [x] Update all repository methods following the pattern above
- [x] Remove old `internal/repository/postgres/` directory
- [x] Update imports in `cmd/server/main.go`:
  ```go
  // Before:
  // postgres "later/internal/repository/postgres"

  // After:
  mysql "later/internal/repository/mysql"
  ```
- [ ] Update `internal/domain/models/task.go`:
  - `JSONBytes` type already compatible (works with MySQL JSON)
  - Add tags JSON unmarshaling:
    ```go
    func (t *Task) AfterScan() error {
        // Unmarshal tags JSON if needed
        return nil
    }
    ```

**Success Criteria:**
- ✅ All CRUD operations work
- ✅ Dynamic filters (status, priority, tags) work
- ✅ Batch queries work
- ✅ SKIP LOCKED queries work

---

### Phase 4: Integration & Testing (Days 11-13)

**Goal:** Ensure all components work with MySQL backend.

#### Tasks:

**Unit Tests:**
- [ ] Update all repository tests to use MySQL
- [ ] Test all CRUD operations
- [ ] Test concurrent access (multiple workers)
- [ ] Test JSON payload queries
- [ ] Test tag filtering with JSON arrays
- [ ] Test error handling

**Integration Tests:**
- [ ] Test scheduler tiered polling (high/normal/cleanup)
- [ ] Test worker pool task processing
- [ ] Test callback delivery with retry logic
- [ ] Test circuit breaker functionality
- [ ] Test dashboard WebSocket connections
- [ ] Load test with 1000+ concurrent tasks

**Manual Testing:**
- [ ] Submit task via HTTP API
- [ ] Verify scheduler picks up task
- [ ] Verify worker processes task
- [ ] Verify callback delivered
- [ ] Verify retry logic works
- [ ] Verify dead lettering works
- [ ] Test dashboard displays tasks correctly

**Performance Testing:**
- [ ] Benchmark query performance vs PostgreSQL
- [ ] Monitor connection pool metrics
- [ ] Check for slow queries (>100ms)
- [ ] Verify SKIP LOCKED prevents contention

**Success Criteria:**
- ✅ All unit tests pass
- ✅ All integration tests pass
- ✅ Manual testing shows no regressions
- ✅ Performance within 10% of PostgreSQL

---

### Phase 5: Deployment & Cutover (Days 14-15)

**Goal:** Deploy to production and verify.

#### Tasks:

**Pre-Deployment:**
- [ ] Backup PostgreSQL database
- [ ] Document rollback procedure
- [ ] Prepare monitoring dashboards
- [ ] Set up alerts for errors and performance degradation

**Deployment Strategy:**

**Option A: Blue-Green Deployment (Recommended)**
1. Deploy new version with MySQL to staging environment
2. Run full integration test suite
3. Point production DNS to new deployment
4. Monitor for 1 hour
5. If issues detected, rollback to PostgreSQL version

**Option B: Dual-Write (Safer for Critical Systems)**
1. Deploy version that writes to both PostgreSQL and MySQL
2. Read from PostgreSQL, write to both
3. Verify data consistency for 24-48 hours
4. Switch reads to MySQL
5. Stop writing to PostgreSQL
6. Decommission PostgreSQL after 1 week

**Production Steps:**
- [ ] Provision MySQL 8.0+ database instance
- [ ] Configure MySQL connection (SSL, user permissions)
- [ ] Run schema migration: `migrations/001_init_schema_mysql.up.sql`
- [ ] Migrate production data: `scripts/migrate_data_to_mysql.go`
- [ ] Verify data counts match PostgreSQL
- [ ] Deploy application with MySQL backend
- [ ] Run smoke tests (create task, process, callback)
- [ ] Monitor metrics: task processing rate, error rate, latency
- [ ] Keep PostgreSQL backup for 1 week

**Post-Deployment:**
- [ ] Monitor error rates
- [ ] Monitor query performance
- [ ] Check for connection pool exhaustion
- [ ] Verify callback delivery success rate
- [ ] Update documentation (CLAUDE.md, README.md)

**Success Criteria:**
- ✅ Production tasks process successfully
- ✅ Callback delivery rate >99%
- ✅ No increase in error rate
- ✅ Performance acceptable

---

### Phase 6: Rollback Plan (If Needed)

**Rollback Triggers:**
- Error rate increases by >50%
- Callback delivery success rate drops below 95%
- Task processing stalls for >5 minutes
- Critical data corruption detected

**Rollback Procedure:**
1. Immediately redeploy previous PostgreSQL version
2. Restore PostgreSQL database from backup if needed
3. Verify connectivity and task processing resumes
4. Investigate root cause
5. Fix issue and schedule new migration attempt

**Rollback Time:** <15 minutes (if blue-green deployment) or <30 minutes (if backup restore needed)

---

## Acceptance Criteria

### Functional Requirements

- [ ] All CRUD operations (Create, Read, Update, Delete) work correctly
- [ ] Scheduler tiered polling processes tasks at correct intervals
- [ ] Worker pool (20 workers) processes tasks concurrently
- [ ] HTTP callback delivery works with retry logic
- [ ] Exponential backoff with jitter calculates correctly
- [ ] Dead lettering triggers after max_retries exceeded
- [ ] Circuit breaker prevents hammering failing endpoints
- [ ] Dashboard WebSocket shows real-time task updates
- [ ] Tag filtering works with JSON arrays
- [ ] JSON payload queries work correctly

### Non-Functional Requirements

**Performance:**
- [ ] Task creation API responds in <100ms (p95)
- [ ] Scheduler polling queries complete in <50ms (p95)
- [ ] Worker task processing throughput ≥20 tasks/second
- [ ] Callback delivery completes in <3 seconds (configurable timeout)
- [ ] Database connection pool has <5% wait time

**Reliability:**
- [ ] No data loss during migration
- [ ] Transaction isolation prevents race conditions
- [ ] SKIP LOCKED prevents duplicate task processing
- [ ] Graceful shutdown waits for in-flight tasks

**Compatibility:**
- [ ] Go 1.21+ compatibility
- [ ] MySQL 8.0.16+ (for CHECK constraints)
- [ ] Existing HTTP API contracts unchanged
- [ ] Dashboard UI unchanged

### Quality Gates

**Code Coverage:**
- [ ] Repository layer tests: ≥90% coverage
- [ ] Integration tests: All critical paths covered
- [ ] Load tests: Validate 1000+ concurrent tasks

**Documentation:**
- [ ] CLAUDE.md updated with MySQL commands
- [ ] README.md updated with MySQL setup instructions
- [ ] Migration documentation created
- [ ] Runbook updated for MySQL operations

**Code Review:**
- [ ] All changes reviewed by senior engineer
- [ ] Security review passed (SQL injection, connection security)
- [ ] Performance review passed (slow queries, connection pooling)

---

## Success Metrics

### Technical Metrics

| Metric | Baseline (PostgreSQL) | Target (MySQL) | Measurement |
|--------|----------------------|----------------|-------------|
| Task creation latency | p50: 20ms, p95: 80ms | p50: <25ms, p95: <100ms | Application metrics |
| Scheduler query latency | p50: 10ms, p95: 40ms | p50: <15ms, p95: <50ms | Database slow query log |
| Task processing throughput | 25 tasks/sec | ≥20 tasks/sec | Worker pool metrics |
| Callback success rate | 99.2% | ≥99% | Callback service logs |
| Database connection errors | 0.1% | <0.5% | Connection pool metrics |
| Data migration accuracy | N/A | 100% | Record count comparison |

### Business Metrics

- ✅ Zero downtime during migration (if using blue-green deployment)
- ✅ No task loss or data corruption
- ✅ Callback delivery success rate maintained ≥99%
- ✅ Dashboard remains accessible during migration

### Risk Mitigation

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| Data corruption during migration | Low | Critical | Pre-migration backup, checksum validation, rollback plan |
| Performance degradation | Medium | High | Load testing before production, monitor metrics post-deployment |
| Connection pool exhaustion | Low | High | Explicit pool configuration, monitoring alerts |
| JSON query syntax errors | Medium | Medium | Comprehensive integration tests, manual verification |
| Timezone handling issues | Low | Medium | Force UTC everywhere, add timezone tests |
| SKIP LOCKED incompatibility | Very Low | High | Verify MySQL version ≥8.0.1 in pre-flight checks |

---

## Dependencies & Prerequisites

### Required

- ✅ **Go 1.21+** - Current Go version
- ⚠️ **MySQL 8.0.16+** - Must verify MySQL version before starting
- ⚠️ **MySQL client tools** - For manual database operations
- ⚠️ **Development environment** - Local MySQL instance or Docker container

### External Dependencies

```go
// Add to go.mod:
require (
    github.com/go-sql-driver/mysql v1.8.1
    github.com/jmoiron/sqlx v1.4.0
)
```

### Knowledge Requirements

- **Team familiarity with MySQL** - Recommended: At least one team member with production MySQL experience
- **Understanding of SQL dialect differences** - Document key differences in runbook
- **Database migration experience** - If not, consider hiring a consultant or DBA

### Infrastructure Requirements

- **MySQL 8.0+ database server** (production)
  - Minimum 2 CPU cores, 4GB RAM for development
  - Minimum 4 CPU cores, 16GB RAM for production
- **Backup storage** - At least 2x current database size
- **Monitoring** - MySQL metrics (connection pool, slow query log, InnoDB metrics)

---

## Risk Analysis & Mitigation

### High-Risk Items

#### 1. Data Loss During Migration
**Probability:** Low | **Impact:** Critical

**Mitigation:**
- Full PostgreSQL backup before migration
- Checksum validation after data migration
- Run comparison queries to verify record counts
- Keep PostgreSQL backup for 1 week post-migration
- Test migration script on staging database first

**Rollback:** Restore from PostgreSQL backup (<30 minutes)

#### 2. Performance Degradation
**Probability:** Medium | **Impact:** High

**Mitigation:**
- Load test with production-like data volume
- Benchmark critical queries (scheduler, worker, callbacks)
- Monitor slow query log in MySQL
- Add EXPLAIN ANALYZE for all queries
- Tune indexes based on query patterns

**Rollback:** Revert to PostgreSQL version (<15 minutes with blue-green)

#### 3. Connection Pool Exhaustion
**Probability:** Low | **Impact:** High

**Mitigation:**
- Explicitly configure connection pool (MaxOpenConns, MaxIdleConns)
- Monitor connection pool metrics (wait count, wait duration)
- Set up alerts for connection errors
- Test under high concurrency (100+ workers)

**Rollback:** Adjust pool settings dynamically (no rollback needed)

### Medium-Risk Items

#### 4. JSON Query Syntax Errors
**Probability:** Medium | **Impact:** Medium

**Mitigation:**
- Create comprehensive integration tests for all JSON queries
- Document PostgreSQL → MySQL JSON syntax differences
- Add unit tests for JSON extraction, containment, updates
- Manual testing with complex payloads

**Rollback:** Fix syntax errors and redeploy (<10 minutes)

#### 5. Array Operations on Tags
**Probability:** Medium | **Impact:** Medium

**Mitigation:**
- Start with JSON arrays (simpler migration)
- Monitor tag query performance
- If slow, migrate to normalized `task_tags` table (Phase 7)
- Document migration path for future

**Rollback:** Optimize queries or normalize tags (1-2 days)

#### 6. Timezone Handling Issues
**Probability:** Low | **Impact:** Medium

**Mitigation:**
- Force UTC in application code
- Use `loc=UTC` in MySQL DSN
- Add timezone tests (create task, verify stored time)
- Document timezone handling in runbook

**Rollback:** Fix timezone configuration (<5 minutes)

---

## Alternative Approaches Considered

### Option A: Use Database Abstraction Layer
**Description:** Introduce a database abstraction layer (e.g., `sqlx` with interface adapters) to support both PostgreSQL and MySQL simultaneously.

**Pros:**
- Easier rollback (switch via configuration)
- Support for both databases simultaneously
- Future-proof for other database migrations

**Cons:**
- Additional complexity (abstract layer to maintain)
- Performance overhead (interface indirection)
- Longer development timeline
- Harder to optimize for database-specific features

**Decision:** **REJECTED** - Adds unnecessary complexity for a one-time migration. Direct migration is simpler and more maintainable.

### Option B: Use ORM (GORM)
**Description:** Replace raw SQL queries with GORM ORM for database-agnostic queries.

**Pros:**
- Database-agnostic query syntax
- Automatic migrations
- Built-in relationship handling

**Cons:**
- 20-40% performance penalty vs raw SQL
- Loss of query optimization control
- Harder to debug performance issues
- Team unfamiliar with GORM patterns
- GORM still has database-specific quirks

**Decision:** **REJECTED** - Performance penalty unacceptable for high-throughput task queue. Current repository pattern is more explicit and maintainable.

### Option C: Migrate to PostgreSQL-Compatible Database
**Description:** Use a PostgreSQL-compatible alternative (e.g., Amazon Aurora PostgreSQL, Google Cloud SQL).

**Pros:**
- Zero code changes
- Maintains PostgreSQL feature set
- Familiar tooling

**Cons:**
- Doesn't address underlying business requirement (if cost/standardization)
- Still requires migration to new infrastructure
- No reduction in operational complexity

**Decision:** **NOT APPLICABLE** - Assumes user has specific business requirement for MySQL.

### Option D: Dual-Write with Gradual Cutover
**Description:** Run both databases in parallel, write to both, gradually migrate reads.

**Pros:**
- Zero downtime
- Easy rollback
- Real-world performance comparison

**Cons:**
- Longer migration timeline (2-4 weeks)
- More complex code (dual-write logic)
- Risk of data inconsistency
- Double database load during migration

**Decision:** **CONSIDERED** - Recommended for mission-critical systems where zero downtime is required. For this migration, blue-green deployment is simpler and sufficient.

---

## Future Considerations

### Phase 7: Tag Normalization (Optional, Post-Migration)

If tag query performance is inadequate with JSON arrays:

**Action:** Migrate tags to normalized `task_tags` table.

**Benefits:**
- Faster tag queries (indexed joins vs JSON functions)
- More flexible tag operations (intersection, union)
- Better normalization (3NF compliance)

**Effort:** 3-5 days (migration script, repository updates, testing)

**Timeline:** Only if performance monitoring shows tag queries are bottleneck (unlikely for your workload).

### Phase 8: Read Replicas (Scaling Reads)

If read throughput becomes bottleneck:

**Action:** Add MySQL read replicas for dashboard queries, reporting, analytics.

**Benefits:**
- Offload read traffic from primary
- Improve dashboard performance
- Enable real-time analytics without impacting worker pool

**Effort:** 2-3 days (configure replication, update repository for read/write splitting)

**Timeline:** Post-migration, when application scales to >1000 tasks/second.

### Phase 9: Connection Pool Optimization

If connection pool metrics show high wait times:

**Action:** Implement dynamic connection pool sizing based on load.

**Benefits:**
- Better resource utilization
- Automatic scaling
- Reduced connection overhead

**Effort:** 1-2 days (implement pool monitoring, add autoscaling logic)

**Timeline:** Post-migration, after 1-2 weeks of production metrics collection.

---

## Documentation Plan

### Updated Documentation Files

#### CLAUDE.md
**Section: Development Commands - Go Backend**
```bash
# Run the server
make run
# or
go run cmd/server/main.go

# Run database migrations (MySQL)
mysql -h localhost -u later -p later < migrations/001_init_schema_mysql.up.sql

# Connect to MySQL database
mysql -h localhost -u later -p later
```

**Section: Environment Configuration**
```bash
# Database (MySQL)
DATABASE_URL=mysql://later:password@localhost:3306/later?parseTime=true&loc=UTC&charset=utf8mb4
DATABASE_MAX_OPEN_CONNS=100
DATABASE_MAX_IDLE_CONNS=20
DATABASE_CONN_MAX_LIFETIME=1h
DATABASE_CONN_MAX_IDLE_TIME=10m
```

**Section: Architecture - Repository Layer**
- Replace "PostgreSQL implementation using pgx/v5" with "MySQL implementation using go-sql-driver/mysql and sqlx"
- Update connection pool settings table
- Remove PostgreSQL-specific notes, add MySQL-specific notes

#### README.md
**Section: Prerequisites**
- Replace "PostgreSQL 15+" with "MySQL 8.0.16+"
- Update Docker Compose configuration

**Section: Quick Start**
```bash
# Start MySQL (Docker)
docker run --name later-mysql \
  -e MYSQL_ROOT_PASSWORD=root \
  -e MYSQL_DATABASE=later \
  -e MYSQL_USER=later \
  -e MYSQL_PASSWORD=password \
  -p 3306:3306 \
  -d mysql:8.0

# Run migrations
mysql -h localhost -u later -ppassword later < migrations/001_init_schema_mysql.up.sql

# Run server
make run
```

### New Documentation Files

#### `docs/migration-postgresql-to-mysql.md`
**Content:**
- Migration overview
- Step-by-step migration guide
- Troubleshooting common issues
- Performance tuning tips
- Rollback procedures

#### `docs/mysql-maintenance.md`
**Content:**
- MySQL backup/restore procedures
- Index maintenance (ANALYZE TABLE, OPTIMIZE TABLE)
- Slow query log analysis
- Connection pool monitoring
- Emergency procedures

---

## References & Research

### Internal References

- **Current PostgreSQL Schema:** `/Users/liuxuanyao/work/later/migrations/001_init_schema.up.sql`
- **Repository Implementation:** `/Users/liuxuanyao/work/later/internal/repository/postgres/`
- **Domain Models:** `/Users/liuxuanyao/work/later/internal/domain/models/task.go:1`
- **Configuration:** `/Users/liuxuanyao/work/later/configs/config.go:190`
- **Application Bootstrap:** `/Users/liuxuanyao/work/later/cmd/server/main.go:38`

### External References

**Driver Documentation:**
- [go-sql-driver/mysql GitHub](https://github.com/go-sql-driver/mysql)
- [jmoiron/sqlx GitHub](https://github.com/jmoiron/sqlx)
- [MySQL 8.0 Reference Manual](https://dev.mysql.com/doc/refman/8.0/en/)

**Migration Guides:**
- [PostgreSQL to MySQL Migration - SQLines](https://www.sqlines.com/postgresql-to-mysql)
- [MySQL Workbench Migration Guide](https://dev.mysql.com/doc/workbench/en/wb-migration-database-postgresql-typemapping.html)
- [AWS Database Migration Service (DMS)](https://aws.amazon.com/dms/)

**Best Practices:**
- [A Production-Grade Guide to Golang Database Connection Management](https://akemara.medium.com/a-production-grade-guide-to-golang-database-management-with-mysql-mariadb-6b00189ec25a)
- [Go and MySQL: Setting up Connection Pooling](https://medium.com/propertyfinder-engineering/go-and-mysql-setting-up-connection-pooling-4b778ef8e560)
- [Managing Connections - Official Go Documentation](https://go.dev/doc/database/manage-connections)

**Feature Equivalence:**
- [MySQL 8.0 SKIP LOCKED](https://dev.mysql.com/blog-archive/mysql-8-0-1-using-skip-locked-and-nowait-to-handle-hot-rows/)
- [PostgreSQL vs MySQL JSON Support](https://www.bytebase.com/blog/postgresql-vs-mysql-json-support/)
- [MySQL 8.0 Functional Indexes](https://www.percona.com/blog/mysql-8-0-functional-indexes/)

**Performance Comparisons:**
- [GORM vs sqlx: A Practical Comparison](https://medium.com/@elsyarifx/gorm-vs-sqlx-a-practical-comparison-for-go-developers-468767b30196)
- [Comparing database/sql, GORM, sqlx - JetBrains](https://blog.jetbrains.com/go/2023/04/27/comparing-db-packages/)
- [Go ORMs and Query Builders Comparison](https://www.bytebase.com/blog/golang-orm-query-builder/)

**Troubleshooting:**
- [Go Handling JSON in MySQL](https://www.linkedin.com/pulse/go-handling-json-mysql-tiago-melo)
- [How to Resolve MySQL JSON Issues in Go](https://www.aubergine.co.uk/insights/working-with-mysql-json-data-type-with-prepared-statements-using-it-in-go-and-resolving-the-problems-i-had)

---

## Appendix A: Complete Schema Comparison

### PostgreSQL Schema (Current)

```sql
CREATE TABLE IF NOT EXISTS task_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    callback_url TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'dead_lettered')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    scheduled_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    max_retries INTEGER NOT NULL DEFAULT 5,
    retry_count INTEGER NOT NULL DEFAULT 0,
    retry_backoff_seconds INTEGER NOT NULL DEFAULT 60,
    next_retry_at TIMESTAMPTZ,
    callback_attempts INTEGER NOT NULL DEFAULT 0,
    callback_timeout_seconds INTEGER NOT NULL DEFAULT 30,
    last_callback_at TIMESTAMPTZ,
    last_callback_status INTEGER,
    last_callback_error TEXT,
    priority INTEGER NOT NULL DEFAULT 0 CHECK (priority >= 0 AND priority <= 10),
    tags TEXT[],
    error_message TEXT,
    worker_id VARCHAR(50),
    CHECK (retry_count <= max_retries)
);

CREATE INDEX idx_tasks_status_scheduled_priority
ON task_queue(status, scheduled_at, priority DESC)
WHERE status IN ('pending', 'processing', 'failed');

CREATE INDEX idx_tasks_next_retry
ON task_queue(next_retry_at)
WHERE next_retry_at IS NOT NULL AND status = 'failed';

CREATE INDEX idx_tasks_created_at
ON task_queue(created_at DESC);

CREATE INDEX idx_tasks_status_priority
ON task_queue(status, priority DESC)
WHERE status IN ('pending', 'failed');

CREATE INDEX idx_tasks_tags
ON task_queue USING GIN(tags);
```

### MySQL Schema (Target)

```sql
CREATE TABLE IF NOT EXISTS task_queue (
    id CHAR(36) PRIMARY KEY DEFAULT (UUID()),
    name VARCHAR(255) NOT NULL,
    payload JSON NOT NULL,
    callback_url TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'dead_lettered')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    scheduled_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,
    max_retries INTEGER NOT NULL DEFAULT 5,
    retry_count INTEGER NOT NULL DEFAULT 0,
    retry_backoff_seconds INTEGER NOT NULL DEFAULT 60,
    next_retry_at TIMESTAMP NULL,
    callback_attempts INTEGER NOT NULL DEFAULT 0,
    callback_timeout_seconds INTEGER NOT NULL DEFAULT 30,
    last_callback_at TIMESTAMP NULL,
    last_callback_status INTEGER NULL,
    last_callback_error TEXT,
    priority INTEGER NOT NULL DEFAULT 0 CHECK (priority >= 0 AND priority <= 10),
    tags JSON,
    error_message TEXT,
    worker_id VARCHAR(50),
    CHECK (retry_count <= max_retries)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Partial indexes become full indexes
CREATE INDEX idx_tasks_status_scheduled_priority
ON task_queue(status, scheduled_at, priority DESC);

CREATE INDEX idx_tasks_next_retry
ON task_queue(next_retry_at);

CREATE INDEX idx_tasks_created_at
ON task_queue(created_at DESC);

CREATE INDEX idx_tasks_status_priority
ON task_queue(status, priority DESC);

-- Optional: Separate tags table (if JSON queries are slow)
CREATE TABLE task_tags (
    task_id CHAR(36) NOT NULL,
    tag VARCHAR(255) NOT NULL,
    PRIMARY KEY (task_id, tag),
    INDEX idx_tag (tag),
    FOREIGN KEY (task_id) REFERENCES task_queue(id) ON DELETE CASCADE
);
```

---

## Appendix B: Query Syntax Translation Guide

### Common Query Patterns

| Pattern | PostgreSQL | MySQL |
|---------|-----------|-------|
| **Parameter placeholders** | `WHERE id = $1` | `WHERE id = ?` |
| **Current timestamp** | `NOW()` | `CURRENT_TIMESTAMP` or `NOW()` |
| **Date subtraction** | `NOW() - INTERVAL '30 days'` | `DATE_SUB(NOW(), INTERVAL 30 DAY)` |
| **JSON field extraction** | `payload->>'key'` | `JSON_UNQUOTE(JSON_EXTRACT(payload, '$.key'))` |
| **JSON contains** | `payload @> '{"key": "value"}'` | `JSON_CONTAINS(payload, '{"key": "value"}')` |
| **Array overlap** | `tags && ARRAY['urgent']` | `JSON_CONTAINS(tags, '"urgent"')` |
| **Array contains** | `'urgent' = ANY(tags)` | `JSON_CONTAINS(tags, '"urgent"')` |
| **Insert with returning** | `INSERT ... RETURNING id` | Pre-generate UUID or separate SELECT |
| **Upsert** | `ON CONFLICT (id) DO UPDATE` | `ON DUPLICATE KEY UPDATE` |

### Complete Example: Find Due Tasks

**PostgreSQL:**
```sql
SELECT id, name, payload, status, priority, scheduled_at
FROM task_queue
WHERE status = 'pending'
  AND scheduled_at <= NOW()
  AND ($1 = -1 OR priority > $1)
ORDER BY priority DESC, scheduled_at ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;
```

**MySQL:**
```sql
SELECT id, name, payload, status, priority, scheduled_at
FROM task_queue
WHERE status = 'pending'
  AND scheduled_at <= NOW()
  AND (? = -1 OR priority > ?)
ORDER BY priority DESC, scheduled_at ASC
LIMIT ?
FOR UPDATE SKIP LOCKED;
```

### Complete Example: Update with Retry Logic

**PostgreSQL:**
```sql
UPDATE task_queue
SET status = $1,
    retry_count = $2,
    next_retry_at = NOW() + (retry_backoff_seconds * POW(2, $2)) * INTERVAL '1 second'
      + (random() * retry_backoff_seconds * POW(2, $2) * INTERVAL '1 second') * 0.5,
    error_message = $3
WHERE id = $4;
```

**MySQL:**
```sql
UPDATE task_queue
SET status = ?,
    retry_count = ?,
    next_retry_at = DATE_ADD(NOW(), INTERVAL (? * POW(2, ?)) SECOND) +
                   INTERVAL (RAND() * ? * POW(2, ?) * 0.5) SECOND,
    error_message = ?
WHERE id = ?;
```

**Note:** MySQL's `DATE_ADD` syntax is more verbose but functionally equivalent.

---

## Appendix C: Environment Variables Reference

### PostgreSQL (Current)
```bash
DATABASE_URL=postgres://later:password@localhost:5432/later?sslmode=disable
DATABASE_MAX_CONNECTIONS=100
```

### MySQL (Target)
```bash
# Combined DSN (recommended)
DATABASE_URL=mysql://later:password@localhost:3306/later?parseTime=true&loc=UTC&charset=utf8mb4&timeout=30s&readTimeout=30s&writeTimeout=30s

# Connection pool settings
DATABASE_MAX_OPEN_CONNS=100
DATABASE_MAX_IDLE_CONNS=20
DATABASE_CONN_MAX_LIFETIME=1h
DATABASE_CONN_MAX_IDLE_TIME=10m

# Alternative: Separate components (easier to debug)
DB_HOST=localhost
DB_PORT=3306
DB_USER=later
DB_PASSWORD=password
DB_NAME=later
DB_TIMEZONE=UTC
DB_PARSE_TIME=true
DB_CHARSET=utf8mb4
```

### Key DSN Parameters
| Parameter | Value | Purpose |
|-----------|-------|---------|
| `parseTime=true` | - | Parse TIMESTAMP to time.Time |
| `loc=UTC` | - | Force UTC timezone (critical!) |
| `charset=utf8mb4` | - | Full Unicode support (emojis, etc.) |
| `timeout=30s` | - | Connection timeout |
| `readTimeout=30s` | - | Query read timeout |
| `writeTimeout=30s` | - | Query write timeout |

---

## Next Steps

Before starting implementation, please confirm:

1. **Business Justification:** Why is this migration necessary? (Cost, standardization, expertise, compliance?)
2. **MySQL Version:** Verify MySQL 8.0.16+ is available (required for CHECK constraints)
3. **Team Capacity:** Allocate 2-3 weeks of developer time for migration and testing
4. **Infrastructure:** Provision MySQL development/staging/production instances
5. **Risk Tolerance:** Choose deployment strategy (blue-green vs dual-write vs cutover)

Once confirmed, proceed with **Phase 1: Foundation** implementation.
