package index

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Store provides high-level operations for symbol storage and retrieval
// It wraps the low-level Storage interface with semantic operations
type Store struct {
	storage Storage
	config  StoreConfig
}

// StoreConfig configures store behavior
type StoreConfig struct {
	// Cache TTL for query results
	QueryCacheTTL time.Duration

	// Maximum number of cached queries
	MaxCachedQueries int

	// Enable/disable query result caching
	CacheEnabled bool

	// Batch size for bulk operations
	BatchSize int

	// Enable statistics collection
	CollectStats bool
}

// DefaultStoreConfig returns sensible defaults for the store
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		QueryCacheTTL:    30 * time.Minute,
		MaxCachedQueries: 1000,
		CacheEnabled:     true,
		BatchSize:        1000,
		CollectStats:     true,
	}
}

// NewStore creates a new store instance
func NewStore(storage Storage, config StoreConfig) *Store {
	return &Store{
		storage: storage,
		config:  config,
	}
}

// Symbol operations

// StoreSymbol stores a symbol with all its metadata and creates necessary indices
func (s *Store) StoreSymbol(ctx context.Context, symbol SymbolInfo) error {
	// Generate keys
	fileHash := s.hashString(symbol.FilePath)
	symbolKey := SymbolKey(fileHash, symbol.ID)

	// Store the symbol data
	symbolData, err := MarshalValue(symbol)
	if err != nil {
		return fmt.Errorf("failed to marshal symbol: %w", err)
	}

	batch := s.storage.Batch()

	// Store main symbol record
	batch.Set(symbolKey, symbolData)

	// Create name index
	nameKey := NameKey(strings.ToLower(symbol.Name))
	if err := s.addToIndex(ctx, batch, nameKey, symbol.ID); err != nil {
		return fmt.Errorf("failed to update name index: %w", err)
	}

	// Create type index
	if symbol.Type != "" {
		typeKey := TypeKey(strings.ToLower(symbol.Type))
		if err := s.addToIndex(ctx, batch, typeKey, symbol.ID); err != nil {
			return fmt.Errorf("failed to update type index: %w", err)
		}
	}

	// Create tag indices
	for _, tag := range symbol.Tags {
		tagKey := TagKey(strings.ToLower(tag))
		if err := s.addToIndex(ctx, batch, tagKey, symbol.ID); err != nil {
			return fmt.Errorf("failed to update tag index: %w", err)
		}
	}

	return s.storage.WriteBatch(ctx, batch)
}

// GetSymbol retrieves a symbol by file path and symbol ID
func (s *Store) GetSymbol(ctx context.Context, filePath, symbolID string) (*SymbolInfo, error) {
	fileHash := s.hashString(filePath)
	symbolKey := SymbolKey(fileHash, symbolID)

	data, err := s.storage.Get(ctx, symbolKey)
	if err != nil {
		return nil, err
	}

	var symbol SymbolInfo
	if err := UnmarshalValue(data, &symbol); err != nil {
		return nil, fmt.Errorf("failed to unmarshal symbol: %w", err)
	}

	return &symbol, nil
}

// DeleteSymbol removes a symbol and updates all indices
func (s *Store) DeleteSymbol(ctx context.Context, filePath, symbolID string) error {
	// First get the symbol to know what indices to update
	symbol, err := s.GetSymbol(ctx, filePath, symbolID)
	if err != nil {
		return err
	}

	fileHash := s.hashString(filePath)
	symbolKey := SymbolKey(fileHash, symbolID)

	batch := s.storage.Batch()

	// Delete main symbol record
	batch.Delete(symbolKey)

	// Remove from name index
	nameKey := NameKey(strings.ToLower(symbol.Name))
	if err := s.removeFromIndex(ctx, batch, nameKey, symbolID); err != nil {
		return fmt.Errorf("failed to update name index: %w", err)
	}

	// Remove from type index
	if symbol.Type != "" {
		typeKey := TypeKey(strings.ToLower(symbol.Type))
		if err := s.removeFromIndex(ctx, batch, typeKey, symbolID); err != nil {
			return fmt.Errorf("failed to update type index: %w", err)
		}
	}

	// Remove from tag indices
	for _, tag := range symbol.Tags {
		tagKey := TagKey(strings.ToLower(tag))
		if err := s.removeFromIndex(ctx, batch, tagKey, symbolID); err != nil {
			return fmt.Errorf("failed to update tag index: %w", err)
		}
	}

	return s.storage.WriteBatch(ctx, batch)
}

