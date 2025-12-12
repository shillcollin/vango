# Phase 3: App Management

> **Creating, configuring, and managing Vango applications**

**Status**: Not Started

---

## Overview

Phase 3 implements the core application management features. Users can create apps, configure environment variables, manage settings, and view app details. This phase creates the "App" entity that everything else (builds, deployments, billing) revolves around.

### Goals

1. **App CRUD**: Create, read, update, delete applications
2. **Slug system**: URL-friendly unique identifiers for apps
3. **Environment variables**: Encrypted storage and management
4. **Settings UI**: Configure app settings (branch, auto-deploy, etc.)
5. **App dashboard**: Overview of app status and recent activity

### Non-Goals

1. Build and deployment (Phases 4-5)
2. Custom domains (Phase 6)
3. Usage/billing tracking (Phase 8)
4. Logs (Phase 9)

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      APP MANAGEMENT FLOW                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  CREATE APP                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ User clicks "New App"                                               ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ Select repository (from Phase 2)                                    ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ Enter app name → Generate slug                                      ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ Configure branch (default: main)                                    ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ Create app record in database                                       ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ Redirect to app dashboard                                           ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  APP ENTITY                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │                                                                      ││
│  │  App                                                                 ││
│  │  ├── id (UUID)                                                      ││
│  │  ├── team_id (owner)                                                ││
│  │  ├── name ("My App")                                                ││
│  │  ├── slug ("my-app") ──▶ my-app.rhone.app                          ││
│  │  ├── github_repo ("user/repo")                                      ││
│  │  ├── github_branch ("main")                                         ││
│  │  ├── github_installation_id                                         ││
│  │  ├── fly_app_id (created on first deploy)                          ││
│  │  ├── region ("iad")                                                 ││
│  │  ├── auto_deploy (true/false)                                       ││
│  │  ├── env_vars (encrypted)                                           ││
│  │  └── created_at, updated_at                                         ││
│  │                                                                      ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Database Schema

```sql
-- internal/database/migrations/003_apps.up.sql

-- Apps table
CREATE TABLE apps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(63) NOT NULL,  -- Fly app names max 63 chars
    github_repo VARCHAR(500),
    github_branch VARCHAR(255) DEFAULT 'main',
    github_installation_id BIGINT REFERENCES github_installations(installation_id),
    fly_app_id VARCHAR(255),
    region VARCHAR(10) DEFAULT 'iad',
    auto_deploy BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, slug)
);

-- Environment variables (encrypted)
CREATE TABLE env_vars (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    key VARCHAR(255) NOT NULL,
    value_encrypted BYTEA NOT NULL,
    nonce BYTEA NOT NULL,  -- For AES-GCM
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (app_id, key)
);

-- Indexes
CREATE INDEX idx_apps_team_id ON apps(team_id);
CREATE INDEX idx_apps_slug ON apps(slug);
CREATE INDEX idx_apps_github_repo ON apps(github_repo);
CREATE INDEX idx_env_vars_app_id ON env_vars(app_id);

-- Triggers
CREATE TRIGGER update_apps_updated_at
    BEFORE UPDATE ON apps
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_env_vars_updated_at
    BEFORE UPDATE ON env_vars
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

```sql
-- internal/database/migrations/003_apps.down.sql

DROP TABLE IF EXISTS env_vars;
DROP TABLE IF EXISTS apps;
```

---

## Core Types

### App Domain Model

```go
// internal/domain/app.go
package domain

import (
    "regexp"
    "strings"
    "time"
    "unicode"

    "github.com/google/uuid"
)

type App struct {
    ID                    uuid.UUID
    TeamID                uuid.UUID
    Name                  string
    Slug                  string
    GitHubRepo            string
    GitHubBranch          string
    GitHubInstallationID  int64
    FlyAppID              string
    Region                string
    AutoDeploy            bool
    CreatedAt             time.Time
    UpdatedAt             time.Time
}

// Slug validation rules:
// - 3-63 characters
// - Lowercase alphanumeric and hyphens only
// - Must start and end with alphanumeric
// - No consecutive hyphens

var slugRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{1,61}[a-z0-9])?$`)

func ValidateSlug(slug string) error {
    if len(slug) < 3 {
        return ErrSlugTooShort
    }
    if len(slug) > 63 {
        return ErrSlugTooLong
    }
    if !slugRegex.MatchString(slug) {
        return ErrSlugInvalid
    }
    if strings.Contains(slug, "--") {
        return ErrSlugConsecutiveHyphens
    }
    return nil
}

// GenerateSlug creates a URL-safe slug from a name
func GenerateSlug(name string) string {
    // Lowercase
    slug := strings.ToLower(name)

    // Replace spaces with hyphens
    slug = strings.ReplaceAll(slug, " ", "-")

    // Remove non-alphanumeric except hyphens
    var result strings.Builder
    for _, r := range slug {
        if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
            result.WriteRune(r)
        }
    }
    slug = result.String()

    // Remove consecutive hyphens
    for strings.Contains(slug, "--") {
        slug = strings.ReplaceAll(slug, "--", "-")
    }

    // Trim hyphens from start/end
    slug = strings.Trim(slug, "-")

    // Truncate to 63 chars
    if len(slug) > 63 {
        slug = slug[:63]
        slug = strings.TrimRight(slug, "-")
    }

    // Ensure minimum length
    if len(slug) < 3 {
        slug = slug + "-app"
    }

    return slug
}

// Domain errors
var (
    ErrSlugTooShort           = errors.New("slug must be at least 3 characters")
    ErrSlugTooLong            = errors.New("slug must be at most 63 characters")
    ErrSlugInvalid            = errors.New("slug must contain only lowercase letters, numbers, and hyphens")
    ErrSlugConsecutiveHyphens = errors.New("slug cannot contain consecutive hyphens")
    ErrSlugTaken              = errors.New("slug is already taken")
    ErrAppNotFound            = errors.New("app not found")
)
```

### Environment Variable Encryption

```go
// internal/domain/env_var.go
package domain

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "io"
    "regexp"

    "github.com/google/uuid"
)

type EnvVar struct {
    ID        uuid.UUID
    AppID     uuid.UUID
    Key       string
    Value     string  // Decrypted value (never stored)
}

// EnvVarCrypto handles encryption/decryption of env var values
type EnvVarCrypto struct {
    key []byte // 32-byte key for AES-256
}

func NewEnvVarCrypto(key string) (*EnvVarCrypto, error) {
    // Key should be 32 bytes for AES-256
    keyBytes := []byte(key)
    if len(keyBytes) != 32 {
        return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(keyBytes))
    }
    return &EnvVarCrypto{key: keyBytes}, nil
}

// Encrypt encrypts a value using AES-256-GCM
func (c *EnvVarCrypto) Encrypt(plaintext string) (ciphertext, nonce []byte, err error) {
    block, err := aes.NewCipher(c.key)
    if err != nil {
        return nil, nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, nil, err
    }

    // Generate random nonce
    nonce = make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, nil, err
    }

    // Encrypt
    ciphertext = gcm.Seal(nil, nonce, []byte(plaintext), nil)
    return ciphertext, nonce, nil
}

// Decrypt decrypts a value using AES-256-GCM
func (c *EnvVarCrypto) Decrypt(ciphertext, nonce []byte) (string, error) {
    block, err := aes.NewCipher(c.key)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }

    return string(plaintext), nil
}

// ValidateEnvKey validates an environment variable key
var envKeyRegex = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func ValidateEnvKey(key string) error {
    if len(key) == 0 {
        return fmt.Errorf("key cannot be empty")
    }
    if len(key) > 255 {
        return fmt.Errorf("key cannot exceed 255 characters")
    }
    if !envKeyRegex.MatchString(key) {
        return fmt.Errorf("key must be uppercase letters, numbers, and underscores, starting with a letter")
    }
    return nil
}

// Reserved keys that cannot be set by users
var reservedEnvKeys = map[string]bool{
    "PORT":           true,
    "FLY_APP_NAME":   true,
    "FLY_REGION":     true,
    "FLY_MACHINE_ID": true,
    "FLY_ALLOC_ID":   true,
    "PRIMARY_REGION": true,
}

func IsReservedEnvKey(key string) bool {
    return reservedEnvKeys[key]
}
```

---

## Database Queries

