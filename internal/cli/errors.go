package cli

import (
	"errors"
	"fmt"
)

// Exit codes form the stable process contract for cfgfc automation.
const (
	ExitSuccess     = 0
	ExitUsage       = 2
	ExitResource    = 3
	ExitInvalidData = 4
	ExitRefusal     = 5
	ExitPersistence = 6
)

// ErrorClass identifies one stable non-success exit-code class.
type ErrorClass string

// Error classes map command failures to the documented exit codes.
const (
	ErrorClassUsage       ErrorClass = "usage"
	ErrorClassResource    ErrorClass = "resource"
	ErrorClassInvalidData ErrorClass = "invalid_data"
	ErrorClassRefusal     ErrorClass = "refusal"
	ErrorClassPersistence ErrorClass = "persistence"
)

// CommandError is the typed failure returned by command handlers.
type CommandError struct {
	Class   ErrorClass
	Code    string
	Message string
	Details any
	cause   error
}

// Error returns the concise user-facing error message.
func (commandErr *CommandError) Error() string {
	return commandErr.Message
}

// Unwrap returns the underlying implementation error when one exists.
func (commandErr *CommandError) Unwrap() error {
	return commandErr.cause
}

// ExitCode maps the typed failure class to its documented process exit code.
func (commandErr *CommandError) ExitCode() int {
	switch commandErr.Class {
	case ErrorClassUsage:
		return ExitUsage
	case ErrorClassResource:
		return ExitResource
	case ErrorClassInvalidData:
		return ExitInvalidData
	case ErrorClassRefusal:
		return ExitRefusal
	case ErrorClassPersistence:
		return ExitPersistence
	default:
		return ExitPersistence
	}
}

// NewUsageError creates an invalid invocation error.
func NewUsageError(code string, message string, cause error) *CommandError {
	return newCommandError(ErrorClassUsage, code, message, nil, cause)
}

// NewResourceError creates a missing, ambiguous, or conflicting resource error.
func NewResourceError(code string, message string, details any, cause error) *CommandError {
	return newCommandError(ErrorClassResource, code, message, details, cause)
}

// NewInvalidDataError creates a malformed or unsupported resource-data error.
func NewInvalidDataError(code string, message string, details any, cause error) *CommandError {
	return newCommandError(ErrorClassInvalidData, code, message, details, cause)
}

// NewRefusalError creates a confirmation or target-ownership refusal error.
func NewRefusalError(code string, message string, details any, cause error) *CommandError {
	return newCommandError(ErrorClassRefusal, code, message, details, cause)
}

// NewPersistenceError creates a filesystem, persistence, or transaction error.
func NewPersistenceError(code string, message string, cause error) *CommandError {
	return newCommandError(ErrorClassPersistence, code, message, nil, cause)
}

// newCommandError constructs one normalized typed command failure.
func newCommandError(class ErrorClass, code string, message string, details any, cause error) *CommandError {
	if code == "" {
		code = string(class)
	}
	if message == "" && cause != nil {
		message = cause.Error()
	}
	if message == "" {
		message = fmt.Sprintf("%s error", class)
	}
	return &CommandError{Class: class, Code: code, Message: message, Details: details, cause: cause}
}

// AsCommandError preserves typed command failures and classifies Cobra parsing failures as usage errors.
func AsCommandError(err error) *CommandError {
	var commandErr *CommandError
	if errors.As(err, &commandErr) {
		return commandErr
	}
	return NewUsageError("invalid_usage", err.Error(), err)
}