// File metadata operations

// StoreFileMetadata stores metadata about an indexed file
func (s *Store) StoreFileMetadata(ctx context.Context, metadata FileMetadata) error {
	pathHash := s.hashString(metadata.Path)
	fileKey := FileKey(pathHash)

	data, err := MarshalValue(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal file metadata: %w", err)
	}

	return s.storage.Set(ctx, fileKey, data)
}

// GetFileMetadata retrieves file metadata by path
func (s *Store) GetFileMetadata(ctx context.Context, filePath string) (*FileMetadata, error) {
	pathHash := s.hashString(filePath)
	fileKey := FileKey(pathHash)

	data, err := s.storage.Get(ctx, fileKey)
	if err != nil {
		return nil, err
	}

	var metadata FileMetadata
	if err := UnmarshalValue(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal file metadata: %w", err)
	}

	return &metadata, nil
}

// GetAllFiles returns metadata for all indexed files
func (s *Store) GetAllFiles(ctx context.Context) ([]FileMetadata, error) {
	var files []FileMetadata

	iter := s.storage.Scan(ctx, []byte(PrefixFile), ScanOptions{})
	defer iter.Close()

	for iter.Next() {
		var metadata FileMetadata
		if err := UnmarshalValue(iter.Value(), &metadata); err != nil {
			continue // Skip corrupted entries
		}
		files = append(files, metadata)
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}

	return files, nil
}

// DeleteFile removes file metadata and all associated symbols
func (s *Store) DeleteFile(ctx context.Context, filePath string) error {
	// First get all symbols in this file
	symbols, err := s.GetSymbolsInFile(ctx, filePath)
	if err != nil {
		return err
	}

	batch := s.storage.Batch()

	// Delete file metadata
	pathHash := s.hashString(filePath)
	fileKey := FileKey(pathHash)
	batch.Delete(fileKey)

	// Delete all symbols in the file
	for _, symbol := range symbols {
		symbolKey := SymbolKey(pathHash, symbol.ID)
		batch.Delete(symbolKey)

		// Remove from indices
		nameKey := NameKey(strings.ToLower(symbol.Name))
		s.removeFromIndexBatch(batch, nameKey, symbol.ID)

		if symbol.Type != "" {
			typeKey := TypeKey(strings.ToLower(symbol.Type))
			s.removeFromIndexBatch(batch, typeKey, symbol.ID)
		}

		for _, tag := range symbol.Tags {
			tagKey := TagKey(strings.ToLower(tag))
			s.removeFromIndexBatch(batch, tagKey, symbol.ID)
		}
	}

	return s.storage.WriteBatch(ctx, batch)
}

// Query operations

