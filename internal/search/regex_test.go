package search

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewRegexSearcher(t *testing.T) {
	searcher, err := NewRegexSearcher()
	if err != nil {
		t.Fatalf("Failed to create regex searcher: %v", err)
	}

	if searcher == nil {
		t.Fatal("Regex searcher is nil")
	}

	if searcher.patterns == nil {
		t.Error("Patterns map is nil")
	}

	if searcher.semaphore == nil {
		t.Error("Semaphore is nil")
	}

	// Clean up
	if err := searcher.Close(); err != nil {
		t.Errorf("Failed to close searcher: %v", err)
	}
}

func TestRegexSearcherBasicSearch(t *testing.T) {
	testDir := createTestDirectory(t)
	defer os.RemoveAll(testDir)

	searcher, err := NewRegexSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:     "test",
		SearchPaths: []string{testDir},
		LineNumbers: true,
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	var resultCount int
	var searchErr error

	done := false
	for !done {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}
			resultCount++

			// Verify result structure
			if result.FilePath == "" {
				t.Error("Result missing file path")
			}
			if result.LineNumber <= 0 {
				t.Error("Result missing line number")
			}
			if !strings.Contains(strings.ToLower(result.Line), "test") &&
				!strings.Contains(strings.ToLower(result.Match), "test") {
				t.Error("Result doesn't contain search pattern")
			}

		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			if searchErr == nil {
				searchErr = err
			}

		case <-time.After(5 * time.Second):
			t.Fatal("Search timeout")
		}

		if results == nil && errs == nil {
			done = true
		}
	}

	if searchErr != nil {
		t.Fatalf("Search error: %v", searchErr)
	}

	if resultCount == 0 {
		t.Error("No search results found")
	}

	// Check statistics
	stats := searcher.Stats()
	if stats.FilesSearched == 0 {
		t.Error("No files were searched according to stats")
	}

	if stats.TotalMatches == 0 {
		t.Error("No matches recorded in stats")
	}
}

func TestRegexSearcherCaseSensitive(t *testing.T) {
	testDir := createTestDirectory(t)
	defer os.RemoveAll(testDir)

	searcher, err := NewRegexSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	// Test case sensitive search
	opts := &SearchOptions{
		Pattern:       "Test", // Capital T
		SearchPaths:   []string{testDir},
		CaseSensitive: true,
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	resultCount := 0
	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}
			resultCount++

			// Verify result contains exact case
			if !strings.Contains(result.Line, "Test") {
				t.Errorf("Case sensitive search returned incorrect result: %s", result.Line)
			}

		case err := <-errs:
			t.Fatalf("Search error: %v", err)

		case <-time.After(5 * time.Second):
			t.Fatal("Search timeout")
		}

		if results == nil {
			break
		}
	}

	// Test case insensitive search
	opts.CaseSensitive = false
	results, errs = searcher.Search(ctx, opts)

	caseInsensitiveCount := 0
	for {
		select {
		case _, ok := <-results:
			if !ok {
				results = nil
				break
			}
			caseInsensitiveCount++

		case err := <-errs:
			t.Fatalf("Case insensitive search error: %v", err)

		case <-time.After(5 * time.Second):
			t.Fatal("Case insensitive search timeout")
		}

		if results == nil {
			break
		}
	}

	// Case insensitive should find more matches
	if caseInsensitiveCount <= resultCount {
		t.Errorf("Case insensitive search should find more matches: %d vs %d", caseInsensitiveCount, resultCount)
	}
}

func TestRegexSearcherWholeWord(t *testing.T) {
	// Create test file with specific content
	tmpDir, err := os.MkdirTemp("", "wholeword_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := "testing test tested tester\npretest posttest\ntest standalone"
	filePath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	searcher, err := NewRegexSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	// Test whole word search
	opts := &SearchOptions{
		Pattern:     "test",
		SearchPaths: []string{tmpDir},
		WholeWord:   true,
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	var resultCount int
	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}
			resultCount++

			// Verify that only whole word matches are returned
			if strings.Contains(result.Line, "testing") || strings.Contains(result.Line, "tested") {
				t.Errorf("Whole word search returned partial match: %s", result.Line)
			}

		case err := <-errs:
			t.Fatalf("Search error: %v", err)

		case <-time.After(5 * time.Second):
			t.Fatal("Search timeout")
		}

		if results == nil {
			break
		}
	}

	if resultCount == 0 {
		t.Error("Whole word search found no results")
	}
}

func TestRegexSearcherContextLines(t *testing.T) {
	// Create test file with numbered lines
	tmpDir, err := os.MkdirTemp("", "context_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	lines := []string{
		"line 1",
		"line 2",
		"line 3 with test",
		"line 4",
		"line 5",
	}
	content := strings.Join(lines, "\n")
	filePath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	searcher, err := NewRegexSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:     "test",
		SearchPaths: []string{tmpDir},
		Context:     2, // 2 lines before and after
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	var result SearchResult
	select {
	case result = <-results:
		// Got result
	case err := <-errs:
		t.Fatalf("Search error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Search timeout")
	}

	if len(result.Context) == 0 {
		t.Error("No context lines returned")
	}

	// Should include lines before and after the match
	expectedContext := []string{"line 1", "line 2", "line 4", "line 5"}
	if len(result.Context) != len(expectedContext) {
		t.Errorf("Expected %d context lines, got %d", len(expectedContext), len(result.Context))
	}
}

