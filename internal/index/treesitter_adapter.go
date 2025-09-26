package index

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/73ai/code-context/internal/parser"
)

// TreeSitterSymbolParser adapts the tree-sitter parser to the SymbolParser interface
type TreeSitterSymbolParser struct {
	extractor *parser.SymbolExtractor
	registry  *parser.LanguageRegistry
}

// NewTreeSitterSymbolParser creates a new TreeSitterSymbolParser
func NewTreeSitterSymbolParser() (*TreeSitterSymbolParser, error) {
	registry, err := parser.NewLanguageRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to create language registry: %w", err)
	}

	extractor := parser.NewSymbolExtractor(registry)
	if extractor == nil {
		registry.Close()
		return nil, fmt.Errorf("failed to create symbol extractor")
	}

	return &TreeSitterSymbolParser{
		extractor: extractor,
		registry:  registry,
	}, nil
}

// ParseFile extracts symbols from a source file using tree-sitter
func (p *TreeSitterSymbolParser) ParseFile(ctx context.Context, filePath string) ([]SymbolInfo, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Use the tree-sitter extractor to extract symbols
	result, err := p.extractor.ExtractSymbols(filePath, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	// Convert parser.Symbol to index.SymbolInfo
	symbols := make([]SymbolInfo, 0, len(result.Symbols))
	for _, sym := range result.Symbols {
		symbolInfo := SymbolInfo{
			ID:          sym.ID,
			Name:        sym.Name,
			Type:        sym.Type,
			Kind:        string(sym.Kind),
			FilePath:    sym.FilePath,
			StartLine:   sym.Line,
			EndLine:     sym.EndLine,
			StartCol:    sym.Column,
			EndCol:      sym.EndColumn,
			Signature:   sym.Signature,
			DocString:   sym.DocString,
			Tags:        sym.Tags,
			Properties:  sym.Properties,
			LastUpdated: time.Now(),
		}
		symbols = append(symbols, symbolInfo)
	}

	return symbols, nil
}

// SupportedLanguages returns the list of supported programming languages
func (p *TreeSitterSymbolParser) SupportedLanguages() []string {
	return p.registry.GetSupportedLanguages()
}

// IsSupported checks if the parser supports the given file
func (p *TreeSitterSymbolParser) IsSupported(filePath string) bool {
	language := p.registry.GetLanguageForFile(filePath)
	return language != ""
}

// ParseReferences extracts references from a source file
// This uses the existing FindReferences functionality from the tree-sitter parser
func (p *TreeSitterSymbolParser) ParseReferences(ctx context.Context, filePath string, symbolIndex SymbolIndex) ([]Reference, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Parse the file using tree-sitter (not needed for reference finding, but kept for consistency)
	_, err = p.extractor.ExtractSymbols(filePath, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	var references []Reference

	// For each symbol in the symbolIndex, find references in this file
	for _, symbol := range symbolIndex {
		// Convert SymbolInfo to parser.Symbol for FindReferences
		parserSymbol := &parser.Symbol{
			Name:     symbol.Name,
			Kind:     parser.SymbolKind(symbol.Kind),
			FilePath: symbol.FilePath,
			Line:     symbol.StartLine,
			Column:   symbol.StartCol,
			EndLine:  symbol.EndLine,
			EndColumn: symbol.EndCol,
		}

		// Use the tree-sitter parser's FindReferences functionality
		locations := p.findReferencesInContent(parserSymbol, content, filePath)

		// Convert parser.Location to index.Reference
		for _, loc := range locations {
			ref := Reference{
				SymbolID: symbol.ID,
				FilePath: loc.File,
				Line:     loc.Line,
				Column:   loc.Column,
				Kind:     "reference", // Default kind, could be enhanced to detect call vs reference
				Context:  "", // Could be enhanced to extract surrounding context
			}
			references = append(references, ref)
		}
	}

	return references, nil
}

// findReferencesInContent finds references to a symbol in file content
// This is adapted from the TreeSitterParser.findReferencesInContent method
func (p *TreeSitterSymbolParser) findReferencesInContent(symbol *parser.Symbol, content []byte, filePath string) []parser.Location {
	var references []parser.Location

	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		// Simple string matching for now - can be enhanced with proper tree-sitter queries
		col := strings.Index(line, symbol.Name)
		for col >= 0 {
			// Basic heuristic to avoid false positives
			if p.isValidReference(line, col, symbol.Name) {
				references = append(references, parser.Location{
					File:     filePath,
					Line:     lineNum + 1, // 1-based line numbers
					Column:   col + 1,     // 1-based column numbers
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

// isValidReference performs basic validation to reduce false positives
func (p *TreeSitterSymbolParser) isValidReference(line string, col int, symbolName string) bool {
	// Check boundaries - ensure it's a complete word
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

// isIdentifierChar checks if a character can be part of an identifier
func isIdentifierChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		   (ch >= 'A' && ch <= 'Z') ||
		   (ch >= '0' && ch <= '9') ||
		   ch == '_' || ch == '$'
}

// SupportsReferences indicates if this parser can extract references
func (p *TreeSitterSymbolParser) SupportsReferences() bool {
	return true
}

// Close releases resources used by the parser
func (p *TreeSitterSymbolParser) Close() error {
	if p.registry != nil {
		return p.registry.Close()
	}
	return nil
}