package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestTreeSitterEndToEnd tests the complete tree-sitter workflow
func TestTreeSitterEndToEnd(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "treesitter_e2e")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files for different languages
	testFiles := map[string]string{
		"main.go": `package main

type User struct {
	Name string
	Age  int
}

func Hello(name string) string {
	return "Hello, " + name
}

var GlobalVar = "test"
const MaxUsers = 100
`,
		"script.py": `class User:
	def __init__(self, name, age):
		self.name = name
		self.age = age

def hello(name):
	return f"Hello, {name}"

global_var = "test"
`,
		"app.js": `class User {
	constructor(name, age) {
		this.name = name;
		this.age = age;
	}
}

function hello(name) {
	return "Hello, " + name;
}

const globalVar = "test";
`,
		"lib.rs": `struct User {
	name: String,
	age: u32,
}

fn hello(name: &str) -> String {
	format!("Hello, {}", name)
}

const MAX_USERS: i32 = 100;
`,
	}

	// Write test files
	for filename, content := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", filename, err)
		}
	}

	// Initialize language registry and symbol extractor
	registry, err := NewLanguageRegistry()
	if err != nil {
		t.Fatalf("Failed to create language registry: %v", err)
	}
	defer registry.Close()

	extractor := NewSymbolExtractor(registry)

	// Extract symbols from the directory
	ctx := context.Background()
	symbolIndex, err := extractor.ExtractSymbolsFromDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	// Verify that symbols were extracted
	stats := symbolIndex.GetStats()
	t.Logf("Extracted symbols: %+v", stats)

	totalSymbols := stats["total_symbols"].(int)
	if totalSymbols == 0 {
		t.Fatal("No symbols were extracted")
	}

	totalFiles := stats["total_files"].(int)
	if totalFiles != len(testFiles) {
		t.Errorf("Expected %d files, got %d", len(testFiles), totalFiles)
	}

	// Test symbol retrieval by type
	functions := symbolIndex.GetSymbolsByKind(SymbolFunction)
	t.Logf("Found %d functions", len(functions))
	if len(functions) == 0 {
		t.Error("Expected to find function symbols")
	}

	classes := symbolIndex.GetSymbolsByKind(SymbolClass)
	t.Logf("Found %d classes", len(classes))

	structs := symbolIndex.GetSymbolsByKind(SymbolStruct)
	t.Logf("Found %d structs", len(structs))

	variables := symbolIndex.GetSymbolsByKind(SymbolVariable)
	t.Logf("Found %d variables", len(variables))

	constants := symbolIndex.GetSymbolsByKind(SymbolConstant)
	t.Logf("Found %d constants", len(constants))

	// Test symbol retrieval by name
	userSymbols := symbolIndex.GetSymbolsByName("User")
	t.Logf("Found %d 'User' symbols", len(userSymbols))

	if len(userSymbols) == 0 {
		t.Error("Expected to find User symbols")
	}

	helloSymbols := symbolIndex.GetSymbolsByName("hello")
	t.Logf("Found %d 'hello' symbols", len(helloSymbols))

	if len(helloSymbols) == 0 {
		// Try case-sensitive search for different naming conventions
		helloSymbols = symbolIndex.GetSymbolsByName("Hello")
		t.Logf("Found %d 'Hello' symbols", len(helloSymbols))
	}

	// Verify symbols have correct information
	for _, symbol := range userSymbols {
		if symbol.Name != "User" {
			t.Errorf("Expected symbol name 'User', got '%s'", symbol.Name)
		}
		if symbol.FilePath == "" {
			t.Error("Symbol file path is empty")
		}
		if symbol.Line <= 0 {
			t.Error("Symbol line should be positive")
		}
		if symbol.Language == "" {
			t.Error("Symbol language is empty")
		}

		t.Logf("User symbol: %s (kind: %s, file: %s, line: %d, lang: %s)",
			symbol.Name, symbol.Kind, filepath.Base(symbol.FilePath), symbol.Line, symbol.Language)
	}

	// Test language distribution
	languages := stats["languages"].(map[string]int)
	expectedLanguages := []string{"go", "python", "javascript", "rust"}
	for _, lang := range expectedLanguages {
		if count, exists := languages[lang]; exists && count > 0 {
			t.Logf("Language %s: %d files", lang, count)
		} else {
			t.Errorf("Expected symbols for language %s", lang)
		}
	}

	t.Logf("Tree-sitter end-to-end test completed successfully!")
	t.Logf("Total symbols: %d, Total files: %d, Languages: %v", totalSymbols, totalFiles, languages)
}