package index

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestBasicStorageDemo demonstrates the basic storage functionality
func TestBasicStorageDemo(t *testing.T) {
	// Create in-memory storage for demonstration
	opts := DefaultBadgerOptions("")
	opts.InMemory = true

	storage, err := NewBadgerStorage(opts)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Test basic operations
	t.Run("Basic Operations", func(t *testing.T) {
		key := []byte("demo-key")
		value := []byte("demo-value")

		// Test Set
		if err := storage.Set(ctx, key, value); err != nil {
			t.Errorf("Set failed: %v", err)
		}

		// Test Get
		retrieved, err := storage.Get(ctx, key)
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}

		if string(retrieved) != string(value) {
			t.Errorf("Value mismatch: expected %s, got %s", value, retrieved)
		}

		t.Log("✓ Basic storage operations work correctly")
	})

	// Test batch operations
	t.Run("Batch Operations", func(t *testing.T) {
		batch := storage.Batch()

		// Add multiple items to batch
		for i := 0; i < 5; i++ {
			key := []byte(fmt.Sprintf("batch-key-%d", i))
			value := []byte(fmt.Sprintf("batch-value-%d", i))
			batch.Set(key, value)
		}

		// Execute batch
		if err := storage.WriteBatch(ctx, batch); err != nil {
			t.Errorf("WriteBatch failed: %v", err)
		}

		// Verify all items were written
		for i := 0; i < 5; i++ {
			key := []byte(fmt.Sprintf("batch-key-%d", i))
			value, err := storage.Get(ctx, key)
			if err != nil {
				t.Errorf("Failed to get batch item %d: %v", i, err)
			} else {
				expectedValue := fmt.Sprintf("batch-value-%d", i)
				if string(value) != expectedValue {
					t.Errorf("Batch item %d: expected %s, got %s", i, expectedValue, value)
				}
			}
		}

		t.Log("✓ Batch operations work correctly")
	})

	// Test statistics
	t.Run("Statistics", func(t *testing.T) {
		stats := storage.Stats()

		if stats.ReadCount == 0 && stats.WriteCount == 0 {
			t.Error("Expected some read/write operations")
		}

		t.Logf("✓ Statistics: %d reads, %d writes", stats.ReadCount, stats.WriteCount)
	})
}

// TestStoreDemo demonstrates the high-level store functionality
func TestStoreDemo(t *testing.T) {
	// Create in-memory storage
	opts := DefaultBadgerOptions("")
	opts.InMemory = true

	storage, err := NewBadgerStorage(opts)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create store
	store := NewStore(storage, DefaultStoreConfig())
	ctx := context.Background()

	// Test symbol operations
	t.Run("Symbol Operations", func(t *testing.T) {
		symbol := SymbolInfo{
			ID:          "demo-func",
			Name:        "DemoFunction",
			Type:        "function",
			Kind:        "function",
			FilePath:    "/demo/file.go",
			StartLine:   10,
			EndLine:     20,
			Signature:   "func DemoFunction() error",
			Tags:        []string{"demo", "test"},
			LastUpdated: time.Now(),
		}

		// Store symbol
		if err := store.StoreSymbol(ctx, symbol); err != nil {
			t.Errorf("Failed to store symbol: %v", err)
		}

		// Retrieve symbol
		retrieved, err := store.GetSymbol(ctx, symbol.FilePath, symbol.ID)
		if err != nil {
			t.Errorf("Failed to get symbol: %v", err)
		} else {
			if retrieved.Name != symbol.Name {
				t.Errorf("Symbol name mismatch: expected %s, got %s", symbol.Name, retrieved.Name)
			}
		}

		t.Log("✓ Symbol storage and retrieval work correctly")
	})

	// Test file metadata
	t.Run("File Metadata", func(t *testing.T) {
		metadata := FileMetadata{
			Path:        "/demo/file.go",
			Hash:        "demo-hash-123",
			Size:        1024,
			ModTime:     time.Now(),
			Language:    "Go",
			SymbolCount: 1,
			IndexedAt:   time.Now(),
		}

		// Store metadata
		if err := store.StoreFileMetadata(ctx, metadata); err != nil {
			t.Errorf("Failed to store file metadata: %v", err)
		}

		// Retrieve metadata
		retrieved, err := store.GetFileMetadata(ctx, metadata.Path)
		if err != nil {
			t.Errorf("Failed to get file metadata: %v", err)
		} else {
			if retrieved.Language != metadata.Language {
				t.Errorf("Language mismatch: expected %s, got %s", metadata.Language, retrieved.Language)
			}
		}

		t.Log("✓ File metadata storage and retrieval work correctly")
	})
}

