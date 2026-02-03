package mysql

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
	"later/configs"
)

// parseDSN ensures the DSN is in correct MySQL format
// Supports both formats for flexibility:
// - mysql://user:pass@host:port/db?params (PostgreSQL-style)
// - user:pass@tcp(host:port)/db?params (MySQL standard)
func parseDSN(databaseURL string) string {
	// If URL starts with mysql://, convert to MySQL DSN format
	if strings.HasPrefix(databaseURL, "mysql://") {
		u, err := url.Parse(databaseURL)
		if err != nil {
			// If parsing fails, strip prefix and hope for the best
			return strings.TrimPrefix(databaseURL, "mysql://")
		}

		// Rebuild in MySQL DSN format: user:pass@tcp(host:port)/dbname?params
		var mysqlDSN strings.Builder

		if u.User != nil {
			mysqlDSN.WriteString(u.User.String())
			mysqlDSN.WriteString("@")
		}

		host := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "3306"
		}
		mysqlDSN.WriteString(fmt.Sprintf("tcp(%s:%s)", host, port))

		if u.Path != "" && u.Path != "/" {
			mysqlDSN.WriteString(u.Path)
		}

		// Add query parameters
		params := u.Query()
		// Enable multi-statements for migrations
		params.Set("multiStatements", "true")

		if len(params) > 0 {
			mysqlDSN.WriteString("?")
			mysqlDSN.WriteString(params.Encode())
		}

		return mysqlDSN.String()
	}

	// Already in MySQL format, just add multiStatements support
	if !strings.Contains(databaseURL, "multiStatements") {
		if strings.Contains(databaseURL, "?") {
			databaseURL += "&multiStatements=true"
		} else {
			databaseURL += "?multiStatements=true"
		}
	}

	return databaseURL
}

// NewConnection creates a new MySQL connection pool
func NewConnection(cfg *configs.DatabaseConfig) (*sqlx.DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Ensure DSN is in correct format
	dsn := parseDSN(cfg.URL)

	// Connect to MySQL
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	// Configure connection pool (MySQL-specific settings)
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	log.Println("MySQL connection pool initialized successfully")
	return db, nil
}

// Close closes the database connection pool
func Close(db *sqlx.DB) error {
	err := db.Close()
	if err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}
	log.Println("MySQL connection pool closed")
	return nil
}

// RunMigrations executes SQL migration files from a directory
func RunMigrations(db *sqlx.DB, migrationsDir string) error {
	// For MVP, we'll execute the migration directly
	// In production, use a migration tool like golang-migrate or goose

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Read and execute the migration file
	migrationSQL, err := os.ReadFile(migrationsDir + "/001_init_schema_mysql.up.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Execute migration, ignoring duplicate index/table errors
	_, err = db.ExecContext(ctx, string(migrationSQL))
	if err != nil {
		// Check if error is about duplicate table/index (safe to ignore)
		errMsg := err.Error()
		if strings.Contains(errMsg, "Duplicate key name") ||
		   strings.Contains(errMsg, "Error 1050") || // Table already exists
		   strings.Contains(errMsg, "Error 1061") { // Duplicate index
			log.Println("MySQL migrations: some objects already exist, continuing...")
		} else {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
	}

	log.Println("MySQL migrations completed successfully")
	return nil
}
