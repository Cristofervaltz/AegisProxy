package providers

import (
	"net/http"

	"github.com/aegisproxy/core/internal/sanitizer"
)

// Usage Stats extracted from responses
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Adapter defines the contract for handling different LLM provider APIs
type Adapter interface {
	// Name returns the provider name (e.g. "openai", "anthropic", "gemini")
	Name() string

	// IsMatch determines if the request should be handled by this provider
	IsMatch(r *http.Request) bool

	// BaseURL returns the upstream base URL (e.g. "https://api.openai.com")
	BaseURL() string

	// SetBaseURL overrides the base URL (useful for testing)
	SetBaseURL(url string)

	// TargetPath returns the path to append to the BaseURL
	TargetPath(r *http.Request) string

	// InjectAuth injects the secure credentials into the request
	InjectAuth(req *http.Request, secureKeys map[string]string)

	// MaskRequest reads the request, masks PII, and returns the modified bytes and model name
	MaskRequest(body []byte, masker *sanitizer.Masker) (maskedBody []byte, model string, isStream bool, err error)

	// UnmaskResponse reads the response, unmasks PII, and returns modified bytes and usage stats
	UnmaskResponse(body []byte, masker *sanitizer.Masker) (unmaskedBody []byte, usage *Usage, err error)

	// HandleStream processes a Server-Sent Events stream, unmasking on the fly
	HandleStream(w http.ResponseWriter, resp *http.Response, masker *sanitizer.Masker)
}