// TestPerformanceDemo demonstrates performance characteristics
func TestPerformanceDemo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance demo in short mode")
	}

	// Create in-memory storage for speed
	opts := DefaultBadgerOptions("")
	opts.InMemory = true

	storage, err := NewBadgerStorage(opts)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Performance test: sequential writes
	t.Run("Sequential Write Performance", func(t *testing.T) {
		numOps := 10000
		start := time.Now()

		for i := 0; i < numOps; i++ {
			key := []byte(fmt.Sprintf("perf-key-%06d", i))
			value := []byte(fmt.Sprintf("perf-value-%06d-data", i))
			if err := storage.Set(ctx, key, value); err != nil {
				t.Errorf("Write %d failed: %v", i, err)
				return
			}
		}

		duration := time.Since(start)
		opsPerSec := float64(numOps) / duration.Seconds()

		t.Logf("✓ Sequential writes: %d ops in %v (%.0f ops/sec)", numOps, duration, opsPerSec)
	})

	// Performance test: batch writes
	t.Run("Batch Write Performance", func(t *testing.T) {
		numOps := 10000
		batchSize := 100
		numBatches := numOps / batchSize

		start := time.Now()

		for b := 0; b < numBatches; b++ {
			batch := storage.Batch()
			for i := 0; i < batchSize; i++ {
				key := []byte(fmt.Sprintf("batch-perf-key-%06d", b*batchSize+i))
				value := []byte(fmt.Sprintf("batch-perf-value-%06d-data", b*batchSize+i))
				batch.Set(key, value)
			}
			if err := storage.WriteBatch(ctx, batch); err != nil {
				t.Errorf("Batch %d failed: %v", b, err)
				return
			}
		}

		duration := time.Since(start)
		opsPerSec := float64(numOps) / duration.Seconds()

		t.Logf("✓ Batch writes: %d ops in %v (%.0f ops/sec)", numOps, duration, opsPerSec)
	})

	// Show final statistics
	stats := storage.Stats()
	t.Logf("Final statistics:")
	t.Logf("  Total operations: %d writes, %d reads", stats.WriteCount, stats.ReadCount)
	t.Logf("  Cache performance: %d hits, %d misses", stats.CacheHits, stats.CacheMisses)
	t.Logf("  Storage size: %d bytes", stats.TotalSize)
}

// TestConfigurationDemo demonstrates various configuration options
func TestConfigurationDemo(t *testing.T) {
	t.Run("BadgerDB Configuration", func(t *testing.T) {
		// Test default configuration
		opts := DefaultBadgerOptions("/tmp/test")
		if opts.ValueLogFileSize != (1 << 30) {
			t.Errorf("Default value log size should be 1GB, got %d", opts.ValueLogFileSize)
		}

		// Test in-memory configuration
		inMemoryOpts := DefaultBadgerOptions("")
		inMemoryOpts.InMemory = true
		storage, err := NewBadgerStorage(inMemoryOpts)
		if err != nil {
			t.Errorf("Failed to create in-memory storage: %v", err)
		} else {
			storage.Close()
			t.Log("✓ In-memory storage configuration works")
		}
	})

	t.Run("Store Configuration", func(t *testing.T) {
		config := DefaultStoreConfig()

		if config.QueryCacheTTL != 30*time.Minute {
			t.Errorf("Default cache TTL should be 30 minutes, got %v", config.QueryCacheTTL)
		}

		if !config.CacheEnabled {
			t.Error("Cache should be enabled by default")
		}

		t.Log("✓ Store configuration is correct")
	})
}