package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// SymbolExtractor provides advanced symbol extraction using tree-sitter
type SymbolExtractor struct {
	registry *LanguageRegistry
	parser   *TreeSitterParser
}

// SymbolIndex maintains an index of all symbols in a codebase
type SymbolIndex struct {
	// Symbol mappings
	symbols       map[string][]*Symbol // symbol_name -> symbols
	symbolsByFile map[string][]*Symbol // file_path -> symbols
	symbolsByType map[SymbolKind][]*Symbol // symbol_type -> symbols

	// References and relationships
	references    map[string][]Location // symbol_id -> reference_locations
	dependencies  map[string][]string   // file_path -> dependent_file_paths

	// Scoping information
	scopes        map[string]*ScopeTree // file_path -> scope_tree

	// Index metadata
	indexTime     time.Time
	totalSymbols  int
	totalFiles    int
	languages     map[string]int // language -> file_count

	mu sync.RWMutex
}

// ScopeTree represents the hierarchical scope structure of a file
type ScopeTree struct {
	Root     *ScopeNode
	FilePath string
}

// ScopeNode represents a scope in the source code
type ScopeNode struct {
	Name       string          `json:"name"`
	Kind       ScopeKind       `json:"kind"`
	StartLine  int            `json:"start_line"`
	EndLine    int            `json:"end_line"`
	Symbols    []*Symbol      `json:"symbols"`
	Children   []*ScopeNode   `json:"children"`
	Parent     *ScopeNode     `json:"-"`
}

// ScopeKind represents the type of scope
type ScopeKind string

const (
	ScopeFile      ScopeKind = "file"
	ScopePackage   ScopeKind = "package"
	ScopeModule    ScopeKind = "module"
	ScopeNamespace ScopeKind = "namespace"
	ScopeClass     ScopeKind = "class"
	ScopeFunction  ScopeKind = "function"
	ScopeMethod    ScopeKind = "method"
	ScopeBlock     ScopeKind = "block"
	ScopeIf        ScopeKind = "if"
	ScopeFor       ScopeKind = "for"
	ScopeWhile     ScopeKind = "while"
	ScopeTry       ScopeKind = "try"
)

// SymbolRelation represents a relationship between symbols
type SymbolRelation struct {
	From         *Symbol         `json:"from"`
	To           *Symbol         `json:"to"`
	RelationType RelationType    `json:"relation_type"`
	Location     Location        `json:"location"`
}

// RelationType represents the type of relationship between symbols
type RelationType string

const (
	RelationCalls      RelationType = "calls"
	RelationInherits   RelationType = "inherits"
	RelationImplements RelationType = "implements"
	RelationImports    RelationType = "imports"
	RelationDefines    RelationType = "defines"
	RelationReferences RelationType = "references"
	RelationOverrides  RelationType = "overrides"
	RelationExtends    RelationType = "extends"
)

// NewSymbolExtractor creates a new symbol extractor
func NewSymbolExtractor(registry *LanguageRegistry) *SymbolExtractor {
	return &SymbolExtractor{
		registry: registry,
		parser:   registry.GetParser(),
	}
}

// NewSymbolIndex creates a new symbol index
func NewSymbolIndex() *SymbolIndex {
	return &SymbolIndex{
		symbols:       make(map[string][]*Symbol),
		symbolsByFile: make(map[string][]*Symbol),
		symbolsByType: make(map[SymbolKind][]*Symbol),
		references:    make(map[string][]Location),
		dependencies:  make(map[string][]string),
		scopes:        make(map[string]*ScopeTree),
		languages:     make(map[string]int),
		indexTime:     time.Now(),
	}
}

// ExtractSymbols extracts symbols from a file with enhanced analysis
func (se *SymbolExtractor) ExtractSymbols(filePath string, content []byte) (*ParseResult, error) {
	result, err := se.parser.ParseFile(filePath, content)
	if err != nil {
		return nil, err
	}

	// Enhance symbols with additional information
	if err := se.enhanceSymbols(result); err != nil {
		return nil, fmt.Errorf("failed to enhance symbols: %w", err)
	}

	// Build scope tree
	scopeTree, err := se.buildScopeTree(result)
	if err != nil {
		return nil, fmt.Errorf("failed to build scope tree: %w", err)
	}

	// Add scope information to result metadata
	if result.Tree != nil {
		// Store scope tree for later use
		se.storeScopeTree(filePath, scopeTree)
	}

	return result, nil
}

