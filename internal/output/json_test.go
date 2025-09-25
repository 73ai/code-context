package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestJSONFormatter tests JSON output formatting
func TestJSONFormatter(t *testing.T) {
	config := FormatterConfig{Format: FormatJSON}

	match := Match{
		Path:           "/test/file.go",
		LineNumber:     42,
		Line:           "func test() {",
		AbsoluteOffset: 1000,
		Submatches: []Submatch{
			{Text: "test", Start: 5, End: 9},
		},
	}

	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf, config)

	// Test file begin
	err := formatter.FormatFileBegin("/test/file.go")
	if err != nil {
		t.Fatalf("Error formatting file begin: %v", err)
	}

	// Test match
	err = formatter.FormatMatch(match)
	if err != nil {
		t.Fatalf("Error formatting match: %v", err)
	}

	// Test file end
	fileResult := FileResult{
		Path:       "/test/file.go",
		MatchCount: 1,
		Stats: FileStats{
			Elapsed:           NewDuration(100 * time.Microsecond),
			Searches:          1,
			SearchesWithMatch: 1,
			BytesSearched:     50,
			BytesPrinted:      100,
			MatchedLines:      1,
			Matches:           1,
		},
	}
	err = formatter.FormatFileEnd(fileResult)
	if err != nil {
		t.Fatalf("Error formatting file end: %v", err)
	}

	// Test summary
	summary := SearchSummary{
		ElapsedTotal: NewDuration(200 * time.Microsecond),
		Stats: FileStats{
			Searches:          1,
			SearchesWithMatch: 1,
			BytesSearched:     50,
			BytesPrinted:      100,
			MatchedLines:      1,
			Matches:           1,
		},
	}
	err = formatter.FormatSummary(summary)
	if err != nil {
		t.Fatalf("Error formatting summary: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have 4 lines: begin, match, end, summary
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d: %v", len(lines), lines)
	}

	// Test each message type
	for i, line := range lines {
		var msg JSONMessage
		err := json.Unmarshal([]byte(line), &msg)
		if err != nil {
			t.Fatalf("Line %d: invalid JSON: %v", i, err)
		}

		switch i {
		case 0:
			if msg.Type != "begin" {
				t.Errorf("Line %d: expected type 'begin', got '%s'", i, msg.Type)
			}
		case 1:
			if msg.Type != "match" {
				t.Errorf("Line %d: expected type 'match', got '%s'", i, msg.Type)
			}
		case 2:
			if msg.Type != "end" {
				t.Errorf("Line %d: expected type 'end', got '%s'", i, msg.Type)
			}
		case 3:
			if msg.Type != "summary" {
				t.Errorf("Line %d: expected type 'summary', got '%s'", i, msg.Type)
			}
		}
	}
}