```sql
-- internal/database/queries/apps.sql

-- name: CreateApp :one
INSERT INTO apps (team_id, name, slug, github_repo, github_branch, github_installation_id, region, auto_deploy)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetApp :one
SELECT * FROM apps WHERE id = $1;

-- name: GetAppBySlug :one
SELECT * FROM apps WHERE team_id = $1 AND slug = $2;

-- name: GetTeamApps :many
SELECT * FROM apps WHERE team_id = $1 ORDER BY created_at DESC;

-- name: UpdateApp :one
UPDATE apps SET
    name = COALESCE(sqlc.narg('name'), name),
    github_branch = COALESCE(sqlc.narg('github_branch'), github_branch),
    region = COALESCE(sqlc.narg('region'), region),
    auto_deploy = COALESCE(sqlc.narg('auto_deploy'), auto_deploy),
    fly_app_id = COALESCE(sqlc.narg('fly_app_id'), fly_app_id),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteApp :exec
DELETE FROM apps WHERE id = $1;

-- name: CheckSlugExists :one
SELECT EXISTS(SELECT 1 FROM apps WHERE team_id = $1 AND slug = $2);

-- name: GetAppByGitHubRepo :many
SELECT * FROM apps WHERE github_repo = $1;

-- Environment variables

-- name: CreateEnvVar :one
INSERT INTO env_vars (app_id, key, value_encrypted, nonce)
VALUES ($1, $2, $3, $4)
ON CONFLICT (app_id, key) DO UPDATE SET
    value_encrypted = EXCLUDED.value_encrypted,
    nonce = EXCLUDED.nonce,
    updated_at = NOW()
RETURNING *;

-- name: GetEnvVar :one
SELECT * FROM env_vars WHERE app_id = $1 AND key = $2;

-- name: GetAppEnvVars :many
SELECT * FROM env_vars WHERE app_id = $1 ORDER BY key;

-- name: DeleteEnvVar :exec
DELETE FROM env_vars WHERE app_id = $1 AND key = $2;

-- name: DeleteAllEnvVars :exec
DELETE FROM env_vars WHERE app_id = $1;
```

---

## Handlers

### App Handlers

