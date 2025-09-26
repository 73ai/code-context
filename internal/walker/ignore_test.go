package walker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreManager_New(t *testing.T) {
	im, err := NewIgnoreManager()
	if err != nil {
		t.Errorf("NewIgnoreManager() error = %v", err)
	}
	if im == nil {
		t.Error("NewIgnoreManager() returned nil")
	}
}

func TestIgnoreManager_ParseRule(t *testing.T) {
	im, _ := NewIgnoreManager()

	tests := []struct {
		pattern  string
		wantErr  bool
		negate   bool
		dirOnly  bool
		anchored bool
	}{
		{"*.txt", false, false, false, false},
		{"!important.txt", false, true, false, false},
		{"temp/", false, false, true, false},
		{"/root/file", false, false, false, true},
		{"dir/subdir/", false, false, true, true},
		{"**/*.go", false, false, false, false},
		{"#comment", true, false, false, false}, // This should be filtered out before parseRule
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			rule, err := im.parseRule(tt.pattern, "/test", 1, "test.gitignore")
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if rule.Negate != tt.negate {
				t.Errorf("parseRule() negate = %v, want %v", rule.Negate, tt.negate)
			}
			if rule.DirOnly != tt.dirOnly {
				t.Errorf("parseRule() dirOnly = %v, want %v", rule.DirOnly, tt.dirOnly)
			}
			if rule.Anchored != tt.anchored {
				t.Errorf("parseRule() anchored = %v, want %v", rule.Anchored, tt.anchored)
			}
		})
	}
}

