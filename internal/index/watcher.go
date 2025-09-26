package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors file system changes and triggers incremental index updates
type Watcher struct {
	store       *Store
	builder     *Builder
	config      WatcherConfig
	fsWatcher   *fsnotify.Watcher
	eventChan   chan WatchEvent
	cancelFunc  context.CancelFunc
	runningMutex sync.RWMutex
	running     bool
}

// WatcherConfig configures the file system watcher behavior
type WatcherConfig struct {
	// Debounce duration to batch rapid file changes
	DebounceDuration time.Duration

	// Maximum number of events to batch together
	BatchSize int

	// Directories to watch (recursive)
	WatchDirs []string

	// File patterns to watch (same as builder patterns)
	IncludePatterns []string
	ExcludePatterns []string

	// Enable recursive directory watching
	Recursive bool

	// Enable detailed logging
	Verbose bool

	// Callback for watch events
	EventCallback func(WatchEvent)

	// Error callback
	ErrorCallback func(error)
}

// DefaultWatcherConfig returns sensible defaults for the watcher
func DefaultWatcherConfig() WatcherConfig {
	return WatcherConfig{
		DebounceDuration: 500 * time.Millisecond,
		BatchSize:        50,
		WatchDirs:        []string{},
		IncludePatterns:  []string{"*.go", "*.py", "*.js", "*.ts", "*.java", "*.cpp", "*.c", "*.h"},
		ExcludePatterns:  []string{"vendor/*", "node_modules/*", ".git/*", "*.test.go"},
		Recursive:        true,
		Verbose:          false,
	}
}

// WatchEvent represents a file system event
type WatchEvent struct {
	Path      string    `json:"path"`
	Operation string    `json:"operation"` // "create", "write", "remove", "rename"
	Time      time.Time `json:"time"`
	Size      int64     `json:"size,omitempty"`
}

// EventBatch represents a batch of events to process together
type EventBatch struct {
	Events    []WatchEvent `json:"events"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time"`
}

// NewWatcher creates a new file system watcher
func NewWatcher(store *Store, builder *Builder, config WatcherConfig) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	return &Watcher{
		store:     store,
		builder:   builder,
		config:    config,
		fsWatcher: fsWatcher,
		eventChan: make(chan WatchEvent, config.BatchSize*2),
	}, nil
}

// Start begins watching the configured directories
func (w *Watcher) Start(ctx context.Context) error {
	w.runningMutex.Lock()
	defer w.runningMutex.Unlock()

	if w.running {
		return fmt.Errorf("watcher is already running")
	}

	// Create cancellable context
	watchCtx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel
	w.running = true

	// Add directories to watch
	for _, dir := range w.config.WatchDirs {
		if err := w.addDirectory(dir); err != nil {
			cancel()
			w.running = false
			return fmt.Errorf("failed to add watch directory %s: %w", dir, err)
		}
	}

	// Start event processing goroutines
	go w.watchFileSystem(watchCtx)
	go w.processEvents(watchCtx)

	if w.config.Verbose {
		fmt.Printf("[WATCHER] Started watching %d directories\n", len(w.config.WatchDirs))
	}

	return nil
}

// Stop stops the file system watcher
func (w *Watcher) Stop() error {
	w.runningMutex.Lock()
	defer w.runningMutex.Unlock()

	if !w.running {
		return nil
	}

	if w.cancelFunc != nil {
		w.cancelFunc()
	}

	err := w.fsWatcher.Close()
	w.running = false

	if w.config.Verbose {
		fmt.Println("[WATCHER] Stopped watching file system")
	}

	return err
}

// IsRunning returns true if the watcher is currently active
func (w *Watcher) IsRunning() bool {
	w.runningMutex.RLock()
	defer w.runningMutex.RUnlock()
	return w.running
}

// AddDirectory adds a directory to watch
func (w *Watcher) AddDirectory(dir string) error {
	w.runningMutex.RLock()
	running := w.running
	w.runningMutex.RUnlock()

	if !running {
		// Just add to config if not running
		w.config.WatchDirs = append(w.config.WatchDirs, dir)
		return nil
	}

	return w.addDirectory(dir)
}

