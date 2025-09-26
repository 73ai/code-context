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
	MaxWorkers int
	MaxDepth int
	FollowSymlinks bool
	HiddenFiles bool
	BufferSize int
	Filters *Filters
	IgnoreRules *IgnoreManager
	Context context.Context
}

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


func (w *Walker) Walk(root string) (<-chan Result, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	rootInfo, err := os.Lstat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to stat root path: %w", err)
	}

	if w.config.IgnoreRules == nil {
		ignoreManager, err := NewIgnoreManager()
		if err != nil {
			return nil, fmt.Errorf("failed to create ignore manager: %w", err)
		}
		w.config.IgnoreRules = ignoreManager
	}

	if err := w.config.IgnoreRules.LoadFromPath(absRoot); err != nil {
		return nil, fmt.Errorf("failed to load ignore rules: %w", err)
	}

	if w.config.Filters == nil {
		w.config.Filters = NewFilters()
	}

	w.config.Filters.SetAllowHidden(w.config.HiddenFiles)

	results := make(chan Result, w.config.BufferSize)

	start := time.Now()

	go func() {
		defer func() {
			w.stats.Duration = time.Since(start)
			close(results)
		}()

		if !rootInfo.IsDir() {
			if w.config.Filters.ShouldInclude(absRoot, rootInfo) {
				w.recordFile(rootInfo.Size())
				select {
				case results <- Result{Path: absRoot, RelPath: filepath.Base(absRoot), Info: rootInfo}:
				case <-w.config.Context.Done():
				}
			}
			return
		}

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

			relPath, _ := filepath.Rel(absRoot, path)
			if relPath == "" {
				relPath = "."
			}

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

			depth := strings.Count(relPath, string(filepath.Separator))
			if w.config.MaxDepth > 0 && depth > w.config.MaxDepth {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !w.config.HiddenFiles && isHidden(path) {
				w.recordFiltered()
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

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


func (w *Walker) Stats() Stats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return *w.stats
}

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

func isHidden(path string) bool {
	base := filepath.Base(path)
	if len(base) > 0 && base[0] == '.' {
		return true
	}

	dir := filepath.Dir(path)
	if dir == "." || dir == "/" {
		return false
	}

	return isHidden(dir)
}

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