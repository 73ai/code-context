package walker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilters_New(t *testing.T) {
	f := NewFilters()
	if f == nil {
		t.Error("NewFilters() returned nil")
	}

	languages := f.GetSupportedLanguages()
	if len(languages) == 0 {
		t.Error("Should have default languages loaded")
	}

	// Check that some common languages are present
	found := make(map[string]bool)
	for _, lang := range languages {
		found[lang] = true
	}

	expectedLangs := []string{"go", "javascript", "python", "java"}
	for _, expected := range expectedLangs {
		if !found[expected] {
			t.Errorf("Expected language %s not found", expected)
		}
	}
}

func TestFilters_DetectByExtension(t *testing.T) {
	f := NewFilters()

	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", "Go"},
		{"script.py", "Python"},
		{"index.js", "JavaScript"},
		{"component.tsx", "TypeScript"},
		{"styles.css", "CSS"},
		{"data.json", "JSON"},
		{"README.md", "Markdown"},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			lang := f.detectByExtension(tt.path)
			if tt.expected == "" {
				if lang != nil {
					t.Errorf("Expected no language for %s, got %s", tt.path, lang.Name)
				}
			} else {
				if lang == nil {
					t.Errorf("Expected language %s for %s, got nil", tt.expected, tt.path)
				} else if lang.Name != tt.expected {
					t.Errorf("Expected language %s for %s, got %s", tt.expected, tt.path, lang.Name)
				}
			}
		})
	}
}

func TestFilters_DetectByPattern(t *testing.T) {
	f := NewFilters()

	tests := []struct {
		path     string
		expected string
	}{
		{"Makefile", "Makefile"},
		{"Dockerfile", "Docker"},
		{"go.mod", "Go"},
		{"package.json", "JSON"},
		{"Cargo.toml", "Rust"},
		{"requirements.txt", "Python"},
		{"build.gradle", "Java"},
		{"unknown-file", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			lang := f.detectByPattern(tt.path)
			if tt.expected == "" {
				if lang != nil {
					t.Errorf("Expected no language for %s, got %s", tt.path, lang.Name)
				}
			} else {
				if lang == nil {
					t.Errorf("Expected language %s for %s, got nil", tt.expected, tt.path)
				} else if lang.Name != tt.expected {
					t.Errorf("Expected language %s for %s, got %s", tt.expected, tt.path, lang.Name)
				}
			}
		})
	}
}

func TestFilters_DetectByShebang(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		shebang  string
		expected string
	}{
		{"python_script", "#!/usr/bin/env python3", "Python"},
		{"bash_script", "#!/bin/bash", "Shell"},
		{"node_script", "#!/usr/bin/env node", "JavaScript"},
		{"ruby_script", "#!/usr/bin/ruby", "Ruby"},
		{"php_script", "#!/usr/bin/php", "PHP"},
		{"no_shebang", "echo 'no shebang'", ""},
	}

	f := NewFilters()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, tt.name)
			content := tt.shebang + "\necho 'test'"
			if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
				t.Fatal(err)
			}

			lang := f.detectByShebang(scriptPath)
			if tt.expected == "" {
				if lang != nil {
					t.Errorf("Expected no language for %s, got %s", tt.name, lang.Name)
				}
			} else {
				if lang == nil {
					t.Errorf("Expected language %s for %s, got nil", tt.expected, tt.name)
				} else if lang.Name != tt.expected {
					t.Errorf("Expected language %s for %s, got %s", tt.expected, tt.name, lang.Name)
				}
			}
		})
	}
}

func TestFilters_IsBinaryFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create text file
	textPath := filepath.Join(tmpDir, "text.txt")
	if err := os.WriteFile(textPath, []byte("Hello, world!\nThis is a text file."), 0644); err != nil {
		t.Fatal(err)
	}

	// Create binary file (with null bytes)
	binaryPath := filepath.Join(tmpDir, "binary.bin")
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x00}
	if err := os.WriteFile(binaryPath, binaryContent, 0644); err != nil {
		t.Fatal(err)
	}

	f := NewFilters()

	if f.isBinaryFile(textPath) {
		t.Error("Text file should not be detected as binary")
	}

	if !f.isBinaryFile(binaryPath) {
		t.Error("Binary file should be detected as binary")
	}
}

