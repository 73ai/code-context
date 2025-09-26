package parser

import (
	"fmt"
	"testing"
	"time"
)

func TestNewSymbolIndex(t *testing.T) {
	index := NewSymbolIndex()

	if index == nil {
		t.Fatal("NewSymbolIndex returned nil")
	}

	if index.symbols == nil {
		t.Error("symbols map not initialized")
	}

	if index.symbolsByFile == nil {
		t.Error("symbolsByFile map not initialized")
	}

	if index.symbolsByType == nil {
		t.Error("symbolsByType map not initialized")
	}

	if index.totalSymbols != 0 {
		t.Error("totalSymbols should be initialized to 0")
	}

	if index.totalFiles != 0 {
		t.Error("totalFiles should be initialized to 0")
	}
}

func TestSymbol_GenerateID(t *testing.T) {
	symbol := &Symbol{
		Name:     "TestFunction",
		Kind:     SymbolFunction,
		FilePath: "/path/to/file.go",
		Line:     10,
		Column:   5,
	}

	symbol.generateID()

	expectedID := "/path/to/file.go:10:5:function:TestFunction"
	if symbol.ID != expectedID {
		t.Errorf("Expected ID %s, got %s", expectedID, symbol.ID)
	}
}

func TestSymbolIndex_GetSymbolsByKind(t *testing.T) {
	index := NewSymbolIndex()

	// Add test symbols
	symbol1 := &Symbol{
		ID:       "test1",
		Name:     "TestFunc",
		Kind:     SymbolFunction,
		FilePath: "test.go",
		Line:     1,
		Column:   1,
	}

	symbol2 := &Symbol{
		ID:       "test2",
		Name:     "TestVar",
		Kind:     SymbolVariable,
		FilePath: "test.go",
		Line:     2,
		Column:   1,
	}

	symbol3 := &Symbol{
		ID:       "test3",
		Name:     "AnotherFunc",
		Kind:     SymbolFunction,
		FilePath: "test.go",
		Line:     3,
		Column:   1,
	}

	// Manually add to index for testing
	index.symbolsByType[SymbolFunction] = []*Symbol{symbol1, symbol3}
	index.symbolsByType[SymbolVariable] = []*Symbol{symbol2}

	// Test getting functions
	functions := index.GetSymbolsByKind(SymbolFunction)
	if len(functions) != 2 {
		t.Errorf("Expected 2 functions, got %d", len(functions))
	}

	// Test getting variables
	variables := index.GetSymbolsByKind(SymbolVariable)
	if len(variables) != 1 {
		t.Errorf("Expected 1 variable, got %d", len(variables))
	}

	// Test getting non-existent kind
	classes := index.GetSymbolsByKind(SymbolClass)
	if len(classes) != 0 {
		t.Errorf("Expected 0 classes, got %d", len(classes))
	}
}

func TestSymbolIndex_GetSymbolsByName(t *testing.T) {
	index := NewSymbolIndex()

	// Add test symbols with same name
	symbol1 := &Symbol{
		ID:       "test1",
		Name:     "User",
		Kind:     SymbolClass,
		FilePath: "user.py",
		Line:     1,
		Column:   1,
	}

	symbol2 := &Symbol{
		ID:       "test2",
		Name:     "User",
		Kind:     SymbolStruct,
		FilePath: "user.go",
		Line:     1,
		Column:   1,
	}

	symbol3 := &Symbol{
		ID:       "test3",
		Name:     "Product",
		Kind:     SymbolClass,
		FilePath: "product.py",
		Line:     1,
		Column:   1,
	}

	// Manually add to index for testing
	index.symbols["User"] = []*Symbol{symbol1, symbol2}
	index.symbols["Product"] = []*Symbol{symbol3}

	// Test getting symbols by name
	userSymbols := index.GetSymbolsByName("User")
	if len(userSymbols) != 2 {
		t.Errorf("Expected 2 User symbols, got %d", len(userSymbols))
	}

	productSymbols := index.GetSymbolsByName("Product")
	if len(productSymbols) != 1 {
		t.Errorf("Expected 1 Product symbol, got %d", len(productSymbols))
	}

	// Test getting non-existent name
	nonExistent := index.GetSymbolsByName("NonExistent")
	if len(nonExistent) != 0 {
		t.Errorf("Expected 0 symbols for NonExistent, got %d", len(nonExistent))
	}
}