```go
// internal/handlers/apps.go
package handlers

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/vangoframework/rhone/internal/database/queries"
    "github.com/vangoframework/rhone/internal/domain"
    "github.com/vangoframework/rhone/internal/middleware"
    "github.com/vangoframework/rhone/internal/templates/pages"
)

func (h *Handlers) ListApps(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)

    apps, err := h.queries.GetTeamApps(ctx, session.TeamID)
    if err != nil {
        h.logger.Error("failed to get apps", "error", err)
        http.Error(w, "Failed to load apps", http.StatusInternalServerError)
        return
    }

    pages.AppList(session, apps).Render(ctx, w)
}

func (h *Handlers) NewApp(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)

    // Check for GitHub installations
    installations, err := h.queries.GetTeamGitHubInstallations(ctx, session.TeamID)
    if err != nil {
        h.logger.Error("failed to get installations", "error", err)
    }

    pages.NewApp(session, installations).Render(ctx, w)
}

func (h *Handlers) CreateApp(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)

    // Parse form
    if err := r.ParseForm(); err != nil {
        http.Error(w, "Invalid form data", http.StatusBadRequest)
        return
    }

    name := r.FormValue("name")
    repo := r.FormValue("repo")
    branch := r.FormValue("branch")
    installationIDStr := r.FormValue("installation_id")

    if name == "" || repo == "" {
        http.Error(w, "Name and repository are required", http.StatusBadRequest)
        return
    }

    // Default branch
    if branch == "" {
        branch = "main"
    }

    // Generate slug
    slug := domain.GenerateSlug(name)

    // Check if slug is taken
    exists, err := h.queries.CheckSlugExists(ctx, queries.CheckSlugExistsParams{
        TeamID: session.TeamID,
        Slug:   slug,
    })
    if err != nil {
        h.logger.Error("failed to check slug", "error", err)
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    // If slug exists, append random suffix
    if exists {
        slug = slug + "-" + randomSuffix(4)
    }

    // Parse installation ID
    installationID, _ := strconv.ParseInt(installationIDStr, 10, 64)

    // Create app
    app, err := h.queries.CreateApp(ctx, queries.CreateAppParams{
        TeamID:               session.TeamID,
        Name:                 name,
        Slug:                 slug,
        GithubRepo:           &repo,
        GithubBranch:         branch,
        GithubInstallationID: &installationID,
        Region:               "iad",
        AutoDeploy:           true,
    })
    if err != nil {
        h.logger.Error("failed to create app", "error", err)
        http.Error(w, "Failed to create app", http.StatusInternalServerError)
        return
    }

    h.logger.Info("app created",
        "app_id", app.ID,
        "slug", app.Slug,
        "repo", repo,
    )

    // Redirect to app dashboard
    http.Redirect(w, r, "/apps/"+app.Slug, http.StatusSeeOther)
}

func (h *Handlers) ShowApp(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)
    slug := chi.URLParam(r, "slug")

    app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
        TeamID: session.TeamID,
        Slug:   slug,
    })
    if err != nil {
        h.logger.Error("app not found", "slug", slug, "error", err)
        http.NotFound(w, r)
        return
    }

    // Get recent deployments (will be empty until Phase 5)
    deployments, _ := h.queries.GetAppDeployments(ctx, queries.GetAppDeploymentsParams{
        AppID: app.ID,
        Limit: 5,
    })

    pages.AppDashboard(session, app, deployments).Render(ctx, w)
}

func (h *Handlers) AppSettings(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)
    slug := chi.URLParam(r, "slug")

    app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
        TeamID: session.TeamID,
        Slug:   slug,
    })
    if err != nil {
        http.NotFound(w, r)
        return
    }

    // Get env vars (keys only, values masked)
    envVars, _ := h.queries.GetAppEnvVars(ctx, app.ID)

    pages.AppSettings(session, app, envVars).Render(ctx, w)
}

func (h *Handlers) UpdateApp(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)
    slug := chi.URLParam(r, "slug")

    app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
        TeamID: session.TeamID,
        Slug:   slug,
    })
    if err != nil {
        http.NotFound(w, r)
        return
    }

    if err := r.ParseForm(); err != nil {
        http.Error(w, "Invalid form data", http.StatusBadRequest)
        return
    }

    // Update fields
    name := r.FormValue("name")
    branch := r.FormValue("branch")
    region := r.FormValue("region")
    autoDeploy := r.FormValue("auto_deploy") == "on"

    _, err = h.queries.UpdateApp(ctx, queries.UpdateAppParams{
        ID:         app.ID,
        Name:       toNullString(name),
        Branch:     toNullString(branch),
        Region:     toNullString(region),
        AutoDeploy: &autoDeploy,
    })
    if err != nil {
        h.logger.Error("failed to update app", "error", err)
        http.Error(w, "Failed to update app", http.StatusInternalServerError)
        return
    }

    http.Redirect(w, r, "/apps/"+slug+"/settings?success=updated", http.StatusSeeOther)
}

func (h *Handlers) DeleteApp(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)
    slug := chi.URLParam(r, "slug")

    app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
        TeamID: session.TeamID,
        Slug:   slug,
    })
    if err != nil {
        http.NotFound(w, r)
        return
    }

    // TODO: Delete Fly app if exists (Phase 5)

    // Delete from database
    if err := h.queries.DeleteApp(ctx, app.ID); err != nil {
        h.logger.Error("failed to delete app", "error", err)
        http.Error(w, "Failed to delete app", http.StatusInternalServerError)
        return
    }

    h.logger.Info("app deleted", "app_id", app.ID, "slug", slug)

    http.Redirect(w, r, "/apps?success=deleted", http.StatusSeeOther)
}

func randomSuffix(n int) string {
    const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
    b := make([]byte, n)
    rand.Read(b)
    for i := range b {
        b[i] = chars[int(b[i])%len(chars)]
    }
    return string(b)
}
```

### Environment Variable Handlers

