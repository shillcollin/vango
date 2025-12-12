package handlers_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vangoframework/rhone/internal/auth"
	"github.com/vangoframework/rhone/internal/config"
	"github.com/vangoframework/rhone/internal/middleware"
)

// testConfig creates a minimal config for testing
func testConfig() *config.Config {
	return &config.Config{
		Port:               "8080",
		BaseURL:            "http://localhost:8080",
		Environment:        "development",
		GitHubClientID:     "test_client_id",
		GitHubClientSecret: "test_client_secret",
		GitHubCallbackURL:  "http://localhost:8080/auth/callback",
		GitHubAppID:        12345,
		GitHubAppSlug:      "test-app",
		SessionSecret:      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		SessionMaxAge:      time.Hour,
		CSRFSecret:         "0123456789abcdef0123456789abcdef",
	}
}

// testRouter creates a minimal router for testing auth flows (no database required)
func testRouter(t *testing.T) http.Handler {
	t.Helper()

	cfg := testConfig()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	sessions := auth.NewSessionStore(cfg.SessionSecret, cfg.SessionMaxAge, false)
	github := auth.NewGitHubOAuth(cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.GitHubCallbackURL)

	r := chi.NewRouter()
	r.Use(middleware.Session(sessions))

	// Public routes
	r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("login page"))
	})

	r.Get("/auth/github", func(w http.ResponseWriter, r *http.Request) {
		state := "test_state"
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    state,
			Path:     "/",
			MaxAge:   600,
			HttpOnly: true,
		})
		authURL := github.AuthorizeURL(state)
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	})

	r.Get("/github/connect", func(w http.ResponseWriter, r *http.Request) {
		state := "test_state"
		http.SetCookie(w, &http.Cookie{
			Name:     "github_app_state",
			Value:    state,
			Path:     "/",
			MaxAge:   600,
			HttpOnly: true,
		})
		installURL := "https://github.com/apps/" + cfg.GitHubAppSlug + "/installations/new?state=" + state
		http.Redirect(w, r, installURL, http.StatusTemporaryRedirect)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth)
		r.Get("/apps", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("apps page"))
		})
		r.Get("/settings", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("settings page"))
		})
	})

	_ = logger // suppress unused warning

	return r
}

func TestLoginRedirect(t *testing.T) {
	router := testRouter(t)
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(server.URL + "/auth/github")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)

	location := resp.Header.Get("Location")
	assert.Contains(t, location, "github.com/login/oauth/authorize")
	assert.Contains(t, location, "client_id=test_client_id")
	assert.Contains(t, location, "state=test_state")

	// Verify state cookie is set
	cookies := resp.Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "oauth_state" {
			stateCookie = c
			break
		}
	}
	require.NotNil(t, stateCookie)
	assert.Equal(t, "test_state", stateCookie.Value)
}

func TestProtectedRouteRedirect(t *testing.T) {
	router := testRouter(t)
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Test /apps requires auth
	resp, err := client.Get(server.URL + "/apps")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/login", resp.Header.Get("Location"))
}

func TestProtectedSettingsRedirect(t *testing.T) {
	router := testRouter(t)
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Test /settings requires auth
	resp, err := client.Get(server.URL + "/settings")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/login", resp.Header.Get("Location"))
}

func TestGitHubAppConnectRedirect(t *testing.T) {
	router := testRouter(t)
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(server.URL + "/github/connect")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)

	location := resp.Header.Get("Location")
	assert.Contains(t, location, "github.com/apps/test-app/installations/new")
	assert.Contains(t, location, "state=test_state")

	// Verify state cookie is set
	cookies := resp.Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "github_app_state" {
			stateCookie = c
			break
		}
	}
	require.NotNil(t, stateCookie)
	assert.Equal(t, "test_state", stateCookie.Value)
}
