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
	filesDiscovered    int64
	filesProcessed     int64
	filesSkipped       int64
	filesErrored       int64
	symbolsIndexed     int64
	referencesIndexed  int64

	// Phase tracking for two-phase indexing
	currentPhase string // "symbols", "references", or "complete"
	phaseStartTime time.Time

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
	FilesDiscovered    int64         `json:"files_discovered"`
	FilesProcessed     int64         `json:"files_processed"`
	FilesSkipped       int64         `json:"files_skipped"`
	FilesErrored       int64         `json:"files_errored"`
	SymbolsIndexed     int64         `json:"symbols_indexed"`
	ReferencesIndexed  int64         `json:"references_indexed"`
	Duration           time.Duration `json:"duration"`
	StartTime          time.Time     `json:"start_time"`
	EndTime            time.Time     `json:"end_time"`
	Errors             []BuildError  `json:"errors,omitempty"`
}

// SymbolParser defines the interface for parsing symbols from source files
type SymbolParser interface {
	// ParseFile extracts symbols from a source file
	ParseFile(ctx context.Context, filePath string) ([]SymbolInfo, error)

	// ParseReferences extracts references from a source file
	// This requires that symbols have been previously parsed and stored
	ParseReferences(ctx context.Context, filePath string, symbolIndex SymbolIndex) ([]Reference, error)

	// SupportedLanguages returns the languages this parser supports
	SupportedLanguages() []string

	// IsSupported checks if the parser supports the given file
	IsSupported(filePath string) bool

	// SupportsReferences indicates if this parser can extract references
	SupportsReferences() bool
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

	// Phase 1: Process symbols
	b.setPhase("symbols")
	err = b.processFilesSymbols(buildCtx, files)
	if err != nil {
		// Stop progress reporting before returning
		if progressDone != nil {
			close(progressDone)
		}
		stats := b.collectStats()
		return stats, fmt.Errorf("failed to process symbols: %w", err)
	}

	// Phase 2: Process references (if parser supports them)
	if b.parser.SupportsReferences() {
		b.setPhase("references")
		err = b.processFilesReferences(buildCtx, files)
		if err != nil {
			// Stop progress reporting before returning
			if progressDone != nil {
				close(progressDone)
			}
			stats := b.collectStats()
			return stats, fmt.Errorf("failed to process references: %w", err)
		}
	}

	b.setPhase("complete")

	// Stop progress reporting
	if progressDone != nil {
		close(progressDone)
	}

	// Collect final statistics
	stats := b.collectStats()

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

// processFilesSymbols processes files to extract symbols in parallel
func (b *Builder) processFilesSymbols(ctx context.Context, files []string) error {
	// Create work channel
	workChan := make(chan string, b.config.BatchSize)

	// Create error group for coordinated cancellation
	g, gCtx := errgroup.WithContext(ctx)

	// Start symbol workers
	for i := 0; i < b.config.Workers; i++ {
		g.Go(func() error {
			return b.symbolWorker(gCtx, workChan)
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

// processFilesReferences processes files to extract references in parallel
func (b *Builder) processFilesReferences(ctx context.Context, files []string) error {
	// Create work channel
	workChan := make(chan string, b.config.BatchSize)

	// Create error group for coordinated cancellation
	g, gCtx := errgroup.WithContext(ctx)

	// Build symbol index for reference resolution
	symbolIndex, err := b.buildSymbolIndex(gCtx)
	if err != nil {
		return fmt.Errorf("failed to build symbol index for references: %w", err)
	}

	if b.config.Verbose {
		fmt.Printf("[INDEX] Built symbol index with %d symbols for reference resolution\n", len(symbolIndex))
	}

	// Start reference workers
	for i := 0; i < b.config.Workers; i++ {
		g.Go(func() error {
			return b.referenceWorker(gCtx, workChan, symbolIndex)
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

// symbolWorker processes files to extract symbols from the work channel
func (b *Builder) symbolWorker(ctx context.Context, workChan <-chan string) error {
	for {
		select {
		case file, ok := <-workChan:
			if !ok {
				return nil // Channel closed
			}

			b.updateCurrentFile(file)
			if err := b.processFileSymbols(ctx, file); err != nil {
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

// referenceWorker processes files to extract references from the work channel
func (b *Builder) referenceWorker(ctx context.Context, workChan <-chan string, symbolIndex SymbolIndex) error {
	for {
		select {
		case file, ok := <-workChan:
			if !ok {
				return nil // Channel closed
			}

			b.updateCurrentFile(file)
			if err := b.processFileReferences(ctx, file, symbolIndex); err != nil {
				b.recordError(file, err)
				atomic.AddInt64(&b.progress.filesErrored, 1)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// processFileSymbols processes a single file to extract symbols and updates the index
func (b *Builder) processFileSymbols(ctx context.Context, filePath string) error {
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

// processFileReferences processes a single file to extract references
func (b *Builder) processFileReferences(ctx context.Context, filePath string, symbolIndex SymbolIndex) error {
	// Check if parser supports this file and references
	if !b.parser.IsSupported(filePath) || !b.parser.SupportsReferences() {
		return nil // Not an error, just skip
	}

	// Parse references from file with retry logic for robustness
	var references []Reference
	var err error
	maxRetries := 2

	for attempt := 0; attempt <= maxRetries; attempt++ {
		references, err = b.parser.ParseReferences(ctx, filePath, symbolIndex)
		if err == nil {
			break // Success
		}

		if attempt < maxRetries {
			if b.config.Verbose {
				fmt.Printf("[INDEX] Reference parsing attempt %d failed for %s: %v, retrying...\n", attempt+1, filePath, err)
			}
			// Add a small delay before retry
			time.Sleep(100 * time.Millisecond)
		} else {
			// Log the final error but don't fail the entire process
			b.recordError(filePath, fmt.Errorf("failed to parse references after %d attempts: %w", maxRetries+1, err))
			return nil // Continue with other files
		}
	}

	// Validate references before storing
	validReferences := b.validateReferences(references, filePath)
	if len(validReferences) < len(references) && b.config.Verbose {
		fmt.Printf("[INDEX] Filtered %d invalid references from %s\n", len(references)-len(validReferences), filePath)
	}

	// Store references in batches for better performance
	if len(validReferences) > 0 {
		// Split into smaller batches to avoid memory pressure
		batchSize := b.config.BatchSize
		if batchSize == 0 {
			batchSize = 100 // Default batch size
		}

		for i := 0; i < len(validReferences); i += batchSize {
			end := i + batchSize
			if end > len(validReferences) {
				end = len(validReferences)
			}

			batch := validReferences[i:end]
			if err := b.store.StoreReferenceBatch(ctx, batch); err != nil {
				// Log error but continue with remaining batches
				b.recordError(filePath, fmt.Errorf("failed to store reference batch %d-%d: %w", i, end-1, err))
				continue
			}

			atomic.AddInt64(&b.progress.referencesIndexed, int64(len(batch)))
		}
	}

	return nil
}

// validateReferences filters out invalid or malformed references
func (b *Builder) validateReferences(references []Reference, filePath string) []Reference {
	var valid []Reference

	for _, ref := range references {
		// Basic validation checks
		if ref.SymbolID == "" {
			if b.config.Verbose {
				fmt.Printf("[INDEX] Skipping reference with empty SymbolID in %s at line %d\n", filePath, ref.Line)
			}
			continue
		}

		if ref.Line < 1 {
			if b.config.Verbose {
				fmt.Printf("[INDEX] Skipping reference with invalid line number %d in %s\n", ref.Line, filePath)
			}
			continue
		}

		if ref.Column < 0 {
			if b.config.Verbose {
				fmt.Printf("[INDEX] Skipping reference with invalid column number %d in %s\n", ref.Column, filePath)
			}
			continue
		}

		// Ensure file path is set correctly
		if ref.FilePath == "" {
			ref.FilePath = filePath
		}

		// Set default kind if not specified
		if ref.Kind == "" {
			ref.Kind = "reference"
		}

		valid = append(valid, ref)
	}

	return valid
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

// setPhase updates the current indexing phase (thread-safe)
func (b *Builder) setPhase(phase string) {
	b.progress.mutex.Lock()
	defer b.progress.mutex.Unlock()

	b.progress.currentPhase = phase
	b.progress.phaseStartTime = time.Now()
}

// updateCurrentFile updates the currently processing file (thread-safe)
func (b *Builder) updateCurrentFile(file string) {
	b.progress.mutex.Lock()
	defer b.progress.mutex.Unlock()

	b.progress.currentFile = file
	b.progress.updateTime = time.Now()
}

// buildSymbolIndex creates an in-memory symbol index from stored symbols
func (b *Builder) buildSymbolIndex(ctx context.Context) (SymbolIndex, error) {
	// Get all indexed files
	files, err := b.store.GetAllFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get file list for symbol index: %w", err)
	}

	if len(files) == 0 {
		if b.config.Verbose {
			fmt.Printf("[INDEX] Warning: No files found for symbol index building\n")
		}
		return SymbolIndex{}, nil
	}

	var allSymbols []SymbolInfo
	errorCount := 0
	maxErrors := len(files) / 2 // Allow up to 50% of files to fail

	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		symbols, err := b.store.GetSymbolsInFile(ctx, file.Path)
		if err != nil {
			errorCount++
			// Log the error but continue with other files
			b.recordError(file.Path, fmt.Errorf("failed to get symbols for index: %w", err))

			// If too many files are failing, something is seriously wrong
			if errorCount > maxErrors {
				return nil, fmt.Errorf("too many files (%d/%d) failed during symbol index building, last error: %w",
					errorCount, len(files), err)
			}
			continue
		}
		allSymbols = append(allSymbols, symbols...)
	}

	if errorCount > 0 && b.config.Verbose {
		fmt.Printf("[INDEX] Symbol index built with %d symbols from %d files (%d files had errors)\n",
			len(allSymbols), len(files)-errorCount, errorCount)
	}

	return SymbolIndex(allSymbols), nil
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
	references := atomic.LoadInt64(&b.progress.referencesIndexed)

	b.progress.mutex.RLock()
	phase := b.progress.currentPhase
	b.progress.mutex.RUnlock()

	elapsed := time.Since(b.progress.startTime)
	remaining := discovered - processed - skipped - errored

	fmt.Printf("[INDEX] Phase: %s, Progress: %d/%d files processed (%d skipped, %d errors), %d symbols, %d references indexed, %v elapsed, current: %s\n",
		phase, processed, discovered, skipped, errored, symbols, references, elapsed.Round(time.Second), currentFile)

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
		FilesDiscovered:    atomic.LoadInt64(&b.progress.filesDiscovered),
		FilesProcessed:     atomic.LoadInt64(&b.progress.filesProcessed),
		FilesSkipped:       atomic.LoadInt64(&b.progress.filesSkipped),
		FilesErrored:       atomic.LoadInt64(&b.progress.filesErrored),
		SymbolsIndexed:     atomic.LoadInt64(&b.progress.symbolsIndexed),
		ReferencesIndexed:  atomic.LoadInt64(&b.progress.referencesIndexed),
		Duration:           endTime.Sub(b.progress.startTime),
		StartTime:          b.progress.startTime,
		EndTime:            endTime,
		Errors:             errors,
	}
}

// GetProgress returns current build progress (thread-safe)
func (b *Builder) GetProgress() BuildProgress {
	b.progress.mutex.RLock()
	defer b.progress.mutex.RUnlock()

	return BuildProgress{
		filesDiscovered:   atomic.LoadInt64(&b.progress.filesDiscovered),
		filesProcessed:    atomic.LoadInt64(&b.progress.filesProcessed),
		filesSkipped:      atomic.LoadInt64(&b.progress.filesSkipped),
		filesErrored:      atomic.LoadInt64(&b.progress.filesErrored),
		symbolsIndexed:    atomic.LoadInt64(&b.progress.symbolsIndexed),
		referencesIndexed: atomic.LoadInt64(&b.progress.referencesIndexed),
		currentPhase:      b.progress.currentPhase,
		phaseStartTime:    b.progress.phaseStartTime,
		startTime:         b.progress.startTime,
		updateTime:        b.progress.updateTime,
		currentFile:       b.progress.currentFile,
		errors:            append([]BuildError{}, b.progress.errors...),
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