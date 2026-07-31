# AegisProxy: Project Context & Handoff

## 📌 What is this project?
**AegisProxy** (AI Data Firewall) is a high-performance reverse proxy written in **Go**, designed for secure interaction between corporate networks and external LLMs (primarily the OpenAI API). 

Its main goal is to intercept requests, mask sensitive data (PII: names, emails, credit cards, phone numbers, API keys) before sending them to the cloud, and restore (unmask) them in the response from the neural network. The LLM provider never sees the actual user data.

## 🏗 What has been done (MVP)
1. **Project Initialized**: Microservice structure created (Go 1.22+).
2. **StateStore**: Thread-safe in-memory LRU cache based on `sync.Map` implemented in `internal/store/statemap.go`. It stores token-to-data bindings (e.g., `[EMAIL_1]` -> `john@doe.com`).
3. **Masker (Sanitizer)**: Masking engine written in `internal/sanitizer/masker.go`.
4. **Proxy Handler**: Logic for intercepting POST requests to `/v1/chat/completions` written in `internal/proxy/handler.go`. It parses JSON, sanitizes text, queries `api.openai.com`, and unmasks the response.
5. **Entry Point**: The server starts on port `8080` in `cmd/server/main.go`.

## 🚀 Future Plans (Next Steps) - ALL COMPLETED!
1. **[DONE] MVP Testing**: Wrote unit tests for the masker and handler.
2. **[DONE] ONNX Runtime Integration (NER)**: Replaced/supplemented regex with a local ML model (SLM) for smart zero-latency entity recognition.
3. **[DONE] Distributed Storage**: Added **Redis** support in `StateStore` for scaling.
4. **[DONE] Configuration**: Extracted hardcoded ports, target APIs, and rules into `.yaml` or environment variables.
5. **[DONE] Streaming**: Added support for `stream: true` (SSE) to proxy chat responses in real-time.

## 🤖 AI Assistant Instructions
If you are reading this file in a new session:
- You are in the root directory of the AegisProxy project.
- Your stack: Go (Golang). Architectural style: minimalism, high performance, minimum external dependencies.
- Check current tasks in `task.md` or ask the user what to do next!
