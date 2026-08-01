//go:build cgo
// +build cgo

package sanitizer

import (
	"context"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/backends"
	"github.com/knights-analytics/hugot/pipelines"
)

// ONNXExtractor uses a Small Language Model (SLM) for NER via ONNX Runtime
type ONNXExtractor struct {
	pipeline *pipelines.TokenClassificationPipeline
	session  *hugot.Session
	mu       sync.Mutex
}

// NewONNXExtractor creates a new ONNXExtractor using the specified model path.
// modelPath must point to a directory containing the model.onnx and tokenizer.json
func NewONNXExtractor(modelPath string) (*ONNXExtractor, error) {
	// Initialize Hugot session
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
