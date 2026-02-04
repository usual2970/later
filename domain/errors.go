package domain

import "errors"

var (
	// ErrInternalServerError is thrown when an internal server error occurs
	ErrInternalServerError = errors.New("internal server error")

	// ErrNotFound is thrown when a requested resource is not found
	ErrNotFound = errors.New("task not found")

	// ErrConflict is thrown when a resource already exists
	ErrConflict = errors.New("task already exists")

	// ErrBadParamInput is thrown when request parameters are invalid
	ErrBadParamInput = errors.New("invalid parameters")

	// ErrTaskCannotDelete is thrown when a task cannot be deleted
	ErrTaskCannotDelete = errors.New("task cannot be deleted in current state")

	// ErrTaskCannotRetry is thrown when a task cannot be retried
	ErrTaskCannotRetry = errors.New("task cannot be retried")
)
