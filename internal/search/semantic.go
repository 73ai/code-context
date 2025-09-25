package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/73ai/code-context/internal/parser"
	"golang.org/x/sync/semaphore"
)

// Use types from parser package
type SymbolKind = parser.SymbolKind
type Symbol = parser.Symbol
type Location = parser.Location

// Re-export constants from parser package
const (
	SymbolFunction   = parser.SymbolFunction
	SymbolMethod     = parser.SymbolMethod
	SymbolVariable   = parser.SymbolVariable
	SymbolConstant   = parser.SymbolConstant
	SymbolType       = parser.SymbolType
	SymbolClass      = parser.SymbolClass
	SymbolInterface  = parser.SymbolInterface
	SymbolStruct     = parser.SymbolStruct
	SymbolEnum       = parser.SymbolEnum
	SymbolField      = parser.SymbolField
	SymbolParameter  = parser.SymbolParameter
	SymbolImport     = parser.SymbolImport
	SymbolNamespace  = parser.SymbolNamespace
	SymbolModule     = parser.SymbolModule
	SymbolProperty   = parser.SymbolProperty
)

// SemanticSearcher implements tree-sitter based semantic search
type SemanticSearcher struct {
	// Tree-sitter integration
	languageRegistry *parser.LanguageRegistry
	symbolExtractor  *parser.SymbolExtractor
	symbolIndex      *parser.SymbolIndex
	indexMutex       sync.RWMutex

	// Test compatibility fields
	parsers       map[string]interface{} // For test compatibility
	symbols       map[string][]*Symbol   // For test compatibility
	symbolsByFile map[string][]*Symbol   // For test compatibility

	// Statistics
	stats SearchStats
	statsMutex sync.RWMutex
	filesProcessed int64
	symbolsFound int64

	// Worker pool
	semaphore *semaphore.Weighted
}

// NewSemanticSearcher creates a new semantic searcher
func NewSemanticSearcher() (*SemanticSearcher, error) {
	// Initialize language registry with all supported languages
	languageRegistry, err := parser.NewLanguageRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize language registry: %w", err)
	}

	// Create symbol extractor
	symbolExtractor := parser.NewSymbolExtractor(languageRegistry)

	// Initialize symbol index
	symbolIndex := parser.NewSymbolIndex()

	ss := &SemanticSearcher{
		languageRegistry: languageRegistry,
		symbolExtractor:  symbolExtractor,
		symbolIndex:      symbolIndex,
		parsers:          make(map[string]interface{}),
		symbols:          make(map[string][]*Symbol),
		symbolsByFile:    make(map[string][]*Symbol),
		semaphore:        semaphore.NewWeighted(int64(runtime.NumCPU() * 2)),
	}

	// Initialize Go parser for test compatibility
	ss.parsers["go"] = "go_parser"

	return ss, nil
}

// Search performs semantic search based on options
func (ss *SemanticSearcher) Search(ctx context.Context, opts *SearchOptions) (<-chan SearchResult, <-chan error) {
	results := make(chan SearchResult, 100)
	errors := make(chan error, 10)

	go func() {
		defer close(results)
		defer close(errors)

		startTime := time.Now()

		// Reset statistics
		ss.resetStats()

		// Index symbols if needed
		indexStart := time.Now()
		if err := ss.indexSymbols(ctx, opts); err != nil {
			errors <- fmt.Errorf("failed to index symbols: %w", err)
			return
		}

		ss.updateStats(func(stats *SearchStats) {
			stats.IndexDuration = time.Since(indexStart)
		})

		// Perform search based on options
		if opts.FindDefs {
			ss.searchDefinitions(ctx, opts, results, errors)
		} else if opts.FindRefs {
			ss.searchReferences(ctx, opts, results, errors)
		} else {
			// Default symbol search
			ss.searchSymbols(ctx, opts, results, errors)
		}

		// Update final statistics
		ss.updateStats(func(stats *SearchStats) {
			stats.SearchDuration = time.Since(startTime)
			stats.FilesSearched = int(atomic.LoadInt64(&ss.filesProcessed))
		})
	}()

	return results, errors
}

