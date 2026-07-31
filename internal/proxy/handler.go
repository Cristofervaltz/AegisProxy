package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"log/slog"

	"github.com/aegisproxy/core/internal/metrics"
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
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
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
		Message      ChatMessage `json:"message,omitempty"`
		Delta        ChatMessage `json:"delta,omitempty"`
		FinishReason string      `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

// ServeHTTP implements the http.Handler interface
func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := "200"
	defer func() {
		duration := time.Since(start).Seconds()
		metrics.RequestsTotal.WithLabelValues(status).Inc()
		metrics.RequestDuration.WithLabelValues(status).Observe(duration)
	}()

	if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
		status = "404"
		http.Error(w, "Only POST /v1/chat/completions is supported", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		status = "400"
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var reqPayload ChatRequest
	if err := json.Unmarshal(body, &reqPayload); err != nil {
		status = "400"
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}

	for i, msg := range reqPayload.Messages {
		reqPayload.Messages[i].Content = h.masker.Mask(msg.Content)
	}

	maskedBody, _ := json.Marshal(reqPayload)

	proxyReq, err := http.NewRequest(r.Method, h.target+r.URL.Path, bytes.NewBuffer(maskedBody))
	if err != nil {
		status = "500"
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	for k, vv := range r.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}
	proxyReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(maskedBody)))
	proxyReq.Header.Del("Accept-Encoding")

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		status = "502"
		http.Error(w, "Failed to reach target", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	status = fmt.Sprintf("%d", resp.StatusCode)

	if reqPayload.Stream {
		h.handleStreamResponse(w, resp)
		return
	}

	h.handleNormalResponse(w, resp)
}

func (h *ProxyHandler) handleNormalResponse(w http.ResponseWriter, resp *http.Response) {
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		var respPayload ChatResponse
		if err := json.Unmarshal(respBody, &respPayload); err == nil {
			for i, choice := range respPayload.Choices {
				respPayload.Choices[i].Message.Content = h.masker.Unmask(choice.Message.Content)
			}
			unmaskedBody, _ := json.Marshal(respPayload)
			respBody = unmaskedBody
		}
	}
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(respBody)))
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func (h *ProxyHandler) handleStreamResponse(w http.ResponseWriter, resp *http.Response) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported by client", http.StatusInternalServerError)
		return
	}

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				slog.Error("Error reading stream", "error", err)
			}
			break
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			if data == "[DONE]" {
				fmt.Fprint(w, "data: [DONE]\n\n")
				flusher.Flush()
				continue
			}

			var chunk ChatResponse
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				for i, choice := range chunk.Choices {
					if choice.Delta.Content != "" {
						chunk.Choices[i].Delta.Content = h.masker.Unmask(choice.Delta.Content)
					}
				}
				unmaskedData, _ := json.Marshal(chunk)
				fmt.Fprintf(w, "data: %s\n\n", string(unmaskedData))
				flusher.Flush()
				continue
			}
		}

		fmt.Fprint(w, line)
		flusher.Flush()
	}
}
