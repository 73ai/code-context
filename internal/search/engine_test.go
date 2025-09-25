package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	if engine == nil {
		t.Fatal("Engine is nil")
	}

	if engine.regexSearcher == nil {
		t.Error("Regex searcher is nil")
	}

	if engine.semanticSearcher == nil {
		t.Error("Semantic searcher is nil")
	}

	// Clean up
	if err := engine.Close(); err != nil {
		t.Errorf("Failed to close engine: %v", err)
	}
}

func TestEngineRegexSearch(t *testing.T) {
	// Create test directory and files
	testDir := createTestDirectory(t)
	defer os.RemoveAll(testDir)

	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	opts := &SearchOptions{
		Pattern:     "test",
		SearchPaths: []string{testDir},
		SearchMode:  ModeRegex,
		LineNumbers: true,
	}

	var output strings.Builder
	ctx := context.Background()

	err = engine.Search(ctx, opts, &output)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	result := output.String()
	if result == "" {
		t.Error("No search results returned")
	}

	// Check that results contain expected content
	if !strings.Contains(result, "test") {
		t.Error("Search results don't contain the search pattern")
	}

	// Verify stats
	stats := engine.Stats()
	if stats.FilesSearched == 0 {
		t.Error("No files were searched")
	}

	if stats.SearchDuration == 0 {
		t.Error("Search duration not recorded")
	}
}

func TestEngineSemanticSearch(t *testing.T) {
	// Create test Go file
	testDir := createGoTestDirectory(t)
	defer os.RemoveAll(testDir)

	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	opts := &SearchOptions{
		Pattern:     "TestFunction",
		SearchPaths: []string{testDir},
		SearchMode:  ModeSemantic,
		FindDefs:    true,
	}

	var output strings.Builder
	ctx := context.Background()

	err = engine.Search(ctx, opts, &output)
	if err != nil {
		t.Fatalf("Semantic search failed: %v", err)
	}

	result := output.String()
	if result == "" {
		t.Error("No semantic search results returned")
	}

	// Verify stats include index duration
	stats := engine.Stats()
	if stats.IndexDuration == 0 {
		t.Error("Index duration not recorded for semantic search")
	}
}

func TestEngineHybridSearch(t *testing.T) {
	testDir := createGoTestDirectory(t)
	defer os.RemoveAll(testDir)

	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	opts := &SearchOptions{
		Pattern:     "func",
		SearchPaths: []string{testDir},
		SearchMode:  ModeHybrid,
		LineNumbers: true,
	}

	var output strings.Builder
	ctx := context.Background()

	err = engine.Search(ctx, opts, &output)
	if err != nil {
		t.Fatalf("Hybrid search failed: %v", err)
	}

	result := output.String()
	if result == "" {
		t.Error("No hybrid search results returned")
	}
}

