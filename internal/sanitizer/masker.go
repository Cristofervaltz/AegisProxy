package sanitizer

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"github.com/aegisproxy/core/internal/store"
)

// Masker handles finding PII and replacing it with tokens
type Masker struct {
	store  store.StateStore
	rules  []maskRule
	nextID int
	mu     sync.Mutex
}

type maskRule struct {
	Type    string
	Pattern *regexp.Regexp
}

// NewMasker creates a new Masker instance with predefined rules
func NewMasker(s store.StateStore) *Masker {
	return &Masker{
		store: s,
		rules: []maskRule{
			{Type: "EMAIL", Pattern: regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)},
			{Type: "CREDIT_CARD", Pattern: regexp.MustCompile(`\b(?:\d[ -]*?){13,16}\b`)},
			{Type: "PHONE", Pattern: regexp.MustCompile(`\+?[0-9][0-9\- \(\)]{7,14}[0-9]`)},
			{Type: "API_KEY", Pattern: regexp.MustCompile(`sk-[a-zA-Z0-9]{32,}`)},
		},
	}
}

// generateToken creates a unique token like [EMAIL_1]
func (m *Masker) generateToken(tokenType string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	return fmt.Sprintf("[%s_%d]", tokenType, m.nextID)
}

// Mask finds PII in the input text, stores it, and returns the sanitized text
func (m *Masker) Mask(text string) string {
	sanitized := text
	for _, rule := range m.rules {
		matches := rule.Pattern.FindAllString(sanitized, -1)
		// Deduplicate matches to replace same PII with same token
		uniqueMatches := make(map[string]bool)
		for _, match := range matches {
			if !uniqueMatches[match] {
				uniqueMatches[match] = true
				
				token := m.generateToken(rule.Type)
				m.store.Set(token, match) // Store mapping: [EMAIL_1] -> test@test.com
				
				// Replace all occurrences of this specific match with the generated token
				sanitized = strings.ReplaceAll(sanitized, match, token)
			}
		}
	}
	return sanitized
}

// Unmask replaces tokens in the text back with their original PII values
func (m *Masker) Unmask(text string) string {
	// A robust unmask would find tokens and look them up.
	// Since tokens look like [TYPE_ID], we can extract them.
	tokenPattern := regexp.MustCompile(`\[[A-Z_]+_\d+\]`)
	unmasked := text
	tokens := tokenPattern.FindAllString(unmasked, -1)
	
	uniqueTokens := make(map[string]bool)
	for _, token := range tokens {
		if !uniqueTokens[token] {
			uniqueTokens[token] = true
			if original, ok := m.store.Get(token); ok {
				unmasked = strings.ReplaceAll(unmasked, token, original)
			}
		}
	}
	return unmasked
}
