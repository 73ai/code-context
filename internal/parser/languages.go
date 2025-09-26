package parser

import (
	"fmt"
	"path/filepath"

	sitter "github.com/tree-sitter/go-tree-sitter"

	// Tree-sitter language parsers
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
	tree_sitter_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
	tree_sitter_cpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

// LanguageRegistry manages all supported programming languages
type LanguageRegistry struct {
	parser *TreeSitterParser
}

// NewLanguageRegistry creates a new language registry
func NewLanguageRegistry() (*LanguageRegistry, error) {
	lr := &LanguageRegistry{
		parser: NewTreeSitterParser(),
	}

	// Initialize all supported languages
	if err := lr.initializeLanguages(); err != nil {
		return nil, fmt.Errorf("failed to initialize languages: %w", err)
	}

	return lr, nil
}

// initializeLanguages sets up all supported language parsers
func (lr *LanguageRegistry) initializeLanguages() error {
	// Initialize tree-sitter languages with actual parsers
	languages := []struct {
		name       string
		language   *sitter.Language
		extensions []string
	}{
		{
			name:       "go",
			language:   sitter.NewLanguage(tree_sitter_go.Language()),
			extensions: []string{".go"},
		},
		{
			name:       "python",
			language:   sitter.NewLanguage(tree_sitter_python.Language()),
			extensions: []string{".py", ".pyx", ".pyi"},
		},
		{
			name:       "javascript",
			language:   sitter.NewLanguage(tree_sitter_javascript.Language()),
			extensions: []string{".js", ".mjs", ".jsx"},
		},
		{
			name:       "rust",
			language:   sitter.NewLanguage(tree_sitter_rust.Language()),
			extensions: []string{".rs"},
		},
		{
			name:       "typescript",
			language:   sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript()),
			extensions: []string{".ts", ".tsx", ".d.ts"},
		},
		{
			name:       "c",
			language:   sitter.NewLanguage(tree_sitter_c.Language()),
			extensions: []string{".c", ".h"},
		},
		{
			name:       "cpp",
			language:   sitter.NewLanguage(tree_sitter_cpp.Language()),
			extensions: []string{".cpp", ".cc", ".cxx", ".hpp", ".hxx", ".h++"},
		},
		{
			name:       "java",
			language:   sitter.NewLanguage(tree_sitter_java.Language()),
			extensions: []string{".java"},
		},
	}

	for _, lang := range languages {
		if err := lr.registerLanguage(lang.name, lang.language, lang.extensions); err != nil {
			// Don't fail completely if a language fails to register - log warning and continue
			fmt.Printf("Warning: failed to register %s with tree-sitter: %v\n", lang.name, err)
			// Register without tree-sitter parser as fallback (will use regex-based parsing)
			if err := lr.registerLanguage(lang.name, nil, lang.extensions); err != nil {
				fmt.Printf("Warning: failed to register %s (fallback): %v\n", lang.name, err)
				continue // Don't fail completely, just skip this language
			}
			fmt.Printf("Registered %s with regex fallback\n", lang.name)
		} else {
			fmt.Printf("Registered %s with tree-sitter\n", lang.name)
		}
	}

	return nil
}

// registerLanguage registers a language with its queries
func (lr *LanguageRegistry) registerLanguage(name string, language *sitter.Language, extensions []string) error {
	// Use direct AST walking instead of queries for now (more stable)
	queries := &QuerySet{}

	config := &LanguageConfig{
		Language:   language,
		Extensions: extensions,
		Queries:    queries,
		Name:       name,
	}

	return lr.parser.RegisterLanguage(config)
}

// loadQueriesForLanguage loads all query files for a specific language
func (lr *LanguageRegistry) loadQueriesForLanguage(langName string, language *sitter.Language) (*QuerySet, error) {
	if language == nil {
		return &QuerySet{}, nil
	}
	queryDir := filepath.Join("queries", langName)

	queries := &QuerySet{}

	// Define query files to load
	queryFiles := map[string]**sitter.Query{
		"functions.scm":    &queries.Functions,
		"variables.scm":    &queries.Variables,
		"types.scm":        &queries.Types,
		"classes.scm":      &queries.Classes,
		"imports.scm":      &queries.Imports,
		"exports.scm":      &queries.Exports,
		"comments.scm":     &queries.Comments,
		"definitions.scm":  &queries.Definitions,
		"references.scm":   &queries.References,
	}

	// Load each query file if it exists
	for filename, queryPtr := range queryFiles {
		queryPath := filepath.Join(queryDir, filename)

		query, err := LoadQueryFromFile(queryPath, language)
		if err != nil {
			// Skip missing query files - they're optional
			continue
		}

		*queryPtr = query
	}

	return queries, nil
}

// GetParser returns the underlying tree-sitter parser
func (lr *LanguageRegistry) GetParser() *TreeSitterParser {
	return lr.parser
}

