package index

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// Builder handles incremental index building with parallel processing
type Builder struct {
	store      *Store
	config     BuilderConfig
	parser     SymbolParser
	progress   *BuildProgress
	cancelFunc context.CancelFunc
}

// BuilderConfig configures the index builder behavior
type BuilderConfig struct {
	// Number of worker goroutines for parallel processing
	Workers int

	// Maximum number of files to process in a single batch
	BatchSize int

	// Enable incremental indexing (only process changed files)
	Incremental bool

	// File patterns to include (glob patterns)
	IncludePatterns []string

	// File patterns to exclude (glob patterns)
	ExcludePatterns []string

	// Maximum file size to process (in bytes, 0 = no limit)
	MaxFileSize int64

	// Follow symbolic links
	FollowSymlinks bool

	// Enable progress reporting
	ReportProgress bool

	// Progress reporting interval
	ProgressInterval time.Duration

	// Enable detailed logging
	Verbose bool
}

// DefaultBuilderConfig returns sensible defaults for the builder
func DefaultBuilderConfig() BuilderConfig {
	return BuilderConfig{
		Workers:         4,
		BatchSize:       100,
		Incremental:     true,
		IncludePatterns: []string{"*.go", "*.py", "*.js", "*.ts", "*.java", "*.cpp", "*.c", "*.h"},
		ExcludePatterns: []string{"vendor/*", "node_modules/*", ".git/*", "*.test.go"},
		MaxFileSize:     10 << 20, // 10MB
		FollowSymlinks:  false,
		ReportProgress:  true,
		ProgressInterval: time.Second,
		Verbose:         false,
	}
}

// BuildProgress tracks the progress of index building
type BuildProgress struct {
	// Atomic counters for thread-safe access
	filesDiscovered int64
	filesProcessed  int64
	filesSkipped    int64
	filesErrored    int64
	symbolsIndexed  int64

	// Timing information
	startTime   time.Time
	updateTime  time.Time
	currentFile string
	mutex       sync.RWMutex

	// Error tracking
	errors []BuildError
}

// BuildError represents an error encountered during indexing
type BuildError struct {
	FilePath string    `json:"file_path"`
	Error    string    `json:"error"`
	Time     time.Time `json:"time"`
}

// BuildStats provides detailed statistics about the build process
type BuildStats struct {
	FilesDiscovered int64         `json:"files_discovered"`
	FilesProcessed  int64         `json:"files_processed"`
	FilesSkipped    int64         `json:"files_skipped"`
	FilesErrored    int64         `json:"files_errored"`
	SymbolsIndexed  int64         `json:"symbols_indexed"`
	Duration        time.Duration `json:"duration"`
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
	Errors          []BuildError  `json:"errors,omitempty"`
}

// SymbolParser defines the interface for parsing symbols from source files
type SymbolParser interface {
	// ParseFile extracts symbols from a source file
	ParseFile(ctx context.Context, filePath string) ([]SymbolInfo, error)

	// SupportedLanguages returns the languages this parser supports
	SupportedLanguages() []string

	// IsSupported checks if the parser supports the given file
	IsSupported(filePath string) bool
}

// NewBuilder creates a new index builder
func NewBuilder(store *Store, parser SymbolParser, config BuilderConfig) *Builder {
	return &Builder{
		store:    store,
		config:   config,
		parser:   parser,
		progress: &BuildProgress{},
	}
}

// BuildIndex builds or updates the index for the specified root directories
func (b *Builder) BuildIndex(ctx context.Context, roots ...string) (*BuildStats, error) {
	// Create cancellable context
	buildCtx, cancel := context.WithCancel(ctx)
	b.cancelFunc = cancel
	defer cancel()

	// Initialize progress tracking
	b.progress = &BuildProgress{
		startTime: time.Now(),
	}

	// Start progress reporting if enabled
	var progressDone chan struct{}
	if b.config.ReportProgress {
		progressDone = make(chan struct{})
		go b.reportProgress(buildCtx, progressDone)
	}

	// Discover files to process
	files, err := b.discoverFiles(buildCtx, roots...)
	if err != nil {
		return nil, fmt.Errorf("failed to discover files: %w", err)
	}

	atomic.StoreInt64(&b.progress.filesDiscovered, int64(len(files)))

	// Filter files for incremental indexing
	if b.config.Incremental {
		files, err = b.filterChangedFiles(buildCtx, files)
		if err != nil {
			return nil, fmt.Errorf("failed to filter changed files: %w", err)
		}
	}

	// Process files in parallel
	err = b.processFiles(buildCtx, files)

	// Stop progress reporting
	if progressDone != nil {
		close(progressDone)
	}

	// Collect final statistics
	stats := b.collectStats()

	if err != nil {
		return stats, fmt.Errorf("failed to process files: %w", err)
	}

	return stats, nil
}

