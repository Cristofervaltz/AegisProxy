//go:build !cgo
// +build !cgo

package sanitizer

import (
	"errors"
)

// ONNXExtractor is a stub for when CGO is disabled
type ONNXExtractor struct{}

func NewONNXExtractor(modelPath string) (*ONNXExtractor, error) {
	return nil, errors.New("ONNX extractor requires CGO_ENABLED=1")
}

func (o *ONNXExtractor) Extract(text string) []Entity {
	return nil
}

func (o *ONNXExtractor) Close() {}
