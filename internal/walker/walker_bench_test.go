package walker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkWalker_Walk(b *testing.B) {
	// Create a large directory structure for benchmarking
	tmpDir := b.TempDir()
	createLargeDirStructure(b, tmpDir, 100, 10, 5) // 100 dirs, 10 files per dir, 5 levels deep

	config := DefaultConfig()
	walker, err := New(config)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		results, err := walker.Walk(tmpDir)
		if err != nil {
			b.Fatal(err)
		}

		// Consume all results
		count := 0
		for range results {
			count++
		}
	}
}

func BenchmarkWalker_Workers(b *testing.B) {
	tmpDir := b.TempDir()
	createLargeDirStructure(b, tmpDir, 50, 20, 3)

	workerCounts := []int{1, 2, 4, 8, 16}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("workers-%d", workers), func(b *testing.B) {
			config := &Config{
				MaxWorkers: workers,
				BufferSize: 1000,
			}

			walker, err := New(config)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				results, err := walker.Walk(tmpDir)
				if err != nil {
					b.Fatal(err)
				}

				count := 0
				for range results {
					count++
				}
			}
		})
	}
}

func BenchmarkWalker_BufferSize(b *testing.B) {
	tmpDir := b.TempDir()
	createLargeDirStructure(b, tmpDir, 30, 15, 4)

	bufferSizes := []int{10, 100, 1000, 10000}

	for _, bufSize := range bufferSizes {
		b.Run(fmt.Sprintf("buffer-%d", bufSize), func(b *testing.B) {
			config := &Config{
				MaxWorkers: 4,
				BufferSize: bufSize,
			}

			walker, err := New(config)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				results, err := walker.Walk(tmpDir)
				if err != nil {
					b.Fatal(err)
				}

				count := 0
				for range results {
					count++
				}
			}
		})
	}
}

func BenchmarkWalker_WithFilters(b *testing.B) {
	tmpDir := b.TempDir()
	createMixedFileStructure(b, tmpDir)

	b.Run("no-filters", func(b *testing.B) {
		config := DefaultConfig()
		walker, err := New(config)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results, err := walker.Walk(tmpDir)
			if err != nil {
				b.Fatal(err)
			}

			count := 0
			for range results {
				count++
			}
		}
	})

	b.Run("with-filters", func(b *testing.B) {
		filters := CreateSourceCodeFilter()
		config := &Config{
			Filters: filters,
		}

		walker, err := New(config)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results, err := walker.Walk(tmpDir)
			if err != nil {
				b.Fatal(err)
			}

			count := 0
			for range results {
				count++
			}
		}
	})
}

func BenchmarkWalker_WithIgnores(b *testing.B) {
	tmpDir := b.TempDir()
	createMixedFileStructure(b, tmpDir)

	// Create .gitignore file
	gitignoreContent := `*.tmp
*.log
build/
node_modules/
target/
*.class
*.o
*.exe
`
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		b.Fatal(err)
	}

	b.Run("no-ignores", func(b *testing.B) {
		config := DefaultConfig()
		config.IgnoreRules = &IgnoreManager{enabled: false}

		walker, err := New(config)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results, err := walker.Walk(tmpDir)
			if err != nil {
				b.Fatal(err)
			}

			count := 0
			for range results {
				count++
			}
		}
	})

	b.Run("with-ignores", func(b *testing.B) {
		config := DefaultConfig()

		walker, err := New(config)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results, err := walker.Walk(tmpDir)
			if err != nil {
				b.Fatal(err)
			}

			count := 0
			for range results {
				count++
			}
		}
	})
}

func BenchmarkIgnoreManager_ShouldIgnore(b *testing.B) {
	im, err := NewIgnoreManager()
	if err != nil {
		b.Fatal(err)
	}

	// Add some common patterns
	patterns := []string{
		"*.tmp", "*.log", "*.bak",
		"build/", "dist/", "node_modules/",
		"target/", "*.class", "*.o",
		"**/*.test", "src/**/*.min.js",
	}

	for _, pattern := range patterns {
		if err := im.AddRule(pattern); err != nil {
			b.Fatal(err)
		}
	}

	testPaths := []string{
		"src/main.go",
		"build/output.txt",
		"temp.tmp",
		"deep/nested/path/file.js",
		"test/unit/helper.test",
		"node_modules/package/index.js",
		"src/components/Button.min.js",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, path := range testPaths {
			im.ShouldIgnore(path, false)
		}
	}
}

func BenchmarkFilters_DetectType(b *testing.B) {
	tmpDir := b.TempDir()

	// Create test files of different types
	testFiles := []struct {
		name    string
		content string
	}{
		{"main.go", "package main\nfunc main() {}"},
		{"script.py", "#!/usr/bin/python3\nprint('hello')"},
		{"index.js", "console.log('hello');"},
		{"styles.css", "body { color: red; }"},
		{"data.json", `{"key": "value"}`},
		{"README.md", "# Title\nContent"},
		{"Dockerfile", "FROM ubuntu:20.04"},
		{"main.c", "#include <stdio.h>\nint main() {}"},
		{"Component.tsx", "export const Component = () => {};"},
		{"config.yaml", "key: value"},
	}

	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file.name)
		if err := os.WriteFile(path, []byte(file.content), 0644); err != nil {
			b.Fatal(err)
		}
	}

	f := NewFilters()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, file := range testFiles {
			path := filepath.Join(tmpDir, file.name)
			info, _ := os.Stat(path)
			f.DetectType(path, info)
		}
	}
}

