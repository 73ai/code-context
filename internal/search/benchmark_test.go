package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// BenchmarkSuite contains comprehensive benchmarks for the search engine
type BenchmarkSuite struct {
	testDataDir string
	smallFiles  []string
	mediumFiles []string
	largeFiles  []string
	goFiles     []string
}

func NewBenchmarkSuite() *BenchmarkSuite {
	return &BenchmarkSuite{}
}

func (bs *BenchmarkSuite) Setup() error {
	var err error
	bs.testDataDir, err = os.MkdirTemp("", "search_benchmark")
	if err != nil {
		return fmt.Errorf("failed to create benchmark directory: %w", err)
	}

	// Create different sized files
	if err := bs.createSmallFiles(); err != nil {
		return fmt.Errorf("failed to create small files: %w", err)
	}

	if err := bs.createMediumFiles(); err != nil {
		return fmt.Errorf("failed to create medium files: %w", err)
	}

	if err := bs.createLargeFiles(); err != nil {
		return fmt.Errorf("failed to create large files: %w", err)
	}

	if err := bs.createGoFiles(); err != nil {
		return fmt.Errorf("failed to create Go files: %w", err)
	}

	return nil
}

func (bs *BenchmarkSuite) Cleanup() {
	if bs.testDataDir != "" {
		os.RemoveAll(bs.testDataDir)
	}
}

func (bs *BenchmarkSuite) createSmallFiles() error {
	// Create 100 small files (1-10 lines each)
	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("line 1 with test content %d\nline 2 with function call\nline 3 with variable assignment\n", i)
		filename := filepath.Join(bs.testDataDir, fmt.Sprintf("small_%d.txt", i))

		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return err
		}

		bs.smallFiles = append(bs.smallFiles, filename)
	}
	return nil
}

func (bs *BenchmarkSuite) createMediumFiles() error {
	// Create 20 medium files (100-1000 lines each)
	for i := 0; i < 20; i++ {
		var lines []string
		for j := 0; j < 500; j++ {
			lines = append(lines, fmt.Sprintf("line %d with test content function variable %d", j, i))
		}
		content := strings.Join(lines, "\n")

		filename := filepath.Join(bs.testDataDir, fmt.Sprintf("medium_%d.txt", i))
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return err
		}

		bs.mediumFiles = append(bs.mediumFiles, filename)
	}
	return nil
}

func (bs *BenchmarkSuite) createLargeFiles() error {
	// Create 5 large files (10,000+ lines each)
	for i := 0; i < 5; i++ {
		var lines []string
		for j := 0; j < 10000; j++ {
			if j%100 == 0 {
				lines = append(lines, fmt.Sprintf("// Function definition %d", j))
			} else if j%50 == 0 {
				lines = append(lines, fmt.Sprintf("var testVariable%d = \"test value %d\"", j, i))
			} else {
				lines = append(lines, fmt.Sprintf("    // Regular line %d with some test content", j))
			}
		}
		content := strings.Join(lines, "\n")

		filename := filepath.Join(bs.testDataDir, fmt.Sprintf("large_%d.txt", i))
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return err
		}

		bs.largeFiles = append(bs.largeFiles, filename)
	}
	return nil
}

func (bs *BenchmarkSuite) createGoFiles() error {
	// Create Go files with realistic structure
	for i := 0; i < 30; i++ {
		content := fmt.Sprintf(`package main

import (
	"fmt"
	"context"
)

// TestStruct%d represents a test structure
type TestStruct%d struct {
	ID     int
	Name   string
	Active bool
}

// TestInterface%d defines test methods
type TestInterface%d interface {
	Process(ctx context.Context) error
	Validate() bool
}

// TestFunction%d performs test operations
func TestFunction%d(param string) (*TestStruct%d, error) {
	if param == "" {
		return nil, fmt.Errorf("empty parameter")
	}

	result := &TestStruct%d{
		ID:     %d,
		Name:   param,
		Active: true,
	}

	return result, nil
}

// Validate checks if the struct is valid
func (ts *TestStruct%d) Validate() bool {
	return ts.ID > 0 && ts.Name != ""
}

// Process implements TestInterface%d
func (ts *TestStruct%d) Process(ctx context.Context) error {
	return nil
}

// Constants and variables for testing
const TestConstant%d = %d
var TestVariable%d = %d
`, i, i, i, i, i, i, i, i, i, i, i, i, i, i, i, i)

		filename := filepath.Join(bs.testDataDir, fmt.Sprintf("test_%d.go", i))
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return err
		}

		bs.goFiles = append(bs.goFiles, filename)
	}
	return nil
}

