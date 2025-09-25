package output

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestFormatterFactory tests the formatter factory functionality
func TestFormatterFactory(t *testing.T) {
	tests := []struct {
		name           string
		config         FormatterConfig
		expectedType   string
	}{
		{
			name:         "Text formatter",
			config:       FormatterConfig{Format: FormatText},
			expectedType: "*output.TextFormatter",
		},
		{
			name:         "JSON formatter",
			config:       FormatterConfig{Format: FormatJSON},
			expectedType: "*output.JSONFormatter",
		},
		{
			name:         "Count formatter",
			config:       FormatterConfig{Format: FormatCount},
			expectedType: "*output.CountFormatter",
		},
		{
			name:         "Files formatter",
			config:       FormatterConfig{Format: FormatFiles},
			expectedType: "*output.FilesFormatter",
		},
		{
			name:         "Default to text formatter",
			config:       FormatterConfig{Format: "unknown"},
			expectedType: "*output.TextFormatter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			factory := NewFormatterFactory(&buf, tt.config)
			formatter := factory.CreateFormatter()

			// Use type assertion to check the actual type
			actualType := getFormatterType(formatter)
			if actualType != tt.expectedType {
				t.Errorf("Expected formatter type %s, got %s", tt.expectedType, actualType)
			}
		})
	}
}

// getFormatterType returns the type name of the formatter
func getFormatterType(f Formatter) string {
	switch f.(type) {
	case *TextFormatter:
		return "*output.TextFormatter"
	case *JSONFormatter:
		return "*output.JSONFormatter"
	case *CountFormatter:
		return "*output.CountFormatter"
	case *FilesFormatter:
		return "*output.FilesFormatter"
	default:
		return "unknown"
	}
}

// TestOutputManager tests the output manager functionality
func TestOutputManager(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{
		Format:          FormatText,
		ShowLineNumbers: true,
		ShowFilenames:   true,
		ShowColors:      false,
	}
	formatter := NewTextFormatter(&buf, config)
	ctx := context.Background()
	manager := NewOutputManager(ctx, formatter)

	// Test processing a match
	match := Match{
		Path:           "/test/file.go",
		LineNumber:     10,
		Line:           "func TestFunction() {",
		AbsoluteOffset: 100,
		Submatches: []Submatch{
			{Text: "Test", Start: 5, End: 9},
		},
	}

	err := manager.ProcessMatch(match)
	if err != nil {
		t.Fatalf("Error processing match: %v", err)
	}

	err = manager.Close()
	if err != nil {
		t.Fatalf("Error closing manager: %v", err)
	}

	output := buf.String()
	expected := "/test/file.go:10:func TestFunction() {\n"
	if output != expected {
		t.Errorf("Expected output %q, got %q", expected, output)
	}
}

// TestOutputManagerWithContext tests context cancellation
func TestOutputManagerWithContext(t *testing.T) {
	var buf bytes.Buffer
	config := FormatterConfig{Format: FormatText}
	formatter := NewTextFormatter(&buf, config)

	ctx, cancel := context.WithCancel(context.Background())
	manager := NewOutputManager(ctx, formatter)

	// Cancel the context
	cancel()

	// Try to process a match - should return context error
	match := Match{
		Path:       "/test/file.go",
		LineNumber: 1,
		Line:       "test line",
	}

	err := manager.ProcessMatch(match)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

// TestDurationFormatting tests duration formatting compatibility
func TestDurationFormatting(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{"microseconds", 354 * time.Microsecond},
		{"milliseconds", 15 * time.Millisecond},
		{"seconds", 2 * time.Second},
		{"minutes", 2*time.Minute + 30*time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDuration(tt.duration)

			// Check that all fields are populated
			if d.Secs < 0 {
				t.Error("Secs should not be negative")
			}
			if d.Nanos < 0 {
				t.Error("Nanos should not be negative")
			}
			if d.Human == "" {
				t.Error("Human readable string should not be empty")
			}

			// Check that nanos is within valid range
			if d.Nanos >= 1e9 {
				t.Errorf("Nanos should be less than 1e9, got %d", d.Nanos)
			}

			// Verify reconstruction
			reconstructed := time.Duration(d.Secs)*time.Second + time.Duration(d.Nanos)*time.Nanosecond
			if reconstructed != tt.duration {
				t.Errorf("Duration reconstruction failed: expected %v, got %v", tt.duration, reconstructed)
			}
		})
	}
}

