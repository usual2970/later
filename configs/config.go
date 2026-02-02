package configs

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Scheduler SchedulerConfig
	Worker    WorkerConfig
	Callback  CallbackConfig
	Log       LogConfig
}

type ServerConfig struct {
	Host string
	Port int
}

func (s ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type DatabaseConfig struct {
	URL            string
	MaxConnections int
}

type SchedulerConfig struct {
	HighPriorityInterval  time.Duration
	NormalPriorityInterval time.Duration
	CleanupInterval        time.Duration
}

type WorkerConfig struct {
	PoolSize int
}

type CallbackConfig struct {
	Secret           string
	DefaultTimeout   time.Duration
	DefaultMaxRetries int
}

type LogConfig struct {
	Level  string
	Format string // "json" or "text"
}

func LoadConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvInt("SERVER_PORT", 8080),
		},
		Database: DatabaseConfig{
			URL:            getEnv("DATABASE_URL", "postgres://later:later@localhost:5432/later?sslmode=disable"),
			MaxConnections: getEnvInt("DATABASE_MAX_CONNECTIONS", 100),
		},
		Scheduler: SchedulerConfig{
			HighPriorityInterval:   time.Duration(getEnvInt("SCHEDULER_HIGH_INTERVAL", 2)) * time.Second,
			NormalPriorityInterval: time.Duration(getEnvInt("SCHEDULER_NORMAL_INTERVAL", 3)) * time.Second,
			CleanupInterval:        time.Duration(getEnvInt("SCHEDULER_CLEANUP_INTERVAL", 30)) * time.Second,
		},
		Worker: WorkerConfig{
			PoolSize: getEnvInt("WORKER_POOL_SIZE", 20),
		},
		Callback: CallbackConfig{
			Secret:           getEnv("CALLBACK_SECRET", "change-this-in-production"),
			DefaultTimeout:   time.Duration(getEnvInt("CALLBACK_TIMEOUT", 30)) * time.Second,
			DefaultMaxRetries: getEnvInt("CALLBACK_MAX_RETRIES", 5),
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal := parseInt(value); intVal != 0 {
			return intVal
		}
	}
	return defaultValue
}

func parseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}
