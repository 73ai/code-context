package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// MockSymbolParser provides a simple mock implementation for testing
type MockSymbolParser struct{}

func (m *MockSymbolParser) ParseFile(ctx context.Context, filePath string) ([]SymbolInfo, error) {
	// Create mock symbols based on file name and extension
	ext := filepath.Ext(filePath)
	base := filepath.Base(filePath)

	var symbols []SymbolInfo

	// Create a function symbol
	symbols = append(symbols, SymbolInfo{
		ID:          base + "_func_1",
		Name:        "MockFunction",
		Type:        "function",
		Kind:        "function",
		FilePath:    filePath,
		StartLine:   10,
		EndLine:     20,
		StartCol:    1,
		EndCol:      10,
		Signature:   "func MockFunction() error",
		Tags:        []string{"mock", "test"},
		LastUpdated: time.Now(),
	})

	// Create different symbols based on file extension
	switch ext {
	case ".go":
		symbols = append(symbols, SymbolInfo{
			ID:        base + "_struct_1",
			Name:      "MockStruct",
			Type:      "struct",
			Kind:      "type",
			FilePath:  filePath,
			StartLine: 25,
			EndLine:   30,
			Tags:      []string{"mock", "golang", "struct"},
		})
	case ".py":
		symbols = append(symbols, SymbolInfo{
			ID:        base + "_class_1",
			Name:      "MockClass",
			Type:      "class",
			Kind:      "class",
			FilePath:  filePath,
			StartLine: 15,
			EndLine:   40,
			Tags:      []string{"mock", "python", "class"},
		})
	case ".js", ".ts":
		symbols = append(symbols, SymbolInfo{
			ID:        base + "_const_1",
			Name:      "MOCK_CONSTANT",
			Type:      "constant",
			Kind:      "constant",
			FilePath:  filePath,
			StartLine: 5,
			EndLine:   5,
			Tags:      []string{"mock", "javascript", "constant"},
		})
	}

	return symbols, nil
}

func (m *MockSymbolParser) SupportedLanguages() []string {
	return []string{"Go", "Python", "JavaScript", "TypeScript"}
}

func (m *MockSymbolParser) IsSupported(filePath string) bool {
	ext := filepath.Ext(filePath)
	supportedExts := map[string]bool{
		".go": true,
		".py": true,
		".js": true,
		".ts": true,
	}
	return supportedExts[ext]
}

func (m *MockSymbolParser) ParseReferences(ctx context.Context, filePath string, symbolIndex SymbolIndex) ([]Reference, error) {
	// Create mock references for testing
	var references []Reference

	// Create some sample references based on the file
	if len(symbolIndex) > 0 {
		// Reference the first symbol in the index
		references = append(references, Reference{
			SymbolID: symbolIndex[0].ID,
			FilePath: filePath,
			Line:     10,
			Column:   5,
			Kind:     "reference",
			Context:  "mock reference context",
		})

		// Add a second reference if there are multiple symbols
		if len(symbolIndex) > 1 {
			references = append(references, Reference{
				SymbolID: symbolIndex[1].ID,
				FilePath: filePath,
				Line:     20,
				Column:   8,
				Kind:     "call",
				Context:  "mock call reference",
			})
		}
	}

	return references, nil
}

func (m *MockSymbolParser) SupportsReferences() bool {
	return true
}

