# Phase 1: Foundation

> **Core infrastructure: Go server, authentication, database, and base UI**

**Status**: Not Started

---

## Overview

Phase 1 establishes the foundational infrastructure for Rhone. This includes the Go web server with Chi router, GitHub OAuth authentication, Neon Postgres database connection, session management, and the base HTMX + Templ UI.

### Goals

1. **Working web server**: Chi router handling HTTP requests
2. **User authentication**: GitHub OAuth flow complete
3. **Database connectivity**: Neon Postgres with migrations
4. **Session management**: Secure cookie-based sessions
5. **Base UI**: Dashboard shell with navigation
6. **Deployment**: Rhone running on Fly.io

### Non-Goals

1. GitHub App integration (Phase 2)
2. App management (Phase 3)
3. Billing integration (Phase 8)
4. Any build/deploy functionality (Phases 4-5)

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           PHASE 1 SCOPE                                  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │                         Chi Router                                   ││
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐                 ││
│  │  │ GET /        │ │ GET /login   │ │ GET /auth/   │                 ││
│  │  │ (dashboard)  │ │ (login page) │ │ callback     │                 ││
│  │  └──────┬───────┘ └──────┬───────┘ └──────┬───────┘                 ││
│  │         │                │                │                          ││
│  │         ▼                ▼                ▼                          ││
│  │  ┌──────────────────────────────────────────────────────────────┐   ││
│  │  │                      Middleware Stack                         │   ││
│  │  │  Logger → Recovery → Session → Auth (optional) → Handler     │   ││
│  │  └──────────────────────────────────────────────────────────────┘   ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │                         Services                                     ││
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐                 ││
│  │  │   Auth       │ │   Session    │ │   User       │                 ││
│  │  │   Service    │ │   Store      │ │   Repository │                 ││
│  │  └──────────────┘ └──────────────┘ └──────────────┘                 ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │                         Database                                     ││
│  │  ┌──────────────────────────────────────────────────────────────┐   ││
│  │  │                    Neon Postgres                              │   ││
│  │  │  ┌────────────┐ ┌────────────┐ ┌────────────┐                │   ││
│  │  │  │   users    │ │   teams    │ │  team_     │                │   ││
│  │  │  │            │ │            │ │  members   │                │   ││
│  │  │  └────────────┘ └────────────┘ └────────────┘                │   ││
│  │  └──────────────────────────────────────────────────────────────┘   ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Project Structure

```
rhone/
├── cmd/
│   └── rhone/
│       └── main.go                 # Entry point
├── internal/
│   ├── config/
│   │   └── config.go               # Environment configuration
│   ├── database/
│   │   ├── database.go             # Connection management
│   │   ├── migrations/
│   │   │   ├── 001_initial.up.sql
│   │   │   └── 001_initial.down.sql
│   │   └── queries/
│   │       ├── queries.sql         # sqlc queries
│   │       ├── db.go               # Generated
│   │       ├── models.go           # Generated
│   │       └── queries.sql.go      # Generated
│   ├── auth/
│   │   ├── github_oauth.go         # GitHub OAuth client
│   │   └── session.go              # Session management
│   ├── middleware/
│   │   ├── logger.go               # Request logging
│   │   ├── recovery.go             # Panic recovery
│   │   ├── session.go              # Session middleware
│   │   └── auth.go                 # Authentication check
│   ├── handlers/
│   │   ├── handlers.go             # Handler dependencies
│   │   ├── home.go                 # Dashboard handler
│   │   └── auth.go                 # Auth handlers
│   └── templates/
│       ├── layouts/
│       │   └── base.templ          # Base HTML layout
│       ├── pages/
│       │   ├── home.templ          # Dashboard page
│       │   └── login.templ         # Login page
│       └── components/
│           ├── nav.templ           # Navigation
│           └── flash.templ         # Flash messages
├── static/
│   ├── css/
│   │   └── styles.css              # Tailwind output
│   └── js/
│       └── app.js                  # HTMX + minimal JS
├── Dockerfile
├── fly.toml
├── go.mod
├── go.sum
├── sqlc.yaml
├── tailwind.config.js
└── package.json                    # For Tailwind build
```

---

## Core Types

### Configuration

