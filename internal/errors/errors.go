package errors

import (
	"fmt"
)

// Kind represents the high-level semantic category of an application error.
type Kind uint8

const (
	KindInternal Kind = iota
	KindNotFound
	KindInvalidInput
	KindUnauthorized
	KindForbidden
	KindConflict
	KindUpstreamFailed
	KindServiceUnavailable
)

// String returns a human-readable representation of the error Kind.
func (k Kind) String() string {
	switch k {
	case KindInternal:
		return "Internal"
	case KindNotFound:
		return "NotFound"
	case KindInvalidInput:
		return "InvalidInput"
	case KindUnauthorized:
		return "Unauthorized"
	case KindForbidden:
		return "Forbidden"
	case KindConflict:
		return "Conflict"
	case KindUpstreamFailed:
		return "UpstreamFailed"
	case KindServiceUnavailable:
		return "ServiceUnavailable"
	default:
		return "Unknown"
	}
}

// AppError is the core domain error type for Tsuruyomi, encapsulating semantic kind,
// machine-readable error code, human-readable message, and optional root cause error.
type AppError struct {
	Kind    Kind   `json:"kind"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

// Error formats the domain error into a descriptive string.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause of this error, conforming to standard library errors.Unwrap.
func (e *AppError) Unwrap() error {
	return e.Err
}

// Is implements semantic matching for errors.Is.
// An AppError matches target if target is an *AppError with the same Kind,
// and either target has an empty Code or matching Code.
func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Kind == t.Kind && (t.Code == "" || e.Code == t.Code)
}

// New creates a new AppError with the given kind, code, message, and root cause.
func New(kind Kind, code, msg string, err error) *AppError {
	return &AppError{
		Kind:    kind,
		Code:    code,
		Message: msg,
		Err:     err,
	}
}

// Internal constructs a KindInternal AppError.
func Internal(code, msg string, err error) *AppError {
	return &AppError{Kind: KindInternal, Code: code, Message: msg, Err: err}
}

// NotFound constructs a KindNotFound AppError.
func NotFound(code, msg string, err error) *AppError {
	return &AppError{Kind: KindNotFound, Code: code, Message: msg, Err: err}
}

// InvalidInput constructs a KindInvalidInput AppError.
func InvalidInput(code, msg string, err error) *AppError {
	return &AppError{Kind: KindInvalidInput, Code: code, Message: msg, Err: err}
}

// Unauthorized constructs a KindUnauthorized AppError.
func Unauthorized(code, msg string, err error) *AppError {
	return &AppError{Kind: KindUnauthorized, Code: code, Message: msg, Err: err}
}

// Forbidden constructs a KindForbidden AppError.
func Forbidden(code, msg string, err error) *AppError {
	return &AppError{Kind: KindForbidden, Code: code, Message: msg, Err: err}
}

// Conflict constructs a KindConflict AppError.
func Conflict(code, msg string, err error) *AppError {
	return &AppError{Kind: KindConflict, Code: code, Message: msg, Err: err}
}

// UpstreamFailed constructs a KindUpstreamFailed AppError.
func UpstreamFailed(code, msg string, err error) *AppError {
	return &AppError{Kind: KindUpstreamFailed, Code: code, Message: msg, Err: err}
}

// ServiceUnavailable constructs a KindServiceUnavailable AppError.
func ServiceUnavailable(code, msg string, err error) *AppError {
	return &AppError{Kind: KindServiceUnavailable, Code: code, Message: msg, Err: err}
}
