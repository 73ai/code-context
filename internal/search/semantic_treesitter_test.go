package search

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSemanticSearcher_TreeSitterIntegration(t *testing.T) {
	// Create a new semantic searcher with tree-sitter
	searcher, err := NewSemanticSearcher()
	if err != nil {
		t.Fatalf("Failed to create semantic searcher: %v", err)
	}
	defer searcher.Close()

	// Create temporary directory with test files
	tmpDir, err := ioutil.TempDir("", "semantic_test")
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

func (u *User) GetName() string {
	return u.Name
}

func CreateUser(name string, age int) *User {
	return &User{Name: name, Age: age}
}

func main() {
	user := CreateUser("Alice", 30)
	fmt.Println(user.GetName())
}`,

		"utils.py": `class Calculator:
    def __init__(self):
        self.result = 0

    def add(self, a, b):
        return a + b

    def multiply(self, a, b):
        return a * b

def create_calculator():
    return Calculator()

PI = 3.14159`,

		"app.js": `class Application {
    constructor(name) {
        this.name = name;
    }

    start() {
        console.log('Starting', this.name);
    }
}

function createApp(name) {
    return new Application(name);
}

const DEFAULT_PORT = 3000;`,
	}

	// Write test files
	for filename, content := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", filename, err)
		}
	}

	// Create search options
	opts := &SearchOptions{
		Pattern:     "User",
		SearchPaths: []string{tmpDir},
		FindDefs:    true,
	}

	// Perform search
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, errors := searcher.Search(ctx, opts)

	// Collect results
	var searchResults []SearchResult
	var searchErrors []error

	// Read results and errors
	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
			} else {
				searchResults = append(searchResults, result)
			}
		case err, ok := <-errors:
			if !ok {
				errors = nil
			} else {
				searchErrors = append(searchErrors, err)
			}
		case <-ctx.Done():
			t.Fatal("Search timed out")
		}

		if results == nil && errors == nil {
			break
		}
	}

	// Check for errors
	if len(searchErrors) > 0 {
		t.Errorf("Search returned errors: %v", searchErrors)
	}

	// Verify we found User symbols
	if len(searchResults) == 0 {
		t.Fatal("Expected to find User symbols")
	}

	// Check that we found User in Go file
	foundGoUser := false
	for _, result := range searchResults {
		if filepath.Base(result.FilePath) == "main.go" && result.SymbolName == "User" {
			foundGoUser = true
			if result.SymbolKind != "struct" {
				t.Errorf("Expected Go User to be struct, got %s", result.SymbolKind)
			}
			break
		}
	}

	if !foundGoUser {
		t.Error("Expected to find User struct in Go file")
	}
}

func TestSemanticSearcher_MultiLanguageSearch(t *testing.T) {
	searcher, err := NewSemanticSearcher()
	if err != nil {
		t.Fatalf("Failed to create semantic searcher: %v", err)
	}
	defer searcher.Close()

	// Create temporary directory
	tmpDir, err := ioutil.TempDir("", "multilang_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files with similar function names
	testFiles := map[string]string{
		"server.go": `package main

func StartServer(port int) error {
	return nil
}

func StopServer() error {
	return nil
}`,

		"server.py": `def start_server(port):
    pass

def stop_server():
    pass

class Server:
    def start(self):
        pass`,

		"server.js": `function startServer(port) {
    return Promise.resolve();
}

function stopServer() {
    return Promise.resolve();
}

class Server {
    start() {
        return Promise.resolve();
    }
}`,

		"server.ts": `interface ServerConfig {
    port: number;
    host: string;
}

class Server {
    constructor(private config: ServerConfig) {}

    start(): Promise<void> {
        return Promise.resolve();
    }

    stop(): Promise<void> {
        return Promise.resolve();
    }
}

function createServer(config: ServerConfig): Server {
    return new Server(config);
}`,

		"server.rs": `pub struct Server {
    port: u16,
}

impl Server {
    pub fn new(port: u16) -> Self {
        Self { port }
    }

    pub fn start(&self) -> Result<(), String> {
        Ok(())
    }

    pub fn stop(&self) -> Result<(), String> {
        Ok(())
    }
}

pub fn create_server(port: u16) -> Server {
    Server::new(port)
}`,
	}

	// Write test files
	for filename, content := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", filename, err)
		}
	}

	// Test searching for "Server" across all languages
	opts := &SearchOptions{
		Pattern:     "Server",
		SearchPaths: []string{tmpDir},
		FindDefs:    true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, errors := searcher.Search(ctx, opts)

	// Collect results
	var searchResults []SearchResult
	var searchErrors []error

	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
			} else {
				searchResults = append(searchResults, result)
			}
		case err, ok := <-errors:
			if !ok {
				errors = nil
			} else {
				searchErrors = append(searchErrors, err)
			}
		case <-ctx.Done():
			t.Fatal("Search timed out")
		}

		if results == nil && errors == nil {
			break
		}
	}

	if len(searchErrors) > 0 {
		t.Errorf("Search returned errors: %v", searchErrors)
	}

	if len(searchResults) == 0 {
		t.Fatal("Expected to find Server symbols")
	}

	// Count symbols by language
	languageCounts := make(map[string]int)
	for _, result := range searchResults {
		ext := filepath.Ext(result.FilePath)
		languageCounts[ext]++
	}

	// We should find Server in multiple languages
	if len(languageCounts) < 3 {
		t.Errorf("Expected Server symbols in at least 3 languages, found in %d", len(languageCounts))
	}

	// Check specific languages
	expectedExts := []string{".go", ".py", ".js", ".ts", ".rs"}
	for _, ext := range expectedExts {
		if count, found := languageCounts[ext]; !found || count == 0 {
			t.Errorf("Expected to find Server symbols in %s files", ext)
		}
	}
}

func TestSemanticSearcher_SymbolTypeFiltering(t *testing.T) {
	searcher, err := NewSemanticSearcher()
	if err != nil {
		t.Fatalf("Failed to create semantic searcher: %v", err)
	}
	defer searcher.Close()

	tmpDir, err := ioutil.TempDir("", "type_filter_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file with various symbol types
	testContent := `package main

import "fmt"

type User struct {
	Name string
}

func User() string {
	return "user function"
}

const User = "user constant"

var User = "user variable"

func main() {
	fmt.Println(User)
}`

	filePath := filepath.Join(tmpDir, "test.go")
	if err := ioutil.WriteFile(filePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test searching for functions only
	opts := &SearchOptions{
		Pattern:     "User",
		SearchPaths: []string{tmpDir},
		FindDefs:    false, // Use general search
		SymbolTypes: []string{"function"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, errors := searcher.Search(ctx, opts)

	// Collect results
	var searchResults []SearchResult
	var searchErrors []error

	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
			} else {
				searchResults = append(searchResults, result)
			}
		case err, ok := <-errors:
			if !ok {
				errors = nil
			} else {
				searchErrors = append(searchErrors, err)
			}
		case <-ctx.Done():
			t.Fatal("Search timed out")
		}

		if results == nil && errors == nil {
			break
		}
	}

	if len(searchErrors) > 0 {
		t.Errorf("Search returned errors: %v", searchErrors)
	}

	// Should find at least the function symbol
	functionFound := false
	for _, result := range searchResults {
		if result.SymbolKind == "function" {
			functionFound = true
		}
		// All results should be functions when filtering by function type
		if result.SymbolKind != "function" && result.SymbolKind != "method" {
			t.Errorf("Expected only function symbols, found %s", result.SymbolKind)
		}
	}

	if !functionFound {
		t.Error("Expected to find function symbol when filtering by function type")
	}
}

func TestSemanticSearcher_GetSupportedLanguages(t *testing.T) {
	searcher, err := NewSemanticSearcher()
	if err != nil {
		t.Fatalf("Failed to create semantic searcher: %v", err)
	}
	defer searcher.Close()

	languages := searcher.GetSupportedLanguages()

	expectedLanguages := []string{"go", "python", "javascript", "typescript", "rust"}

	if len(languages) != len(expectedLanguages) {
		t.Errorf("Expected %d supported languages, got %d", len(expectedLanguages), len(languages))
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
			t.Errorf("Expected to find language %s in supported languages", expected)
		}
	}
}

func TestSemanticSearcher_GetLanguageFeatures(t *testing.T) {
	searcher, err := NewSemanticSearcher()
	if err != nil {
		t.Fatalf("Failed to create semantic searcher: %v", err)
	}
	defer searcher.Close()

	features := searcher.GetLanguageFeatures()

	if len(features) == 0 {
		t.Error("Expected language features to be returned")
	}

	// Check that we have features for expected languages
	expectedLanguages := []string{"go", "python", "javascript", "typescript", "rust"}
	for _, lang := range expectedLanguages {
		if _, exists := features[lang]; !exists {
			t.Errorf("Expected features for language %s", lang)
		}
	}
}

func TestSemanticSearcher_Stats(t *testing.T) {
	searcher, err := NewSemanticSearcher()
	if err != nil {
		t.Fatalf("Failed to create semantic searcher: %v", err)
	}
	defer searcher.Close()

	// Test stats before any indexing
	stats := searcher.Stats()
	if stats.TotalFiles != 0 {
		t.Error("Expected 0 files before indexing")
	}

	// Test index stats
	indexStats := searcher.GetIndexStats()
	if indexStats == nil {
		t.Error("Expected index stats to be returned")
	}
}

func BenchmarkSemanticSearcher_TreeSitterSearch(b *testing.B) {
	searcher, err := NewSemanticSearcher()
	if err != nil {
		b.Fatalf("Failed to create semantic searcher: %v", err)
	}
	defer searcher.Close()

	// Create temporary directory
	tmpDir, err := ioutil.TempDir("", "benchmark_test")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a larger test file
	content := `package main

import "fmt"

type User struct {
	ID   int
	Name string
	Age  int
}

type Product struct {
	ID    int
	Name  string
	Price float64
}

func (u *User) GetName() string {
	return u.Name
}

func (u *User) SetName(name string) {
	u.Name = name
}

func (p *Product) GetPrice() float64 {
	return p.Price
}

func CreateUser(name string, age int) *User {
	return &User{Name: name, Age: age}
}

func CreateProduct(name string, price float64) *Product {
	return &Product{Name: name, Price: price}
}

func main() {
	user := CreateUser("Alice", 30)
	product := CreateProduct("Book", 19.99)

	fmt.Printf("User: %s\n", user.GetName())
	fmt.Printf("Product: %s, Price: $%.2f\n", product.Name, product.GetPrice())
}`

	filePath := filepath.Join(tmpDir, "large_test.go")
	if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
		b.Fatalf("Failed to write test file: %v", err)
	}

	opts := &SearchOptions{
		Pattern:     "User",
		SearchPaths: []string{tmpDir},
		FindDefs:    true,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		results, errors := searcher.Search(ctx, opts)

		// Consume results
		for {
			select {
			case _, ok := <-results:
				if !ok {
					results = nil
				}
			case _, ok := <-errors:
				if !ok {
					errors = nil
				}
			case <-ctx.Done():
				b.Fatal("Search timed out")
			}

			if results == nil && errors == nil {
				break
			}
		}

		cancel()
	}
}