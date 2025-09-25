package index

import (
	"context"
	"fmt"
	"os"
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

// Close releases resources used by the parser
func (p *TreeSitterSymbolParser) Close() error {
	if p.registry != nil {
		return p.registry.Close()
	}
	return nil
}