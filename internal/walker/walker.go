// Package walker provides fast, concurrent file system traversal with gitignore support
package walker

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Result represents a file discovered during traversal
type Result struct {
	Path      string      // Absolute path to the file
	RelPath   string      // Path relative to the walk root
	Info      fs.FileInfo // File information
	IsSymlink bool        // Whether this is a symbolic link
	Error     error       // Any error encountered processing this file
}

// Stats contains traversal statistics
type Stats struct {
	FilesFound     int64         // Total files found
	FilesFiltered  int64         // Files filtered out
	DirsTraversed  int64         // Directories traversed
	DirsIgnored    int64         // Directories ignored
	SymlinksFound  int64         // Symbolic links found
	Errors         int64         // Errors encountered
	Duration       time.Duration // Total traversal time
	BytesTraversed int64         // Total bytes of files traversed
}

// Config holds configuration for the walker
type Config struct {
	// MaxWorkers controls the number of concurrent workers
	// If 0, uses runtime.GOMAXPROCS(0)
	MaxWorkers int

	// MaxDepth limits directory traversal depth (0 = unlimited)
	MaxDepth int

	// FollowSymlinks determines if symbolic links should be followed
	FollowSymlinks bool

	// HiddenFiles controls whether hidden files/directories are included
	HiddenFiles bool

	// BufferSize is the size of the result channel buffer
	BufferSize int

	// Filters contains file filtering rules
	Filters *Filters

	// IgnoreRules contains gitignore-style ignore rules
	IgnoreRules *IgnoreManager

	// Context for cancellation
	Context context.Context
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		MaxWorkers:     runtime.GOMAXPROCS(0),
		MaxDepth:       0,
		FollowSymlinks: false,
		HiddenFiles:    false,
		BufferSize:     1000,
		Context:        context.Background(),
	}
}

// Walker provides concurrent file system traversal
type Walker struct {
	config *Config
	stats  *Stats
	mu     sync.RWMutex
}

// New creates a new Walker with the given configuration
func New(config *Config) (*Walker, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if config.MaxWorkers <= 0 {
		config.MaxWorkers = runtime.GOMAXPROCS(0)
	}

	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}

	if config.Context == nil {
		config.Context = context.Background()
	}

	return &Walker{
		config: config,
		stats:  &Stats{},
	}, nil
}


// Walk traverses the file system starting from the given root path
func (w *Walker) Walk(root string) (<-chan Result, error) {
	// Resolve absolute path
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if root exists
	rootInfo, err := os.Lstat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to stat root path: %w", err)
	}

	// Initialize ignore rules if needed
	if w.config.IgnoreRules == nil {
		ignoreManager, err := NewIgnoreManager()
		if err != nil {
			return nil, fmt.Errorf("failed to create ignore manager: %w", err)
		}
		w.config.IgnoreRules = ignoreManager
	}

	// Load gitignore files starting from root
	if err := w.config.IgnoreRules.LoadFromPath(absRoot); err != nil {
		return nil, fmt.Errorf("failed to load ignore rules: %w", err)
	}

	// Initialize filters if needed
	if w.config.Filters == nil {
		w.config.Filters = NewFilters()
	}

	// Configure filters based on walker settings
	w.config.Filters.SetAllowHidden(w.config.HiddenFiles)

	results := make(chan Result, w.config.BufferSize)

	// Start timing
	start := time.Now()

	// Use a simpler approach with filepath.WalkDir for now
	go func() {
		defer func() {
			w.stats.Duration = time.Since(start)
			close(results)
		}()

		if !rootInfo.IsDir() {
			// Handle single file
			if w.config.Filters.ShouldInclude(absRoot, rootInfo) {
				w.recordFile(rootInfo.Size())
				select {
				case results <- Result{Path: absRoot, RelPath: filepath.Base(absRoot), Info: rootInfo}:
				case <-w.config.Context.Done():
				}
			}
			return
		}

		// Walk directory tree
		filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				w.recordError()
				select {
				case results <- Result{Path: path, Error: err}:
				case <-w.config.Context.Done():
					return fmt.Errorf("context canceled")
				}
				return nil // Continue walking
			}

			// Get relative path
			relPath, _ := filepath.Rel(absRoot, path)
			if relPath == "" {
				relPath = "."
			}

			// Check context cancellation
			select {
			case <-w.config.Context.Done():
				return fmt.Errorf("context canceled")
			default:
			}

			info, err := d.Info()
			if err != nil {
				w.recordError()
				select {
				case results <- Result{Path: path, RelPath: relPath, Error: err}:
				case <-w.config.Context.Done():
					return fmt.Errorf("context canceled")
				}
				return nil
			}

			// Check depth limit
			depth := strings.Count(relPath, string(filepath.Separator))
			if w.config.MaxDepth > 0 && depth > w.config.MaxDepth {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Check hidden files
			if !w.config.HiddenFiles && isHidden(path) {
				w.recordFiltered()
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Check ignore rules
			if w.config.IgnoreRules.ShouldIgnore(relPath, info.IsDir()) {
				if info.IsDir() {
					w.recordDirIgnored()
					return filepath.SkipDir
				} else {
					w.recordFiltered()
				}
				return nil
			}

			if info.IsDir() {
				w.recordDirTraversed()
			} else {
				// Regular file
				if w.config.Filters.ShouldInclude(path, info) {
					w.recordFile(info.Size())
					select {
					case results <- Result{Path: path, RelPath: relPath, Info: info}:
					case <-w.config.Context.Done():
						return fmt.Errorf("context canceled")
					}
				} else {
					w.recordFiltered()
				}
			}

			return nil
		})
	}()

	return results, nil
}


// Stats returns current traversal statistics
func (w *Walker) Stats() Stats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return *w.stats
}

// Thread-safe stat recording methods
func (w *Walker) recordFile(size int64) {
	w.mu.Lock()
	w.stats.FilesFound++
	w.stats.BytesTraversed += size
	w.mu.Unlock()
}

func (w *Walker) recordFiltered() {
	w.mu.Lock()
	w.stats.FilesFiltered++
	w.mu.Unlock()
}

func (w *Walker) recordDirTraversed() {
	w.mu.Lock()
	w.stats.DirsTraversed++
	w.mu.Unlock()
}

func (w *Walker) recordDirIgnored() {
	w.mu.Lock()
	w.stats.DirsIgnored++
	w.mu.Unlock()
}

func (w *Walker) recordSymlink() {
	w.mu.Lock()
	w.stats.SymlinksFound++
	w.mu.Unlock()
}

func (w *Walker) recordError() {
	w.mu.Lock()
	w.stats.Errors++
	w.mu.Unlock()
}

// isHidden checks if a file or directory is hidden
func isHidden(path string) bool {
	// Check if the file itself is hidden
	base := filepath.Base(path)
	if len(base) > 0 && base[0] == '.' {
		return true
	}

	// Check if any directory in the path is hidden
	dir := filepath.Dir(path)
	if dir == "." || dir == "/" {
		return false
	}

	// Recursively check parent directories
	return isHidden(dir)
}

// WalkSimple provides a simple interface for file system traversal
func WalkSimple(root string) ([]Result, error) {
	walker, err := New(DefaultConfig())
	if err != nil {
		return nil, err
	}

	results, err := walker.Walk(root)
	if err != nil {
		return nil, err
	}

	var files []Result
	for result := range results {
		if result.Error != nil {
			return nil, result.Error
		}
		if result.Info != nil && !result.Info.IsDir() {
			files = append(files, result)
		}
	}

	return files, nil
}