func TestSymbolIndex_GetSymbolsInFile(t *testing.T) {
	index := NewSymbolIndex()

	// Add test symbols
	symbol1 := &Symbol{
		ID:       "test1",
		Name:     "func1",
		Kind:     SymbolFunction,
		FilePath: "file1.go",
		Line:     1,
		Column:   1,
	}

	symbol2 := &Symbol{
		ID:       "test2",
		Name:     "var1",
		Kind:     SymbolVariable,
		FilePath: "file1.go",
		Line:     5,
		Column:   1,
	}

	symbol3 := &Symbol{
		ID:       "test3",
		Name:     "func2",
		Kind:     SymbolFunction,
		FilePath: "file2.go",
		Line:     1,
		Column:   1,
	}

	// Manually add to index for testing
	index.symbolsByFile["file1.go"] = []*Symbol{symbol1, symbol2}
	index.symbolsByFile["file2.go"] = []*Symbol{symbol3}

	// Test getting symbols in file1
	file1Symbols := index.GetSymbolsInFile("file1.go")
	if len(file1Symbols) != 2 {
		t.Errorf("Expected 2 symbols in file1.go, got %d", len(file1Symbols))
	}

	// Test getting symbols in file2
	file2Symbols := index.GetSymbolsInFile("file2.go")
	if len(file2Symbols) != 1 {
		t.Errorf("Expected 1 symbol in file2.go, got %d", len(file2Symbols))
	}

	// Test getting symbols in non-existent file
	nonExistent := index.GetSymbolsInFile("nonexistent.go")
	if len(nonExistent) != 0 {
		t.Errorf("Expected 0 symbols in nonexistent.go, got %d", len(nonExistent))
	}
}

func TestSymbolIndex_GetReferences(t *testing.T) {
	index := NewSymbolIndex()

	// Add test references
	ref1 := Location{File: "file1.go", Line: 10, Column: 5}
	ref2 := Location{File: "file2.go", Line: 20, Column: 10}
	ref3 := Location{File: "file3.go", Line: 30, Column: 15}

	// Manually add to index for testing
	index.references["symbol1"] = []Location{ref1, ref2}
	index.references["symbol2"] = []Location{ref3}

	// Test getting references
	symbol1Refs := index.GetReferences("symbol1")
	if len(symbol1Refs) != 2 {
		t.Errorf("Expected 2 references for symbol1, got %d", len(symbol1Refs))
	}

	symbol2Refs := index.GetReferences("symbol2")
	if len(symbol2Refs) != 1 {
		t.Errorf("Expected 1 reference for symbol2, got %d", len(symbol2Refs))
	}

	// Test getting references for non-existent symbol
	nonExistent := index.GetReferences("nonexistent")
	if len(nonExistent) != 0 {
		t.Errorf("Expected 0 references for nonexistent symbol, got %d", len(nonExistent))
	}
}

func TestSymbolIndex_GetStats(t *testing.T) {
	index := NewSymbolIndex()

	// Set some test data
	index.totalSymbols = 100
	index.totalFiles = 10
	index.languages["go"] = 5
	index.languages["python"] = 3
	index.languages["javascript"] = 2

	index.symbols["TestFunc"] = []*Symbol{{ID: "1"}, {ID: "2"}}
	index.symbols["TestVar"] = []*Symbol{{ID: "3"}}

	index.symbolsByType[SymbolFunction] = []*Symbol{{ID: "1"}, {ID: "2"}}
	index.symbolsByType[SymbolVariable] = []*Symbol{{ID: "3"}}

	stats := index.GetStats()

	// Test basic stats
	if stats["total_symbols"].(int) != 100 {
		t.Errorf("Expected total_symbols 100, got %v", stats["total_symbols"])
	}

	if stats["total_files"].(int) != 10 {
		t.Errorf("Expected total_files 10, got %v", stats["total_files"])
	}

	if stats["unique_names"].(int) != 2 {
		t.Errorf("Expected unique_names 2, got %v", stats["unique_names"])
	}

	if stats["symbol_types"].(int) != 2 {
		t.Errorf("Expected symbol_types 2, got %v", stats["symbol_types"])
	}

	// Test languages stats
	languages, ok := stats["languages"].(map[string]int)
	if !ok {
		t.Error("Languages should be a map[string]int")
	} else {
		if languages["go"] != 5 {
			t.Errorf("Expected go files 5, got %d", languages["go"])
		}
		if languages["python"] != 3 {
			t.Errorf("Expected python files 3, got %d", languages["python"])
		}
	}

	// Test indexed_at is set
	if _, ok := stats["indexed_at"].(time.Time); !ok {
		t.Error("indexed_at should be a time.Time")
	}
}

