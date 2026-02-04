# Logger Package

全局日志包，基于 `go.uber.org/zap`，支持环境自适应配置和自动日志轮转。

## 特性

- 🔧 **环境自适应**: 开发环境使用彩色控制台输出，生产环境使用 JSON 文件日志
- 📁 **自动日志轮转**: 集成 lumberjack，支持按大小、时间、备份数量自动切割
- 🚀 **高性能**: 零内存分配的 JSON 结构化日志
- 🎯 **类型安全**: 编译时字段类型检查
- 🌍 **全局单例**: 统一的日志接口，避免重复初始化

## 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `APP_ENV` | 运行环境 | `development` |
| `LOG_LEVEL` | 日志级别 | `debug` (dev/test), `info` (prod) |
| `LOG_FILE` | 日志文件路径 | `logs/app.log` (prod only) |

## 快速开始

### 1. 基本使用

```go
import (
    "later/internal/infrastructure/logger"
)

func main() {
    // 从环境变量初始化
    if err := logger.InitFromEnv(); err != nil {
        panic(err)
    }
    defer logger.Sync()

    // 记录日志
    logger.Info("Server started",
        logger.String("port", "8080"),
        logger.String("environment", "production"),
    )

    logger.Error("Failed to connect to database",
        logger.String("host", "localhost:5432"),
        logger.Error(err),
    )
}
```

### 2. 自定义配置

```go
import (
    "later/internal/infrastructure/logger"
    "go.uber.org/zap"
)

func main() {
    cfg := &logger.Config{
        Environment: "production",
        Level:       "info",
        Filename:    "/var/log/app.log",
        MaxSize:     500,    // 500 MB
        MaxBackups:  10,     // 保留 10 个备份
        MaxAge:      30,     // 保留 30 天
        Compress:    true,   // 压缩旧日志
    }

    if err := logger.Init(cfg); err != nil {
        panic(err)
    }
    defer logger.Sync()
}
```

### 3. 命名 Logger

```go
import (
    "later/internal/infrastructure/logger"
)

func NewUserService() *UserService {
    log := logger.Named("UserService")
    return &UserService{log: log}
}

func (s *UserService) CreateUser(id string) {
    s.log.Info("Creating user",
        logger.String("user_id", id),
    )
}
```

### 4. 添加结构化字段

```go
import (
    "later/internal/infrastructure/logger"
)

func ProcessRequest(req *Request) {
    // 创建带字段的 logger
    log := logger.With(
        logger.String("request_id", req.ID),
        logger.String("user_id", req.UserID),
    )

    log.Info("Processing request")
    log.Info("Request completed",
        logger.Int("duration_ms", 150),
    )
}
```

## 日志级别

```go
logger.Debug("Detailed debugging information")    // 开发环境
logger.Info("General informational message")      // 常规信息
logger.Warn("Warning message")                    // 警告
logger.Error("Error occurred")                    // 错误
logger.Fatal("Fatal error, exiting...")           // 致命错误，程序退出
```

## 输出示例

### 开发环境 (彩色控制台)

```
3:45PM INF database.go:45 > Database connected {"host": "localhost:5432", "port": 5432}
3:45PM DBG user_service.go:23 > Creating user {"user_id": "12345", "email": "user@example.com"}
3:45PM ERR auth.go:67 > Authentication failed {"attempt": 3, "reason": "invalid credentials"}
```

### 生产环境 (JSON 文件)

```json
{"level":"info","timestamp":"2026-02-03T15:45:00.123Z","caller":"database.go:45","msg":"Database connected","host":"localhost:5432","port":5432,"environment":"production","service":"later"}
{"level":"debug","timestamp":"2026-02-03T15:45:00.456Z","caller":"user_service.go:23","msg":"Creating user","user_id":"12345","email":"user@example.com","environment":"production","service":"later"}
{"level":"error","timestamp":"2026-02-03T15:45:01.789Z","caller":"auth.go:67","msg":"Authentication failed","attempt":3,"reason":"invalid credentials","stacktrace":"...","environment":"production","service":"later"}
```

## 日志轮转

生产环境自动执行日志轮转：

- **MaxSize**: 单个日志文件最大 500 MB
- **MaxBackups**: 保留最多 10 个历史文件
- **MaxAge**: 保留最近 30 天的日志
- **Compress**: 自动 gzip 压缩旧日志

日志文件命名示例：
```
logs/app.log           # 当前日志
logs/app-2026-02-02.log.gz   # 压缩的历史日志
logs/app-2026-02-01.log.gz
```

## 常用字段类型

```go
logger.String("key", "value")              // 字符串
logger.Int("key", 123)                     // 整数
logger.Int64("key", int64(123))            // 64位整数
logger.Float64("key", 123.45)              // 浮点数
logger.Bool("key", true)                   // 布尔值
logger.Duration("key", duration)           // 时间间隔
logger.Time("key", time.Now())             // 时间
logger.Err(err)                            // 错误对象（简写）
logger.Error(err)                          // 错误对象
logger.Any("key", anyValue)                // 任意类型
```

## 最佳实践

### ✅ 推荐

```go
// 使用结构化字段
logger.Info("User login",
    logger.String("user_id", userID),
    logger.String("ip", clientIP),
)

// 使用命名 logger
log := logger.Named("PaymentService")

// 创建带固定字段的 logger
log := logger.With(
    logger.String("request_id", reqID),
    logger.String("user_id", userID),
)
```

### ❌ 避免

```go
// 不要使用字符串拼接
logger.Info(fmt.Sprintf("User %s logged in from %s", userID, clientIP))

// 不要记录敏感信息
logger.Info("User login",
    logger.String("password", password),  // 危险！
)

// 不要在生产环境使用 Debug 级别记录敏感信息
logger.Debug("Request body", logger.String("body", string(body)))
```

## 性能建议

1. **使用预分配字段**: 避免在热路径中重复创建字段

```go
// 好的做法
var logField = logger.String("service", "api")
func handler() {
    logger.Info("Processing", logField)
}

// 避免在循环中创建字符串
for i := 0; i < 1000; i++ {
    logger.Info("Item", logger.String("index", strconv.Itoa(i)))
}
```

2. **条件日志**: 对于昂贵的日志操作

```go
if logger.Get().Core().Enabled(zapcore.DebugLevel) {
    logger.Debug("Expensive data", logger.String("data", expensiveOperation()))
}
```

## 故障排查

### 日志未输出

确保调用了 `defer logger.Sync()` 以刷新缓冲区。

### 日志级别不生效

检查 `LOG_LEVEL` 环境变量是否正确设置。

### 日志文件未创建

确保：
1. `APP_ENV=production`
2. 日志目录存在且可写
3. `LOG_FILE` 路径正确
