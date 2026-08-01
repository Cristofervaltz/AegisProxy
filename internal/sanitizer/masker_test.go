package sanitizer

import (
	"github.com/aegisproxy/core/internal/store"
	"strings"
	"testing"
)

func TestMasker(t *testing.T) {
	memStore := store.NewMemoryStore()
	masker := NewMasker(memStore)

	text := "My email is john@doe.com and my card is 1234-5678-1234-5678. API: sk-12345678901234567890123456789012"
	masked := masker.Mask(text)

	if strings.Contains(masked, "john@doe.com") {
		t.Errorf("Expected email to be masked, got: %s", masked)
	}
	if strings.Contains(masked, "1234-5678-1234-5678") {
		t.Errorf("Expected credit card to be masked, got: %s", masked)
	}
	if strings.Contains(masked, "sk-12345678901234567890123456789012") {
		t.Errorf("Expected API key to be masked, got: %s", masked)
	}

	if !strings.Contains(masked, "[EMAIL_") {
		t.Errorf("Expected EMAIL token in masked text, got: %s", masked)
	}
	if !strings.Contains(masked, "[CREDIT_CARD_") {
		t.Errorf("Expected CREDIT_CARD token in masked text, got: %s", masked)
	}
	if !strings.Contains(masked, "[API_KEY_") {
		t.Errorf("Expected API_KEY token in masked text, got: %s", masked)
	}

	// Test Unmask
	unmasked := masker.Unmask(masked)
	if unmasked != text {
		t.Errorf("Expected unmasked text to be equal to original.\nOriginal: %s\nUnmasked: %s", text, unmasked)
	}
}