```go
// internal/config/config.go
package config

import (
    "fmt"
    "os"
    "time"
)

type Config struct {
    // Server
    Port            string
    BaseURL         string
    Environment     string // development, staging, production

    // Database
    DatabaseURL     string

    // GitHub OAuth
    GitHubClientID     string
    GitHubClientSecret string
    GitHubCallbackURL  string

    // Session
    SessionSecret   string
    SessionMaxAge   time.Duration

    // Security
    CSRFSecret      string
}

func Load() (*Config, error) {
    cfg := &Config{
        Port:        getEnv("PORT", "8080"),
        BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),
        Environment: getEnv("ENVIRONMENT", "development"),

        DatabaseURL: mustGetEnv("DATABASE_URL"),

        GitHubClientID:     mustGetEnv("GITHUB_CLIENT_ID"),
        GitHubClientSecret: mustGetEnv("GITHUB_CLIENT_SECRET"),

        SessionSecret: mustGetEnv("SESSION_SECRET"),
        SessionMaxAge: 7 * 24 * time.Hour, // 1 week

        CSRFSecret: mustGetEnv("CSRF_SECRET"),
    }

    cfg.GitHubCallbackURL = cfg.BaseURL + "/auth/callback"

    return cfg, nil
}

func getEnv(key, fallback string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return fallback
}

func mustGetEnv(key string) string {
    value := os.Getenv(key)
    if value == "" {
        panic(fmt.Sprintf("required environment variable %s is not set", key))
    }
    return value
}

func (c *Config) IsDevelopment() bool {
    return c.Environment == "development"
}

func (c *Config) IsProduction() bool {
    return c.Environment == "production"
}
```

### Database Connection

```go
// internal/database/database.go
package database

import (
    "context"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
    Pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*DB, error) {
    config, err := pgxpool.ParseConfig(databaseURL)
    if err != nil {
        return nil, fmt.Errorf("parse database url: %w", err)
    }

    // Connection pool settings
    config.MaxConns = 25
    config.MinConns = 5
    config.MaxConnLifetime = 1 * time.Hour
    config.MaxConnIdleTime = 30 * time.Minute
    config.HealthCheckPeriod = 1 * time.Minute

    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("create pool: %w", err)
    }

    // Verify connection
    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("ping database: %w", err)
    }

    return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
    db.Pool.Close()
}

// Health check for /health endpoint
func (db *DB) Health(ctx context.Context) error {
    return db.Pool.Ping(ctx)
}
```

### User Model

```go
// internal/database/queries/models.go (generated by sqlc)
package queries

import (
    "time"

    "github.com/google/uuid"
)

type User struct {
    ID              uuid.UUID
    GitHubID        int64
    GitHubUsername  string
    Email           *string
    AvatarURL       *string
    StripeCustomerID *string
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type Team struct {
    ID                   uuid.UUID
    Name                 string
    Slug                 string
    StripeSubscriptionID *string
    Plan                 string
    CreatedAt            time.Time
    UpdatedAt            time.Time
}

type TeamMember struct {
    TeamID    uuid.UUID
    UserID    uuid.UUID
    Role      string
    CreatedAt time.Time
}
```

### Session Store

```go
// internal/auth/session.go
package auth

import (
    "encoding/gob"
    "net/http"
    "time"

    "github.com/google/uuid"
    "github.com/gorilla/securecookie"
)

func init() {
    // Register types for gob encoding
    gob.Register(uuid.UUID{})
    gob.Register(SessionData{})
}

type SessionData struct {
    UserID    uuid.UUID
    Email     string
    Username  string
    AvatarURL string
    TeamID    uuid.UUID  // Current team context
    TeamSlug  string
    CreatedAt time.Time
    ExpiresAt time.Time
}

type SessionStore struct {
    cookie *securecookie.SecureCookie
    name   string
    maxAge int
    secure bool
}

func NewSessionStore(secret string, maxAge time.Duration, secure bool) *SessionStore {
    // Use secret for both hash and encryption keys
    hashKey := []byte(secret)[:32]
    blockKey := []byte(secret)[32:64]

    return &SessionStore{
        cookie: securecookie.New(hashKey, blockKey),
        name:   "rhone_session",
        maxAge: int(maxAge.Seconds()),
        secure: secure,
    }
}

func (s *SessionStore) Get(r *http.Request) (*SessionData, error) {
    cookie, err := r.Cookie(s.name)
    if err != nil {
        return nil, err
    }

    var data SessionData
    if err := s.cookie.Decode(s.name, cookie.Value, &data); err != nil {
        return nil, err
    }

    // Check expiration
    if time.Now().After(data.ExpiresAt) {
        return nil, http.ErrNoCookie
    }

    return &data, nil
}

func (s *SessionStore) Set(w http.ResponseWriter, data *SessionData) error {
    data.CreatedAt = time.Now()
    data.ExpiresAt = time.Now().Add(time.Duration(s.maxAge) * time.Second)

    encoded, err := s.cookie.Encode(s.name, data)
    if err != nil {
        return err
    }

    http.SetCookie(w, &http.Cookie{
        Name:     s.name,
        Value:    encoded,
        Path:     "/",
        MaxAge:   s.maxAge,
        HttpOnly: true,
        Secure:   s.secure,
        SameSite: http.SameSiteLaxMode,
    })

    return nil
}

func (s *SessionStore) Clear(w http.ResponseWriter) {
    http.SetCookie(w, &http.Cookie{
        Name:     s.name,
        Value:    "",
        Path:     "/",
        MaxAge:   -1,
        HttpOnly: true,
        Secure:   s.secure,
        SameSite: http.SameSiteLaxMode,
    })
}
```

