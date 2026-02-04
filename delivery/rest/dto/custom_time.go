package dto

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CustomTime wraps time.Time to support multiple datetime formats
type CustomTime struct {
	time.Time
}

// UnmarshalJSON parses JSON string into CustomTime with support for multiple formats
func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}

	// Remove quotes
	s := strings.Trim(string(b), "\"")
	if s == "" {
		return nil
	}

	// Try different time formats in order of preference
	// For formats without timezone, explicitly parse as UTC
	var lastErr error

	// First try RFC3339 formats (include timezone info)
	formatsWithTZ := []string{
		time.RFC3339,                  // "2006-01-02T15:04:05Z07:00"
		time.RFC3339Nano,              // "2006-01-02T15:04:05.999999999Z07:00"
		"2006-01-02T15:04:05Z07",      // "2006-01-02T15:04:05+08" (without : in timezone)
	}

	for _, format := range formatsWithTZ {
		t, err := time.Parse(format, s)
		if err == nil {
			ct.Time = t.UTC() // Convert to UTC
			return nil
		}
		lastErr = err
	}

	// Try formats without timezone, parse in UTC location
	formatsNoTZ := []string{
		"2006-01-02T15:04:05",         // "2006-01-02T15:04:05" (no timezone, assume UTC)
		"2006-01-02T15:04",            // "2006-01-02T15:04" (user's format)
		"2006-01-02 15:04:05",         // "2006-01-02 15:04:05"
		"2006-01-02 15:04",            // "2006-01-02 15:04"
		"2006-01-02",                  // "2006-01-02" (date only, assume midnight UTC)
	}

	utcLoc := time.FixedZone("UTC", 0)
	for _, format := range formatsNoTZ {
		t, err := time.ParseInLocation(format, s, utcLoc)
		if err == nil {
			ct.Time = t.UTC() // Ensure it's stored as UTC
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("cannot parse time \"%s\". Expected format: YYYY-MM-DDTHH:MM:SSZ (e.g., 2026-02-03T16:20:00+08:00). Last error: %v", s, lastErr)
}

// MarshalJSON converts CustomTime to JSON string (always UTC)
func (ct *CustomTime) MarshalJSON() ([]byte, error) {
	if ct.Time.IsZero() {
		return []byte("null"), nil
	}
	// Always format as UTC with Z suffix
	utcTime := ct.Time.UTC()
	return json.Marshal(utcTime.Format(time.RFC3339))
}

// ToTime returns the underlying time.Time
func (ct *CustomTime) ToTime() *time.Time {
	if ct.Time.IsZero() {
		return nil
	}
	return &ct.Time
}
