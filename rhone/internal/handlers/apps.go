package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/vangoframework/rhone/internal/database/queries"
	"github.com/vangoframework/rhone/internal/domain"
	"github.com/vangoframework/rhone/internal/middleware"
	"github.com/vangoframework/rhone/internal/templates/components"
	"github.com/vangoframework/rhone/internal/templates/pages"
)

// ListApps shows the list of apps for the current team.
func (h *Handlers) ListApps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)

	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	apps, err := h.queries.GetTeamApps(ctx, uuidToPgUUID(session.TeamID))
	if err != nil {
		h.logger.Error("failed to get apps", "error", err)
		http.Error(w, "Failed to load apps", http.StatusInternalServerError)
		return
	}

	var appViews []pages.AppView
	for _, app := range apps {
		appViews = append(appViews, appToView(app))
	}

	pages.Apps(session, appViews).Render(ctx, w)
}

// NewApp shows the form to create a new app.
func (h *Handlers) NewApp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)

	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	pages.NewApp(session).Render(ctx, w)
}

// CreateApp handles POST /apps to create a new app.
func (h *Handlers) CreateApp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)

	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	// Generate and validate slug
	slug := domain.GenerateSlug(name)
	if slug == "" {
		http.Error(w, "Could not generate a valid slug from the app name", http.StatusBadRequest)
		return
	}
	if err := domain.ValidateSlug(slug); err != nil {
		http.Error(w, "Invalid app name: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Check slug uniqueness, append suffix if taken
	exists, err := h.queries.SlugExists(ctx, queries.SlugExistsParams{
		TeamID: uuidToPgUUID(session.TeamID),
		Slug:   slug,
	})
	if err != nil {
		h.logger.Error("failed to check slug", "error", err)
		http.Error(w, "Failed to create app", http.StatusInternalServerError)
		return
	}
	if exists {
		// Append random suffix to make unique
		slug = slug + "-" + randomSuffix(4)
		// Truncate if needed (slug + "-" + 4 chars = 5 extra)
		if len(slug) > 63 {
			slug = slug[:63]
			slug = strings.TrimRight(slug, "-")
		}
	}

	// Parse optional fields
	githubRepo := r.FormValue("github_repo")
	branch := r.FormValue("branch")
	if branch == "" {
		branch = "main"
	}
	region := r.FormValue("region")
	if region == "" {
		region = "iad"
	}
	autoDeploy := r.FormValue("auto_deploy") == "on" || r.FormValue("auto_deploy") == "true"

	installationIDStr := r.FormValue("github_installation_id")
	var installationID pgtype.Int8
	if installationIDStr != "" {
		id, err := strconv.ParseInt(installationIDStr, 10, 64)
		if err == nil {
			installationID = pgtype.Int8{Int64: id, Valid: true}
		}
	}

	// Create app
	app, err := h.queries.CreateApp(ctx, queries.CreateAppParams{
		TeamID:               uuidToPgUUID(session.TeamID),
		Name:                 name,
		Slug:                 slug,
		GithubRepo:           pgtype.Text{String: githubRepo, Valid: githubRepo != ""},
		GithubBranch:         pgtype.Text{String: branch, Valid: true},
		GithubInstallationID: installationID,
		Region:               pgtype.Text{String: region, Valid: true},
		AutoDeploy:           pgtype.Bool{Bool: autoDeploy, Valid: true},
	})
	if err != nil {
		h.logger.Error("failed to create app", "error", err)
		http.Error(w, "Failed to create app", http.StatusInternalServerError)
		return
	}

	h.logger.Info("app created",
		"app_id", app.ID,
		"slug", slug,
		"repo", githubRepo,
	)

	http.Redirect(w, r, "/apps/"+slug, http.StatusSeeOther)
}

// ShowApp shows the app dashboard.
func (h *Handlers) ShowApp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)
	slug := chi.URLParam(r, "slug")

	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
		TeamID: uuidToPgUUID(session.TeamID),
		Slug:   slug,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		h.logger.Error("failed to get app", "slug", slug, "error", err)
		http.Error(w, "Failed to load app", http.StatusInternalServerError)
		return
	}

	pages.AppDashboard(session, appToView(app)).Render(ctx, w)
}

// AppSettings shows the app settings page.
func (h *Handlers) AppSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)
	slug := chi.URLParam(r, "slug")

	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
		TeamID: uuidToPgUUID(session.TeamID),
		Slug:   slug,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		h.logger.Error("failed to get app", "slug", slug, "error", err)
		http.Error(w, "Failed to load app", http.StatusInternalServerError)
		return
	}

	// Get env vars (keys only, values masked)
	envVars, err := h.queries.GetAppEnvVars(ctx, app.ID)
	if err != nil {
		h.logger.Error("failed to get env vars", "error", err)
		envVars = []queries.EnvVar{}
	}

	var envVarViews []pages.EnvVarView
	for _, env := range envVars {
		envVarViews = append(envVarViews, pages.EnvVarView{
			Key:       env.Key,
			CreatedAt: env.CreatedAt.Time.Format("Jan 2, 2006"),
			UpdatedAt: env.UpdatedAt.Time.Format("Jan 2, 2006"),
		})
	}

	pages.AppSettings(session, appToView(app), envVarViews).Render(ctx, w)
}