// RemoveDirectory removes a directory from watching
func (w *Watcher) RemoveDirectory(dir string) error {
	err := w.fsWatcher.Remove(dir)
	if err != nil {
		return err
	}

	// Remove from config
	newDirs := make([]string, 0, len(w.config.WatchDirs))
	for _, d := range w.config.WatchDirs {
		if d != dir {
			newDirs = append(newDirs, d)
		}
	}
	w.config.WatchDirs = newDirs

	// Remove subdirectories if recursive
	if w.config.Recursive {
		// Remove all watched directories that are subdirectories of dir
		for _, watchedDir := range w.fsWatcher.WatchList() {
			if isSubdirectory(watchedDir, dir) {
				w.fsWatcher.Remove(watchedDir)
			}
		}
	}

	return nil
}

// addDirectory adds a directory (and subdirectories if recursive) to the watcher
func (w *Watcher) addDirectory(dir string) error {
	if err := w.fsWatcher.Add(dir); err != nil {
		return err
	}

	if w.config.Recursive {
		return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() && path != dir {
				// Skip excluded directories
				if w.shouldExcludeDirectory(path) {
					return filepath.SkipDir
				}

				if err := w.fsWatcher.Add(path); err != nil {
					// Log error but continue walking
					if w.config.ErrorCallback != nil {
						w.config.ErrorCallback(fmt.Errorf("failed to watch directory %s: %w", path, err))
					}
				}
			}

			return nil
		})
	}

	return nil
}

// watchFileSystem watches for file system events and converts them to WatchEvents
func (w *Watcher) watchFileSystem(ctx context.Context) {
	defer close(w.eventChan)

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return // Watcher was closed
			}

			watchEvent := w.convertEvent(event)
			if watchEvent != nil && w.shouldProcessFile(watchEvent.Path) {
				select {
				case w.eventChan <- *watchEvent:
				case <-ctx.Done():
					return
				}

				// Call event callback if configured
				if w.config.EventCallback != nil {
					w.config.EventCallback(*watchEvent)
				}
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return // Watcher was closed
			}

			if w.config.ErrorCallback != nil {
				w.config.ErrorCallback(fmt.Errorf("watcher error: %w", err))
			}

		case <-ctx.Done():
			return
		}
	}
}

// convertEvent converts fsnotify.Event to WatchEvent
func (w *Watcher) convertEvent(event fsnotify.Event) *WatchEvent {
	var operation string

	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		operation = "create"
	case event.Op&fsnotify.Write == fsnotify.Write:
		operation = "write"
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		operation = "remove"
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		operation = "rename"
	default:
		return nil // Ignore other operations
	}

	// Get file size for create/write operations
	var size int64
	if operation == "create" || operation == "write" {
		if stat, err := os.Stat(event.Name); err == nil {
			size = stat.Size()
		}
	}

	return &WatchEvent{
		Path:      event.Name,
		Operation: operation,
		Time:      time.Now(),
		Size:      size,
	}
}

// processEvents processes batched events and triggers index updates
func (w *Watcher) processEvents(ctx context.Context) {
	var events []WatchEvent
	var timer *time.Timer
	var timerChan <-chan time.Time

	for {
		select {
		case event, ok := <-w.eventChan:
			if !ok {
				// Process remaining events before exiting
				if len(events) > 0 {
					w.processBatch(ctx, events)
				}
				return
			}

			// Add event to batch
			events = append(events, event)

			// Start or reset debounce timer
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(w.config.DebounceDuration)
			timerChan = timer.C

			// Process immediately if batch is full
			if len(events) >= w.config.BatchSize {
				if timer != nil {
					timer.Stop()
				}
				w.processBatch(ctx, events)
				events = nil
				timerChan = nil
			}

		case <-timerChan:
			// Debounce timer expired, process batch
			if len(events) > 0 {
				w.processBatch(ctx, events)
				events = nil
			}
			timerChan = nil

		case <-ctx.Done():
			// Process remaining events before exiting
			if len(events) > 0 {
				w.processBatch(ctx, events)
			}
			return
		}
	}
}