func TestFilters_ShouldInclude(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := []struct {
		name    string
		content string
		size    int64
	}{
		{"small.txt", "small", 5},
		{"large.txt", "large content that exceeds limits", 32},
		{".hidden", "hidden file", 11},
		{"script.py", "#!/usr/bin/python\nprint('hello')", 30},
	}

	for _, file := range files {
		path := filepath.Join(tmpDir, file.name)
		if err := os.WriteFile(path, []byte(file.content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("size limits", func(t *testing.T) {
		f := NewFilters()
		f.SetSizeRange(10, 25) // Min 10 bytes, max 25 bytes
		f.SetAllowHidden(true)  // Allow hidden files for size testing

		smallInfo, _ := os.Stat(filepath.Join(tmpDir, "small.txt"))
		largeInfo, _ := os.Stat(filepath.Join(tmpDir, "large.txt"))
		hiddenInfo, _ := os.Stat(filepath.Join(tmpDir, ".hidden"))

		if f.ShouldInclude("small.txt", smallInfo) {
			t.Error("Should exclude file smaller than minimum size")
		}

		if f.ShouldInclude("large.txt", largeInfo) {
			t.Error("Should exclude file larger than maximum size")
		}

		if !f.ShouldInclude(".hidden", hiddenInfo) {
			t.Error("Should include file within size range")
		}
	})

	t.Run("hidden files", func(t *testing.T) {
		f := NewFilters()
		hiddenInfo, _ := os.Stat(filepath.Join(tmpDir, ".hidden"))
		normalInfo, _ := os.Stat(filepath.Join(tmpDir, "small.txt"))

		// Test with hidden files disabled
		f.SetAllowHidden(false)
		if f.ShouldInclude(".hidden", hiddenInfo) {
			t.Error("Should exclude hidden files when disabled")
		}
		if !f.ShouldInclude("small.txt", normalInfo) {
			t.Error("Should include normal files")
		}

		// Test with hidden files enabled
		f.SetAllowHidden(true)
		if !f.ShouldInclude(".hidden", hiddenInfo) {
			t.Error("Should include hidden files when enabled")
		}
	})

	t.Run("extension filters", func(t *testing.T) {
		f := NewFilters()
		txtInfo, _ := os.Stat(filepath.Join(tmpDir, "small.txt"))
		pyInfo, _ := os.Stat(filepath.Join(tmpDir, "script.py"))

		// Include only .py files
		f.IncludeExtension(".py")
		if f.ShouldInclude("small.txt", txtInfo) {
			t.Error("Should exclude .txt when only .py is included")
		}
		if !f.ShouldInclude("script.py", pyInfo) {
			t.Error("Should include .py files")
		}
	})

	t.Run("type filters", func(t *testing.T) {
		f := NewFilters()
		pyInfo, _ := os.Stat(filepath.Join(tmpDir, "script.py"))

		// Include only Python files
		f.IncludeType("Python")
		if !f.ShouldInclude("script.py", pyInfo) {
			t.Error("Should include Python files")
		}

		// Exclude Python files
		f = NewFilters()
		f.ExcludeType("Python")
		if f.ShouldInclude("script.py", pyInfo) {
			t.Error("Should exclude Python files when excluded")
		}
	})
}

func TestFilters_DetectType(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := []struct {
		name     string
		content  string
		expected string
	}{
		{"main.go", "package main\nfunc main() {}", "Go"},
		{"script.py", "#!/usr/bin/python\nprint('hello')", "Python"},
		{"styles.css", "body { color: red; }", "CSS"},
		{"data.json", `{"key": "value"}`, "JSON"},
		{"README.md", "# Title\nContent", "Markdown"},
	}

	f := NewFilters()

	for _, file := range files {
		t.Run(file.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, file.name)
			if err := os.WriteFile(path, []byte(file.content), 0644); err != nil {
				t.Fatal(err)
			}

			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}

			fileType := f.DetectType(path, info)
			if fileType.Language == nil {
				t.Errorf("Expected language %s for %s, got nil", file.expected, file.name)
			} else if fileType.Language.Name != file.expected {
				t.Errorf("Expected language %s for %s, got %s", file.expected, file.name, fileType.Language.Name)
			}

			if fileType.IsBinary {
				t.Errorf("File %s should not be detected as binary", file.name)
			}

			if !fileType.IsText {
				t.Errorf("File %s should be detected as text", file.name)
			}

			if fileType.Confidence == 0.0 {
				t.Errorf("Detection confidence should be greater than 0 for %s", file.name)
			}
		})
	}
}

