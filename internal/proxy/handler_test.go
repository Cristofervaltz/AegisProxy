package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

	handler := NewProxyHandler(masker, targetServer.URL)

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
}

// Helper to extract the token generated in the test
func extractToken(body string) string {
	var req ChatRequest
	_ = json.Unmarshal([]byte(body), &req)
	words := strings.Split(req.Messages[0].Content, " ")
	return words[len(words)-1] // It's at the end "Hi, my email is [EMAIL_1]."
}
