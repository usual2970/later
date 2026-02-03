# Configuration Guide

The Later Task Service uses [Viper](https://github.com/spf13/viper) for flexible configuration management with YAML files and environment variable overrides.

## Configuration File Discovery

The service automatically searches for `config.yaml` in the following order (first found is used):

1. **Environment Variable**: `LATER_CONFIG_FILE` (highest priority)
2. **Current Directory**: `./configs/config.yaml` (relative to working directory)
3. **Executable Directory**: `<exe_dir>/configs/config.yaml`
4. **Project Root**: `<project_root>/configs/config.yaml` (detected by `go.mod`)

### Example: Running from Different Directories

```bash
# From project root (automatic)
cd /path/to/later
go run cmd/server/main.go

# From different directory using environment variable
cd /anywhere
LATER_CONFIG_FILE=/path/to/later/configs/config.yaml go run /path/to/later/cmd/server/main.go

# Using absolute path
LATER_CONFIG_FILE=/absolute/path/to/config.yaml ./server
```

## Quick Start

### 1. Using Default Configuration

```bash
# Run from project root
cd /path/to/later
go run cmd/server/main.go
```

### 2. Using Custom Config File

```bash
# Set custom config file location
export LATER_CONFIG_FILE=/path/to/custom-config.yaml
go run cmd/server/main.go
```

### 3. Using Environment Variables

```bash
# Override specific values
LATER_SERVER_PORT=9090 \
LATER_DATABASE_URL="postgres://user:pass@localhost:5432/mydb?sslmode=disable" \
go run cmd/server/main.go
```

### 4. Using .env File

```bash
# Copy example env file
cp .env.example .env

# Edit .env with your settings
vim .env

# Load .env and run
source .env && go run cmd/server/main.go
```

## Configuration File Structure

The `configs/config.yaml` file contains all configurable parameters:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

database:
  url: "postgres://later:later@localhost:5432/later?sslmode=disable"
  max_connections: 100

scheduler:
  high_priority_interval: 2s
  normal_priority_interval: 3s
  cleanup_interval: 30s

worker:
  pool_size: 20

callback:
  secret: "change-this-in-production"
  default_timeout: 30s
  default_max_retries: 5

log:
  level: "info"
  format: "json"
```

## Environment Variables

All configuration values can be overridden using environment variables with the `LATER_` prefix:

| Config Path | Environment Variable | Example |
|------------|---------------------|---------|
| `server.host` | `LATER_SERVER_HOST` | `LATER_SERVER_HOST=0.0.0.0` |
| `server.port` | `LATER_SERVER_PORT` | `LATER_SERVER_PORT=8080` |
| `database.url` | `LATER_DATABASE_URL` | `LATER_DATABASE_URL=postgres://...` |
| `database.max_connections` | `LATER_DATABASE_MAX_CONNECTIONS` | `LATER_DATABASE_MAX_CONNECTIONS=100` |
| `scheduler.high_priority_interval` | `LATER_SCHEDULER_HIGH_PRIORITY_INTERVAL` | `LATER_SCHEDULER_HIGH_PRIORITY_INTERVAL=2s` |
| `scheduler.normal_priority_interval` | `LATER_SCHEDULER_NORMAL_PRIORITY_INTERVAL` | `LATER_SCHEDULER_NORMAL_PRIORITY_INTERVAL=3s` |
| `scheduler.cleanup_interval` | `LATER_SCHEDULER_CLEANUP_INTERVAL` | `LATER_SCHEDULER_CLEANUP_INTERVAL=30s` |
| `worker.pool_size` | `LATER_WORKER_POOL_SIZE` | `LATER_WORKER_POOL_SIZE=20` |
| `callback.secret` | `LATER_CALLBACK_SECRET` | `LATER_CALLBACK_SECRET=your-secret` |
| `callback.default_timeout` | `LATER_CALLBACK_DEFAULT_TIMEOUT` | `LATER_CALLBACK_DEFAULT_TIMEOUT=30s` |
| `callback.default_max_retries` | `LATER_CALLBACK_DEFAULT_MAX_RETRIES` | `LATER_CALLBACK_DEFAULT_MAX_RETRIES=5` |
| `log.level` | `LATER_LOG_LEVEL` | `LATER_LOG_LEVEL=info` |
| `log.format` | `LATER_LOG_FORMAT` | `LATER_LOG_FORMAT=json` |

## Configuration Parameters

### Server

- **host**: Server bind address (default: `0.0.0.0`)
- **port**: Server port (default: `8080`)

### Database

- **url**: PostgreSQL connection string
- **max_connections**: Maximum database connections (default: `100`)

### Scheduler

- **high_priority_interval**: Polling interval for high-priority tasks (default: `2s`)
- **normal_priority_interval**: Polling interval for normal tasks (default: `3s`)
- **cleanup_interval**: Interval for cleanup operations (default: `30s`)

### Worker

- **pool_size**: Number of concurrent worker goroutines (default: `20`)

### Callback

- **secret**: HMAC secret for callback signature verification
- **default_timeout**: Default HTTP timeout for callbacks (default: `30s`)
- **default_max_retries**: Default maximum retry attempts (default: `5`)

### Logging

- **level**: Log level - `debug`, `info`, `warn`, `error` (default: `info`)
- **format**: Log format - `json` or `text` (default: `json`)

## Production Deployment

For production deployment:

1. **Generate a secure callback secret**:
   ```bash
   openssl rand -base64 32
   ```

2. **Use environment variables for sensitive data**:
   ```bash
   LATER_CALLBACK_SECRET=<generated-secret> \
   LATER_DATABASE_URL=<production-db-url> \
   LATER_CONFIG_FILE=/etc/later/config.yaml \
   ./server
   ```

3. **Adjust worker pool size** based on your load:
   ```bash
   LATER_WORKER_POOL_SIZE=50
   ```

4. **Configure appropriate database connection pool**:
   ```bash
   LATER_DATABASE_MAX_CONNECTIONS=200
   ```

## VS Code Debugging

The `.vscode/launch.json` is pre-configured to run the server from the project root:

```json
{
    "name": "Launch Server",
    "type": "go",
    "request": "launch",
    "mode": "auto",
    "program": "${workspaceFolder}/cmd/server",
    "cwd": "${workspaceFolder}",
    "env": {
        "LATER_CONFIG_FILE": "${workspaceFolder}/configs/config.yaml"
    }
}
```

**Press F5** in VS Code to start debugging.

## Docker Configuration

When using Docker, you can pass environment variables in `docker-compose.yaml`:

```yaml
version: '3.8'
services:
  server:
    image: later-task-service:latest
    environment:
      - LATER_CONFIG_FILE=/etc/later/config.yaml
      - LATER_SERVER_HOST=0.0.0.0
      - LATER_SERVER_PORT=8080
      - LATER_DATABASE_URL=postgres://user:pass@db:5432/later?sslmode=disable
      - LATER_CALLBACK_SECRET=${CALLBACK_SECRET}
      - LATER_WORKER_POOL_SIZE=20
    volumes:
      - ./configs:/etc/later
    ports:
      - "8080:8080"
```

## Duration Format

All duration values use Go's duration format:

- `ns` - nanoseconds
- `us` / `µs` - microseconds
- `ms` - milliseconds
- `s` - seconds
- `m` - minutes
- `h` - hours

Examples:
- `30s` - 30 seconds
- `5m` - 5 minutes
- `1h30m` - 1 hour and 30 minutes
- `500ms` - 500 milliseconds

## Troubleshooting

### Config File Not Found

**Error**: `config file not found (searched in: ./configs/config.yaml, ...)`

**Solutions**:
1. Run from project root directory
2. Set `LATER_CONFIG_FILE` environment variable with full path
3. Create `configs/config.yaml` in project root

Example:
```bash
# Solution 1: Run from project root
cd /path/to/later
go run cmd/server/main.go

# Solution 2: Use environment variable
LATER_CONFIG_FILE=/path/to/config.yaml go run cmd/server/main.go
```

### Invalid Duration Format

**Error**: `invalid scheduler.high_priority_interval: invalid duration`

**Solution**: Ensure duration values include units (e.g., `30s`, `5m`, not `30` or `5`)

### Environment Variables Not Working

Ensure:
1. Variables are prefixed with `LATER_`
2. Nested config uses underscore separator (e.g., `LATER_SERVER_PORT` for `server.port`)
3. Duration values include units (e.g., `30s` not `30`)

### Running from Different Directory

When running the executable from a different directory than the project root:

```bash
# Always set CONFIG_FILE explicitly
LATER_CONFIG_FILE=/path/to/later/configs/config.yaml /path/to/server

# Or use absolute paths in the config file itself
```

## Configuration Validation

The service validates all configuration values on startup:

- ✅ All duration values must be positive
- ✅ Port must be between 1-65535
- ✅ Worker pool size must be positive
- ✅ Database connections must be positive
- ✅ Max retries must be non-negative

Invalid configurations will cause the service to fail fast with a clear error message.
