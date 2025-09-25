package index

import (
	"context"
	"testing"
	"time"
)

func TestStore(t *testing.T) {
	// Create in-memory storage for testing
	opts := DefaultBadgerOptions("")
	opts.InMemory = true

	storage, err := NewBadgerStorage(opts)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	store := NewStore(storage, DefaultStoreConfig())
	ctx := context.Background()

	t.Run("Symbol Operations", func(t *testing.T) {
		testSymbolOperations(t, store, ctx)
	})

	t.Run("File Metadata Operations", func(t *testing.T) {
		testFileMetadataOperations(t, store, ctx)
	})

	t.Run("Search Operations", func(t *testing.T) {
		testSearchOperations(t, store, ctx)
	})
}

func testSymbolOperations(t *testing.T, store *Store, ctx context.Context) {
	symbol := SymbolInfo{
		ID:        "test-symbol-1",
		Name:      "TestFunction",
		Type:      "function",
		Kind:      "function",
		FilePath:  "/test/file.go",
		StartLine: 10,
		EndLine:   20,
		StartCol:  1,
		EndCol:    10,
		Signature: "func TestFunction() error",
		Tags:      []string{"exported", "test"},
		Properties: map[string]string{
			"visibility": "public",
		},
		LastUpdated: time.Now(),
	}

	// Test storing symbol
	if err := store.StoreSymbol(ctx, symbol); err != nil {
		t.Errorf("Failed to store symbol: %v", err)
	}

	// Test retrieving symbol
	retrieved, err := store.GetSymbol(ctx, symbol.FilePath, symbol.ID)
	if err != nil {
		t.Errorf("Failed to get symbol: %v", err)
	}

	// Verify symbol data
	if retrieved.ID != symbol.ID {
		t.Errorf("Symbol ID mismatch: expected %s, got %s", symbol.ID, retrieved.ID)
	}
	if retrieved.Name != symbol.Name {
		t.Errorf("Symbol name mismatch: expected %s, got %s", symbol.Name, retrieved.Name)
	}
	if retrieved.Type != symbol.Type {
		t.Errorf("Symbol type mismatch: expected %s, got %s", symbol.Type, retrieved.Type)
	}

	// Test updating symbol
	symbol.Name = "UpdatedFunction"
	if err := store.StoreSymbol(ctx, symbol); err != nil {
		t.Errorf("Failed to update symbol: %v", err)
	}

	// Verify update
	updated, err := store.GetSymbol(ctx, symbol.FilePath, symbol.ID)
	if err != nil {
		t.Errorf("Failed to get updated symbol: %v", err)
	}
	if updated.Name != "UpdatedFunction" {
		t.Errorf("Symbol name not updated: expected UpdatedFunction, got %s", updated.Name)
	}

	// Test deleting symbol
	if err := store.DeleteSymbol(ctx, symbol.FilePath, symbol.ID); err != nil {
		t.Errorf("Failed to delete symbol: %v", err)
	}

	// Verify deletion
	_, err = store.GetSymbol(ctx, symbol.FilePath, symbol.ID)
	if err == nil {
		t.Error("Symbol should not exist after deletion")
	}
}

func testFileMetadataOperations(t *testing.T, store *Store, ctx context.Context) {
	metadata := FileMetadata{
		Path:        "/test/file.go",
		Hash:        "abcdef123456",
		Size:        1024,
		ModTime:     time.Now().Truncate(time.Second), // Truncate for comparison
		Language:    "Go",
		SymbolCount: 5,
		IndexedAt:   time.Now().Truncate(time.Second),
	}

	// Test storing file metadata
	if err := store.StoreFileMetadata(ctx, metadata); err != nil {
		t.Errorf("Failed to store file metadata: %v", err)
	}

	// Test retrieving file metadata
	retrieved, err := store.GetFileMetadata(ctx, metadata.Path)
	if err != nil {
		t.Errorf("Failed to get file metadata: %v", err)
	}

	// Verify metadata
	if retrieved.Path != metadata.Path {
		t.Errorf("File path mismatch: expected %s, got %s", metadata.Path, retrieved.Path)
	}
	if retrieved.Hash != metadata.Hash {
		t.Errorf("File hash mismatch: expected %s, got %s", metadata.Hash, retrieved.Hash)
	}
	if retrieved.Language != metadata.Language {
		t.Errorf("File language mismatch: expected %s, got %s", metadata.Language, retrieved.Language)
	}

	// Test getting all files
	allFiles, err := store.GetAllFiles(ctx)
	if err != nil {
		t.Errorf("Failed to get all files: %v", err)
	}

	found := false
	for _, file := range allFiles {
		if file.Path == metadata.Path {
			found = true
			break
		}
	}
	if !found {
		t.Error("File not found in all files list")
	}
}