func TestEngineSearchTimeout(t *testing.T) {
	testDir := createTestDirectory(t)
	defer os.RemoveAll(testDir)

	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	opts := &SearchOptions{
		Pattern:     "test",
		SearchPaths: []string{testDir},
		SearchMode:  ModeRegex,
		Timeout:     1 * time.Nanosecond, // Very short timeout
	}

	var output strings.Builder
	ctx := context.Background()

	err = engine.Search(ctx, opts, &output)
	if err == nil {
		t.Error("Expected timeout error, but got none")
	}

	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestEngineJSONOutput(t *testing.T) {
	testDir := createTestDirectory(t)
	defer os.RemoveAll(testDir)

	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	opts := &SearchOptions{
		Pattern:     "test",
		SearchPaths: []string{testDir},
		SearchMode:  ModeRegex,
		JSON:        true,
	}

	var output strings.Builder
	ctx := context.Background()

	err = engine.Search(ctx, opts, &output)
	if err != nil {
		t.Fatalf("JSON search failed: %v", err)
	}

	result := output.String()
	if result == "" {
		t.Error("No JSON search results returned")
	}

	// Verify JSON format
	if !strings.Contains(result, `"type":`) {
		t.Error("Output doesn't appear to be valid JSON")
	}
}

func TestEngineSearchOptions(t *testing.T) {
	testDir := createTestDirectory(t)
	defer os.RemoveAll(testDir)

	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	tests := []struct {
		name string
		opts *SearchOptions
	}{
		{
			name: "Case sensitive search",
			opts: &SearchOptions{
				Pattern:       "Test",
				SearchPaths:   []string{testDir},
				SearchMode:    ModeRegex,
				CaseSensitive: true,
			},
		},
		{
			name: "Whole word search",
			opts: &SearchOptions{
				Pattern:     "test",
				SearchPaths: []string{testDir},
				SearchMode:  ModeRegex,
				WholeWord:   true,
			},
		},
		{
			name: "Context search",
			opts: &SearchOptions{
				Pattern:     "test",
				SearchPaths: []string{testDir},
				SearchMode:  ModeRegex,
				Context:     2,
			},
		},
		{
			name: "Count only",
			opts: &SearchOptions{
				Pattern:     "test",
				SearchPaths: []string{testDir},
				SearchMode:  ModeRegex,
				Count:       true,
			},
		},
		{
			name: "Files with matches",
			opts: &SearchOptions{
				Pattern:          "test",
				SearchPaths:      []string{testDir},
				SearchMode:       ModeRegex,
				FilesWithMatches: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output strings.Builder
			ctx := context.Background()

			err := engine.Search(ctx, tt.opts, &output)
			if err != nil {
				t.Fatalf("Search failed for %s: %v", tt.name, err)
			}

			result := output.String()
			if result == "" {
				t.Errorf("No results for %s", tt.name)
			}
		})
	}
}

// Helper functions for testing

func createTestDirectory(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "search_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create test files
	testFiles := map[string]string{
		"file1.txt": "This is a test file\nwith multiple lines\ncontaining test data",
		"file2.txt": "Another test file\nwith different content\nfor testing purposes",
		"file3.log": "Log file with test entries\nINFO: test started\nERROR: test failed",
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	return tmpDir
}

func createGoTestDirectory(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "semantic_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create test Go files
	goContent := `package main

import "fmt"

// TestFunction is a test function
func TestFunction() {
	fmt.Println("Hello, World!")
}

type TestStruct struct {
	Field1 string
	Field2 int
}

const TestConstant = "test value"

var TestVariable = 42

func (ts *TestStruct) TestMethod() {
	// Test method implementation
}
`

	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create Go test file: %v", err)
	}

	return tmpDir
}

func BenchmarkEngineRegexSearch(b *testing.B) {
	testDir := createBenchmarkDirectory(b)
	defer os.RemoveAll(testDir)

	engine, err := NewEngine()
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	opts := &SearchOptions{
		Pattern:     "function",
		SearchPaths: []string{testDir},
		SearchMode:  ModeRegex,
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var output strings.Builder
		if err := engine.Search(ctx, opts, &output); err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}


func createBenchmarkDirectory(b *testing.B) string {
	tmpDir, err := os.MkdirTemp("", "search_benchmark")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create multiple test files for benchmarking
	for i := 0; i < 100; i++ {
		content := strings.Repeat("This is a test function with some content\n", 100)
		filename := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			b.Fatalf("Failed to create benchmark file: %v", err)
		}
	}

	// Create Go files for semantic search benchmarks
	goContent := `package main

func TestFunction%d() {
	// This is a test function
	for i := 0; i < 100; i++ {
		// Do something
	}
}

type TestStruct%d struct {
	Field1 string
	Field2 int
}
`

	for i := 0; i < 20; i++ {
		content := fmt.Sprintf(goContent, i, i)
		filename := filepath.Join(tmpDir, fmt.Sprintf("test%d.go", i))
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			b.Fatalf("Failed to create benchmark Go file: %v", err)
		}
	}

	return tmpDir
}