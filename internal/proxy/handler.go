package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/aegisproxy/core/internal/sanitizer"
)

// ProxyHandler intercepts and sanitizes OpenAI API requests
type ProxyHandler struct {
	masker *sanitizer.Masker
	target string
}

// NewProxyHandler creates a new ProxyHandler
func NewProxyHandler(m *sanitizer.Masker, target string) *ProxyHandler {
	return &ProxyHandler{
		masker: m,
		target: target,
	}
}

// ChatMessage represents the OpenAI chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents the incoming OpenAI request
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

// ChatResponse represents the OpenAI API response
type ChatResponse struct {
	Id      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
}

// ServeHTTP implements the http.Handler interface
func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
		http.Error(w, "Only POST /v1/chat/completions is supported", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var reqPayload ChatRequest
	if err := json.Unmarshal(body, &reqPayload); err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}

	if reqPayload.Stream {
		http.Error(w, "Streaming is not supported in MVP", http.StatusNotImplemented)
		return
	}

	// 1. Mask Request
	for i, msg := range reqPayload.Messages {
		reqPayload.Messages[i].Content = h.masker.Mask(msg.Content)
	}

	maskedBody, _ := json.Marshal(reqPayload)

	// 2. Forward Request
	proxyReq, err := http.NewRequest(r.Method, h.target+r.URL.Path, bytes.NewBuffer(maskedBody))
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	for k, vv := range r.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}
	proxyReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(maskedBody)))
	// Remove Accept-Encoding to prevent gzip response which is harder to parse on the fly for MVP
	proxyReq.Header.Del("Accept-Encoding")

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Failed to reach target", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// 3. Unmask Response
	if resp.StatusCode == http.StatusOK {
		var respPayload ChatResponse
		if err := json.Unmarshal(respBody, &respPayload); err == nil {
			for i, choice := range respPayload.Choices {
				respPayload.Choices[i].Message.Content = h.masker.Unmask(choice.Message.Content)
			}
			unmaskedBody, _ := json.Marshal(respPayload)
			respBody = unmaskedBody
		} else {
			fmt.Println("Error unmarshaling response:", err)
		}
	}

	// 4. Return to client
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(respBody)))
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}
