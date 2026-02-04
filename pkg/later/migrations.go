package later

import (
	"context"
	"fmt"
)

// RunMigrations explicitly runs database migrations
// This can be called manually if AutoMigration is disabled
func (l *Later) RunMigrations(ctx context.Context) error {
	return l.runMigrations()
}

// Close closes the database connection if Later owns it
// This is called automatically by Shutdown, but can be called explicitly if needed
func (l *Later) Close() error {
	if l.closeDB && l.db != nil {
		return l.db.Close()
	}
	return fmt.Errorf("database is not owned by Later (shared mode)")
}