func BenchmarkFilters_ShouldInclude(b *testing.B) {
	tmpDir := b.TempDir()

	// Create test files
	testFiles := []string{
		"main.go", "script.py", "index.js", "styles.css",
		"data.json", "README.md", "test.tmp", "build/output",
		".hidden", "large-file.txt",
	}

	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			b.Fatal(err)
		}
	}

	f := CreateSourceCodeFilter()

	// Get file info for each test file
	fileInfos := make(map[string]os.FileInfo)
	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file)
		info, _ := os.Stat(path)
		fileInfos[file] = info
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, file := range testFiles {
			path := filepath.Join(tmpDir, file)
			f.ShouldInclude(path, fileInfos[file])
		}
	}
}

func BenchmarkWalker_Comparison(b *testing.B) {
	// Create a realistic project structure
	tmpDir := b.TempDir()
	createRealisticProject(b, tmpDir)

	b.Run("filepath.Walk", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			count := 0
			filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					count++
				}
				return nil
			})
		}
	})

	b.Run("walker", func(b *testing.B) {
		walker, err := New(DefaultConfig())
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results, err := walker.Walk(tmpDir)
			if err != nil {
				b.Fatal(err)
			}

			count := 0
			for range results {
				count++
			}
		}
	})
}

func BenchmarkWalker_MemoryUsage(b *testing.B) {
	tmpDir := b.TempDir()
	createLargeDirStructure(b, tmpDir, 200, 20, 4)

	config := &Config{
		MaxWorkers: 4,
		BufferSize: 100, // Small buffer to test memory efficiency
	}

	walker, err := New(config)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		results, err := walker.Walk(tmpDir)
		if err != nil {
			b.Fatal(err)
		}

		// Process results one by one to minimize memory usage
		count := 0
		for range results {
			count++
		}
	}
}

// Helper functions for benchmark setup

func createLargeDirStructure(b *testing.B, root string, dirs, filesPerDir, depth int) {
	if depth <= 0 {
		return
	}

	for i := 0; i < dirs; i++ {
		dirPath := filepath.Join(root, fmt.Sprintf("dir_%d", i))
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			b.Fatal(err)
		}

		// Create files in this directory
		for j := 0; j < filesPerDir; j++ {
			filePath := filepath.Join(dirPath, fmt.Sprintf("file_%d.txt", j))
			if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
				b.Fatal(err)
			}
		}

		// Recurse to create subdirectories
		createLargeDirStructure(b, dirPath, dirs/2, filesPerDir/2, depth-1)
	}
}

func createMixedFileStructure(b *testing.B, root string) {
	files := []struct {
		path    string
		content string
	}{
		{"src/main.go", "package main"},
		{"src/utils.go", "package main"},
		{"test/main_test.go", "package main"},
		{"build/output.exe", "binary"},
		{"build/temp.tmp", "temp"},
		{"docs/README.md", "# Project"},
		{"node_modules/pkg/index.js", "module.exports = {};"},
		{"target/classes/Main.class", "binary"},
		{".hidden/config", "config"},
		{"logs/error.log", "error message"},
		{"scripts/build.py", "#!/usr/bin/python3"},
		{"assets/style.css", "body {}"},
		{"config/app.json", `{"debug": true}`},
		{"data/sample.yaml", "key: value"},
	}

	for _, file := range files {
		fullPath := filepath.Join(root, file.path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(file.content), 0644); err != nil {
			b.Fatal(err)
		}
	}
}

func createRealisticProject(b *testing.B, root string) {
	// Simulate a realistic Go project with dependencies
	files := []struct {
		path    string
		content string
	}{
		// Main application
		{"cmd/app/main.go", "package main"},
		{"internal/server/server.go", "package server"},
		{"internal/db/db.go", "package db"},
		{"pkg/utils/utils.go", "package utils"},

		// Tests
		{"internal/server/server_test.go", "package server"},
		{"pkg/utils/utils_test.go", "package utils"},

		// Config files
		{"go.mod", "module example.com/app"},
		{"go.sum", "checksums"},
		{".gitignore", "*.tmp\nbuild/"},
		{"Dockerfile", "FROM golang:1.21"},
		{"docker-compose.yml", "version: '3'"},

		// Documentation
		{"README.md", "# Project"},
		{"docs/api.md", "# API"},

		// Build artifacts (should be ignored)
		{"build/app", "binary"},
		{"build/temp.tmp", "temp"},

		// Dependencies (large directory)
		{"vendor/github.com/pkg/errors/errors.go", "package errors"},
		{"vendor/github.com/gorilla/mux/mux.go", "package mux"},
	}

	for _, file := range files {
		fullPath := filepath.Join(root, file.path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(file.content), 0644); err != nil {
			b.Fatal(err)
		}
	}

	// Add many vendor files to simulate a real project
	for i := 0; i < 100; i++ {
		vendorFile := filepath.Join(root, "vendor", "github.com", "example", fmt.Sprintf("file%d.go", i))
		if err := os.MkdirAll(filepath.Dir(vendorFile), 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(vendorFile, []byte("package example"), 0644); err != nil {
			b.Fatal(err)
		}
	}
}