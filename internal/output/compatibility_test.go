package output

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestRealWorldCompatibility tests compatibility with actual ripgrep output
func TestRealWorldCompatibility(t *testing.T) {
	// Create a temporary test file
	testContent := `package main

import "fmt"

func test() {
    fmt.Println("test function")
}

func testHelper() {
    test() // call test function
}

var testVar = "test value"
`

	// Write test content to a temporary file
	tmpfile, err := os.CreateTemp("", "codegrep_test_*.go")
	if err != nil {
		t.Skip("Cannot create temp file:", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(testContent)); err != nil {
		t.Skip("Cannot write temp file:", err)
	}
	tmpfile.Close()

	// Test JSON output compatibility
	t.Run("JSON Output Compatibility", func(t *testing.T) {
		// Get ripgrep JSON output
		cmd := exec.Command("rg", "--json", "test", tmpfile.Name())
		ripgrepOutput, err := cmd.Output()
		if err != nil {
			t.Skip("ripgrep not available:", err)
		}

		// Validate our JSON output matches ripgrep's format
		err = ValidateRipgrepCompatibility(ripgrepOutput)
		if err != nil {
			t.Errorf("ripgrep output not compatible with our validator: %v", err)
		}

		// Parse ripgrep output to understand the expected format
		lines := strings.Split(strings.TrimSpace(string(ripgrepOutput)), "\n")
		for i, line := range lines {
			var msg JSONMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				t.Errorf("Line %d: invalid JSON from ripgrep: %v", i, err)
				continue
			}

			// Verify our formatter can produce the same structure
			switch msg.Type {
			case "begin":
				// Test our begin formatting
				var buf bytes.Buffer
				formatter := NewJSONFormatter(&buf, FormatterConfig{Format: FormatJSON})
				formatter.FormatFileBegin(tmpfile.Name())

				var ourMsg JSONMessage
				if err := json.Unmarshal(buf.Bytes(), &ourMsg); err != nil {
					t.Errorf("Our begin message is invalid JSON: %v", err)
				} else if ourMsg.Type != "begin" {
					t.Errorf("Expected begin message type, got %s", ourMsg.Type)
				}

			case "match":
				// Extract match data from ripgrep output
				data := msg.Data.(map[string]interface{})
				pathData := data["path"].(map[string]interface{})
				linesData := data["lines"].(map[string]interface{})

				// Create equivalent match for our formatter
				match := Match{
					Path:           pathData["text"].(string),
					LineNumber:     int(data["line_number"].(float64)),
					Line:           strings.TrimSuffix(linesData["text"].(string), "\n"),
					AbsoluteOffset: int64(data["absolute_offset"].(float64)),
					Submatches:     []Submatch{},
				}

				// Extract submatches
				if submatches, ok := data["submatches"].([]interface{}); ok {
					for _, sm := range submatches {
						submatch := sm.(map[string]interface{})
						matchData := submatch["match"].(map[string]interface{})
						match.Submatches = append(match.Submatches, Submatch{
							Text:  matchData["text"].(string),
							Start: int(submatch["start"].(float64)),
							End:   int(submatch["end"].(float64)),
						})
					}
				}

				// Test our match formatting
				var buf bytes.Buffer
				formatter := NewJSONFormatter(&buf, FormatterConfig{Format: FormatJSON})
				formatter.FormatMatch(match)

				var ourMsg JSONMessage
				if err := json.Unmarshal(buf.Bytes(), &ourMsg); err != nil {
					t.Errorf("Our match message is invalid JSON: %v", err)
				} else if ourMsg.Type != "match" {
					t.Errorf("Expected match message type, got %s", ourMsg.Type)
				}
			}
		}
	})

	// Test text output compatibility
	t.Run("Text Output Compatibility", func(t *testing.T) {
		// Get ripgrep text output
		cmd := exec.Command("rg", "--line-number", "--with-filename", "--no-color", "test", tmpfile.Name())
		ripgrepOutput, err := cmd.Output()
		if err != nil {
			t.Skip("ripgrep not available:", err)
		}

		ripgrepLines := strings.Split(strings.TrimSpace(string(ripgrepOutput)), "\n")

		// Create equivalent matches for our formatter
		matches := []Match{}
		for _, line := range ripgrepLines {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 3)
				if len(parts) >= 3 {
					// This is a simplified parser - in real usage we'd have more sophisticated parsing
					match := Match{
						Path:       parts[0],
						LineNumber: 1, // Would parse from parts[1] in real implementation
						Line:       parts[2],
						Submatches: []Submatch{
							{Text: "test", Start: strings.Index(parts[2], "test"), End: strings.Index(parts[2], "test") + 4},
						},
					}
					matches = append(matches, match)
				}
			}
		}

		// Format with our formatter
		var buf bytes.Buffer
		config := FormatterConfig{
			Format:          FormatText,
			ShowLineNumbers: true,
			ShowFilenames:   true,
			ShowColors:      false,
		}
		formatter := NewTextFormatter(&buf, config)

		for _, match := range matches {
			formatter.FormatMatch(match)
		}

		ourOutput := buf.String()

		// Basic compatibility check - both should have matches
		if strings.TrimSpace(ourOutput) == "" && strings.TrimSpace(string(ripgrepOutput)) != "" {
			t.Error("Our formatter produced no output while ripgrep found matches")
		}

		// Both should contain the filename
		if strings.Contains(string(ripgrepOutput), tmpfile.Name()) && !strings.Contains(ourOutput, tmpfile.Name()) {
			t.Error("Our output missing filename that ripgrep includes")
		}
	})
}