### GitHub OAuth

```go
// internal/auth/github_oauth.go
package auth

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"
)

type GitHubOAuth struct {
    ClientID     string
    ClientSecret string
    CallbackURL  string
    Scopes       []string
}

type GitHubUser struct {
    ID        int64  `json:"id"`
    Login     string `json:"login"`
    Email     string `json:"email"`
    AvatarURL string `json:"avatar_url"`
    Name      string `json:"name"`
}

type GitHubTokenResponse struct {
    AccessToken string `json:"access_token"`
    TokenType   string `json:"token_type"`
    Scope       string `json:"scope"`
    Error       string `json:"error"`
    ErrorDesc   string `json:"error_description"`
}

func NewGitHubOAuth(clientID, clientSecret, callbackURL string) *GitHubOAuth {
    return &GitHubOAuth{
        ClientID:     clientID,
        ClientSecret: clientSecret,
        CallbackURL:  callbackURL,
        Scopes:       []string{"read:user", "user:email"},
    }
}

// AuthorizeURL returns the GitHub OAuth authorization URL
func (g *GitHubOAuth) AuthorizeURL(state string) string {
    params := url.Values{
        "client_id":    {g.ClientID},
        "redirect_uri": {g.CallbackURL},
        "scope":        {strings.Join(g.Scopes, " ")},
        "state":        {state},
    }
    return "https://github.com/login/oauth/authorize?" + params.Encode()
}

// ExchangeCode exchanges the authorization code for an access token
func (g *GitHubOAuth) ExchangeCode(ctx context.Context, code string) (string, error) {
    data := url.Values{
        "client_id":     {g.ClientID},
        "client_secret": {g.ClientSecret},
        "code":          {code},
        "redirect_uri":  {g.CallbackURL},
    }

    req, err := http.NewRequestWithContext(ctx, "POST",
        "https://github.com/login/oauth/access_token",
        strings.NewReader(data.Encode()))
    if err != nil {
        return "", err
    }

    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Accept", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }

    var token GitHubTokenResponse
    if err := json.Unmarshal(body, &token); err != nil {
        return "", err
    }

    if token.Error != "" {
        return "", fmt.Errorf("github oauth error: %s - %s", token.Error, token.ErrorDesc)
    }

    return token.AccessToken, nil
}

// GetUser fetches the authenticated user's profile
func (g *GitHubOAuth) GetUser(ctx context.Context, accessToken string) (*GitHubUser, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+accessToken)
    req.Header.Set("Accept", "application/vnd.github+json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("github api error: %s", string(body))
    }

    var user GitHubUser
    if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
        return nil, err
    }

    // Fetch primary email if not public
    if user.Email == "" {
        email, err := g.getPrimaryEmail(ctx, accessToken)
        if err == nil {
            user.Email = email
        }
    }

    return &user, nil
}

func (g *GitHubOAuth) getPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
    if err != nil {
        return "", err
    }

    req.Header.Set("Authorization", "Bearer "+accessToken)
    req.Header.Set("Accept", "application/vnd.github+json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var emails []struct {
        Email    string `json:"email"`
        Primary  bool   `json:"primary"`
        Verified bool   `json:"verified"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
        return "", err
    }

    for _, e := range emails {
        if e.Primary && e.Verified {
            return e.Email, nil
        }
    }

    return "", fmt.Errorf("no primary verified email found")
}
```

---

## Middleware

### Logger Middleware

```go
// internal/middleware/logger.go
package middleware

