package utils

import "fmt"

// NewError creates a new error with a message
func NewError(msg string) error {
	return fmt.Errorf("%s", msg)
}

// WrapError wraps an error with additional context
func WrapError(err error, msg string) error {
	if err == nil {
		return fmt.Errorf("%s", msg)
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// TimeoutError creates a timeout error
func TimeoutError(operation string) error {
	return fmt.Errorf("%s: operation timed out", operation)
}