// ExtractSymbolsFromDirectory extracts symbols from all supported files in a directory
func (se *SymbolExtractor) ExtractSymbolsFromDirectory(ctx context.Context, dirPath string) (*SymbolIndex, error) {
	index := NewSymbolIndex()

	// Walk directory and collect files
	files, err := se.collectSupportedFiles(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to collect files: %w", err)
	}

	index.totalFiles = len(files)

	// Process files concurrently
	semaphore := make(chan struct{}, 10) // Limit concurrent processing
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		wg.Add(1)
		go func(filePath string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Process single file
			if err := se.processFile(filePath, index, &mu); err != nil {
				fmt.Printf("Warning: failed to process %s: %v\n", filePath, err)
			}
		}(file)
	}

	wg.Wait()

	// Post-process the index
	if err := se.postProcessIndex(index); err != nil {
		return nil, fmt.Errorf("failed to post-process index: %w", err)
	}

	return index, nil
}

// processFile processes a single file and updates the index
func (se *SymbolExtractor) processFile(filePath string, index *SymbolIndex, mu *sync.Mutex) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	result, err := se.ExtractSymbols(filePath, content)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	// Update index
	index.symbolsByFile[filePath] = result.Symbols
	index.totalSymbols += len(result.Symbols)
	index.languages[result.Language]++

	// Index symbols by name and type
	for _, symbol := range result.Symbols {
		index.symbols[symbol.Name] = append(index.symbols[symbol.Name], symbol)
		index.symbolsByType[symbol.Kind] = append(index.symbolsByType[symbol.Kind], symbol)
	}

	return nil
}

// enhanceSymbols adds additional metadata to symbols
func (se *SymbolExtractor) enhanceSymbols(result *ParseResult) error {
	for _, symbol := range result.Symbols {
		// Generate unique symbol ID
		symbol.generateID()

		// Infer additional properties based on context
		se.inferSymbolProperties(symbol, result)

		// Extract documentation from nearby comments
		se.extractDocumentation(symbol, result)

		// Determine visibility/accessibility
		se.determineVisibility(symbol, result)
	}

	return nil
}

// generateID generates a unique identifier for a symbol
func (s *Symbol) generateID() {
	s.ID = fmt.Sprintf("%s:%d:%d:%s:%s",
		s.FilePath, s.Line, s.Column, s.Kind, s.Name)
}

// inferSymbolProperties infers additional properties based on context and language
func (se *SymbolExtractor) inferSymbolProperties(symbol *Symbol, result *ParseResult) {
	// Language-specific property inference
	switch result.Language {
	case "go":
		se.inferGoProperties(symbol, result)
	case "python":
		se.inferPythonProperties(symbol, result)
	case "javascript", "typescript":
		se.inferJSProperties(symbol, result)
	case "rust":
		se.inferRustProperties(symbol, result)
	}
}

// inferGoProperties infers Go-specific properties
func (se *SymbolExtractor) inferGoProperties(symbol *Symbol, result *ParseResult) {
	// Check if symbol is exported (starts with uppercase)
	if len(symbol.Name) > 0 && strings.ToUpper(string(symbol.Name[0])) == string(symbol.Name[0]) {
		symbol.Modifiers = append(symbol.Modifiers, "public")
	} else {
		symbol.Modifiers = append(symbol.Modifiers, "private")
	}

	// Infer receiver type for methods
	if symbol.Kind == SymbolMethod {
		// Extract receiver information from signature
		if strings.Contains(symbol.Signature, "receiver") {
			symbol.Type = "method"
		}
	}
}

// inferPythonProperties infers Python-specific properties
func (se *SymbolExtractor) inferPythonProperties(symbol *Symbol, result *ParseResult) {
	// Check naming conventions
	name := symbol.Name

	if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
		symbol.Modifiers = append(symbol.Modifiers, "magic")
	} else if strings.HasPrefix(name, "__") {
		symbol.Modifiers = append(symbol.Modifiers, "private")
	} else if strings.HasPrefix(name, "_") {
		symbol.Modifiers = append(symbol.Modifiers, "protected")
	} else {
		symbol.Modifiers = append(symbol.Modifiers, "public")
	}

	// Check for decorators
	if symbol.Kind == SymbolFunction || symbol.Kind == SymbolMethod {
		// This would require more sophisticated parsing to detect decorators
		// For now, we'll leave this as a placeholder
	}
}

