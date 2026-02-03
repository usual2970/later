# 日志迁移指南

## 从标准库 `log` 迁移到结构化日志

### 基本迁移对照表

| 标准库 log | 结构化日志 logger | 说明 |
|------------|-------------------|------|
| `log.Printf("User %s logged in", id)` | `logger.Info("User logged in", zap.String("user_id", id))` | 使用结构化字段 |
| `log.Print("Server started")` | `logger.Info("Server started")` | 简单日志 |
| `log.Fatalf("Failed: %v", err)` | `logger.Fatal("Failed", zap.Error(err))` | 致命错误 |
| `log.Println("Processing...")` | `logger.Debug("Processing...")` | 调试信息 |

### 迁移步骤

#### 1. 替换导入

```go
// 旧代码
import "log"

// 新代码
import (
    "later/internal/infrastructure/logger"
    "go.uber.org/zap"
)
```

#### 2. 初始化 logger

```go
// 在 main 函数开始处
func main() {
    // 初始化 logger
    if err := logger.InitFromEnv(); err != nil {
        panic(err)
    }
    defer logger.Sync()

    // ... 其他代码
}
```

#### 3. 替换日志调用

```go
// ❌ 旧代码
log.Printf("Server started on %s", address)
log.Printf("Failed to connect: %v", err)
log.Println("Worker pool started")

// ✅ 新代码
logger.Info("Server started",
    zap.String("address", address),
)
logger.Error("Failed to connect", zap.Error(err))
logger.Info("Worker pool started")
```

### 实际迁移示例

#### 示例 1: HTTP 请求日志

```go
// ❌ 旧代码
log.Printf("[%s] %s %s status=%d latency=%s",
    time.Now().Format(time.RFC3339),
    req.Method,
    req.URL.Path,
    statusCode,
    latency,
)

// ✅ 新代码
logger.Info("Request processed",
    zap.String("method", req.Method),
    zap.String("path", req.URL.Path),
    zap.Int("status", statusCode),
    zap.Duration("latency", latency),
)
```

#### 示例 2: 数据库操作

```go
// ❌ 旧代码
log.Printf("DB query executed in %v: %s", duration, query)

// ✅ 新代码
logger.Debug("DB query executed",
    zap.Duration("duration", duration),
    zap.String("query", query),
)
```

#### 示例 3: 错误处理

```go
// ❌ 旧代码
if err := db.Connect(); err != nil {
    log.Fatalf("Failed to connect to database: %v", err)
}

// ✅ 新代码
if err := db.Connect(); err != nil {
    logger.Fatal("Failed to connect to database",
        zap.String("host", db.Host),
        zap.Int("port", db.Port),
        zap.Error(err),
    )
}
```

### 为服务创建命名 Logger

```go
// 在服务构造函数中
func NewUserService(db *Database) *UserService {
    return &UserService{
        db:  db,
        log: logger.Named("UserService"), // 创建命名 logger
    }
}

// 在方法中使用
func (s *UserService) CreateUser(user *User) error {
    s.log.Info("Creating user",
        zap.String("user_id", user.ID),
        zap.String("email", user.Email),
    )

    // ... 业务逻辑

    s.log.Info("User created successfully",
        zap.String("user_id", user.ID),
    )
    return nil
}
```

### 使用带固定字段的 Logger

```go
// 为请求创建 logger
func HandleRequest(req *Request) {
    requestLog := logger.With(
        zap.String("request_id", req.ID),
        zap.String("user_id", req.UserID),
        zap.String("ip", req.ClientIP),
    )

    requestLog.Info("Processing request")
    // 所有日志自动包含 request_id, user_id, ip

    // 执行操作
    if err := process(req); err != nil {
        requestLog.Error("Processing failed", zap.Error(err))
        return
    }

    requestLog.Info("Processing completed",
        zap.Int("duration_ms", calculateDuration()),
    )
}
```

### 环境配置

```bash
# 开发环境 - 彩色控制台输出
export APP_ENV=development
export LOG_LEVEL=debug

# 生产环境 - JSON 文件日志
export APP_ENV=production
export LOG_LEVEL=info
export LOG_FILE=/var/log/app/app.log
```

### 验证迁移

1. **编译检查**
```bash
go build ./...
```

2. **运行测试**
```bash
go test -v ./...
```

3. **查看日志输出**

开发环境（彩色）:
```
2026-02-03T15:55:59.297+0800 INFO main server/main.go:100 Server started {"address": "localhost:8080"}
```

生产环境（JSON 文件）:
```json
{"level":"info","timestamp":"2026-02-03T15:56:15.716+0800","caller":"server/main.go:100","msg":"Server started","address":"localhost:8080","environment":"production","service":"later"}
```

### 常见问题

#### Q: 我有很多 log.Printf 调用，如何快速迁移？

A: 使用正则表达式搜索替换：

1. 查找: `log\.Printf\("(.+)\\.%v", (.+)\)`
2. 替换为结构化字段

或使用 IDE 的重构工具（如 GoLand 的 "Replace Structurally"）。

#### Q: 如何处理格式化字符串中的多个变量？

A: 将每个变量转换为独立的字段：

```go
// ❌ 旧代码
log.Printf("User %s from %s performed %s at %v", user, city, action, time)

// ✅ 新代码
logger.Info("User action performed",
    zap.String("user", user),
    zap.String("city", city),
    zap.String("action", action),
    zap.Time("timestamp", time),
)
```

#### Q: Debug 日志在生产环境会输出吗？

A: 不会。生产环境默认使用 Info 级别，Debug 日志会被自动过滤。

#### Q: 如何临时调整某个服务的日志级别？

A: 为该服务创建命名 logger 并单独配置：

```go
serviceLog := logger.Named("VerboseService")
// 可以通过配置文件单独控制此 logger 的级别
```

### 迁移检查清单

- [ ] 替换所有 `import "log"` 为 `logger`
- [ ] 在 `main()` 中初始化 `logger.InitFromEnv()`
- [ ] 添加 `defer logger.Sync()`
- [ ] 替换所有 `log.Printf` 为结构化日志
- [ ] 替换所有 `log.Fatalf` 为 `logger.Fatal`
- [ ] 为主要服务创建命名 logger
- [ ] 设置环境变量（APP_ENV, LOG_LEVEL）
- [ ] 测试开发环境日志输出
- [ ] 测试生产环境日志文件生成
- [ ] 验证日志轮转功能

### 需要帮助？

- 查看详细文档: `internal/infrastructure/logger/README.md`
- 查看实现总结: `docs/LOGGER_IMPLEMENTATION.md`
- 查看示例代码: `internal/infrastructure/logger/logger_test.go`