import (
    "log/slog"
    "net/http"
    "time"
)

type responseWriter struct {
    http.ResponseWriter
    status int
    size   int
}

func (rw *responseWriter) WriteHeader(status int) {
    rw.status = status
    rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
    size, err := rw.ResponseWriter.Write(b)
    rw.size += size
    return size, err
}

func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()

            rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
            next.ServeHTTP(rw, r)

            logger.Info("request",
                "method", r.Method,
                "path", r.URL.Path,
                "status", rw.status,
                "size", rw.size,
                "duration", time.Since(start),
                "ip", r.RemoteAddr,
            )
        })
    }
}
```

### Recovery Middleware

```go
// internal/middleware/recovery.go
package middleware

import (
    "log/slog"
    "net/http"
    "runtime/debug"
)

func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if err := recover(); err != nil {
                    logger.Error("panic recovered",
                        "error", err,
                        "path", r.URL.Path,
                        "stack", string(debug.Stack()),
                    )

                    http.Error(w, "Internal Server Error", http.StatusInternalServerError)
                }
            }()

            next.ServeHTTP(w, r)
        })
    }
}
```

### Session Middleware

```go
// internal/middleware/session.go
package middleware

import (
    "context"
    "net/http"

    "github.com/vangoframework/rhone/internal/auth"
)

type contextKey string

const SessionContextKey contextKey = "session"

func Session(store *auth.SessionStore) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            session, err := store.Get(r)
            if err == nil && session != nil {
                ctx := context.WithValue(r.Context(), SessionContextKey, session)
                r = r.WithContext(ctx)
            }

            next.ServeHTTP(w, r)
        })
    }
}

// GetSession retrieves the session from context
func GetSession(ctx context.Context) *auth.SessionData {
    session, ok := ctx.Value(SessionContextKey).(*auth.SessionData)
    if !ok {
        return nil
    }
    return session
}
```

### Auth Middleware

```go
// internal/middleware/auth.go
package middleware

import (
    "net/http"
)

// RequireAuth redirects unauthenticated users to login
func RequireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        session := GetSession(r.Context())
        if session == nil {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }

        next.ServeHTTP(w, r)
    })
}

// OptionalAuth allows both authenticated and unauthenticated access
func OptionalAuth(next http.Handler) http.Handler {
    return next // Session already loaded by Session middleware
}
```

---

## Handlers

### Handler Dependencies

```go
// internal/handlers/handlers.go
package handlers

import (
    "log/slog"

    "github.com/vangoframework/rhone/internal/auth"
    "github.com/vangoframework/rhone/internal/config"
    "github.com/vangoframework/rhone/internal/database"
    "github.com/vangoframework/rhone/internal/database/queries"
)

type Handlers struct {
    config   *config.Config
    db       *database.DB
    queries  *queries.Queries
    sessions *auth.SessionStore
    github   *auth.GitHubOAuth
    logger   *slog.Logger
}

func New(
    cfg *config.Config,
    db *database.DB,
    sessions *auth.SessionStore,
    github *auth.GitHubOAuth,
    logger *slog.Logger,
) *Handlers {
    return &Handlers{
        config:   cfg,
        db:       db,
        queries:  queries.New(db.Pool),
        sessions: sessions,
        github:   github,
        logger:   logger,
    }
}
```

### Home Handler

```go
// internal/handlers/home.go
package handlers

import (
    "net/http"

    "github.com/vangoframework/rhone/internal/middleware"
    "github.com/vangoframework/rhone/internal/templates/pages"
)

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
    session := middleware.GetSession(r.Context())

    // Render dashboard or landing page based on auth state
    if session != nil {
        pages.Dashboard(session).Render(r.Context(), w)
    } else {
        pages.Landing().Render(r.Context(), w)
    }
}
```

### Auth Handlers

```go
// internal/handlers/auth.go
package handlers