// processBatch processes a batch of file system events
func (w *Watcher) processBatch(ctx context.Context, events []WatchEvent) {
	if len(events) == 0 {
		return
	}

	batch := EventBatch{
		Events:    events,
		StartTime: events[0].Time,
		EndTime:   events[len(events)-1].Time,
	}

	if w.config.Verbose {
		fmt.Printf("[WATCHER] Processing batch of %d events\n", len(events))
	}

	// Group events by operation type
	writes := make([]string, 0)
	removes := make([]string, 0)

	for _, event := range events {
		switch event.Operation {
		case "create", "write":
			// Treat creates and writes the same - they both need indexing
			if !contains(writes, event.Path) {
				writes = append(writes, event.Path)
			}
		case "remove", "rename":
			// Treat removes and renames as removes
			if !contains(removes, event.Path) {
				removes = append(removes, event.Path)
			}
		}
	}

	// Process removes first
	for _, path := range removes {
		if err := w.store.DeleteFile(ctx, path); err != nil {
			if w.config.ErrorCallback != nil {
				w.config.ErrorCallback(fmt.Errorf("failed to remove file %s from index: %w", path, err))
			}
		}
	}

	// Process creates/writes
	if len(writes) > 0 {
		// Create a temporary builder config for incremental updates
		builderConfig := w.builder.Config()
		builderConfig.Incremental = false // Force reprocessing of changed files

		tempBuilder := NewBuilder(w.store, w.builder.parser, builderConfig)

		// Process only the changed files
		stats, err := tempBuilder.BuildIndex(ctx, writes...)
		if err != nil {
			if w.config.ErrorCallback != nil {
				w.config.ErrorCallback(fmt.Errorf("failed to index changed files: %w", err))
			}
		} else if w.config.Verbose {
			fmt.Printf("[WATCHER] Indexed %d files, %d symbols\n", stats.FilesProcessed, stats.SymbolsIndexed)
		}
	}

	if w.config.Verbose {
		duration := batch.EndTime.Sub(batch.StartTime)
		fmt.Printf("[WATCHER] Batch processed in %v (events from %v to %v)\n",
			duration, batch.StartTime.Format("15:04:05"), batch.EndTime.Format("15:04:05"))
	}
}

// shouldProcessFile checks if a file should be processed based on include/exclude patterns
func (w *Watcher) shouldProcessFile(path string) bool {
	// Check exclude patterns first
	for _, pattern := range w.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return false
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return false
		}
	}

	// Check include patterns
	if len(w.config.IncludePatterns) == 0 {
		return true // No include patterns means include all (not excluded)
	}

	for _, pattern := range w.config.IncludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}

	return false
}

// shouldExcludeDirectory checks if a directory should be excluded from watching
func (w *Watcher) shouldExcludeDirectory(path string) bool {
	dirName := filepath.Base(path)

	// Common directories to exclude
	excludedDirs := []string{
		".git", ".svn", ".hg", ".bzr",
		"node_modules", "vendor", "target",
		".vscode", ".idea", "__pycache__",
	}

	for _, excluded := range excludedDirs {
		if dirName == excluded {
			return true
		}
	}

	// Check custom exclude patterns
	for _, pattern := range w.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, dirName); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}

	return false
}

// GetWatchedDirectories returns the list of currently watched directories
func (w *Watcher) GetWatchedDirectories() []string {
	if w.fsWatcher == nil {
		return w.config.WatchDirs
	}
	return w.fsWatcher.WatchList()
}

// GetConfig returns the current watcher configuration
func (w *Watcher) GetConfig() WatcherConfig {
	return w.config
}

// SetConfig updates the watcher configuration
func (w *Watcher) SetConfig(config WatcherConfig) error {
	w.config = config

	// If running, restart with new configuration
	if w.IsRunning() {
		// Store current context (this is a simplified approach)
		if err := w.Stop(); err != nil {
			return err
		}

		// Note: In a real implementation, you'd want to preserve the context
		// This is a simplified version for demonstration
		return w.Start(context.Background())
	}

	return nil
}

// Utility functions

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// isSubdirectory checks if child is a subdirectory of parent
func isSubdirectory(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// WatchStats provides statistics about watcher activity
type WatchStats struct {
	WatchedDirs   int       `json:"watched_dirs"`
	EventsTotal   int64     `json:"events_total"`
	EventsCreated int64     `json:"events_created"`
	EventsWritten int64     `json:"events_written"`
	EventsRemoved int64     `json:"events_removed"`
	BatchesTotal  int64     `json:"batches_total"`
	LastEvent     time.Time `json:"last_event"`
	Uptime        time.Duration `json:"uptime"`
}

// GetStats returns watcher statistics (this would need to be implemented with actual counters)
func (w *Watcher) GetStats() WatchStats {
	var watchedDirs int
	if w.fsWatcher != nil {
		watchedDirs = len(w.fsWatcher.WatchList())
	}

	return WatchStats{
		WatchedDirs: watchedDirs,
		// Other stats would need to be tracked during operation
	}
}