// UpdateApp handles POST /apps/{slug}/settings to update app settings.
func (h *Handlers) UpdateApp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)
	slug := chi.URLParam(r, "slug")

	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
		TeamID: uuidToPgUUID(session.TeamID),
		Slug:   slug,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to load app", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	branch := r.FormValue("branch")
	region := r.FormValue("region")
	autoDeploy := r.FormValue("auto_deploy") == "on"

	_, err = h.queries.UpdateApp(ctx, queries.UpdateAppParams{
		ID:           app.ID,
		TeamID:       uuidToPgUUID(session.TeamID),
		Name:         name,
		GithubBranch: branch,
		Region:       region,
		AutoDeploy:   pgtype.Bool{Bool: autoDeploy, Valid: true},
	})
	if err != nil {
		h.logger.Error("failed to update app", "error", err)
		http.Error(w, "Failed to update app", http.StatusInternalServerError)
		return
	}

	h.logger.Info("app updated", "app_id", app.ID, "slug", slug)

	http.Redirect(w, r, "/apps/"+slug+"/settings?success=updated", http.StatusSeeOther)
}

// DeleteApp handles DELETE /apps/{slug} to delete an app.
func (h *Handlers) DeleteApp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)
	slug := chi.URLParam(r, "slug")

	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
		TeamID: uuidToPgUUID(session.TeamID),
		Slug:   slug,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to load app", http.StatusInternalServerError)
		return
	}

	// TODO: Delete Fly app if exists (Phase 5)

	// Delete from database (cascades to env_vars)
	if err := h.queries.DeleteApp(ctx, queries.DeleteAppParams{
		ID:     app.ID,
		TeamID: uuidToPgUUID(session.TeamID),
	}); err != nil {
		h.logger.Error("failed to delete app", "error", err)
		http.Error(w, "Failed to delete app", http.StatusInternalServerError)
		return
	}

	h.logger.Info("app deleted", "app_id", app.ID, "slug", slug)

	// HTMX response - redirect to apps list
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/apps")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/apps", http.StatusSeeOther)
}

// SlugPreview handles GET /api/slug/preview for HTMX live slug preview.
func (h *Handlers) SlugPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)

	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		components.SlugPreviewEmpty().Render(ctx, w)
		return
	}

	slug := domain.GenerateSlug(name)
	if slug == "" {
		components.SlugPreviewInvalid(slug, "Could not generate slug from name").Render(ctx, w)
		return
	}

	// Check if slug is valid
	if err := domain.ValidateSlug(slug); err != nil {
		components.SlugPreviewInvalid(slug, err.Error()).Render(ctx, w)
		return
	}

	// Check availability
	exists, err := h.queries.SlugExists(ctx, queries.SlugExistsParams{
		TeamID: uuidToPgUUID(session.TeamID),
		Slug:   slug,
	})
	if err != nil {
		h.logger.Error("failed to check slug", "error", err)
		http.Error(w, "Error checking slug", http.StatusInternalServerError)
		return
	}

	if exists {
		components.SlugPreviewTaken(slug).Render(ctx, w)
		return
	}

	components.SlugPreviewAvailable(slug).Render(ctx, w)
}

// SetEnvVar handles POST /apps/{slug}/env to set an environment variable.
func (h *Handlers) SetEnvVar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)
	slug := chi.URLParam(r, "slug")

	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
		TeamID: uuidToPgUUID(session.TeamID),
		Slug:   slug,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to load app", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	key := strings.ToUpper(strings.TrimSpace(r.FormValue("key")))
	value := r.FormValue("value")

	// Validate key
	if err := domain.ValidateEnvKey(key); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check reserved keys
	if domain.IsReservedEnvKey(key) {
		http.Error(w, "This environment variable key is reserved", http.StatusBadRequest)
		return
	}

	// Encrypt value
	ciphertext, nonce, err := h.crypto.Encrypt(value)
	if err != nil {
		h.logger.Error("failed to encrypt env var", "error", err)
		http.Error(w, "Failed to save environment variable", http.StatusInternalServerError)
		return
	}

	// Store (upsert)
	_, err = h.queries.CreateEnvVar(ctx, queries.CreateEnvVarParams{
		AppID:          app.ID,
		Key:            key,
		ValueEncrypted: ciphertext,
		Nonce:          nonce,
	})
	if err != nil {
		h.logger.Error("failed to create env var", "error", err)
		http.Error(w, "Failed to save environment variable", http.StatusInternalServerError)
		return
	}

	h.logger.Info("env var set", "app_id", app.ID, "key", key)

	// Return updated env var list for HTMX
	h.renderEnvVarList(w, r, app.ID, slug)
}