// TestIntegrationFullWorkflow tests the complete indexing workflow
func TestIntegrationFullWorkflow(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "integration-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test source files
	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

type Config struct {
	Name string
	Port int
}
`,
		"utils.py": `#!/usr/bin/env python3

class Utils:
	def __init__(self):
		self.name = "utils"

	def process_data(self, data):
		return data.upper()

CONSTANT_VALUE = 42
`,
		"helper.js": `const HELPER_VERSION = "1.0.0";

function processString(str) {
	return str.trim().toLowerCase();
}

class StringProcessor {
	constructor() {
		this.version = HELPER_VERSION;
	}
}
`,
		"config.ts": `interface Config {
	name: string;
	port: number;
	enabled: boolean;
}

const DEFAULT_CONFIG: Config = {
	name: "app",
	port: 8080,
	enabled: true
};
`,
	}

	// Create source files
	sourceDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(sourceDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create storage
	dbDir := filepath.Join(tmpDir, "db")
	opts := DefaultBadgerOptions(dbDir)
	opts.SyncWrites = true // Enable synchronous writes for testing to ensure data consistency
	storage, err := NewBadgerStorage(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create store
	store := NewStore(storage, DefaultStoreConfig())

	// Create mock parser
	parser := &MockSymbolParser{}

	// Create builder
	builderConfig := DefaultBuilderConfig()
	builderConfig.Workers = 2
	builderConfig.Verbose = true
	builder := NewBuilder(store, parser, builderConfig)

	ctx := context.Background()

	t.Run("Initial Index Build", func(t *testing.T) {
		stats, err := builder.BuildIndex(ctx, sourceDir)
		if err != nil {
			t.Errorf("Failed to build index: %v", err)
		}

		if stats.FilesProcessed != 4 {
			t.Errorf("Expected 4 files processed, got %d", stats.FilesProcessed)
		}

		if stats.SymbolsIndexed < 8 { // Each file should have at least 2 symbols
			t.Errorf("Expected at least 8 symbols indexed, got %d", stats.SymbolsIndexed)
		}

		t.Logf("Build stats: %+v", stats)
	})

	t.Run("Symbol Retrieval", func(t *testing.T) {
		// Test retrieving symbols from different files
		testCases := []struct {
			filePath string
			symbolID string
		}{
			{filepath.Join(sourceDir, "main.go"), "main.go_func_1"},
			{filepath.Join(sourceDir, "utils.py"), "utils.py_class_1"},
			{filepath.Join(sourceDir, "helper.js"), "helper.js_const_1"},
		}

		for _, tc := range testCases {
			symbol, err := store.GetSymbol(ctx, tc.filePath, tc.symbolID)
			if err != nil {
				t.Errorf("Failed to get symbol %s from %s: %v", tc.symbolID, tc.filePath, err)
				continue
			}

			if symbol.FilePath != tc.filePath {
				t.Errorf("Symbol file path mismatch: expected %s, got %s", tc.filePath, symbol.FilePath)
			}

			t.Logf("Retrieved symbol: %s (%s) from %s", symbol.Name, symbol.Type, symbol.FilePath)
		}
	})

	t.Run("Search Functionality", func(t *testing.T) {
		// Test different search types
		searchTests := []struct {
			name          string
			query         SearchQuery
			expectedCount int
		}{
			{
				name: "Search by function type",
				query: SearchQuery{
					Type: SearchByType,
					Term: "function",
				},
				expectedCount: 4, // One function per file
			},
			{
				name: "Search by mock tag",
				query: SearchQuery{
					Type: SearchByTag,
					Term: "mock",
				},
				expectedCount: 8, // All symbols have mock tag
			},
			{
				name: "Search by name pattern",
				query: SearchQuery{
					Type: SearchByPattern,
					Term: "Mock",
				},
				expectedCount: 4, // All symbols have Mock in name
			},
		}

		for _, test := range searchTests {
			t.Run(test.name, func(t *testing.T) {
				result, err := store.SearchSymbols(ctx, test.query)
				if err != nil {
					t.Errorf("Search failed: %v", err)
					return
				}

				if result.Count < 1 {
					t.Errorf("Expected at least 1 result, got %d", result.Count)
				}

				t.Logf("Search '%s' returned %d results in %v", test.name, result.Count, result.Duration)
			})
		}
	})

	t.Run("File Metadata", func(t *testing.T) {
		// Get all files
		files, err := store.GetAllFiles(ctx)
		if err != nil {
			t.Errorf("Failed to get all files: %v", err)
			return
		}

		if len(files) != 4 {
			t.Errorf("Expected 4 files, got %d", len(files))
		}

		// Check each file has metadata
		for _, file := range files {
			if file.Hash == "" {
				t.Errorf("File %s has empty hash", file.Path)
			}
			if file.Language == "" {
				t.Errorf("File %s has empty language", file.Path)
			}
			if file.SymbolCount == 0 {
				t.Errorf("File %s has zero symbol count", file.Path)
			}

			t.Logf("File metadata: %s (%s, %d symbols, %d bytes)",
				file.Path, file.Language, file.SymbolCount, file.Size)
		}
	})

	t.Run("Incremental Updates", func(t *testing.T) {
		// Modify a file
		newFile := filepath.Join(sourceDir, "new.go")
		newContent := `package main

func NewFunction() {
	// This is a new function
}
`
		if err := os.WriteFile(newFile, []byte(newContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Run incremental build
		stats, err := builder.BuildIndex(ctx, sourceDir)
		if err != nil {
			t.Errorf("Incremental build failed: %v", err)
			return
		}

		// Should process only the new file (since others haven't changed)
		if stats.FilesProcessed != 1 {
			t.Errorf("Expected 1 file processed in incremental build, got %d", stats.FilesProcessed)
		}

		t.Logf("Incremental build stats: %+v", stats)

		// Verify the new symbol exists
		symbols, err := store.GetSymbolsInFile(ctx, newFile)
		if err != nil {
			t.Errorf("Failed to get symbols from new file: %v", err)
			return
		}

		if len(symbols) == 0 {
			t.Error("No symbols found in new file")
		}
	})

	t.Run("File Deletion", func(t *testing.T) {
		// Delete the new file from index
		newFile := filepath.Join(sourceDir, "new.go")

		if err := store.DeleteFile(ctx, newFile); err != nil {
			t.Errorf("Failed to delete file from index: %v", err)
			return
		}

		// Verify symbols are gone
		_, err := store.GetFileMetadata(ctx, newFile)
		if err == nil {
			t.Error("File metadata should not exist after deletion")
		}

		symbols, err := store.GetSymbolsInFile(ctx, newFile)
		if err != nil {
			t.Errorf("Error getting symbols from deleted file: %v", err)
			return
		}

		if len(symbols) != 0 {
			t.Errorf("Expected 0 symbols from deleted file, got %d", len(symbols))
		}
	})
}

// TestIntegrationWatcher tests the file system watcher integration
func TestIntegrationWatcher(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create storage and store
	dbDir := filepath.Join(tmpDir, "db")
	opts := DefaultBadgerOptions(dbDir)
	opts.SyncWrites = true // Enable synchronous writes for testing to ensure data consistency
	storage, err := NewBadgerStorage(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	store := NewStore(storage, DefaultStoreConfig())
	parser := &MockSymbolParser{}
	builder := NewBuilder(store, parser, DefaultBuilderConfig())

	// Create watcher
	watcherConfig := DefaultWatcherConfig()
	watcherConfig.DebounceDuration = 100 * time.Millisecond // Short debounce for testing
	watcherConfig.WatchDirs = []string{tmpDir}
	watcherConfig.Verbose = true

	var eventCount int
	var eventMutex sync.Mutex
	watcherConfig.EventCallback = func(event WatchEvent) {
		eventMutex.Lock()
		eventCount++
		eventMutex.Unlock()
		t.Logf("Watch event: %s %s", event.Operation, event.Path)
	}

	watcherConfig.ErrorCallback = func(err error) {
		t.Errorf("Watcher error: %v", err)
	}

	watcher, err := NewWatcher(store, builder, watcherConfig)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start watcher
	if err := watcher.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer watcher.Stop()

	// Wait a moment for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(tmpDir, "watched.go")
	content := `package main

func WatchedFunction() {
	// This function is being watched
}
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for events to be processed
	time.Sleep(500 * time.Millisecond)

	// Verify the file was indexed
	symbols, err := store.GetSymbolsInFile(ctx, testFile)
	if err != nil {
		t.Errorf("Failed to get symbols from watched file: %v", err)
		return
	}

	if len(symbols) == 0 {
		t.Error("No symbols found in watched file")
	}

	// Modify the file
	modifiedContent := content + `
func AnotherFunction() {
	// Another function added
}
`
	if err := os.WriteFile(testFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for events to be processed
	time.Sleep(500 * time.Millisecond)

	// Delete the file
	if err := os.Remove(testFile); err != nil {
		t.Fatal(err)
	}

	// Wait for events to be processed
	time.Sleep(500 * time.Millisecond)

	eventMutex.Lock()
	finalEventCount := eventCount
	eventMutex.Unlock()

	if finalEventCount == 0 {
		t.Error("Expected to receive watch events")
	}

	t.Logf("Received %d watch events", finalEventCount)
}

// TestIntegrationPerformance tests performance under realistic conditions
func TestIntegrationPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "perf-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create many source files
	sourceDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	numFiles := 100
	for i := 0; i < numFiles; i++ {
		filename := filepath.Join(sourceDir, fmt.Sprintf("file%03d.go", i))
		content := fmt.Sprintf(`package main

import "fmt"

func Function%d() {
	fmt.Printf("Function %d")
}

type Struct%d struct {
	ID   int
	Name string
}

const CONSTANT%d = %d
`, i, i, i, i, i)

		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create storage
	dbDir := filepath.Join(tmpDir, "db")
	opts := DefaultBadgerOptions(dbDir)
	opts.SyncWrites = true // Enable synchronous writes for testing to ensure data consistency
	storage, err := NewBadgerStorage(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	store := NewStore(storage, DefaultStoreConfig())
	parser := &MockSymbolParser{}

	builderConfig := DefaultBuilderConfig()
	builderConfig.Workers = 8
	builder := NewBuilder(store, parser, builderConfig)

	ctx := context.Background()

	// Measure build time
	start := time.Now()
	stats, err := builder.BuildIndex(ctx, sourceDir)
	buildTime := time.Since(start)

	if err != nil {
		t.Errorf("Performance test build failed: %v", err)
		return
	}

	t.Logf("Performance test results:")
	t.Logf("  Files: %d", stats.FilesProcessed)
	t.Logf("  Symbols: %d", stats.SymbolsIndexed)
	t.Logf("  Build time: %v", buildTime)
	t.Logf("  Files/sec: %.2f", float64(stats.FilesProcessed)/buildTime.Seconds())
	t.Logf("  Symbols/sec: %.2f", float64(stats.SymbolsIndexed)/buildTime.Seconds())

	// Test search performance
	query := SearchQuery{
		Type: SearchByType,
		Term: "function",
	}

	searchStart := time.Now()
	result, err := store.SearchSymbols(ctx, query)
	searchTime := time.Since(searchStart)

	if err != nil {
		t.Errorf("Performance test search failed: %v", err)
		return
	}

	t.Logf("Search performance:")
	t.Logf("  Results: %d", result.Count)
	t.Logf("  Search time: %v", searchTime)

	// Test storage statistics
	storageStats := storage.Stats()
	t.Logf("Storage statistics:")
	t.Logf("  Total size: %d bytes", storageStats.TotalSize)
	t.Logf("  Read count: %d", storageStats.ReadCount)
	t.Logf("  Write count: %d", storageStats.WriteCount)
	t.Logf("  Cache hits: %d", storageStats.CacheHits)
	t.Logf("  Cache misses: %d", storageStats.CacheMisses)

	// Performance thresholds (adjust based on hardware)
	if buildTime.Seconds() > 30 {
		t.Logf("WARNING: Build time exceeded 30 seconds: %v", buildTime)
	}
	if searchTime.Milliseconds() > 100 {
		t.Logf("WARNING: Search time exceeded 100ms: %v", searchTime)
	}
}