// Benchmark functions
func BenchmarkEngineRegexSmallFiles(b *testing.B) {
	suite := NewBenchmarkSuite()
	if err := suite.Setup(); err != nil {
		b.Fatalf("Benchmark setup failed: %v", err)
	}
	defer suite.Cleanup()

	engine, err := NewEngine()
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	opts := &SearchOptions{
		Pattern:     "test",
		SearchPaths: []string{suite.testDataDir},
		SearchMode:  ModeRegex,
		MaxWorkers:  runtime.NumCPU(),
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

func BenchmarkEngineRegexLargeFiles(b *testing.B) {
	suite := NewBenchmarkSuite()
	if err := suite.Setup(); err != nil {
		b.Fatalf("Benchmark setup failed: %v", err)
	}
	defer suite.Cleanup()

	engine, err := NewEngine()
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// Search only in large files
	var largePaths []string
	for _, file := range suite.largeFiles {
		largePaths = append(largePaths, file)
	}

	opts := &SearchOptions{
		Pattern:     "Function",
		SearchPaths: largePaths,
		SearchMode:  ModeRegex,
		MaxWorkers:  runtime.NumCPU(),
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

func BenchmarkEngineSemanticSearch(b *testing.B) {
	suite := NewBenchmarkSuite()
	if err := suite.Setup(); err != nil {
		b.Fatalf("Benchmark setup failed: %v", err)
	}
	defer suite.Cleanup()

	engine, err := NewEngine()
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	opts := &SearchOptions{
		Pattern:     "TestFunction",
		SearchPaths: []string{suite.testDataDir},
		SearchMode:  ModeSemantic,
		FindDefs:    true,
		MaxWorkers:  runtime.NumCPU(),
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var output strings.Builder
		if err := engine.Search(ctx, opts, &output); err != nil {
			b.Fatalf("Semantic search failed: %v", err)
		}
	}
}

func BenchmarkEngineHybridSearch(b *testing.B) {
	suite := NewBenchmarkSuite()
	if err := suite.Setup(); err != nil {
		b.Fatalf("Benchmark setup failed: %v", err)
	}
	defer suite.Cleanup()

	engine, err := NewEngine()
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	opts := &SearchOptions{
		Pattern:     "TestStruct",
		SearchPaths: []string{suite.testDataDir},
		SearchMode:  ModeHybrid,
		MaxWorkers:  runtime.NumCPU(),
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var output strings.Builder
		if err := engine.Search(ctx, opts, &output); err != nil {
			b.Fatalf("Hybrid search failed: %v", err)
		}
	}
}

func BenchmarkRegexSearcherConcurrency(b *testing.B) {
	suite := NewBenchmarkSuite()
	if err := suite.Setup(); err != nil {
		b.Fatalf("Benchmark setup failed: %v", err)
	}
	defer suite.Cleanup()

	workerCounts := []int{1, 2, 4, 8, 16}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			searcher, err := NewRegexSearcher()
			if err != nil {
				b.Fatalf("Failed to create searcher: %v", err)
			}
			defer searcher.Close()

			opts := &SearchOptions{
				Pattern:     "function",
				SearchPaths: []string{suite.testDataDir},
				MaxWorkers:  workers,
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
						b.Fatalf("Search error: %v", err)
					}

					if results == nil {
						break
					}
				}
			}
		})
	}
}

func BenchmarkRegexPatternTypes(b *testing.B) {
	suite := NewBenchmarkSuite()
	if err := suite.Setup(); err != nil {
		b.Fatalf("Benchmark setup failed: %v", err)
	}
	defer suite.Cleanup()

	patterns := map[string]string{
		"literal":    "test",
		"simple":     "test.*content",
		"complex":    `\b(function|method|procedure)\s+\w+\s*\(`,
		"multiline":  "(?s)function.*{.*}",
		"lookahead":  `test(?=\s+content)`,
	}

	for name, pattern := range patterns {
		b.Run(name, func(b *testing.B) {
			searcher, err := NewRegexSearcher()
			if err != nil {
				b.Fatalf("Failed to create searcher: %v", err)
			}
			defer searcher.Close()

			opts := &SearchOptions{
				Pattern:     pattern,
				SearchPaths: []string{suite.testDataDir},
				Multiline:   strings.Contains(pattern, "(?s)"),
				DotMatchAll: strings.Contains(pattern, "(?s)"),
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
						b.Fatalf("Search error: %v", err)
					}

					if results == nil {
						break
					}
				}
			}
		})
	}
}