func testSearchOperations(t *testing.T, store *Store, ctx context.Context) {
	// Create test symbols with different names, types, and tags
	symbols := []SymbolInfo{
		{
			ID:       "func1",
			Name:     "TestFunction",
			Type:     "function",
			Kind:     "function",
			FilePath: "/test/file1.go",
			Tags:     []string{"exported", "test"},
		},
		{
			ID:       "func2",
			Name:     "HelperFunction",
			Type:     "function",
			Kind:     "function",
			FilePath: "/test/file2.go",
			Tags:     []string{"private", "helper"},
		},
		{
			ID:       "struct1",
			Name:     "TestStruct",
			Type:     "struct",
			Kind:     "type",
			FilePath: "/test/file3.go",
			Tags:     []string{"exported"},
		},
	}

	// Store test symbols
	for _, symbol := range symbols {
		if err := store.StoreSymbol(ctx, symbol); err != nil {
			t.Errorf("Failed to store symbol %s: %v", symbol.ID, err)
		}
	}

	// Test search by name
	query := SearchQuery{
		Type: SearchByName,
		Term: "TestFunction",
	}

	result, err := store.SearchSymbols(ctx, query)
	if err != nil {
		t.Errorf("Search by name failed: %v", err)
	}

	if result.Count != 1 {
		t.Errorf("Expected 1 result for name search, got %d", result.Count)
	}

	if len(result.Symbols) > 0 && result.Symbols[0].Name != "TestFunction" {
		t.Errorf("Wrong symbol returned for name search: expected TestFunction, got %s", result.Symbols[0].Name)
	}

	// Test search by type
	query = SearchQuery{
		Type: SearchByType,
		Term: "function",
	}

	result, err = store.SearchSymbols(ctx, query)
	if err != nil {
		t.Errorf("Search by type failed: %v", err)
	}

	if result.Count != 2 {
		t.Errorf("Expected 2 results for type search, got %d", result.Count)
	}

	// Test search by tag
	query = SearchQuery{
		Type: SearchByTag,
		Term: "exported",
	}

	result, err = store.SearchSymbols(ctx, query)
	if err != nil {
		t.Errorf("Search by tag failed: %v", err)
	}

	if result.Count != 2 {
		t.Errorf("Expected 2 results for tag search, got %d", result.Count)
	}

	// Test search with filters
	query = SearchQuery{
		Type: SearchByType,
		Term: "function",
		Filters: []Filter{
			{
				Field:    "name",
				Operator: "contains",
				Value:    "Test",
			},
		},
	}

	result, err = store.SearchSymbols(ctx, query)
	if err != nil {
		t.Errorf("Search with filters failed: %v", err)
	}

	if result.Count != 1 {
		t.Errorf("Expected 1 result for filtered search, got %d", result.Count)
	}

	// Test search with limit
	query = SearchQuery{
		Type:  SearchByType,
		Term:  "function",
		Limit: 1,
	}

	result, err = store.SearchSymbols(ctx, query)
	if err != nil {
		t.Errorf("Search with limit failed: %v", err)
	}

	if result.Count > 1 {
		t.Errorf("Expected at most 1 result for limited search, got %d", result.Count)
	}
}

func TestStoreConfiguration(t *testing.T) {
	config := DefaultStoreConfig()

	// Test default values
	if config.QueryCacheTTL != 30*time.Minute {
		t.Errorf("Expected default cache TTL to be 30 minutes, got %v", config.QueryCacheTTL)
	}

	if config.MaxCachedQueries != 1000 {
		t.Errorf("Expected default max cached queries to be 1000, got %d", config.MaxCachedQueries)
	}

	if !config.CacheEnabled {
		t.Error("Expected cache to be enabled by default")
	}

	if config.BatchSize != 1000 {
		t.Errorf("Expected default batch size to be 1000, got %d", config.BatchSize)
	}
}

func TestKeyGeneration(t *testing.T) {
	// Test key generation functions
	fileHash := "abcdef123456"
	symbolID := "func1"

	symbolKey := SymbolKey(fileHash, symbolID)
	expectedSymbolKey := []byte("sym:abcdef123456:func1")
	if string(symbolKey) != string(expectedSymbolKey) {
		t.Errorf("SymbolKey mismatch: expected %s, got %s", expectedSymbolKey, symbolKey)
	}

	fileKey := FileKey("pathHash123")
	expectedFileKey := []byte("file:pathHash123")
	if string(fileKey) != string(expectedFileKey) {
		t.Errorf("FileKey mismatch: expected %s, got %s", expectedFileKey, fileKey)
	}

	nameKey := NameKey("TestFunction")
	expectedNameKey := []byte("name:testfunction")  // NameKey converts to lowercase
	if string(nameKey) != string(expectedNameKey) {
		t.Errorf("NameKey mismatch: expected %s, got %s", expectedNameKey, nameKey)
	}
}