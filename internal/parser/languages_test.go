package parser

import (
	"testing"
)

func TestNewLanguageRegistry(t *testing.T) {
	registry, err := NewLanguageRegistry()
	if err != nil {
		t.Fatalf("Failed to create language registry: %v", err)
	}
	defer registry.Close()

	if registry == nil {
		t.Fatal("NewLanguageRegistry returned nil")
	}

	if registry.parser == nil {
		t.Error("Parser not initialized")
	}

	// Test that supported languages are registered
	supportedLanguages := registry.GetSupportedLanguages()
	expectedCount := 5 // go, python, javascript, typescript, rust

	if len(supportedLanguages) != expectedCount {
		t.Errorf("Expected %d supported languages, got %d", expectedCount, len(supportedLanguages))
	}
}

func TestLanguageRegistry_GetLanguageFeatures(t *testing.T) {
	registry, err := NewLanguageRegistry()
	if err != nil {
		t.Fatalf("Failed to create language registry: %v", err)
	}
	defer registry.Close()

	features := registry.GetLanguageFeatures()

	expectedFeatures := []string{"go", "python", "javascript", "typescript", "rust"}

	if len(features) != len(expectedFeatures) {
		t.Errorf("Expected %d language features, got %d", len(expectedFeatures), len(features))
	}

	// Test Go language features
	goFeatures := features[0]
	if goFeatures.Language != "go" {
		t.Errorf("Expected first language to be 'go', got %s", goFeatures.Language)
	}

	if !goFeatures.SupportsFunctions {
		t.Error("Go should support functions")
	}

	if !goFeatures.SupportsTypes {
		t.Error("Go should support types")
	}

	if goFeatures.SupportsClasses {
		t.Error("Go should not support classes")
	}

	// Test Python language features
	var pythonFeatures *LanguageFeatures
	for _, f := range features {
		if f.Language == "python" {
			pythonFeatures = &f
			break
		}
	}

	if pythonFeatures == nil {
		t.Fatal("Python features not found")
	}

	if !pythonFeatures.SupportsClasses {
		t.Error("Python should support classes")
	}

	if !pythonFeatures.SupportsFunctions {
		t.Error("Python should support functions")
	}

	// Test TypeScript language features
	var tsFeatures *LanguageFeatures
	for _, f := range features {
		if f.Language == "typescript" {
			tsFeatures = &f
			break
		}
	}

	if tsFeatures == nil {
		t.Fatal("TypeScript features not found")
	}

	if !tsFeatures.SupportsTypes {
		t.Error("TypeScript should support types")
	}

	if !tsFeatures.SupportsClasses {
		t.Error("TypeScript should support classes")
	}

	if !tsFeatures.SupportsInterfaces {
		t.Error("TypeScript should support interfaces")
	}
}

func TestLanguageRegistry_GetLanguageForFile(t *testing.T) {
	registry, err := NewLanguageRegistry()
	if err != nil {
		t.Fatalf("Failed to create language registry: %v", err)
	}
	defer registry.Close()

	testCases := []struct {
		filePath string
		expected string
	}{
		// Go files
		{"main.go", "go"},
		{"src/utils.go", "go"},
		{"/absolute/path/file.go", "go"},

		// Python files
		{"script.py", "python"},
		{"module.pyx", "python"},
		{"types.pyi", "python"},

		// JavaScript files
		{"app.js", "javascript"},
		{"bundle.mjs", "javascript"},
		{"Component.jsx", "javascript"},

		// TypeScript files
		{"types.ts", "typescript"},
		{"Component.tsx", "typescript"},
		{"definitions.d.ts", "typescript"},

		// Rust files
		{"main.rs", "rust"},
		{"lib.rs", "rust"},

		// Unsupported files
		{"README.md", ""},
		{"config.json", ""},
		{"styles.css", ""},
		{"template.html", ""},
		{"binary.exe", ""},
		{"archive.tar.gz", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.filePath, func(t *testing.T) {
			result := registry.GetLanguageForFile(tc.filePath)
			if result != tc.expected {
				t.Errorf("For file %s, expected language %s, got %s", tc.filePath, tc.expected, result)
			}
		})
	}
}

func TestGetDefaultQueries(t *testing.T) {
	testCases := []struct {
		language string
		hasQuery map[string]bool
	}{
		{
			language: "go",
			hasQuery: map[string]bool{
				"functions": true,
				"variables": true,
				"types":     true,
				"imports":   true,
				"classes":   false, // Go doesn't have classes
			},
		},
		{
			language: "python",
			hasQuery: map[string]bool{
				"functions": true,
				"variables": true,
				"classes":   true,
				"imports":   true,
				"types":     false, // Not in default Python queries
			},
		},
		{
			language: "javascript",
			hasQuery: map[string]bool{
				"functions": true,
				"variables": true,
				"classes":   true,
				"imports":   true,
			},
		},
		{
			language: "typescript",
			hasQuery: map[string]bool{
				"functions":  true,
				"variables":  true,
				"classes":    true,
				"interfaces": true,
				"types":      true,
			},
		},
		{
			language: "rust",
			hasQuery: map[string]bool{
				"functions": true,
				"variables": true,
				"structs":   true,
				"enums":     true,
				"traits":    true,
				"imports":   true,
			},
		},
		{
			language: "unknown",
			hasQuery: map[string]bool{}, // Should return empty map
		},
	}

	for _, tc := range testCases {
		t.Run(tc.language, func(t *testing.T) {
			queries := GetDefaultQueries(tc.language)

			if tc.language == "unknown" {
				if len(queries) != 0 {
					t.Errorf("Expected empty queries for unknown language, got %d", len(queries))
				}
				return
			}

			// Check expected queries
			for queryType, shouldExist := range tc.hasQuery {
				query, exists := queries[queryType]
				if shouldExist && !exists {
					t.Errorf("Expected %s query for %s language", queryType, tc.language)
				}
				if !shouldExist && exists {
					t.Errorf("Did not expect %s query for %s language", queryType, tc.language)
				}
				if exists && query == "" {
					t.Errorf("Query %s for %s language should not be empty", queryType, tc.language)
				}
			}
		})
	}
}

