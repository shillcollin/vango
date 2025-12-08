// Package main is the entry point for the webdemo Vango application.
package main

import (
	"bytes"
	"log"
	"log/slog"
	"net/http"
	"os"

	"webdemo-vango/app"
	"webdemo-vango/chat"

	"github.com/vango-dev/vango/v2/pkg/render"
	"github.com/vango-dev/vango/v2/pkg/server"
)

func main() {
	// Load environment variables
	if err := loadDotEnv(".env"); err != nil && !os.IsNotExist(err) {
		log.Printf("warning: failed to load .env: %v", err)
	}

	// Initialize chat backend
	chatService, err := chat.NewService()
	if err != nil {
		log.Fatalf("failed to initialize chat service: %v", err)
	}

	// Configure server
	cfg := server.DefaultServerConfig()
	cfg.Address = getEnvOr("VANGO_ADDR", ":3000")

	// Create server
	srv := server.New(cfg)
	srv.SetLogger(slog.Default())

	// Set root component factory - called for each WebSocket session
	srv.SetRootComponent(func() server.Component {
		return app.Root(chatService)
	})

	// Create HTTP mux for initial page load + static files
	mux := http.NewServeMux()

	// Serve static files
	mux.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))

	// Serve Vango client JS at /_vango/client.js
	mux.HandleFunc("/_vango/client.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, "public/_vango/client.js")
	})

	// Home page - renders initial HTML
	// The thin client connects via WebSocket for live interactivity
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Create the app component
		appComponent := app.Root(chatService)

		// Render the page
		renderer := render.NewRenderer(render.RendererConfig{})

		var buf bytes.Buffer
		err := renderer.RenderPage(&buf, render.PageData{
			Title:       "Vango Chat Demo",
			Body:        appComponent.Render(),
			StyleSheets: []string{"/public/styles.css"},
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		log.Printf("[DEBUG] SSR rendered with %d handlers\n", len(renderer.GetHandlers()))

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(buf.Bytes())
	})

	// Register API routes for backwards compatibility
	chat.RegisterAPIRoutes(srv, chatService)

	// Set HTTP handler (WebSocket handled at /_vango/ws automatically)
	srv.SetHandler(mux)

	log.Println("🚀 Vango webdemo running at http://localhost" + cfg.Address)
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

	for _, line := range splitLines(string(data)) {
		line = trimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		key, value, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func splitKeyValue(s string) (key, value string, ok bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			key = trimSpace(s[:i])
			value = trimSpace(s[i+1:])
			// Remove quotes
			if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') {
				if value[len(value)-1] == value[0] {
					value = value[1 : len(value)-1]
				}
			}
			return key, value, key != ""
		}
	}
	return "", "", false
}
