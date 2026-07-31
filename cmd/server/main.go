package main

import (
	"log"
	"net/http"

	"github.com/aegisproxy/core/internal/proxy"
	"github.com/aegisproxy/core/internal/sanitizer"
	"github.com/aegisproxy/core/internal/store"
)

func main() {
	// Initialize in-memory state store
	stateStore := store.NewMemoryStore()

	// Initialize the PII Masker
	masker := sanitizer.NewMasker(stateStore)

	// Initialize the Proxy Handler targeting OpenAI
	targetAPI := "https://api.openai.com"
	proxyHandler := proxy.NewProxyHandler(masker, targetAPI)

	http.Handle("/", proxyHandler)

	port := ":8080"
	log.Printf("AegisProxy is starting on port %s", port)
	log.Printf("Forwarding requests to %s", targetAPI)
	
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