```go
// internal/handlers/env_vars.go
package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/vangoframework/rhone/internal/domain"
    "github.com/vangoframework/rhone/internal/middleware"
)

type EnvVarInput struct {
    Key   string `json:"key"`
    Value string `json:"value"`
}

func (h *Handlers) SetEnvVar(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)
    slug := chi.URLParam(r, "slug")

    app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
        TeamID: session.TeamID,
        Slug:   slug,
    })
    if err != nil {
        http.NotFound(w, r)
        return
    }

    // Parse input
    var input EnvVarInput
    if r.Header.Get("Content-Type") == "application/json" {
        if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
    } else {
        input.Key = r.FormValue("key")
        input.Value = r.FormValue("value")
    }

    // Validate key
    if err := domain.ValidateEnvKey(input.Key); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Check reserved keys
    if domain.IsReservedEnvKey(input.Key) {
        http.Error(w, "Cannot set reserved environment variable", http.StatusBadRequest)
        return
    }

    // Encrypt value
    ciphertext, nonce, err := h.envCrypto.Encrypt(input.Value)
    if err != nil {
        h.logger.Error("failed to encrypt env var", "error", err)
        http.Error(w, "Encryption failed", http.StatusInternalServerError)
        return
    }

    // Store
    _, err = h.queries.CreateEnvVar(ctx, queries.CreateEnvVarParams{
        AppID:          app.ID,
        Key:            input.Key,
        ValueEncrypted: ciphertext,
        Nonce:          nonce,
    })
    if err != nil {
        h.logger.Error("failed to save env var", "error", err)
        http.Error(w, "Failed to save environment variable", http.StatusInternalServerError)
        return
    }

    h.logger.Info("env var set", "app_id", app.ID, "key", input.Key)

    // HTMX response
    if r.Header.Get("HX-Request") == "true" {
        w.Header().Set("HX-Refresh", "true")
        w.WriteHeader(http.StatusOK)
        return
    }

    http.Redirect(w, r, "/apps/"+slug+"/settings", http.StatusSeeOther)
}

func (h *Handlers) DeleteEnvVar(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)
    slug := chi.URLParam(r, "slug")
    key := chi.URLParam(r, "key")

    app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
        TeamID: session.TeamID,
        Slug:   slug,
    })
    if err != nil {
        http.NotFound(w, r)
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

    // HTMX response
    if r.Header.Get("HX-Request") == "true" {
        w.WriteHeader(http.StatusOK)
        return
    }

    http.Redirect(w, r, "/apps/"+slug+"/settings", http.StatusSeeOther)
}

// GetDecryptedEnvVars returns all env vars decrypted (for deployment)
func (h *Handlers) GetDecryptedEnvVars(appID uuid.UUID) (map[string]string, error) {
    ctx := context.Background()

    envVars, err := h.queries.GetAppEnvVars(ctx, appID)
    if err != nil {
        return nil, err
    }

    result := make(map[string]string, len(envVars))
    for _, ev := range envVars {
        value, err := h.envCrypto.Decrypt(ev.ValueEncrypted, ev.Nonce)
        if err != nil {
            return nil, fmt.Errorf("failed to decrypt %s: %w", ev.Key, err)
        }
        result[ev.Key] = value
    }

    return result, nil
}
```

---

## Templates

### App List Page

```go
// internal/templates/pages/apps.templ
package pages

import (
    "github.com/vangoframework/rhone/internal/auth"
    "github.com/vangoframework/rhone/internal/database/queries"
    "github.com/vangoframework/rhone/internal/templates/layouts"
)

templ AppList(session *auth.SessionData, apps []queries.App) {
    @layouts.App("Apps") {
        <div class="sm:flex sm:items-center">
            <div class="sm:flex-auto">
                <h1 class="text-2xl font-semibold text-gray-900">Apps</h1>
                <p class="mt-2 text-sm text-gray-700">
                    Your Vango applications.
                </p>
            </div>
            <div class="mt-4 sm:ml-16 sm:mt-0 sm:flex-none">
                <a href="/apps/new" class="block rounded-md bg-indigo-600 px-3 py-2 text-center text-sm font-semibold text-white shadow-sm hover:bg-indigo-500">
                    New App
                </a>
            </div>
        </div>

        if len(apps) > 0 {
            <div class="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                for _, app := range apps {
                    @AppCard(app)
                }
            </div>
        } else {
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
        }
    }
}

templ AppCard(app queries.App) {
    <a href={ templ.SafeURL("/apps/" + app.Slug) } class="block rounded-lg border bg-white p-6 shadow-sm hover:shadow-md transition-shadow">
        <div class="flex items-center justify-between">
            <h3 class="text-lg font-medium text-gray-900">{ app.Name }</h3>
            <span class="inline-flex items-center rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800">
                Live
            </span>
        </div>
        <p class="mt-1 text-sm text-gray-500">{ app.Slug }.rhone.app</p>
        if app.GithubRepo != nil {
            <p class="mt-2 text-sm text-gray-400">{ *app.GithubRepo }</p>
        }
    </a>
}
```

### New App Page

