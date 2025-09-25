package search

import (
	"errors"
	"fmt"
	"strings"
)

// Search engine error types
var (
	ErrInvalidPattern    = errors.New("invalid search pattern")
	ErrInvalidRegex      = errors.New("invalid regular expression")
	ErrFileNotFound      = errors.New("file not found")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrFileTooBig        = errors.New("file too large to process")
	ErrBinaryFile        = errors.New("binary file skipped")
	ErrUnsupportedFile   = errors.New("unsupported file type")
	ErrSearchTimeout     = errors.New("search operation timed out")
	ErrIndexCorrupt      = errors.New("search index is corrupted")
	ErrInsufficientMemory = errors.New("insufficient memory for operation")
	ErrConcurrencyLimit   = errors.New("concurrency limit exceeded")
)

// SearchError provides detailed error information
type SearchError struct {
	Type      string `json:"type"`
	Message   string `json:"message"`
	FilePath  string `json:"file_path,omitempty"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	Operation string `json:"operation,omitempty"`
	Cause     error  `json:"-"`
}

func (e *SearchError) Error() string {
	var parts []string

	if e.Type != "" {
		parts = append(parts, fmt.Sprintf("[%s]", e.Type))
	}

	parts = append(parts, e.Message)

	if e.FilePath != "" {
		if e.Line > 0 {
			if e.Column > 0 {
				parts = append(parts, fmt.Sprintf("at %s:%d:%d", e.FilePath, e.Line, e.Column))
			} else {
				parts = append(parts, fmt.Sprintf("at %s:%d", e.FilePath, e.Line))
			}
		} else {
			parts = append(parts, fmt.Sprintf("in %s", e.FilePath))
		}
	}

	if e.Operation != "" {
		parts = append(parts, fmt.Sprintf("during %s", e.Operation))
	}

	result := strings.Join(parts, " ")

	if e.Cause != nil {
		result += fmt.Sprintf(": %v", e.Cause)
	}

	return result
}

func (e *SearchError) Unwrap() error {
	return e.Cause
}

// NewSearchError creates a new search error
func NewSearchError(errorType, message string) *SearchError {
	return &SearchError{
		Type:    errorType,
		Message: message,
	}
}

// NewFileError creates a file-specific error
func NewFileError(errorType, message, filePath string, cause error) *SearchError {
	return &SearchError{
		Type:     errorType,
		Message:  message,
		FilePath: filePath,
		Cause:    cause,
	}
}

// NewParseError creates a parsing error with location information
func NewParseError(message, filePath string, line, column int, cause error) *SearchError {
	return &SearchError{
		Type:      "parse_error",
		Message:   message,
		FilePath:  filePath,
		Line:      line,
		Column:    column,
		Operation: "parsing",
		Cause:     cause,
	}
}

// Error classification functions
func IsPatternError(err error) bool {
	return errors.Is(err, ErrInvalidPattern) || errors.Is(err, ErrInvalidRegex)
}

func IsFileError(err error) bool {
	return errors.Is(err, ErrFileNotFound) ||
		   errors.Is(err, ErrPermissionDenied) ||
		   errors.Is(err, ErrFileTooBig) ||
		   errors.Is(err, ErrBinaryFile) ||
		   errors.Is(err, ErrUnsupportedFile)
}

func IsSystemError(err error) bool {
	return errors.Is(err, ErrSearchTimeout) ||
		   errors.Is(err, ErrInsufficientMemory) ||
		   errors.Is(err, ErrConcurrencyLimit)
}

func IsRecoverableError(err error) bool {
	// File-level errors are recoverable (skip the file and continue)
	if IsFileError(err) {
		return true
	}

	// Parse errors are recoverable
	if searchErr, ok := err.(*SearchError); ok {
		return searchErr.Type == "parse_error"
	}

	return false
}

// Error handling utilities
type ErrorHandler struct {
	errors []error
	maxErrors int
	skipBinaryFiles bool
	skipLargeFiles bool
	continueOnError bool
}

func NewErrorHandler(maxErrors int, options ...func(*ErrorHandler)) *ErrorHandler {
	eh := &ErrorHandler{
		errors: make([]error, 0, maxErrors),
		maxErrors: maxErrors,
		skipBinaryFiles: true,
		skipLargeFiles: true,
		continueOnError: true,
	}

	for _, option := range options {
		option(eh)
	}

	return eh
}

func WithSkipBinaryFiles(skip bool) func(*ErrorHandler) {
	return func(eh *ErrorHandler) {
		eh.skipBinaryFiles = skip
	}
}

func WithSkipLargeFiles(skip bool) func(*ErrorHandler) {
	return func(eh *ErrorHandler) {
		eh.skipLargeFiles = skip
	}
}

func WithContinueOnError(continue_ bool) func(*ErrorHandler) {
	return func(eh *ErrorHandler) {
		eh.continueOnError = continue_
	}
}

func (eh *ErrorHandler) HandleError(err error) bool {
	if err == nil {
		return true
	}

	// Check if we should skip this error
	if eh.shouldSkipError(err) {
		return true
	}

	// Record the error
	if len(eh.errors) < eh.maxErrors {
		eh.errors = append(eh.errors, err)
	}

	// Decide whether to continue
	if !eh.continueOnError {
		return false
	}

	// Continue for recoverable errors
	return IsRecoverableError(err)
}

func (eh *ErrorHandler) shouldSkipError(err error) bool {
	if eh.skipBinaryFiles && errors.Is(err, ErrBinaryFile) {
		return true
	}

	if eh.skipLargeFiles && errors.Is(err, ErrFileTooBig) {
		return true
	}

	return false
}

func (eh *ErrorHandler) GetErrors() []error {
	return eh.errors
}

func (eh *ErrorHandler) HasErrors() bool {
	return len(eh.errors) > 0
}

func (eh *ErrorHandler) ErrorCount() int {
	return len(eh.errors)
}

// Wrapping functions for common error scenarios
func WrapFileError(err error, filePath string) error {
	if err == nil {
		return nil
	}

	// Classify the error
	message := err.Error()
	errorType := "file_error"

	if strings.Contains(message, "permission denied") {
		errorType = "permission_error"
	} else if strings.Contains(message, "no such file") {
		errorType = "not_found"
	} else if strings.Contains(message, "file too large") {
		errorType = "size_error"
	}

	return NewFileError(errorType, message, filePath, err)
}

func WrapRegexError(err error, pattern string) error {
	if err == nil {
		return nil
	}

	return &SearchError{
		Type:      "regex_error",
		Message:   fmt.Sprintf("invalid regular expression: %s", pattern),
		Operation: "pattern_compilation",
		Cause:     err,
	}
}

func WrapParserError(err error, filePath string, operation string) error {
	if err == nil {
		return nil
	}

	return &SearchError{
		Type:      "parser_error",
		Message:   "failed to parse file",
		FilePath:  filePath,
		Operation: operation,
		Cause:     err,
	}
}

// Recovery utilities
type RecoveryAction int

const (
	RecoverySkipFile RecoveryAction = iota
	RecoverySkipDirectory
	RecoveryRetry
	RecoveryAbort
)

func DetermineRecoveryAction(err error) RecoveryAction {
	if err == nil {
		return RecoveryRetry
	}

	// Binary files and large files - skip the file
	if errors.Is(err, ErrBinaryFile) || errors.Is(err, ErrFileTooBig) {
		return RecoverySkipFile
	}

	// Permission denied - skip directory if it's a directory error
	if errors.Is(err, ErrPermissionDenied) {
		return RecoverySkipDirectory
	}

	// File not found - skip file
	if errors.Is(err, ErrFileNotFound) {
		return RecoverySkipFile
	}

	// Parse errors - skip file
	if searchErr, ok := err.(*SearchError); ok && searchErr.Type == "parse_error" {
		return RecoverySkipFile
	}

	// System errors - abort
	if IsSystemError(err) {
		return RecoveryAbort
	}

	// Default - skip file
	return RecoverySkipFile
}

// Error reporting utilities
func FormatErrorSummary(errors []error) string {
	if len(errors) == 0 {
		return "No errors occurred"
	}

	errorCounts := make(map[string]int)
	for _, err := range errors {
		if searchErr, ok := err.(*SearchError); ok {
			errorCounts[searchErr.Type]++
		} else {
			errorCounts["unknown"]++
		}
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Encountered %d error(s):", len(errors)))

	for errorType, count := range errorCounts {
		parts = append(parts, fmt.Sprintf("  %s: %d", errorType, count))
	}

	return strings.Join(parts, "\n")
}

func FormatErrorDetails(errors []error, maxDetails int) string {
	if len(errors) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "Error details:")

	limit := len(errors)
	if maxDetails > 0 && maxDetails < limit {
		limit = maxDetails
	}

	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("  %d. %v", i+1, errors[i]))
	}

	if len(errors) > limit {
		parts = append(parts, fmt.Sprintf("  ... and %d more error(s)", len(errors)-limit))
	}

	return strings.Join(parts, "\n")
}