// GetSupportedLanguages returns a list of all supported languages
func (lr *LanguageRegistry) GetSupportedLanguages() []string {
	return []string{
		"go",
		"python",
		"javascript",
		"typescript",
		"rust",
	}
}

// GetLanguageForFile determines the programming language for a file
func (lr *LanguageRegistry) GetLanguageForFile(filePath string) string {
	ext := filepath.Ext(filePath)

	switch ext {
	case ".go":
		return "go"
	case ".py", ".pyx", ".pyi":
		return "python"
	case ".js", ".mjs", ".jsx":
		return "javascript"
	case ".ts", ".tsx", ".d.ts":
		return "typescript"
	case ".rs":
		return "rust"
	default:
		return ""
	}
}

// Close cleans up all language resources
func (lr *LanguageRegistry) Close() error {
	return lr.parser.Close()
}

// LanguageFeatures describes the capabilities of each language parser
type LanguageFeatures struct {
	Language            string   `json:"language"`
	Extensions          []string `json:"extensions"`
	SupportsFunctions   bool     `json:"supports_functions"`
	SupportsClasses     bool     `json:"supports_classes"`
	SupportsVariables   bool     `json:"supports_variables"`
	SupportsTypes       bool     `json:"supports_types"`
	SupportsImports     bool     `json:"supports_imports"`
	SupportsExports     bool     `json:"supports_exports"`
	SupportsComments    bool     `json:"supports_comments"`
	SupportsReferences  bool     `json:"supports_references"`
	SupportsInterfaces  bool     `json:"supports_interfaces"`
}

// GetLanguageFeatures returns the features supported by each language
func (lr *LanguageRegistry) GetLanguageFeatures() []LanguageFeatures {
	return []LanguageFeatures{
		{
			Language:            "go",
			Extensions:          []string{".go"},
			SupportsFunctions:   true,
			SupportsClasses:     false, // Go uses structs and interfaces
			SupportsVariables:   true,
			SupportsTypes:       true,
			SupportsImports:     true,
			SupportsExports:     true, // Public/private via capitalization
			SupportsComments:    true,
			SupportsReferences:  true,
			SupportsInterfaces:  true,
		},
		{
			Language:            "python",
			Extensions:          []string{".py", ".pyx", ".pyi"},
			SupportsFunctions:   true,
			SupportsClasses:     true,
			SupportsVariables:   true,
			SupportsTypes:       true, // With type hints
			SupportsImports:     true,
			SupportsExports:     true, // __all__ and module-level definitions
			SupportsComments:    true,
			SupportsReferences:  true,
			SupportsInterfaces:  false, // Python doesn't have explicit interfaces
		},
		{
			Language:            "javascript",
			Extensions:          []string{".js", ".mjs", ".jsx"},
			SupportsFunctions:   true,
			SupportsClasses:     true, // ES6+ classes
			SupportsVariables:   true,
			SupportsTypes:       false, // No native types, but JSDoc
			SupportsImports:     true, // ES modules
			SupportsExports:     true,
			SupportsComments:    true,
			SupportsReferences:  true,
			SupportsInterfaces:  false, // JavaScript doesn't have interfaces
		},
		{
			Language:            "typescript",
			Extensions:          []string{".ts", ".tsx", ".d.ts"},
			SupportsFunctions:   true,
			SupportsClasses:     true,
			SupportsVariables:   true,
			SupportsTypes:       true, // Full type system
			SupportsImports:     true,
			SupportsExports:     true,
			SupportsComments:    true,
			SupportsReferences:  true,
			SupportsInterfaces:  true,
		},
		{
			Language:            "rust",
			Extensions:          []string{".rs"},
			SupportsFunctions:   true,
			SupportsClasses:     false, // Rust uses structs and traits
			SupportsVariables:   true,
			SupportsTypes:       true,
			SupportsImports:     true, // use statements
			SupportsExports:     true, // pub keyword
			SupportsComments:    true,
			SupportsReferences:  true,
			SupportsInterfaces:  false, // Rust uses traits, not interfaces
		},
	}
}

// LanguageSpecificParser provides language-specific parsing utilities
type LanguageSpecificParser struct {
	Language string
	registry *LanguageRegistry
}

// NewLanguageSpecificParser creates a parser for a specific language
func NewLanguageSpecificParser(language string, registry *LanguageRegistry) *LanguageSpecificParser {
	return &LanguageSpecificParser{
		Language: language,
		registry: registry,
	}
}

// ParseFile parses a file using language-specific logic
func (lsp *LanguageSpecificParser) ParseFile(filePath string, content []byte) (*ParseResult, error) {
	return lsp.registry.parser.ParseFile(filePath, content)
}

