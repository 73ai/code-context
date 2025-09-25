package index

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
)

// BadgerStorage implements the Storage interface using BadgerDB
// Optimized for high-performance code indexing with efficient prefix scanning
type BadgerStorage struct {
	db    *badger.DB
	opts  BadgerOptions
	stats *badgerStats
	mutex sync.RWMutex
}

// BadgerOptions configures the BadgerDB instance for optimal performance
type BadgerOptions struct {
	// Directory to store the database files
	Dir string

	// InMemory creates an in-memory database (for testing)
	InMemory bool

	// ReadOnly opens database in read-only mode
	ReadOnly bool

	// ValueLogFileSize sets the maximum size of value log files (default: 1GB)
	ValueLogFileSize int64

	// NumMemtables sets the number of memtables (default: 5)
	NumMemtables int

	// NumLevelZeroTables sets L0 table count (default: 5)
	NumLevelZeroTables int

	// NumLevelZeroTablesStall sets L0 stall count (default: 15)
	NumLevelZeroTablesStall int

	// SyncWrites enables synchronous writes (default: false for performance)
	SyncWrites bool

	// CompactL0OnClose enables L0 compaction on close (default: true)
	CompactL0OnClose bool

	// Cache size in MB (0 = no cache)
	BlockCacheSize int64

	// Index cache size in MB (0 = no cache)
	IndexCacheSize int64
}

// DefaultBadgerOptions returns optimized options for code indexing workloads
func DefaultBadgerOptions(dir string) BadgerOptions {
	return BadgerOptions{
		Dir:                     dir,
		InMemory:                false,
		ReadOnly:                false,
		ValueLogFileSize:        1 << 30, // 1GB
		NumMemtables:            5,
		NumLevelZeroTables:      5,
		NumLevelZeroTablesStall: 15,
		SyncWrites:              false, // Better performance for bulk indexing
		CompactL0OnClose:        true,
		BlockCacheSize:          256, // 256MB block cache
		IndexCacheSize:          64,  // 64MB index cache
	}
}

type badgerStats struct {
	// Operation counters
	readCount   int64
	writeCount  int64
	scanCount   int64
	deleteCount int64

	// Cache metrics
	cacheHits   int64
	cacheMisses int64

	// Timing metrics (in nanoseconds)
	totalReadTime  int64
	totalWriteTime int64
	totalScanTime  int64

	// Last updated
	lastUpdated time.Time
}

// NewBadgerStorage creates a new BadgerDB-backed storage instance
func NewBadgerStorage(opts BadgerOptions) (*BadgerStorage, error) {
	badgerOpts := badger.DefaultOptions(opts.Dir).
		WithValueLogFileSize(opts.ValueLogFileSize).
		WithNumMemtables(opts.NumMemtables).
		WithNumLevelZeroTables(opts.NumLevelZeroTables).
		WithNumLevelZeroTablesStall(opts.NumLevelZeroTablesStall).
		WithSyncWrites(opts.SyncWrites).
		WithCompactL0OnClose(opts.CompactL0OnClose)

	// Configure caching for better performance
	if opts.BlockCacheSize > 0 {
		badgerOpts = badgerOpts.WithBlockCacheSize(opts.BlockCacheSize << 20) // Convert MB to bytes
	}
	if opts.IndexCacheSize > 0 {
		badgerOpts = badgerOpts.WithIndexCacheSize(opts.IndexCacheSize << 20) // Convert MB to bytes
	}

	// Optimize for read-heavy workloads (code searching)
	badgerOpts = badgerOpts.
		WithDetectConflicts(false). // Faster writes, we handle conflicts at app level
		WithNumGoroutines(8).       // More goroutines for compaction
		WithCompression(options.ZSTD)

	// Configure for in-memory or read-only modes
	if opts.InMemory {
		badgerOpts = badgerOpts.WithInMemory(true)
	}
	if opts.ReadOnly {
		badgerOpts = badgerOpts.WithReadOnly(true)
	}

	// Optimize logging for production use
	badgerOpts = badgerOpts.WithLogger(nil) // Disable internal logging

	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger database: %w", err)
	}

	storage := &BadgerStorage{
		db:   db,
		opts: opts,
		stats: &badgerStats{
			lastUpdated: time.Now(),
		},
	}

	// Start background garbage collection
	go storage.runGC()

	return storage, nil
}

// runGC runs periodic garbage collection
func (bs *BadgerStorage) runGC() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Run value log GC
		for {
			err := bs.db.RunValueLogGC(0.5) // Run GC if 50% of space can be reclaimed
			if err != nil {
				break
			}
		}
	}
}

// Get retrieves a value by key
func (bs *BadgerStorage) Get(ctx context.Context, key []byte) ([]byte, error) {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&bs.stats.readCount, 1)
		atomic.AddInt64(&bs.stats.totalReadTime, time.Since(start).Nanoseconds())
	}()

	var result []byte
	err := bs.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				atomic.AddInt64(&bs.stats.cacheMisses, 1)
				return ErrKeyNotFound
			}
			return err
		}

		atomic.AddInt64(&bs.stats.cacheHits, 1)
		return item.Value(func(val []byte) error {
			result = append([]byte{}, val...) // Copy the value
			return nil
		})
	})

	return result, err
}

