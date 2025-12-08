package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/shillcollin/gai/obs"
)

func main() {
	if err := loadDotEnv(".env", "apps/webdemo/backend/.env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("failed to load .env: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	shutdown, err := initObservability(ctx, "gai-web-demo")
	if err != nil {
		log.Printf("observability init warning: %v", err)
	}
	if shutdown != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := shutdown(shutdownCtx); err != nil {
				log.Printf("observability shutdown warning: %v", err)
			}
		}()
	}

	providers, err := buildProviders()
	if err != nil {
		log.Fatalf("provider init failed: %v", err)
	}
	if len(providers) == 0 {
		log.Fatal("no providers available; set API keys before starting the server")
	}

	prompts, err := loadPromptAssets()
	if err != nil {
		log.Fatalf("prompt init failed: %v", err)
	}
	if prompts.System.Version != "" {
		log.Printf("loaded prompt chat_system version=%s fingerprint=%s", prompts.System.Version, prompts.System.Fingerprint)
	}
	if prompts.ToolLimit.Version != "" {
		log.Printf("loaded prompt tool_limit_finalizer version=%s fingerprint=%s", prompts.ToolLimit.Version, prompts.ToolLimit.Fingerprint)
	}

	tavily := newTavilyClient(http.DefaultClient, os.Getenv("TAVILY_API_KEY"))

	api := &chatHandler{providers: providers, tavily: tavily, prompt: prompts.System, prompts: prompts.Registry, toolLimit: prompts.ToolLimit}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/providers", api.handleProviders)
	mux.HandleFunc("/api/chat", api.handleChat)
	mux.HandleFunc("/api/chat/stream", api.handleChatStream)

	addr := strings.TrimSpace(os.Getenv("GAI_WEB_DEMO_ADDR"))
	if addr == "" {
		addr = ":8080"
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           withCORS(withJSONHeaders(mux)),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("GAI web demo listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}

func withJSONHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Default to JSON for API responses, but avoid forcing a content type on
        // streaming endpoints which set NDJSON to enable incremental rendering.
        if r.URL.Path != "/api/chat/stream" {
            w.Header().Set("Content-Type", "application/json")
        }
        w.Header().Set("Cache-Control", "no-store")
        next.ServeHTTP(w, r)
    })
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func initObservability(ctx context.Context, service string) (func(context.Context) error, error) {
	opts := obs.DefaultOptions()
	opts.ServiceName = service

	setExporterFromEnv(&opts)
	configureMetricsFromEnv(&opts)
	configureBraintrustFromEnv(&opts)

	if opts.Exporter == obs.ExporterNone && !opts.Braintrust.Enabled {
		log.Printf("observability disabled (no exporter / Braintrust)")
		return func(context.Context) error { return nil }, nil
	}
	log.Printf("observability enabled: exporter=%s braintrust=%t", opts.Exporter, opts.Braintrust.Enabled)
	return obs.Init(ctx, opts)
}

func setExporterFromEnv(opts *obs.Options) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GAI_OBS_EXPORTER"))) {
	case "stdout":
		opts.Exporter = obs.ExporterStdout
	case "otlp":
		opts.Exporter = obs.ExporterOTLP
	case "none":
		opts.Exporter = obs.ExporterNone
	}

	if opts.Exporter == obs.ExporterOTLP {
		if endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")); endpoint != "" {
			opts.Endpoint = endpoint
		}
		if strings.EqualFold(os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"), "true") {
			opts.Insecure = true
		}
	}

	if opts.Exporter == obs.ExporterOTLP && opts.Endpoint == "" {
		log.Printf("OTLP exporter requested but OTEL_EXPORTER_OTLP_ENDPOINT missing; disabling exporter")
		opts.Exporter = obs.ExporterNone
	}

	if opts.Exporter != obs.ExporterOTLP {
		opts.Endpoint = ""
		opts.Insecure = false
	}
}

func configureMetricsFromEnv(opts *obs.Options) {
	if strings.EqualFold(os.Getenv("GAI_OBS_DISABLE_METRICS"), "true") {
		opts.DisableMetrics = true
	}
	if ratio := strings.TrimSpace(os.Getenv("GAI_OBS_SAMPLE_RATIO")); ratio != "" {
		if v, err := strconv.ParseFloat(ratio, 64); err == nil && v > 0 && v <= 1 {
			opts.SampleRatio = v
		}
	}
}

func configureBraintrustFromEnv(opts *obs.Options) {
	key := strings.TrimSpace(os.Getenv("BRAINTRUST_API_KEY"))
	if key == "" {
		opts.Braintrust.Enabled = false
		return
	}
	proj := strings.TrimSpace(os.Getenv("BRAINTRUST_PROJECT_NAME"))
	projID := strings.TrimSpace(os.Getenv("BRAINTRUST_PROJECT_ID"))
	if proj == "" && projID == "" {
		log.Printf("Braintrust API key provided but project metadata missing; disabling Braintrust sink")
		opts.Braintrust.Enabled = false
		return
	}
	opts.Braintrust.Enabled = true
	dataset := strings.TrimSpace(os.Getenv("BRAINTRUST_DATASET"))
	if dataset == "" {
		dataset = "<auto>"
	}
	log.Printf("Braintrust enabled (dataset=%s)", dataset)
	opts.Braintrust.APIKey = key
	opts.Braintrust.Project = proj
	opts.Braintrust.ProjectID = projID
	opts.Braintrust.Dataset = strings.TrimSpace(os.Getenv("BRAINTRUST_DATASET"))
	if baseURL := strings.TrimSpace(os.Getenv("BRAINTRUST_BASE_URL")); baseURL != "" {
		opts.Braintrust.BaseURL = baseURL
	}
}

func loadDotEnv(paths ...string) error {
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			if key == "" {
				continue
			}
			val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
			}
		}
		return nil
	}
	return os.ErrNotExist
}
