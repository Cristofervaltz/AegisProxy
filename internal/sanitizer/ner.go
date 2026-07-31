package sanitizer

import "regexp"

// Entity represents a detected PII entity in text
type Entity struct {
	Type  string
	Value string
}

// Extractor is an interface for finding PII in text
type Extractor interface {
	Extract(text string) []Entity
}

// RegexExtractor uses regular expressions to find basic PII
type RegexExtractor struct {
	rules []struct {
		Type    string
		Pattern *regexp.Regexp
	}
}

// NewRegexExtractor creates a RegexExtractor with default rules
func NewRegexExtractor() *RegexExtractor {
	return &RegexExtractor{
		rules: []struct {
			Type    string
			Pattern *regexp.Regexp
		}{
			{Type: "EMAIL", Pattern: regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)},
			{Type: "CREDIT_CARD", Pattern: regexp.MustCompile(`\b(?:\d[ -]*?){13,16}\b`)},
			{Type: "PHONE", Pattern: regexp.MustCompile(`\+?[0-9][0-9\- \(\)]{7,14}[0-9]`)},
			{Type: "API_KEY", Pattern: regexp.MustCompile(`sk-[a-zA-Z0-9]{32,}`)},
		},
	}
}

func (r *RegexExtractor) Extract(text string) []Entity {
	var entities []Entity
	for _, rule := range r.rules {
		matches := rule.Pattern.FindAllString(text, -1)
		for _, match := range matches {
			entities = append(entities, Entity{Type: rule.Type, Value: match})
		}
	}
	return entities
}

// ONNXExtractor is a placeholder for a local Small Language Model (SLM) NER
// that uses ONNX Runtime for zero-latency unstructured text parsing.
type ONNXExtractor struct {
	modelPath string
	// placeholder for ONNX session/model
}

func NewONNXExtractor(modelPath string) *ONNXExtractor {
	return &ONNXExtractor{
		modelPath: modelPath,
	}
}

func (o *ONNXExtractor) Extract(text string) []Entity {
	// TODO: Implement actual ONNX inference here.
	// For MVP, we simulate returning nothing or mock data
	// Example: tokenize text -> feed to ONNX model -> parse logits -> return Entities
	return nil
}
