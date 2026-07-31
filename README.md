# AegisProxy (AI Data Firewall)

## 📌 What is this project?
**AegisProxy** is a high-performance reverse proxy written in **Go**, designed for secure interaction between corporate networks and external LLMs (primarily the OpenAI API). 

Its main goal is to intercept requests, mask sensitive data (PII: names, emails, credit cards, phone numbers, API keys) before sending them to the cloud, and restore (unmask) them in the response from the neural network. The LLM provider never sees the actual user data.

## 🏗 Architecture & MVP
1. **StateStore**: A thread-safe in-memory LRU cache based on `sync.Map`. It stores mappings between tokens and real data (e.g., `[EMAIL_1]` -> `john@doe.com`).
2. **Masker (Sanitizer)**: The masking engine. Currently operates using regular expressions, covering basic data types, with support for extensible Extractors (like ONNX models).
3. **Proxy Handler**: Intercepts POST requests to `/v1/chat/completions`. It parses JSON, passes the text through the Masker, forwards the request to `api.openai.com`, and unmasks the response.

## 🚀 Roadmap (Future Plans)
1. **[DONE] MVP Testing**: Write unit tests and perform manual testing.
2. **[DONE] ONNX Runtime Integration (NER)**: Replace/supplement regex with a local ML model for smart entity recognition.
3. **[DONE] Distributed Storage**: Support **Redis** in `StateStore` for horizontal scaling.
4. **[DONE] Configuration**: Extract ports, keys, and rules into `.yaml` / `.env`.
5. **[DONE] Streaming**: Support `stream: true` (SSE) to proxy responses in real-time.
