package walker

import (
	"fmt"
	"os"
	"path/filepath"
)

func ExampleWalker_Walk() {
	// Create a temporary directory for demonstration
	tmpDir, err := os.MkdirTemp("", "walker_example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some test files
	testFiles := []string{
		"main.go",
		"utils.go",
		"test/main_test.go",
		"docs/README.md",
		".gitignore",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(fullPath, []byte("example content"), 0644); err != nil {
			panic(err)
		}
	}

	// Create walker with default configuration
	walker, err := New(DefaultConfig())
	if err != nil {
		panic(err)
	}

	// Walk the directory
	results, err := walker.Walk(tmpDir)
	if err != nil {
		panic(err)
	}

	// Count files found
	count := 0
	for result := range results {
		if result.Error == nil && result.Info != nil && !result.Info.IsDir() {
			count++
		}
	}

	// Get statistics
	stats := walker.Stats()

	fmt.Printf("Files found: %d\n", stats.FilesFound)
	fmt.Printf("Directories traversed: %d\n", stats.DirsTraversed)
	fmt.Printf("Files filtered: %d\n", stats.FilesFiltered)
	fmt.Printf("Duration: %v\n", stats.Duration > 0)

	// Output:
	// Files found: 4
	// Directories traversed: 3
	// Files filtered: 1
	// Duration: true
}