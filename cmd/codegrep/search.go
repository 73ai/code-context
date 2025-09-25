package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// SearchResult represents a single search result
type SearchResult struct {
	File       string            `json:"file"`
	LineNumber int               `json:"line_number,omitempty"`
	Column     int               `json:"column,omitempty"`
	Match      string            `json:"match"`
	Context    map[string]string `json:"context,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// SearchResults holds the complete search results
type SearchResults struct {
	Results   []SearchResult `json:"results"`
	Stats     SearchStats    `json:"stats"`
	Semantic  bool           `json:"semantic"`
	Pattern   string         `json:"pattern"`
	Timestamp time.Time      `json:"timestamp"`
}

// SearchStats contains search statistics
type SearchStats struct {
	FilesSearched int           `json:"files_searched"`
	LinesSearched int           `json:"lines_searched"`
	Matches       int           `json:"matches"`
	Duration      time.Duration `json:"duration"`
	Engine        string        `json:"engine"`
}

// RealSearchEngine implements the SearchEngine interface
type RealSearchEngine struct {
	config     *Config
	regexCache map[string]*regexp.Regexp
	mu         sync.RWMutex
}

// NewRealSearchEngine creates a new search engine
func NewRealSearchEngine(config *Config) (*RealSearchEngine, error) {
	engine := &RealSearchEngine{
		config:     config,
		regexCache: make(map[string]*regexp.Regexp),
	}

	// Validate configuration
	if err := engine.validateConfig(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return engine, nil
}

// Search executes the search based on configuration
func (e *RealSearchEngine) Search(ctx context.Context, config *Config) (*SearchResults, error) {
	startTime := time.Now()

	results := &SearchResults{
		Pattern:   config.Pattern,
		Timestamp: startTime,
		Semantic:  e.isSemanticSearch(),
	}

	if e.isSemanticSearch() {
		return e.searchSemantic(ctx, results)
	}

	return e.searchRegex(ctx, results)
}

// Close cleans up resources
func (e *RealSearchEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear regex cache
	e.regexCache = make(map[string]*regexp.Regexp)
	return nil
}

func (e *RealSearchEngine) validateConfig() error {
	if e.config.Pattern == "" && !e.isSemanticSearch() {
		return fmt.Errorf("pattern is required for regex search")
	}

	if e.config.Threads < 0 {
		return fmt.Errorf("threads must be >= 0")
	}

	if e.config.MaxDepth < 0 {
		return fmt.Errorf("max-depth must be >= 0")
	}

	return nil
}

func (e *RealSearchEngine) isSemanticSearch() bool {
	return e.config.Symbols || e.config.Refs || e.config.Types || e.config.CallGraph
}

func (e *RealSearchEngine) searchRegex(ctx context.Context, results *SearchResults) (*SearchResults, error) {
	// Compile regex pattern
	pattern, err := e.compilePattern()
	if err != nil {
		return nil, fmt.Errorf("failed to compile pattern: %w", err)
	}

	// Set up worker pool
	numWorkers := e.config.Threads
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	// Channel for files to process
	fileChan := make(chan string, numWorkers*2)
	resultChan := make(chan []SearchResult, numWorkers)
	errorChan := make(chan error, numWorkers)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go e.regexWorker(ctx, &wg, pattern, fileChan, resultChan, errorChan)
	}

	// Start file walker
	go func() {
		defer close(fileChan)
		e.walkFiles(ctx, fileChan)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// Process results
	var allResults []SearchResult
	var filesSearched int

	for workerResults := range resultChan {
		allResults = append(allResults, workerResults...)
		filesSearched++
	}

	// Check for errors
	select {
	case err := <-errorChan:
		if err != nil {
			return nil, fmt.Errorf("search error: %w", err)
		}
	default:
	}

	results.Results = allResults
	results.Stats = SearchStats{
		FilesSearched: filesSearched,
		Matches:       len(allResults),
		Duration:      time.Since(results.Timestamp),
		Engine:        "regex",
	}

	return results, nil
}

func (e *RealSearchEngine) searchSemantic(ctx context.Context, results *SearchResults) (*SearchResults, error) {
	// Semantic search implementation would go here
	// For now, return a placeholder
	results.Results = []SearchResult{
		{
			File:     "example.go",
			LineNumber: 42,
			Match:    fmt.Sprintf("semantic match for '%s'", e.config.Pattern),
			Metadata: map[string]string{
				"type": e.getSemanticType(),
				"scope": "function",
			},
		},
	}

	results.Stats = SearchStats{
		FilesSearched: 1,
		Matches:       1,
		Duration:      time.Since(results.Timestamp),
		Engine:        "semantic",
	}

	return results, nil
}

func (e *RealSearchEngine) getSemanticType() string {
	if e.config.Symbols {
		return "symbol"
	}
	if e.config.Refs {
		return "reference"
	}
	if e.config.Types {
		return "type"
	}
	if e.config.CallGraph {
		return "call"
	}
	return "unknown"
}

func (e *RealSearchEngine) compilePattern() (*regexp.Regexp, error) {
	e.mu.RLock()
	if cached, ok := e.regexCache[e.config.Pattern]; ok {
		e.mu.RUnlock()
		return cached, nil
	}
	e.mu.RUnlock()

	pattern := e.config.Pattern

	// Handle fixed strings
	if e.config.FixedStrings {
		pattern = regexp.QuoteMeta(pattern)
	}

	// Handle word boundaries
	if e.config.WordRegexp {
		pattern = `\b` + pattern + `\b`
	}

	// Handle line boundaries
	if e.config.LineRegexp {
		pattern = `^` + pattern + `$`
	}

	// Handle case insensitivity and multiline modes with inline flags
	var flagStr strings.Builder
	if e.config.IgnoreCase {
		flagStr.WriteString("i")
	}
	if e.config.Multiline {
		flagStr.WriteString("m")
	}
	if e.config.DotMatchesAll {
		flagStr.WriteString("s")
	}

	// Create pattern with inline flags if any are set
	if flagStr.Len() > 0 {
		pattern = fmt.Sprintf("(?%s)%s", flagStr.String(), pattern)
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	// Cache the compiled regex
	e.mu.Lock()
	e.regexCache[e.config.Pattern] = compiled
	e.mu.Unlock()

	return compiled, nil
}


func (e *RealSearchEngine) regexWorker(
	ctx context.Context,
	wg *sync.WaitGroup,
	pattern *regexp.Regexp,
	fileChan <-chan string,
	resultChan chan<- []SearchResult,
	errorChan chan<- error,
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case filePath, ok := <-fileChan:
			if !ok {
				return
			}

			results, err := e.searchFile(filePath, pattern)
			if err != nil {
				select {
				case errorChan <- err:
				case <-ctx.Done():
				}
				return
			}

			if len(results) > 0 {
				select {
				case resultChan <- results:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (e *RealSearchEngine) searchFile(filePath string, pattern *regexp.Regexp) ([]SearchResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Check file size limit
	if e.config.MaxFilesize > 0 {
		info, err := file.Stat()
		if err != nil {
			return nil, err
		}
		if info.Size() > int64(e.config.MaxFilesize) {
			return nil, nil // Skip file
		}
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Skip binary files unless explicitly requested
	if !e.config.Binary && isBinary(content) {
		return nil, nil
	}

	return e.findMatches(filePath, string(content), pattern), nil
}

func (e *RealSearchEngine) findMatches(filePath, content string, pattern *regexp.Regexp) []SearchResult {
	var results []SearchResult
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		if e.config.MaxCount > 0 && len(results) >= e.config.MaxCount {
			break
		}

		matches := pattern.FindAllStringIndex(line, -1)
		if e.config.Invert {
			if len(matches) == 0 {
				// Line doesn't match, include it (inverted)
				results = append(results, SearchResult{
					File:       filePath,
					LineNumber: lineNum + 1,
					Match:      line,
				})
			}
		} else {
			for _, match := range matches {
				result := SearchResult{
					File:       filePath,
					LineNumber: lineNum + 1,
					Column:     match[0] + 1,
				}

				if e.config.OnlyMatching {
					result.Match = line[match[0]:match[1]]
				} else {
					result.Match = line
				}

				// Add context if requested
				if e.config.Context > 0 || e.config.ContextBefore > 0 || e.config.ContextAfter > 0 {
					result.Context = e.getContext(lines, lineNum)
				}

				results = append(results, result)

				if e.config.MaxCount > 0 && len(results) >= e.config.MaxCount {
					break
				}
			}
		}
	}

	return results
}

func (e *RealSearchEngine) getContext(lines []string, lineNum int) map[string]string {
	context := make(map[string]string)

	beforeLines := e.config.ContextBefore
	if e.config.Context > 0 {
		beforeLines = e.config.Context
	}

	afterLines := e.config.ContextAfter
	if e.config.Context > 0 {
		afterLines = e.config.Context
	}

	// Before context
	if beforeLines > 0 {
		start := max(0, lineNum-beforeLines)
		var beforeContext []string
		for i := start; i < lineNum; i++ {
			beforeContext = append(beforeContext, lines[i])
		}
		if len(beforeContext) > 0 {
			context["before"] = strings.Join(beforeContext, "\n")
		}
	}

	// After context
	if afterLines > 0 {
		end := min(len(lines), lineNum+1+afterLines)
		var afterContext []string
		for i := lineNum + 1; i < end; i++ {
			afterContext = append(afterContext, lines[i])
		}
		if len(afterContext) > 0 {
			context["after"] = strings.Join(afterContext, "\n")
		}
	}

	return context
}

func (e *RealSearchEngine) walkFiles(ctx context.Context, fileChan chan<- string) {
	for _, path := range e.config.Paths {
		if err := e.walkPath(ctx, path, fileChan); err != nil {
			fmt.Fprintf(os.Stderr, "Error walking path %s: %v\n", path, err)
		}
	}
}

func (e *RealSearchEngine) walkPath(ctx context.Context, root string, fileChan chan<- string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if d.IsDir() {
			// Check max depth
			if e.config.MaxDepth > 0 {
				depth := strings.Count(strings.TrimPrefix(path, root), string(os.PathSeparator))
				if depth >= e.config.MaxDepth {
					return filepath.SkipDir
				}
			}

			// Skip hidden directories unless requested
			if !e.config.Hidden && isHidden(d.Name()) {
				return filepath.SkipDir
			}

			return nil
		}

		// Skip hidden files unless requested
		if !e.config.Hidden && isHidden(d.Name()) {
			return nil
		}

		// Apply file filters
		if !e.shouldSearchFile(path) {
			return nil
		}

		select {
		case fileChan <- path:
		case <-ctx.Done():
			return ctx.Err()
		}

		return nil
	})
}

func (e *RealSearchEngine) shouldSearchFile(path string) bool {
	// Check type filters
	if len(e.config.Type) > 0 {
		if !e.matchesType(path, e.config.Type) {
			return false
		}
	}

	if len(e.config.TypeNot) > 0 {
		if e.matchesType(path, e.config.TypeNot) {
			return false
		}
	}

	// Check glob filters
	if len(e.config.Glob) > 0 {
		if !e.matchesGlob(path, e.config.Glob) {
			return false
		}
	}

	if len(e.config.GlobNot) > 0 {
		if e.matchesGlob(path, e.config.GlobNot) {
			return false
		}
	}

	return true
}

func (e *RealSearchEngine) matchesType(path string, types []string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, typ := range types {
		switch typ {
		case "go":
			if ext == ".go" {
				return true
			}
		case "py", "python":
			if ext == ".py" {
				return true
			}
		case "js", "javascript":
			if ext == ".js" {
				return true
			}
		case "ts", "typescript":
			if ext == ".ts" || ext == ".tsx" {
				return true
			}
		case "rs", "rust":
			if ext == ".rs" {
				return true
			}
		case "c":
			if ext == ".c" || ext == ".h" {
				return true
			}
		case "cpp", "cxx":
			if ext == ".cpp" || ext == ".cxx" || ext == ".cc" || ext == ".hpp" {
				return true
			}
		case "java":
			if ext == ".java" {
				return true
			}
		}
	}
	return false
}

func (e *RealSearchEngine) matchesGlob(path string, globs []string) bool {
	for _, glob := range globs {
		if matched, _ := filepath.Match(glob, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

// Utility functions
func isBinary(content []byte) bool {
	// Simple heuristic: if file contains null bytes, consider it binary
	for _, b := range content[:min(8192, len(content))] {
		if b == 0 {
			return true
		}
	}
	return false
}

func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Update the newSearchEngine function in root.go to use RealSearchEngine
func newSearchEngine(config *Config) (SearchEngine, error) {
	return NewRealSearchEngine(config)
}