func TestIgnoreManager_PatternToRegex(t *testing.T) {
	im, _ := NewIgnoreManager()

	tests := []struct {
		pattern string
		path    string
		isDir   bool
		want    bool
	}{
		{"*.txt", "file.txt", false, true},
		{"*.txt", "file.go", false, false},
		{"temp", "temp", true, true},
		{"temp", "temp/file.txt", false, true},
		{"temp/", "temp", true, true},
		{"**/*.go", "src/main.go", false, true},
		{"**/*.go", "deep/nested/file.go", false, true},
		{"src/**", "src/file.txt", false, true},
		{"src/**", "src/deep/file.txt", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			rule, err := im.parseRule(tt.pattern, "/test", 1, "test")
			if err != nil {
				t.Fatal(err)
			}

			got := im.ruleMatches(rule, tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("ruleMatches(%s, %s, %v) = %v, want %v", tt.pattern, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestIgnoreManager_ShouldIgnore(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a .gitignore file
	gitignoreContent := `# Test gitignore
*.tmp
build/
!important.tmp
src/**/*.test
`
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		t.Fatal(err)
	}

	im, err := NewIgnoreManager()
	if err != nil {
		t.Fatal(err)
	}

	if err := im.LoadFromPath(tmpDir); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path    string
		isDir   bool
		ignored bool
	}{
		{"file.tmp", false, true},
		{"important.tmp", false, false}, // Negated
		{"build", true, true},
		{"build/output", false, true},
		{"src", true, false},
		{"src/file.go", false, false},
		{"src/deep/file.test", false, true},
		{"regular.txt", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := im.ShouldIgnore(tt.path, tt.isDir)
			if got != tt.ignored {
				t.Errorf("ShouldIgnore(%s, %v) = %v, want %v", tt.path, tt.isDir, got, tt.ignored)
			}
		})
	}
}

func TestIgnoreManager_LoadFromPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directory structure with multiple .gitignore files
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Root .gitignore
	rootGitignore := filepath.Join(tmpDir, ".gitignore")
	if err := os.WriteFile(rootGitignore, []byte("*.tmp\nbuild/"), 0644); err != nil {
		t.Fatal(err)
	}

	// Subdirectory .gitignore
	subGitignore := filepath.Join(subDir, ".gitignore")
	if err := os.WriteFile(subGitignore, []byte("*.log\n!important.log"), 0644); err != nil {
		t.Fatal(err)
	}

	// .rgignore file
	rgignorePath := filepath.Join(tmpDir, ".rgignore")
	if err := os.WriteFile(rgignorePath, []byte("*.bak"), 0644); err != nil {
		t.Fatal(err)
	}

	im, err := NewIgnoreManager()
	if err != nil {
		t.Fatal(err)
	}

	if err := im.LoadFromPath(subDir); err != nil {
		t.Fatal(err)
	}

	stats := im.GetStats()
	totalFiles := stats["total_files"].(int)
	if totalFiles < 2 {
		t.Errorf("Expected at least 2 ignore files loaded, got %d", totalFiles)
	}

	// Test that rules from both files are active
	if !im.ShouldIgnore("test.tmp", false) {
		t.Error("Rule from root .gitignore should be active")
	}
	if !im.ShouldIgnore("test.log", false) {
		t.Error("Rule from subdirectory .gitignore should be active")
	}
	if !im.ShouldIgnore("test.bak", false) {
		t.Error("Rule from .rgignore should be active")
	}
}

func TestIgnoreManager_AddRule(t *testing.T) {
	im, err := NewIgnoreManager()
	if err != nil {
		t.Fatal(err)
	}

	if err := im.AddRule("*.custom"); err != nil {
		t.Fatal(err)
	}

	if !im.ShouldIgnore("test.custom", false) {
		t.Error("Custom rule should ignore *.custom files")
	}
}

func TestIgnoreManager_Cache(t *testing.T) {
	im, err := NewIgnoreManager()
	if err != nil {
		t.Fatal(err)
	}

	if err := im.AddRule("*.test"); err != nil {
		t.Fatal(err)
	}

	// First call should compute and cache
	result1 := im.ShouldIgnore("file.test", false)
	if !result1 {
		t.Error("Should ignore .test files")
	}

	// Second call should use cache
	result2 := im.ShouldIgnore("file.test", false)
	if result2 != result1 {
		t.Error("Cache should return same result")
	}

	// Clear cache and verify it still works
	im.ClearCache()
	result3 := im.ShouldIgnore("file.test", false)
	if result3 != result1 {
		t.Error("Should work after cache clear")
	}
}

func TestIgnoreManager_SetEnabled(t *testing.T) {
	im, err := NewIgnoreManager()
	if err != nil {
		t.Fatal(err)
	}

	if err := im.AddRule("*.ignore"); err != nil {
		t.Fatal(err)
	}

	// Should ignore when enabled
	if !im.ShouldIgnore("test.ignore", false) {
		t.Error("Should ignore when enabled")
	}

	// Should not ignore when disabled
	im.SetEnabled(false)
	if im.ShouldIgnore("test.ignore", false) {
		t.Error("Should not ignore when disabled")
	}

	// Re-enable
	im.SetEnabled(true)
	if !im.ShouldIgnore("test.ignore", false) {
		t.Error("Should ignore when re-enabled")
	}
}

func TestIgnoreManager_AddCommonPatterns(t *testing.T) {
	im, err := NewIgnoreManager()
	if err != nil {
		t.Fatal(err)
	}

	if err := im.AddCommonPatterns("go"); err != nil {
		t.Fatal(err)
	}

	// Test that Go patterns are active
	if !im.ShouldIgnore("test.exe", false) {
		t.Error("Should ignore .exe files with Go patterns")
	}
	if !im.ShouldIgnore("vendor", true) {
		t.Error("Should ignore vendor/ directory with Go patterns")
	}

	// Test unknown language
	if err := im.AddCommonPatterns("unknown"); err == nil {
		t.Error("Should return error for unknown language")
	}
}

func TestIgnoreManager_RuleMatching(t *testing.T) {
	im, err := NewIgnoreManager()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		pattern string
		path    string
		isDir   bool
		want    bool
	}{
		// Basic patterns
		{"*.txt", "file.txt", false, true},
		{"*.txt", "file.go", false, false},
		{"temp", "temp", false, true},
		{"temp", "temp", true, true},

		// Directory-only patterns
		{"temp/", "temp", true, true},
		{"temp/", "temp", false, false},

		// Anchored patterns
		{"/root", "root", false, true},
		{"/root", "subdir/root", false, false},
		{"dir/file", "dir/file", false, true},
		{"dir/file", "other/dir/file", false, false},

		// Wildcard patterns
		{"**/*.go", "main.go", false, true},
		{"**/*.go", "src/main.go", false, true},
		{"**/*.go", "deep/nested/src/main.go", false, true},
		{"src/**", "src/file.txt", false, true},
		{"src/**", "src/deep/file.txt", false, true},

		// Character classes
		{"*.[ch]", "file.c", false, true},
		{"*.[ch]", "file.h", false, true},
		{"*.[ch]", "file.go", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			rule, err := im.parseRule(tt.pattern, "/test", 1, "test")
			if err != nil {
				t.Fatal(err)
			}

			got := im.ruleMatches(rule, tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("ruleMatches(%s, %s, %v) = %v, want %v", tt.pattern, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestIgnoreManager_Negation(t *testing.T) {
	tmpDir := t.TempDir()

	gitignoreContent := `*.tmp
!important.tmp
build/
!build/keep/
!build/keep/**
`
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		t.Fatal(err)
	}

	im, err := NewIgnoreManager()
	if err != nil {
		t.Fatal(err)
	}

	if err := im.LoadFromPath(tmpDir); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path    string
		isDir   bool
		ignored bool
	}{
		{"temp.tmp", false, true},
		{"important.tmp", false, false}, // Negated
		{"build", true, true},
		{"build/file.txt", false, true},
		{"build/keep", true, false}, // Negated
		{"build/keep/file.txt", false, false}, // Negated by **
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := im.ShouldIgnore(tt.path, tt.isDir)
			if got != tt.ignored {
				t.Errorf("ShouldIgnore(%s, %v) = %v, want %v", tt.path, tt.isDir, got, tt.ignored)
			}
		})
	}
}