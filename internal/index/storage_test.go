package index

import (
	"context"
	"testing"
)

func TestStorageInterface(t *testing.T) {
	// Test with in-memory BadgerDB for fast tests
	opts := DefaultBadgerOptions("")
	opts.InMemory = true

	storage, err := NewBadgerStorage(opts)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	t.Run("Basic Operations", func(t *testing.T) {
		testBasicOperations(t, storage, ctx)
	})

	t.Run("Batch Operations", func(t *testing.T) {
		testBatchOperations(t, storage, ctx)
	})

	t.Run("Scanning", func(t *testing.T) {
		testScanning(t, storage, ctx)
	})

	t.Run("Transactions", func(t *testing.T) {
		testTransactions(t, storage, ctx)
	})
}

func testBasicOperations(t *testing.T, storage Storage, ctx context.Context) {
	key := []byte("test-key")
	value := []byte("test-value")

	// Test Has on non-existent key
	exists, err := storage.Has(ctx, key)
	if err != nil {
		t.Errorf("Has failed: %v", err)
	}
	if exists {
		t.Error("Key should not exist")
	}

	// Test Get on non-existent key
	_, err = storage.Get(ctx, key)
	if err != ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}

	// Test Set
	if err := storage.Set(ctx, key, value); err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Test Has on existing key
	exists, err = storage.Has(ctx, key)
	if err != nil {
		t.Errorf("Has failed: %v", err)
	}
	if !exists {
		t.Error("Key should exist")
	}

	// Test Get on existing key
	retrieved, err := storage.Get(ctx, key)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if string(retrieved) != string(value) {
		t.Errorf("Expected %s, got %s", value, retrieved)
	}

	// Test Delete
	if err := storage.Delete(ctx, key); err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Test Has after delete
	exists, err = storage.Has(ctx, key)
	if err != nil {
		t.Errorf("Has failed: %v", err)
	}
	if exists {
		t.Error("Key should not exist after delete")
	}
}

func testBatchOperations(t *testing.T, storage Storage, ctx context.Context) {
	batch := storage.Batch()

	// Add multiple items to batch
	for i := 0; i < 10; i++ {
		key := []byte("batch-key-" + string(rune(i)))
		value := []byte("batch-value-" + string(rune(i)))
		batch.Set(key, value)
	}

	// Test batch size
	if batch.Size() != 10 {
		t.Errorf("Expected batch size 10, got %d", batch.Size())
	}

	// Execute batch
	if err := storage.WriteBatch(ctx, batch); err != nil {
		t.Errorf("WriteBatch failed: %v", err)
	}

	// Verify all items were written
	for i := 0; i < 10; i++ {
		key := []byte("batch-key-" + string(rune(i)))
		expectedValue := []byte("batch-value-" + string(rune(i)))

		value, err := storage.Get(ctx, key)
		if err != nil {
			t.Errorf("Failed to get batch item %d: %v", i, err)
		}
		if string(value) != string(expectedValue) {
			t.Errorf("Batch item %d: expected %s, got %s", i, expectedValue, value)
		}
	}

	// Test batch clear
	batch.Clear()
	if batch.Size() != 0 {
		t.Errorf("Expected batch size 0 after clear, got %d", batch.Size())
	}
}

func testScanning(t *testing.T, storage Storage, ctx context.Context) {
	// Set up test data with common prefix
	prefix := "scan-test:"
	testData := map[string]string{
		prefix + "a": "value-a",
		prefix + "b": "value-b",
		prefix + "c": "value-c",
		"other":      "other-value",
	}

	// Insert test data
	for key, value := range testData {
		if err := storage.Set(ctx, []byte(key), []byte(value)); err != nil {
			t.Errorf("Failed to set test data %s: %v", key, err)
		}
	}

	// Test prefix scan
	iter := storage.Scan(ctx, []byte(prefix), ScanOptions{})
	defer iter.Close()

	found := make(map[string]string)
	for iter.Next() {
		key := string(iter.Key())
		value := string(iter.Value())
		found[key] = value
	}

	if err := iter.Error(); err != nil {
		t.Errorf("Scan error: %v", err)
	}

	// Verify we found the right keys
	expectedCount := 3
	if len(found) != expectedCount {
		t.Errorf("Expected %d items, found %d", expectedCount, len(found))
	}

	for key, expectedValue := range testData {
		if key != "other" { // Skip non-prefix item
			if value, exists := found[key]; !exists {
				t.Errorf("Missing key %s", key)
			} else if value != expectedValue {
				t.Errorf("Key %s: expected %s, got %s", key, expectedValue, value)
			}
		}
	}

	// Test keys-only scan
	iter = storage.Scan(ctx, []byte(prefix), ScanOptions{KeysOnly: true})
	defer iter.Close()

	keyCount := 0
	for iter.Next() {
		keyCount++
		if iter.Value() == nil || len(iter.Value()) > 0 {
			// In keys-only mode, value might be empty or nil
			// This is implementation-dependent
		}
	}

	if keyCount != expectedCount {
		t.Errorf("Keys-only scan: expected %d keys, got %d", expectedCount, keyCount)
	}
}

func testTransactions(t *testing.T, storage Storage, ctx context.Context) {
	key1 := []byte("txn-key-1")
	key2 := []byte("txn-key-2")
	value1 := []byte("txn-value-1")
	value2 := []byte("txn-value-2")

	// Test successful transaction
	err := storage.Transaction(ctx, func(txn Txn) error {
		if err := txn.Set(key1, value1); err != nil {
			return err
		}
		if err := txn.Set(key2, value2); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		t.Errorf("Transaction failed: %v", err)
	}

	// Verify both keys were set
	val1, err := storage.Get(ctx, key1)
	if err != nil || string(val1) != string(value1) {
		t.Errorf("Transaction key1 not set correctly")
	}

	val2, err := storage.Get(ctx, key2)
	if err != nil || string(val2) != string(value2) {
		t.Errorf("Transaction key2 not set correctly")
	}

	// Test transaction with read
	err = storage.Transaction(ctx, func(txn Txn) error {
		val, err := txn.Get(key1)
		if err != nil {
			return err
		}
		if string(val) != string(value1) {
			t.Errorf("Transaction read: expected %s, got %s", value1, val)
		}
		return nil
	})

	if err != nil {
		t.Errorf("Read transaction failed: %v", err)
	}
}