import (
    "crypto/rand"
    "encoding/hex"
    "net/http"

    "github.com/google/uuid"
    "github.com/vangoframework/rhone/internal/auth"
    "github.com/vangoframework/rhone/internal/templates/pages"
)

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
    pages.Login().Render(r.Context(), w)
}

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
        Email:          toNullString(githubUser.Email),
        AvatarUrl:      toNullString(githubUser.AvatarURL),
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
        UserID:    user.ID,
        Email:     user.Email,
        Username:  user.GithubUsername,
        AvatarURL: nullStringValue(user.AvatarUrl),
        TeamID:    team.ID,
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

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
    h.sessions.Clear(w)
    http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Helper to get or create a personal team for the user
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

func generateState() string {
    b := make([]byte, 16)
    rand.Read(b)
    return hex.EncodeToString(b)
}

func toNullString(s string) *string {
    if s == "" {
        return nil
    }
    return &s
}

func nullStringValue(s *string) string {
    if s == nil {
        return ""
    }
    return *s
}
```

---

## Templates

### Base Layout

```go
// internal/templates/layouts/base.templ
package layouts

templ Base(title string) {
    <!DOCTYPE html>
    <html lang="en" class="h-full bg-gray-50">
    <head>
        <meta charset="UTF-8"/>
        <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
        <title>{ title } | Rhone</title>
        <link rel="stylesheet" href="/static/css/styles.css"/>
        <script src="https://unpkg.com/htmx.org@1.9.10"></script>
        <script src="https://unpkg.com/htmx.org@1.9.10/dist/ext/sse.js"></script>
    </head>
    <body class="h-full" hx-boost="true">
        { children... }
    </body>
    </html>
}

templ App(title string) {
    @Base(title) {
        <div class="min-h-full">
            @Nav()
            <main class="py-10">
                <div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
                    { children... }
                </div>
            </main>
        </div>
    }
}
```

### Navigation Component

```go
// internal/templates/components/nav.templ
package components

import "github.com/vangoframework/rhone/internal/auth"

templ Nav(session *auth.SessionData) {
    <nav class="bg-white shadow-sm">
        <div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div class="flex h-16 justify-between">
                <div class="flex">
                    <div class="flex flex-shrink-0 items-center">
                        <a href="/" class="text-xl font-bold text-indigo-600">Rhone</a>
                    </div>
                    if session != nil {
                        <div class="hidden sm:ml-6 sm:flex sm:space-x-8">
                            <a href="/apps" class="inline-flex items-center border-b-2 border-transparent px-1 pt-1 text-sm font-medium text-gray-500 hover:border-gray-300 hover:text-gray-700">
                                Apps
                            </a>
                            <a href="/settings" class="inline-flex items-center border-b-2 border-transparent px-1 pt-1 text-sm font-medium text-gray-500 hover:border-gray-300 hover:text-gray-700">
                                Settings
                            </a>
                        </div>
                    }
                </div>
                <div class="flex items-center">
                    if session != nil {
                        <div class="flex items-center space-x-4">
                            <span class="text-sm text-gray-500">{ session.TeamSlug }</span>
                            <img class="h-8 w-8 rounded-full" src={ session.AvatarURL } alt={ session.Username }/>
                            <a href="/logout" class="text-sm text-gray-500 hover:text-gray-700">Logout</a>
                        </div>
                    } else {
                        <a href="/login" class="rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500">
                            Sign in with GitHub
                        </a>
                    }
                </div>
            </div>
        </div>
    </nav>
}
```

### Dashboard Page

```go
// internal/templates/pages/dashboard.templ
package pages

import (
    "github.com/vangoframework/rhone/internal/auth"
    "github.com/vangoframework/rhone/internal/templates/layouts"
    "github.com/vangoframework/rhone/internal/templates/components"
)

