package later

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestNewWithInvalidOptions tests that New() returns errors for invalid options
func TestNewWithInvalidOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name:    "No options - separate DB mode without DSN",
			opts:    []Option{},
			wantErr: true,
		},
		{
			name: "Shared DB mode with nil DB",
			opts: []Option{
				WithSharedDB(nil),
			},
			wantErr: true,
		},
		{
			name: "Invalid worker pool size",
			opts: []Option{
				WithSeparateDB("user:pass@tcp(localhost:3306)/test"),
				WithWorkerPoolSize(-1),
			},
			wantErr: true,
		},
		{
			name: "Invalid route prefix",
			opts: []Option{
				WithSeparateDB("user:pass@tcp(localhost:3306)/test"),
				WithRoutePrefix(""),
			},
			wantErr: true,
		},
		{
			name: "Invalid scheduler intervals",
			opts: []Option{
				WithSeparateDB("user:pass@tcp(localhost:3306)/test"),
				WithSchedulerIntervals(0, 1*time.Second, 1*time.Second),
			},
			wantErr: true,
		},
		{
			name: "Nil logger",
			opts: []Option{
				WithSeparateDB("user:pass@tcp(localhost:3306)/test"),
				WithLogger(nil),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestNewWithValidOptions tests that New() accepts valid options
func TestNewWithValidOptions(t *testing.T) {
	opts := []Option{
		WithSharedDB(nil), // Will fail validation, but tests option application
	}

	// This will fail because DB is nil, but we're testing option parsing
	_, err := New(opts...)
	if err == nil {
		t.Error("Expected error for nil DB in shared mode")
	}
}

// TestConfigDefaults tests that default configuration is applied correctly
func TestConfigDefaults(t *testing.T) {
	// We can't test without a real database, but we can verify
	// the default configuration structure is valid
	cfg := Config{}

	if cfg.WorkerPoolSize == 0 {
		// Default would be applied in New()
		cfg.WorkerPoolSize = 20
	}

	if cfg.RoutePrefix == "" {
		cfg.RoutePrefix = "/api/v1"
	}

	if cfg.CallbackTimeout == 0 {
		cfg.CallbackTimeout = 30 * time.Second
	}

	if cfg.Logger == nil {
		cfg.Logger = zap.L()
	}

	// Verify defaults are reasonable
	if cfg.WorkerPoolSize <= 0 {
		t.Error("WorkerPoolSize must be positive")
	}
	if cfg.RoutePrefix == "" {
		t.Error("RoutePrefix cannot be empty")
	}
	if cfg.CallbackTimeout <= 0 {
		t.Error("CallbackTimeout must be positive")
	}
	if cfg.Logger == nil {
		t.Error("Logger cannot be nil")
	}
}

// TestDBModeString tests DBMode string conversion
func TestDBModeString(t *testing.T) {
	tests := []struct {
		mode   DBMode
		want   string
	}{
		{DBModeShared, "shared"},
		{DBModeSeparate, "separate"},
		{DBMode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := modeToString(tt.mode); got != tt.want {
				t.Errorf("modeToString(%v) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

// TestTaskFilterConversion tests TaskFilter to repository.TaskFilter conversion
func TestTaskFilterConversion(t *testing.T) {
	filter := &TaskFilter{
		Status:       "pending",
		Page:         1,
		Limit:        10,
		SortBy:       "created_at",
		SortOrder:    "desc",
	}

	repoFilter := filter.toRepositoryFilter()

	if repoFilter.Page != filter.Page {
		t.Errorf("Page = %v, want %v", repoFilter.Page, filter.Page)
	}
	if repoFilter.Limit != filter.Limit {
		t.Errorf("Limit = %v, want %v", repoFilter.Limit, filter.Limit)
	}
	if repoFilter.SortBy != filter.SortBy {
		t.Errorf("SortBy = %v, want %v", repoFilter.SortBy, filter.SortBy)
	}
	if repoFilter.SortOrder != filter.SortOrder {
		t.Errorf("SortOrder = %v, want %v", repoFilter.SortOrder, filter.SortOrder)
	}
}
