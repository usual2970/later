package entity

import (
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// JSONBytes handles JSON payload that can be either:
// - A JSON object (will be marshaled to bytes)
// - A base64-encoded string (will be decoded)
type JSONBytes []byte

// Scan implements sql.Scanner for JSONBytes
func (j *JSONBytes) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*j = v
	case string:
		*j = []byte(v)
	default:
		return fmt.Errorf("unsupported type for JSONBytes: %T", value)
	}

	return nil
}

// Value implements driver.Valuer for JSONBytes
func (j JSONBytes) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

// UnmarshalJSON supports both JSON objects and base64 strings
func (j *JSONBytes) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return errors.New("payload cannot be empty")
	}

	// Try to unmarshal as a JSON object first
	var obj interface{}
	if err := json.Unmarshal(b, &obj); err == nil {
		// It's a valid JSON object, marshal it to bytes
		bytes, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		*j = bytes
		return nil
	}

	// Try to unmarshal as a string (base64 encoded)
	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		// It's a string, try to base64 decode it
		decoded, err := base64.StdEncoding.DecodeString(str)
		if err != nil {
			return fmt.Errorf("invalid base64 string: %v", err)
		}

		// Verify it's valid JSON
		if !json.Valid(decoded) {
			return errors.New("decoded payload is not valid JSON")
		}

		*j = decoded
		return nil
	}

	return errors.New("payload must be a JSON object or a base64-encoded JSON string")
}

// MarshalJSON converts JSONBytes to JSON (as string)
func (j JSONBytes) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return json.Marshal(string(j))
}

// Valid checks if the JSON bytes are valid
func (j JSONBytes) Valid() bool {
	return json.Valid(j)
}

// String returns the string representation
func (j JSONBytes) String() string {
	return string(j)
}
