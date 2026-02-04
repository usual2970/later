package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	globalLogger *zap.Logger
	once         sync.Once
)

// Config defines logger configuration
type Config struct {
	Environment string // "development", "testing", "production"
	Level       string // "debug", "info", "warn", "error"
	// File logging configuration (only used in production)
	Filename   string // Log file path
	MaxSize    int    // Maximum size in megabytes
	MaxBackups int    // Maximum number of old log files to retain
	MaxAge     int    // Maximum number of days to retain old log files
	Compress   bool   // Compress rotated files with gzip
}

// DefaultConfig returns default logger configuration based on environment
func DefaultConfig(env string) *Config {
	switch env {
	case "production", "prod":
		return &Config{
			Environment: "production",
			Level:       "info",
			Filename:    "logs/app.log",
			MaxSize:     500, // 500 MB
			MaxBackups:  10,
			MaxAge:      30, // 30 days
			Compress:    true,
		}
	case "testing", "test":
		return &Config{
			Environment: "testing",
			Level:       "debug",
		}
	default: // development
		return &Config{
			Environment: "development",
			Level:       "debug",
		}
	}
}

// Init initializes the global logger with the given configuration
// Must be called before using the logger
func Init(cfg *Config) error {
	var err error
	once.Do(func() {
		err = initLogger(cfg)
	})
	return err
}

// InitFromEnv initializes the global logger based on environment variable
// Uses APP_ENV or defaults to "development"
func InitFromEnv() error {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}
	cfg := DefaultConfig(env)

	// Override log level from ENV if specified
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.Level = logLevel
	}

	// Override log file path from ENV if specified
	if logFile := os.Getenv("LOG_FILE"); logFile != "" {
		cfg.Filename = logFile
	}

	return Init(cfg)
}

// initLogger creates and sets the global logger
func initLogger(cfg *Config) error {
	var logger *zap.Logger
	var err error

	level := parseLogLevel(cfg.Level)

	if cfg.Environment == "production" {
		// Production: JSON logging to file with rotation
		logger, err = newProductionLogger(cfg, level)
	} else {
		// Development/Testing: Console logging
		logger, err = newDevelopmentLogger(level)
	}

	if err != nil {
		return err
	}

	globalLogger = logger
	return nil
}

// newProductionLogger creates a production logger with file rotation
func newProductionLogger(cfg *Config, level zapcore.Level) (*zap.Logger, error) {
	// Configure log rotation with lumberjack
	writer := &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	// Create JSON encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// Create core
	core := zapcore.NewCore(encoder, zapcore.AddSync(writer), level)

	// Create logger with caller information
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1), // Skip wrapper function
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Fields(
			zap.String("environment", cfg.Environment),
			zap.String("service", "later"),
		),
	)

	return logger, nil
}

// newDevelopmentLogger creates a development logger with console output
func newDevelopmentLogger(level zapcore.Level) (*zap.Logger, error) {
	// Create development config
	config := zap.NewDevelopmentConfig()

	// Customize encoder config
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	// Set log level
	config.Level = zap.NewAtomicLevelAt(level)

	// Build logger
	logger, err := config.Build(
		zap.AddCallerSkip(1), // Skip wrapper function
	)
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// parseLogLevel converts string log level to zapcore.Level
func parseLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// Get returns the global logger instance
// Returns a no-op logger if not initialized
func Get() *zap.Logger {
	if globalLogger != nil {
		return globalLogger
	}
	// Return no-op logger if not initialized
	return zap.NewNop()
}

// Named returns a named logger from the global logger
func Named(name string) *zap.Logger {
	return Get().Named(name)
}

// With returns a logger with additional fields
func With(fields ...zap.Field) *zap.Logger {
	return Get().With(fields...)
}

// Sync flushes any buffered log entries
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// Convenience functions matching zap's interface

// Debug logs a message at debug level
func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

// Info logs a message at info level
func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

// Warn logs a message at warn level
func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

// Error logs a message at error level
func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

// Fatal logs a message at fatal level and exits
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}

// DPanic logs a message at panic level in development, and error level in production
func DPanic(msg string, fields ...zap.Field) {
	Get().DPanic(msg, fields...)
}

// Panic logs a message at panic level and panics
func Panic(msg string, fields ...zap.Field) {
	Get().Panic(msg, fields...)
}
