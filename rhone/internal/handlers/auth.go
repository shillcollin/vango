package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/vangoframework/rhone/internal/auth"
	"github.com/vangoframework/rhone/internal/database/queries"
	"github.com/vangoframework/rhone/internal/templates/pages"
)

// Login renders the login page.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	pages.Login().Render(r.Context(), w)
}

// LoginStart initiates the GitHub OAuth flow.
func (h *Handlers) LoginStart(w http.ResponseWriter, r *http.Request) {
	// Generate random state for CSRF protection
	state := generateState()

	// Store state in cookie for verification
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   h.config.IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to GitHub
	authURL := h.github.AuthorizeURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// AuthCallback handles the OAuth callback from GitHub.
func (h *Handlers) AuthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		h.logger.Error("missing oauth state cookie")
		http.Redirect(w, r, "/login?error=invalid_state", http.StatusSeeOther)
		return
	}

	if r.URL.Query().Get("state") != stateCookie.Value {
		h.logger.Error("oauth state mismatch")
		http.Redirect(w, r, "/login?error=invalid_state", http.StatusSeeOther)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Check for error from GitHub
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		h.logger.Error("github oauth error", "error", errMsg)
		http.Redirect(w, r, "/login?error=github_error", http.StatusSeeOther)
		return
	}

	// Exchange code for token
	code := r.URL.Query().Get("code")
	accessToken, err := h.github.ExchangeCode(ctx, code)
	if err != nil {
		h.logger.Error("failed to exchange code", "error", err)
		http.Redirect(w, r, "/login?error=token_exchange", http.StatusSeeOther)
		return
	}

	// Get user info
	githubUser, err := h.github.GetUser(ctx, accessToken)
	if err != nil {
		h.logger.Error("failed to get github user", "error", err)
		http.Redirect(w, r, "/login?error=user_fetch", http.StatusSeeOther)
		return
	}

	// Upsert user in database
	user, err := h.queries.UpsertUser(ctx, queries.UpsertUserParams{
		GithubID:       githubUser.ID,
		GithubUsername: githubUser.Login,
		Email:          toPgText(githubUser.Email),
		AvatarUrl:      toPgText(githubUser.AvatarURL),
	})
	if err != nil {
		h.logger.Error("failed to upsert user", "error", err)
		http.Redirect(w, r, "/login?error=database", http.StatusSeeOther)
		return
	}

	// Get or create default team
	team, err := h.getOrCreateDefaultTeam(ctx, user)
	if err != nil {
		h.logger.Error("failed to get/create team", "error", err)
		http.Redirect(w, r, "/login?error=database", http.StatusSeeOther)
		return
	}

	// Create session
	session := &auth.SessionData{
		UserID:    pgUUIDToUUID(user.ID),
		Email:     pgTextToString(user.Email),
		Username:  user.GithubUsername,
		AvatarURL: pgTextToString(user.AvatarUrl),
		TeamID:    pgUUIDToUUID(team.ID),
		TeamSlug:  team.Slug,
	}

	if err := h.sessions.Set(w, session); err != nil {
		h.logger.Error("failed to set session", "error", err)
		http.Redirect(w, r, "/login?error=session", http.StatusSeeOther)
		return
	}

	// Redirect to dashboard
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout clears the session and redirects to home.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Clear(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// getOrCreateDefaultTeam returns the user's first team or creates a personal one.
func (h *Handlers) getOrCreateDefaultTeam(ctx context.Context, user queries.User) (queries.Team, error) {
	// Check if user has any teams
	teams, err := h.queries.GetUserTeams(ctx, user.ID)
	if err != nil {
		return queries.Team{}, err
	}

	if len(teams) > 0 {
		return teams[0], nil
	}

	// Create personal team
	team, err := h.queries.CreateTeam(ctx, queries.CreateTeamParams{
		Name: user.GithubUsername + "'s Team",
		Slug: user.GithubUsername,
		Plan: "free",
	})
	if err != nil {
		return queries.Team{}, err
	}

	// Add user as owner
	_, err = h.queries.AddTeamMember(ctx, queries.AddTeamMemberParams{
		TeamID: team.ID,
		UserID: user.ID,
		Role:   "owner",
	})
	if err != nil {
		return queries.Team{}, err
	}

	return team, nil
}

// generateState generates a random state string for CSRF protection.
func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// toPgText converts a string to pgtype.Text.
func toPgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

// pgTextToString converts pgtype.Text to a string.
func pgTextToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// pgUUIDToUUID converts pgtype.UUID to uuid.UUID.
func pgUUIDToUUID(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.UUID{}
	}
	return uuid.UUID(p.Bytes)
}