// DeleteEnvVar handles DELETE /apps/{slug}/env/{key}.
func (h *Handlers) DeleteEnvVar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)
	slug := chi.URLParam(r, "slug")
	key := chi.URLParam(r, "key")

	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
		TeamID: uuidToPgUUID(session.TeamID),
		Slug:   slug,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to load app", http.StatusInternalServerError)
		return
	}

	if err := h.queries.DeleteEnvVar(ctx, queries.DeleteEnvVarParams{
		AppID: app.ID,
		Key:   key,
	}); err != nil {
		h.logger.Error("failed to delete env var", "error", err)
		http.Error(w, "Failed to delete environment variable", http.StatusInternalServerError)
		return
	}

	h.logger.Info("env var deleted", "app_id", app.ID, "key", key)

	// Return updated env var list for HTMX
	h.renderEnvVarList(w, r, app.ID, slug)
}

// EnvVarForm handles GET /apps/{slug}/env/new to render the env var form.
func (h *Handlers) EnvVarForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)
	slug := chi.URLParam(r, "slug")

	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pages.EnvVarForm(slug).Render(ctx, w)
}

// renderEnvVarList renders the env var list for HTMX responses.
func (h *Handlers) renderEnvVarList(w http.ResponseWriter, r *http.Request, appID pgtype.UUID, slug string) {
	ctx := r.Context()

	envVars, err := h.queries.GetAppEnvVars(ctx, appID)
	if err != nil {
		h.logger.Error("failed to get env vars", "error", err)
		envVars = []queries.EnvVar{}
	}

	var envVarViews []pages.EnvVarView
	for _, env := range envVars {
		envVarViews = append(envVarViews, pages.EnvVarView{
			Key:       env.Key,
			CreatedAt: env.CreatedAt.Time.Format("Jan 2, 2006"),
			UpdatedAt: env.UpdatedAt.Time.Format("Jan 2, 2006"),
		})
	}

	pages.EnvVarList(slug, envVarViews).Render(ctx, w)
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

// appToView converts a database App to a template AppView.
func appToView(app queries.App) pages.AppView {
	var githubRepo string
	if app.GithubRepo.Valid {
		githubRepo = app.GithubRepo.String
	}

	var githubBranch string
	if app.GithubBranch.Valid {
		githubBranch = app.GithubBranch.String
	} else {
		githubBranch = "main"
	}

	var region string
	if app.Region.Valid {
		region = app.Region.String
	} else {
		region = "iad"
	}

	var autoDeploy bool
	if app.AutoDeploy.Valid {
		autoDeploy = app.AutoDeploy.Bool
	}

	return pages.AppView{
		ID:           pgUUIDToString(app.ID),
		Name:         app.Name,
		Slug:         app.Slug,
		GitHubRepo:   githubRepo,
		GitHubBranch: githubBranch,
		Region:       region,
		AutoDeploy:   autoDeploy,
		CreatedAt:    app.CreatedAt.Time.Format("Jan 2, 2006"),
	}
}

// pgUUIDToString converts pgtype.UUID to string.
func pgUUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuidBytesToString(u.Bytes)
}

// uuidBytesToString converts UUID bytes to string format.
func uuidBytesToString(b [16]byte) string {
	return strings.ToLower(strings.Replace(
		strings.Replace(
			strings.Replace(
				strings.Replace(
					strings.Replace(
						uuidToHex(b),
						"", "-", 8),
					"", "-", 13),
				"", "-", 18),
			"", "-", 23),
		"", "", 0))
}

func uuidToHex(b [16]byte) string {
	const hexDigits = "0123456789abcdef"
	buf := make([]byte, 32)
	for i, v := range b {
		buf[i*2] = hexDigits[v>>4]
		buf[i*2+1] = hexDigits[v&0x0f]
	}
	return string(buf)
}

// randomSuffix generates a random alphanumeric suffix of length n.
func randomSuffix(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// GetDecryptedEnvVars returns all environment variables for an app decrypted.
// This is used during deployment to provide env vars to the deployed app.
func (h *Handlers) GetDecryptedEnvVars(ctx context.Context, appID pgtype.UUID) (map[string]string, error) {
	envVars, err := h.queries.GetAppEnvVars(ctx, appID)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(envVars))
	for _, ev := range envVars {
		value, err := h.crypto.Decrypt(ev.ValueEncrypted, ev.Nonce)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt %s: %w", ev.Key, err)
		}
		result[ev.Key] = value
	}

	return result, nil
}
