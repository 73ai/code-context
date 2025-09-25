package walker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkWalkSimple(b *testing.B) {
	// Create a test directory structure
	tmpDir := b.TempDir()

	// Create a modest directory structure for consistent benchmarking
	for i := 0; i < 10; i++ {
		dirPath := filepath.Join(tmpDir, fmt.Sprintf("dir_%d", i))
		os.MkdirAll(dirPath, 0755)

		for j := 0; j < 10; j++ {
			filePath := filepath.Join(dirPath, fmt.Sprintf("file_%d.txt", j))
			os.WriteFile(filePath, []byte("test content"), 0644)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		results, err := WalkSimple(tmpDir)
		if err != nil {
			b.Fatal(err)
		}
		// Ensure results are consumed
		_ = len(results)
	}
}

func BenchmarkFilters_DetectLanguage(b *testing.B) {
	tmpDir := b.TempDir()

	// Create files of different types
	testFiles := []string{
		"main.go", "script.py", "index.js", "styles.css",
		"README.md", "Dockerfile", "config.json",
	}

	for _, filename := range testFiles {
		path := filepath.Join(tmpDir, filename)
		os.WriteFile(path, []byte("test content"), 0644)
	}

	f := NewFilters()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, filename := range testFiles {
			path := filepath.Join(tmpDir, filename)
			info, _ := os.Stat(path)
			f.DetectType(path, info)
		}
	}
}