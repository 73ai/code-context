package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Config holds all command-line options
type Config struct {
	// Basic search options
	Pattern   string   `json:"pattern"`
	Paths     []string `json:"paths"`
	CaseSensitive bool `json:"case_sensitive"`

	// Ripgrep compatibility flags
	IgnoreCase       bool     `json:"ignore_case"`        // -i, --ignore-case
	LineNumber       bool     `json:"line_number"`        // -n, --line-number
	WithFilename     bool     `json:"with_filename"`      // -H, --with-filename
	FilesWithMatches bool     `json:"files_with_matches"` // -l, --files-with-matches
	Count            bool     `json:"count"`              // -c, --count
	OnlyMatching     bool     `json:"only_matching"`      // -o, --only-matching
	ContextAfter     int      `json:"context_after"`      // -A, --after-context
	ContextBefore    int      `json:"context_before"`     // -B, --before-context
	Context          int      `json:"context"`            // -C, --context

	// File type filtering
	Type       []string `json:"type"`        // -t, --type
	TypeNot    []string `json:"type_not"`    // -T, --type-not
	Glob       []string `json:"glob"`        // -g, --glob
	GlobNot    []string `json:"glob_not"`    // --iglob

	// Output control
	JSON        bool   `json:"json"`         // --json
	Color       string `json:"color"`        // --color
	NoHeading   bool   `json:"no_heading"`   // --no-heading
	NullSep     bool   `json:"null_sep"`     // -0, --null
	PathSep     string `json:"path_sep"`     // --path-separator

	// Search behavior
	WordRegexp    bool `json:"word_regexp"`     // -w, --word-regexp
	LineRegexp    bool `json:"line_regexp"`     // -x, --line-regexp
	FixedStrings  bool `json:"fixed_strings"`   // -F, --fixed-strings
	PCREMode      bool `json:"pcre_mode"`       // -P, --pcre2
	Multiline     bool `json:"multiline"`       // -U, --multiline
	DotMatchesAll bool `json:"dot_matches_all"` // --multiline-dotall

	// Performance and limits
	MaxCount    int  `json:"max_count"`     // -m, --max-count
	MaxDepth    int  `json:"max_depth"`     // --max-depth
	MaxFilesize int  `json:"max_filesize"`  // --max-filesize
	Threads     int  `json:"threads"`       // -j, --threads

	// Hidden files and directories
	Hidden    bool `json:"hidden"`     // --hidden
	NoIgnore  bool `json:"no_ignore"`  // --no-ignore
	NoGlobal  bool `json:"no_global"`  // --no-ignore-global
	NoParent  bool `json:"no_parent"`  // --no-ignore-parent
	NoVcs     bool `json:"no_vcs"`     // --no-ignore-vcs

	// Other ripgrep options
	Invert       bool   `json:"invert"`        // -v, --invert-match
	Quiet        bool   `json:"quiet"`         // -q, --quiet
	Binary       bool   `json:"binary"`        // -a, --binary
	Replace      string `json:"replace"`       // -r, --replace
	Encoding     string `json:"encoding"`      // -E, --encoding

	// NEW: Semantic search flags
	Symbols   bool `json:"symbols"`    // --symbols - find symbol definitions
	Refs      bool `json:"refs"`       // --refs - find symbol references
	Types     bool `json:"types"`      // --types - find type definitions
	CallGraph bool `json:"call_graph"` // --call-graph - show call relationships

	// Index management
	NoIndex    bool   `json:"no_index"`     // --no-index - disable index usage
	IndexPath  string `json:"index_path"`   // --index-path - custom index location
	RebuildIndex bool `json:"rebuild_index"` // --rebuild-index - force index rebuild
}

var config Config

