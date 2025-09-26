package walker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWalker_New(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   int
	}{
		{
			name:   "default config",
			config: nil,
			want:   1, // Should create walker
		},
		{
			name:   "custom config",
			config: &Config{MaxWorkers: 4, BufferSize: 500},
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			walker, err := New(tt.config)
			if err != nil {
				t.Errorf("New() error = %v", err)
				return
			}
			if walker == nil {
				t.Errorf("New() returned nil walker")
			}
		})
	}
}

func TestWalker_WalkSimple(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create test files
	testFiles := []string{
		"main.go",
		"utils.go",
		"README.md",
		"subdir/helper.go",
		"subdir/data.json",
		".hidden/secret.txt",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := WalkSimple(tmpDir)
	if err != nil {
		t.Fatalf("WalkSimple() error = %v", err)
	}

	// Should find at least the Go files
	foundFiles := make(map[string]bool)
	for _, result := range results {
		rel, _ := filepath.Rel(tmpDir, result.Path)
		foundFiles[rel] = true
	}

	expectedFiles := []string{"main.go", "utils.go", "README.md", "subdir/helper.go", "subdir/data.json"}
	for _, expected := range expectedFiles {
		if !foundFiles[expected] {
			t.Errorf("Expected file %s not found in results", expected)
		}
	}
}

func TestWalker_Walk_Context(t *testing.T) {
	tmpDir := t.TempDir()

	// Create many files to ensure we can test cancellation
	for i := 0; i < 100; i++ {
		path := filepath.Join(tmpDir, "file"+filepath.FromSlash(string(rune(i)))+".txt")
		os.WriteFile(path, []byte("content"), 0644)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	config := &Config{
		Context:    ctx,
		MaxWorkers: 1,
	}

	walker, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	results, err := walker.Walk(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Should respect context cancellation
	count := 0
	for range results {
		count++
		if count > 50 {
			t.Error("Context cancellation not respected")
			break
		}
	}
}

func TestWalker_Stats(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	testFiles := []string{
		"file1.go",
		"file2.txt",
		"dir1/file3.go",
		"dir2/file4.py",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	walker, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	results, err := walker.Walk(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Consume all results
	count := 0
	for range results {
		count++
	}

	stats := walker.Stats()
	if stats.FilesFound == 0 {
		t.Error("Stats should show files found")
	}
	if stats.DirsTraversed == 0 {
		t.Error("Stats should show directories traversed")
	}
	if stats.Duration == 0 {
		t.Error("Stats should show duration")
	}
}

func TestWalker_MaxDepth(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directory structure
	deepPath := filepath.Join(tmpDir, "a", "b", "c", "d", "deep.txt")
	if err := os.MkdirAll(filepath.Dir(deepPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deepPath, []byte("deep content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Also create a shallow file
	shallowPath := filepath.Join(tmpDir, "shallow.txt")
	if err := os.WriteFile(shallowPath, []byte("shallow content"), 0644); err != nil {
		t.Fatal(err)
	}

	config := &Config{
		MaxDepth: 2,
	}

	walker, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	results, err := walker.Walk(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	foundDeep := false
	foundShallow := false
	for result := range results {
		if result.Info != nil && !result.Info.IsDir() {
			rel, _ := filepath.Rel(tmpDir, result.Path)
			if rel == "shallow.txt" {
				foundShallow = true
			}
			if filepath.Base(result.Path) == "deep.txt" {
				foundDeep = true
			}
		}
	}

	if !foundShallow {
		t.Error("Should find shallow file within max depth")
	}
	if foundDeep {
		t.Error("Should not find deep file beyond max depth")
	}
}

func TestWalker_HiddenFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create hidden and visible files
	files := []string{
		"visible.txt",
		".hidden.txt",
		".hiddendir/file.txt",
	}

	for _, file := range files {
		fullPath := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("exclude hidden", func(t *testing.T) {
		config := &Config{HiddenFiles: false}
		walker, err := New(config)
		if err != nil {
			t.Fatal(err)
		}

		results, err := walker.Walk(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		foundHidden := false
		foundVisible := false
		for result := range results {
			if result.Info != nil && !result.Info.IsDir() {
				name := filepath.Base(result.Path)
				if name == ".hidden.txt" || name == "file.txt" {
					foundHidden = true
				}
				if name == "visible.txt" {
					foundVisible = true
				}
			}
		}

		if foundHidden {
			t.Error("Should not find hidden files when HiddenFiles=false")
		}
		if !foundVisible {
			t.Error("Should find visible files")
		}
	})

	t.Run("include hidden", func(t *testing.T) {
		config := &Config{HiddenFiles: true}
		walker, err := New(config)
		if err != nil {
			t.Fatal(err)
		}

		results, err := walker.Walk(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		foundHidden := false
		for result := range results {
			if result.Info != nil && !result.Info.IsDir() {
				name := filepath.Base(result.Path)
				if name == ".hidden.txt" {
					foundHidden = true
					break
				}
			}
		}

		if !foundHidden {
			t.Error("Should find hidden files when HiddenFiles=true")
		}
	})
}

func TestWalker_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "single.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	walker, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	results, err := walker.Walk(filePath)
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	var result Result
	for r := range results {
		result = r
		count++
	}

	if count != 1 {
		t.Errorf("Expected 1 result, got %d", count)
	}

	if result.Path != filePath {
		t.Errorf("Expected path %s, got %s", filePath, result.Path)
	}

	if result.Info.IsDir() {
		t.Error("Result should not be a directory")
	}
}

func TestWalker_NonexistentPath(t *testing.T) {
	walker, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	_, err = walker.Walk("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for nonexistent path")
	}
}

func TestIsHidden(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/path/to/.hidden", true},
		{"/path/to/visible", false},
		{"/.dotfile", true},
		{"regular", false},
		{".hiddenfile", true},
		{"file.txt", false},
		{"/path/.hidden/file.txt", true}, // Tests the base of the path
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isHidden(tt.path); got != tt.want {
				t.Errorf("isHidden(%s) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}