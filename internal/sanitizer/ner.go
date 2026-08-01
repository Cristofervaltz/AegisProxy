package sanitizer

import (
	"context"
	"regexp"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/backends"
	"github.com/knights-analytics/hugot/pipelines"
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

// ONNXExtractor uses a Small Language Model (SLM) for NER via ONNX Runtime
type ONNXExtractor struct {
	pipeline *pipelines.TokenClassificationPipeline
	session  *hugot.Session
	mu       sync.Mutex
}

// NewONNXExtractor creates a new ONNXExtractor using the specified model path.
// modelPath must point to a directory containing the model.onnx and tokenizer.json
func NewONNXExtractor(modelPath string) (*ONNXExtractor, error) {
	// Initialize Hugot session (defaults to GoMLX backend)
	session, err := hugot.NewGoSession(context.Background())
	if err != nil {
		return nil, err
	}

	config := backends.PipelineConfig[*pipelines.TokenClassificationPipeline]{
		ModelPath: modelPath,
		Name:      "ner",
	}
	
	// Create token classification pipeline
	pipe, err := hugot.NewPipeline(session, config)
	if err != nil {
		session.Destroy()
		return nil, err
	}

	return &ONNXExtractor{
		pipeline: pipe,
		session:  session,
	}, nil
}

func (o *ONNXExtractor) Extract(text string) []Entity {
	if o.pipeline == nil {
		return nil
	}
	
	res, err := o.pipeline.RunPipeline(context.Background(), []string{text})
	if err != nil || len(res.Entities) == 0 {
		return nil
	}

	var entities []Entity
	for _, ent := range res.Entities[0] {
		label := ent.Entity
		// Remove B- or I- prefixes standard in NER datasets (e.g. B-PER -> PER)
		if len(label) > 2 && (label[:2] == "B-" || label[:2] == "I-") {
			label = label[2:]
		}
		
		// For MVP, we filter only typical PII labels like PER (Person), ORG (Organization), LOC (Location)
		if label == "PER" || label == "ORG" || label == "LOC" {
			entities = append(entities, Entity{
				Type:  label,
				Value: ent.Word,
			})
		}
	}
	return entities
}

// Close destroys the underlying ONNX session to free memory
func (o *ONNXExtractor) Close() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.session != nil {
		o.session.Destroy()
		o.session = nil
	}
}