templ Dashboard(session *auth.SessionData) {
    @layouts.Base("Dashboard") {
        @components.Nav(session)
        <main class="py-10">
            <div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
                <div class="sm:flex sm:items-center">
                    <div class="sm:flex-auto">
                        <h1 class="text-2xl font-semibold text-gray-900">
                            Welcome, { session.Username }
                        </h1>
                        <p class="mt-2 text-sm text-gray-700">
                            Your Vango applications will appear here.
                        </p>
                    </div>
                    <div class="mt-4 sm:ml-16 sm:mt-0 sm:flex-none">
                        <a href="/apps/new" class="block rounded-md bg-indigo-600 px-3 py-2 text-center text-sm font-semibold text-white shadow-sm hover:bg-indigo-500">
                            New App
                        </a>
                    </div>
                </div>

                <!-- Empty state -->
                <div class="mt-10 text-center">
                    <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/>
                    </svg>
                    <h3 class="mt-2 text-sm font-semibold text-gray-900">No apps</h3>
                    <p class="mt-1 text-sm text-gray-500">Get started by creating a new app.</p>
                    <div class="mt-6">
                        <a href="/apps/new" class="inline-flex items-center rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500">
                            <svg class="-ml-0.5 mr-1.5 h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                                <path d="M10.75 4.75a.75.75 0 00-1.5 0v4.5h-4.5a.75.75 0 000 1.5h4.5v4.5a.75.75 0 001.5 0v-4.5h4.5a.75.75 0 000-1.5h-4.5v-4.5z"/>
                            </svg>
                            New App
                        </a>
                    </div>
                </div>
            </div>
        </main>
    }
}
```

### Login Page

```go
// internal/templates/pages/login.templ
package pages

import "github.com/vangoframework/rhone/internal/templates/layouts"

templ Login() {
    @layouts.Base("Login") {
        <div class="flex min-h-full flex-col justify-center py-12 sm:px-6 lg:px-8">
            <div class="sm:mx-auto sm:w-full sm:max-w-md">
                <h1 class="text-center text-3xl font-bold text-indigo-600">Rhone</h1>
                <h2 class="mt-6 text-center text-2xl font-bold leading-9 tracking-tight text-gray-900">
                    Sign in to your account
                </h2>
            </div>

            <div class="mt-10 sm:mx-auto sm:w-full sm:max-w-[480px]">
                <div class="bg-white px-6 py-12 shadow sm:rounded-lg sm:px-12">
                    <div class="space-y-6">
                        <a href="/auth/github" class="flex w-full justify-center items-center gap-3 rounded-md bg-gray-900 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900">
                            <svg class="h-5 w-5" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M10 0C4.477 0 0 4.484 0 10.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0110 4.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.203 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.942.359.31.678.921.678 1.856 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0020 10.017C20 4.484 15.522 0 10 0z" clip-rule="evenodd"/>
                            </svg>
                            Continue with GitHub
                        </a>
                    </div>
                </div>

                <p class="mt-10 text-center text-sm text-gray-500">
                    By signing in, you agree to our
                    <a href="/terms" class="font-semibold text-indigo-600 hover:text-indigo-500">Terms of Service</a>
                    and
                    <a href="/privacy" class="font-semibold text-indigo-600 hover:text-indigo-500">Privacy Policy</a>.
                </p>
            </div>
        </div>
    }
}
```

---

## Database Migrations

### Initial Migration

```sql
-- internal/database/migrations/001_initial.up.sql

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id BIGINT UNIQUE NOT NULL,
    github_username VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    avatar_url VARCHAR(500),
    stripe_customer_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Teams table
CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    stripe_subscription_id VARCHAR(255),
    plan VARCHAR(50) NOT NULL DEFAULT 'free',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Team members table
CREATE TABLE team_members (
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, user_id)
);

-- Indexes
CREATE INDEX idx_users_github_id ON users(github_id);
CREATE INDEX idx_teams_slug ON teams(slug);
CREATE INDEX idx_team_members_user_id ON team_members(user_id);

-- Updated at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply trigger to tables
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_teams_updated_at
    BEFORE UPDATE ON teams
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

```sql
-- internal/database/migrations/001_initial.down.sql

DROP TRIGGER IF EXISTS update_teams_updated_at ON teams;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS users;
```

### sqlc Queries

```sql
-- internal/database/queries/queries.sql

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByGitHubID :one
SELECT * FROM users WHERE github_id = $1;

-- name: UpsertUser :one
INSERT INTO users (github_id, github_username, email, avatar_url)
VALUES ($1, $2, $3, $4)
ON CONFLICT (github_id) DO UPDATE SET
    github_username = EXCLUDED.github_username,
    email = COALESCE(EXCLUDED.email, users.email),
    avatar_url = COALESCE(EXCLUDED.avatar_url, users.avatar_url),
    updated_at = NOW()
RETURNING *;

-- name: GetTeam :one
SELECT * FROM teams WHERE id = $1;

-- name: GetTeamBySlug :one
SELECT * FROM teams WHERE slug = $1;

-- name: CreateTeam :one
INSERT INTO teams (name, slug, plan)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserTeams :many
SELECT t.* FROM teams t
JOIN team_members tm ON t.id = tm.team_id
WHERE tm.user_id = $1
ORDER BY t.created_at;

-- name: GetTeamMember :one
SELECT * FROM team_members WHERE team_id = $1 AND user_id = $2;

-- name: AddTeamMember :one
INSERT INTO team_members (team_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTeamMembers :many
SELECT u.*, tm.role, tm.created_at as joined_at
FROM users u
JOIN team_members tm ON u.id = tm.user_id
WHERE tm.team_id = $1
ORDER BY tm.created_at;
```

