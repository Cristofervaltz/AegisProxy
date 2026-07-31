package main

import (
	"log"
	"net/http"
	"time"

	"github.com/aegisproxy/core/internal/config"
	"github.com/aegisproxy/core/internal/proxy"
	"github.com/aegisproxy/core/internal/sanitizer"
	"github.com/aegisproxy/core/internal/store"
)

func main() {
	cfg := config.LoadConfig()

	// Initialize State Store based on config
	var stateStore store.StateStore
	if cfg.StoreType == "redis" {
		log.Printf("Using Redis for StateStore at %s", cfg.RedisAddr)
		stateStore = store.NewRedisStore(cfg.RedisAddr, "", 0, 24*time.Hour)
	} else {
		log.Println("Using InMemory for StateStore")
		stateStore = store.NewMemoryStore()
	}

	// Initialize Extractors (Regex + ONNX Stub)
	regexExt := sanitizer.NewRegexExtractor()
	onnxExt := sanitizer.NewONNXExtractor("./models/ner_model.onnx")

	// Initialize the PII Masker
	masker := sanitizer.NewMasker(stateStore, regexExt, onnxExt)

	// Initialize the Proxy Handler targeting configured API
	proxyHandler := proxy.NewProxyHandler(masker, cfg.TargetAPI)

	http.Handle("/", proxyHandler)

	log.Printf("AegisProxy is starting on port %s", cfg.Port)
	log.Printf("Forwarding requests to %s", cfg.TargetAPI)
	
	if err := http.ListenAndServe(cfg.Port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