var rootCmd = &cobra.Command{
	Use:   "codegrep [OPTIONS] PATTERN [PATH...]",
	Short: "A ripgrep-compatible semantic code search tool",
	Long: `codegrep is a fast semantic code search tool that maintains full compatibility with ripgrep
while adding powerful semantic search capabilities powered by tree-sitter.

EXAMPLES:
    # Basic ripgrep-compatible usage
    codegrep "func.*main" src/
    codegrep -i "error" --type go
    codegrep -n -A 3 "TODO" .

    # Semantic search features
    codegrep --symbols "handleRequest" src/
    codegrep --refs "User" --type go
    codegrep --call-graph "main" --json`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	Args: func(cmd *cobra.Command, args []string) error {
		// At minimum we need a pattern, unless using semantic-only modes
		if len(args) == 0 && !config.Symbols && !config.Refs && !config.Types && !config.CallGraph {
			return fmt.Errorf("missing required argument: PATTERN")
		}
		if len(args) > 0 {
			config.Pattern = args[0]
			if len(args) > 1 {
				config.Paths = args[1:]
			}
		}
		if len(config.Paths) == 0 {
			config.Paths = []string{"."}
		}
		return nil
	},
	RunE: runSearch,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.Flags().BoolVarP(&config.IgnoreCase, "ignore-case", "i", false, "Case insensitive search")
	rootCmd.Flags().BoolVarP(&config.LineNumber, "line-number", "n", false, "Show line numbers")
	rootCmd.Flags().BoolVarP(&config.WithFilename, "with-filename", "H", false, "Show file names")
	rootCmd.Flags().BoolVarP(&config.FilesWithMatches, "files-with-matches", "l", false, "Only show files with matches")
	rootCmd.Flags().BoolVarP(&config.Count, "count", "c", false, "Show count of matches per file")
	rootCmd.Flags().BoolVarP(&config.OnlyMatching, "only-matching", "o", false, "Show only matching parts")

	rootCmd.Flags().IntVarP(&config.ContextAfter, "after-context", "A", 0, "Show NUM lines after each match")
	rootCmd.Flags().IntVarP(&config.ContextBefore, "before-context", "B", 0, "Show NUM lines before each match")
	rootCmd.Flags().IntVarP(&config.Context, "context", "C", 0, "Show NUM lines before and after each match")

	rootCmd.Flags().StringSliceVarP(&config.Type, "type", "t", nil, "Search only files matching TYPE")
	rootCmd.Flags().StringSliceVarP(&config.TypeNot, "type-not", "T", nil, "Do not search files matching TYPE")
	rootCmd.Flags().StringSliceVarP(&config.Glob, "glob", "g", nil, "Include files matching GLOB")
	rootCmd.Flags().StringSliceVar(&config.GlobNot, "iglob", nil, "Exclude files matching GLOB")

	rootCmd.Flags().BoolVar(&config.JSON, "json", false, "Output results in JSON format")
	rootCmd.Flags().StringVar(&config.Color, "color", "auto", "When to use colors (never, auto, always)")
	rootCmd.Flags().BoolVar(&config.NoHeading, "no-heading", false, "Don't print file names")
	rootCmd.Flags().BoolVarP(&config.NullSep, "null", "0", false, "Use null separator")
	rootCmd.Flags().StringVar(&config.PathSep, "path-separator", "/", "Path separator to use")

	rootCmd.Flags().BoolVarP(&config.WordRegexp, "word-regexp", "w", false, "Only match whole words")
	rootCmd.Flags().BoolVarP(&config.LineRegexp, "line-regexp", "x", false, "Only match whole lines")
	rootCmd.Flags().BoolVarP(&config.FixedStrings, "fixed-strings", "F", false, "Treat pattern as literal string")
	rootCmd.Flags().BoolVarP(&config.PCREMode, "pcre2", "P", false, "Use PCRE2 regex engine")
	rootCmd.Flags().BoolVarP(&config.Multiline, "multiline", "U", false, "Enable multiline mode")
	rootCmd.Flags().BoolVar(&config.DotMatchesAll, "multiline-dotall", false, "Allow . to match newlines")

	rootCmd.Flags().IntVarP(&config.MaxCount, "max-count", "m", 0, "Limit matches per file")
	rootCmd.Flags().IntVar(&config.MaxDepth, "max-depth", 0, "Limit directory traversal depth")
	rootCmd.Flags().IntVar(&config.MaxFilesize, "max-filesize", 0, "Skip files larger than SIZE bytes")
	rootCmd.Flags().IntVarP(&config.Threads, "threads", "j", 0, "Number of threads to use (0 = auto)")

	rootCmd.Flags().BoolVar(&config.Hidden, "hidden", false, "Search hidden files and directories")
	rootCmd.Flags().BoolVar(&config.NoIgnore, "no-ignore", false, "Don't respect ignore files")
	rootCmd.Flags().BoolVar(&config.NoGlobal, "no-ignore-global", false, "Don't respect global ignore files")
	rootCmd.Flags().BoolVar(&config.NoParent, "no-ignore-parent", false, "Don't respect parent ignore files")
	rootCmd.Flags().BoolVar(&config.NoVcs, "no-ignore-vcs", false, "Don't respect VCS ignore files")

	rootCmd.Flags().BoolVarP(&config.Invert, "invert-match", "v", false, "Invert matching")
	rootCmd.Flags().BoolVarP(&config.Quiet, "quiet", "q", false, "Suppress normal output")
	rootCmd.Flags().BoolVarP(&config.Binary, "binary", "a", false, "Search binary files")
	rootCmd.Flags().StringVarP(&config.Replace, "replace", "r", "", "Replace matches with string")
	rootCmd.Flags().StringVarP(&config.Encoding, "encoding", "E", "auto", "Text encoding to use")

	rootCmd.Flags().BoolVar(&config.Symbols, "symbols", false, "Find symbol definitions")
	rootCmd.Flags().BoolVar(&config.Refs, "refs", false, "Find symbol references")
	rootCmd.Flags().BoolVar(&config.Types, "types", false, "Find type definitions")
	rootCmd.Flags().BoolVar(&config.CallGraph, "call-graph", false, "Show call graph relationships")

	rootCmd.Flags().BoolVar(&config.NoIndex, "no-index", false, "Disable index usage for semantic search")
	rootCmd.Flags().StringVar(&config.IndexPath, "index-path", "", "Custom index location")
	rootCmd.Flags().BoolVar(&config.RebuildIndex, "rebuild-index", false, "Force rebuild of semantic index")

	viper.BindPFlags(rootCmd.Flags())
}

