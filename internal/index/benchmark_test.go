package index

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

// BenchmarkBadgerStorage tests the performance of BadgerDB operations
func BenchmarkBadgerStorage(b *testing.B) {
	// Create temporary directory for benchmark
	tmpDir, err := os.MkdirTemp("", "badger-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	opts := DefaultBadgerOptions(tmpDir)
	storage, err := NewBadgerStorage(opts)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	b.Run("Sequential Writes", func(b *testing.B) {
		benchmarkSequentialWrites(b, storage, ctx)
	})

	b.Run("Sequential Reads", func(b *testing.B) {
		benchmarkSequentialReads(b, storage, ctx)
	})

	b.Run("Random Reads", func(b *testing.B) {
		benchmarkRandomReads(b, storage, ctx)
	})

	b.Run("Batch Writes", func(b *testing.B) {
		benchmarkBatchWrites(b, storage, ctx)
	})

	b.Run("Concurrent Writes", func(b *testing.B) {
		benchmarkConcurrentWrites(b, storage, ctx)
	})

	b.Run("Concurrent Reads", func(b *testing.B) {
		benchmarkConcurrentReads(b, storage, ctx)
	})

	b.Run("Mixed Workload", func(b *testing.B) {
		benchmarkMixedWorkload(b, storage, ctx)
	})

	b.Run("Prefix Scanning", func(b *testing.B) {
		benchmarkPrefixScanning(b, storage, ctx)
	})
}

func benchmarkSequentialWrites(b *testing.B, storage Storage, ctx context.Context) {
	keys := make([][]byte, b.N)
	values := make([][]byte, b.N)

	// Pre-generate test data
	for i := 0; i < b.N; i++ {
		keys[i] = []byte(fmt.Sprintf("key-%010d", i))
		values[i] = []byte(fmt.Sprintf("value-%010d-%s", i, randomString(100)))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := storage.Set(ctx, keys[i], values[i]); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkSequentialReads(b *testing.B, storage Storage, ctx context.Context) {
	// Pre-populate with test data
	for i := 0; i < b.N; i++ {
		key := []byte(fmt.Sprintf("read-key-%010d", i))
		value := []byte(fmt.Sprintf("read-value-%010d-%s", i, randomString(100)))
		if err := storage.Set(ctx, key, value); err != nil {
			b.Fatal(err)
		}
	}

	keys := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = []byte(fmt.Sprintf("read-key-%010d", i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := storage.Get(ctx, keys[i])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkRandomReads(b *testing.B, storage Storage, ctx context.Context) {
	// Pre-populate with test data
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("rand-key-%010d", i))
		value := []byte(fmt.Sprintf("rand-value-%010d-%s", i, randomString(100)))
		if err := storage.Set(ctx, key, value); err != nil {
			b.Fatal(err)
		}
	}

	// Generate random indices
	indices := make([]int, b.N)
	for i := 0; i < b.N; i++ {
		indices[i] = rand.Intn(numKeys)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := []byte(fmt.Sprintf("rand-key-%010d", indices[i]))
		_, err := storage.Get(ctx, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkBatchWrites(b *testing.B, storage Storage, ctx context.Context) {
	batchSize := 1000
	numBatches := b.N / batchSize

	b.ResetTimer()
	b.ReportAllocs()

	for batch := 0; batch < numBatches; batch++ {
		writeBatch := storage.Batch()

		for i := 0; i < batchSize; i++ {
			key := []byte(fmt.Sprintf("batch-key-%010d-%010d", batch, i))
			value := []byte(fmt.Sprintf("batch-value-%010d-%010d-%s", batch, i, randomString(100)))
			writeBatch.Set(key, value)
		}

		if err := storage.WriteBatch(ctx, writeBatch); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkConcurrentWrites(b *testing.B, storage Storage, ctx context.Context) {
	numWorkers := 10
	keysPerWorker := b.N / numWorkers

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for worker := 0; worker < numWorkers; worker++ {
		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < keysPerWorker; i++ {
				key := []byte(fmt.Sprintf("worker-%03d-key-%010d", workerID, i))
				value := []byte(fmt.Sprintf("worker-%03d-value-%010d-%s", workerID, i, randomString(100)))

				if err := storage.Set(ctx, key, value); err != nil {
					b.Error(err)
					return
				}
			}
		}(worker)
	}

	wg.Wait()
}

func benchmarkConcurrentReads(b *testing.B, storage Storage, ctx context.Context) {
	// Pre-populate with test data
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("concurrent-read-key-%010d", i))
		value := []byte(fmt.Sprintf("concurrent-read-value-%010d-%s", i, randomString(100)))
		if err := storage.Set(ctx, key, value); err != nil {
			b.Fatal(err)
		}
	}

	numWorkers := 10
	readsPerWorker := b.N / numWorkers

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for worker := 0; worker < numWorkers; worker++ {
		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < readsPerWorker; i++ {
				keyIndex := rand.Intn(numKeys)
				key := []byte(fmt.Sprintf("concurrent-read-key-%010d", keyIndex))

				_, err := storage.Get(ctx, key)
				if err != nil {
					b.Error(err)
					return
				}
			}
		}(worker)
	}

	wg.Wait()
}

func benchmarkMixedWorkload(b *testing.B, storage Storage, ctx context.Context) {
	// 70% reads, 20% writes, 10% deletes
	readRatio := 0.7
	writeRatio := 0.2

	// Pre-populate with some data
	initialKeys := 1000
	for i := 0; i < initialKeys; i++ {
		key := []byte(fmt.Sprintf("mixed-key-%010d", i))
		value := []byte(fmt.Sprintf("mixed-value-%010d-%s", i, randomString(100)))
		if err := storage.Set(ctx, key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r := rand.Float64()

		if r < readRatio {
			// Read operation
			keyIndex := rand.Intn(initialKeys + i/10) // Account for new writes
			key := []byte(fmt.Sprintf("mixed-key-%010d", keyIndex))
			storage.Get(ctx, key) // Ignore errors for missing keys
		} else if r < readRatio+writeRatio {
			// Write operation
			key := []byte(fmt.Sprintf("mixed-key-%010d", initialKeys+i))
			value := []byte(fmt.Sprintf("mixed-value-%010d-%s", initialKeys+i, randomString(100)))
			if err := storage.Set(ctx, key, value); err != nil {
				b.Fatal(err)
			}
		} else {
			// Delete operation
			keyIndex := rand.Intn(initialKeys + i/10)
			key := []byte(fmt.Sprintf("mixed-key-%010d", keyIndex))
			storage.Delete(ctx, key) // Ignore errors for missing keys
		}
	}
}

func benchmarkPrefixScanning(b *testing.B, storage Storage, ctx context.Context) {
	// Pre-populate with test data using multiple prefixes
	prefixes := []string{"scan-a:", "scan-b:", "scan-c:", "scan-d:", "scan-e:"}
	keysPerPrefix := 1000

	for _, prefix := range prefixes {
		for i := 0; i < keysPerPrefix; i++ {
			key := []byte(fmt.Sprintf("%s%010d", prefix, i))
			value := []byte(fmt.Sprintf("value-%s%010d-%s", prefix, i, randomString(100)))
			if err := storage.Set(ctx, key, value); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		prefix := prefixes[i%len(prefixes)]
		iter := storage.Scan(ctx, []byte(prefix), ScanOptions{Limit: 100})

		count := 0
		for iter.Next() {
			count++
		}
		iter.Close()

		if err := iter.Error(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStore tests high-level store operations
func BenchmarkStore(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "store-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	opts := DefaultBadgerOptions(tmpDir)
	storage, err := NewBadgerStorage(opts)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	store := NewStore(storage, DefaultStoreConfig())
	ctx := context.Background()

	b.Run("Symbol Storage", func(b *testing.B) {
		benchmarkSymbolStorage(b, store, ctx)
	})

	b.Run("Symbol Search", func(b *testing.B) {
		benchmarkSymbolSearch(b, store, ctx)
	})

	b.Run("File Operations", func(b *testing.B) {
		benchmarkFileOperations(b, store, ctx)
	})
}

func benchmarkSymbolStorage(b *testing.B, store *Store, ctx context.Context) {
	symbols := make([]SymbolInfo, b.N)

	// Pre-generate symbols
	for i := 0; i < b.N; i++ {
		symbols[i] = SymbolInfo{
			ID:        fmt.Sprintf("symbol-%010d", i),
			Name:      fmt.Sprintf("Function_%d", i),
			Type:      "function",
			Kind:      "function",
			FilePath:  fmt.Sprintf("/test/file_%d.go", i/100), // 100 symbols per file
			StartLine: i%1000 + 1,
			EndLine:   i%1000 + 10,
			StartCol:  1,
			EndCol:    20,
			Signature: fmt.Sprintf("func Function_%d() error", i),
			Tags:      []string{"generated", "benchmark"},
			Properties: map[string]string{
				"visibility": "public",
				"complexity": strconv.Itoa(rand.Intn(10) + 1),
			},
			LastUpdated: time.Now(),
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := store.StoreSymbol(ctx, symbols[i]); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkSymbolSearch(b *testing.B, store *Store, ctx context.Context) {
	// Pre-populate with symbols
	numSymbols := 10000
	for i := 0; i < numSymbols; i++ {
		symbol := SymbolInfo{
			ID:       fmt.Sprintf("search-symbol-%010d", i),
			Name:     fmt.Sprintf("SearchFunction_%d", i),
			Type:     "function",
			Kind:     "function",
			FilePath: fmt.Sprintf("/test/search_file_%d.go", i/100),
			Tags:     []string{"searchable", fmt.Sprintf("group_%d", i%10)},
		}
		if err := store.StoreSymbol(ctx, symbol); err != nil {
			b.Fatal(err)
		}
	}

	// Prepare search queries
	queries := []SearchQuery{
		{Type: SearchByName, Term: "SearchFunction_1000"},
		{Type: SearchByType, Term: "function"},
		{Type: SearchByTag, Term: "searchable"},
		{Type: SearchByPattern, Term: "SearchFunction"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		query := queries[i%len(queries)]
		_, err := store.SearchSymbols(ctx, query)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkFileOperations(b *testing.B, store *Store, ctx context.Context) {
	files := make([]FileMetadata, b.N)

	// Pre-generate file metadata
	for i := 0; i < b.N; i++ {
		files[i] = FileMetadata{
			Path:        fmt.Sprintf("/test/bench_file_%010d.go", i),
			Hash:        fmt.Sprintf("hash_%010d", i),
			Size:        int64(rand.Intn(100000) + 1000),
			ModTime:     time.Now(),
			Language:    "Go",
			SymbolCount: rand.Intn(50) + 1,
			IndexedAt:   time.Now(),
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := store.StoreFileMetadata(ctx, files[i]); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBuilder tests the index builder performance
func BenchmarkBuilder(b *testing.B) {
	// This benchmark would require a mock parser implementation
	// For now, we'll skip it as it requires more complex setup
	b.Skip("Builder benchmarks require mock parser implementation")
}

// Utility functions for benchmarks

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

// BenchmarkMemoryUsage provides memory usage benchmarks
func BenchmarkMemoryUsage(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "memory-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	opts := DefaultBadgerOptions(tmpDir)
	storage, err := NewBadgerStorage(opts)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	b.Run("Large Values", func(b *testing.B) {
		valueSize := 10 * 1024 // 10KB values

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := []byte(fmt.Sprintf("large-key-%010d", i))
			value := make([]byte, valueSize)
			rand.Read(value)

			if err := storage.Set(ctx, key, value); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Many Small Keys", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := []byte(fmt.Sprintf("small-key-%010d", i))
			value := []byte(fmt.Sprintf("small-value-%d", i))

			if err := storage.Set(ctx, key, value); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Performance comparison benchmarks
func BenchmarkComparison(b *testing.B) {
	b.Run("InMemory vs OnDisk", func(b *testing.B) {
		b.Run("InMemory", func(b *testing.B) {
			opts := DefaultBadgerOptions("")
			opts.InMemory = true
			benchmarkStoragePerformance(b, opts)
		})

		b.Run("OnDisk", func(b *testing.B) {
			tmpDir, err := os.MkdirTemp("", "disk-bench-*")
			if err != nil {
				b.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			opts := DefaultBadgerOptions(tmpDir)
			benchmarkStoragePerformance(b, opts)
		})
	})
}

func benchmarkStoragePerformance(b *testing.B, opts BadgerOptions) {
	storage, err := NewBadgerStorage(opts)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := []byte(fmt.Sprintf("perf-key-%010d", i))
		value := []byte(fmt.Sprintf("perf-value-%010d-%s", i, randomString(100)))

		if err := storage.Set(ctx, key, value); err != nil {
			b.Fatal(err)
		}
	}
}