func TestRegexSearcherCountOnly(t *testing.T) {
	testDir := createTestDirectory(t)
	defer os.RemoveAll(testDir)

	searcher, err := NewRegexSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:     "test",
		SearchPaths: []string{testDir},
		Count:       true,
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	var resultCount int
	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}
			resultCount++

			// For count mode, the Match field should contain the count
			if result.Match == "" {
				t.Error("Count result missing match count")
			}
			if result.Match == "0" {
				t.Error("Count should not be zero")
			}

		case err := <-errs:
			t.Fatalf("Search error: %v", err)

		case <-time.After(5 * time.Second):
			t.Fatal("Search timeout")
		}

		if results == nil {
			break
		}
	}

	if resultCount == 0 {
		t.Error("No count results returned")
	}
}

func TestRegexSearcherFilesWithMatches(t *testing.T) {
	testDir := createTestDirectory(t)
	defer os.RemoveAll(testDir)

	searcher, err := NewRegexSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:          "test",
		SearchPaths:      []string{testDir},
		FilesWithMatches: true,
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	var resultCount int
	seenFiles := make(map[string]bool)

	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}
			resultCount++

			// For files with matches mode, should only get file paths
			if result.FilePath == "" {
				t.Error("Files with matches result missing file path")
			}

			// Should not get the same file twice
			if seenFiles[result.FilePath] {
				t.Errorf("Duplicate file in results: %s", result.FilePath)
			}
			seenFiles[result.FilePath] = true

		case err := <-errs:
			t.Fatalf("Search error: %v", err)

		case <-time.After(5 * time.Second):
			t.Fatal("Search timeout")
		}

		if results == nil {
			break
		}
	}

	if resultCount == 0 {
		t.Error("No files with matches returned")
	}
}

func TestRegexSearcherOnlyMatching(t *testing.T) {
	testDir := createTestDirectory(t)
	defer os.RemoveAll(testDir)

	searcher, err := NewRegexSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:      "test",
		SearchPaths:  []string{testDir},
		OnlyMatching: true,
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	var resultCount int
	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}
			resultCount++

			// For only matching mode, should get individual matches
			if result.Match == "" {
				t.Error("Only matching result missing match text")
			}
			if result.Match != "test" {
				t.Errorf("Expected match 'test', got '%s'", result.Match)
			}

		case err := <-errs:
			t.Fatalf("Search error: %v", err)

		case <-time.After(5 * time.Second):
			t.Fatal("Search timeout")
		}

		if results == nil {
			break
		}
	}

	if resultCount == 0 {
		t.Error("No only matching results returned")
	}
}

func TestRegexSearcherInvertMatch(t *testing.T) {
	// Create test file with specific content
	tmpDir, err := os.MkdirTemp("", "invert_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := "line with test\nline without match\nanother test line\nplain line"
	filePath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	searcher, err := NewRegexSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:     "test",
		SearchPaths: []string{tmpDir},
		InvertMatch: true,
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	var resultCount int
	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}
			resultCount++

			// Inverted matches should not contain the pattern
			if strings.Contains(result.Line, "test") {
				t.Errorf("Inverted match should not contain pattern: %s", result.Line)
			}

		case err := <-errs:
			t.Fatalf("Search error: %v", err)

		case <-time.After(5 * time.Second):
			t.Fatal("Search timeout")
		}

		if results == nil {
			break
		}
	}

	if resultCount == 0 {
		t.Error("No inverted match results returned")
	}

	// Should find 2 lines without "test"
	if resultCount != 2 {
		t.Errorf("Expected 2 inverted matches, got %d", resultCount)
	}
}

func TestRegexSearcherMultiline(t *testing.T) {
	// Create test file with multiline pattern
	tmpDir, err := os.MkdirTemp("", "multiline_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := "start\ntest\npattern\nend"
	filePath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	searcher, err := NewRegexSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:     "test.*pattern",
		SearchPaths: []string{tmpDir},
		Multiline:   true,
		DotMatchAll: true,
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	var resultCount int
	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}
			resultCount++

			// Multiline match should span multiple lines
			if !strings.Contains(result.Match, "\n") {
				t.Error("Multiline match should contain newlines")
			}

		case err := <-errs:
			t.Fatalf("Search error: %v", err)

		case <-time.After(5 * time.Second):
			t.Fatal("Search timeout")
		}

		if results == nil {
			break
		}
	}

	if resultCount == 0 {
		t.Error("No multiline results returned")
	}
}

func BenchmarkRegexSearcherSmallFiles(b *testing.B) {
	testDir := createBenchmarkDirectory(b)
	defer os.RemoveAll(testDir)

	searcher, err := NewRegexSearcher()
	if err != nil {
		b.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:     "function",
		SearchPaths: []string{testDir},
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		results, errs := searcher.Search(ctx, opts)

		// Consume results
		for {
			select {
			case _, ok := <-results:
				if !ok {
					results = nil
				}
			case err := <-errs:
				b.Fatalf("Benchmark search error: %v", err)
			}

			if results == nil {
				break
			}
		}
	}
}

func BenchmarkRegexSearcherLargeFiles(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "large_benchmark")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a large file
	largeContent := strings.Repeat("This is a test function with some content\n", 10000)
	filePath := filepath.Join(tmpDir, "large.txt")
	if err := os.WriteFile(filePath, []byte(largeContent), 0644); err != nil {
		b.Fatalf("Failed to create large file: %v", err)
	}

	searcher, err := NewRegexSearcher()
	if err != nil {
		b.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:     "function",
		SearchPaths: []string{tmpDir},
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		results, errs := searcher.Search(ctx, opts)

		// Consume results
		for {
			select {
			case _, ok := <-results:
				if !ok {
					results = nil
				}
			case err := <-errs:
				b.Fatalf("Benchmark search error: %v", err)
			}

			if results == nil {
				break
			}
		}
	}
}