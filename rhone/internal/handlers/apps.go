package handlers

import (
	"net/http"

	"github.com/vangoframework/rhone/internal/middleware"
	"github.com/vangoframework/rhone/internal/templates/pages"
)

// ListApps shows the list of apps for the current team.
// Stub handler - to be implemented in Phase 3.
func (h *Handlers) ListApps(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 3
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head><title>Apps | Rhone</title></head>
		<body>
			<h1>Apps</h1>
			<p>This page will be implemented in Phase 3.</p>
			<a href="/">Back to Dashboard</a>
		</body>
		</html>
	`))
}

// NewApp shows the form to create a new app.
// Stub handler - to be implemented in Phase 3.
func (h *Handlers) NewApp(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 3
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head><title>New App | Rhone</title></head>
		<body>
			<h1>New App</h1>
			<p>This page will be implemented in Phase 3.</p>
			<a href="/">Back to Dashboard</a>
		</body>
		</html>
	`))
}

// Settings shows the settings page with GitHub integrations.
func (h *Handlers) Settings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)

	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Fetch GitHub installations for the team
	installations, err := h.queries.GetTeamGitHubInstallations(ctx, uuidToPgUUID(session.TeamID))
	if err != nil {
		h.logger.Error("failed to get github installations", "error", err)
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}

	// Convert to view models
	var installationViews []pages.GitHubInstallationView
	for _, inst := range installations {
		installationViews = append(installationViews, pages.GitHubInstallationView{
			InstallationID: inst.InstallationID,
			AccountLogin:   inst.AccountLogin,
			AccountType:    inst.AccountType,
			IsSuspended:    inst.SuspendedAt.Valid,
		})
	}

	pages.Settings(session, installationViews).Render(ctx, w)
}