// TestSemanticInfo tests semantic information structure
func TestSemanticInfo(t *testing.T) {
	semantic := &SemanticInfo{
		SymbolType: "function",
		SymbolName: "TestFunction",
		Scope:      "main",
		Definition: &Location{
			Path:         "/src/main.go",
			LineNumber:   10,
			ColumnNumber: 5,
		},
		References: []Location{
			{Path: "/src/test.go", LineNumber: 20, ColumnNumber: 8},
			{Path: "/src/util.go", LineNumber: 30, ColumnNumber: 12},
		},
	}

	// Test that semantic info can be serialized to JSON
	jsonData, err := json.Marshal(semantic)
	if err != nil {
		t.Fatalf("Error marshaling semantic info: %v", err)
	}

	// Test that it can be unmarshaled back
	var unmarshaled SemanticInfo
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Error unmarshaling semantic info: %v", err)
	}

	// Verify fields
	if unmarshaled.SymbolType != semantic.SymbolType {
		t.Errorf("SymbolType mismatch: expected %s, got %s", semantic.SymbolType, unmarshaled.SymbolType)
	}
	if len(unmarshaled.References) != len(semantic.References) {
		t.Errorf("References count mismatch: expected %d, got %d", len(semantic.References), len(unmarshaled.References))
	}
}

// TestMatchStructure tests the match structure
func TestMatchStructure(t *testing.T) {
	match := Match{
		Path:           "/test/file.go",
		Language:       "go",
		LineNumber:     42,
		ColumnNumber:   10,
		AbsoluteOffset: 1024,
		Line:           "func TestFunction() {",
		Submatches: []Submatch{
			{Text: "Test", Start: 5, End: 9},
			{Text: "Function", Start: 9, End: 17},
		},
		BeforeContext: []ContextLine{
			{LineNumber: 41, Text: "// This is a test"},
		},
		AfterContext: []ContextLine{
			{LineNumber: 43, Text: "    return nil"},
		},
		Semantic: &SemanticInfo{
			SymbolType: "function",
			SymbolName: "TestFunction",
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(match)
	if err != nil {
		t.Fatalf("Error marshaling match: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Match
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Error unmarshaling match: %v", err)
	}

	// Verify key fields
	if unmarshaled.Path != match.Path {
		t.Errorf("Path mismatch: expected %s, got %s", match.Path, unmarshaled.Path)
	}
	if len(unmarshaled.Submatches) != len(match.Submatches) {
		t.Errorf("Submatches count mismatch: expected %d, got %d", len(match.Submatches), len(unmarshaled.Submatches))
	}
	if len(unmarshaled.BeforeContext) != len(match.BeforeContext) {
		t.Errorf("BeforeContext count mismatch: expected %d, got %d", len(match.BeforeContext), len(unmarshaled.BeforeContext))
	}
}

// BenchmarkTextFormatter benchmarks text formatting performance
func BenchmarkTextFormatter(b *testing.B) {
	var buf bytes.Buffer
	config := FormatterConfig{
		Format:          FormatText,
		ShowLineNumbers: true,
		ShowFilenames:   true,
		ShowColors:      true,
	}
	formatter := NewTextFormatter(&buf, config)

	match := Match{
		Path:           "/test/very/long/path/to/some/file.go",
		LineNumber:     1000,
		Line:           "func VeryLongFunctionNameForTesting() error { return nil }",
		AbsoluteOffset: 50000,
		Submatches: []Submatch{
			{Text: "VeryLongFunctionNameForTesting", Start: 5, End: 35},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		formatter.FormatMatch(match)
	}
}

// BenchmarkJSONFormatter benchmarks JSON formatting performance
func BenchmarkJSONFormatter(b *testing.B) {
	var buf bytes.Buffer
	config := FormatterConfig{Format: FormatJSON}
	formatter := NewJSONFormatter(&buf, config)

	match := Match{
		Path:           "/test/very/long/path/to/some/file.go",
		LineNumber:     1000,
		Line:           "func VeryLongFunctionNameForTesting() error { return nil }",
		AbsoluteOffset: 50000,
		Submatches: []Submatch{
			{Text: "VeryLongFunctionNameForTesting", Start: 5, End: 35},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		formatter.FormatMatch(match)
	}
}