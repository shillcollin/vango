// Package main is the entry point for the Kanban board demo.
package main

import (
	"bytes"
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"webdemo-kanban/pkg/app"
	"webdemo-kanban/pkg/db"
	"webdemo-kanban/pkg/hub"

	"github.com/vango-dev/vango/v2/pkg/render"
	"github.com/vango-dev/vango/v2/pkg/server"
)

func main() {
	// Load environment variables
	if err := loadDotEnv(".env"); err != nil && !os.IsNotExist(err) {
		log.Printf("warning: failed to load .env: %v", err)
	}

	// Initialize database pool
	var pool *db.Pool
	var err error
	pool, err = db.NewPool(context.Background())
	if err != nil {
		log.Printf("warning: Database not configured: %v", err)
		log.Println("Running in demo mode without persistence")
		pool = nil
	} else {
		defer pool.Close()
		log.Println("✅ Connected to PostgreSQL database")
	}

	// Initialize Hub
	h := hub.GetHub(pool)

	// Configure server
	cfg := server.DefaultServerConfig()
	cfg.Address = getEnvOr("VANGO_ADDR", ":3000")

	// Create server
	srv := server.New(cfg)
	srv.SetLogger(slog.Default())

	// Set root component factory - called for each WebSocket session
	// The path will be set from the request URL via context (handled by server)
	srv.SetRootComponent(func() server.Component {
		// WebSocket sessions need the path from somewhere
		// For now, default to "/" - the client will send the actual path
		return app.Root(pool, h, "/")
	})

	// Create HTTP mux
	mux := http.NewServeMux()

	// Serve static files
	mux.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))

	// Serve Vango client JS
	mux.HandleFunc("/_vango/client.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, "public/_vango/client.js")
	})

	// SSR handler for all pages
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Get the request path for routing
		path := r.URL.Path
		log.Printf("[DEBUG] SSR request for path: %s", path)

		// Create app component with the correct initial path
		appComponent := app.Root(pool, h, path)

		// Render
		renderer := render.NewRenderer(render.RendererConfig{})

		var buf bytes.Buffer
		err := renderer.RenderPage(&buf, render.PageData{
			Title:       "Kanban Board",
			Body:        appComponent.Render(),
			StyleSheets: []string{"/public/styles.css"},
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(buf.Bytes())
	})

	srv.SetHandler(mux)

	log.Println("🚀 Kanban Board running at http://localhost" + cfg.Address)
	log.Println("📡 WebSocket endpoint: ws://localhost" + cfg.Address + "/_vango/ws")
	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadDotEnv(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Remove quotes
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') {
			if value[len(value)-1] == value[0] {
				value = value[1 : len(value)-1]
			}
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	return nil
}
