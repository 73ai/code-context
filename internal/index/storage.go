package index

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"
)

// Storage defines the unified interface for all storage operations in codegrep.
// It provides key-value storage with prefix scanning, transactions, and batch operations
// optimized for storing code symbols, metadata, and query results.
type Storage interface {
	// Basic key-value operations
	Get(ctx context.Context, key []byte) ([]byte, error)
	Set(ctx context.Context, key, value []byte) error
	Delete(ctx context.Context, key []byte) error
	Has(ctx context.Context, key []byte) (bool, error)

	// Batch operations for efficient bulk writes
	Batch() Batch
	WriteBatch(ctx context.Context, batch Batch) error

	// Prefix scanning for range queries
	Scan(ctx context.Context, prefix []byte, opts ScanOptions) Iterator

	// Transactions for atomic multi-operation updates
	Transaction(ctx context.Context, fn func(Txn) error) error

	// Database management
	Backup(ctx context.Context, w io.Writer) error
	Restore(ctx context.Context, r io.Reader) error
	Close() error

	// Statistics and monitoring
	Stats() StorageStats
	Size() (int64, error)

	// Maintenance operations
	GC(ctx context.Context) error
	Compact(ctx context.Context) error
}

// Batch represents a collection of operations to be executed atomically
type Batch interface {
	Set(key, value []byte)
	Delete(key []byte)
	Clear()
	Size() int
}

// Txn represents a transaction for atomic multi-operation updates
type Txn interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	Delete(key []byte) error
	Has(key []byte) (bool, error)
	Scan(prefix []byte, opts ScanOptions) Iterator
}

// Iterator provides sequential access to key-value pairs
type Iterator interface {
	Next() bool
	Key() []byte
	Value() []byte
	Error() error
	Close()
}

// ScanOptions controls prefix scanning behavior
type ScanOptions struct {
	// Reverse iterates in reverse order
	Reverse bool

	// Limit restricts the number of results (0 = no limit)
	Limit int

	// KeysOnly returns only keys, not values (more efficient)
	KeysOnly bool

	// StartAfter begins iteration after this key
	StartAfter []byte
}

// StorageStats provides insights into storage performance and usage
type StorageStats struct {
	// Size metrics
	TotalSize     int64 `json:"total_size"`
	KeyCount      int64 `json:"key_count"`
	IndexSize     int64 `json:"index_size"`

	// Performance metrics
	ReadCount     int64 `json:"read_count"`
	WriteCount    int64 `json:"write_count"`
	ScanCount     int64 `json:"scan_count"`

	// Cache metrics
	CacheHits     int64 `json:"cache_hits"`
	CacheMisses   int64 `json:"cache_misses"`

	// Timing metrics (in nanoseconds)
	AvgReadTime   int64 `json:"avg_read_time"`
	AvgWriteTime  int64 `json:"avg_write_time"`
	AvgScanTime   int64 `json:"avg_scan_time"`

	// Last update timestamp
	LastUpdated   time.Time `json:"last_updated"`
}

// Key prefixes for different data types stored in the database
const (
	// Symbol storage prefixes
	PrefixSymbol     = "sym:"  // sym:{file_hash}:{symbol_id} -> SymbolInfo
	PrefixFile       = "file:" // file:{file_path_hash} -> FileMetadata
	PrefixRef        = "ref:"  // ref:{symbol_hash}:{file_hash}:{line} -> Reference

	// Index prefixes
	PrefixName       = "name:" // name:{symbol_name} -> []symbol_id
	PrefixType       = "type:" // type:{type_name} -> []symbol_id
	PrefixTag        = "tag:"  // tag:{tag_name} -> []symbol_id

	// Cache prefixes
	PrefixQuery      = "query:" // query:{query_hash} -> QueryResult
	PrefixStats      = "stats:" // stats:{timestamp} -> SearchStats

	// Metadata prefixes
	PrefixConfig     = "config:" // config:key -> value
	PrefixVersion    = "version" // version -> schema_version
	PrefixTimestamp  = "ts:"     // ts:last_update -> timestamp
)

// SymbolInfo represents a code symbol with its metadata
type SymbolInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Kind        string            `json:"kind"`
	FilePath    string            `json:"file_path"`
	StartLine   int               `json:"start_line"`
	EndLine     int               `json:"end_line"`
	StartCol    int               `json:"start_col"`
	EndCol      int               `json:"end_col"`
	Signature   string            `json:"signature,omitempty"`
	DocString   string            `json:"doc_string,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
	LastUpdated time.Time         `json:"last_updated"`
}

// FileMetadata contains metadata about indexed files
type FileMetadata struct {
	Path        string    `json:"path"`
	Hash        string    `json:"hash"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
	Language    string    `json:"language"`
	SymbolCount int       `json:"symbol_count"`
	IndexedAt   time.Time `json:"indexed_at"`
}

// Reference represents a reference to a symbol
type Reference struct {
	SymbolID   string `json:"symbol_id"`
	FilePath   string `json:"file_path"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Kind       string `json:"kind"` // "definition", "reference", "call", etc.
	Context    string `json:"context,omitempty"`
}

// QueryResult caches search results for performance
type QueryResult struct {
	Query     string        `json:"query"`
	Results   []SymbolInfo  `json:"results"`
	Count     int           `json:"count"`
	Duration  time.Duration `json:"duration"`
	CachedAt  time.Time     `json:"cached_at"`
	ExpiresAt time.Time     `json:"expires_at"`
}

// Helper functions for key construction
func SymbolKey(fileHash, symbolID string) []byte {
	return []byte(PrefixSymbol + fileHash + ":" + symbolID)
}

func FileKey(pathHash string) []byte {
	return []byte(PrefixFile + pathHash)
}

func RefKey(symbolHash, fileHash string, line int) []byte {
	return []byte(PrefixRef + symbolHash + ":" + fileHash + ":" + string(rune(line)))
}

func NameKey(name string) []byte {
	return []byte(PrefixName + strings.ToLower(name))
}

func TypeKey(typeName string) []byte {
	return []byte(PrefixType + strings.ToLower(typeName))
}

func TagKey(tag string) []byte {
	return []byte(PrefixTag + strings.ToLower(tag))
}

func QueryKey(queryHash string) []byte {
	return []byte(PrefixQuery + queryHash)
}

func ConfigKey(key string) []byte {
	return []byte(PrefixConfig + key)
}

// JSON marshaling helpers for storage values
func MarshalValue(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func UnmarshalValue(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// StorageError wraps storage-specific errors
type StorageError struct {
	Op  string
	Key string
	Err error
}

func (e *StorageError) Error() string {
	return "storage " + e.Op + " " + e.Key + ": " + e.Err.Error()
}

func (e *StorageError) Unwrap() error {
	return e.Err
}

// Common storage errors
var (
	ErrKeyNotFound   = &StorageError{Op: "get", Err: io.EOF}
	ErrKeyExists     = &StorageError{Op: "set", Err: io.ErrUnexpectedEOF}
	ErrBatchTooLarge = &StorageError{Op: "batch", Err: io.ErrShortBuffer}
	ErrTxnConflict   = &StorageError{Op: "txn", Err: io.ErrClosedPipe}
)