// discoverFiles finds all files matching the include/exclude patterns
func (b *Builder) discoverFiles(ctx context.Context, roots ...string) ([]string, error) {
	var files []string
	var mutex sync.Mutex

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Check context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Skip directories
			if d.IsDir() {
				return nil
			}

			// Handle symbolic links
			if d.Type()&fs.ModeSymlink != 0 && !b.config.FollowSymlinks {
				return nil
			}

			// Check file size limit
			if b.config.MaxFileSize > 0 {
				info, err := d.Info()
				if err != nil {
					return nil // Skip files we can't stat
				}
				if info.Size() > b.config.MaxFileSize {
					return nil
				}
			}

			// Check include/exclude patterns
			if b.shouldProcessFile(path) {
				mutex.Lock()
				files = append(files, path)
				mutex.Unlock()
			}

			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return files, nil
}

// shouldProcessFile checks if a file matches include/exclude patterns
func (b *Builder) shouldProcessFile(path string) bool {
	// Check exclude patterns first
	for _, pattern := range b.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return false
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return false
		}
	}

	// Check include patterns
	if len(b.config.IncludePatterns) == 0 {
		return true // No include patterns means include all (not excluded)
	}

	for _, pattern := range b.config.IncludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}

	return false
}

// filterChangedFiles filters files for incremental indexing
func (b *Builder) filterChangedFiles(ctx context.Context, files []string) ([]string, error) {
	var changedFiles []string

	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Get file modification time
		stat, err := os.Stat(file)
		if err != nil {
			continue // Skip files we can't stat
		}

		// Check if file exists in index and compare modification times
		metadata, err := b.store.GetFileMetadata(ctx, file)
		if err != nil || metadata.ModTime.Before(stat.ModTime()) {
			changedFiles = append(changedFiles, file)
		} else {
			atomic.AddInt64(&b.progress.filesSkipped, 1)
		}
	}

	return changedFiles, nil
}

// processFiles processes files in parallel using worker goroutines
func (b *Builder) processFiles(ctx context.Context, files []string) error {
	// Create work channel
	workChan := make(chan string, b.config.BatchSize)

	// Create error group for coordinated cancellation
	g, gCtx := errgroup.WithContext(ctx)

	// Start workers
	for i := 0; i < b.config.Workers; i++ {
		g.Go(func() error {
			return b.worker(gCtx, workChan)
		})
	}

	// Send work to workers
	g.Go(func() error {
		defer close(workChan)

		for _, file := range files {
			select {
			case workChan <- file:
			case <-gCtx.Done():
				return gCtx.Err()
			}
		}
		return nil
	})

	return g.Wait()
}