// inferJSProperties infers JavaScript/TypeScript-specific properties
func (se *SymbolExtractor) inferJSProperties(symbol *Symbol, result *ParseResult) {
	// Check naming conventions
	name := symbol.Name

	if strings.HasPrefix(name, "_") {
		symbol.Modifiers = append(symbol.Modifiers, "private")
	} else {
		symbol.Modifiers = append(symbol.Modifiers, "public")
	}

	// For TypeScript, we could extract more type information
	if result.Language == "typescript" {
		// This would be enhanced with actual TypeScript AST parsing
	}
}

// inferRustProperties infers Rust-specific properties
func (se *SymbolExtractor) inferRustProperties(symbol *Symbol, result *ParseResult) {
	// Rust has explicit visibility modifiers
	// This would be enhanced with actual parsing of pub/pub(crate)/etc.

	// For now, assume public if not explicitly marked
	if len(symbol.Modifiers) == 0 {
		symbol.Modifiers = append(symbol.Modifiers, "private")
	}
}

// extractDocumentation extracts documentation from comments
func (se *SymbolExtractor) extractDocumentation(symbol *Symbol, result *ParseResult) {
	// This would require parsing comments near the symbol
	// For now, we'll use any existing docstring from the tree-sitter extraction
	if symbol.DocString != "" {
		// Clean up the docstring
		symbol.DocString = strings.TrimSpace(symbol.DocString)
		symbol.DocString = strings.Trim(symbol.DocString, "/**/")
		symbol.DocString = strings.Trim(symbol.DocString, "\"\"\"")
		symbol.DocString = strings.Trim(symbol.DocString, "'''")
	}
}

// determineVisibility determines symbol visibility
func (se *SymbolExtractor) determineVisibility(symbol *Symbol, result *ParseResult) {
	// This was partially handled in language-specific inference
	// Additional logic could be added here for complex visibility rules
}

// buildScopeTree builds a hierarchical scope tree for a file
func (se *SymbolExtractor) buildScopeTree(result *ParseResult) (*ScopeTree, error) {
	scopeTree := &ScopeTree{
		FilePath: result.FilePath,
		Root: &ScopeNode{
			Name:    "file",
			Kind:    ScopeFile,
			Symbols: make([]*Symbol, 0),
			Children: make([]*ScopeNode, 0),
		},
	}

	// Group symbols by their line ranges to build hierarchy
	for _, symbol := range result.Symbols {
		se.insertSymbolIntoScope(scopeTree.Root, symbol)
	}

	return scopeTree, nil
}

// insertSymbolIntoScope inserts a symbol into the appropriate scope node
func (se *SymbolExtractor) insertSymbolIntoScope(scope *ScopeNode, symbol *Symbol) {
	// Simple insertion logic - can be enhanced with proper tree-sitter scope analysis

	// Check if symbol belongs in an existing child scope
	for _, child := range scope.Children {
		if symbol.Line >= child.StartLine && symbol.EndLine <= child.EndLine {
			se.insertSymbolIntoScope(child, symbol)
			return
		}
	}

	// Create new scope if this is a scope-creating symbol
	if se.isScopeCreatingSymbol(symbol) {
		newScope := &ScopeNode{
			Name:      symbol.Name,
			Kind:      se.symbolKindToScopeKind(symbol.Kind),
			StartLine: symbol.Line,
			EndLine:   symbol.EndLine,
			Symbols:   []*Symbol{symbol},
			Children:  make([]*ScopeNode, 0),
			Parent:    scope,
		}
		scope.Children = append(scope.Children, newScope)
	} else {
		// Add to current scope
		scope.Symbols = append(scope.Symbols, symbol)
	}
}

// isScopeCreatingSymbol determines if a symbol creates a new scope
func (se *SymbolExtractor) isScopeCreatingSymbol(symbol *Symbol) bool {
	switch symbol.Kind {
	case SymbolFunction, SymbolMethod, SymbolClass, SymbolInterface, SymbolStruct:
		return true
	default:
		return false
	}
}

// symbolKindToScopeKind converts a symbol kind to a scope kind
func (se *SymbolExtractor) symbolKindToScopeKind(kind SymbolKind) ScopeKind {
	switch kind {
	case SymbolFunction:
		return ScopeFunction
	case SymbolMethod:
		return ScopeMethod
	case SymbolClass:
		return ScopeClass
	default:
		return ScopeBlock
	}
}

// storeScopeTree stores a scope tree (placeholder for caching)
func (se *SymbolExtractor) storeScopeTree(filePath string, scopeTree *ScopeTree) {
	// This could be enhanced to store in a cache or database
}

