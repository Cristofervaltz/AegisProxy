package sanitizer

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"log/slog"

	"github.com/aegisproxy/core/internal/metrics"
	"github.com/aegisproxy/core/internal/store"
)

// Masker handles finding PII and replacing it with tokens
type Masker struct {
	store      store.StateStore
	extractors []Extractor
	nextID     int
	mu         sync.Mutex
}

// NewMasker creates a new Masker instance with given extractors
func NewMasker(s store.StateStore, extractors ...Extractor) *Masker {
	if len(extractors) == 0 {
		extractors = []Extractor{NewRegexExtractor()}
	}
	return &Masker{
		store:      s,
		extractors: extractors,
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
	
	var allEntities []Entity
	for _, ext := range m.extractors {
		allEntities = append(allEntities, ext.Extract(sanitized)...)
	}

	uniqueMatches := make(map[string]bool)
	for _, entity := range allEntities {
		match := entity.Value
		if !uniqueMatches[match] {
			uniqueMatches[match] = true
			
			token := m.generateToken(entity.Type)
			m.store.Set(token, match)
			
			// Increment metric
			metrics.TokensMasked.WithLabelValues(entity.Type).Inc()
			slog.Debug("Masked entity", "type", entity.Type, "token", token)

			sanitized = strings.ReplaceAll(sanitized, match, token)
		}
	}

	return sanitized
}

// Unmask replaces tokens in the text back with their original PII values
func (m *Masker) Unmask(text string) string {
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
