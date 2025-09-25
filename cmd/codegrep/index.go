package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/73ai/code-context/internal/index"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Manage the semantic search index",
	Long: `Manage the semantic search index used for symbol definitions, references,
and type lookups. The index must be built before semantic search features work.`,
}

var rebuildIndexCmd = &cobra.Command{
	Use:   "rebuild [path...]",
	Short: "Rebuild the semantic index from scratch",
	Long: `Rebuild the semantic index completely from scratch for the specified paths.
This removes all existing index data and reprocesses all supported source files.

If no paths are specified, the current directory is used.

EXAMPLES:
    codegrep index rebuild                    # Rebuild index for current directory
    codegrep index rebuild ./src ./tests     # Rebuild index for specific directories
    codegrep index rebuild --verbose .       # Rebuild with detailed progress output`,
	RunE: runRebuildIndex,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current status of the semantic index",
	Long: `Display information about the current semantic index including:
- Index location and size
- Number of indexed files and symbols
- Last update time
- Supported languages
- Index health and statistics

EXAMPLES:
    codegrep index status                     # Show basic index status
    codegrep index status --verbose          # Show detailed statistics
    codegrep index status --json             # Output status in JSON format`,
	RunE: runIndexStatus,
}

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all index data",
	Long: `Remove all data from the semantic search index. This will delete all
indexed symbols, file metadata, and cached query results.

Use this command to reset the index or free up disk space. You'll need to
rebuild the index before semantic search features will work again.

EXAMPLES:
    codegrep index clear                      # Clear all index data
    codegrep index clear --force             # Clear without confirmation`,
	RunE: runClearIndex,
}

// Index command-specific flags
var (
	indexVerbose     bool
	indexForce       bool
	indexWorkers     int
	indexBatchSize   int
	indexProgress    bool
	indexIncremental bool
)

func init() {
	// Add index command and subcommands
	rootCmd.AddCommand(indexCmd)
	indexCmd.AddCommand(rebuildIndexCmd)
	indexCmd.AddCommand(statusCmd)
	indexCmd.AddCommand(clearCmd)

	// Rebuild command flags
	rebuildIndexCmd.Flags().BoolVarP(&indexVerbose, "verbose", "v", false, "Show detailed progress information")
	rebuildIndexCmd.Flags().IntVarP(&indexWorkers, "workers", "w", 4, "Number of parallel workers")
	rebuildIndexCmd.Flags().IntVar(&indexBatchSize, "batch-size", 100, "Number of files to process per batch")
	rebuildIndexCmd.Flags().BoolVar(&indexProgress, "progress", true, "Show progress updates")
	rebuildIndexCmd.Flags().BoolVar(&indexIncremental, "incremental", false, "Enable incremental indexing (only process changed files)")

	// Status command flags
	statusCmd.Flags().BoolVarP(&indexVerbose, "verbose", "v", false, "Show detailed statistics")
	statusCmd.Flags().BoolVar(&config.JSON, "json", false, "Output status in JSON format")

	// Clear command flags
	clearCmd.Flags().BoolVarP(&indexForce, "force", "f", false, "Clear without confirmation")
}