func BenchmarkSemanticSearcherIndexing(b *testing.B) {
	suite := NewBenchmarkSuite()
	if err := suite.Setup(); err != nil {
		b.Fatalf("Benchmark setup failed: %v", err)
	}
	defer suite.Cleanup()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		searcher, err := NewSemanticSearcher()
		if err != nil {
			b.Fatalf("Failed to create searcher: %v", err)
		}

		opts := &SearchOptions{
			SearchPaths: []string{suite.testDataDir},
		}

		ctx := context.Background()
		if err := searcher.indexSymbols(ctx, opts); err != nil {
			b.Fatalf("Indexing failed: %v", err)
		}

		searcher.Close()
	}
}

func BenchmarkMemoryUsage(b *testing.B) {
	suite := NewBenchmarkSuite()
	if err := suite.Setup(); err != nil {
		b.Fatalf("Benchmark setup failed: %v", err)
	}
	defer suite.Cleanup()

	var initialMem, peakMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)

	engine, err := NewEngine()
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	opts := &SearchOptions{
		Pattern:     "TestFunction",
		SearchPaths: []string{suite.testDataDir},
		SearchMode:  ModeHybrid,
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var output strings.Builder
		if err := engine.Search(ctx, opts, &output); err != nil {
			b.Fatalf("Search failed: %v", err)
		}

		runtime.ReadMemStats(&peakMem)
		if peakMem.HeapInuse > peakMem.HeapInuse {
			peakMem = peakMem
		}
	}

	memoryUsed := peakMem.HeapInuse - initialMem.HeapInuse
	b.Logf("Memory used: %d bytes (%.2f MB)", memoryUsed, float64(memoryUsed)/(1024*1024))
}

// Performance comparison benchmarks
func BenchmarkPerformanceComparison(b *testing.B) {
	suite := NewBenchmarkSuite()
	if err := suite.Setup(); err != nil {
		b.Fatalf("Benchmark setup failed: %v", err)
	}
	defer suite.Cleanup()

	// Test different search modes for the same pattern
	pattern := "TestFunction"

	searchModes := map[string]SearchMode{
		"regex":    ModeRegex,
		"semantic": ModeSemantic,
		"hybrid":   ModeHybrid,
	}

	for modeName, mode := range searchModes {
		b.Run(modeName, func(b *testing.B) {
			engine, err := NewEngine()
			if err != nil {
				b.Fatalf("Failed to create engine: %v", err)
			}
			defer engine.Close()

			opts := &SearchOptions{
				Pattern:     pattern,
				SearchPaths: []string{suite.testDataDir},
				SearchMode:  mode,
			}

			if mode == ModeSemantic {
				opts.FindDefs = true
			}

			ctx := context.Background()

			start := time.Now()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var output strings.Builder
				if err := engine.Search(ctx, opts, &output); err != nil {
					b.Fatalf("Search failed: %v", err)
				}
			}

			elapsed := time.Since(start)
			stats := engine.Stats()

			b.Logf("Mode: %s, Files: %d, Matches: %d, Time: %v",
				modeName, stats.FilesSearched, stats.TotalMatches, elapsed)
		})
	}
}

// Test helper to run all benchmarks with a single setup
func BenchmarkAll(b *testing.B) {
	suite := NewBenchmarkSuite()
	if err := suite.Setup(); err != nil {
		b.Fatalf("Benchmark setup failed: %v", err)
	}
	defer suite.Cleanup()

	b.Run("regex_small", func(b *testing.B) { BenchmarkEngineRegexSmallFiles(b) })
	b.Run("regex_large", func(b *testing.B) { BenchmarkEngineRegexLargeFiles(b) })
	b.Run("semantic", func(b *testing.B) { BenchmarkEngineSemanticSearch(b) })
	b.Run("hybrid", func(b *testing.B) { BenchmarkEngineHybridSearch(b) })
}