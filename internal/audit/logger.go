package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEvent represents a single API request/response cycle
type AuditEvent struct {
	Timestamp        time.Time `json:"timestamp"`
	ClientIP         string    `json:"client_ip"`
	Model            string    `json:"model"`
	DurationMs       int64     `json:"duration_ms"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	PIITokensMasked  int       `json:"pii_tokens_masked"`
}

// Logger defines how audit events are recorded
type Logger interface {
	LogEvent(event AuditEvent) error
}

// FileAuditLogger records audit events to a JSONL file
type FileAuditLogger struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileAuditLogger creates a new logger that writes to the given file path
func NewFileAuditLogger(filePath string) (*FileAuditLogger, error) {
	err := os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &FileAuditLogger{
		file: file,
	}, nil
}

// LogEvent writes the audit event as a JSON line to the file
func (l *FileAuditLogger) LogEvent(event AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err = l.file.Write(append(data, '\n'))
	return err
}

// Close closes the underlying file
func (l *FileAuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