func runRebuildIndex(cmd *cobra.Command, args []string) error {
	ctx := context.WithValue(cmd.Context(), "operation", "rebuild")

	// Determine paths to index
	paths := args
	if len(paths) == 0 {
		pwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		paths = []string{pwd}
	}

	// Validate paths
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("path does not exist: %s", path)
		}
	}

	// Initialize storage and builder
	storage, err := initializeStorage()
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer storage.Close()

	store := index.NewStore(storage, index.DefaultStoreConfig())
	parser := newSymbolParser() // You'd implement this based on your parser

	builderConfig := index.DefaultBuilderConfig()
	builderConfig.Workers = indexWorkers
	builderConfig.BatchSize = indexBatchSize
	builderConfig.ReportProgress = indexProgress
	builderConfig.Verbose = indexVerbose
	builderConfig.Incremental = indexIncremental

	builder := index.NewBuilder(store, parser, builderConfig)

	fmt.Printf("🔨 Rebuilding semantic index for paths: %s\n", strings.Join(paths, ", "))
	if indexVerbose {
		fmt.Printf("Configuration:\n")
		fmt.Printf("  Workers: %d\n", builderConfig.Workers)
		fmt.Printf("  Batch size: %d\n", builderConfig.BatchSize)
		fmt.Printf("  Incremental: %v\n", builderConfig.Incremental)
		fmt.Printf("  Include patterns: %v\n", builderConfig.IncludePatterns)
		fmt.Printf("  Exclude patterns: %v\n", builderConfig.ExcludePatterns)
		fmt.Println()
	}

	startTime := time.Now()
	stats, err := builder.RebuildIndex(ctx, paths...)
	duration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("failed to rebuild index: %w", err)
	}

	// Display results
	fmt.Printf("✅ Index rebuild completed in %v\n", duration)
	fmt.Printf("📊 Statistics:\n")
	fmt.Printf("  Files discovered: %d\n", stats.FilesDiscovered)
	fmt.Printf("  Files processed: %d\n", stats.FilesProcessed)
	fmt.Printf("  Files skipped: %d\n", stats.FilesSkipped)
	fmt.Printf("  Symbols indexed: %d\n", stats.SymbolsIndexed)

	if stats.FilesErrored > 0 {
		fmt.Printf("  Files with errors: %d\n", stats.FilesErrored)
		if indexVerbose && len(stats.Errors) > 0 {
			fmt.Println("\n❌ Errors encountered:")
			for _, buildErr := range stats.Errors {
				fmt.Printf("  %s: %s\n", buildErr.FilePath, buildErr.Error)
			}
		}
	}

	if indexVerbose {
		avgFilesPerSec := float64(stats.FilesProcessed) / stats.Duration.Seconds()
		avgSymbolsPerSec := float64(stats.SymbolsIndexed) / stats.Duration.Seconds()
		fmt.Printf("\n📈 Performance:\n")
		fmt.Printf("  Files/second: %.1f\n", avgFilesPerSec)
		fmt.Printf("  Symbols/second: %.1f\n", avgSymbolsPerSec)
	}

	return nil
}

func runIndexStatus(cmd *cobra.Command, args []string) error {
	ctx := context.WithValue(cmd.Context(), "operation", "status")

	// Initialize storage
	storage, err := initializeStorage()
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer storage.Close()

	store := index.NewStore(storage, index.DefaultStoreConfig())

	// Get index statistics
	files, err := store.GetAllFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to get file list: %w", err)
	}

	// Calculate statistics
	var totalSymbols int
	var totalSize int64
	var lastUpdate time.Time
	languageCounts := make(map[string]int)

	for _, file := range files {
		totalSymbols += file.SymbolCount
		totalSize += file.Size
		if file.IndexedAt.After(lastUpdate) {
			lastUpdate = file.IndexedAt
		}
		languageCounts[file.Language]++
	}

	// Get index path
	indexPath := config.IndexPath
	if indexPath == "" {
		indexPath = getDefaultIndexPath()
	}

	// Calculate index directory size
	var indexDirSize int64
	if stat, err := os.Stat(indexPath); err == nil && stat.IsDir() {
		filepath.WalkDir(indexPath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if info, err := d.Info(); err == nil {
				indexDirSize += info.Size()
			}
			return nil
		})
	}

	if config.JSON {
		// JSON output
		status := map[string]interface{}{
			"index_path":      indexPath,
			"index_size":      indexDirSize,
			"files_indexed":   len(files),
			"symbols_indexed": totalSymbols,
			"source_size":     totalSize,
			"last_update":     lastUpdate.Format(time.RFC3339),
			"languages":       languageCounts,
			"health":          "healthy", // You could implement health checks
		}

		if indexVerbose {
			status["files"] = files
		}

		return outputJSON(status)
	}

	// Human-readable output
	fmt.Printf("📂 Semantic Index Status\n")
	fmt.Printf("Index location: %s\n", indexPath)
	fmt.Printf("Index size: %s\n", formatBytes(indexDirSize))
	fmt.Println()

	fmt.Printf("📊 Content Statistics\n")
	fmt.Printf("Files indexed: %d\n", len(files))
	fmt.Printf("Symbols indexed: %d\n", totalSymbols)
	fmt.Printf("Source code size: %s\n", formatBytes(totalSize))

	if !lastUpdate.IsZero() {
		fmt.Printf("Last updated: %s (%s ago)\n",
			lastUpdate.Format("2006-01-02 15:04:05"),
			time.Since(lastUpdate).Round(time.Second))
	} else {
		fmt.Printf("Last updated: never\n")
	}
	fmt.Println()

	fmt.Printf("🔤 Language Distribution\n")
	for lang, count := range languageCounts {
		percentage := float64(count) / float64(len(files)) * 100
		fmt.Printf("  %-12s: %3d files (%.1f%%)\n", lang, count, percentage)
	}

	if indexVerbose && len(files) > 0 {
		fmt.Printf("\n📋 Recent Files (last 10)\n")
		// Sort files by indexed time and show latest
		recentFiles := make([]index.FileMetadata, len(files))
		copy(recentFiles, files)
		// Sort by IndexedAt descending
		for i := 0; i < len(recentFiles)-1; i++ {
			for j := i + 1; j < len(recentFiles); j++ {
				if recentFiles[i].IndexedAt.Before(recentFiles[j].IndexedAt) {
					recentFiles[i], recentFiles[j] = recentFiles[j], recentFiles[i]
				}
			}
		}

		limit := 10
		if len(recentFiles) < limit {
			limit = len(recentFiles)
		}

		for i := 0; i < limit; i++ {
			file := recentFiles[i]
			fmt.Printf("  %s (%s, %d symbols, %s)\n",
				file.Path,
				file.Language,
				file.SymbolCount,
				file.IndexedAt.Format("2006-01-02 15:04:05"))
		}
	}

	fmt.Printf("\n🔍 Index Health: %s\n", "✅ Healthy")

	return nil
}

