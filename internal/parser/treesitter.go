package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// SymbolKind represents different types of code symbols
type SymbolKind string

const (
	SymbolKindFunction    SymbolKind = "function"
	SymbolKindVariable    SymbolKind = "variable"
	SymbolKindType        SymbolKind = "type"
	SymbolKindClass       SymbolKind = "class"
	SymbolKindInterface   SymbolKind = "interface"
	SymbolKindImport      SymbolKind = "import"
	SymbolKindConstant    SymbolKind = "constant"
	SymbolKindField       SymbolKind = "field"
	SymbolKindMethod      SymbolKind = "method"
	SymbolKindNamespace   SymbolKind = "namespace"
	SymbolKindProperty    SymbolKind = "property"
	SymbolKindEnum        SymbolKind = "enum"
	SymbolKindStruct      SymbolKind = "struct"
	SymbolKindParameter   SymbolKind = "parameter"
	SymbolKindModule      SymbolKind = "module"

	// Backward compatibility aliases (without "Kind" suffix)
	SymbolFunction  = SymbolKindFunction
	SymbolVariable  = SymbolKindVariable
	SymbolType      = SymbolKindType
	SymbolClass     = SymbolKindClass
	SymbolInterface = SymbolKindInterface
	SymbolImport    = SymbolKindImport
	SymbolConstant  = SymbolKindConstant
	SymbolField     = SymbolKindField
	SymbolMethod    = SymbolKindMethod
	SymbolNamespace = SymbolKindNamespace
	SymbolProperty  = SymbolKindProperty
	SymbolEnum      = SymbolKindEnum
	SymbolStruct    = SymbolKindStruct
	SymbolParameter = SymbolKindParameter
	SymbolModule    = SymbolKindModule
)

// Location represents a position in source code
type Location struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	EndLine   int `json:"end_line,omitempty"`
	EndColumn int `json:"end_column,omitempty"`
}

