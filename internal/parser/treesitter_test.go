package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTreeSitterParser_ParseFile(t *testing.T) {
	// Create test registry
	registry, err := NewLanguageRegistry()
	if err != nil {
		t.Fatalf("Failed to create language registry: %v", err)
	}
	defer registry.Close()

	parser := registry.GetParser()

	testCases := []struct {
		name        string
		language    string
		content     string
		expectedMin int // minimum expected symbols
	}{
		{
			name:        "Go function",
			language:    "go",
			content:     "package main\n\nfunc Hello() string {\n    return \"hello\"\n}",
			expectedMin: 1,
		},
		{
			name:        "Python function",
			language:    "python",
			content:     "def hello():\n    return \"hello\"",
			expectedMin: 1,
		},
		{
			name:        "JavaScript function",
			language:    "javascript",
			content:     "function hello() {\n    return 'hello';\n}",
			expectedMin: 1,
		},
		{
			name:        "TypeScript interface",
			language:    "typescript",
			content:     "interface User {\n    name: string;\n    age: number;\n}",
			expectedMin: 1,
		},
		{
			name:        "Rust function",
			language:    "rust",
			content:     "fn hello() -> String {\n    \"hello\".to_string()\n}",
			expectedMin: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary file
			tmpDir, err := os.MkdirTemp("", "parser_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			ext := getExtensionForLanguage(tc.language)
			filename := "test" + ext
			filePath := filepath.Join(tmpDir, filename)

			if err := os.WriteFile(filePath, []byte(tc.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Parse the file
			result, err := parser.ParseFile(filePath, []byte(tc.content))
			if err != nil {
				t.Fatalf("Failed to parse file: %v", err)
			}

			if len(result.Symbols) < tc.expectedMin {
				t.Errorf("Expected at least %d symbols, got %d", tc.expectedMin, len(result.Symbols))
			}

			if result.Language != tc.language {
				t.Errorf("Expected language %s, got %s", tc.language, result.Language)
			}

			// Verify symbols have required fields
			for _, symbol := range result.Symbols {
				if symbol.Name == "" {
					t.Error("Symbol name is empty")
				}
				if symbol.FilePath != filePath {
					t.Errorf("Symbol file path mismatch: expected %s, got %s", filePath, symbol.FilePath)
				}
				if symbol.Line <= 0 {
					t.Error("Symbol line number should be positive")
				}
			}
		})
	}
}

func TestLanguageRegistry_GetSupportedLanguages(t *testing.T) {
	registry, err := NewLanguageRegistry()
	if err != nil {
		t.Fatalf("Failed to create language registry: %v", err)
	}
	defer registry.Close()

	languages := registry.GetSupportedLanguages()
	expectedLanguages := []string{"go", "python", "javascript", "typescript", "rust"}

	if len(languages) != len(expectedLanguages) {
		t.Errorf("Expected %d languages, got %d", len(expectedLanguages), len(languages))
	}

	for _, expected := range expectedLanguages {
		found := false
		for _, lang := range languages {
			if lang == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Language %s not found in supported languages", expected)
		}
	}
}


func TestSymbolExtractor_ExtractSymbolsFromDirectory(t *testing.T) {
	// Create test registry
	registry, err := NewLanguageRegistry()
	if err != nil {
		t.Fatalf("Failed to create language registry: %v", err)
	}
	defer registry.Close()

	extractor := NewSymbolExtractor(registry)

	// Create temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "extractor_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files for different languages
	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

type User struct {
	Name string
	Age  int
}

func (u *User) String() string {
	return fmt.Sprintf("%s (%d)", u.Name, u.Age)
}

func main() {
	user := &User{Name: "Alice", Age: 30}
	fmt.Println(user)
}

const Version = "1.0.0"
var Debug = false`,

		"utils.py": `"""Utility functions for the application."""

class Calculator:
    """A simple calculator class."""

    def __init__(self):
        self.history = []

    def add(self, a, b):
        """Add two numbers."""
        result = a + b
        self.history.append(f"{a} + {b} = {result}")
        return result

    def multiply(self, a, b):
        """Multiply two numbers."""
        result = a * b
        self.history.append(f"{a} * {b} = {result}")
        return result

PI = 3.14159
DEBUG_MODE = False`,

		"app.js": `/**
 * Main application module
 */

class Application {
    constructor(name) {
        this.name = name;
        this.version = '1.0.0';
    }

    start() {
        console.log("Starting " + this.name + " v" + this.version);
    }

    stop() {
        console.log('Stopping application');
    }
}

function createApp(name) {
    return new Application(name);
}

const DEFAULT_CONFIG = {
    debug: false,
    port: 3000
};

export { Application, createApp, DEFAULT_CONFIG };`,

		"types.ts": `interface User {
    id: number;
    name: string;
    email: string;
    createdAt: Date;
}

type UserRole = 'admin' | 'user' | 'guest';

class UserService {
    private users: User[] = [];

    constructor() {
        this.loadUsers();
    }

    async loadUsers(): Promise<void> {
        // Load users from database
    }

    findUser(id: number): User | undefined {
        return this.users.find(u => u.id === id);
    }

    createUser(userData: Omit<User, 'id' | 'createdAt'>): User {
        const user: User = {
            id: Date.now(),
            ...userData,
            createdAt: new Date()
        };
        this.users.push(user);
        return user;
    }
}

export { User, UserRole, UserService };`,

		"lib.rs": `//! A library for demonstration

use std::collections::HashMap;

/// A simple user struct
#[derive(Debug, Clone)]
pub struct User {
    pub id: u32,
    pub name: String,
    pub email: String,
}

/// User role enumeration
#[derive(Debug, PartialEq)]
pub enum UserRole {
    Admin,
    User,
    Guest,
}

/// User management service
pub struct UserService {
    users: HashMap<u32, User>,
    next_id: u32,
}

impl UserService {
    /// Create a new user service
    pub fn new() -> Self {
        Self {
            users: HashMap::new(),
            next_id: 1,
        }
    }

    /// Add a new user
    pub fn add_user(&mut self, name: String, email: String) -> u32 {
        let id = self.next_id;
        self.next_id += 1;

        let user = User { id, name, email };
        self.users.insert(id, user);

        id
    }

    /// Find a user by ID
    pub fn find_user(&self, id: u32) -> Option<&User> {
        self.users.get(&id)
    }
}

impl Default for UserService {
    fn default() -> Self {
        Self::new()
    }
}

pub const VERSION: &str = "1.0.0";
pub static DEBUG: bool = false;`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", filename, err)
		}
	}

	// Extract symbols from directory
	ctx := context.Background()
	index, err := extractor.ExtractSymbolsFromDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	// Verify statistics
	stats := index.GetStats()
	if stats["total_files"].(int) != len(testFiles) {
		t.Errorf("Expected %d files, got %d", len(testFiles), stats["total_files"])
	}

	if stats["total_symbols"].(int) == 0 {
		t.Error("Expected some symbols to be extracted")
	}

	// Test language distribution
	languages := stats["languages"].(map[string]int)
	expectedLanguages := []string{"go", "python", "javascript", "typescript", "rust"}
	for _, lang := range expectedLanguages {
		if count, exists := languages[lang]; !exists || count == 0 {
			t.Errorf("Expected symbols for language %s", lang)
		}
	}

	// Test symbol retrieval by kind
	functions := index.GetSymbolsByKind(SymbolFunction)
	if len(functions) == 0 {
		t.Error("Expected to find some functions")
	}

	classes := index.GetSymbolsByKind(SymbolClass)
	if len(classes) == 0 {
		t.Error("Expected to find some classes")
	}

	structs := index.GetSymbolsByKind(SymbolStruct)
	if len(structs) == 0 {
		t.Error("Expected to find some structs")
	}

	// Test symbol retrieval by name
	userSymbols := index.GetSymbolsByName("User")
	if len(userSymbols) == 0 {
		t.Error("Expected to find User symbols")
	}

	// Verify that we have User symbols from multiple languages
	languageCount := make(map[string]int)
	for _, symbol := range userSymbols {
		lang := registry.GetLanguageForFile(symbol.FilePath)
		languageCount[lang]++
	}

	if len(languageCount) < 2 {
		t.Error("Expected User symbols from multiple languages")
	}
}

func TestParseResult_Validation(t *testing.T) {
	registry, err := NewLanguageRegistry()
	if err != nil {
		t.Fatalf("Failed to create language registry: %v", err)
	}
	defer registry.Close()

	parser := registry.GetParser()

	// Test with invalid syntax
	invalidContent := "func invalid syntax here"
	tmpDir, err := os.MkdirTemp("", "validation_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "invalid.go")
	result, err := parser.ParseFile(filePath, []byte(invalidContent))

	// Should not fail completely, but might have fewer symbols or errors
	if err != nil {
		t.Logf("Parse error for invalid content (expected): %v", err)
	} else if result != nil {
		t.Logf("Parsed invalid content with %d symbols", len(result.Symbols))
	}
}

func BenchmarkTreeSitterParser_ParseFile(b *testing.B) {
	registry, err := NewLanguageRegistry()
	if err != nil {
		b.Fatalf("Failed to create language registry: %v", err)
	}
	defer registry.Close()

	parser := registry.GetParser()

	// Use a realistic Go file for benchmarking
	content := `package main

import (
	"fmt"
	"net/http"
	"log"
)

type Server struct {
	port int
	mux  *http.ServeMux
}

func NewServer(port int) *Server {
	return &Server{
		port: port,
		mux:  http.NewServeMux(),
	}
}

func (s *Server) Start() error {
	s.mux.HandleFunc("/", s.handleRoot)
	s.mux.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting server on %s", addr)

	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World!")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

func main() {
	server := NewServer(8080)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.ParseFile("benchmark.go", []byte(content))
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}

// Helper function to get file extension for a language
func getExtensionForLanguage(language string) string {
	switch language {
	case "go":
		return ".go"
	case "python":
		return ".py"
	case "javascript":
		return ".js"
	case "typescript":
		return ".ts"
	case "rust":
		return ".rs"
	default:
		return ".txt"
	}
}