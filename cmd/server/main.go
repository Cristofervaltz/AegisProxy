package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aegisproxy/core/internal/admin"
	"github.com/aegisproxy/core/internal/config"
	"github.com/aegisproxy/core/internal/proxy"
	"github.com/aegisproxy/core/internal/sanitizer"
	"github.com/aegisproxy/core/internal/store"
)

func main() {
	// Setup structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.LoadConfig()

	// Initialize State Store based on config
	var stateStore store.StateStore
	if cfg.StoreType == "redis" {
		slog.Info("Using Redis for StateStore", "addr", cfg.RedisAddr)
		stateStore = store.NewRedisStore(cfg.RedisAddr, "", 0, 24*time.Hour)
	} else {
		slog.Info("Using InMemory for StateStore")
		stateStore = store.NewMemoryStore()
	}

	// Initialize Extractors (Regex + ONNX Stub)
	regexExt := sanitizer.NewRegexExtractor()
	onnxExt := sanitizer.NewONNXExtractor("./models/ner_model.onnx")

	// Initialize Admin Server (Metrics & UI)
	adminServer := admin.NewAdminServer(regexExt)
	go adminServer.Start(":9090")

	// Initialize the PII Masker
	masker := sanitizer.NewMasker(stateStore, regexExt, onnxExt)

	// Initialize the Proxy Handler targeting configured API
	proxyHandler := proxy.NewProxyHandler(masker, cfg.TargetAPI)

	http.Handle("/", proxyHandler)

	slog.Info("AegisProxy is starting", "port", cfg.Port, "target", cfg.TargetAPI)
	
	if err := http.ListenAndServe(cfg.Port, nil); err != nil {
		slog.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}
