package chat

import (
	"encoding/json"
	"net/http"

	"github.com/vango-dev/vango/v2/pkg/server"
)

// RegisterAPIRoutes registers REST API routes for backwards compatibility.
func RegisterAPIRoutes(srv *server.Server, svc *Service) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/providers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(svc.Providers())
	})

	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request")
			return
		}

		// Create event channel
		eventCh := make(chan StreamEvent, 100)

		// Run in background
		go func() {
			_ = svc.SendMessage(r.Context(), req, eventCh)
		}()

		// Collect full response
		var text string
		var usage *Usage
		for event := range eventCh {
			switch event.Type {
			case "text.delta":
				text += event.TextDelta
			case "finish":
				usage = event.Usage
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"text":  text,
			"usage": usage,
		})
	})

	srv.SetHandler(mux)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