// Set stores a key-value pair
func (bs *BadgerStorage) Set(ctx context.Context, key, value []byte) error {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&bs.stats.writeCount, 1)
		atomic.AddInt64(&bs.stats.totalWriteTime, time.Since(start).Nanoseconds())
	}()

	return bs.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

// Delete removes a key
func (bs *BadgerStorage) Delete(ctx context.Context, key []byte) error {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&bs.stats.deleteCount, 1)
		atomic.AddInt64(&bs.stats.totalWriteTime, time.Since(start).Nanoseconds())
	}()

	return bs.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// Has checks if a key exists
func (bs *BadgerStorage) Has(ctx context.Context, key []byte) (bool, error) {
	err := bs.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		return err
	})

	if err == badger.ErrKeyNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// badgerBatch implements the Batch interface
type badgerBatch struct {
	wb    *badger.WriteBatch
	count int
}

// Batch creates a new batch for bulk operations
func (bs *BadgerStorage) Batch() Batch {
	return &badgerBatch{
		wb: bs.db.NewWriteBatch(),
	}
}

func (bb *badgerBatch) Set(key, value []byte) {
	bb.wb.Set(key, value)
	bb.count++
}

func (bb *badgerBatch) Delete(key []byte) {
	bb.wb.Delete(key)
	bb.count++
}

func (bb *badgerBatch) Clear() {
	bb.wb.Cancel()
	bb.count = 0
}

func (bb *badgerBatch) Size() int {
	return bb.count
}

// WriteBatch executes a batch of operations atomically
func (bs *BadgerStorage) WriteBatch(ctx context.Context, batch Batch) error {
	bb, ok := batch.(*badgerBatch)
	if !ok {
		return fmt.Errorf("invalid batch type")
	}

	start := time.Now()
	defer func() {
		atomic.AddInt64(&bs.stats.writeCount, int64(bb.count))
		atomic.AddInt64(&bs.stats.totalWriteTime, time.Since(start).Nanoseconds())
	}()

	return bb.wb.Flush()
}

// badgerIterator implements the Iterator interface
type badgerIterator struct {
	iter   *badger.Iterator
	txn    *badger.Txn
	ctx    context.Context
	err    error
	closed bool
	first  bool // Track if this is the first call to Next()
}

func (bi *badgerIterator) Next() bool {
	if bi.closed || bi.err != nil {
		return false
	}

	// Check context cancellation
	select {
	case <-bi.ctx.Done():
		bi.err = bi.ctx.Err()
		return false
	default:
	}

	// For the first call, don't advance - just check if we have a valid item
	// For subsequent calls, advance to the next item
	if !bi.first {
		bi.first = true
		return bi.iter.Valid()
	}

	// Advance to the next item
	bi.iter.Next()
	return bi.iter.Valid()
}

func (bi *badgerIterator) Key() []byte {
	if !bi.iter.Valid() {
		return nil
	}
	return bi.iter.Item().KeyCopy(nil)
}

func (bi *badgerIterator) Value() []byte {
	if !bi.iter.Valid() {
		return nil
	}

	var value []byte
	bi.err = bi.iter.Item().Value(func(val []byte) error {
		value = append([]byte{}, val...)
		return nil
	})
	return value
}

func (bi *badgerIterator) Error() error {
	return bi.err
}

func (bi *badgerIterator) Close() {
	if !bi.closed {
		bi.iter.Close()
		if bi.txn != nil {
			bi.txn.Discard()
			bi.txn = nil
		}
		bi.closed = true
	}
}

// Scan creates an iterator for prefix scanning
func (bs *BadgerStorage) Scan(ctx context.Context, prefix []byte, opts ScanOptions) Iterator {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&bs.stats.scanCount, 1)
		atomic.AddInt64(&bs.stats.totalScanTime, time.Since(start).Nanoseconds())
	}()

	// Create a read transaction that will be managed by the iterator
	txn := bs.db.NewTransaction(false)
	badgerOpts := badger.DefaultIteratorOptions
	badgerOpts.Reverse = opts.Reverse
	badgerOpts.PrefetchValues = !opts.KeysOnly

	iter := txn.NewIterator(badgerOpts)

	// Position the iterator
	if opts.StartAfter != nil {
		iter.Seek(opts.StartAfter)
		if iter.Valid() && string(iter.Item().Key()) == string(opts.StartAfter) {
			iter.Next() // Skip the StartAfter key itself
		}
	} else {
		iter.Seek(prefix)
	}

	return &badgerIterator{
		iter:  iter,
		txn:   txn,
		ctx:   ctx,
		first: false,
	}
}

// badgerTxn implements the Txn interface
type badgerTxn struct {
	txn *badger.Txn
	bs  *BadgerStorage
}