// TestJSONFormatterRipgrepCompatibility tests compatibility with ripgrep's JSON format
func TestJSONFormatterRipgrepCompatibility(t *testing.T) {
	config := FormatterConfig{Format: FormatJSON}

	match := Match{
		Path:           "/test/file.js",
		LineNumber:     1,
		Line:           "function test() {",
		AbsoluteOffset: 0,
		Submatches: []Submatch{
			{Text: "test", Start: 9, End: 13},
		},
	}

	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf, config)

	err := formatter.FormatFileBegin("/test/file.js")
	if err != nil {
		t.Fatalf("Error formatting file begin: %v", err)
	}

	err = formatter.FormatMatch(match)
	if err != nil {
		t.Fatalf("Error formatting match: %v", err)
	}

	fileResult := FileResult{
		Path:       "/test/file.js",
		MatchCount: 1,
		Stats: FileStats{
			Elapsed:           NewDuration(354 * time.Microsecond),
			Searches:          1,
			SearchesWithMatch: 1,
			BytesSearched:     17,
			BytesPrinted:      50,
			MatchedLines:      1,
			Matches:           1,
		},
	}
	err = formatter.FormatFileEnd(fileResult)
	if err != nil {
		t.Fatalf("Error formatting file end: %v", err)
	}

	output := buf.Bytes()

	// Validate ripgrep compatibility
	err = ValidateRipgrepCompatibility(output)
	if err != nil {
		t.Errorf("Output not ripgrep compatible: %v", err)
	}

	// Check specific format requirements
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Parse match message and verify structure
	var matchMsg JSONMessage
	err = json.Unmarshal([]byte(lines[1]), &matchMsg)
	if err != nil {
		t.Fatalf("Error parsing match message: %v", err)
	}

	// Convert to JSONMatchData to check fields
	matchData, ok := matchMsg.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Match data is not a map")
	}

	// Check required fields
	requiredFields := []string{"path", "lines", "line_number", "absolute_offset", "submatches"}
	for _, field := range requiredFields {
		if _, exists := matchData[field]; !exists {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// Check path structure
	pathData, ok := matchData["path"].(map[string]interface{})
	if !ok {
		t.Error("Path should be an object with 'text' field")
	} else if pathData["text"] != "/test/file.js" {
		t.Errorf("Expected path text '/test/file.js', got %v", pathData["text"])
	}

	// Check lines structure
	linesData, ok := matchData["lines"].(map[string]interface{})
	if !ok {
		t.Error("Lines should be an object with 'text' field")
	} else {
		lineText := linesData["text"].(string)
		if !strings.HasSuffix(lineText, "\n") {
			t.Error("Line text should end with newline for ripgrep compatibility")
		}
	}

	// Check submatches structure
	submatchesData, ok := matchData["submatches"].([]interface{})
	if !ok {
		t.Error("Submatches should be an array")
	} else if len(submatchesData) != 1 {
		t.Errorf("Expected 1 submatch, got %d", len(submatchesData))
	} else {
		submatch := submatchesData[0].(map[string]interface{})
		matchField := submatch["match"].(map[string]interface{})
		if matchField["text"] != "test" {
			t.Errorf("Expected submatch text 'test', got %v", matchField["text"])
		}
	}
}

// TestJSONFormatterNewlineHandling tests proper newline handling
func TestJSONFormatterNewlineHandling(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool // should end with newline
	}{
		{
			name:     "Line without newline",
			line:     "function test() {",
			expected: true,
		},
		{
			name:     "Line with newline",
			line:     "function test() {\n",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := FormatterConfig{Format: FormatJSON}
			match := Match{
				Path:       "test.js",
				LineNumber: 1,
				Line:       tt.line,
				Submatches: []Submatch{
					{Text: "test", Start: 9, End: 13},
				},
			}

			var buf bytes.Buffer
			formatter := NewJSONFormatter(&buf, config)

			err := formatter.FormatMatch(match)
			if err != nil {
				t.Fatalf("Error formatting match: %v", err)
			}

			output := buf.String()
			var msg JSONMessage
			err = json.Unmarshal([]byte(output), &msg)
			if err != nil {
				t.Fatalf("Error parsing JSON: %v", err)
			}

			matchData := msg.Data.(map[string]interface{})
			linesData := matchData["lines"].(map[string]interface{})
			lineText := linesData["text"].(string)

			hasNewline := strings.HasSuffix(lineText, "\n")
			if hasNewline != tt.expected {
				t.Errorf("Expected newline suffix: %v, got: %v (line: %q)", tt.expected, hasNewline, lineText)
			}
		})
	}
}

// TestJSONLinesFormatter tests JSON Lines format
func TestJSONLinesFormatter(t *testing.T) {
	config := FormatterConfig{Format: FormatJSON, IncludeSemantic: true}

	match := Match{
		Path:       "/test/file.go",
		Language:   "go",
		LineNumber: 10,
		Line:       "func TestFunction() {",
		Submatches: []Submatch{
			{Text: "Test", Start: 5, End: 9},
		},
		Semantic: &SemanticInfo{
			SymbolType: "function",
			SymbolName: "TestFunction",
		},
	}

	var buf bytes.Buffer
	formatter := NewJSONLinesFormatter(&buf, config)

	err := formatter.FormatMatch(match)
	if err != nil {
		t.Fatalf("Error formatting match: %v", err)
	}

	summary := SearchSummary{
		ElapsedTotal: NewDuration(100 * time.Microsecond),
		Stats:        FileStats{Matches: 1},
		Files:        map[string]int{"go": 1},
	}
	err = formatter.FormatSummary(summary)
	if err != nil {
		t.Fatalf("Error formatting summary: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %v", len(lines), lines)
	}

	// Parse match line
	var jsonMatch JSONLinesMatch
	err = json.Unmarshal([]byte(lines[0]), &jsonMatch)
	if err != nil {
		t.Fatalf("Error parsing JSON match: %v", err)
	}

	if jsonMatch.Path != match.Path {
		t.Errorf("Expected path %s, got %s", match.Path, jsonMatch.Path)
	}

	if jsonMatch.Semantic == nil {
		t.Error("Expected semantic information")
	} else if jsonMatch.Semantic.SymbolType != "function" {
		t.Errorf("Expected symbol type 'function', got %s", jsonMatch.Semantic.SymbolType)
	}

	// Parse summary line
	var summaryData map[string]interface{}
	err = json.Unmarshal([]byte(lines[1]), &summaryData)
	if err != nil {
		t.Fatalf("Error parsing JSON summary: %v", err)
	}

	if summaryData["type"] != "summary" {
		t.Errorf("Expected type 'summary', got %v", summaryData["type"])
	}

	if files, ok := summaryData["files"].(map[string]interface{}); !ok {
		t.Error("Expected files map in summary")
	} else if files["go"] != float64(1) {
		t.Errorf("Expected 1 go file, got %v", files["go"])
	}
}

