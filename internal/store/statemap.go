package store

import "sync"

// StateStore defines the interface for storing token-to-value mappings
type StateStore interface {
	Set(key string, value string)
	Get(key string) (string, bool)
}

// MemoryStore is an in-memory, thread-safe implementation of StateStore
type MemoryStore struct {
	data sync.Map
}

// NewMemoryStore creates a new MemoryStore
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Set stores the token and its original value
func (s *MemoryStore) Set(key string, value string) {
	s.data.Store(key, value)
}

// Get retrieves the original value for a given token
func (s *MemoryStore) Get(key string) (string, bool) {
	val, ok := s.data.Load(key)
	if !ok {
		return "", false
	}
	return val.(string), true
}
