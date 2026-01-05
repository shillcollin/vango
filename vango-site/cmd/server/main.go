package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/vango-dev/vango-site/pkg/pages"
	"github.com/vango-dev/vango/v2/pkg/render"
	"github.com/vango-dev/vango/v2/pkg/server"
)

//go:embed static
var staticFiles embed.FS

//go:embed client
var clientFiles embed.FS

func main() {
	// Create Vango server with dev mode for local development
	config := server.DefaultServerConfig().WithDevMode()
	srv := server.New(config)

	// Create renderer for SSR
	renderer := render.NewRenderer(render.RendererConfig{
		Pretty: true,
	})

	// Serve static files (CSS, images)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Serve Vango thin client
	clientFS, err := fs.Sub(clientFiles, "client")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/_vango/", http.StripPrefix("/_vango/", http.FileServer(http.FS(clientFS))))

	// Set up the root component factory for WebSocket sessions
	srv.SetRootComponent(func() server.Component {
		return pages.HomeComponent()
	})

	// Homepage SSR handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// Render the initial page with thin client script
		component := pages.HomeComponent()
		node := component.Render()

		pageData := render.PageData{
			Title:        "Vango - Full-stack Go Apps",
			Body:         node,
			ClientScript: "/_vango/client.js",
			StyleSheets:  []string{"/static/css/styles.css"},
			Lang:         "en",
			Debug:        true,
			Meta: []render.MetaTag{
				{Name: "description", Content: "Server-rendered by default, with opt-in client-side interactivity via JS or WASM. AI's new favorite framework."},
				{Name: "viewport", Content: "width=device-width, initial-scale=1"},
			},
			Links: []render.LinkTag{
				{Rel: "icon", Href: "/static/img/vango-icon.svg", Type: "image/svg+xml"},
				{Rel: "preconnect", Href: "https://fonts.googleapis.com"},
				{Rel: "preconnect", Href: "https://fonts.gstatic.com"},
				{Rel: "stylesheet", Href: "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap"},
			},
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := renderer.RenderPage(w, pageData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// Mount Vango WebSocket handler
	http.Handle("/_vango/ws", srv.WebSocketHandler())
	http.Handle("/_vango/live", srv.WebSocketHandler())

	log.Println("Vango website running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