```go
// internal/templates/pages/new_app.templ
package pages

import (
    "github.com/vangoframework/rhone/internal/auth"
    "github.com/vangoframework/rhone/internal/database/queries"
    "github.com/vangoframework/rhone/internal/templates/layouts"
    "github.com/vangoframework/rhone/internal/templates/components"
)

templ NewApp(session *auth.SessionData, installations []queries.GithubInstallation) {
    @layouts.App("New App") {
        <div class="max-w-2xl mx-auto">
            <h1 class="text-2xl font-semibold text-gray-900">Create New App</h1>
            <p class="mt-2 text-sm text-gray-500">
                Deploy a Vango application from a GitHub repository.
            </p>

            <form method="POST" action="/apps" class="mt-8 space-y-6">
                <!-- App Name -->
                <div>
                    <label for="name" class="block text-sm font-medium text-gray-700">
                        App Name
                    </label>
                    <input
                        type="text"
                        name="name"
                        id="name"
                        required
                        placeholder="My Awesome App"
                        class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                        hx-get="/api/slug/preview"
                        hx-trigger="keyup changed delay:300ms"
                        hx-target="#slug-preview"
                    />
                    <p id="slug-preview" class="mt-1 text-sm text-gray-500"></p>
                </div>

                <!-- Repository Selector -->
                <div
                    hx-get="/api/repos/selector"
                    hx-trigger="load"
                    hx-swap="innerHTML"
                >
                    <div class="animate-pulse">
                        <div class="h-4 bg-gray-200 rounded w-24 mb-4"></div>
                        <div class="h-40 bg-gray-100 rounded"></div>
                    </div>
                </div>

                <!-- Hidden installation_id (set by repo selector) -->
                <input type="hidden" name="installation_id" id="installation_id"/>

                <!-- Branch -->
                <div>
                    <label for="branch" class="block text-sm font-medium text-gray-700">
                        Branch
                    </label>
                    <input
                        type="text"
                        name="branch"
                        id="branch"
                        value="main"
                        class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                    />
                </div>

                <!-- Submit -->
                <div class="flex justify-end gap-3">
                    <a href="/apps" class="rounded-md bg-white px-3 py-2 text-sm font-semibold text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 hover:bg-gray-50">
                        Cancel
                    </a>
                    <button type="submit" class="rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500">
                        Create App
                    </button>
                </div>
            </form>
        </div>
    }
}
```

### App Settings Page

