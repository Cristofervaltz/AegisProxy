package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aegisproxy/core/internal/audit"
	"github.com/aegisproxy/core/internal/providers"
	"github.com/aegisproxy/core/internal/sanitizer"
	"github.com/aegisproxy/core/internal/store"
)

func TestProxyHandler(t *testing.T) {
	memStore := store.NewMemoryStore()
	masker := sanitizer.NewMasker(memStore)

	// Mock target server (OpenAI API)
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		// Verify target server receives MASKED data
		if strings.Contains(bodyStr, "john@doe.com") {
			t.Errorf("Target server received unmasked email: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, "[EMAIL_") {
			t.Errorf("Target server did not receive EMAIL token: %s", bodyStr)
		}

		// Verify Authorization header was injected securely
		if auth := r.Header.Get("Authorization"); auth != "Bearer super_secure_mock_key" {
			t.Errorf("Expected injected Authorization header, got: %s", auth)
		}

		// Send back a mock response mimicking OpenAI
		mockResp := ChatResponse{
			Id:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1677652288,
			Model:   "gpt-3.5-turbo-0301",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      ChatMessage `json:"message,omitempty"`
				Delta        ChatMessage `json:"delta,omitempty"`
				FinishReason string      `json:"finish_reason,omitempty"`
			}{
				{
					Index: 0,
					Message: ChatMessage{
						Role: "assistant",
						// The bot repeats the token, we expect the proxy to unmask it
						Content: "Hello, your email is " + extractToken(bodyStr) + ".",
					},
					FinishReason: "stop",
				},
			},
		}

		respBytes, _ := json.Marshal(mockResp)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(respBytes)
	}))
	defer targetServer.Close()

	mockLogger := &MockAuditLogger{}
	handler := NewProxyHandler(masker, map[string]string{"OPENAI_API_KEY": "super_secure_mock_key"}, mockLogger)

	for _, a := range handler.adapters {
		if a.Name() == "openai" {
			a.(*providers.OpenAIAdapter).SetBaseURL(targetServer.URL)
		}
	}

	reqPayload := ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hi, my email is john@doe.com."},
		},
	}
	reqBytes, _ := json.Marshal(reqPayload)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status OK, got %v", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var proxyResp ChatResponse
	if err := json.Unmarshal(respBody, &proxyResp); err != nil {
		t.Fatalf("Failed to parse proxy response: %v", err)
	}

	finalContent := proxyResp.Choices[0].Message.Content
	if !strings.Contains(finalContent, "john@doe.com") {
		t.Errorf("Expected proxy to unmask email in response, got: %s", finalContent)
	}

	// Wait a tiny bit for the async audit logger goroutine
	time.Sleep(10 * time.Millisecond)
	if mockLogger.LastEvent == nil {
		t.Fatalf("Expected audit event to be logged, but got nil")
	}
	if mockLogger.LastEvent.PIITokensMasked != 1 {
		t.Errorf("Expected 1 PII token masked in audit log, got %d", mockLogger.LastEvent.PIITokensMasked)
	}
	if mockLogger.LastEvent.Model != "gpt-3.5-turbo" {
		t.Errorf("Expected model gpt-3.5-turbo in audit log, got %s", mockLogger.LastEvent.Model)
	}
}

// Helper to extract the token generated in the test
func extractToken(body string) string {
	var req ChatRequest
	_ = json.Unmarshal([]byte(body), &req)
	words := strings.Split(req.Messages[0].Content, " ")
	return words[len(words)-1] // It's at the end "Hi, my email is [EMAIL_1]."
}

// MockAuditLogger captures the last logged event for testing
type MockAuditLogger struct {
	LastEvent *audit.AuditEvent
}

func (m *MockAuditLogger) LogEvent(event audit.AuditEvent) error {
	m.LastEvent = &event
	return nil
}

type ChatMessage struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}
type ChatResponse struct {
	Id      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message,omitempty"`
		Delta        ChatMessage `json:"delta,omitempty"`
		FinishReason string      `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}