// SearchSymbols searches for symbols by name, type, or tag
func (s *Store) SearchSymbols(ctx context.Context, query SearchQuery) (*SearchResult, error) {
	start := time.Now()

	// Check cache first if enabled
	if s.config.CacheEnabled {
		if result := s.getCachedResult(ctx, query); result != nil {
			return result, nil
		}
	}

	var symbolIDs []string
	var err error

	switch query.Type {
	case SearchByName:
		symbolIDs, err = s.searchByName(ctx, query.Term)
	case SearchByType:
		symbolIDs, err = s.searchByType(ctx, query.Term)
	case SearchByTag:
		symbolIDs, err = s.searchByTag(ctx, query.Term)
	case SearchByPattern:
		symbolIDs, err = s.searchByPattern(ctx, query.Term)
	default:
		return nil, fmt.Errorf("unsupported search type: %v", query.Type)
	}

	if err != nil {
		return nil, err
	}

	// Retrieve full symbol information
	symbols, err := s.getSymbolsByIDs(ctx, symbolIDs, query.Limit)
	if err != nil {
		return nil, err
	}

	// Apply filters
	symbols = s.applyFilters(symbols, query.Filters)

	// Sort results
	s.sortResults(symbols, query.SortBy)

	// Apply limit and offset
	symbols = s.applyPagination(symbols, query.Limit, query.Offset)

	result := &SearchResult{
		Query:     query,
		Symbols:   symbols,
		Count:     len(symbols),
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}

	// Cache result if enabled
	if s.config.CacheEnabled {
		s.cacheResult(ctx, query, result)
	}

	return result, nil
}

// SearchQuery represents a search request
type SearchQuery struct {
	Type    SearchType    `json:"type"`
	Term    string        `json:"term"`
	Filters []Filter      `json:"filters,omitempty"`
	SortBy  SortOption    `json:"sort_by,omitempty"`
	Limit   int           `json:"limit,omitempty"`
	Offset  int           `json:"offset,omitempty"`
}

// SearchType defines the type of search
type SearchType int

const (
	SearchByName SearchType = iota
	SearchByType
	SearchByTag
	SearchByPattern
)

// Filter represents a search filter
type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// SortOption defines how to sort results
type SortOption struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc"`
}