// collectSupportedFiles collects all files that can be parsed
func (se *SymbolExtractor) collectSupportedFiles(dirPath string) ([]string, error) {
	var files []string
	extensions := se.parser.GetSupportedExtensions()

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if info.IsDir() {
			// Skip common directories
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "target" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file extension is supported
		ext := strings.ToLower(filepath.Ext(path))
		for _, supportedExt := range extensions {
			if ext == supportedExt {
				files = append(files, path)
				break
			}
		}

		return nil
	})

	return files, err
}

// postProcessIndex performs post-processing on the symbol index
func (se *SymbolExtractor) postProcessIndex(index *SymbolIndex) error {
	index.mu.Lock()
	defer index.mu.Unlock()

	// Sort symbols for consistent ordering
	for _, symbols := range index.symbols {
		sort.Slice(symbols, func(i, j int) bool {
			if symbols[i].FilePath != symbols[j].FilePath {
				return symbols[i].FilePath < symbols[j].FilePath
			}
			return symbols[i].Line < symbols[j].Line
		})
	}

	// Build cross-references and dependencies
	if err := se.buildCrossReferences(index); err != nil {
		return fmt.Errorf("failed to build cross-references: %w", err)
	}

	return nil
}

// buildCrossReferences builds symbol cross-references
func (se *SymbolExtractor) buildCrossReferences(index *SymbolIndex) error {
	// This is a simplified implementation
	// A full implementation would use tree-sitter queries to find actual references

	for filePath, symbols := range index.symbolsByFile {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		for _, symbol := range symbols {
			// Find references to this symbol in other files
			refs := se.findSymbolReferences(symbol, content, index)
			if len(refs) > 0 {
				index.references[symbol.ID] = refs
			}
		}
	}

	return nil
}

// findSymbolReferences finds references to a symbol
func (se *SymbolExtractor) findSymbolReferences(symbol *Symbol, content []byte, index *SymbolIndex) []Location {
	var references []Location

	// Simple text-based reference finding
	// This could be enhanced with proper tree-sitter query-based reference finding
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		col := strings.Index(line, symbol.Name)
		for col >= 0 {
			if se.isValidReference(line, col, symbol.Name) {
				references = append(references, Location{
					File:     symbol.FilePath,
					Line:     lineNum + 1,
					Column:   col + 1,
				})
			}

			// Look for more occurrences in the same line
			remaining := line[col+len(symbol.Name):]
			nextCol := strings.Index(remaining, symbol.Name)
			if nextCol >= 0 {
				col = col + len(symbol.Name) + nextCol
			} else {
				break
			}
		}
	}

	return references
}

// isValidReference validates if a text match is a valid symbol reference
func (se *SymbolExtractor) isValidReference(line string, col int, symbolName string) bool {
	// Check boundaries to ensure it's a complete word
	if col > 0 {
		prevChar := line[col-1]
		if isIdentifierChar(prevChar) {
			return false
		}
	}

	endCol := col + len(symbolName)
	if endCol < len(line) {
		nextChar := line[endCol]
		if isIdentifierChar(nextChar) {
			return false
		}
	}

	return true
}

// GetSymbolsByKind returns symbols filtered by kind
func (index *SymbolIndex) GetSymbolsByKind(kind SymbolKind) []*Symbol {
	index.mu.RLock()
	defer index.mu.RUnlock()

	return index.symbolsByType[kind]
}

// GetSymbolsByName returns symbols with a specific name
func (index *SymbolIndex) GetSymbolsByName(name string) []*Symbol {
	index.mu.RLock()
	defer index.mu.RUnlock()

	return index.symbols[name]
}

// GetSymbolsInFile returns all symbols in a specific file
func (index *SymbolIndex) GetSymbolsInFile(filePath string) []*Symbol {
	index.mu.RLock()
	defer index.mu.RUnlock()

	return index.symbolsByFile[filePath]
}

// GetReferences returns references for a symbol
func (index *SymbolIndex) GetReferences(symbolID string) []Location {
	index.mu.RLock()
	defer index.mu.RUnlock()

	return index.references[symbolID]
}

// GetStats returns index statistics
func (index *SymbolIndex) GetStats() map[string]interface{} {
	index.mu.RLock()
	defer index.mu.RUnlock()

	return map[string]interface{}{
		"total_symbols":  index.totalSymbols,
		"total_files":    index.totalFiles,
		"languages":      index.languages,
		"indexed_at":     index.indexTime,
		"unique_names":   len(index.symbols),
		"symbol_types":   len(index.symbolsByType),
	}
}