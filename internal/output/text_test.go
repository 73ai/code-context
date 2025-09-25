package output

import (
	"bytes"
	"strings"
	"testing"
)

// TestTextFormatterBasic tests basic text formatting
func TestTextFormatterBasic(t *testing.T) {
	tests := []struct {
		name     string
		config   FormatterConfig
		match    Match
		expected string
	}{
		{
			name: "Simple match without line numbers or filenames",
			config: FormatterConfig{
				Format:          FormatText,
				ShowLineNumbers: false,
				ShowFilenames:   false,
				ShowColors:      false,
			},
			match: Match{
				Path:       "test.go",
				LineNumber: 1,
				Line:       "func test() {",
				Submatches: []Submatch{
					{Text: "test", Start: 5, End: 9},
				},
			},
			expected: "func test() {\n",
		},
		{
			name: "Match with line numbers",
			config: FormatterConfig{
				Format:          FormatText,
				ShowLineNumbers: true,
				ShowFilenames:   false,
				ShowColors:      false,
			},
			match: Match{
				Path:       "test.go",
				LineNumber: 42,
				Line:       "func test() {",
				Submatches: []Submatch{
					{Text: "test", Start: 5, End: 9},
				},
			},
			expected: "42:func test() {\n",
		},
		{
			name: "Match with filenames",
			config: FormatterConfig{
				Format:          FormatText,
				ShowLineNumbers: false,
				ShowFilenames:   true,
				ShowColors:      false,
			},
			match: Match{
				Path:       "/path/to/test.go",
				LineNumber: 1,
				Line:       "func test() {",
				Submatches: []Submatch{
					{Text: "test", Start: 5, End: 9},
				},
			},
			expected: "/path/to/test.go:func test() {\n",
		},
		{
			name: "Match with both line numbers and filenames",
			config: FormatterConfig{
				Format:          FormatText,
				ShowLineNumbers: true,
				ShowFilenames:   true,
				ShowColors:      false,
			},
			match: Match{
				Path:       "/path/to/test.go",
				LineNumber: 42,
				Line:       "func test() {",
				Submatches: []Submatch{
					{Text: "test", Start: 5, End: 9},
				},
			},
			expected: "/path/to/test.go:42:func test() {\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := NewTextFormatter(&buf, tt.config)

			err := formatter.FormatMatch(tt.match)
			if err != nil {
				t.Fatalf("Error formatting match: %v", err)
			}

			output := buf.String()
			if output != tt.expected {
				t.Errorf("Expected output:\n%q\nGot:\n%q", tt.expected, output)
			}
		})
	}
}

// TestTextFormatterColors tests color highlighting
func TestTextFormatterColors(t *testing.T) {
	config := FormatterConfig{
		Format:          FormatText,
		ShowLineNumbers: true,
		ShowFilenames:   true,
		ShowColors:      true,
	}

	match := Match{
		Path:       "test.go",
		LineNumber: 1,
		Line:       "func test() {",
		Submatches: []Submatch{
			{Text: "test", Start: 5, End: 9},
		},
	}

	var buf bytes.Buffer
	formatter := NewTextFormatter(&buf, config)

	err := formatter.FormatMatch(match)
	if err != nil {
		t.Fatalf("Error formatting match: %v", err)
	}

	output := buf.String()

	// Should contain color codes
	if !strings.Contains(output, "\033[") {
		t.Error("Expected output to contain ANSI color codes")
	}

	// Should contain the highlighted match
	if !strings.Contains(output, BrightRed+"test"+Reset) {
		t.Error("Expected output to contain highlighted match text")
	}

	// Should contain colored filename
	if !strings.Contains(output, Magenta+"test.go"+Reset) {
		t.Error("Expected output to contain colored filename")
	}

	// Should contain colored line number
	if !strings.Contains(output, Green+"1"+Reset) {
		t.Error("Expected output to contain colored line number")
	}
}

