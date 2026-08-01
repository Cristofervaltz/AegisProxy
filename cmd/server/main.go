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

	"context"
	"github.com/knights-analytics/hugot"
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

	// Initialize Extractors
	regexExt := sanitizer.NewRegexExtractor()

	var extractors []sanitizer.Extractor
	extractors = append(extractors, regexExt)

	modelPath := "./models/distilbert-ner"
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		slog.Info("Downloading NER model from Hugging Face... This might take a while.")
		os.MkdirAll("./models", 0755)
		opt := hugot.NewDownloadOptions()
		_, err := hugot.DownloadModel(context.Background(), "knights-analytics/distilbert-base-uncased-finetuned-ner", modelPath, opt)
		if err != nil {
			slog.Error("Failed to download model", "error", err)
		} else {
			slog.Info("Model downloaded successfully")
		}
	}

	onnxExt, err := sanitizer.NewONNXExtractor(modelPath)
	if err != nil {
		slog.Error("Failed to initialize ONNX Extractor. Continuing with Regex only.", "error", err)
	} else {
		extractors = append(extractors, onnxExt)
		defer onnxExt.Close()
	}

	// Initialize Admin Server (Metrics & UI)
	adminServer := admin.NewAdminServer(regexExt)
	go adminServer.Start(":9090")

	// Initialize the PII Masker
	masker := sanitizer.NewMasker(stateStore, extractors...)

	// Initialize the Proxy Handler targeting configured API
	proxyHandler := proxy.NewProxyHandler(masker, cfg.TargetAPI)

	http.Handle("/", proxyHandler)

	slog.Info("AegisProxy is starting", "port", cfg.Port, "target", cfg.TargetAPI)

	if err := http.ListenAndServe(cfg.Port, nil); err != nil {
		slog.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}