```go
// internal/templates/pages/app_settings.templ
package pages

import (
    "github.com/vangoframework/rhone/internal/auth"
    "github.com/vangoframework/rhone/internal/database/queries"
    "github.com/vangoframework/rhone/internal/templates/layouts"
)

templ AppSettings(session *auth.SessionData, app queries.App, envVars []queries.EnvVar) {
    @layouts.App(app.Name + " Settings") {
        <div class="max-w-4xl">
            <!-- Header -->
            <div class="mb-8">
                <nav class="text-sm text-gray-500 mb-2">
                    <a href="/apps" class="hover:text-gray-700">Apps</a>
                    <span class="mx-2">/</span>
                    <a href={ templ.SafeURL("/apps/" + app.Slug) } class="hover:text-gray-700">{ app.Name }</a>
                    <span class="mx-2">/</span>
                    <span>Settings</span>
                </nav>
                <h1 class="text-2xl font-semibold text-gray-900">Settings</h1>
            </div>

            <!-- General Settings -->
            <div class="bg-white shadow rounded-lg mb-8">
                <div class="px-4 py-5 sm:p-6">
                    <h3 class="text-lg font-medium text-gray-900">General</h3>
                    <form method="POST" action={ templ.SafeURL("/apps/" + app.Slug + "/settings") } class="mt-6 space-y-4">
                        <div>
                            <label class="block text-sm font-medium text-gray-700">Name</label>
                            <input
                                type="text"
                                name="name"
                                value={ app.Name }
                                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                            />
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700">Branch</label>
                            <input
                                type="text"
                                name="branch"
                                value={ app.GithubBranch }
                                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                            />
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700">Region</label>
                            <select name="region" class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm">
                                <option value="iad" selected?={ app.Region == "iad" }>Ashburn, Virginia (iad)</option>
                                <option value="ord" selected?={ app.Region == "ord" }>Chicago (ord)</option>
                                <option value="sea" selected?={ app.Region == "sea" }>Seattle (sea)</option>
                                <option value="lax" selected?={ app.Region == "lax" }>Los Angeles (lax)</option>
                                <option value="lhr" selected?={ app.Region == "lhr" }>London (lhr)</option>
                                <option value="fra" selected?={ app.Region == "fra" }>Frankfurt (fra)</option>
                                <option value="sin" selected?={ app.Region == "sin" }>Singapore (sin)</option>
                                <option value="syd" selected?={ app.Region == "syd" }>Sydney (syd)</option>
                            </select>
                        </div>
                        <div class="flex items-center">
                            <input
                                type="checkbox"
                                name="auto_deploy"
                                id="auto_deploy"
                                checked?={ app.AutoDeploy }
                                class="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-600"
                            />
                            <label for="auto_deploy" class="ml-2 text-sm text-gray-700">
                                Auto-deploy on push to branch
                            </label>
                        </div>
                        <div class="pt-4">
                            <button type="submit" class="rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500">
                                Save Changes
                            </button>
                        </div>
                    </form>
                </div>
            </div>

            <!-- Environment Variables -->
            <div class="bg-white shadow rounded-lg mb-8">
                <div class="px-4 py-5 sm:p-6">
                    <div class="flex items-center justify-between">
                        <h3 class="text-lg font-medium text-gray-900">Environment Variables</h3>
                        <button
                            type="button"
                            class="text-sm text-indigo-600 hover:text-indigo-500"
                            hx-get={ "/apps/" + app.Slug + "/env/new" }
                            hx-target="#env-form"
                            hx-swap="innerHTML"
                        >
                            + Add Variable
                        </button>
                    </div>
                    <p class="mt-1 text-sm text-gray-500">
                        Environment variables are encrypted and available to your app at runtime.
                    </p>

                    <div id="env-form" class="mt-4"></div>

                    if len(envVars) > 0 {
                        <table class="mt-4 min-w-full divide-y divide-gray-200">
                            <thead>
                                <tr>
                                    <th class="py-3 text-left text-xs font-medium text-gray-500 uppercase">Key</th>
                                    <th class="py-3 text-left text-xs font-medium text-gray-500 uppercase">Value</th>
                                    <th class="py-3"></th>
                                </tr>
                            </thead>
                            <tbody class="divide-y divide-gray-200">
                                for _, env := range envVars {
                                    <tr>
                                        <td class="py-3 text-sm font-mono text-gray-900">{ env.Key }</td>
                                        <td class="py-3 text-sm text-gray-500">••••••••</td>
                                        <td class="py-3 text-right">
                                            <button
                                                class="text-red-600 hover:text-red-500 text-sm"
                                                hx-delete={ "/apps/" + app.Slug + "/env/" + env.Key }
                                                hx-confirm="Delete this environment variable?"
                                                hx-swap="none"
                                            >
                                                Delete
                                            </button>
                                        </td>
                                    </tr>
                                }
                            </tbody>
                        </table>
                    } else {
                        <p class="mt-4 text-sm text-gray-500 text-center py-8 bg-gray-50 rounded">
                            No environment variables set.
                        </p>
                    }
                </div>
            </div>

            <!-- Danger Zone -->
            <div class="bg-white shadow rounded-lg border-2 border-red-200">
                <div class="px-4 py-5 sm:p-6">
                    <h3 class="text-lg font-medium text-red-600">Danger Zone</h3>
                    <p class="mt-1 text-sm text-gray-500">
                        Permanently delete this app and all its data.
                    </p>
                    <div class="mt-4">
                        <button
                            type="button"
                            class="rounded-md bg-red-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-red-500"
                            hx-delete={ "/apps/" + app.Slug }
                            hx-confirm={ "Are you sure you want to delete " + app.Name + "? This action cannot be undone." }
                        >
                            Delete App
                        </button>
                    </div>
                </div>
            </div>
        </div>
    }
}
```

---

## Routes

```go
// Add to cmd/rhone/main.go

r.Group(func(r chi.Router) {
    r.Use(middleware.RequireAuth)

    // Apps
    r.Get("/apps", h.ListApps)
    r.Get("/apps/new", h.NewApp)
    r.Post("/apps", h.CreateApp)
    r.Get("/apps/{slug}", h.ShowApp)
    r.Get("/apps/{slug}/settings", h.AppSettings)
    r.Post("/apps/{slug}/settings", h.UpdateApp)
    r.Delete("/apps/{slug}", h.DeleteApp)

    // Environment variables
    r.Post("/apps/{slug}/env", h.SetEnvVar)
    r.Delete("/apps/{slug}/env/{key}", h.DeleteEnvVar)

    // API
    r.Get("/api/slug/preview", h.SlugPreview)
})
```

