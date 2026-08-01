package sanitizer

import (
	"regexp"
	"sync"
)

// Entity represents a detected PII entity in text
type Entity struct {
	Type  string
	Value string
}

// Extractor is an interface for finding PII in text
type Extractor interface {
	Extract(text string) []Entity
}

type regexRule struct {
	Type    string
	Pattern *regexp.Regexp
}

// RegexExtractor uses regular expressions to find basic PII
type RegexExtractor struct {
	rules []regexRule
	mu    sync.RWMutex
}

// NewRegexExtractor creates a RegexExtractor with default rules
func NewRegexExtractor() *RegexExtractor {
	return &RegexExtractor{
		rules: []regexRule{
			{Type: "EMAIL", Pattern: regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)},
			{Type: "CREDIT_CARD", Pattern: regexp.MustCompile(`\b(?:\d[ -]*?){13,16}\b`)},
			{Type: "PHONE", Pattern: regexp.MustCompile(`\+?[0-9][0-9\- \(\)]{7,14}[0-9]`)},
			{Type: "API_KEY", Pattern: regexp.MustCompile(`sk-[a-zA-Z0-9]{32,}`)},
		},
	}
}

// AddRule allows dynamic addition of new masking rules
func (r *RegexExtractor) AddRule(ruleType, pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules = append(r.rules, regexRule{Type: ruleType, Pattern: compiled})
	return nil
}

// GetRules returns a list of current rule types and patterns
func (r *RegexExtractor) GetRules() []map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []map[string]string
	for _, rule := range r.rules {
		out = append(out, map[string]string{
			"type":    rule.Type,
			"pattern": rule.Pattern.String(),
		})
	}
	return out
}

func (r *RegexExtractor) Extract(text string) []Entity {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entities []Entity
	for _, rule := range r.rules {
		matches := rule.Pattern.FindAllString(text, -1)
		for _, match := range matches {
			entities = append(entities, Entity{Type: rule.Type, Value: match})
		}
	}
	return entities
}