// TestSemanticJSONFormatter tests semantic JSON extensions
func TestSemanticJSONFormatter(t *testing.T) {
	config := FormatterConfig{Format: FormatJSON, IncludeSemantic: true}

	match := Match{
		Path:       "/test/file.go",
		LineNumber: 10,
		Line:       "func TestFunction() {",
		Submatches: []Submatch{
			{Text: "Test", Start: 5, End: 9},
		},
		Semantic: &SemanticInfo{
			SymbolType: "function",
			SymbolName: "TestFunction",
			Scope:      "main",
			Definition: &Location{
				Path:         "/test/file.go",
				LineNumber:   10,
				ColumnNumber: 5,
			},
			References: []Location{
				{Path: "/test/other.go", LineNumber: 20, ColumnNumber: 8},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewSemanticJSONFormatter(&buf, config)

	err := formatter.FormatMatch(match)
	if err != nil {
		t.Fatalf("Error formatting match: %v", err)
	}

	output := buf.String()
	var msg JSONMessage
	err = json.Unmarshal([]byte(output), &msg)
	if err != nil {
		t.Fatalf("Error parsing JSON: %v", err)
	}

	matchData := msg.Data.(map[string]interface{})
	semanticData, exists := matchData["semantic"]
	if !exists {
		t.Error("Expected semantic data in match")
		return
	}

	semantic := semanticData.(map[string]interface{})
	if semantic["symbol_type"] != "function" {
		t.Errorf("Expected symbol_type 'function', got %v", semantic["symbol_type"])
	}

	if semantic["symbol_name"] != "TestFunction" {
		t.Errorf("Expected symbol_name 'TestFunction', got %v", semantic["symbol_name"])
	}

	// Check definition
	if definition, ok := semantic["definition"].(map[string]interface{}); !ok {
		t.Error("Expected definition object")
	} else {
		if definition["path"] != "/test/file.go" {
			t.Errorf("Expected definition path '/test/file.go', got %v", definition["path"])
		}
		if definition["line_number"] != float64(10) {
			t.Errorf("Expected definition line 10, got %v", definition["line_number"])
		}
	}

	// Check references
	if references, ok := semantic["references"].([]interface{}); !ok {
		t.Error("Expected references array")
	} else if len(references) != 1 {
		t.Errorf("Expected 1 reference, got %d", len(references))
	} else {
		ref := references[0].(map[string]interface{})
		if ref["path"] != "/test/other.go" {
			t.Errorf("Expected reference path '/test/other.go', got %v", ref["path"])
		}
	}
}

// TestValidateRipgrepCompatibility tests the validation function
func TestValidateRipgrepCompatibility(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "Valid ripgrep JSON",
			input: `{"type":"begin","data":{"path":{"text":"/test/file.go"}}}
{"type":"match","data":{"path":{"text":"/test/file.go"},"lines":{"text":"func test()\n"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"test"},"start":5,"end":9}]}}
{"type":"end","data":{"path":{"text":"/test/file.go"},"binary_offset":null,"stats":{"elapsed":{"secs":0,"nanos":100000,"human":"100µs"},"searches":1,"searches_with_match":1,"bytes_searched":10,"bytes_printed":50,"matched_lines":1,"matches":1}}}`,
			wantErr: false,
		},
		{
			name: "Invalid JSON",
			input: `{"type":"begin","data":{"path":{"text":"/test/file.go"}}
invalid json line`,
			wantErr: true,
		},
		{
			name: "Invalid message type",
			input: `{"type":"invalid","data":{}}`,
			wantErr: true,
		},
		{
			name: "Missing required field in begin",
			input: `{"type":"begin","data":{}}`,
			wantErr: true,
		},
		{
			name: "Missing required field in match",
			input: `{"type":"match","data":{"path":{"text":"test.go"}}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRipgrepCompatibility([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRipgrepCompatibility() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// BenchmarkJSONFormatterDetailed benchmarks JSON formatting performance
func BenchmarkJSONFormatterDetailed(b *testing.B) {
	config := FormatterConfig{Format: FormatJSON}
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
		var buf bytes.Buffer
		formatter := NewJSONFormatter(&buf, config)
		formatter.FormatMatch(match)
	}
}

// BenchmarkJSONLinesFormatter benchmarks JSON Lines formatting performance
func BenchmarkJSONLinesFormatter(b *testing.B) {
	config := FormatterConfig{Format: FormatJSON, IncludeSemantic: true}
	match := Match{
		Path:       "/test/file.go",
		LineNumber: 100,
		Line:       "func TestFunction() error { return nil }",
		Submatches: []Submatch{
			{Text: "TestFunction", Start: 5, End: 17},
		},
		Semantic: &SemanticInfo{
			SymbolType: "function",
			SymbolName: "TestFunction",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		formatter := NewJSONLinesFormatter(&buf, config)
		formatter.FormatMatch(match)
	}
}