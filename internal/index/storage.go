package index

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Storage defines the unified interface for all storage operations in codegrep.
// It provides key-value storage with prefix scanning, transactions, and batch operations
// optimized for storing code symbols, metadata, and query results.
type Storage interface {
	Get(ctx context.Context, key []byte) ([]byte, error)
	Set(ctx context.Context, key, value []byte) error
	Delete(ctx context.Context, key []byte) error
	Has(ctx context.Context, key []byte) (bool, error)

	Batch() Batch
	WriteBatch(ctx context.Context, batch Batch) error

	Scan(ctx context.Context, prefix []byte, opts ScanOptions) Iterator

	Transaction(ctx context.Context, fn func(Txn) error) error

	Backup(ctx context.Context, w io.Writer) error
	Restore(ctx context.Context, r io.Reader) error
	Close() error

	Stats() StorageStats
	Size() (int64, error)

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
	Reverse bool

	Limit int

	KeysOnly bool

	StartAfter []byte
}

// StorageStats provides insights into storage performance and usage
type StorageStats struct {
	TotalSize     int64 `json:"total_size"`
	KeyCount      int64 `json:"key_count"`
	IndexSize     int64 `json:"index_size"`

	ReadCount     int64 `json:"read_count"`
	WriteCount    int64 `json:"write_count"`
	ScanCount     int64 `json:"scan_count"`

	CacheHits     int64 `json:"cache_hits"`
	CacheMisses   int64 `json:"cache_misses"`

	AvgReadTime   int64 `json:"avg_read_time"`
	AvgWriteTime  int64 `json:"avg_write_time"`
	AvgScanTime   int64 `json:"avg_scan_time"`

	LastUpdated   time.Time `json:"last_updated"`
}

const (
	PrefixSymbol     = "sym:"  // sym:{file_hash}:{symbol_id} -> SymbolInfo
	PrefixFile       = "file:" // file:{file_path_hash} -> FileMetadata
	PrefixRef        = "ref:"  // ref:{symbol_hash}:{file_hash}:{line} -> Reference

	PrefixName       = "name:" // name:{symbol_name} -> []symbol_id
	PrefixType       = "type:" // type:{type_name} -> []symbol_id
	PrefixTag        = "tag:"  // tag:{tag_name} -> []symbol_id

	PrefixQuery      = "query:" // query:{query_hash} -> QueryResult
	PrefixStats      = "stats:" // stats:{timestamp} -> SearchStats

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

// SymbolIndex represents a collection of symbols for reference finding
type SymbolIndex []SymbolInfo

// QueryResult caches search results for performance
type QueryResult struct {
	Query     string        `json:"query"`
	Results   []SymbolInfo  `json:"results"`
	Count     int           `json:"count"`
	Duration  time.Duration `json:"duration"`
	CachedAt  time.Time     `json:"cached_at"`
	ExpiresAt time.Time     `json:"expires_at"`
}

func SymbolKey(fileHash, symbolID string) []byte {
	return []byte(PrefixSymbol + fileHash + ":" + symbolID)
}

func FileKey(pathHash string) []byte {
	return []byte(PrefixFile + pathHash)
}

func RefKey(symbolHash, fileHash string, line int) []byte {
	return []byte(fmt.Sprintf("%s%s:%s:%d", PrefixRef, symbolHash, fileHash, line))
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

var (
	ErrKeyNotFound   = &StorageError{Op: "get", Err: io.EOF}
	ErrKeyExists     = &StorageError{Op: "set", Err: io.ErrUnexpectedEOF}
	ErrBatchTooLarge = &StorageError{Op: "batch", Err: io.ErrShortBuffer}
	ErrTxnConflict   = &StorageError{Op: "txn", Err: io.ErrClosedPipe}
)