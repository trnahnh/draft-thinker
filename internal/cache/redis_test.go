package cache

import (
	"context"
	"testing"
)

type mockKVStore struct {
	data map[string][]byte
}

func newMockKVStore() *mockKVStore {
	return &mockKVStore{data: make(map[string][]byte)}
}

func (m *mockKVStore) Set(_ context.Context, key string, data []byte) error {
	m.data[key] = make([]byte, len(data))
	copy(m.data[key], data)
	return nil
}

func (m *mockKVStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	val, ok := m.data[key]
	if !ok {
		return nil, false, nil
	}
	return val, true, nil
}

func (m *mockKVStore) Del(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockKVStore) Close() error {
	return nil
}

func TestMockKVStore_SetGetDel(t *testing.T) {
	kv := newMockKVStore()
	ctx := context.Background()

	err := kv.Set(ctx, "key1", []byte("value1"))
	if err != nil {
		t.Fatalf("set: %v", err)
	}

	val, ok, err := kv.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok {
		t.Fatal("expected key to exist")
	}
	if string(val) != "value1" {
		t.Errorf("value: got %q, want %q", val, "value1")
	}

	_, ok, err = kv.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("get missing: %v", err)
	}
	if ok {
		t.Error("expected missing key to return false")
	}

	err = kv.Del(ctx, "key1")
	if err != nil {
		t.Fatalf("del: %v", err)
	}

	_, ok, err = kv.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("get after del: %v", err)
	}
	if ok {
		t.Error("expected deleted key to return false")
	}
}
