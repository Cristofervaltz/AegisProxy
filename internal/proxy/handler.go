package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aegisproxy/core/internal/audit"
	"github.com/aegisproxy/core/internal/metrics"
	"github.com/aegisproxy/core/internal/sanitizer"
)

// ProxyHandler intercepts and sanitizes OpenAI API requests
type ProxyHandler struct {
	masker       *sanitizer.Masker
	target       string
	secureAPIKey string
	auditLogger  audit.Logger
}

// NewProxyHandler creates a new ProxyHandler
func NewProxyHandler(m *sanitizer.Masker, target string, secureKey string, auditLogger audit.Logger) *ProxyHandler {
	return &ProxyHandler{
		masker:       m,
		target:       target,
		secureAPIKey: secureKey,
		auditLogger:  auditLogger,
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
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
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

	maskedBody, err := json.Marshal(reqPayload)
	if err != nil {
		status = "500"
		http.Error(w, "Failed to marshal request", http.StatusInternalServerError)
		return
	}

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

	if h.secureAPIKey != "" {
		proxyReq.Header.Set("Authorization", "Bearer "+h.secureAPIKey)
	}

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
		h.dispatchAuditEvent(start, r, reqPayload.Model, maskedBody, nil) // Stream doesn't give usage easily yet
		return
	}

	respPayload := h.handleNormalResponse(w, resp)
	h.dispatchAuditEvent(start, r, reqPayload.Model, maskedBody, respPayload)
}

func (h *ProxyHandler) handleNormalResponse(w http.ResponseWriter, resp *http.Response) *ChatResponse {
	respBody, _ := io.ReadAll(resp.Body)
	var respPayload *ChatResponse

	if resp.StatusCode == http.StatusOK {
		var parsed ChatResponse
		if err := json.Unmarshal(respBody, &parsed); err == nil {
			respPayload = &parsed
			for i, choice := range parsed.Choices {
				parsed.Choices[i].Message.Content = h.masker.Unmask(choice.Message.Content)
			}
			unmaskedBody, err := json.Marshal(parsed)
			if err == nil {
				respBody = unmaskedBody
			}
		}
	}
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(respBody)))
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)

	return respPayload
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
				unmaskedData, err := json.Marshal(chunk)
				if err == nil {
					fmt.Fprintf(w, "data: %s\n\n", string(unmaskedData))
					flusher.Flush()
				}
				continue
			}
		}

		fmt.Fprint(w, line)
		flusher.Flush()
	}
}

func (h *ProxyHandler) dispatchAuditEvent(start time.Time, r *http.Request, model string, maskedBody []byte, respPayload *ChatResponse) {
	if h.auditLogger == nil {
		return
	}

	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = strings.Split(forwarded, ",")[0]
	}

	tokenPattern := regexp.MustCompile(`\[[A-Z_]+_\d+\]`)
	piiMasked := len(tokenPattern.FindAllString(string(maskedBody), -1))

	var pTokens, cTokens, tTokens int
	if respPayload != nil && respPayload.Usage != nil {
		pTokens = respPayload.Usage.PromptTokens
		cTokens = respPayload.Usage.CompletionTokens
		tTokens = respPayload.Usage.TotalTokens
	}

	event := audit.AuditEvent{
		Timestamp:        start,
		ClientIP:         ip,
		Model:            model,
		DurationMs:       time.Since(start).Milliseconds(),
		PromptTokens:     pTokens,
		CompletionTokens: cTokens,
		TotalTokens:      tTokens,
		PIITokensMasked:  piiMasked,
	}

	go func() {
		if err := h.auditLogger.LogEvent(event); err != nil {
			slog.Error("Failed to write audit log", "error", err)
		}
	}()
}