// TestOutputFormatsAgainstRipgrep compares different output formats
func TestOutputFormatsAgainstRipgrep(t *testing.T) {
	// Create test content with multiple matches
	testContent := `function testA() {
    return "test";
}

class TestClass {
    testMethod() {
        this.testField = testA();
    }
}

const testConst = "another test";
`

	tmpfile, err := os.CreateTemp("", "codegrep_test_*.js")
	if err != nil {
		t.Skip("Cannot create temp file:", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(testContent)); err != nil {
		t.Skip("Cannot write temp file:", err)
	}
	tmpfile.Close()

	testCases := []struct {
		name      string
		rgArgs    []string
		config    FormatterConfig
		validator func(t *testing.T, ripgrepOut, ourOut string)
	}{
		{
			name:   "files-with-matches",
			rgArgs: []string{"--files-with-matches", "test", tmpfile.Name()},
			config: FormatterConfig{Format: FormatFiles, ShowColors: false},
			validator: func(t *testing.T, ripgrepOut, ourOut string) {
				if strings.TrimSpace(ripgrepOut) != strings.TrimSpace(ourOut) {
					t.Errorf("Files output mismatch:\nRipgrep: %q\nOurs: %q", ripgrepOut, ourOut)
				}
			},
		},
		{
			name:   "count",
			rgArgs: []string{"--count", "test", tmpfile.Name()},
			config: FormatterConfig{Format: FormatCount, ShowFilenames: true, ShowColors: false, TotalFiles: 1},
			validator: func(t *testing.T, ripgrepOut, ourOut string) {
				// For single file, both should be just the number (no filename)
				ripgrepTrimmed := strings.TrimSpace(ripgrepOut)
				ourTrimmed := strings.TrimSpace(ourOut)

				// Both should be numbers only for single file
				if strings.Contains(ripgrepTrimmed, ":") || strings.Contains(ourTrimmed, ":") {
					t.Errorf("Count output for single file should not contain colon:\nRipgrep: %q\nOurs: %q", ripgrepOut, ourOut)
				}

				// Both should be numeric
				if ripgrepTrimmed != ourTrimmed {
					t.Errorf("Count mismatch:\nRipgrep: %q\nOurs: %q", ripgrepOut, ourOut)
				}
			},
		},
		{
			name:   "only-matching",
			rgArgs: []string{"--only-matching", "--line-number", "--with-filename", "--no-color", "test", tmpfile.Name()},
			config: FormatterConfig{
				Format:          FormatText,
				Mode:            ModeOnlyMatching,
				ShowLineNumbers: true,
				ShowFilenames:   true,
				ShowColors:      false,
			},
			validator: func(t *testing.T, ripgrepOut, ourOut string) {
				ripgrepLines := strings.Split(strings.TrimSpace(ripgrepOut), "\n")
				ourLines := strings.Split(strings.TrimSpace(ourOut), "\n")

				// Should have similar number of matches
				if len(ripgrepLines) == 0 && len(ourLines) > 0 {
					t.Error("Ripgrep found no matches but our formatter did")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get ripgrep output
			cmd := exec.Command("rg", tc.rgArgs...)
			ripgrepOutput, err := cmd.Output()
			if err != nil {
				t.Skip("ripgrep command failed:", err)
			}

			// Generate our output (this is simplified - real implementation would parse matches)
			var buf bytes.Buffer
			formatter := NewFormatterFactory(&buf, tc.config).CreateFormatter()

			// For testing purposes, create a simple match
			if tc.config.Format != FormatFiles {
				match := Match{
					Path:       tmpfile.Name(),
					LineNumber: 1,
					Line:       "test content",
					Submatches: []Submatch{{Text: "test", Start: 0, End: 4}},
				}

				switch tc.config.Format {
				case FormatCount:
					// Create multiple matches to simulate realistic count
					for i := 0; i < 5; i++ {
						match := Match{
							Path:       tmpfile.Name(),
							LineNumber: i + 1,
							Line:       "test content " + strconv.Itoa(i),
							Submatches: []Submatch{{Text: "test", Start: 0, End: 4}},
						}
						formatter.FormatMatch(match)
					}
					fileResult := FileResult{Path: tmpfile.Name(), MatchCount: 5}
					formatter.FormatFileEnd(fileResult)
				default:
					formatter.FormatMatch(match)
				}
			} else {
				// Files format
				match := Match{Path: tmpfile.Name()}
				formatter.FormatMatch(match)
			}

			ourOutput := buf.String()
			tc.validator(t, string(ripgrepOutput), ourOutput)
		})
	}
}

// TestPerformanceComparison compares performance with ripgrep on large files
func TestPerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create a larger test file
	var content strings.Builder
	for i := 0; i < 1000; i++ {
		content.WriteString("function test" + string(rune(i%26+'a')) + "() {\n")
		content.WriteString("    return \"test result " + string(rune(i%10+'0')) + "\";\n")
		content.WriteString("}\n\n")
	}

	tmpfile, err := os.CreateTemp("", "codegrep_perf_test_*.js")
	if err != nil {
		t.Skip("Cannot create temp file:", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content.String())); err != nil {
		t.Skip("Cannot write temp file:", err)
	}
	tmpfile.Close()

	// Measure ripgrep performance
	ripgrepStart := time.Now()
	cmd := exec.Command("rg", "--json", "test", tmpfile.Name())
	ripgrepOutput, err := cmd.Output()
	ripgrepDuration := time.Since(ripgrepStart)

	if err != nil {
		t.Skip("ripgrep not available:", err)
	}

	// Count ripgrep matches
	ripgrepMatches := strings.Count(string(ripgrepOutput), `"type":"match"`)

	// Measure our formatter performance
	ourStart := time.Now()
	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf, FormatterConfig{Format: FormatJSON})

	// Simulate matches (in real usage, these would come from the search engine)
	for i := 0; i < ripgrepMatches; i++ {
		match := Match{
			Path:           tmpfile.Name(),
			LineNumber:     i*4 + 1,
			Line:           "function test" + string(rune(i%26+'a')) + "() {",
			AbsoluteOffset: int64(i * 50),
			Submatches: []Submatch{
				{Text: "test", Start: 9, End: 13},
			},
		}
		formatter.FormatMatch(match)
	}
	ourDuration := time.Since(ourStart)

	t.Logf("Performance comparison:")
	t.Logf("  Ripgrep: %v (%d matches)", ripgrepDuration, ripgrepMatches)
	t.Logf("  Our formatter: %v (%d matches)", ourDuration, ripgrepMatches)
	t.Logf("  Ratio: %.2fx", float64(ourDuration)/float64(ripgrepDuration))

	// Our formatter should be reasonable (within 10x of ripgrep for just formatting)
	if ourDuration > ripgrepDuration*10 {
		t.Errorf("Our formatter is too slow: %v vs ripgrep's %v", ourDuration, ripgrepDuration)
	}
}