func TestNewLanguageSpecificParser(t *testing.T) {
	registry, err := NewLanguageRegistry()
	if err != nil {
		t.Fatalf("Failed to create language registry: %v", err)
	}
	defer registry.Close()

	parser := NewLanguageSpecificParser("go", registry)

	if parser == nil {
		t.Fatal("NewLanguageSpecificParser returned nil")
	}

	if parser.Language != "go" {
		t.Errorf("Expected language 'go', got %s", parser.Language)
	}

	if parser.registry != registry {
		t.Error("Registry not set correctly")
	}
}

func TestLanguageSpecificParser_GetSymbolKindTypes(t *testing.T) {
	registry, err := NewLanguageRegistry()
	if err != nil {
		t.Fatalf("Failed to create language registry: %v", err)
	}
	defer registry.Close()

	testCases := []struct {
		language      string
		expectedTypes []SymbolKind
		notExpected   []SymbolKind
	}{
		{
			language: "go",
			expectedTypes: []SymbolKind{
				SymbolFunction, SymbolMethod, SymbolVariable, SymbolConstant,
				SymbolType, SymbolStruct, SymbolInterface, SymbolImport,
			},
			notExpected: []SymbolKind{SymbolClass},
		},
		{
			language: "python",
			expectedTypes: []SymbolKind{
				SymbolFunction, SymbolMethod, SymbolClass, SymbolVariable,
				SymbolImport, SymbolProperty,
			},
			notExpected: []SymbolKind{SymbolStruct, SymbolInterface},
		},
		{
			language: "javascript",
			expectedTypes: []SymbolKind{
				SymbolFunction, SymbolMethod, SymbolClass, SymbolVariable,
				SymbolConstant, SymbolImport, SymbolProperty,
			},
			notExpected: []SymbolKind{SymbolStruct, SymbolInterface},
		},
		{
			language: "typescript",
			expectedTypes: []SymbolKind{
				SymbolFunction, SymbolMethod, SymbolClass, SymbolInterface,
				SymbolType, SymbolVariable, SymbolConstant, SymbolImport, SymbolProperty,
			},
			notExpected: []SymbolKind{SymbolStruct},
		},
		{
			language: "rust",
			expectedTypes: []SymbolKind{
				SymbolFunction, SymbolMethod, SymbolStruct, SymbolEnum,
				SymbolType, SymbolVariable, SymbolConstant, SymbolImport,
			},
			notExpected: []SymbolKind{SymbolClass, SymbolInterface},
		},
		{
			language:      "unknown",
			expectedTypes: []SymbolKind{},
			notExpected:   []SymbolKind{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.language, func(t *testing.T) {
			parser := NewLanguageSpecificParser(tc.language, registry)
			symbolTypes := parser.GetSymbolKindTypes()

			if tc.language == "unknown" {
				if len(symbolTypes) != 0 {
					t.Errorf("Expected no symbol types for unknown language, got %d", len(symbolTypes))
				}
				return
			}

			// Check that expected types are present
			for _, expectedType := range tc.expectedTypes {
				found := false
				for _, symbolType := range symbolTypes {
					if symbolType == expectedType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected symbol type %s for language %s", expectedType, tc.language)
				}
			}

			// Check that not expected types are not present
			for _, notExpectedType := range tc.notExpected {
				found := false
				for _, symbolType := range symbolTypes {
					if symbolType == notExpectedType {
						found = true
						break
					}
				}
				if found {
					t.Errorf("Did not expect symbol type %s for language %s", notExpectedType, tc.language)
				}
			}
		})
	}
}

func TestLanguageRegistry_Close(t *testing.T) {
	registry, err := NewLanguageRegistry()
	if err != nil {
		t.Fatalf("Failed to create language registry: %v", err)
	}

	// Should not error
	err = registry.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Should be safe to call multiple times
	err = registry.Close()
	if err != nil {
		t.Errorf("Second Close returned error: %v", err)
	}
}

func BenchmarkLanguageRegistry_GetLanguageForFile(b *testing.B) {
	registry, err := NewLanguageRegistry()
	if err != nil {
		b.Fatalf("Failed to create language registry: %v", err)
	}
	defer registry.Close()

	testFiles := []string{
		"main.go",
		"script.py",
		"app.js",
		"types.ts",
		"lib.rs",
		"config.json", // unsupported
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, file := range testFiles {
			_ = registry.GetLanguageForFile(file)
		}
	}
}

func BenchmarkGetDefaultQueries(b *testing.B) {
	languages := []string{"go", "python", "javascript", "typescript", "rust"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, lang := range languages {
			_ = GetDefaultQueries(lang)
		}
	}
}