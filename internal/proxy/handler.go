package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aegisproxy/core/internal/audit"
	"github.com/aegisproxy/core/internal/metrics"
	"github.com/aegisproxy/core/internal/providers"
	"github.com/aegisproxy/core/internal/sanitizer"
)

// ProxyHandler intercepts and sanitizes API requests for multiple LLM providers
type ProxyHandler struct {
	masker      *sanitizer.Masker
	secureKeys  map[string]string
	auditLogger audit.Logger
	adapters    []providers.Adapter
}

// NewProxyHandler creates a new ProxyHandler
func NewProxyHandler(m *sanitizer.Masker, secureKeys map[string]string, auditLogger audit.Logger) *ProxyHandler {
	return &ProxyHandler{
		masker:      m,
		secureKeys:  secureKeys,
		auditLogger: auditLogger,
		adapters: []providers.Adapter{
			&providers.OpenAIAdapter{},
			&providers.AnthropicAdapter{},
			&providers.GeminiAdapter{},
		},
	}
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

	var adapter providers.Adapter
	for _, a := range h.adapters {
		if a.IsMatch(r) {
			adapter = a
			break
		}
	}

	if adapter == nil {
		status = "404"
		http.Error(w, "Unsupported API endpoint", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		status = "400"
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	maskedBody, model, isStream, err := adapter.MaskRequest(body, h.masker)
	if err != nil {
		status = "400"
		http.Error(w, "Failed to parse or mask request JSON", http.StatusBadRequest)
		return
	}

	targetURL := adapter.BaseURL() + adapter.TargetPath(r)
	proxyReq, err := http.NewRequest(r.Method, targetURL, bytes.NewBuffer(maskedBody))
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

	adapter.InjectAuth(proxyReq, h.secureKeys)

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		status = "502"
		http.Error(w, "Failed to reach target", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	status = fmt.Sprintf("%d", resp.StatusCode)

	if isStream {
		adapter.HandleStream(w, resp, h.masker)
		h.dispatchAuditEvent(start, r, model, maskedBody, nil)
		return
	}

	respBody, _ := io.ReadAll(resp.Body)
	var usage *providers.Usage

	if resp.StatusCode == http.StatusOK {
		unmaskedBody, extractedUsage, err := adapter.UnmaskResponse(respBody, h.masker)
		if err == nil {
			respBody = unmaskedBody
			usage = extractedUsage
		} else {
			slog.Error("Failed to unmask response", "error", err)
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

	h.dispatchAuditEvent(start, r, model, maskedBody, usage)
}

func (h *ProxyHandler) dispatchAuditEvent(start time.Time, r *http.Request, model string, maskedBody []byte, usage *providers.Usage) {
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
	if usage != nil {
		pTokens = usage.PromptTokens
		cTokens = usage.CompletionTokens
		tTokens = usage.TotalTokens
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
