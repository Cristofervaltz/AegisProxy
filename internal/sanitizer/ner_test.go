package sanitizer

import (
	"os"
	"testing"
)

func TestONNXExtractor(t *testing.T) {
	modelPath := "../../models/distilbert-ner"
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		t.Skip("Skipping ONNXExtractor test because model is not downloaded at " + modelPath)
	}

	extractor, err := NewONNXExtractor(modelPath)
	if err != nil {
		t.Fatalf("Failed to initialize ONNXExtractor: %v", err)
	}
	defer extractor.Close()

	text := "My name is John Doe and I work at Google in New York."
	entities := extractor.Extract(text)

	// We expect PER (John Doe), ORG (Google), LOC (New York)
	// Because tokens might be split into subwords, we just check if any PER, ORG, LOC was found
	
	foundPER := false
	foundORG := false
	foundLOC := false

	for _, ent := range entities {
		t.Logf("Found entity: [%s] %s", ent.Type, ent.Value)
		if ent.Type == "PER" {
			foundPER = true
		}
		if ent.Type == "ORG" {
			foundORG = true
		}
		if ent.Type == "LOC" {
			foundLOC = true
		}
	}

	if !foundPER {
		t.Errorf("Expected to find PER (John Doe), got none")
	}
	if !foundORG {
		t.Errorf("Expected to find ORG (Google), got none")
	}
	if !foundLOC {
		t.Errorf("Expected to find LOC (New York), got none")
	}
}