// GetSymbolKindTypes returns the types of symbols this language can extract
func (lsp *LanguageSpecificParser) GetSymbolKindTypes() []SymbolKind {
	switch lsp.Language {
	case "go":
		return []SymbolKind{
			SymbolKindFunction,
			SymbolKindMethod,
			SymbolKindVariable,
			SymbolKindConstant,
			SymbolKindType,
			SymbolKindStruct,
			SymbolKindInterface,
			SymbolKindImport,
		}
	case "python":
		return []SymbolKind{
			SymbolKindFunction,
			SymbolKindMethod,
			SymbolKindClass,
			SymbolKindVariable,
			SymbolKindImport,
			SymbolKindProperty,
		}
	case "javascript":
		return []SymbolKind{
			SymbolKindFunction,
			SymbolKindMethod,
			SymbolKindClass,
			SymbolKindVariable,
			SymbolKindConstant,
			SymbolKindImport,
			SymbolKindProperty,
		}
	case "typescript":
		return []SymbolKind{
			SymbolKindFunction,
			SymbolKindMethod,
			SymbolKindClass,
			SymbolKindInterface,
			SymbolKindType,
			SymbolKindVariable,
			SymbolKindConstant,
			SymbolKindImport,
			SymbolKindProperty,
		}
	case "rust":
		return []SymbolKind{
			SymbolKindFunction,
			SymbolKindMethod,
			SymbolKindStruct,
			SymbolKindEnum,
			SymbolKindType,
			SymbolKindVariable,
			SymbolKindConstant,
			SymbolKindImport,
		}
	default:
		return []SymbolKind{}
	}
}

// GetDefaultQueries returns default query strings for languages that don't have query files
func GetDefaultQueries(language string) map[string]string {
	switch language {
	case "go":
		return map[string]string{
			"functions": `
				(function_declaration
					name: (identifier) @name) @function

				(method_declaration
					name: (identifier) @name
					receiver: (parameter_list) @receiver) @method
			`,
			"variables": `
				(var_declaration
					(var_spec
						name: (identifier) @name)) @variable

				(const_declaration
					(const_spec
						name: (identifier) @name)) @constant
			`,
			"types": `
				(type_declaration
					(type_spec
						name: (identifier) @name)) @type
			`,
			"imports": `
				(import_declaration
					(import_spec
						path: (interpreted_string_literal) @path
						name: (identifier)? @name)) @import
			`,
		}
	case "python":
		return map[string]string{
			"functions": `
				(function_definition
					name: (identifier) @name) @function
			`,
			"classes": `
				(class_definition
					name: (identifier) @name) @class
			`,
			"variables": `
				(assignment
					left: (identifier) @name) @variable
			`,
			"imports": `
				(import_statement
					name: (dotted_name) @name) @import

				(import_from_statement
					module_name: (dotted_name) @module
					name: (dotted_name) @name) @import
			`,
		}
	case "javascript":
		return map[string]string{
			"functions": `
				(function_declaration
					name: (identifier) @name) @function

				(arrow_function
					parameter: (identifier) @name) @function

				(method_definition
					name: (property_identifier) @name) @method
			`,
			"classes": `
				(class_declaration
					name: (identifier) @name) @class
			`,
			"variables": `
				(variable_declaration
					(variable_declarator
						name: (identifier) @name)) @variable
			`,
			"imports": `
				(import_statement
					source: (string) @source
					(import_clause
						(identifier) @name)) @import
			`,
		}
	case "typescript":
		return map[string]string{
			"functions": `
				(function_declaration
					name: (identifier) @name) @function

				(method_definition
					name: (property_identifier) @name) @method
			`,
			"classes": `
				(class_declaration
					name: (type_identifier) @name) @class
			`,
			"interfaces": `
				(interface_declaration
					name: (type_identifier) @name) @interface
			`,
			"types": `
				(type_alias_declaration
					name: (type_identifier) @name) @type
			`,
			"variables": `
				(variable_declaration
					(variable_declarator
						name: (identifier) @name)) @variable
			`,
		}
	case "rust":
		return map[string]string{
			"functions": `
				(function_item
					name: (identifier) @name) @function
			`,
			"structs": `
				(struct_item
					name: (type_identifier) @name) @struct
			`,
			"enums": `
				(enum_item
					name: (type_identifier) @name) @enum
			`,
			"traits": `
				(trait_item
					name: (type_identifier) @name) @trait
			`,
			"variables": `
				(let_declaration
					pattern: (identifier) @name) @variable
			`,
			"imports": `
				(use_declaration
					argument: (identifier) @name) @import
			`,
		}
	default:
		return map[string]string{}
	}
}

// CreateQueriesFromDefaults creates Query objects from default query strings
func CreateQueriesFromDefaults(language string, treeSitterLang *sitter.Language) (*QuerySet, error) {
	// For now, return empty queries to allow compilation
	// TODO: Implement proper tree-sitter query parsing when external parsers are available
	return &QuerySet{}, nil
}