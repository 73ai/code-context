package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSemanticSearcher(t *testing.T) {
	searcher, err := NewSemanticSearcher()
	if err != nil {
		t.Fatalf("Failed to create semantic searcher: %v", err)
	}

	if searcher == nil {
		t.Fatal("Semantic searcher is nil")
	}

	if searcher.parsers == nil {
		t.Error("Parsers map is nil")
	}

	if searcher.symbols == nil {
		t.Error("Symbols map is nil")
	}

	if searcher.symbolsByFile == nil {
		t.Error("SymbolsByFile map is nil")
	}

	// Verify Go parser is initialized
	if _, exists := searcher.parsers["go"]; !exists {
		t.Error("Go parser not initialized")
	}

	// Clean up
	if err := searcher.Close(); err != nil {
		t.Errorf("Failed to close searcher: %v", err)
	}
}

func TestSemanticSearcherSymbolSearch(t *testing.T) {
	testDir := createGoTestDirectory(t)
	defer os.RemoveAll(testDir)

	searcher, err := NewSemanticSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:     "TestFunction",
		SearchPaths: []string{testDir},
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	var resultCount int
	var searchErr error

	done := false
	for !done {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}
			resultCount++

			// Verify semantic result structure
			if result.SymbolName == "" {
				t.Error("Semantic result missing symbol name")
			}
			if result.SymbolKind == "" {
				t.Error("Semantic result missing symbol kind")
			}
			if result.FilePath == "" {
				t.Error("Semantic result missing file path")
			}
			if result.LineNumber <= 0 {
				t.Error("Semantic result missing line number")
			}

			if result.SymbolName != "TestFunction" {
				t.Errorf("Expected symbol name 'TestFunction', got '%s'", result.SymbolName)
			}

		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			if searchErr == nil {
				searchErr = err
			}

		case <-time.After(10 * time.Second):
			t.Fatal("Semantic search timeout")
		}

		if results == nil && errs == nil {
			done = true
		}
	}

	if searchErr != nil {
		t.Fatalf("Semantic search error: %v", searchErr)
	}

	if resultCount == 0 {
		t.Error("No semantic search results found")
	}

	// Check statistics
	stats := searcher.Stats()
	if stats.FilesSearched == 0 {
		t.Error("No files were searched according to stats")
	}
}

func TestSemanticSearcherDefinitionSearch(t *testing.T) {
	testDir := createGoTestDirectory(t)
	defer os.RemoveAll(testDir)

	searcher, err := NewSemanticSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:     "TestStruct",
		SearchPaths: []string{testDir},
		FindDefs:    true,
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	var resultCount int
	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}
			resultCount++

			// Verify it's a definition result
			if result.Metadata["search_type"] != "definition" {
				t.Error("Expected definition search type")
			}

			if result.SymbolName != "TestStruct" {
				t.Errorf("Expected symbol name 'TestStruct', got '%s'", result.SymbolName)
			}

			// Should be a struct type
			if result.SymbolKind != string(SymbolStruct) {
				t.Errorf("Expected struct symbol kind, got '%s'", result.SymbolKind)
			}

		case err := <-errs:
			t.Fatalf("Definition search error: %v", err)

		case <-time.After(10 * time.Second):
			t.Fatal("Definition search timeout")
		}

		if results == nil {
			break
		}
	}

	if resultCount == 0 {
		t.Error("No definition search results found")
	}
}

