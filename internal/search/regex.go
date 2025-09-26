package search

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"golang.org/x/sync/semaphore"
)

// RegexSearcher implements high-performance regex search with ripgrep compatibility
type RegexSearcher struct {
	patterns map[string]*regexp.Regexp
	patternsMutex sync.RWMutex

	stats       SearchStats
	statsMutex  sync.RWMutex
	filesProcessed int64
	bytesProcessed int64
	matchCount     int64

	semaphore *semaphore.Weighted
}

func NewRegexSearcher() (*RegexSearcher, error) {
	return &RegexSearcher{
		patterns:  make(map[string]*regexp.Regexp),
		semaphore: semaphore.NewWeighted(int64(runtime.NumCPU() * 2)),
	}, nil
}

func (rs *RegexSearcher) Search(ctx context.Context, opts *SearchOptions) (<-chan SearchResult, <-chan error) {
	results := make(chan SearchResult, 100)
	errors := make(chan error, 10)

	go func() {
		defer close(results)
		defer close(errors)

		startTime := time.Now()

		// Reset statistics
		rs.resetStats()

		// Compile regex pattern
		pattern, err := rs.compilePattern(opts)
		if err != nil {
			errors <- fmt.Errorf("failed to compile pattern: %w", err)
			return
		}

		// Collect files to search
		files, err := rs.collectFiles(opts)
		if err != nil {
			errors <- fmt.Errorf("failed to collect files: %w", err)
			return
		}

		rs.updateStats(func(stats *SearchStats) {
			stats.TotalFiles = len(files)
		})

		// Process files in parallel
		var wg sync.WaitGroup
		workers := opts.MaxWorkers
		if workers <= 0 {
			workers = runtime.NumCPU()
		}

		for _, file := range files {
			if err := rs.semaphore.Acquire(ctx, 1); err != nil {
				errors <- fmt.Errorf("failed to acquire semaphore: %w", err)
				return
			}

			wg.Add(1)
			go func(filePath string) {
				defer wg.Done()
				defer rs.semaphore.Release(1)

				if err := rs.searchFile(ctx, filePath, pattern, opts, results); err != nil {
					select {
					case errors <- fmt.Errorf("error searching %s: %w", filePath, err):
					case <-ctx.Done():
						return
					}
				}
			}(file)
		}

		wg.Wait()

		// Update final statistics
		rs.updateStats(func(stats *SearchStats) {
			stats.SearchDuration = time.Since(startTime)
			stats.TotalMatches = int(atomic.LoadInt64(&rs.matchCount))
			stats.FilesSearched = int(atomic.LoadInt64(&rs.filesProcessed))
			stats.BytesSearched = atomic.LoadInt64(&rs.bytesProcessed)
		})
	}()

	return results, errors
}

func (rs *RegexSearcher) compilePattern(opts *SearchOptions) (*regexp.Regexp, error) {
	patternKey := rs.getPatternKey(opts)

	rs.patternsMutex.RLock()
	if pattern, exists := rs.patterns[patternKey]; exists {
		rs.patternsMutex.RUnlock()
		return pattern, nil
	}
	rs.patternsMutex.RUnlock()

	// Build regex flags
	flags := ""
	if !opts.CaseSensitive {
		flags += "i"
	}
	if opts.Multiline {
		flags += "m"
	}
	if opts.DotMatchAll {
		flags += "s"
	}

	// Prepare pattern
	pattern := opts.Pattern
	if opts.WholeWord {
		pattern = `\b` + pattern + `\b`
	}

	// Add flags prefix if needed
	if flags != "" {
		pattern = "(?" + flags + ")" + pattern
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern '%s': %w", opts.Pattern, err)
	}

	rs.patternsMutex.Lock()
	rs.patterns[patternKey] = compiled
	rs.patternsMutex.Unlock()

	return compiled, nil
}

func (rs *RegexSearcher) getPatternKey(opts *SearchOptions) string {
	var keyParts []string
	keyParts = append(keyParts, opts.Pattern)

	if !opts.CaseSensitive {
		keyParts = append(keyParts, "case_insensitive")
	}
	if opts.WholeWord {
		keyParts = append(keyParts, "whole_word")
	}
	if opts.Multiline {
		keyParts = append(keyParts, "multiline")
	}
	if opts.DotMatchAll {
		keyParts = append(keyParts, "dot_all")
	}

	return strings.Join(keyParts, "|")
}

func (rs *RegexSearcher) collectFiles(opts *SearchOptions) ([]string, error) {
	var files []string

	for _, path := range opts.SearchPaths {
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip files with errors
			}

			if info.IsDir() {
				return nil
			}

			// Skip large files
			if opts.MaxFileSize > 0 && info.Size() > opts.MaxFileSize {
				return nil
			}

			// Apply file type filters
			if !rs.shouldIncludeFile(filePath, opts) {
				return nil
			}

			files = append(files, filePath)
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error walking path %s: %w", path, err)
		}
	}

	return files, nil
}