func initConfig() {
	// Set config file name and paths
	viper.SetConfigName(".codegrep")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME")

	// Environment variable support
	viper.SetEnvPrefix("CODEGREP")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Read config file if it exists
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func runSearch(cmd *cobra.Command, args []string) error {
	// Validate semantic flag combinations
	if err := validateSemanticFlags(); err != nil {
		return err
	}

	// Initialize search engine based on flags
	engine, err := NewRealSearchEngine(&config)
	if err != nil {
		return fmt.Errorf("failed to initialize search engine: %w", err)
	}
	defer engine.Close()

	// Execute the search
	results, err := engine.Search(cmd.Context(), &config)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Format and output results
	formatter := NewRealOutputFormatter(&config)
	return formatter.Output(os.Stdout, results)
}

func validateSemanticFlags() error {
	semanticFlags := []bool{config.Symbols, config.Refs, config.Types, config.CallGraph}
	semanticCount := 0
	for _, flag := range semanticFlags {
		if flag {
			semanticCount++
		}
	}

	// Only one semantic flag can be used at a time
	if semanticCount > 1 {
		return fmt.Errorf("only one semantic flag can be used at a time (--symbols, --refs, --types, --call-graph)")
	}

	// Semantic flags require a pattern (symbol name)
	if semanticCount > 0 && config.Pattern == "" {
		return fmt.Errorf("semantic search requires a pattern (symbol name)")
	}

	// Some ripgrep flags don't make sense with semantic search
	if semanticCount > 0 {
		if config.FixedStrings {
			return fmt.Errorf("--fixed-strings cannot be used with semantic search")
		}
		if config.PCREMode {
			return fmt.Errorf("--pcre2 cannot be used with semantic search")
		}
		if config.Replace != "" {
			return fmt.Errorf("--replace cannot be used with semantic search")
		}
	}

	return nil
}

type SearchEngine interface {
	Search(ctx context.Context, config *Config) (*SearchResults, error)
	Close() error
}

type OutputFormatter interface {
	Output(writer io.Writer, results interface{}) error
}