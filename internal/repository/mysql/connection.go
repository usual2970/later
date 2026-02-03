package mysql

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
	"later/configs"
)

// NewConnection creates a new MySQL connection pool
func NewConnection(cfg *configs.DatabaseConfig) (*sqlx.DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to MySQL
	db, err := sqlx.Connect("mysql", cfg.URL)
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

	_, err = db.ExecContext(ctx, string(migrationSQL))
	if err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	log.Println("MySQL migrations completed successfully")
	return nil
}