func (rs *RegexSearcher) shouldIncludeFile(filePath string, opts *SearchOptions) bool {
	// Check file type filters
	if len(opts.FileTypes) > 0 {
		ext := strings.ToLower(filepath.Ext(filePath))
		found := false
		for _, fileType := range opts.FileTypes {
			if strings.HasSuffix(ext, strings.ToLower(fileType)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check glob patterns
	if len(opts.Globs) > 0 {
		matched := false
		for _, glob := range opts.Globs {
			if match, _ := filepath.Match(glob, filepath.Base(filePath)); match {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check exclude patterns
	for _, excludeGlob := range opts.ExcludeGlobs {
		if match, _ := filepath.Match(excludeGlob, filepath.Base(filePath)); match {
			return false
		}
	}

	return true
}

func (rs *RegexSearcher) searchFile(ctx context.Context, filePath string, pattern *regexp.Regexp,
	opts *SearchOptions, results chan<- SearchResult) error {

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Check if file is binary
	if rs.isBinaryFile(file) {
		return nil
	}

	// Reset file position
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	atomic.AddInt64(&rs.filesProcessed, 1)

	// Use multiline search if specified
	if opts.Multiline {
		return rs.searchFileMultiline(file, filePath, pattern, opts, results)
	}

	if opts.Count || opts.FilesWithMatches || opts.FilesWithoutMatches {
		return rs.searchFileCountOnly(file, filePath, pattern, opts, results)
	}

	return rs.searchFileWithContext(file, filePath, pattern, opts, results)
}

func (rs *RegexSearcher) isBinaryFile(file *os.File) bool {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return true
	}

	// Check for null bytes or other binary indicators
	for i := 0; i < n; i++ {
		if buf[i] == 0 || (buf[i] < 32 && buf[i] != '\t' && buf[i] != '\n' && buf[i] != '\r') {
			return true
		}
	}

	// Check for valid UTF-8
	return !utf8.Valid(buf[:n])
}

func (rs *RegexSearcher) searchFileCountOnly(file *os.File, filePath string, pattern *regexp.Regexp,
	opts *SearchOptions, results chan<- SearchResult) error {

	scanner := bufio.NewScanner(file)
	matchCount := 0
	hasMatches := false

	for scanner.Scan() {
		line := scanner.Text()
		atomic.AddInt64(&rs.bytesProcessed, int64(len(line)+1))

		matches := pattern.FindAllString(line, -1)
		if len(matches) > 0 {
			hasMatches = true
			if opts.OnlyMatching {
				matchCount += len(matches)
			} else {
				matchCount++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning file: %w", err)
	}

	// Send results based on options
	if opts.Count {
		results <- SearchResult{
			FilePath: filePath,
			Match:    fmt.Sprintf("%d", matchCount),
		}
	} else if (opts.FilesWithMatches && hasMatches) || (opts.FilesWithoutMatches && !hasMatches) {
		results <- SearchResult{
			FilePath: filePath,
		}
	}

	if hasMatches {
		atomic.AddInt64(&rs.matchCount, int64(matchCount))
	}

	return nil
}

func (rs *RegexSearcher) searchFileWithContext(file *os.File, filePath string, pattern *regexp.Regexp,
	opts *SearchOptions, results chan<- SearchResult) error {

	scanner := bufio.NewScanner(file)
	lines := make([]string, 0, 1000)

	// Read all lines first for context processing
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		atomic.AddInt64(&rs.bytesProcessed, int64(len(line)+1))
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning file: %w", err)
	}

	// Process lines for matches
	contextBefore := opts.ContextBefore
	contextAfter := opts.ContextAfter
	if opts.Context > 0 {
		contextBefore = opts.Context
		contextAfter = opts.Context
	}

	for lineNum, line := range lines {
		var matches [][]int
		if opts.InvertMatch {
			if pattern.MatchString(line) {
				continue
			}
			// For inverted matches, the entire line is a "match"
			matches = [][]int{{0, len(line)}}
		} else {
			matches = pattern.FindAllStringIndex(line, -1)
			if len(matches) == 0 {
				continue
			}
		}

		atomic.AddInt64(&rs.matchCount, int64(len(matches)))

		// Extract context lines
		var contextLines []string
		if contextBefore > 0 || contextAfter > 0 {
			startLine := maxInt(0, lineNum-contextBefore)
			endLine := minInt(len(lines)-1, lineNum+contextAfter)

			for i := startLine; i <= endLine; i++ {
				if i != lineNum {
					contextLines = append(contextLines, lines[i])
				}
			}
		}

		// Generate results for each match
		if opts.OnlyMatching {
			for _, match := range matches {
				matchText := line[match[0]:match[1]]
				results <- SearchResult{
					FilePath:    filePath,
					LineNumber:  lineNum + 1,
					ColumnStart: match[0] + 1,
					ColumnEnd:   match[1] + 1,
					Match:       matchText,
					Context:     contextLines,
				}
			}
		} else {
			// Return the whole line with first match position
			firstMatch := matches[0]
			results <- SearchResult{
				FilePath:    filePath,
				LineNumber:  lineNum + 1,
				ColumnStart: firstMatch[0] + 1,
				ColumnEnd:   firstMatch[1] + 1,
				Line:        line,
				Match:       line[firstMatch[0]:firstMatch[1]],
				Context:     contextLines,
			}
		}
	}

	return nil
}

func (rs *RegexSearcher) resetStats() {
	rs.statsMutex.Lock()
	defer rs.statsMutex.Unlock()

	rs.stats = SearchStats{}
	atomic.StoreInt64(&rs.filesProcessed, 0)
	atomic.StoreInt64(&rs.bytesProcessed, 0)
	atomic.StoreInt64(&rs.matchCount, 0)
}

func (rs *RegexSearcher) updateStats(fn func(*SearchStats)) {
	rs.statsMutex.Lock()
	defer rs.statsMutex.Unlock()
	fn(&rs.stats)
}

func (rs *RegexSearcher) Stats() SearchStats {
	rs.statsMutex.RLock()
	defer rs.statsMutex.RUnlock()

	stats := rs.stats
	stats.FilesSearched = int(atomic.LoadInt64(&rs.filesProcessed))
	stats.BytesSearched = atomic.LoadInt64(&rs.bytesProcessed)
	stats.TotalMatches = int(atomic.LoadInt64(&rs.matchCount))

	// Get current memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	stats.PeakMemoryUsage = int64(m.HeapInuse)

	return stats
}

func (rs *RegexSearcher) Close() error {
	rs.patternsMutex.Lock()
	defer rs.patternsMutex.Unlock()

	// Clear pattern cache
	rs.patterns = make(map[string]*regexp.Regexp)

	return nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (rs *RegexSearcher) searchFileMultiline(file *os.File, filePath string, pattern *regexp.Regexp,
	opts *SearchOptions, results chan<- SearchResult) error {

	// Read entire file for multiline matching
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	atomic.AddInt64(&rs.bytesProcessed, int64(len(content)))

	if rs.isBinaryContent(content) {
		return nil
	}

	contentStr := string(content)
	matches := pattern.FindAllStringIndex(contentStr, -1)

	if len(matches) == 0 && !opts.InvertMatch {
		return nil
	}

	if opts.InvertMatch && len(matches) > 0 {
		return nil
	}

	atomic.AddInt64(&rs.matchCount, int64(len(matches)))

	// Convert byte positions to line/column positions
	lines := strings.Split(contentStr, "\n")
	byteToLineCol := rs.buildByteToLineColMap(lines)

	for _, match := range matches {
		startPos := byteToLineCol[match[0]]
		endPos := byteToLineCol[match[1]-1]
		startLine, startCol := startPos.line, startPos.col
		endLine, endCol := endPos.line, endPos.col

		matchText := contentStr[match[0]:match[1]]

		result := SearchResult{
			FilePath:    filePath,
			LineNumber:  startLine + 1,
			ColumnStart: startCol + 1,
			ColumnEnd:   endCol + 1,
			Match:       matchText,
		}

		// Add context if requested
		if opts.Context > 0 || opts.ContextBefore > 0 || opts.ContextAfter > 0 {
			contextBefore := opts.Context
			contextAfter := opts.Context
			if opts.ContextBefore > 0 {
				contextBefore = opts.ContextBefore
			}
			if opts.ContextAfter > 0 {
				contextAfter = opts.ContextAfter
			}

			result.Context = rs.extractContextLines(lines, startLine, endLine, contextBefore, contextAfter)
		}

		results <- result
	}

	return nil
}

func (rs *RegexSearcher) buildByteToLineColMap(lines []string) map[int]struct{ line, col int } {
	byteToLineCol := make(map[int]struct{ line, col int })
	bytePos := 0

	for lineNum, line := range lines {
		for col := 0; col < len(line); col++ {
			byteToLineCol[bytePos] = struct{ line, col int }{lineNum, col}
			bytePos++
		}
		// Account for newline character
		byteToLineCol[bytePos] = struct{ line, col int }{lineNum, len(line)}
		bytePos++
	}

	return byteToLineCol
}

func (rs *RegexSearcher) extractContextLines(lines []string, startLine, endLine, contextBefore, contextAfter int) []string {
	var context []string

	// Add lines before
	for i := maxInt(0, startLine-contextBefore); i < startLine; i++ {
		context = append(context, lines[i])
	}

	// Add lines after
	for i := endLine + 1; i <= minInt(len(lines)-1, endLine+contextAfter); i++ {
		context = append(context, lines[i])
	}

	return context
}

func (rs *RegexSearcher) isBinaryContent(content []byte) bool {
	// Check first 8KB for binary indicators
	checkSize := minInt(len(content), 8192)

	for i := 0; i < checkSize; i++ {
		if content[i] == 0 {
			return true
		}
	}

	return !utf8.Valid(content[:checkSize])
}