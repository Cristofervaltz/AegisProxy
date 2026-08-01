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

type OpenAIAdapter struct {
	baseURL string
}

type chatMessage struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

type chatResponse struct {
	Id      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      chatMessage `json:"message,omitempty"`
		Delta        chatMessage `json:"delta,omitempty"`
		FinishReason string      `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

func (a *OpenAIAdapter) Name() string { return "openai" }

func (a *OpenAIAdapter) IsMatch(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/v1/chat/completions")
}

func (a *OpenAIAdapter) BaseURL() string {
	if a.baseURL != "" {
		return a.baseURL
	}
	return "https://api.openai.com"
}

func (a *OpenAIAdapter) SetBaseURL(url string) {
	a.baseURL = url
}

func (a *OpenAIAdapter) TargetPath(r *http.Request) string {
	return r.URL.Path // keep the same path
}

func (a *OpenAIAdapter) InjectAuth(req *http.Request, secureKeys map[string]string) {
	if key, ok := secureKeys["OPENAI_API_KEY"]; ok && key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
}

func (a *OpenAIAdapter) MaskRequest(body []byte, masker *sanitizer.Masker) ([]byte, string, bool, error) {
	var reqPayload chatRequest
	if err := json.Unmarshal(body, &reqPayload); err != nil {
		return nil, "", false, err
	}

	for i, msg := range reqPayload.Messages {
		reqPayload.Messages[i].Content = masker.Mask(msg.Content)
	}

	maskedBody, err := json.Marshal(reqPayload)
	return maskedBody, reqPayload.Model, reqPayload.Stream, err
}

func (a *OpenAIAdapter) UnmaskResponse(body []byte, masker *sanitizer.Masker) ([]byte, *Usage, error) {
	var respPayload chatResponse
	if err := json.Unmarshal(body, &respPayload); err != nil {
		return body, nil, err // return original on error
	}

	for i, choice := range respPayload.Choices {
		respPayload.Choices[i].Message.Content = masker.Unmask(choice.Message.Content)
	}

	unmaskedBody, err := json.Marshal(respPayload)
	if err != nil {
		return body, nil, err
	}

	var usage *Usage
	if respPayload.Usage != nil {
		usage = &Usage{
			PromptTokens:     respPayload.Usage.PromptTokens,
			CompletionTokens: respPayload.Usage.CompletionTokens,
			TotalTokens:      respPayload.Usage.TotalTokens,
		}
	}

	return unmaskedBody, usage, nil
}

func (a *OpenAIAdapter) HandleStream(w http.ResponseWriter, resp *http.Response, masker *sanitizer.Masker) {
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
			if data == "[DONE]" {
				fmt.Fprint(w, "data: [DONE]\n\n")
				flusher.Flush()
				continue
			}

			var chunk chatResponse
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				for i, choice := range chunk.Choices {
					if choice.Delta.Content != "" {
						chunk.Choices[i].Delta.Content = masker.Unmask(choice.Delta.Content)
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
