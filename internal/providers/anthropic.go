package providers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/aegisproxy/core/internal/sanitizer"
)

type AnthropicAdapter struct {
	baseURL string
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // can be string or array of objects
}

type anthropicRequest struct {
	Model    string             `json:"model"`
	System   string             `json:"system,omitempty"`
	Messages []anthropicMessage `json:"messages"`
	Stream   bool               `json:"stream,omitempty"`
}

type anthropicResponse struct {
	Id      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"delta,omitempty"`
}

func (a *AnthropicAdapter) Name() string { return "anthropic" }

func (a *AnthropicAdapter) IsMatch(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/v1/messages")
}

func (a *AnthropicAdapter) BaseURL() string {
	if a.baseURL != "" {
		return a.baseURL
	}
	return "https://api.anthropic.com"
}

func (a *AnthropicAdapter) SetBaseURL(url string) {
	a.baseURL = url
}

func (a *AnthropicAdapter) TargetPath(r *http.Request) string {
	return r.URL.Path
}

func (a *AnthropicAdapter) InjectAuth(req *http.Request, secureKeys map[string]string) {
	if key, ok := secureKeys["ANTHROPIC_API_KEY"]; ok && key != "" {
		req.Header.Set("x-api-key", key)
	}
	// Also ensure version header is set if missing, but usually client sends it.
}

func maskAnthropicContent(content interface{}, masker *sanitizer.Masker) interface{} {
	switch v := content.(type) {
	case string:
		return masker.Mask(v)
	case []interface{}:
		for i, part := range v {
			if partMap, ok := part.(map[string]interface{}); ok {
				if t, ok := partMap["type"].(string); ok && t == "text" {
					if textVal, ok := partMap["text"].(string); ok {
						partMap["text"] = masker.Mask(textVal)
						v[i] = partMap
					}
				}
			}
		}
		return v
	default:
		return content
	}
}

func (a *AnthropicAdapter) MaskRequest(body []byte, masker *sanitizer.Masker) ([]byte, string, bool, error) {
	var reqPayload anthropicRequest
	if err := json.Unmarshal(body, &reqPayload); err != nil {
		return nil, "", false, err
	}

	if reqPayload.System != "" {
		reqPayload.System = masker.Mask(reqPayload.System)
	}

	for i, msg := range reqPayload.Messages {
		reqPayload.Messages[i].Content = maskAnthropicContent(msg.Content, masker)
	}

	maskedBody, err := json.Marshal(reqPayload)
	return maskedBody, reqPayload.Model, reqPayload.Stream, err
}

func (a *AnthropicAdapter) UnmaskResponse(body []byte, masker *sanitizer.Masker) ([]byte, *Usage, error) {
	var respPayload anthropicResponse
	if err := json.Unmarshal(body, &respPayload); err != nil {
		return body, nil, err
	}

	for i, contentBlock := range respPayload.Content {
		if contentBlock.Type == "text" {
			respPayload.Content[i].Text = masker.Unmask(contentBlock.Text)
		}
	}

	unmaskedBody, err := json.Marshal(respPayload)
	if err != nil {
		return body, nil, err
	}

	var usage *Usage
	if respPayload.Usage != nil {
		usage = &Usage{
			PromptTokens:     respPayload.Usage.InputTokens,
			CompletionTokens: respPayload.Usage.OutputTokens,
			TotalTokens:      respPayload.Usage.InputTokens + respPayload.Usage.OutputTokens,
		}
	}

	return unmaskedBody, usage, nil
}

func (a *AnthropicAdapter) HandleStream(w http.ResponseWriter, resp *http.Response, masker *sanitizer.Masker) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
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
			var chunk anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				if chunk.Type == "content_block_delta" && chunk.Delta.Type == "text_delta" && chunk.Delta.Text != "" {
					chunk.Delta.Text = masker.Unmask(chunk.Delta.Text)
					unmaskedData, err := json.Marshal(chunk)
					if err == nil {
						fmt.Fprintf(w, "data: %s\n\n", string(unmaskedData))
						flusher.Flush()
						continue
					}
				}
			}
		}

		fmt.Fprint(w, line)
		flusher.Flush()
	}
}