func TestFilters_CustomPatterns(t *testing.T) {
	f := NewFilters()

	// Add custom pattern
	if err := f.AddCustomPattern(`.*_test\.go$`); err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "main_test.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if !f.ShouldInclude(testFile, info) {
		t.Error("Custom pattern should match test files")
	}
}

func TestFilters_GetLanguageExtensions(t *testing.T) {
	f := NewFilters()

	// Test known language
	goExts := f.GetLanguageExtensions("go")
	if len(goExts) == 0 {
		t.Error("Go should have extensions")
	}

	found := false
	for _, ext := range goExts {
		if ext == ".go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Go extensions should include .go")
	}

	// Test unknown language
	unknownExts := f.GetLanguageExtensions("unknown")
	if unknownExts != nil {
		t.Error("Unknown language should return nil extensions")
	}
}

func TestFilters_AddLanguage(t *testing.T) {
	f := NewFilters()

	customLang := &Language{
		Name:       "TestLang",
		Extensions: []string{".test", ".tst"},
		Patterns:   []string{"test.config"},
		MimeTypes:  []string{"text/x-test"},
	}

	f.AddLanguage(customLang)

	// Test extension detection
	if lang := f.detectByExtension("file.test"); lang == nil || lang.Name != "TestLang" {
		t.Error("Should detect custom language by extension")
	}

	// Test pattern detection
	if lang := f.detectByPattern("test.config"); lang == nil || lang.Name != "TestLang" {
		t.Error("Should detect custom language by pattern")
	}

	// Check in supported languages
	languages := f.GetSupportedLanguages()
	found := false
	for _, lang := range languages {
		if lang == "testlang" { // Should be lowercase
			found = true
			break
		}
	}
	if !found {
		t.Error("Custom language should be in supported languages list")
	}
}

func TestCreateSourceCodeFilter(t *testing.T) {
	f := CreateSourceCodeFilter()

	tmpDir := t.TempDir()

	// Test source code file
	goFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(goFile, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test binary file
	exeFile := filepath.Join(tmpDir, "program.exe")
	if err := os.WriteFile(exeFile, []byte{0x4D, 0x5A}, 0644); err != nil {
		t.Fatal(err)
	}

	// Test hidden file
	hiddenFile := filepath.Join(tmpDir, ".hidden.go")
	if err := os.WriteFile(hiddenFile, []byte("package hidden"), 0644); err != nil {
		t.Fatal(err)
	}

	goInfo, _ := os.Stat(goFile)
	exeInfo, _ := os.Stat(exeFile)
	hiddenInfo, _ := os.Stat(hiddenFile)

	if !f.ShouldInclude(goFile, goInfo) {
		t.Error("Source code filter should include .go files")
	}

	if f.ShouldInclude(exeFile, exeInfo) {
		t.Error("Source code filter should exclude .exe files")
	}

	if f.ShouldInclude(hiddenFile, hiddenInfo) {
		t.Error("Source code filter should exclude hidden files")
	}
}

func TestCreateTextFileFilter(t *testing.T) {
	f := CreateTextFileFilter()

	if !f.binaryDetection {
		t.Error("Text file filter should enable binary detection")
	}
}

func TestIsBinaryContent(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{"text content", []byte("Hello, world!"), false},
		{"null bytes", []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x00}, true},
		{"mostly printable", []byte("Hello\nWorld\t!"), false},
		{"many non-printable", []byte{0x01, 0x02, 0x03, 0x04, 0x05}, true},
		{"empty", []byte{}, false},
		{"mixed content", []byte("Text\x01\x02\x03more text"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBinaryContent(tt.content)
			if got != tt.expected {
				t.Errorf("isBinaryContent() = %v, want %v", got, tt.expected)
			}
		})
	}
}