// TestTextFormatterContextLines tests context line formatting
func TestTextFormatterContextLines(t *testing.T) {
	config := FormatterConfig{
		Format:          FormatText,
		ShowLineNumbers: true,
		ShowFilenames:   true,
		ShowColors:      false,
	}

	match := Match{
		Path:       "test.go",
		LineNumber: 5,
		Line:       "func test() {",
		Submatches: []Submatch{
			{Text: "test", Start: 5, End: 9},
		},
		BeforeContext: []ContextLine{
			{LineNumber: 3, Text: "// Comment before"},
			{LineNumber: 4, Text: ""},
		},
		AfterContext: []ContextLine{
			{LineNumber: 6, Text: "    return nil"},
			{LineNumber: 7, Text: "}"},
		},
	}

	var buf bytes.Buffer
	formatter := NewTextFormatter(&buf, config)

	err := formatter.FormatMatch(match)
	if err != nil {
		t.Fatalf("Error formatting match: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Should have 5 lines total (2 before + 1 match + 2 after)
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines, got %d: %v", len(lines), lines)
	}

	// Check context line format (should use "-" separator)
	if !strings.Contains(lines[0], "test.go-3-// Comment before") {
		t.Errorf("Expected context line format, got: %q", lines[0])
	}

	// Check match line format (should use ":" separator)
	if !strings.Contains(lines[2], "test.go:5:func test() {") {
		t.Errorf("Expected match line format, got: %q", lines[2])
	}
}

// TestTextFormatterOnlyMatching tests only-matching mode
func TestTextFormatterOnlyMatching(t *testing.T) {
	config := FormatterConfig{
		Format:          FormatText,
		Mode:            ModeOnlyMatching,
		ShowLineNumbers: true,
		ShowFilenames:   true,
		ShowColors:      false,
	}

	match := Match{
		Path:       "test.go",
		LineNumber: 1,
		Line:       "func testFunction() { return testValue }",
		Submatches: []Submatch{
			{Text: "test", Start: 5, End: 9},
			{Text: "test", Start: 30, End: 34},
		},
	}

	var buf bytes.Buffer
	formatter := NewTextFormatter(&buf, config)

	err := formatter.FormatMatch(match)
	if err != nil {
		t.Fatalf("Error formatting match: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Should have 2 lines, one for each submatch
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %v", len(lines), lines)
	}

	// Each line should only contain the matched text
	for i, line := range lines {
		if !strings.Contains(line, "test.go:1:test") {
			t.Errorf("Line %d doesn't match expected format: %q (full output: %q)", i, line, output)
		}
	}
}

// TestTextFormatterMultipleMatches tests multiple matches in a single line
func TestTextFormatterMultipleMatches(t *testing.T) {
	config := FormatterConfig{
		Format:          FormatText,
		ShowLineNumbers: false,
		ShowFilenames:   false,
		ShowColors:      true,
	}

	match := Match{
		Path:       "test.go",
		LineNumber: 1,
		Line:       "test function test result",
		Submatches: []Submatch{
			{Text: "test", Start: 0, End: 4},
			{Text: "test", Start: 14, End: 18},
		},
	}

	var buf bytes.Buffer
	formatter := NewTextFormatter(&buf, config)

	err := formatter.FormatMatch(match)
	if err != nil {
		t.Fatalf("Error formatting match: %v", err)
	}

	output := buf.String()

	// Should highlight both occurrences of "test"
	expectedHighlights := 2
	actualHighlights := strings.Count(output, BrightRed+"test"+Reset)
	if actualHighlights != expectedHighlights {
		t.Errorf("Expected %d highlighted matches, got %d", expectedHighlights, actualHighlights)
	}

	// Should contain the complete line with highlights
	expected := BrightRed + "test" + Reset + " function " + BrightRed + "test" + Reset + " result\n"
	if output != expected {
		t.Errorf("Expected output:\n%q\nGot:\n%q", expected, output)
	}
}

// TestCountFormatter tests count formatting
func TestCountFormatter(t *testing.T) {
	tests := []struct {
		name     string
		config   FormatterConfig
		matches  []Match
		expected string
	}{
		{
			name: "Single file with matches",
			config: FormatterConfig{
				Format:        FormatCount,
				ShowFilenames: true,
				ShowColors:    false,
				TotalFiles:    1,
			},
			matches: []Match{
				{
					Path: "test.go",
					Submatches: []Submatch{
						{Text: "test", Start: 0, End: 4},
					},
				},
				{
					Path: "test.go",
					Submatches: []Submatch{
						{Text: "test", Start: 0, End: 4},
						{Text: "test", Start: 10, End: 14},
					},
				},
			},
			expected: "3\n", // No filename for single file
		},
		{
			name: "Multiple files with matches",
			config: FormatterConfig{
				Format:        FormatCount,
				ShowFilenames: true,
				ShowColors:    false,
				TotalFiles:    2,
			},
			matches: []Match{
				{
					Path: "test1.go",
					Submatches: []Submatch{
						{Text: "test", Start: 0, End: 4},
					},
				},
			},
			expected: "test1.go:1\n", // Filename shown for multiple files
		},
		{
			name: "Without filename",
			config: FormatterConfig{
				Format:        FormatCount,
				ShowFilenames: false,
				ShowColors:    false,
				TotalFiles:    1,
			},
			matches: []Match{
				{
					Path: "test.go",
					Submatches: []Submatch{
						{Text: "test", Start: 0, End: 4},
					},
				},
			},
			expected: "1\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := NewCountFormatter(&buf, tt.config)

			// Process all matches
			for _, match := range tt.matches {
				err := formatter.FormatMatch(match)
				if err != nil {
					t.Fatalf("Error formatting match: %v", err)
				}
			}

			// End file processing
			fileResult := FileResult{
				Path:       tt.matches[0].Path,
				MatchCount: len(tt.matches),
			}
			err := formatter.FormatFileEnd(fileResult)
			if err != nil {
				t.Fatalf("Error ending file: %v", err)
			}

			output := buf.String()
			if output != tt.expected {
				t.Errorf("Expected output:\n%q\nGot:\n%q", tt.expected, output)
			}
		})
	}
}

// TestFilesFormatter tests files-only formatting
func TestFilesFormatter(t *testing.T) {
	config := FormatterConfig{
		Format:     FormatFiles,
		ShowColors: false,
	}

	matches := []Match{
		{Path: "/path/to/file1.go"},
		{Path: "/path/to/file1.go"}, // Duplicate should not appear twice
		{Path: "/path/to/file2.go"},
		{Path: "/path/to/file3.go"},
	}

	var buf bytes.Buffer
	formatter := NewFilesFormatter(&buf, config)

	for _, match := range matches {
		err := formatter.FormatMatch(match)
		if err != nil {
			t.Fatalf("Error formatting match: %v", err)
		}
	}

	output := buf.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Should have 3 unique files
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %v", len(lines), lines)
	}

	expectedFiles := []string{
		"/path/to/file1.go",
		"/path/to/file2.go",
		"/path/to/file3.go",
	}

	for i, expectedFile := range expectedFiles {
		if lines[i] != expectedFile {
			t.Errorf("Line %d: expected %q, got %q", i, expectedFile, lines[i])
		}
	}
}

