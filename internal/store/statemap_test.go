package store

import "testing"

func TestMemoryStore(t *testing.T) {
	s := NewMemoryStore()

	// Test Get on non-existent key
	_, ok := s.Get("unknown")
	if ok {
		t.Error("Expected Get to return false for unknown key")
	}

	// Test Set and Get
	s.Set("[EMAIL_1]", "john@doe.com")
	val, ok := s.Get("[EMAIL_1]")
	if !ok {
		t.Error("Expected Get to return true for existing key")
	}
	if val != "john@doe.com" {
		t.Errorf("Expected 'john@doe.com', got '%s'", val)
	}

	// Test Override
	s.Set("[EMAIL_1]", "jane@doe.com")
	val, _ = s.Get("[EMAIL_1]")
	if val != "jane@doe.com" {
		t.Errorf("Expected 'jane@doe.com', got '%s'", val)
	}
}