// worker processes files from the work channel
func (b *Builder) worker(ctx context.Context, workChan <-chan string) error {
	for {
		select {
		case file, ok := <-workChan:
			if !ok {
				return nil // Channel closed
			}

			b.updateCurrentFile(file)
			if err := b.processFile(ctx, file); err != nil {
				b.recordError(file, err)
				atomic.AddInt64(&b.progress.filesErrored, 1)
			} else {
				atomic.AddInt64(&b.progress.filesProcessed, 1)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// processFile processes a single file and updates the index
func (b *Builder) processFile(ctx context.Context, filePath string) error {
	// Check if parser supports this file
	if !b.parser.IsSupported(filePath) {
		return nil // Not an error, just skip
	}

	// Get file info for metadata
	stat, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Calculate file hash
	hash, err := b.calculateFileHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Parse symbols from file
	symbols, err := b.parser.ParseFile(ctx, filePath)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Update file metadata
	language := b.detectLanguage(filePath)
	metadata := FileMetadata{
		Path:        filePath,
		Hash:        hash,
		Size:        stat.Size(),
		ModTime:     stat.ModTime(),
		Language:    language,
		SymbolCount: len(symbols),
		IndexedAt:   time.Now(),
	}

	if err := b.store.StoreFileMetadata(ctx, metadata); err != nil {
		return fmt.Errorf("failed to store file metadata: %w", err)
	}

	// Store symbols
	for _, symbol := range symbols {
		// Ensure symbol has required metadata
		symbol.FilePath = filePath
		symbol.LastUpdated = time.Now()

		if err := b.store.StoreSymbol(ctx, symbol); err != nil {
			return fmt.Errorf("failed to store symbol %s: %w", symbol.ID, err)
		}

		atomic.AddInt64(&b.progress.symbolsIndexed, 1)
	}

	return nil
}

// calculateFileHash calculates SHA256 hash of file contents
func (b *Builder) calculateFileHash(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:]), nil
}

// detectLanguage detects the programming language from file extension
func (b *Builder) detectLanguage(filePath string) string {
	ext := filepath.Ext(filePath)

	switch ext {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".js":
		return "JavaScript"
	case ".ts":
		return "TypeScript"
	case ".java":
		return "Java"
	case ".cpp", ".cc", ".cxx":
		return "C++"
	case ".c":
		return "C"
	case ".h", ".hpp":
		return "C/C++ Header"
	case ".rs":
		return "Rust"
	case ".rb":
		return "Ruby"
	case ".php":
		return "PHP"
	case ".cs":
		return "C#"
	default:
		return "Unknown"
	}
}

// updateCurrentFile updates the currently processing file (thread-safe)
func (b *Builder) updateCurrentFile(file string) {
	b.progress.mutex.Lock()
	defer b.progress.mutex.Unlock()

	b.progress.currentFile = file
	b.progress.updateTime = time.Now()
}

// recordError records a build error (thread-safe)
func (b *Builder) recordError(file string, err error) {
	b.progress.mutex.Lock()
	defer b.progress.mutex.Unlock()

	b.progress.errors = append(b.progress.errors, BuildError{
		FilePath: file,
		Error:    err.Error(),
		Time:     time.Now(),
	})
}

// reportProgress reports build progress at regular intervals
func (b *Builder) reportProgress(ctx context.Context, done <-chan struct{}) {
	ticker := time.NewTicker(b.config.ProgressInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.logProgress()
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// logProgress logs the current progress
func (b *Builder) logProgress() {
	if !b.config.Verbose {
		return
	}

	b.progress.mutex.RLock()
	currentFile := b.progress.currentFile
	b.progress.mutex.RUnlock()

	discovered := atomic.LoadInt64(&b.progress.filesDiscovered)
	processed := atomic.LoadInt64(&b.progress.filesProcessed)
	skipped := atomic.LoadInt64(&b.progress.filesSkipped)
	errored := atomic.LoadInt64(&b.progress.filesErrored)
	symbols := atomic.LoadInt64(&b.progress.symbolsIndexed)

	elapsed := time.Since(b.progress.startTime)
	remaining := discovered - processed - skipped - errored

	fmt.Printf("[INDEX] Progress: %d/%d files processed (%d skipped, %d errors), %d symbols indexed, %v elapsed, current: %s\n",
		processed, discovered, skipped, errored, symbols, elapsed.Round(time.Second), currentFile)

	if processed > 0 {
		avgTime := elapsed / time.Duration(processed)
		eta := time.Duration(remaining) * avgTime
		fmt.Printf("[INDEX] ETA: %v (avg: %v per file)\n", eta.Round(time.Second), avgTime.Round(time.Millisecond))
	}
}

// collectStats collects final build statistics
func (b *Builder) collectStats() *BuildStats {
	endTime := time.Now()

	b.progress.mutex.RLock()
	errors := make([]BuildError, len(b.progress.errors))
	copy(errors, b.progress.errors)
	b.progress.mutex.RUnlock()

	return &BuildStats{
		FilesDiscovered: atomic.LoadInt64(&b.progress.filesDiscovered),
		FilesProcessed:  atomic.LoadInt64(&b.progress.filesProcessed),
		FilesSkipped:    atomic.LoadInt64(&b.progress.filesSkipped),
		FilesErrored:    atomic.LoadInt64(&b.progress.filesErrored),
		SymbolsIndexed:  atomic.LoadInt64(&b.progress.symbolsIndexed),
		Duration:        endTime.Sub(b.progress.startTime),
		StartTime:       b.progress.startTime,
		EndTime:         endTime,
		Errors:          errors,
	}
}

// GetProgress returns current build progress (thread-safe)
func (b *Builder) GetProgress() BuildProgress {
	b.progress.mutex.RLock()
	defer b.progress.mutex.RUnlock()

	return BuildProgress{
		filesDiscovered: atomic.LoadInt64(&b.progress.filesDiscovered),
		filesProcessed:  atomic.LoadInt64(&b.progress.filesProcessed),
		filesSkipped:    atomic.LoadInt64(&b.progress.filesSkipped),
		filesErrored:    atomic.LoadInt64(&b.progress.filesErrored),
		symbolsIndexed:  atomic.LoadInt64(&b.progress.symbolsIndexed),
		startTime:       b.progress.startTime,
		updateTime:      b.progress.updateTime,
		currentFile:     b.progress.currentFile,
		errors:          append([]BuildError{}, b.progress.errors...),
	}
}

// Cancel cancels the ongoing build process
func (b *Builder) Cancel() {
	if b.cancelFunc != nil {
		b.cancelFunc()
	}
}

// IsRunning returns true if a build is currently in progress
func (b *Builder) IsRunning() bool {
	return b.cancelFunc != nil
}

// RebuildIndex completely rebuilds the index from scratch
func (b *Builder) RebuildIndex(ctx context.Context, roots ...string) (*BuildStats, error) {
	// Clear the existing index
	if err := b.clearIndex(ctx); err != nil {
		return nil, fmt.Errorf("failed to clear index: %w", err)
	}

	// Disable incremental indexing for rebuild
	originalIncremental := b.config.Incremental
	b.config.Incremental = false
	defer func() {
		b.config.Incremental = originalIncremental
	}()

	return b.BuildIndex(ctx, roots...)
}

// clearIndex removes all data from the index
func (b *Builder) clearIndex(ctx context.Context) error {
	// Get all files and remove them
	files, err := b.store.GetAllFiles(ctx)
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := b.store.DeleteFile(ctx, file.Path); err != nil {
			return fmt.Errorf("failed to delete file %s: %w", file.Path, err)
		}
	}

	return nil
}

// Config returns the builder configuration
func (b *Builder) Config() BuilderConfig {
	return b.config
}

// SetConfig updates the builder configuration
func (b *Builder) SetConfig(config BuilderConfig) {
	b.config = config
}