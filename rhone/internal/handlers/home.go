package handlers

import (
	"net/http"

	"github.com/vangoframework/rhone/internal/middleware"
	"github.com/vangoframework/rhone/internal/templates/pages"
)

// Home handles the root path.
// Shows dashboard for authenticated users, landing page for others.
func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r.Context())

	// Render dashboard or landing page based on auth state
	if session != nil {
		pages.Dashboard(session).Render(r.Context(), w)
	} else {
		pages.Landing().Render(r.Context(), w)
	}
}