// TestFilesFormatterWithColors tests files formatter with colors
func TestFilesFormatterWithColors(t *testing.T) {
	config := FormatterConfig{
		Format:     FormatFiles,
		ShowColors: true,
	}

	match := Match{Path: "test.go"}

	var buf bytes.Buffer
	formatter := NewFilesFormatter(&buf, config)

	err := formatter.FormatMatch(match)
	if err != nil {
		t.Fatalf("Error formatting match: %v", err)
	}

	output := buf.String()

	// Should contain colored filename
	if !strings.Contains(output, Magenta+"test.go"+Reset) {
		t.Error("Expected output to contain colored filename")
	}
}

// TestTextFormatterHighlightLine tests line highlighting with overlapping matches
func TestTextFormatterHighlightLine(t *testing.T) {
	formatter := &TextFormatter{
		config: FormatterConfig{ShowColors: true},
	}

	tests := []struct {
		name       string
		line       string
		submatches []Submatch
		expected   string
	}{
		{
			name: "Single match",
			line: "hello world",
			submatches: []Submatch{
				{Text: "hello", Start: 0, End: 5},
			},
			expected: BrightRed + "hello" + Reset + " world",
		},
		{
			name: "Multiple non-overlapping matches",
			line: "hello world test",
			submatches: []Submatch{
				{Text: "hello", Start: 0, End: 5},
				{Text: "test", Start: 12, End: 16},
			},
			expected: BrightRed + "hello" + Reset + " world " + BrightRed + "test" + Reset,
		},
		{
			name: "Matches in wrong order",
			line: "hello world test",
			submatches: []Submatch{
				{Text: "test", Start: 12, End: 16},  // Second match listed first
				{Text: "hello", Start: 0, End: 5},  // First match listed second
			},
			expected: BrightRed + "hello" + Reset + " world " + BrightRed + "test" + Reset,
		},
		{
			name:       "No matches",
			line:       "hello world",
			submatches: []Submatch{},
			expected:   "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.highlightLine(tt.line, tt.submatches)
			if result != tt.expected {
				t.Errorf("Expected:\n%q\nGot:\n%q", tt.expected, result)
			}
		})
	}
}

// TestTextFormatterColorize tests color application
func TestTextFormatterColorize(t *testing.T) {
	tests := []struct {
		name     string
		colors   bool
		text     string
		color    string
		expected string
	}{
		{
			name:     "With colors",
			colors:   true,
			text:     "test",
			color:    Red,
			expected: Red + "test" + Reset,
		},
		{
			name:     "Without colors",
			colors:   false,
			text:     "test",
			color:    Red,
			expected: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := &TextFormatter{
				config: FormatterConfig{ShowColors: tt.colors},
			}

			result := formatter.colorize(tt.text, tt.color)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}