func TestScopeNode_Creation(t *testing.T) {
	root := &ScopeNode{
		Name:     "file",
		Kind:     ScopeFile,
		Symbols:  make([]*Symbol, 0),
		Children: make([]*ScopeNode, 0),
	}

	if root.Name != "file" {
		t.Errorf("Expected name 'file', got %s", root.Name)
	}

	if root.Kind != ScopeFile {
		t.Errorf("Expected kind ScopeFile, got %s", root.Kind)
	}

	if len(root.Symbols) != 0 {
		t.Error("Expected empty symbols slice")
	}

	if len(root.Children) != 0 {
		t.Error("Expected empty children slice")
	}

	// Test adding child scope
	child := &ScopeNode{
		Name:     "function",
		Kind:     ScopeFunction,
		Parent:   root,
		Symbols:  make([]*Symbol, 0),
		Children: make([]*ScopeNode, 0),
	}

	root.Children = append(root.Children, child)

	if len(root.Children) != 1 {
		t.Error("Expected one child")
	}

	if root.Children[0].Parent != root {
		t.Error("Child parent should point to root")
	}
}

func TestSymbolRelation_Creation(t *testing.T) {
	from := &Symbol{
		ID:   "symbol1",
		Name: "caller",
		Kind: SymbolFunction,
	}

	to := &Symbol{
		ID:   "symbol2",
		Name: "callee",
		Kind: SymbolFunction,
	}

	location := Location{
		File:     "test.go",
		Line:     10,
		Column:   5,
	}

	relation := &SymbolRelation{
		From:         from,
		To:           to,
		RelationType: RelationCalls,
		Location:     location,
	}

	if relation.From.Name != "caller" {
		t.Errorf("Expected from name 'caller', got %s", relation.From.Name)
	}

	if relation.To.Name != "callee" {
		t.Errorf("Expected to name 'callee', got %s", relation.To.Name)
	}

	if relation.RelationType != RelationCalls {
		t.Errorf("Expected relation type RelationCalls, got %s", relation.RelationType)
	}

	if relation.Location.Line != 10 {
		t.Errorf("Expected location line 10, got %d", relation.Location.Line)
	}
}

func BenchmarkSymbolIndex_GetSymbolsByKind(b *testing.B) {
	index := NewSymbolIndex()

	// Create many symbols
	symbols := make([]*Symbol, 1000)
	for i := 0; i < 1000; i++ {
		symbols[i] = &Symbol{
			ID:       fmt.Sprintf("symbol%d", i),
			Name:     fmt.Sprintf("symbol%d", i),
			Kind:     SymbolFunction,
			FilePath: "test.go",
			Line:     i + 1,
			Column:   1,
		}
	}

	index.symbolsByType[SymbolFunction] = symbols

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := index.GetSymbolsByKind(SymbolFunction)
		if len(results) != 1000 {
			b.Fatalf("Expected 1000 symbols, got %d", len(results))
		}
	}
}

func BenchmarkSymbolIndex_GetSymbolsByName(b *testing.B) {
	index := NewSymbolIndex()

	// Create symbols with same name
	symbols := make([]*Symbol, 100)
	for i := 0; i < 100; i++ {
		symbols[i] = &Symbol{
			ID:       fmt.Sprintf("symbol%d", i),
			Name:     "TestFunction",
			Kind:     SymbolFunction,
			FilePath: fmt.Sprintf("file%d.go", i),
			Line:     1,
			Column:   1,
		}
	}

	index.symbols["TestFunction"] = symbols

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := index.GetSymbolsByName("TestFunction")
		if len(results) != 100 {
			b.Fatalf("Expected 100 symbols, got %d", len(results))
		}
	}
}