---

## Main Entry Point

```go
// cmd/rhone/main.go
package main

import (
    "context"
    "log"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-chi/chi/v5"
    chimiddleware "github.com/go-chi/chi/v5/middleware"

    "github.com/vangoframework/rhone/internal/auth"
    "github.com/vangoframework/rhone/internal/config"
    "github.com/vangoframework/rhone/internal/database"
    "github.com/vangoframework/rhone/internal/handlers"
    "github.com/vangoframework/rhone/internal/middleware"
)

func main() {
    // Logger
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    slog.SetDefault(logger)

    // Load config
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("failed to load config: %v", err)
    }

    // Database
    ctx := context.Background()
    db, err := database.New(ctx, cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("failed to connect to database: %v", err)
    }
    defer db.Close()

    // Session store
    sessions := auth.NewSessionStore(
        cfg.SessionSecret,
        cfg.SessionMaxAge,
        cfg.IsProduction(),
    )

    // GitHub OAuth
    github := auth.NewGitHubOAuth(
        cfg.GitHubClientID,
        cfg.GitHubClientSecret,
        cfg.GitHubCallbackURL,
    )

    // Handlers
    h := handlers.New(cfg, db, sessions, github, logger)

    // Router
    r := chi.NewRouter()

    // Global middleware
    r.Use(chimiddleware.RequestID)
    r.Use(chimiddleware.RealIP)
    r.Use(middleware.Logger(logger))
    r.Use(middleware.Recovery(logger))
    r.Use(middleware.Session(sessions))

    // Static files
    fileServer := http.FileServer(http.Dir("static"))
    r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

    // Health check
    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        if err := db.Health(r.Context()); err != nil {
            http.Error(w, "unhealthy", http.StatusServiceUnavailable)
            return
        }
        w.Write([]byte("ok"))
    })

    // Public routes
    r.Get("/", h.Home)
    r.Get("/login", h.Login)
    r.Get("/auth/github", h.LoginStart)
    r.Get("/auth/callback", h.AuthCallback)
    r.Get("/logout", h.Logout)

    // Protected routes
    r.Group(func(r chi.Router) {
        r.Use(middleware.RequireAuth)

        r.Get("/apps", h.ListApps)
        r.Get("/apps/new", h.NewApp)
        r.Get("/settings", h.Settings)
    })

    // Server
    server := &http.Server{
        Addr:         ":" + cfg.Port,
        Handler:      r,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    // Graceful shutdown
    shutdown := make(chan os.Signal, 1)
    signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

    go func() {
        logger.Info("server starting", "port", cfg.Port)
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatalf("server error: %v", err)
        }
    }()

    <-shutdown
    logger.Info("shutting down...")

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("shutdown error: %v", err)
    }

    logger.Info("shutdown complete")
}
```

---

## Fly.io Configuration

```toml
# fly.toml
app = "rhone"
primary_region = "iad"

[build]
  dockerfile = "Dockerfile"

[env]
  PORT = "8080"
  ENVIRONMENT = "production"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true
  min_machines_running = 1

  [http_service.concurrency]
    type = "requests"
    hard_limit = 250
    soft_limit = 200

[[vm]]
  cpu_kind = "shared"
  cpus = 1
  memory_mb = 512
```

```dockerfile
# Dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /rhone ./cmd/rhone

# Runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /rhone /app/rhone
COPY static/ /app/static/

EXPOSE 8080

CMD ["/app/rhone"]
```

---

## Testing Strategy

### Unit Tests