func TestSemanticSearcherReferenceSearch(t *testing.T) {
	// Create Go file with references
	tmpDir, err := os.MkdirTemp("", "semantic_refs_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	goContent := `package main

type TestType struct {
	Field string
}

func main() {
	var instance TestType
	instance.Field = "value"

	another := TestType{Field: "test"}
	_ = another
}
`

	filePath := filepath.Join(tmpDir, "refs.go")
	if err := os.WriteFile(filePath, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create Go test file: %v", err)
	}

	searcher, err := NewSemanticSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:     "TestType",
		SearchPaths: []string{tmpDir},
		FindRefs:    true,
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	var resultCount int
	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}
			resultCount++

			// Verify it's a reference result
			if result.Metadata["search_type"] != "reference" {
				t.Error("Expected reference search type")
			}

			if result.SymbolName != "TestType" {
				t.Errorf("Expected symbol name 'TestType', got '%s'", result.SymbolName)
			}

		case err := <-errs:
			t.Fatalf("Reference search error: %v", err)

		case <-time.After(10 * time.Second):
			t.Fatal("Reference search timeout")
		}

		if results == nil {
			break
		}
	}

	// Should find some references
	if resultCount == 0 {
		t.Error("No reference search results found")
	}
}

func TestSemanticSearcherSymbolTypes(t *testing.T) {
	testDir := createGoTestDirectory(t)
	defer os.RemoveAll(testDir)

	searcher, err := NewSemanticSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	// Test filtering by symbol type
	opts := &SearchOptions{
		Pattern:     "Test",
		SearchPaths: []string{testDir},
		SymbolTypes: []string{"function"}, // Only functions
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	var resultCount int
	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}
			resultCount++

			// Should only return function symbols
			if result.SymbolKind != string(SymbolFunction) && result.SymbolKind != string(SymbolMethod) {
				t.Errorf("Expected function/method symbol, got '%s'", result.SymbolKind)
			}

		case err := <-errs:
			t.Fatalf("Symbol type search error: %v", err)

		case <-time.After(10 * time.Second):
			t.Fatal("Symbol type search timeout")
		}

		if results == nil {
			break
		}
	}

	if resultCount == 0 {
		t.Error("No symbol type filtered results found")
	}
}

func TestSemanticSearcherCaseSensitive(t *testing.T) {
	testDir := createGoTestDirectory(t)
	defer os.RemoveAll(testDir)

	searcher, err := NewSemanticSearcher()
	if err != nil {
		t.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	// Test case sensitive
	opts := &SearchOptions{
		Pattern:       "testfunction", // lowercase
		SearchPaths:   []string{testDir},
		CaseSensitive: true,
	}

	ctx := context.Background()
	results, errs := searcher.Search(ctx, opts)

	caseSensitiveCount := 0
	for {
		select {
		case _, ok := <-results:
			if !ok {
				results = nil
				break
			}
			caseSensitiveCount++

		case err := <-errs:
			t.Fatalf("Case sensitive search error: %v", err)

		case <-time.After(10 * time.Second):
			t.Fatal("Case sensitive search timeout")
		}

		if results == nil {
			break
		}
	}

	// Test case insensitive
	opts.CaseSensitive = false
	results, errs = searcher.Search(ctx, opts)

	caseInsensitiveCount := 0
	for {
		select {
		case _, ok := <-results:
			if !ok {
				results = nil
				break
			}
			caseInsensitiveCount++

		case err := <-errs:
			t.Fatalf("Case insensitive search error: %v", err)

		case <-time.After(10 * time.Second):
			t.Fatal("Case insensitive search timeout")
		}

		if results == nil {
			break
		}
	}

	// Case insensitive should find more or equal results
	if caseInsensitiveCount < caseSensitiveCount {
		t.Errorf("Case insensitive should find >= results: %d vs %d", caseInsensitiveCount, caseSensitiveCount)
	}
}

func TestGoParserParseFile(t *testing.T) {
	goContent := `package main

import "fmt"

// TestFunction demonstrates parsing
func TestFunction(param string) int {
	fmt.Println(param)
	return 42
}

// TestStruct is a test structure
type TestStruct struct {
	Field1 string
	Field2 int
}

const TestConstant = "value"

var TestVariable = 100

func (ts *TestStruct) TestMethod() {
	// Method implementation
}

type TestInterface interface {
	Method() string
}
`

	parser := &GoParser{}
	symbols, err := parser.ParseFile("test.go", []byte(goContent))
	if err != nil {
		t.Fatalf("Failed to parse Go file: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("No symbols extracted")
	}

	// Verify different symbol types are extracted
	symbolsByKind := make(map[SymbolKind][]*Symbol)
	for _, symbol := range symbols {
		symbolsByKind[symbol.Kind] = append(symbolsByKind[symbol.Kind], symbol)
	}

	// Should find functions
	if len(symbolsByKind[SymbolFunction]) == 0 {
		t.Error("No functions found")
	}

	// Should find methods
	if len(symbolsByKind[SymbolMethod]) == 0 {
		t.Error("No methods found")
	}

	// Should find structs
	if len(symbolsByKind[SymbolStruct]) == 0 {
		t.Error("No structs found")
	}

	// Should find interfaces
	if len(symbolsByKind[SymbolInterface]) == 0 {
		t.Error("No interfaces found")
	}

	// Should find constants
	if len(symbolsByKind[SymbolConstant]) == 0 {
		t.Error("No constants found")
	}

	// Should find variables
	if len(symbolsByKind[SymbolVariable]) == 0 {
		t.Error("No variables found")
	}

	// Verify specific symbols
	functionFound := false
	structFound := false
	for _, symbol := range symbols {
		if symbol.Name == "TestFunction" && symbol.Kind == SymbolFunction {
			functionFound = true
			if symbol.Line <= 0 {
				t.Error("Function symbol missing line number")
			}
			if symbol.DocString == "" {
				t.Error("Function symbol missing doc string")
			}
		}
		if symbol.Name == "TestStruct" && symbol.Kind == SymbolStruct {
			structFound = true
		}
	}

	if !functionFound {
		t.Error("TestFunction not found in symbols")
	}
	if !structFound {
		t.Error("TestStruct not found in symbols")
	}
}

func TestGoParserGetFileExtensions(t *testing.T) {
	parser := &GoParser{}
	extensions := parser.GetFileExtensions()

	if len(extensions) == 0 {
		t.Error("No file extensions returned")
	}

	found := false
	for _, ext := range extensions {
		if ext == ".go" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Go file extension not found")
	}
}

func TestGoParserFindReferences(t *testing.T) {
	// Create test files
	tmpDir, err := os.MkdirTemp("", "go_refs_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// File with definition
	defContent := `package main

func TargetFunction() {
	// Definition
}
`

	defPath := filepath.Join(tmpDir, "def.go")
	if err := os.WriteFile(defPath, []byte(defContent), 0644); err != nil {
		t.Fatalf("Failed to create definition file: %v", err)
	}

	// File with references
	refContent := `package main

func main() {
	TargetFunction()
	TargetFunction()
}
`

	refPath := filepath.Join(tmpDir, "ref.go")
	if err := os.WriteFile(refPath, []byte(refContent), 0644); err != nil {
		t.Fatalf("Failed to create reference file: %v", err)
	}

	parser := &GoParser{}
	symbol := &Symbol{
		Name:     "TargetFunction",
		Kind:     SymbolFunction,
		FilePath: defPath,
		Line:     3,
	}

	files := []string{defPath, refPath}
	references, err := parser.FindReferences(symbol, files)
	if err != nil {
		t.Fatalf("Failed to find references: %v", err)
	}

	if len(references) == 0 {
		t.Error("No references found")
	}

	// Should find references in both files
	fileCount := make(map[string]int)
	for _, ref := range references {
		fileCount[ref.File]++
	}

	if len(fileCount) == 0 {
		t.Error("No files with references found")
	}
}

func BenchmarkSemanticSearcherGoFiles(b *testing.B) {
	testDir := createLargeGoProject(b)
	defer os.RemoveAll(testDir)

	searcher, err := NewSemanticSearcher()
	if err != nil {
		b.Fatalf("Failed to create searcher: %v", err)
	}
	defer searcher.Close()

	opts := &SearchOptions{
		Pattern:     "TestFunction",
		SearchPaths: []string{testDir},
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		results, errs := searcher.Search(ctx, opts)

		// Consume results
		for {
			select {
			case _, ok := <-results:
				if !ok {
					results = nil
				}
			case err := <-errs:
				b.Fatalf("Benchmark search error: %v", err)
			}

			if results == nil {
				break
			}
		}
	}
}

func BenchmarkGoParserParseFile(b *testing.B) {
	goContent := `package main

import "fmt"

func TestFunction1() { fmt.Println("test") }
func TestFunction2() { fmt.Println("test") }
func TestFunction3() { fmt.Println("test") }

type TestStruct1 struct { Field string }
type TestStruct2 struct { Field string }

const Constant1 = "value"
const Constant2 = "value"

var Variable1 = 1
var Variable2 = 2
`

	parser := &GoParser{}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := parser.ParseFile("test.go", []byte(goContent))
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}

// Helper function to create a large Go project for benchmarking
func createLargeGoProject(b *testing.B) string {
	tmpDir, err := os.MkdirTemp("", "large_go_project")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create multiple Go files
	for i := 0; i < 50; i++ {
		content := fmt.Sprintf(`package main

import "fmt"

func TestFunction%d() {
	fmt.Println("Function %d")
}

type TestStruct%d struct {
	Field1 string
	Field2 int
}

const TestConstant%d = "value%d"

var TestVariable%d = %d

func (ts *TestStruct%d) Method%d() {
	// Method implementation
}
`, i, i, i, i, i, i, i, i, i)

		filename := filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i))
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			b.Fatalf("Failed to create Go file: %v", err)
		}
	}

	return tmpDir
}