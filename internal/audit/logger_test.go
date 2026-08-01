package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileAuditLogger(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewFileAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	event := AuditEvent{
		Timestamp:        time.Now(),
		ClientIP:         "127.0.0.1",
		Model:            "gpt-4",
		DurationMs:       150,
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
		PIITokensMasked:  3,
	}

	err = logger.LogEvent(event)
	if err != nil {
		t.Fatalf("Failed to log event: %v", err)
	}
	logger.Close()

	// Verify the file was created and contains the correct JSON
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var parsed AuditEvent
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Failed to parse log file JSON: %v", err)
	}

	if parsed.ClientIP != "127.0.0.1" {
		t.Errorf("Expected IP 127.0.0.1, got %s", parsed.ClientIP)
	}
	if parsed.PIITokensMasked != 3 {
		t.Errorf("Expected 3 PII tokens masked, got %d", parsed.PIITokensMasked)
	}
}
