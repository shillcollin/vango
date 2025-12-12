package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/vangoframework/rhone/internal/database/queries"
	"github.com/vangoframework/rhone/internal/middleware"
	"github.com/vangoframework/rhone/internal/templates/components"
)

// ConnectGitHub redirects to GitHub App installation page.
func (h *Handlers) ConnectGitHub(w http.ResponseWriter, r *http.Request) {
	// Generate state for CSRF protection
	state := generateState()

	http.SetCookie(w, &http.Cookie{
		Name:     "github_app_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   h.config.IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to GitHub App installation
	installURL := fmt.Sprintf(
		"https://github.com/apps/%s/installations/new?state=%s",
		h.config.GitHubAppSlug,
		state,
	)

	http.Redirect(w, r, installURL, http.StatusTemporaryRedirect)
}

// GitHubCallback handles the callback after GitHub App installation.
func (h *Handlers) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)

	// Verify session exists
	if session == nil {
		h.logger.Error("no session in github callback")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Verify state
	stateCookie, err := r.Cookie("github_app_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		h.logger.Error("invalid github app state")
		http.Redirect(w, r, "/settings?error=invalid_state", http.StatusSeeOther)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "github_app_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Get installation ID from query
	installationIDStr := r.URL.Query().Get("installation_id")
	if installationIDStr == "" {
		// User may have clicked "Cancel"
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	installationID, err := strconv.ParseInt(installationIDStr, 10, 64)
	if err != nil {
		h.logger.Error("invalid installation_id", "value", installationIDStr)
		http.Redirect(w, r, "/settings?error=invalid_installation", http.StatusSeeOther)
		return
	}

	// Fetch installation details from GitHub
	installation, err := h.githubApp.GetInstallation(ctx, installationID)
	if err != nil {
		h.logger.Error("failed to get installation", "error", err)
		http.Redirect(w, r, "/settings?error=github_error", http.StatusSeeOther)
		return
	}

	// Store installation in database
	_, err = h.queries.CreateGitHubInstallation(ctx, queries.CreateGitHubInstallationParams{
		TeamID:         uuidToPgUUID(session.TeamID),
		InstallationID: installationID,
		AccountType:    installation.Account.Type,
		AccountLogin:   installation.Account.Login,
		AccountID:      installation.Account.ID,
	})
	if err != nil {
		h.logger.Error("failed to store installation", "error", err)
		http.Redirect(w, r, "/settings?error=database_error", http.StatusSeeOther)
		return
	}

	h.logger.Info("github app installed",
		"team_id", session.TeamID,
		"installation_id", installationID,
		"account", installation.Account.Login,
	)

	// Redirect to settings with success
	http.Redirect(w, r, "/settings?success=github_connected", http.StatusSeeOther)
}

// ListRepositories returns available repositories for the team.
func (h *Handlers) ListRepositories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)

	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get all installations for the team
	installations, err := h.queries.GetTeamGitHubInstallations(ctx, uuidToPgUUID(session.TeamID))
	if err != nil {
		h.logger.Error("failed to get installations", "error", err)
		http.Error(w, "Failed to load repositories", http.StatusInternalServerError)
		return
	}

	// Collect repos from all installations
	var allRepos []components.RepoView
	for _, inst := range installations {
		if inst.SuspendedAt.Valid {
			continue // Skip suspended installations
		}

		repos, err := h.githubApp.ListInstallationRepos(ctx, inst.InstallationID)
		if err != nil {
			h.logger.Warn("failed to list repos for installation",
				"installation_id", inst.InstallationID,
				"error", err,
			)
			continue
		}

		for _, repo := range repos {
			allRepos = append(allRepos, components.RepoView{
				FullName:       repo.FullName,
				Description:    repo.Description,
				Private:        repo.Private,
				DefaultBranch:  repo.DefaultBranch,
				InstallationID: inst.InstallationID,
			})
		}
	}

	// Render repository list
	components.RepoSelector(allRepos, "").Render(ctx, w)
}

// SearchRepositories searches/filters repositories by query.
func (h *Handlers) SearchRepositories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)

	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := strings.ToLower(r.URL.Query().Get("q"))

	// Get all installations for the team
	installations, err := h.queries.GetTeamGitHubInstallations(ctx, uuidToPgUUID(session.TeamID))
	if err != nil {
		h.logger.Error("failed to get installations", "error", err)
		http.Error(w, "Failed to load repositories", http.StatusInternalServerError)
		return
	}

	// Collect and filter repos from all installations
	var allRepos []components.RepoView
	for _, inst := range installations {
		if inst.SuspendedAt.Valid {
			continue // Skip suspended installations
		}

		repos, err := h.githubApp.ListInstallationRepos(ctx, inst.InstallationID)
		if err != nil {
			h.logger.Warn("failed to list repos for installation",
				"installation_id", inst.InstallationID,
				"error", err,
			)
			continue
		}

		for _, repo := range repos {
			// Filter by query if provided
			if query != "" {
				if !strings.Contains(strings.ToLower(repo.FullName), query) &&
					!strings.Contains(strings.ToLower(repo.Description), query) {
					continue
				}
			}

			allRepos = append(allRepos, components.RepoView{
				FullName:       repo.FullName,
				Description:    repo.Description,
				Private:        repo.Private,
				DefaultBranch:  repo.DefaultBranch,
				InstallationID: inst.InstallationID,
			})
		}
	}

	// Render filtered repository list
	components.RepoList(allRepos).Render(ctx, w)
}

// RepoSelector renders the repository selector component (HTMX partial).
func (h *Handlers) RepoSelector(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session := middleware.GetSession(ctx)

	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if team has any GitHub installations
	installations, err := h.queries.GetTeamGitHubInstallations(ctx, uuidToPgUUID(session.TeamID))
	if err != nil {
		h.logger.Error("failed to get installations", "error", err)
		http.Error(w, "Failed to check GitHub connection", http.StatusInternalServerError)
		return
	}

	if len(installations) == 0 {
		// No installations - show connect button
		components.GitHubConnectPrompt().Render(ctx, w)
		return
	}

	// Has installations - load repos
	h.ListRepositories(w, r)
}

// uuidToPgUUID converts uuid.UUID to pgtype.UUID.
func uuidToPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}