```go
// internal/auth/session_test.go
package auth_test

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/vangoframework/rhone/internal/auth"
)

func TestSessionStore(t *testing.T) {
    store := auth.NewSessionStore(
        "01234567890123456789012345678901234567890123456789012345678901234567",
        time.Hour,
        false,
    )

    t.Run("set and get session", func(t *testing.T) {
        session := &auth.SessionData{
            UserID:   uuid.New(),
            Username: "testuser",
            Email:    "test@example.com",
        }

        // Set session
        w := httptest.NewRecorder()
        err := store.Set(w, session)
        require.NoError(t, err)

        // Get session
        cookies := w.Result().Cookies()
        require.Len(t, cookies, 1)

        req := httptest.NewRequest("GET", "/", nil)
        req.AddCookie(cookies[0])

        got, err := store.Get(req)
        require.NoError(t, err)
        assert.Equal(t, session.UserID, got.UserID)
        assert.Equal(t, session.Username, got.Username)
    })

    t.Run("expired session returns error", func(t *testing.T) {
        expiredStore := auth.NewSessionStore(
            "01234567890123456789012345678901234567890123456789012345678901234567",
            -time.Hour, // Already expired
            false,
        )

        session := &auth.SessionData{
            UserID:   uuid.New(),
            Username: "testuser",
        }

        w := httptest.NewRecorder()
        err := expiredStore.Set(w, session)
        require.NoError(t, err)

        cookies := w.Result().Cookies()
        req := httptest.NewRequest("GET", "/", nil)
        req.AddCookie(cookies[0])

        _, err = expiredStore.Get(req)
        assert.Error(t, err)
    })
}
```

### Integration Tests

```go
// internal/handlers/auth_test.go
package handlers_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestLoginRedirect(t *testing.T) {
    // Setup test server with handlers
    h := setupTestHandlers(t)
    server := httptest.NewServer(h.Router())
    defer server.Close()

    // Test login redirect
    client := &http.Client{
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse
        },
    }

    resp, err := client.Get(server.URL + "/auth/github")
    assert.NoError(t, err)
    assert.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)

    location := resp.Header.Get("Location")
    assert.Contains(t, location, "github.com/login/oauth/authorize")
}

func TestProtectedRouteRedirect(t *testing.T) {
    h := setupTestHandlers(t)
    server := httptest.NewServer(h.Router())
    defer server.Close()

    client := &http.Client{
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse
        },
    }

    resp, err := client.Get(server.URL + "/apps")
    assert.NoError(t, err)
    assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
    assert.Equal(t, "/login", resp.Header.Get("Location"))
}
```

---

## File Structure Summary

```
internal/
├── config/
│   └── config.go            # Environment configuration
├── database/
│   ├── database.go          # Connection pool management
│   ├── migrations/
│   │   ├── 001_initial.up.sql
│   │   └── 001_initial.down.sql
│   └── queries/
│       ├── queries.sql      # sqlc input
│       ├── db.go            # Generated
│       ├── models.go        # Generated
│       └── queries.sql.go   # Generated
├── auth/
│   ├── github_oauth.go      # GitHub OAuth client
│   └── session.go           # Cookie session store
├── middleware/
│   ├── logger.go            # Request logging
│   ├── recovery.go          # Panic recovery
│   ├── session.go           # Session loading
│   └── auth.go              # Auth requirement
├── handlers/
│   ├── handlers.go          # Dependency container
│   ├── home.go              # Dashboard handler
│   └── auth.go              # Auth handlers
└── templates/
    ├── layouts/
    │   └── base.templ       # Base HTML layout
    ├── pages/
    │   ├── dashboard.templ  # Dashboard page
    │   └── login.templ      # Login page
    └── components/
        └── nav.templ        # Navigation
```

---

## Exit Criteria

Phase 1 is complete when:

1. [ ] Go server starts and serves HTTP requests
2. [ ] Chi router handles all defined routes
3. [ ] Neon Postgres connection works
4. [ ] Migrations run successfully
5. [ ] GitHub OAuth login flow works end-to-end
6. [ ] Session persists across requests
7. [ ] Protected routes redirect to login
8. [ ] Dashboard renders for authenticated users
9. [ ] Templ templates compile and render
10. [ ] HTMX enhancement works (hx-boost)
11. [ ] Static files served correctly
12. [ ] Health check endpoint works
13. [ ] Rhone deploys to Fly.io
14. [ ] Unit tests pass
15. [ ] Integration tests pass

---

## Dependencies

- **Requires**: Nothing (this is the foundation)
- **Required by**: All subsequent phases

---

*Phase 1 Specification - Version 1.0*