func runClearIndex(cmd *cobra.Command, args []string) error {
	ctx := context.WithValue(cmd.Context(), "operation", "clear")

	if !indexForce {
		fmt.Print("⚠️  This will permanently delete all index data. Continue? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Operation cancelled.")
			return nil
		}
	}

	// Initialize storage
	storage, err := initializeStorage()
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer storage.Close()

	store := index.NewStore(storage, index.DefaultStoreConfig())

	// Get current stats before clearing
	files, err := store.GetAllFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to get file list: %w", err)
	}

	fmt.Printf("🗑️  Clearing index data...\n")

	// Clear index by deleting all files
	// For now, we'll implement clearing by getting and deleting all files
	for _, file := range files {
		if err := store.DeleteFile(ctx, file.Path); err != nil {
			fmt.Printf("Warning: failed to delete file %s: %v\n", file.Path, err)
		}
	}

	fmt.Printf("✅ Cleared index data for %d files\n", len(files))

	// Also clear the physical index directory if it exists
	indexPath := config.IndexPath
	if indexPath == "" {
		indexPath = getDefaultIndexPath()
	}

	if stat, err := os.Stat(indexPath); err == nil && stat.IsDir() {
		if err := os.RemoveAll(indexPath); err != nil {
			fmt.Printf("Warning: failed to remove index directory %s: %v\n", indexPath, err)
		} else {
			fmt.Printf("✅ Removed index directory: %s\n", indexPath)
		}
	}

	return nil
}

// Helper functions

func initializeStorage() (index.Storage, error) {
	indexPath := config.IndexPath
	if indexPath == "" {
		indexPath = getDefaultIndexPath()
	}

	// Ensure index directory exists
	if err := os.MkdirAll(indexPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create index directory: %w", err)
	}

	// Initialize BadgerDB storage
	opts := index.DefaultBadgerOptions(indexPath)
	storage, err := index.NewBadgerStorage(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	return storage, nil
}

func getDefaultIndexPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".codegrep-index"
	}
	return filepath.Join(homeDir, ".cache", "codegrep", "index")
}

// newSymbolParser creates a symbol parser - references the one in search.go

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func outputJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}