// SearchResult contains search results with metadata
type SearchResult struct {
	Query     SearchQuery   `json:"query"`
	Symbols   []SymbolInfo  `json:"symbols"`
	Count     int           `json:"count"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// GetSymbolsInFile returns all symbols defined in a specific file
func (s *Store) GetSymbolsInFile(ctx context.Context, filePath string) ([]SymbolInfo, error) {
	fileHash := s.hashString(filePath)
	prefix := []byte(PrefixSymbol + fileHash + ":")

	var symbols []SymbolInfo
	iter := s.storage.Scan(ctx, prefix, ScanOptions{})
	defer iter.Close()

	for iter.Next() {
		var symbol SymbolInfo
		if err := UnmarshalValue(iter.Value(), &symbol); err != nil {
			continue // Skip corrupted entries
		}
		symbols = append(symbols, symbol)
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}

	return symbols, nil
}

// Cache operations

func (s *Store) getCachedResult(ctx context.Context, query SearchQuery) *SearchResult {
	queryHash := s.hashQuery(query)
	queryKey := QueryKey(queryHash)

	data, err := s.storage.Get(ctx, queryKey)
	if err != nil {
		return nil
	}

	var cached QueryResult
	if err := UnmarshalValue(data, &cached); err != nil {
		return nil
	}

	// Check if cache entry has expired
	if time.Now().After(cached.ExpiresAt) {
		// Delete expired entry
		s.storage.Delete(ctx, queryKey)
		return nil
	}

	return &SearchResult{
		Query:     query,
		Symbols:   cached.Results,
		Count:     cached.Count,
		Duration:  cached.Duration,
		Timestamp: cached.CachedAt,
	}
}

func (s *Store) cacheResult(ctx context.Context, query SearchQuery, result *SearchResult) {
	queryHash := s.hashQuery(query)
	queryKey := QueryKey(queryHash)

	cached := QueryResult{
		Query:     s.queryToString(query),
		Results:   result.Symbols,
		Count:     result.Count,
		Duration:  result.Duration,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(s.config.QueryCacheTTL),
	}

	data, err := MarshalValue(cached)
	if err != nil {
		return // Silently fail caching
	}

	s.storage.Set(ctx, queryKey, data)
}

// Index management helpers

func (s *Store) addToIndex(ctx context.Context, batch Batch, indexKey []byte, symbolID string) error {
	// Get existing index
	data, err := s.storage.Get(ctx, indexKey)
	var symbolIDs []string
	if err == nil {
		if err := UnmarshalValue(data, &symbolIDs); err != nil {
			return err
		}
	}

	// Add new symbol ID if not already present
	found := false
	for _, id := range symbolIDs {
		if id == symbolID {
			found = true
			break
		}
	}

	if !found {
		symbolIDs = append(symbolIDs, symbolID)
		newData, err := MarshalValue(symbolIDs)
		if err != nil {
			return err
		}
		batch.Set(indexKey, newData)
	}

	return nil
}

func (s *Store) removeFromIndex(ctx context.Context, batch Batch, indexKey []byte, symbolID string) error {
	data, err := s.storage.Get(ctx, indexKey)
	if err != nil {
		return nil // Index doesn't exist, nothing to remove
	}

	var symbolIDs []string
	if err := UnmarshalValue(data, &symbolIDs); err != nil {
		return err
	}

	// Remove symbol ID
	newSymbolIDs := make([]string, 0, len(symbolIDs))
	for _, id := range symbolIDs {
		if id != symbolID {
			newSymbolIDs = append(newSymbolIDs, id)
		}
	}

	if len(newSymbolIDs) == 0 {
		// Remove empty index
		batch.Delete(indexKey)
	} else {
		newData, err := MarshalValue(newSymbolIDs)
		if err != nil {
			return err
		}
		batch.Set(indexKey, newData)
	}

	return nil
}

func (s *Store) removeFromIndexBatch(batch Batch, indexKey []byte, symbolID string) {
	// Simplified version for batch operations - doesn't check existing data
	// This should be used when we know the symbol exists in the index
	batch.Delete(indexKey) // Will be reconstructed if needed
}

// Search implementation

func (s *Store) searchByName(ctx context.Context, name string) ([]string, error) {
	nameKey := NameKey(strings.ToLower(name))
	data, err := s.storage.Get(ctx, nameKey)
	if err != nil {
		return nil, err
	}

	var symbolIDs []string
	if err := UnmarshalValue(data, &symbolIDs); err != nil {
		return nil, err
	}

	return symbolIDs, nil
}

func (s *Store) searchByType(ctx context.Context, typeName string) ([]string, error) {
	typeKey := TypeKey(strings.ToLower(typeName))
	data, err := s.storage.Get(ctx, typeKey)
	if err != nil {
		return nil, err
	}

	var symbolIDs []string
	if err := UnmarshalValue(data, &symbolIDs); err != nil {
		return nil, err
	}

	return symbolIDs, nil
}

func (s *Store) searchByTag(ctx context.Context, tag string) ([]string, error) {
	tagKey := TagKey(strings.ToLower(tag))
	data, err := s.storage.Get(ctx, tagKey)
	if err != nil {
		return nil, err
	}

	var symbolIDs []string
	if err := UnmarshalValue(data, &symbolIDs); err != nil {
		return nil, err
	}

	return symbolIDs, nil
}

func (s *Store) searchByPattern(ctx context.Context, pattern string) ([]string, error) {
	// For pattern search, we need to scan all name indices
	var allSymbolIDs []string

	iter := s.storage.Scan(ctx, []byte(PrefixName), ScanOptions{})
	defer iter.Close()

	for iter.Next() {
		key := string(iter.Key())
		name := strings.TrimPrefix(key, PrefixName)

		// Simple pattern matching - can be enhanced with regex
		if strings.Contains(name, strings.ToLower(pattern)) {
			var symbolIDs []string
			if err := UnmarshalValue(iter.Value(), &symbolIDs); err == nil {
				allSymbolIDs = append(allSymbolIDs, symbolIDs...)
			}
		}
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}

	return allSymbolIDs, nil
}

func (s *Store) getSymbolsByIDs(ctx context.Context, symbolIDs []string, limit int) ([]SymbolInfo, error) {
	if limit > 0 && len(symbolIDs) > limit {
		symbolIDs = symbolIDs[:limit]
	}

	var symbols []SymbolInfo

	// Use batch reading for better performance
	for _, id := range symbolIDs {
		// For this simplified version, we'll need to scan to find the symbol
		// In a real implementation, we'd store a reverse mapping from ID to key
		if len(symbols) >= limit && limit > 0 {
			break
		}

		// This is a simplified approach - in practice you'd maintain a better mapping
		iter := s.storage.Scan(ctx, []byte(PrefixSymbol), ScanOptions{})
		for iter.Next() {
			var symbol SymbolInfo
			if err := UnmarshalValue(iter.Value(), &symbol); err == nil {
				if symbol.ID == id {
					symbols = append(symbols, symbol)
					break
				}
			}
		}
		iter.Close()
	}

	return symbols, nil
}

func (s *Store) applyFilters(symbols []SymbolInfo, filters []Filter) []SymbolInfo {
	if len(filters) == 0 {
		return symbols
	}

	filtered := make([]SymbolInfo, 0, len(symbols))
	for _, symbol := range symbols {
		if s.matchesFilters(symbol, filters) {
			filtered = append(filtered, symbol)
		}
	}

	return filtered
}

func (s *Store) matchesFilters(symbol SymbolInfo, filters []Filter) bool {
	for _, filter := range filters {
		if !s.matchesFilter(symbol, filter) {
			return false
		}
	}
	return true
}

func (s *Store) matchesFilter(symbol SymbolInfo, filter Filter) bool {
	var fieldValue interface{}

	switch filter.Field {
	case "name":
		fieldValue = symbol.Name
	case "type":
		fieldValue = symbol.Type
	case "kind":
		fieldValue = symbol.Kind
	case "file_path":
		fieldValue = symbol.FilePath
	default:
		return true // Unknown field, don't filter
	}

	switch filter.Operator {
	case "equals":
		return fieldValue == filter.Value
	case "contains":
		if str, ok := fieldValue.(string); ok {
			if filterStr, ok := filter.Value.(string); ok {
				return strings.Contains(strings.ToLower(str), strings.ToLower(filterStr))
			}
		}
	case "startswith":
		if str, ok := fieldValue.(string); ok {
			if filterStr, ok := filter.Value.(string); ok {
				return strings.HasPrefix(strings.ToLower(str), strings.ToLower(filterStr))
			}
		}
	}

	return false
}

func (s *Store) sortResults(symbols []SymbolInfo, sortBy SortOption) {
	if sortBy.Field == "" {
		return
	}

	sort.Slice(symbols, func(i, j int) bool {
		var less bool
		switch sortBy.Field {
		case "name":
			less = symbols[i].Name < symbols[j].Name
		case "type":
			less = symbols[i].Type < symbols[j].Type
		case "file_path":
			less = symbols[i].FilePath < symbols[j].FilePath
		case "line":
			less = symbols[i].StartLine < symbols[j].StartLine
		default:
			return false
		}

		if sortBy.Desc {
			return !less
		}
		return less
	})
}

func (s *Store) applyPagination(symbols []SymbolInfo, limit, offset int) []SymbolInfo {
	if offset >= len(symbols) {
		return []SymbolInfo{}
	}

	end := len(symbols)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	return symbols[offset:end]
}

// Utility functions

func (s *Store) hashString(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

func (s *Store) hashQuery(query SearchQuery) string {
	queryStr := s.queryToString(query)
	return s.hashString(queryStr)
}

func (s *Store) queryToString(query SearchQuery) string {
	return fmt.Sprintf("%d:%s:%v:%v:%d:%d", query.Type, query.Term, query.Filters, query.SortBy, query.Limit, query.Offset)
}

// Storage returns the underlying storage interface
func (s *Store) Storage() Storage {
	return s.storage
}

// Config returns the store configuration
func (s *Store) Config() StoreConfig {
	return s.config
}

// Close closes the underlying storage
func (s *Store) Close() error {
	return s.storage.Close()
}