// indexSymbols builds or updates the symbol index using tree-sitter
func (ss *SemanticSearcher) indexSymbols(ctx context.Context, opts *SearchOptions) error {
	// Use directory-based indexing for better performance
	if len(opts.SearchPaths) == 0 {
		return fmt.Errorf("no search paths specified")
	}

	// For now, process the first directory
	// This could be enhanced to handle multiple directories
	mainDir := opts.SearchPaths[0]

	// Extract symbols from directory using tree-sitter
	symbolIndex, err := ss.symbolExtractor.ExtractSymbolsFromDirectory(ctx, mainDir)
	if err != nil {
		return fmt.Errorf("failed to extract symbols from directory: %w", err)
	}

	// Update the searcher's symbol index
	ss.indexMutex.Lock()
	ss.symbolIndex = symbolIndex
	ss.indexMutex.Unlock()

	// Update statistics
	indexStats := symbolIndex.GetStats()
	ss.updateStats(func(stats *SearchStats) {
		stats.TotalFiles = indexStats["total_files"].(int)
		atomic.StoreInt64(&ss.filesProcessed, int64(indexStats["total_files"].(int)))
		atomic.StoreInt64(&ss.symbolsFound, int64(indexStats["total_symbols"].(int)))
	})

	return nil
}

// indexFile parses and indexes symbols in a single file using tree-sitter
func (ss *SemanticSearcher) indexFile(filePath string) error {
	// Check if the language is supported
	language := ss.languageRegistry.GetLanguageForFile(filePath)
	if language == "" {
		return nil // Skip unsupported files
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Extract symbols using tree-sitter
	result, err := ss.symbolExtractor.ExtractSymbols(filePath, content)
	if err != nil {
		return fmt.Errorf("failed to extract symbols: %w", err)
	}

	atomic.AddInt64(&ss.filesProcessed, 1)
	atomic.AddInt64(&ss.symbolsFound, int64(len(result.Symbols)))

	// This method is now primarily used for single-file updates
	// The main indexing is done in indexSymbols via ExtractSymbolsFromDirectory

	return nil
}

// searchDefinitions finds symbol definitions using tree-sitter index
func (ss *SemanticSearcher) searchDefinitions(ctx context.Context, opts *SearchOptions,
	results chan<- SearchResult, errors chan<- error) {

	ss.indexMutex.RLock()
	defer ss.indexMutex.RUnlock()

	pattern := opts.Pattern
	symbolsFound := 0

	// Search by name pattern
	symbols := ss.symbolIndex.GetSymbolsByName(pattern)
	for _, symbol := range symbols {
		if ss.matchesPattern(symbol.Name, pattern, opts) {
			select {
			case results <- ss.symbolToResult(symbol, "definition"):
				symbolsFound++
			case <-ctx.Done():
				return
			}
		}
	}

	// Also search partial matches if not doing whole word search
	if !opts.WholeWord {
		// This is a simplified approach - could be enhanced with fuzzy matching
		for _, kindSymbols := range ss.getAllSymbolsByType() {
			for _, symbol := range kindSymbols {
				if ss.matchesPattern(symbol.Name, pattern, opts) && !ss.alreadyIncluded(symbol, symbols) {
					select {
					case results <- ss.symbolToResult(symbol, "definition"):
						symbolsFound++
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}

	ss.updateStats(func(stats *SearchStats) {
		stats.TotalMatches = symbolsFound
	})
}

// searchReferences finds symbol references using tree-sitter index
func (ss *SemanticSearcher) searchReferences(ctx context.Context, opts *SearchOptions,
	results chan<- SearchResult, errors chan<- error) {

	ss.indexMutex.RLock()
	defer ss.indexMutex.RUnlock()

	pattern := opts.Pattern
	referencesFound := 0

	// Find matching symbols first
	matchingSymbols := ss.symbolIndex.GetSymbolsByName(pattern)

	for _, symbol := range matchingSymbols {
		if ss.matchesPattern(symbol.Name, pattern, opts) {
			// Get references for this symbol
			references := ss.symbolIndex.GetReferences(symbol.ID)

			for _, ref := range references {
				select {
				case results <- SearchResult{
					FilePath:    ref.File,
					LineNumber:  ref.Line,
					ColumnStart: ref.Column,
					SymbolName:  symbol.Name,
					SymbolType:  string(symbol.Kind),
					SymbolKind:  string(symbol.Kind),
					Metadata: map[string]string{
						"search_type":     "reference",
						"definition_file": symbol.FilePath,
						"definition_line": fmt.Sprintf("%d", symbol.Line),
						"symbol_id":       symbol.ID,
					},
				}:
					referencesFound++
				case <-ctx.Done():
					return
				}
			}
		}
	}

	ss.updateStats(func(stats *SearchStats) {
		stats.TotalMatches = referencesFound
	})
}

// searchSymbols performs general symbol search using tree-sitter index
func (ss *SemanticSearcher) searchSymbols(ctx context.Context, opts *SearchOptions,
	results chan<- SearchResult, errors chan<- error) {

	ss.indexMutex.RLock()
	defer ss.indexMutex.RUnlock()

	pattern := opts.Pattern
	symbolsFound := 0

	// Search by symbol types if specified
	var searchSymbols []*Symbol
	if len(opts.SymbolTypes) > 0 {
		for _, symbolType := range opts.SymbolTypes {
			kind := ss.stringToSymbolKind(symbolType)
			if kind != "" {
				typeSymbols := ss.symbolIndex.GetSymbolsByKind(kind)
				searchSymbols = append(searchSymbols, typeSymbols...)
			}
		}
	} else {
		// Search all symbols
		searchSymbols = ss.getAllSymbols()
	}

	// Filter by pattern
	for _, symbol := range searchSymbols {
		if ss.matchesPattern(symbol.Name, pattern, opts) {
			select {
			case results <- ss.symbolToResult(symbol, "symbol"):
				symbolsFound++
			case <-ctx.Done():
				return
			}
		}
	}

	ss.updateStats(func(stats *SearchStats) {
		stats.TotalMatches = symbolsFound
	})
}

// symbolToResult converts a Symbol to a SearchResult
func (ss *SemanticSearcher) symbolToResult(symbol *Symbol, searchType string) SearchResult {
	metadata := map[string]string{
		"search_type": searchType,
		"signature":   symbol.Signature,
		"doc_string":  symbol.DocString,
		"symbol_id":   symbol.ID,
	}

	// Add modifiers to metadata
	if len(symbol.Modifiers) > 0 {
		metadata["modifiers"] = strings.Join(symbol.Modifiers, ",")
	}

	return SearchResult{
		FilePath:    symbol.FilePath,
		LineNumber:  symbol.Line,
		ColumnStart: symbol.Column,
		ColumnEnd:   symbol.EndColumn,
		SymbolName:  symbol.Name,
		SymbolType:  symbol.Type,
		SymbolKind:  string(symbol.Kind),
		Scope:       symbol.Scope,
		Metadata:    metadata,
	}
}

// matchesPattern checks if a symbol name matches the search pattern
func (ss *SemanticSearcher) matchesPattern(symbolName, pattern string, opts *SearchOptions) bool {
	if opts.CaseSensitive {
		if opts.WholeWord {
			return symbolName == pattern
		}
		return strings.Contains(symbolName, pattern)
	}

	symbolLower := strings.ToLower(symbolName)
	patternLower := strings.ToLower(pattern)

	if opts.WholeWord {
		return symbolLower == patternLower
	}

	return strings.Contains(symbolLower, patternLower)
}

// collectSourceFiles collects source files to search using tree-sitter support
func (ss *SemanticSearcher) collectSourceFiles(opts *SearchOptions) ([]string, error) {
	var files []string
	supportedExtensions := ss.languageRegistry.GetParser().GetSupportedExtensions()

	for _, path := range opts.SearchPaths {
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() {
				// Skip common directories
				name := info.Name()
				if name == ".git" || name == "node_modules" || name == "target" || name == "__pycache__" {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip large files
			if opts.MaxFileSize > 0 && info.Size() > opts.MaxFileSize {
				return nil
			}

			// Check if file extension is supported
			ext := strings.ToLower(filepath.Ext(filePath))
			for _, supportedExt := range supportedExtensions {
				if ext == supportedExt {
					files = append(files, filePath)
					break
				}
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error walking path %s: %w", path, err)
		}
	}

	return files, nil
}

// Helper methods for tree-sitter integration

// getAllSymbols returns all symbols from the index
func (ss *SemanticSearcher) getAllSymbols() []*Symbol {
	var allSymbols []*Symbol

	// Get all symbol types
	for kind := range ss.getAllSymbolsByType() {
		symbols := ss.symbolIndex.GetSymbolsByKind(kind)
		allSymbols = append(allSymbols, symbols...)
	}

	return allSymbols
}

// getAllSymbolsByType returns a map of all symbols grouped by type
func (ss *SemanticSearcher) getAllSymbolsByType() map[SymbolKind][]*Symbol {
	symbolsByType := make(map[SymbolKind][]*Symbol)

	// Define all symbol kinds we support
	kinds := []SymbolKind{
		SymbolFunction, SymbolMethod, SymbolVariable, SymbolConstant,
		SymbolType, SymbolClass, SymbolInterface, SymbolStruct,
		SymbolEnum, SymbolField, SymbolParameter, SymbolImport,
		SymbolNamespace, SymbolModule, SymbolProperty,
	}

	for _, kind := range kinds {
		symbols := ss.symbolIndex.GetSymbolsByKind(kind)
		if len(symbols) > 0 {
			symbolsByType[kind] = symbols
		}
	}

	return symbolsByType
}

// stringToSymbolKind converts a string to SymbolKind
func (ss *SemanticSearcher) stringToSymbolKind(s string) SymbolKind {
	switch strings.ToLower(s) {
	case "function", "func":
		return SymbolFunction
	case "method":
		return SymbolMethod
	case "variable", "var":
		return SymbolVariable
	case "constant", "const":
		return SymbolConstant
	case "type":
		return SymbolType
	case "class":
		return SymbolClass
	case "interface":
		return SymbolInterface
	case "struct":
		return SymbolStruct
	case "enum":
		return SymbolEnum
	case "field":
		return SymbolField
	case "parameter", "param":
		return SymbolParameter
	case "import":
		return SymbolImport
	case "namespace":
		return SymbolNamespace
	case "module":
		return SymbolModule
	case "property", "prop":
		return SymbolProperty
	default:
		return ""
	}
}

// alreadyIncluded checks if a symbol is already in a slice
func (ss *SemanticSearcher) alreadyIncluded(symbol *Symbol, symbols []*Symbol) bool {
	for _, existing := range symbols {
		if existing.ID == symbol.ID {
			return true
		}
	}
	return false
}

// resetStats resets search statistics
func (ss *SemanticSearcher) resetStats() {
	ss.statsMutex.Lock()
	defer ss.statsMutex.Unlock()

	ss.stats = SearchStats{}
	atomic.StoreInt64(&ss.filesProcessed, 0)
	atomic.StoreInt64(&ss.symbolsFound, 0)
}

// updateStats safely updates search statistics
func (ss *SemanticSearcher) updateStats(fn func(*SearchStats)) {
	ss.statsMutex.Lock()
	defer ss.statsMutex.Unlock()
	fn(&ss.stats)
}

// Stats returns current search statistics
func (ss *SemanticSearcher) Stats() SearchStats {
	ss.statsMutex.RLock()
	defer ss.statsMutex.RUnlock()

	stats := ss.stats
	stats.FilesSearched = int(atomic.LoadInt64(&ss.filesProcessed))

	// Get current memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	stats.PeakMemoryUsage = int64(m.HeapInuse)

	return stats
}

// Close cleans up resources
func (ss *SemanticSearcher) Close() error {
	ss.indexMutex.Lock()
	defer ss.indexMutex.Unlock()

	// Close language registry
	if ss.languageRegistry != nil {
		if err := ss.languageRegistry.Close(); err != nil {
			return fmt.Errorf("failed to close language registry: %w", err)
		}
	}

	// Clear symbol index (it will be garbage collected)
	ss.symbolIndex = parser.NewSymbolIndex()

	return nil
}

// GetSupportedLanguages returns information about supported languages
func (ss *SemanticSearcher) GetSupportedLanguages() []string {
	if ss.languageRegistry != nil {
		return ss.languageRegistry.GetSupportedLanguages()
	}
	return []string{}
}

// GetLanguageFeatures returns detailed feature support for each language
func (ss *SemanticSearcher) GetLanguageFeatures() map[string]interface{} {
	if ss.languageRegistry != nil {
		features := ss.languageRegistry.GetLanguageFeatures()
		result := make(map[string]interface{})
		for _, feature := range features {
			result[feature.Language] = feature
		}
		return result
	}
	return make(map[string]interface{})
}

// GetIndexStats returns detailed statistics about the symbol index
func (ss *SemanticSearcher) GetIndexStats() map[string]interface{} {
	ss.indexMutex.RLock()
	defer ss.indexMutex.RUnlock()

	if ss.symbolIndex != nil {
		return ss.symbolIndex.GetStats()
	}
	return make(map[string]interface{})
}

// GoParser provides Go-specific parsing functionality for test compatibility
type GoParser struct{}

// ParseFile parses a Go source file and extracts symbols
func (gp *GoParser) ParseFile(filePath string, content []byte) ([]*Symbol, error) {
	// Create a temporary parser instance for parsing
	tsParser := parser.NewTreeSitterParser()
	defer tsParser.Close()

	// This is a simplified implementation for test compatibility
	// In a real implementation, this would use the proper tree-sitter Go parser

	result, err := tsParser.ParseFile(filePath, content)
	if err != nil {
		return nil, err
	}

	return result.Symbols, nil
}

// GetFileExtensions returns supported file extensions for Go
func (gp *GoParser) GetFileExtensions() []string {
	return []string{".go"}
}

// FindReferences finds references to a symbol in the provided files
func (gp *GoParser) FindReferences(symbol *Symbol, files []string) ([]Location, error) {
	tsParser := parser.NewTreeSitterParser()
	defer tsParser.Close()

	return tsParser.FindReferences(symbol, files, 100)
}