// Transaction executes a function within a transaction
func (bs *BadgerStorage) Transaction(ctx context.Context, fn func(Txn) error) error {
	return bs.db.Update(func(txn *badger.Txn) error {
		btxn := &badgerTxn{
			txn: txn,
			bs:  bs,
		}
		return fn(btxn)
	})
}

func (bt *badgerTxn) Get(key []byte) ([]byte, error) {
	item, err := bt.txn.Get(key)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}

	var result []byte
	err = item.Value(func(val []byte) error {
		result = append([]byte{}, val...)
		return nil
	})
	return result, err
}

func (bt *badgerTxn) Set(key, value []byte) error {
	return bt.txn.Set(key, value)
}

func (bt *badgerTxn) Delete(key []byte) error {
	return bt.txn.Delete(key)
}

func (bt *badgerTxn) Has(key []byte) (bool, error) {
	_, err := bt.txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (bt *badgerTxn) Scan(prefix []byte, opts ScanOptions) Iterator {
	badgerOpts := badger.DefaultIteratorOptions
	badgerOpts.Reverse = opts.Reverse
	badgerOpts.PrefetchValues = !opts.KeysOnly

	iter := bt.txn.NewIterator(badgerOpts)
	iter.Seek(prefix)

	return &badgerIterator{
		iter:  iter,
		txn:   nil, // Transaction is managed externally in this case
		ctx:   context.Background(),
		first: false,
	}
}

// Backup creates a backup of the database
func (bs *BadgerStorage) Backup(ctx context.Context, w io.Writer) error {
	_, err := bs.db.Backup(w, 0)
	return err
}

// Restore restores the database from a backup
func (bs *BadgerStorage) Restore(ctx context.Context, r io.Reader) error {
	return bs.db.Load(r, 256) // Load with 256 goroutines for faster restore
}

// Close closes the database connection
func (bs *BadgerStorage) Close() error {
	return bs.db.Close()
}

// Stats returns storage statistics
func (bs *BadgerStorage) Stats() StorageStats {
	bs.mutex.RLock()
	defer bs.mutex.RUnlock()

	lsm, vlog := bs.db.Size()

	readCount := atomic.LoadInt64(&bs.stats.readCount)
	writeCount := atomic.LoadInt64(&bs.stats.writeCount)
	scanCount := atomic.LoadInt64(&bs.stats.scanCount)

	totalReadTime := atomic.LoadInt64(&bs.stats.totalReadTime)
	totalWriteTime := atomic.LoadInt64(&bs.stats.totalWriteTime)
	totalScanTime := atomic.LoadInt64(&bs.stats.totalScanTime)

	var avgReadTime, avgWriteTime, avgScanTime int64
	if readCount > 0 {
		avgReadTime = totalReadTime / readCount
	}
	if writeCount > 0 {
		avgWriteTime = totalWriteTime / writeCount
	}
	if scanCount > 0 {
		avgScanTime = totalScanTime / scanCount
	}

	return StorageStats{
		TotalSize:     lsm + vlog,
		KeyCount:      0, // BadgerDB doesn't provide easy key count
		IndexSize:     lsm,
		ReadCount:     readCount,
		WriteCount:    writeCount,
		ScanCount:     scanCount,
		CacheHits:     atomic.LoadInt64(&bs.stats.cacheHits),
		CacheMisses:   atomic.LoadInt64(&bs.stats.cacheMisses),
		AvgReadTime:   avgReadTime,
		AvgWriteTime:  avgWriteTime,
		AvgScanTime:   avgScanTime,
		LastUpdated:   bs.stats.lastUpdated,
	}
}

// Size returns the total size of the database
func (bs *BadgerStorage) Size() (int64, error) {
	lsm, vlog := bs.db.Size()
	return lsm + vlog, nil
}

// GC runs garbage collection
func (bs *BadgerStorage) GC(ctx context.Context) error {
	// Run value log GC until no more cleanup is possible
	for {
		err := bs.db.RunValueLogGC(0.5)
		if err != nil {
			if err == badger.ErrNoRewrite {
				return nil // No more GC needed
			}
			return err
		}
	}
}

// Compact runs database compaction
func (bs *BadgerStorage) Compact(ctx context.Context) error {
	// Flatten the LSM tree by running compaction
	for level := 0; level < 7; level++ {
		err := bs.db.Flatten(level)
		if err != nil {
			return err
		}
	}
	return nil
}

// DropAll removes all data from the database (useful for testing)
func (bs *BadgerStorage) DropAll(ctx context.Context) error {
	return bs.db.DropAll()
}

// GetSequence gets a sequence generator for auto-incrementing IDs
func (bs *BadgerStorage) GetSequence(key []byte, bandwidth uint64) (*badger.Sequence, error) {
	return bs.db.GetSequence(key, bandwidth)
}

// Path returns the directory path of the database
func (bs *BadgerStorage) Path() string {
	return bs.opts.Dir
}

// IsReadOnly returns true if the database is opened in read-only mode
func (bs *BadgerStorage) IsReadOnly() bool {
	return bs.opts.ReadOnly
}

// Opts returns the BadgerOptions used to create this storage
func (bs *BadgerStorage) Opts() BadgerOptions {
	return bs.opts
}