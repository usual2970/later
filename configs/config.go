package configs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
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
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

func (s ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type DatabaseConfig struct {
	URL              string        `mapstructure:"url"`
	MaxConnections   int           `mapstructure:"max_connections"`
	MaxOpenConns     int           `mapstructure:"max_open_conns"`
	MaxIdleConns     int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime  time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime  time.Duration `mapstructure:"conn_max_idle_time"`
	Timezone         string        `mapstructure:"timezone"`
}

type SchedulerConfig struct {
	HighPriorityInterval   time.Duration `mapstructure:"high_priority_interval"`
	NormalPriorityInterval time.Duration `mapstructure:"normal_priority_interval"`
	CleanupInterval        time.Duration `mapstructure:"cleanup_interval"`
}

type WorkerConfig struct {
	PoolSize int `mapstructure:"pool_size"`
}

type CallbackConfig struct {
	Secret           string        `mapstructure:"secret"`
	DefaultTimeout   time.Duration `mapstructure:"default_timeout"`
	DefaultMaxRetries int          `mapstructure:"default_max_retries"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // "json" or "text"
}

// LoadConfig loads configuration from config.yaml and environment variables
// Environment variables take precedence over config file values
//
// Config file search order (first found is used):
// 1. Path from LATER_CONFIG_FILE environment variable
// 2. ./configs/config.yaml (relative to working directory)
// 3. <executable_dir>/configs/config.yaml
// 4. <project_root>/configs/config.yaml (detected by go.mod)
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Determine config file path
	if configPath == "" {
		configPath = findConfigFile()
		if configPath == "" {
			return nil, fmt.Errorf("config file not found (searched in: ./configs/config.yaml, <exe>/configs/config.yaml, <project_root>/configs/config.yaml)")
		}
	}

	// Read from config file
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// If config file doesn't exist, use defaults
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found; will use defaults and environment variables
	}

	// Enable environment variable override
	v.SetEnvPrefix("LATER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set defaults BEFORE unmarshalling
	setDefaults(v)

	// Unmarshal config
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Parse duration strings and override config values
	if err := parseDurations(v, &config); err != nil {
		return nil, fmt.Errorf("failed to parse durations: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// findConfigFile searches for config.yaml in multiple locations
func findConfigFile() string {
	// Check environment variable first
	if envPath := os.Getenv("LATER_CONFIG_FILE"); envPath != "" {
		if fileExists(envPath) {
			return envPath
		}
	}

	// Candidate paths to search
	candidates := []string{
		"./configs/config.yaml",           // Relative to working directory
		"./config.yaml",                   // Current directory
	}

	// Add executable directory paths
	if exeDir, err := getExecutableDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(exeDir, "configs", "config.yaml"),
			filepath.Join(exeDir, "config.yaml"),
		)
	}

	// Add project root paths (detected by go.mod)
	if projectRoot, err := findProjectRoot(); err == nil {
		candidates = append(candidates,
			filepath.Join(projectRoot, "configs", "config.yaml"),
			filepath.Join(projectRoot, "config.yaml"),
		)
	}

	// Return first existing file
	for _, candidate := range candidates {
		absPath, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if fileExists(absPath) {
			return absPath
		}
	}

	return ""
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// getExecutableDir returns the directory containing the executable
func getExecutableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// findProjectRoot attempts to find the project root by looking for go.mod
func findProjectRoot() (string, error) {
	// Start from current directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Search up the directory tree
	for {
		// Check if go.mod exists in this directory
		if fileExists(filepath.Join(dir, "go.mod")) {
			return dir, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)

	// Database defaults (MySQL)
	v.SetDefault("database.url", "mysql://later:later@localhost:3306/later?parseTime=true&loc=UTC&charset=utf8mb4")
	v.SetDefault("database.max_connections", 100)
	v.SetDefault("database.max_open_conns", 100)
	v.SetDefault("database.max_idle_conns", 20)
	v.SetDefault("database.conn_max_lifetime", "1h")
	v.SetDefault("database.conn_max_idle_time", "10m")
	v.SetDefault("database.timezone", "UTC")

	// Scheduler defaults (as strings, will be parsed later)
	v.SetDefault("scheduler.high_priority_interval", "2s")
	v.SetDefault("scheduler.normal_priority_interval", "3s")
	v.SetDefault("scheduler.cleanup_interval", "30s")

	// Worker defaults
	v.SetDefault("worker.pool_size", 20)

	// Callback defaults
	v.SetDefault("callback.secret", "change-this-in-production")
	v.SetDefault("callback.default_timeout", "30s")
	v.SetDefault("callback.default_max_retries", 5)

	// Log defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
}

// parseDurations parses duration strings into time.Duration values
func parseDurations(v *viper.Viper, config *Config) error {
	// Parse scheduler intervals
	if highInterval := v.GetString("scheduler.high_priority_interval"); highInterval != "" {
		d, err := time.ParseDuration(highInterval)
		if err != nil {
			return fmt.Errorf("invalid scheduler.high_priority_interval: %w", err)
		}
		config.Scheduler.HighPriorityInterval = d
	}

	if normalInterval := v.GetString("scheduler.normal_priority_interval"); normalInterval != "" {
		d, err := time.ParseDuration(normalInterval)
		if err != nil {
			return fmt.Errorf("invalid scheduler.normal_priority_interval: %w", err)
		}
		config.Scheduler.NormalPriorityInterval = d
	}

	if cleanupInterval := v.GetString("scheduler.cleanup_interval"); cleanupInterval != "" {
		d, err := time.ParseDuration(cleanupInterval)
		if err != nil {
			return fmt.Errorf("invalid scheduler.cleanup_interval: %w", err)
		}
		config.Scheduler.CleanupInterval = d
	}

	// Parse callback timeout
	if timeout := v.GetString("callback.default_timeout"); timeout != "" {
		d, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("invalid callback.default_timeout: %w", err)
		}
		config.Callback.DefaultTimeout = d
	}

	return nil
}

// validateConfig validates the configuration values
func validateConfig(config *Config) error {
	// Validate scheduler intervals
	if config.Scheduler.HighPriorityInterval <= 0 {
		return fmt.Errorf("scheduler.high_priority_interval must be positive")
	}
	if config.Scheduler.NormalPriorityInterval <= 0 {
		return fmt.Errorf("scheduler.normal_priority_interval must be positive")
	}
	if config.Scheduler.CleanupInterval <= 0 {
		return fmt.Errorf("scheduler.cleanup_interval must be positive")
	}

	// Validate callback timeout
	if config.Callback.DefaultTimeout <= 0 {
		return fmt.Errorf("callback.default_timeout must be positive")
	}

	// Validate worker pool size
	if config.Worker.PoolSize <= 0 {
		return fmt.Errorf("worker.pool_size must be positive")
	}

	// Validate server port
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}

	// Validate database max connections
	if config.Database.MaxConnections <= 0 {
		return fmt.Errorf("database.max_connections must be positive")
	}

	// Validate callback max retries
	if config.Callback.DefaultMaxRetries < 0 {
		return fmt.Errorf("callback.default_max_retries must be non-negative")
	}

	return nil
}