---

## Testing Strategy

### Unit Tests

```go
// internal/domain/app_test.go
package domain_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/vangoframework/rhone/internal/domain"
)

func TestGenerateSlug(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"simple", "My App", "my-app"},
        {"spaces", "My Cool App", "my-cool-app"},
        {"special chars", "App! @#$ Test", "app-test"},
        {"numbers", "App 123", "app-123"},
        {"long name", "This Is A Very Long Application Name That Exceeds The Maximum Length", "this-is-a-very-long-application-name-that-exceeds-the-max"},
        {"short", "ab", "ab-app"},
        {"consecutive hyphens", "My  --  App", "my-app"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := domain.GenerateSlug(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}

func TestValidateSlug(t *testing.T) {
    tests := []struct {
        slug  string
        valid bool
    }{
        {"my-app", true},
        {"my-app-123", true},
        {"ab", false},             // too short
        {"My-App", false},         // uppercase
        {"-my-app", false},        // starts with hyphen
        {"my-app-", false},        // ends with hyphen
        {"my--app", false},        // consecutive hyphens
    }

    for _, tt := range tests {
        t.Run(tt.slug, func(t *testing.T) {
            err := domain.ValidateSlug(tt.slug)
            if tt.valid {
                assert.NoError(t, err)
            } else {
                assert.Error(t, err)
            }
        })
    }
}
```

```go
// internal/domain/env_var_test.go
package domain_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/vangoframework/rhone/internal/domain"
)

func TestEnvVarCrypto(t *testing.T) {
    key := "01234567890123456789012345678901" // 32 bytes
    crypto, err := domain.NewEnvVarCrypto(key)
    require.NoError(t, err)

    plaintext := "my-secret-value"

    // Encrypt
    ciphertext, nonce, err := crypto.Encrypt(plaintext)
    require.NoError(t, err)
    assert.NotEmpty(t, ciphertext)
    assert.NotEmpty(t, nonce)
    assert.NotEqual(t, plaintext, string(ciphertext))

    // Decrypt
    decrypted, err := crypto.Decrypt(ciphertext, nonce)
    require.NoError(t, err)
    assert.Equal(t, plaintext, decrypted)
}

func TestValidateEnvKey(t *testing.T) {
    tests := []struct {
        key   string
        valid bool
    }{
        {"DATABASE_URL", true},
        {"API_KEY", true},
        {"MY_VAR_123", true},
        {"lowercase", false},
        {"123_START", false},
        {"HAS SPACE", false},
        {"", false},
    }

    for _, tt := range tests {
        t.Run(tt.key, func(t *testing.T) {
            err := domain.ValidateEnvKey(tt.key)
            if tt.valid {
                assert.NoError(t, err)
            } else {
                assert.Error(t, err)
            }
        })
    }
}
```

---

## File Structure

```
internal/
├── domain/
│   ├── app.go           # App model and slug generation
│   ├── app_test.go
│   ├── env_var.go       # Env var model and encryption
│   └── env_var_test.go
├── database/
│   ├── migrations/
│   │   ├── 003_apps.up.sql
│   │   └── 003_apps.down.sql
│   └── queries/
│       └── apps.sql
├── handlers/
│   ├── apps.go          # App CRUD handlers
│   └── env_vars.go      # Env var handlers
└── templates/
    └── pages/
        ├── apps.templ         # App list
        ├── new_app.templ      # Create app form
        ├── app_dashboard.templ # App overview
        └── app_settings.templ  # App settings
```

---

## Exit Criteria

Phase 3 is complete when:

1. [ ] Apps can be created with name and GitHub repo
2. [ ] Slugs are generated and validated correctly
3. [ ] Slugs are unique per team
4. [ ] Apps can be listed for a team
5. [ ] App details page renders correctly
6. [ ] App settings can be updated
7. [ ] Apps can be deleted
8. [ ] Environment variables can be added (encrypted)
9. [ ] Environment variables can be deleted
10. [ ] Reserved env keys are blocked
11. [ ] Unit tests pass
12. [ ] Integration tests pass

---

## Dependencies

- **Requires**: Phase 1 (auth, db), Phase 2 (GitHub repo selection)
- **Required by**: Phase 4 (build needs app), Phase 5 (deploy needs app)

---

*Phase 3 Specification - Version 1.0*
