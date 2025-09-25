package search

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"
)

// SearchMode defines the type of search to perform
type SearchMode int

const (
	ModeRegex SearchMode = iota
	ModeSemantic
	ModeHybrid
)

// SearchOptions defines configuration for search operations
type SearchOptions struct {
	// Core search configuration
	Pattern     string
	SearchPaths []string
	SearchMode  SearchMode

	// Regex options (ripgrep compatible)
	CaseSensitive    bool
	WholeWord        bool
	InvertMatch      bool
	OnlyMatching     bool
	Count            bool
	FilesWithMatches bool
	FilesWithoutMatches bool

	// Context options
	ContextBefore int
	ContextAfter  int
	Context       int

	// Multiline options
	Multiline    bool
	DotMatchAll  bool

	// File filtering
	FileTypes    []string
	Globs        []string
	ExcludeGlobs []string

	// Performance options
	MaxWorkers   int
	MaxFileSize  int64
	Timeout      time.Duration

	// Output options
	LineNumbers  bool
	WithFilename bool
	NoHeading    bool
	JSON         bool
	OutputPath   string

	// Semantic options
	SymbolTypes  []string
	FindRefs     bool
	FindDefs     bool
	Scoped       bool
	CrossLang    bool
}

// SearchResult represents a single search match
type SearchResult struct {
	FilePath    string            `json:"file_path"`
	LineNumber  int               `json:"line_number,omitempty"`
	ColumnStart int               `json:"column_start,omitempty"`
	ColumnEnd   int               `json:"column_end,omitempty"`
	Line        string            `json:"line,omitempty"`
	Match       string            `json:"match,omitempty"`
	Context     []string          `json:"context,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`

	// Semantic-specific fields
	SymbolName  string `json:"symbol_name,omitempty"`
	SymbolType  string `json:"symbol_type,omitempty"`
	SymbolKind  string `json:"symbol_kind,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

// SearchStats provides statistics about the search operation
type SearchStats struct {
	FilesSearched    int           `json:"files_searched"`
	FilesSkipped     int           `json:"files_skipped"`
	TotalMatches     int           `json:"total_matches"`
	TotalFiles       int           `json:"total_files"`
	SearchDuration   time.Duration `json:"search_duration"`
	IndexDuration    time.Duration `json:"index_duration,omitempty"`
	BytesSearched    int64         `json:"bytes_searched"`
	PeakMemoryUsage  int64         `json:"peak_memory_usage"`
}

// Searcher interface defines the contract for search implementations
type Searcher interface {
	Search(ctx context.Context, opts *SearchOptions) (<-chan SearchResult, <-chan error)
	Stats() SearchStats
	Close() error
}

// Engine coordinates between different search implementations
type Engine struct {
	regexSearcher    Searcher
	semanticSearcher Searcher

	// Performance monitoring
	stats      SearchStats
	statsMutex sync.RWMutex
}

// NewEngine creates a new search engine with the specified configuration
func NewEngine() (*Engine, error) {
	// Initialize regex searcher
	regexSearcher, err := NewRegexSearcher()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize regex searcher: %w", err)
	}

	// Initialize semantic searcher
	semanticSearcher, err := NewSemanticSearcher()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize semantic searcher: %w", err)
	}

	return &Engine{
		regexSearcher:    regexSearcher,
		semanticSearcher: semanticSearcher,
		stats:            SearchStats{},
	}, nil
}

// Search performs a search operation based on the provided options
func (e *Engine) Search(ctx context.Context, opts *SearchOptions, output io.Writer) error {
	startTime := time.Now()

	// Set default options
	if opts.MaxWorkers == 0 {
		opts.MaxWorkers = runtime.NumCPU()
	}
	if opts.MaxFileSize == 0 {
		opts.MaxFileSize = 100 * 1024 * 1024 // 100MB default
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	// Create context with timeout
	searchCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Reset stats
	e.statsMutex.Lock()
	e.stats = SearchStats{}
	e.statsMutex.Unlock()

	var searcher Searcher
	switch opts.SearchMode {
	case ModeRegex:
		searcher = e.regexSearcher
	case ModeSemantic:
		searcher = e.semanticSearcher
	case ModeHybrid:
		return e.searchHybrid(searchCtx, opts, output)
	default:
		return fmt.Errorf("unsupported search mode: %v", opts.SearchMode)
	}

	// Start the search
	results, errs := searcher.Search(searchCtx, opts)

	// Process results
	formatter := NewFormatter(opts, output)
	defer formatter.Close()

	var searchErr error
	totalMatches := 0

	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
				break
			}

			if err := formatter.FormatResult(&result); err != nil {
				return fmt.Errorf("failed to format result: %w", err)
			}
			totalMatches++

		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}

			if searchErr == nil {
				searchErr = err
			}

		case <-searchCtx.Done():
			return fmt.Errorf("search timeout exceeded: %w", searchCtx.Err())
		}

		if results == nil && errs == nil {
			break
		}
	}

	// Update final stats
	searcherStats := searcher.Stats()
	e.statsMutex.Lock()
	e.stats = searcherStats
	e.stats.TotalMatches = totalMatches
	e.stats.SearchDuration = time.Since(startTime)
	e.statsMutex.Unlock()

	// Format final stats if requested
	if opts.JSON {
		if err := formatter.FormatStats(&e.stats); err != nil {
			return fmt.Errorf("failed to format stats: %w", err)
		}
	}

	return searchErr
}

// searchHybrid performs a hybrid search using both regex and semantic searchers
func (e *Engine) searchHybrid(ctx context.Context, opts *SearchOptions, output io.Writer) error {
	// First, perform regex search for fast results
	regexOpts := *opts
	regexOpts.SearchMode = ModeRegex

	regexResults, regexErrs := e.regexSearcher.Search(ctx, &regexOpts)

	// Then, perform semantic search for enhanced results
	semanticOpts := *opts
	semanticOpts.SearchMode = ModeSemantic

	semanticResults, semanticErrs := e.semanticSearcher.Search(ctx, &semanticOpts)

	// Merge and deduplicate results
	formatter := NewFormatter(opts, output)
	defer formatter.Close()

	seen := make(map[string]bool)
	var searchErr error
	totalMatches := 0

	for {
		select {
		case result, ok := <-regexResults:
			if !ok {
				regexResults = nil
				break
			}

			key := fmt.Sprintf("%s:%d:%d", result.FilePath, result.LineNumber, result.ColumnStart)
			if !seen[key] {
				seen[key] = true
				if err := formatter.FormatResult(&result); err != nil {
					return fmt.Errorf("failed to format regex result: %w", err)
				}
				totalMatches++
			}

		case result, ok := <-semanticResults:
			if !ok {
				semanticResults = nil
				break
			}

			key := fmt.Sprintf("%s:%d:%d", result.FilePath, result.LineNumber, result.ColumnStart)
			if !seen[key] {
				seen[key] = true
				if err := formatter.FormatResult(&result); err != nil {
					return fmt.Errorf("failed to format semantic result: %w", err)
				}
				totalMatches++
			}

		case err, ok := <-regexErrs:
			if !ok {
				regexErrs = nil
				break
			}
			if searchErr == nil {
				searchErr = err
			}

		case err, ok := <-semanticErrs:
			if !ok {
				semanticErrs = nil
				break
			}
			if searchErr == nil {
				searchErr = err
			}

		case <-ctx.Done():
			return fmt.Errorf("hybrid search timeout: %w", ctx.Err())
		}

		if regexResults == nil && semanticResults == nil && regexErrs == nil && semanticErrs == nil {
			break
		}
	}

	// Merge stats from both searchers
	regexStats := e.regexSearcher.Stats()
	semanticStats := e.semanticSearcher.Stats()

	e.statsMutex.Lock()
	e.stats = SearchStats{
		FilesSearched:   regexStats.FilesSearched + semanticStats.FilesSearched,
		FilesSkipped:    regexStats.FilesSkipped + semanticStats.FilesSkipped,
		TotalMatches:    totalMatches,
		TotalFiles:      regexStats.TotalFiles,
		SearchDuration:  regexStats.SearchDuration + semanticStats.SearchDuration,
		IndexDuration:   semanticStats.IndexDuration,
		BytesSearched:   regexStats.BytesSearched + semanticStats.BytesSearched,
		PeakMemoryUsage: maxInt64(regexStats.PeakMemoryUsage, semanticStats.PeakMemoryUsage),
	}
	e.statsMutex.Unlock()

	return searchErr
}

// Stats returns current search statistics
func (e *Engine) Stats() SearchStats {
	e.statsMutex.RLock()
	defer e.statsMutex.RUnlock()
	return e.stats
}

// Close cleans up resources used by the engine
func (e *Engine) Close() error {
	var errs []error

	if e.regexSearcher != nil {
		if err := e.regexSearcher.Close(); err != nil {
			errs = append(errs, fmt.Errorf("regex searcher close: %w", err))
		}
	}

	if e.semanticSearcher != nil {
		if err := e.semanticSearcher.Close(); err != nil {
			errs = append(errs, fmt.Errorf("semantic searcher close: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("engine close errors: %v", errs)
	}

	return nil
}

// Utility functions
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}