package handlers

import (
	"net/http"
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

// Settings shows the settings page.
// Stub handler - to be implemented in later phases.
func (h *Handlers) Settings(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in later phases
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head><title>Settings | Rhone</title></head>
		<body>
			<h1>Settings</h1>
			<p>This page will be implemented in a later phase.</p>
			<a href="/">Back to Dashboard</a>
		</body>
		</html>
	`))
}