// Symbol represents a code symbol with its metadata
type Symbol struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Kind        SymbolKind        `json:"kind"`
	Location    Location          `json:"location"`
	FilePath    string            `json:"file_path"`
	Line        int               `json:"line"`
	Column      int               `json:"column"`
	EndLine     int               `json:"end_line,omitempty"`
	EndColumn   int               `json:"end_column,omitempty"`
	Type        string            `json:"type,omitempty"`
	Signature   string            `json:"signature,omitempty"`
	DocString   string            `json:"doc_string,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
	Modifiers   []string          `json:"modifiers,omitempty"`
	Language    string            `json:"language"`
	Scope       string            `json:"scope,omitempty"`
}

// TreeSitterParser provides tree-sitter based parsing functionality
type TreeSitterParser struct {
	// Language parsers by file extension
	parsers map[string]*LanguageConfig
	mu      sync.RWMutex
}

// LanguageConfig holds configuration for a specific language parser
type LanguageConfig struct {
	Language   *sitter.Language
	Extensions []string
	Queries    *QuerySet
	Name       string
}

// QuerySet contains compiled queries for a language
type QuerySet struct {
	Functions    *sitter.Query
	Variables    *sitter.Query
	Types        *sitter.Query
	Classes      *sitter.Query
	Imports      *sitter.Query
	Exports      *sitter.Query
	Comments     *sitter.Query
	Definitions  *sitter.Query
	References   *sitter.Query
}

// ParseResult contains the result of parsing a file
type ParseResult struct {
	Symbols    []*Symbol
	Tree       *sitter.Tree
	Language   string
	FilePath   string
	Errors     []ParseError
}

// ParseError represents a parsing error
type ParseError struct {
	Line    int
	Column  int
	Message string
}

// NewTreeSitterParser creates a new tree-sitter parser instance
func NewTreeSitterParser() *TreeSitterParser {
	return &TreeSitterParser{
		parsers: make(map[string]*LanguageConfig),
	}
}

// RegisterLanguage registers a language configuration
func (p *TreeSitterParser) RegisterLanguage(config *LanguageConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Register by all extensions
	for _, ext := range config.Extensions {
		p.parsers[strings.ToLower(ext)] = config
	}

	return nil
}

// ParseFile parses a file and extracts symbols using tree-sitter
func (p *TreeSitterParser) ParseFile(filePath string, content []byte) (*ParseResult, error) {
	config := p.getLanguageConfig(filePath)
	if config == nil {
		return nil, fmt.Errorf("unsupported file type: %s", filepath.Ext(filePath))
	}

	// Create parser
	parser := sitter.NewParser()
	defer parser.Close()

	if config.Language != nil {
		if err := parser.SetLanguage(config.Language); err != nil {
			return nil, fmt.Errorf("failed to set language: %w", err)
		}
	}

	// Parse the content
	tree := parser.Parse(content, nil)
	if tree == nil {
		// If tree-sitter parsing fails, fall back to basic parsing
		return p.fallbackParse(filePath, content, config.Name)
	}

	result := &ParseResult{
		Tree:     tree,
		Language: config.Name,
		FilePath: filePath,
		Symbols:  make([]*Symbol, 0),
	}

	// Extract symbols using direct AST walking (more reliable than queries)
	symbols := p.extractSymbolsDirectly(tree, content, filePath, config.Name)
	result.Symbols = symbols

	return result, nil
}

// ParseFileAsync parses a file asynchronously
func (p *TreeSitterParser) ParseFileAsync(ctx context.Context, filePath string, content []byte) <-chan *ParseResult {
	results := make(chan *ParseResult, 1)

	go func() {
		defer close(results)

		select {
		case <-ctx.Done():
			return
		default:
		}

		result, err := p.ParseFile(filePath, content)
		if err != nil {
			result = &ParseResult{
				FilePath: filePath,
				Errors: []ParseError{{
					Message: err.Error(),
				}},
			}
		}

		select {
		case results <- result:
		case <-ctx.Done():
		}
	}()

	return results
}

// extractSymbolsDirectly extracts symbols by walking the AST directly (more reliable than queries)
func (p *TreeSitterParser) extractSymbolsDirectly(tree *sitter.Tree, content []byte, filePath, language string) []*Symbol {
	var symbols []*Symbol
	rootNode := tree.RootNode()

	// Walk the tree and extract symbols
	var walkNodes func(*sitter.Node)
	walkNodes = func(node *sitter.Node) {
		if node == nil {
			return
		}

		symbol := p.nodeToSymbol(node, content, filePath, language)
		if symbol != nil {
			symbols = append(symbols, symbol)
		}

		// Recursively walk children
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil {
				walkNodes(child)
			}
		}
	}

	walkNodes(rootNode)
	return symbols
}

// nodeToSymbol converts a tree-sitter node to a Symbol if it represents a symbol
func (p *TreeSitterParser) nodeToSymbol(node *sitter.Node, content []byte, filePath, language string) *Symbol {
	nodeType := node.Kind()

	switch language {
	case "go":
		return p.goNodeToSymbol(node, nodeType, content, filePath)
	case "python":
		return p.pythonNodeToSymbol(node, nodeType, content, filePath)
	case "javascript":
		return p.jsNodeToSymbol(node, nodeType, content, filePath)
	case "typescript":
		return p.typescriptNodeToSymbol(node, nodeType, content, filePath)
	case "rust":
		return p.rustNodeToSymbol(node, nodeType, content, filePath)
	default:
		return nil
	}
}

// goNodeToSymbol converts Go-specific nodes to symbols
func (p *TreeSitterParser) goNodeToSymbol(node *sitter.Node, nodeType string, content []byte, filePath string) *Symbol {
	switch nodeType {
	case "function_declaration":
		// Check if this is a method (has receiver)
		if p.hasReceiver(node) {
			return p.extractGoFunction(node, content, filePath, SymbolMethod)
		}
		return p.extractGoFunction(node, content, filePath, SymbolFunction)
	case "method_declaration":
		return p.extractGoFunction(node, content, filePath, SymbolMethod)
	case "type_declaration":
		return p.extractGoType(node, content, filePath)
	case "var_declaration":
		return p.extractGoVariable(node, content, filePath)
	case "const_declaration":
		return p.extractGoConstant(node, content, filePath)
	default:
		return nil
	}
}

// hasReceiver checks if a function declaration has a receiver (i.e., is a method)
func (p *TreeSitterParser) hasReceiver(node *sitter.Node) bool {
	if node == nil {
		return false
	}

	// Look for parameter_list child
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "parameter_list" {
			// Check if this parameter list has parentheses with a receiver
			childText := p.getNodeText(child, nil)
			// A method receiver will be like (ts *TestStruct) before the function name
			if strings.Contains(childText, "*") || strings.Contains(childText, " ") {
				return true
			}
		}
	}
	return false
}

// extractPrecedingComment looks for comment nodes that precede the given node
func (p *TreeSitterParser) extractPrecedingComment(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Get the line number of the current node
	startLine := int(node.StartPosition().Row)

	// Look through the content lines to find comments directly before this node
	lines := strings.Split(string(content), "\n")

	// Look for comments on the line immediately preceding the function
	if startLine > 0 {
		prevLine := lines[startLine-1]
		prevLine = strings.TrimSpace(prevLine)
		if strings.HasPrefix(prevLine, "//") {
			comment := strings.TrimSpace(prevLine[2:])
			return comment
		}
	}

	return ""
}

// pythonNodeToSymbol converts Python-specific nodes to symbols
func (p *TreeSitterParser) pythonNodeToSymbol(node *sitter.Node, nodeType string, content []byte, filePath string) *Symbol {
	switch nodeType {
	case "function_definition":
		return p.extractPythonFunction(node, content, filePath)
	case "class_definition":
		return p.extractPythonClass(node, content, filePath)
	default:
		return nil
	}
}

// jsNodeToSymbol converts JavaScript-specific nodes to symbols
func (p *TreeSitterParser) jsNodeToSymbol(node *sitter.Node, nodeType string, content []byte, filePath string) *Symbol {
	switch nodeType {
	case "function_declaration":
		return p.extractJSFunction(node, content, filePath)
	case "class_declaration":
		return p.extractJSClass(node, content, filePath)
	default:
		return nil
	}
}

// rustNodeToSymbol converts Rust-specific nodes to symbols
func (p *TreeSitterParser) rustNodeToSymbol(node *sitter.Node, nodeType string, content []byte, filePath string) *Symbol {
	switch nodeType {
	case "function_item":
		return p.extractRustFunction(node, content, filePath)
	case "struct_item":
		return p.extractRustStruct(node, content, filePath)
	default:
		return nil
	}
}

// Helper functions for extracting specific symbol types

func (p *TreeSitterParser) extractGoFunction(node *sitter.Node, content []byte, filePath string, kind SymbolKind) *Symbol {
	// Find the function name
	var nameNode *sitter.Node

	if kind == SymbolMethod {
		// For methods, look for field_identifier (method name)
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "field_identifier" {
				nameNode = child
				break
			}
		}
	} else {
		// For regular functions, look for identifier
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "identifier" {
				nameNode = child
				break
			}
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	// Extract documentation comment (look for preceding comment)
	docString := p.extractPrecedingComment(node, content)

	return &Symbol{
		Name:      name,
		Kind:      kind,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "go",
		DocString: docString,
	}
}

func (p *TreeSitterParser) extractGoType(node *sitter.Node, content []byte, filePath string) *Symbol {
	// Find type_spec child, then identifier within it
	var nameNode *sitter.Node
	var typeDefNode *sitter.Node

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "type_spec" {
			// Look for identifier and type definition in type_spec
			for j := uint(0); j < child.ChildCount(); j++ {
				grandChild := child.Child(j)
				if grandChild != nil {
					if grandChild.Kind() == "type_identifier" {
						nameNode = grandChild
					} else if grandChild.Kind() == "struct_type" {
						typeDefNode = grandChild
					} else if grandChild.Kind() == "interface_type" {
						typeDefNode = grandChild
					}
				}
			}
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	// Determine the specific symbol kind based on the type definition
	var symbolKind SymbolKind = SymbolType // default
	if typeDefNode != nil {
		switch typeDefNode.Kind() {
		case "struct_type":
			symbolKind = SymbolStruct
		case "interface_type":
			symbolKind = SymbolInterface
		}
	}

	startPoint := nameNode.StartPosition()
	endPoint := nameNode.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      symbolKind,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "go",
	}
}

func (p *TreeSitterParser) extractGoVariable(node *sitter.Node, content []byte, filePath string) *Symbol {
	// Find var_spec child, then identifier within it
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "var_spec" {
			// Look for identifier in var_spec
			for j := uint(0); j < child.ChildCount(); j++ {
				grandChild := child.Child(j)
				if grandChild != nil && grandChild.Kind() == "identifier" {
					nameNode = grandChild
					break
				}
			}
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := nameNode.StartPosition()
	endPoint := nameNode.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      SymbolVariable,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "go",
	}
}

func (p *TreeSitterParser) extractGoConstant(node *sitter.Node, content []byte, filePath string) *Symbol {
	// Similar to extractGoVariable but for constants
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "const_spec" {
			for j := uint(0); j < child.ChildCount(); j++ {
				grandChild := child.Child(j)
				if grandChild != nil && grandChild.Kind() == "identifier" {
					nameNode = grandChild
					break
				}
			}
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := nameNode.StartPosition()
	endPoint := nameNode.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      SymbolConstant,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "go",
	}
}

func (p *TreeSitterParser) extractPythonFunction(node *sitter.Node, content []byte, filePath string) *Symbol {
	// Find the function name
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "identifier" {
			nameNode = child
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      SymbolFunction,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "python",
	}
}

func (p *TreeSitterParser) extractPythonClass(node *sitter.Node, content []byte, filePath string) *Symbol {
	// Similar to function extraction
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "identifier" {
			nameNode = child
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      SymbolClass,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "python",
	}
}

func (p *TreeSitterParser) extractJSFunction(node *sitter.Node, content []byte, filePath string) *Symbol {
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "identifier" {
			nameNode = child
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      SymbolFunction,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "javascript",
	}
}

func (p *TreeSitterParser) extractJSClass(node *sitter.Node, content []byte, filePath string) *Symbol {
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "identifier" {
			nameNode = child
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      SymbolClass,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "javascript",
	}
}

func (p *TreeSitterParser) extractRustFunction(node *sitter.Node, content []byte, filePath string) *Symbol {
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "identifier" {
			nameNode = child
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      SymbolFunction,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "rust",
	}
}

func (p *TreeSitterParser) extractRustStruct(node *sitter.Node, content []byte, filePath string) *Symbol {
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "type_identifier" {
			nameNode = child
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      SymbolStruct,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "rust",
	}
}

// extractSymbols extracts symbols from the parsed tree using queries
func (p *TreeSitterParser) extractSymbols(tree *sitter.Tree, queries *QuerySet, content []byte, filePath string) ([]*Symbol, error) {
	var symbols []*Symbol
	rootNode := tree.RootNode()

	// Extract different types of symbols
	if queries.Functions != nil {
		funcs, err := p.extractWithQuery(queries.Functions, rootNode, content, filePath, SymbolFunction)
		if err != nil {
			return nil, fmt.Errorf("failed to extract functions: %w", err)
		}
		symbols = append(symbols, funcs...)
	}

	if queries.Variables != nil {
		vars, err := p.extractWithQuery(queries.Variables, rootNode, content, filePath, SymbolVariable)
		if err != nil {
			return nil, fmt.Errorf("failed to extract variables: %w", err)
		}
		symbols = append(symbols, vars...)
	}

	if queries.Types != nil {
		types, err := p.extractWithQuery(queries.Types, rootNode, content, filePath, SymbolType)
		if err != nil {
			return nil, fmt.Errorf("failed to extract types: %w", err)
		}
		symbols = append(symbols, types...)
	}

	if queries.Classes != nil {
		classes, err := p.extractWithQuery(queries.Classes, rootNode, content, filePath, SymbolClass)
		if err != nil {
			return nil, fmt.Errorf("failed to extract classes: %w", err)
		}
		symbols = append(symbols, classes...)
	}

	if queries.Imports != nil {
		imports, err := p.extractWithQuery(queries.Imports, rootNode, content, filePath, SymbolImport)
		if err != nil {
			return nil, fmt.Errorf("failed to extract imports: %w", err)
		}
		symbols = append(symbols, imports...)
	}

	return symbols, nil
}

// extractWithQuery extracts symbols using a specific query
func (p *TreeSitterParser) extractWithQuery(query *sitter.Query, node *sitter.Node, content []byte, filePath string, defaultKind SymbolKind) ([]*Symbol, error) {
	var symbols []*Symbol

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(query, node, content)

	for match := matches.Next(); match != nil; match = matches.Next() {
		symbol := p.processMatch(match, content, filePath, defaultKind, query)
		if symbol != nil {
			symbols = append(symbols, symbol)
		}
	}

	return symbols, nil
}

// processMatch processes a query match and creates a symbol
func (p *TreeSitterParser) processMatch(match *sitter.QueryMatch, content []byte, filePath string, defaultKind SymbolKind, query *sitter.Query) *Symbol {
	if len(match.Captures) == 0 {
		return nil
	}

	var nameNode *sitter.Node
	var typeNode *sitter.Node
	var docNode *sitter.Node
	var bodyNode *sitter.Node

	// Process captures to find relevant nodes
	for _, capture := range match.Captures {
		captureName := query.CaptureNames()[capture.Index]

		switch captureName {
		case "name":
			nameNode = &capture.Node
		case "type":
			typeNode = &capture.Node
		case "doc":
			docNode = &capture.Node
		case "body":
			bodyNode = &capture.Node
		}
	}

	if nameNode == nil {
		return nil
	}

	// Extract symbol information
	startPoint := nameNode.StartPosition()
	endPoint := nameNode.EndPosition()

	symbol := &Symbol{
		Name:      p.getNodeText(nameNode, content),
		Kind:      defaultKind,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	// Extract type information if available
	if typeNode != nil {
		symbol.Type = p.getNodeText(typeNode, content)
	}

	// Extract documentation if available
	if docNode != nil {
		symbol.DocString = strings.TrimSpace(p.getNodeText(docNode, content))
	}

	// Generate signature for functions
	if defaultKind == SymbolFunction || defaultKind == SymbolMethod {
		if bodyNode != nil {
			symbol.Signature = p.generateFunctionSignature(nameNode, bodyNode, content)
		}
	}

	// Determine more specific symbol kind based on context
	symbol.Kind = p.refineSymbolKind(symbol, match, query, content)

	return symbol
}

// getNodeText extracts text content from a tree-sitter node
func (p *TreeSitterParser) getNodeText(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	start := node.StartByte()
	end := node.EndByte()

	if start >= uint(len(content)) || end > uint(len(content)) || start >= end {
		return ""
	}

	return string(content[start:end])
}

// generateFunctionSignature creates a function signature string
func (p *TreeSitterParser) generateFunctionSignature(nameNode, bodyNode *sitter.Node, content []byte) string {
	if nameNode == nil {
		return ""
	}

	// Start with function name
	name := p.getNodeText(nameNode, content)

	// Try to find parameter list
	current := nameNode.NextSibling()
	var params string

	for current != nil && current.StartByte() < bodyNode.StartByte() {
		nodeType := current.Kind()
		if strings.Contains(nodeType, "parameter") || strings.Contains(nodeType, "argument") {
			if params == "" {
				params = "("
			}
			params += p.getNodeText(current, content)
		}
		current = current.NextSibling()
	}

	if params != "" && !strings.HasSuffix(params, ")") {
		params += ")"
	} else if params == "" {
		params = "()"
	}

	return name + params
}

// refineSymbolKind determines more specific symbol kinds based on context
func (p *TreeSitterParser) refineSymbolKind(symbol *Symbol, match *sitter.QueryMatch, query *sitter.Query, content []byte) SymbolKind {
	// Check if this is a method (function with receiver/self)
	for _, capture := range match.Captures {
		captureName := query.CaptureNames()[capture.Index]

		switch captureName {
		case "method":
			return SymbolMethod
		case "class":
			return SymbolClass
		case "interface":
			return SymbolInterface
		case "struct":
			return SymbolStruct
		case "enum":
			return SymbolEnum
		case "constant":
			return SymbolConstant
		case "field":
			return SymbolField
		case "property":
			return SymbolProperty
		}
	}

	return symbol.Kind
}

// getLanguageConfig returns the language configuration for a file
func (p *TreeSitterParser) getLanguageConfig(filePath string) *LanguageConfig {
	ext := strings.ToLower(filepath.Ext(filePath))

	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.parsers[ext]
}

// GetSupportedExtensions returns all supported file extensions
func (p *TreeSitterParser) GetSupportedExtensions() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	extensions := make([]string, 0, len(p.parsers))
	for ext := range p.parsers {
		extensions = append(extensions, ext)
	}

	return extensions
}

// FindReferences finds references to a symbol across multiple files
func (p *TreeSitterParser) FindReferences(symbol *Symbol, files []string, maxResults int) ([]Location, error) {
	var references []Location
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrent parsing
	semaphore := make(chan struct{}, 10)

	for _, file := range files {
		if len(references) >= maxResults {
			break
		}

		wg.Add(1)
		go func(filePath string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			content, err := os.ReadFile(filePath)
			if err != nil {
				return
			}

			refs := p.findReferencesInContent(symbol, content, filePath)

			mu.Lock()
			references = append(references, refs...)
			mu.Unlock()
		}(file)
	}

	wg.Wait()

	// Sort and limit results
	if len(references) > maxResults {
		references = references[:maxResults]
	}

	return references, nil
}

// findReferencesInContent finds references to a symbol in file content
func (p *TreeSitterParser) findReferencesInContent(symbol *Symbol, content []byte, filePath string) []Location {
	var references []Location

	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		// Simple string matching for now - can be enhanced with proper tree-sitter queries
		col := strings.Index(line, symbol.Name)
		for col >= 0 {
			// Basic heuristic to avoid false positives
			if p.isValidReference(line, col, symbol.Name) {
				references = append(references, Location{
					File:     filePath,
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

// isValidReference performs basic validation to reduce false positives
func (p *TreeSitterParser) isValidReference(line string, col int, symbolName string) bool {
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

// LoadQueryFromFile loads a tree-sitter query from a file
func LoadQueryFromFile(filePath string, language *sitter.Language) (*sitter.Query, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read query file: %w", err)
	}

	query, queryErr := sitter.NewQuery(language, string(content))
	if queryErr != nil {
		return nil, fmt.Errorf("failed to compile query: %s", queryErr.Message)
	}

	return query, nil
}

// Close cleans up parser resources
func (p *TreeSitterParser) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close all queries
	for _, config := range p.parsers {
		if config.Queries != nil {
			p.closeQuerySet(config.Queries)
		}
	}

	p.parsers = make(map[string]*LanguageConfig)
	return nil
}

// closeQuerySet closes all queries in a query set
func (p *TreeSitterParser) closeQuerySet(qs *QuerySet) {
	if qs.Functions != nil {
		qs.Functions.Close()
	}
	if qs.Variables != nil {
		qs.Variables.Close()
	}
	if qs.Types != nil {
		qs.Types.Close()
	}
	if qs.Classes != nil {
		qs.Classes.Close()
	}
	if qs.Imports != nil {
		qs.Imports.Close()
	}
	if qs.Exports != nil {
		qs.Exports.Close()
	}
	if qs.Comments != nil {
		qs.Comments.Close()
	}
	if qs.Definitions != nil {
		qs.Definitions.Close()
	}
	if qs.References != nil {
		qs.References.Close()
	}
}

// CompileQuery compiles a tree-sitter query string
func CompileQuery(queryString string, language *sitter.Language) (*sitter.Query, error) {
	query, queryErr := sitter.NewQuery(language, queryString)
	if queryErr != nil {
		return nil, fmt.Errorf("failed to compile query: %s", queryErr.Message)
	}
	return query, nil
}

// ValidateTree checks if a parsed tree has errors
func ValidateTree(tree *sitter.Tree) []ParseError {
	var errors []ParseError

	rootNode := tree.RootNode()
	var walk func(*sitter.Node)

	walk = func(node *sitter.Node) {
		if node.HasError() || node.Kind() == "ERROR" {
			startPoint := node.StartPosition()
			errors = append(errors, ParseError{
				Line:    int(startPoint.Row) + 1,
				Column:  int(startPoint.Column) + 1,
				Message: fmt.Sprintf("Syntax error at %s node", node.Kind()),
			})
		}

		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil {
				walk(child)
			}
		}
	}

	walk(rootNode)
	return errors
}

// typescriptNodeToSymbol converts TypeScript-specific nodes to symbols
func (p *TreeSitterParser) typescriptNodeToSymbol(node *sitter.Node, nodeType string, content []byte, filePath string) *Symbol {
	switch nodeType {
	case "function_declaration":
		return p.extractTypescriptFunction(node, content, filePath)
	case "class_declaration":
		return p.extractTypescriptClass(node, content, filePath)
	case "interface_declaration":
		return p.extractTypescriptInterface(node, content, filePath)
	case "type_alias_declaration":
		return p.extractTypescriptType(node, content, filePath)
	default:
		return nil
	}
}

func (p *TreeSitterParser) extractTypescriptFunction(node *sitter.Node, content []byte, filePath string) *Symbol {
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "identifier" {
			nameNode = child
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      SymbolFunction,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "typescript",
	}
}

func (p *TreeSitterParser) extractTypescriptClass(node *sitter.Node, content []byte, filePath string) *Symbol {
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && (child.Kind() == "type_identifier" || child.Kind() == "identifier") {
			nameNode = child
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      SymbolClass,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "typescript",
	}
}

func (p *TreeSitterParser) extractTypescriptInterface(node *sitter.Node, content []byte, filePath string) *Symbol {
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && (child.Kind() == "type_identifier" || child.Kind() == "identifier") {
			nameNode = child
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      SymbolInterface,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "typescript",
	}
}

func (p *TreeSitterParser) extractTypescriptType(node *sitter.Node, content []byte, filePath string) *Symbol {
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && (child.Kind() == "type_identifier" || child.Kind() == "identifier") {
			nameNode = child
			break
		}
	}

	if nameNode == nil {
		return nil
	}

	name := p.getNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	return &Symbol{
		Name:      name,
		Kind:      SymbolType,
		Location: Location{
			File:      filePath,
			Line:      int(startPoint.Row) + 1,
			Column:    int(startPoint.Column) + 1,
			EndLine:   int(endPoint.Row) + 1,
			EndColumn: int(endPoint.Column) + 1,
		},
		FilePath:  filePath,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
		Language:  "typescript",
	}
}

// fallbackParse provides basic symbol extraction when tree-sitter parsing fails
func (p *TreeSitterParser) fallbackParse(filePath string, content []byte, language string) (*ParseResult, error) {
	result := &ParseResult{
		Tree:     nil, // No tree available
		Language: language,
		FilePath: filePath,
		Symbols:  make([]*Symbol, 0),
	}

	// Use regex-based fallback parsing
	symbols := p.extractSymbolsWithRegex(content, filePath, language)
	result.Symbols = symbols

	return result, nil
}

// extractSymbolsWithRegex extracts basic symbols using regex patterns when tree-sitter fails
func (p *TreeSitterParser) extractSymbolsWithRegex(content []byte, filePath, language string) []*Symbol {
	var symbols []*Symbol
	lines := strings.Split(string(content), "\n")

	switch language {
	case "typescript":
		symbols = append(symbols, p.extractTypescriptSymbolsRegex(lines, filePath)...)
	case "javascript":
		symbols = append(symbols, p.extractJavascriptSymbolsRegex(lines, filePath)...)
	case "go":
		symbols = append(symbols, p.extractGoSymbolsRegex(lines, filePath)...)
	case "python":
		symbols = append(symbols, p.extractPythonSymbolsRegex(lines, filePath)...)
	case "rust":
		symbols = append(symbols, p.extractRustSymbolsRegex(lines, filePath)...)
	}

	return symbols
}

func (p *TreeSitterParser) extractTypescriptSymbolsRegex(lines []string, filePath string) []*Symbol {
	var symbols []*Symbol

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)

		// Match interface declarations
		if strings.HasPrefix(line, "interface ") || strings.Contains(line, "interface ") {
			if parts := strings.Fields(line); len(parts) >= 2 {
				for i, part := range parts {
					if part == "interface" && i+1 < len(parts) {
						name := strings.TrimSuffix(parts[i+1], "{")
						name = strings.TrimSpace(name)
						if name != "" {
							symbols = append(symbols, &Symbol{
								Name:      name,
								Kind:      SymbolInterface,
								FilePath:  filePath,
								Line:      lineNum + 1,
								Column:    1,
								Language:  "typescript",
								Location: Location{
									File:   filePath,
									Line:   lineNum + 1,
									Column: 1,
								},
							})
						}
						break
					}
				}
			}
		}

		// Match type declarations
		if strings.HasPrefix(line, "type ") || strings.Contains(line, "type ") {
			if parts := strings.Fields(line); len(parts) >= 2 {
				for i, part := range parts {
					if part == "type" && i+1 < len(parts) {
						name := strings.Split(parts[i+1], "=")[0]
						name = strings.TrimSpace(name)
						if name != "" {
							symbols = append(symbols, &Symbol{
								Name:      name,
								Kind:      SymbolType,
								FilePath:  filePath,
								Line:      lineNum + 1,
								Column:    1,
								Language:  "typescript",
								Location: Location{
									File:   filePath,
									Line:   lineNum + 1,
									Column: 1,
								},
							})
						}
						break
					}
				}
			}
		}

		// Match class declarations
		if strings.HasPrefix(line, "class ") || strings.Contains(line, "class ") {
			if parts := strings.Fields(line); len(parts) >= 2 {
				for i, part := range parts {
					if part == "class" && i+1 < len(parts) {
						name := strings.TrimSuffix(parts[i+1], "{")
						name = strings.Split(name, " ")[0] // Handle "class Name extends" case
						name = strings.TrimSpace(name)
						if name != "" {
							symbols = append(symbols, &Symbol{
								Name:      name,
								Kind:      SymbolClass,
								FilePath:  filePath,
								Line:      lineNum + 1,
								Column:    1,
								Language:  "typescript",
								Location: Location{
									File:   filePath,
									Line:   lineNum + 1,
									Column: 1,
								},
							})
						}
						break
					}
				}
			}
		}

		// Match function declarations (including methods)
		if strings.Contains(line, "function ") || (strings.Contains(line, "(") && strings.Contains(line, ")") && strings.Contains(line, "{")) {
			if strings.Contains(line, "function ") {
				if parts := strings.Fields(line); len(parts) >= 2 {
					for i, part := range parts {
						if part == "function" && i+1 < len(parts) {
							name := strings.Split(parts[i+1], "(")[0]
							name = strings.TrimSpace(name)
							if name != "" {
								symbols = append(symbols, &Symbol{
									Name:      name,
									Kind:      SymbolFunction,
									FilePath:  filePath,
									Line:      lineNum + 1,
									Column:    1,
									Language:  "typescript",
									Location: Location{
										File:   filePath,
										Line:   lineNum + 1,
										Column: 1,
									},
								})
							}
							break
						}
					}
				}
			} else if strings.Contains(line, "(") && strings.Contains(line, ")") && !strings.HasPrefix(line, "//") {
				// Match method-like patterns: name(...) { or name(...): type {
				parenIndex := strings.Index(line, "(")
				if parenIndex > 0 {
					beforeParen := line[:parenIndex]
					// Extract potential method name
					words := strings.Fields(beforeParen)
					if len(words) > 0 {
						lastWord := words[len(words)-1]
						// Skip common keywords
						if lastWord != "if" && lastWord != "while" && lastWord != "for" && lastWord != "catch" && lastWord != "switch" {
							symbols = append(symbols, &Symbol{
								Name:      lastWord,
								Kind:      SymbolMethod,
								FilePath:  filePath,
								Line:      lineNum + 1,
								Column:    1,
								Language:  "typescript",
								Location: Location{
									File:   filePath,
									Line:   lineNum + 1,
									Column: 1,
								},
							})
						}
					}
				}
			}
		}
	}

	return symbols
}

// Add basic regex fallbacks for other languages as well
func (p *TreeSitterParser) extractJavascriptSymbolsRegex(lines []string, filePath string) []*Symbol {
	// Similar to TypeScript but without types and interfaces
	var symbols []*Symbol
	// Basic implementation - can be expanded
	return symbols
}

func (p *TreeSitterParser) extractGoSymbolsRegex(lines []string, filePath string) []*Symbol {
	var symbols []*Symbol

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)

		// Match function declarations
		if strings.HasPrefix(line, "func ") {
			// Check if it's a method (has receiver): func (receiver) methodName
			if strings.HasPrefix(line, "func (") {
				// Method declaration - find the method name after the receiver
				receiverEnd := strings.Index(line, ") ")
				if receiverEnd > 0 {
					afterReceiver := strings.TrimSpace(line[receiverEnd+2:])
					if methodParts := strings.Fields(afterReceiver); len(methodParts) > 0 {
						methodName := strings.Split(methodParts[0], "(")[0]
						if methodName != "" {
							// Extract docstring from previous lines
							docString := p.extractDocString(lines, lineNum)
							symbols = append(symbols, &Symbol{
								Name:      methodName,
								Kind:      SymbolMethod,
								FilePath:  filePath,
								Line:      lineNum + 1,
								Column:    1,
								Language:  "go",
								DocString: docString,
							})
						}
					}
				}
			} else if parts := strings.Fields(line); len(parts) >= 2 {
				// Regular function declaration
				funcName := strings.Split(parts[1], "(")[0]
				if funcName != "" {
					// Extract docstring from previous lines
					docString := p.extractDocString(lines, lineNum)
					symbols = append(symbols, &Symbol{
						Name:      funcName,
						Kind:      SymbolFunction,
						FilePath:  filePath,
						Line:      lineNum + 1,
						Column:    1,
						Language:  "go",
						DocString: docString,
					})
				}
			}
		}

		// Match type declarations
		if strings.HasPrefix(line, "type ") {
			if parts := strings.Fields(line); len(parts) >= 3 {
				typeName := parts[1]
				typeKind := parts[2]
				var symbolKind SymbolKind = SymbolType

				if typeKind == "struct" || strings.Contains(typeKind, "struct") {
					symbolKind = SymbolStruct
				} else if typeKind == "interface" || strings.Contains(typeKind, "interface") {
					symbolKind = SymbolInterface
				}

				if typeName != "" {
					// Extract docstring from previous lines
					docString := p.extractDocString(lines, lineNum)
					symbols = append(symbols, &Symbol{
						Name:      typeName,
						Kind:      symbolKind,
						FilePath:  filePath,
						Line:      lineNum + 1,
						Column:    1,
						Language:  "go",
						DocString: docString,
					})
				}
			}
		}

		// Match variable declarations
		if strings.HasPrefix(line, "var ") {
			if parts := strings.Fields(line); len(parts) >= 2 {
				varName := parts[1]
				if varName != "" && !strings.Contains(varName, "(") {
					symbols = append(symbols, &Symbol{
						Name:      varName,
						Kind:      SymbolVariable,
						FilePath:  filePath,
						Line:      lineNum + 1,
						Column:    1,
						Language:  "go",
					})
				}
			}
		}

		// Match constant declarations
		if strings.HasPrefix(line, "const ") {
			if parts := strings.Fields(line); len(parts) >= 2 {
				constName := parts[1]
				if constName != "" {
					symbols = append(symbols, &Symbol{
						Name:      constName,
						Kind:      SymbolConstant,
						FilePath:  filePath,
						Line:      lineNum + 1,
						Column:    1,
						Language:  "go",
					})
				}
			}
		}
	}

	return symbols
}

// extractDocString extracts documentation from the lines preceding a symbol
func (p *TreeSitterParser) extractDocString(lines []string, lineNum int) string {
	var docLines []string

	// Look backwards from the symbol line for comments
	for i := lineNum - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "//") {
			// Add the comment without the // prefix
			comment := strings.TrimSpace(strings.TrimPrefix(line, "//"))
			docLines = append([]string{comment}, docLines...) // prepend
		} else if line == "" {
			// Skip empty lines
			continue
		} else {
			// Stop when we hit a non-comment, non-empty line
			break
		}
	}

	if len(docLines) > 0 {
		return strings.Join(docLines, " ")
	}
	return ""
}

func (p *TreeSitterParser) extractPythonSymbolsRegex(lines []string, filePath string) []*Symbol {
	var symbols []*Symbol
	// Basic implementation - can be expanded
	return symbols
}

func (p *TreeSitterParser) extractRustSymbolsRegex(lines []string, filePath string) []*Symbol {
	var symbols []*Symbol
	// Basic implementation - can be expanded
	return symbols
}