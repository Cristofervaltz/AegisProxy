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

type GeminiAdapter struct {
	baseURL string
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts,omitempty"`
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason,omitempty"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata,omitempty"`
}

func (a *GeminiAdapter) Name() string { return "gemini" }

func (a *GeminiAdapter) IsMatch(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/v1beta/models/") && strings.Contains(r.URL.Path, "generateContent")
}

func (a *GeminiAdapter) BaseURL() string {
	if a.baseURL != "" {
		return a.baseURL
	}
	return "https://generativelanguage.googleapis.com"
}

func (a *GeminiAdapter) SetBaseURL(url string) {
	a.baseURL = url
}

func (a *GeminiAdapter) TargetPath(r *http.Request) string {
	return r.URL.Path
}

func (a *GeminiAdapter) InjectAuth(req *http.Request, secureKeys map[string]string) {
	if key, ok := secureKeys["GEMINI_API_KEY"]; ok && key != "" {
		req.Header.Set("x-goog-api-key", key)
	}
}

func (a *GeminiAdapter) MaskRequest(body []byte, masker *sanitizer.Masker) ([]byte, string, bool, error) {
	var reqPayload geminiRequest
	if err := json.Unmarshal(body, &reqPayload); err != nil {
		return nil, "", false, err
	}

	for i, content := range reqPayload.Contents {
		for j, part := range content.Parts {
			reqPayload.Contents[i].Parts[j].Text = masker.Mask(part.Text)
		}
	}

	maskedBody, err := json.Marshal(reqPayload)
	// Gemini models are usually in the URL path. 
	// We'll return "gemini" as model name for now, or extract from path if needed.
	isStream := strings.Contains(string(body), "streamGenerateContent")
	return maskedBody, "gemini", isStream, err
}

func (a *GeminiAdapter) UnmaskResponse(body []byte, masker *sanitizer.Masker) ([]byte, *Usage, error) {
	var respPayload geminiResponse
	if err := json.Unmarshal(body, &respPayload); err != nil {
		return body, nil, err
	}

	for i, candidate := range respPayload.Candidates {
		for j, part := range candidate.Content.Parts {
			respPayload.Candidates[i].Content.Parts[j].Text = masker.Unmask(part.Text)
		}
	}

	unmaskedBody, err := json.Marshal(respPayload)
	if err != nil {
		return body, nil, err
	}

	var usage *Usage
	if respPayload.UsageMetadata != nil {
		usage = &Usage{
			PromptTokens:     respPayload.UsageMetadata.PromptTokenCount,
			CompletionTokens: respPayload.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      respPayload.UsageMetadata.TotalTokenCount,
		}
	}

	return unmaskedBody, usage, nil
}

func (a *GeminiAdapter) HandleStream(w http.ResponseWriter, resp *http.Response, masker *sanitizer.Masker) {
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

	// Gemini stream is usually JSON array chunks, e.g. "data: {...}" or just raw JSON arrays chunked
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
			var chunk geminiResponse
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				for i, candidate := range chunk.Candidates {
					for j, part := range candidate.Content.Parts {
						if part.Text != "" {
							chunk.Candidates[i].Content.Parts[j].Text = masker.Unmask(part.Text)
						}
					}
				}
				unmaskedData, err := json.Marshal(chunk)
				if err == nil {
					fmt.Fprintf(w, "data: %s\n\n", string(unmaskedData))
					flusher.Flush()
					continue
				}
			}
		}

		fmt.Fprint(w, line)
		flusher.Flush()
	}
}
