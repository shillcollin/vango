# Phase 2: GitHub Integration

> **GitHub App installation and repository access**

**Status**: Not Started

---

## Overview

Phase 2 implements the GitHub App integration that allows Rhone to access user repositories. This is distinct from GitHub OAuth (Phase 1) which handles user authentication. The GitHub App provides scoped, temporary access tokens for cloning repositories during builds.

### Goals

1. **GitHub App installation flow**: Users can install the Rhone GitHub App
2. **Repository listing**: List accessible repositories for the user
3. **Installation token exchange**: Get temporary tokens for repo access
4. **Repository selector UI**: Interactive repo selection component

### Non-Goals

1. Webhook handling (Phase 10)
2. Commit status updates (Phase 10)
3. Pull request comments
4. Repository write access

---

## Why GitHub App (Not OAuth)?

```
┌─────────────────────────────────────────────────────────────────────────┐
│                   OAUTH vs GITHUB APP                                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  OAUTH TOKEN                          GITHUB APP TOKEN                   │
│  ───────────────────────────          ───────────────────────────        │
│  • Access to ALL user repos           • Access to SELECTED repos only    │
│  • Long-lived token                   • Short-lived token (1 hour)       │
│  • Single permission set              • Per-installation permissions     │
│  • User must trust completely         • User controls scope precisely    │
│                                                                          │
│  Example:                             Example:                           │
│  User has 50 private repos            User installs on 2 repos           │
│  OAuth: We see all 50                 App: We see only those 2           │
│                                                                          │
│  SECURITY IMPLICATION:                SECURITY IMPLICATION:              │
│  If token leaks, ALL repos exposed    If token leaks, only selected     │
│                                       repos exposed                      │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      GITHUB APP FLOW                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. USER INSTALLS APP                                                    │
│     ┌─────────────────────────────────────────────────────────────────┐ │
│     │ User clicks "Connect GitHub" in Rhone                           │ │
│     │                    │                                             │ │
│     │                    ▼                                             │ │
│     │ Redirect to: github.com/apps/rhone-cloud/installations/new      │ │
│     │                    │                                             │ │
│     │                    ▼                                             │ │
│     │ User selects account/org and repos                              │ │
│     │                    │                                             │ │
│     │                    ▼                                             │ │
│     │ GitHub redirects to: rhone.app/github/callback?installation_id= │ │
│     └─────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  2. RHONE STORES INSTALLATION                                           │
│     ┌─────────────────────────────────────────────────────────────────┐ │
│     │ Receive installation_id from callback                           │ │
│     │                    │                                             │ │
│     │                    ▼                                             │ │
│     │ Store in database: github_installations table                   │ │
│     │   - installation_id                                             │ │
│     │   - team_id (which team this belongs to)                        │ │
│     │   - account_type (User or Organization)                         │ │
│     │   - account_login (username or org name)                        │ │
│     └─────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  3. GETTING REPO ACCESS (at deploy time)                                │
│     ┌─────────────────────────────────────────────────────────────────┐ │
│     │ User deploys repo "myuser/myapp"                                │ │
│     │                    │                                             │ │
│     │                    ▼                                             │ │
│     │ Find installation for this repo                                 │ │
│     │                    │                                             │ │
│     │                    ▼                                             │ │
│     │ POST /app/installations/{id}/access_tokens                      │ │
│     │   - Authenticated with GitHub App private key (JWT)             │ │
│     │   - Returns temporary token (1 hour)                            │ │
│     │                    │                                             │ │
│     │                    ▼                                             │ │
│     │ Clone using: https://x-access-token:{token}@github.com/...     │ │
│     └─────────────────────────────────────────────────────────────────┘ │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## GitHub App Configuration

### Creating the App

Create at: `https://github.com/settings/apps/new`

```yaml
Name: Rhone Cloud
Description: Deploy Vango apps to the cloud

Homepage URL: https://rhone.app
Callback URL: https://rhone.app/github/callback
Setup URL: https://rhone.app/github/setup (optional)
Webhook URL: https://rhone.app/webhooks/github
Webhook Secret: [generate secure random string]

Permissions:
  Repository:
    Contents: Read-only        # Clone repos
    Metadata: Read-only        # List repos

  Account:
    # None needed

Subscribe to events:
  - Push                       # For auto-deploy (Phase 10)

Where can this app be installed?
  - Any account               # Allow anyone to use Rhone
```

### Environment Variables

```bash
# GitHub App credentials
GITHUB_APP_ID=123456
GITHUB_APP_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"
GITHUB_APP_WEBHOOK_SECRET=whsec_...

# Note: Private key can also be base64 encoded for easier env var handling
GITHUB_APP_PRIVATE_KEY_BASE64=LS0tLS1CRUdJTi...
```

---

## Database Schema

```sql
-- internal/database/migrations/002_github_apps.up.sql

-- GitHub App installations
CREATE TABLE github_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    installation_id BIGINT UNIQUE NOT NULL,
    account_type VARCHAR(50) NOT NULL,  -- 'User' or 'Organization'
    account_login VARCHAR(255) NOT NULL,
    account_id BIGINT NOT NULL,
    suspended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for looking up installations by team
CREATE INDEX idx_github_installations_team_id ON github_installations(team_id);

-- Index for looking up by account (to find which team owns an account)
CREATE INDEX idx_github_installations_account ON github_installations(account_login);

-- Trigger for updated_at
CREATE TRIGGER update_github_installations_updated_at
    BEFORE UPDATE ON github_installations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

```sql
-- internal/database/migrations/002_github_apps.down.sql

DROP TABLE IF EXISTS github_installations;
```

---

## Core Types

### GitHub App Client

```go
// internal/auth/github_app.go
package auth

import (
    "context"
    "crypto/rsa"
    "crypto/x509"
    "encoding/base64"
    "encoding/json"
    "encoding/pem"
    "fmt"
    "io"
    "net/http"
    "strconv"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

type GitHubApp struct {
    AppID      int64
    PrivateKey *rsa.PrivateKey
    httpClient *http.Client
}

func NewGitHubApp(appID int64, privateKeyPEM string) (*GitHubApp, error) {
    // Handle base64-encoded key
    decoded, err := base64.StdEncoding.DecodeString(privateKeyPEM)
    if err == nil {
        privateKeyPEM = string(decoded)
    }

    block, _ := pem.Decode([]byte(privateKeyPEM))
    if block == nil {
        return nil, fmt.Errorf("failed to parse PEM block")
    }

    key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
    if err != nil {
        // Try PKCS8 format
        pkcs8Key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
        if err != nil {
            return nil, fmt.Errorf("failed to parse private key: %w", err)
        }
        var ok bool
        key, ok = pkcs8Key.(*rsa.PrivateKey)
        if !ok {
            return nil, fmt.Errorf("expected RSA private key")
        }
    }

    return &GitHubApp{
        AppID:      appID,
        PrivateKey: key,
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }, nil
}

// GenerateJWT creates a JWT for authenticating as the GitHub App
func (g *GitHubApp) GenerateJWT() (string, error) {
    now := time.Now()

    claims := jwt.MapClaims{
        "iat": now.Add(-60 * time.Second).Unix(), // 60 seconds in the past
        "exp": now.Add(10 * time.Minute).Unix(),  // 10 minutes max
        "iss": strconv.FormatInt(g.AppID, 10),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    return token.SignedString(g.PrivateKey)
}

// Installation represents a GitHub App installation
type Installation struct {
    ID              int64     `json:"id"`
    Account         Account   `json:"account"`
    RepositorySelection string `json:"repository_selection"` // "all" or "selected"
    AccessTokensURL string    `json:"access_tokens_url"`
    SuspendedAt     *time.Time `json:"suspended_at"`
}

type Account struct {
    ID    int64  `json:"id"`
    Login string `json:"login"`
    Type  string `json:"type"` // "User" or "Organization"
}

// GetInstallation fetches an installation by ID
func (g *GitHubApp) GetInstallation(ctx context.Context, installationID int64) (*Installation, error) {
    jwt, err := g.GenerateJWT()
    if err != nil {
        return nil, err
    }

    url := fmt.Sprintf("https://api.github.com/app/installations/%d", installationID)
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+jwt)
    req.Header.Set("Accept", "application/vnd.github+json")
    req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

    resp, err := g.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("github api error %d: %s", resp.StatusCode, string(body))
    }

    var installation Installation
    if err := json.NewDecoder(resp.Body).Decode(&installation); err != nil {
        return nil, err
    }

    return &installation, nil
}

// InstallationToken is a temporary access token for an installation
type InstallationToken struct {
    Token       string    `json:"token"`
    ExpiresAt   time.Time `json:"expires_at"`
    Permissions map[string]string `json:"permissions"`
}

// GetInstallationToken exchanges installation ID for a temporary access token
func (g *GitHubApp) GetInstallationToken(ctx context.Context, installationID int64) (*InstallationToken, error) {
    jwt, err := g.GenerateJWT()
    if err != nil {
        return nil, err
    }

    url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
    req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+jwt)
    req.Header.Set("Accept", "application/vnd.github+json")
    req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

    resp, err := g.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("github api error %d: %s", resp.StatusCode, string(body))
    }

    var token InstallationToken
    if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
        return nil, err
    }

    return &token, nil
}

// Repository represents a GitHub repository
type Repository struct {
    ID          int64  `json:"id"`
    Name        string `json:"name"`
    FullName    string `json:"full_name"`
    Private     bool   `json:"private"`
    Description string `json:"description"`
    DefaultBranch string `json:"default_branch"`
    CloneURL    string `json:"clone_url"`
    HTMLURL     string `json:"html_url"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// ListInstallationRepos lists all repositories accessible to an installation
func (g *GitHubApp) ListInstallationRepos(ctx context.Context, installationID int64) ([]Repository, error) {
    token, err := g.GetInstallationToken(ctx, installationID)
    if err != nil {
        return nil, err
    }

    var allRepos []Repository
    page := 1

    for {
        url := fmt.Sprintf("https://api.github.com/installation/repositories?per_page=100&page=%d", page)
        req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
        if err != nil {
            return nil, err
        }

        req.Header.Set("Authorization", "Bearer "+token.Token)
        req.Header.Set("Accept", "application/vnd.github+json")
        req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

        resp, err := g.httpClient.Do(req)
        if err != nil {
            return nil, err
        }

        var result struct {
            TotalCount   int          `json:"total_count"`
            Repositories []Repository `json:"repositories"`
        }

        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            resp.Body.Close()
            return nil, err
        }
        resp.Body.Close()

        allRepos = append(allRepos, result.Repositories...)

        if len(allRepos) >= result.TotalCount {
            break
        }
        page++
    }

    return allRepos, nil
}

// CloneURL returns the authenticated clone URL for a repository
func (g *GitHubApp) CloneURL(token, repoFullName string) string {
    return fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, repoFullName)
}
```

### Installation Repository

```go
// internal/database/queries/github.sql

-- name: CreateGitHubInstallation :one
INSERT INTO github_installations (
    team_id, installation_id, account_type, account_login, account_id
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (installation_id) DO UPDATE SET
    team_id = EXCLUDED.team_id,
    account_type = EXCLUDED.account_type,
    account_login = EXCLUDED.account_login,
    account_id = EXCLUDED.account_id,
    suspended_at = NULL,
    updated_at = NOW()
RETURNING *;

-- name: GetGitHubInstallation :one
SELECT * FROM github_installations WHERE id = $1;

-- name: GetGitHubInstallationByInstallationID :one
SELECT * FROM github_installations WHERE installation_id = $1;

-- name: GetTeamGitHubInstallations :many
SELECT * FROM github_installations
WHERE team_id = $1
ORDER BY created_at DESC;

-- name: DeleteGitHubInstallation :exec
DELETE FROM github_installations WHERE installation_id = $1;

-- name: SuspendGitHubInstallation :exec
UPDATE github_installations
SET suspended_at = NOW(), updated_at = NOW()
WHERE installation_id = $1;

-- name: UnsuspendGitHubInstallation :exec
UPDATE github_installations
SET suspended_at = NULL, updated_at = NOW()
WHERE installation_id = $1;
```

---

## Handlers

### GitHub Installation Handlers

```go
// internal/handlers/github.go
package handlers

import (
    "net/http"
    "strconv"

    "github.com/vangoframework/rhone/internal/database/queries"
    "github.com/vangoframework/rhone/internal/middleware"
    "github.com/vangoframework/rhone/internal/templates/pages"
)

// ConnectGitHub redirects to GitHub App installation page
func (h *Handlers) ConnectGitHub(w http.ResponseWriter, r *http.Request) {
    session := middleware.GetSession(r.Context())

    // Generate state for CSRF protection
    state := generateState()

    http.SetCookie(w, &http.Cookie{
        Name:     "github_app_state",
        Value:    state,
        Path:     "/",
        MaxAge:   600,
        HttpOnly: true,
        Secure:   h.config.IsProduction(),
        SameSite: http.SameSiteLaxMode,
    })

    // Redirect to GitHub App installation
    // The state parameter is passed through for verification
    installURL := fmt.Sprintf(
        "https://github.com/apps/%s/installations/new?state=%s",
        h.config.GitHubAppSlug,
        state,
    )

    http.Redirect(w, r, installURL, http.StatusTemporaryRedirect)
}

// GitHubCallback handles the callback after GitHub App installation
func (h *Handlers) GitHubCallback(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)

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
        TeamID:        session.TeamID,
        InstallationID: installationID,
        AccountType:   installation.Account.Type,
        AccountLogin:  installation.Account.Login,
        AccountID:     installation.Account.ID,
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

// ListRepositories returns available repositories for the team
func (h *Handlers) ListRepositories(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)

    // Get all installations for the team
    installations, err := h.queries.GetTeamGitHubInstallations(ctx, session.TeamID)
    if err != nil {
        h.logger.Error("failed to get installations", "error", err)
        http.Error(w, "Failed to load repositories", http.StatusInternalServerError)
        return
    }

    // Collect repos from all installations
    var allRepos []RepoWithInstallation
    for _, inst := range installations {
        if inst.SuspendedAt != nil {
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
            allRepos = append(allRepos, RepoWithInstallation{
                Repository:     repo,
                InstallationID: inst.InstallationID,
                AccountLogin:   inst.AccountLogin,
            })
        }
    }

    // Render repository list
    pages.RepositoryList(allRepos).Render(ctx, w)
}

type RepoWithInstallation struct {
    auth.Repository
    InstallationID int64
    AccountLogin   string
}

// RepoSelector renders the repository selector component (HTMX partial)
func (h *Handlers) RepoSelector(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)

    // Check if team has any GitHub installations
    installations, err := h.queries.GetTeamGitHubInstallations(ctx, session.TeamID)
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
```

---

## Templates

### GitHub Connect Prompt

```go
// internal/templates/components/github.templ
package components

templ GitHubConnectPrompt() {
    <div class="text-center py-12 bg-gray-50 rounded-lg border-2 border-dashed border-gray-300">
        <svg class="mx-auto h-12 w-12 text-gray-400" fill="currentColor" viewBox="0 0 24 24">
            <path fill-rule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.203 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.942.359.31.678.921.678 1.856 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clip-rule="evenodd"/>
        </svg>
        <h3 class="mt-4 text-lg font-semibold text-gray-900">Connect GitHub</h3>
        <p class="mt-2 text-sm text-gray-500 max-w-md mx-auto">
            Connect your GitHub account to deploy repositories. You'll choose which repositories to grant access to.
        </p>
        <div class="mt-6">
            <a href="/github/connect" class="inline-flex items-center gap-2 rounded-md bg-gray-900 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-gray-800">
                <svg class="h-5 w-5" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 0C4.477 0 0 4.484 0 10.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0110 4.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.203 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.942.359.31.678.921.678 1.856 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0020 10.017C20 4.484 15.522 0 10 0z" clip-rule="evenodd"/>
                </svg>
                Connect GitHub
            </a>
        </div>
    </div>
}
```

### Repository Selector

```go
// internal/templates/components/repo_selector.templ
package components

import (
    "fmt"
    "github.com/vangoframework/rhone/internal/handlers"
)

templ RepoSelector(repos []handlers.RepoWithInstallation, selectedRepo string) {
    <div class="space-y-4">
        <div class="flex items-center justify-between">
            <label class="block text-sm font-medium text-gray-700">
                Select Repository
            </label>
            <a href="/github/connect" class="text-sm text-indigo-600 hover:text-indigo-500">
                + Add more repositories
            </a>
        </div>

        <div class="relative">
            <input
                type="text"
                placeholder="Search repositories..."
                class="block w-full rounded-md border-0 py-2 px-3 text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-indigo-600 sm:text-sm"
                hx-get="/api/repos/search"
                hx-trigger="keyup changed delay:300ms"
                hx-target="#repo-list"
                name="q"
            />
        </div>

        <div id="repo-list" class="max-h-80 overflow-y-auto border rounded-md divide-y">
            for _, repo := range repos {
                <label class="flex items-center p-3 hover:bg-gray-50 cursor-pointer">
                    <input
                        type="radio"
                        name="repo"
                        value={ repo.FullName }
                        data-installation-id={ fmt.Sprintf("%d", repo.InstallationID) }
                        class="h-4 w-4 text-indigo-600 border-gray-300 focus:ring-indigo-600"
                        if repo.FullName == selectedRepo {
                            checked
                        }
                    />
                    <div class="ml-3 flex-1">
                        <div class="flex items-center gap-2">
                            <span class="text-sm font-medium text-gray-900">{ repo.FullName }</span>
                            if repo.Private {
                                <span class="inline-flex items-center rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600">
                                    Private
                                </span>
                            }
                        </div>
                        if repo.Description != "" {
                            <p class="text-sm text-gray-500 truncate">{ repo.Description }</p>
                        }
                    </div>
                    <span class="text-xs text-gray-400">{ repo.DefaultBranch }</span>
                </label>
            }

            if len(repos) == 0 {
                <div class="p-8 text-center text-gray-500">
                    <p>No repositories found.</p>
                    <p class="mt-2 text-sm">
                        <a href="/github/connect" class="text-indigo-600 hover:text-indigo-500">
                            Connect more repositories
                        </a>
                    </p>
                </div>
            }
        </div>
    </div>
}
```

### Settings Page with GitHub Section

```go
// internal/templates/pages/settings.templ
package pages

import (
    "github.com/vangoframework/rhone/internal/auth"
    "github.com/vangoframework/rhone/internal/database/queries"
    "github.com/vangoframework/rhone/internal/templates/layouts"
)

templ Settings(session *auth.SessionData, installations []queries.GithubInstallation) {
    @layouts.App("Settings") {
        <div class="space-y-10">
            <div>
                <h1 class="text-2xl font-semibold text-gray-900">Settings</h1>
                <p class="mt-1 text-sm text-gray-500">
                    Manage your team settings and integrations.
                </p>
            </div>

            <!-- GitHub Connections -->
            <div class="bg-white shadow rounded-lg">
                <div class="px-4 py-5 sm:p-6">
                    <div class="flex items-center justify-between">
                        <div>
                            <h3 class="text-lg font-medium text-gray-900">GitHub Connections</h3>
                            <p class="mt-1 text-sm text-gray-500">
                                Connect GitHub accounts to deploy repositories.
                            </p>
                        </div>
                        <a href="/github/connect" class="inline-flex items-center gap-2 rounded-md bg-gray-900 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-gray-800">
                            <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                                <path d="M10.75 4.75a.75.75 0 00-1.5 0v4.5h-4.5a.75.75 0 000 1.5h4.5v4.5a.75.75 0 001.5 0v-4.5h4.5a.75.75 0 000-1.5h-4.5v-4.5z"/>
                            </svg>
                            Add Connection
                        </a>
                    </div>

                    if len(installations) > 0 {
                        <ul class="mt-6 divide-y divide-gray-200">
                            for _, inst := range installations {
                                <li class="flex items-center justify-between py-4">
                                    <div class="flex items-center gap-3">
                                        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-gray-100">
                                            <svg class="h-5 w-5 text-gray-600" fill="currentColor" viewBox="0 0 20 20">
                                                <path fill-rule="evenodd" d="M10 0C4.477 0 0 4.484 0 10.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0110 4.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.203 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.942.359.31.678.921.678 1.856 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0020 10.017C20 4.484 15.522 0 10 0z" clip-rule="evenodd"/>
                                            </svg>
                                        </div>
                                        <div>
                                            <p class="text-sm font-medium text-gray-900">{ inst.AccountLogin }</p>
                                            <p class="text-sm text-gray-500">{ inst.AccountType }</p>
                                        </div>
                                    </div>
                                    <div class="flex items-center gap-2">
                                        if inst.SuspendedAt != nil {
                                            <span class="inline-flex items-center rounded-md bg-yellow-100 px-2 py-1 text-xs font-medium text-yellow-800">
                                                Suspended
                                            </span>
                                        } else {
                                            <span class="inline-flex items-center rounded-md bg-green-100 px-2 py-1 text-xs font-medium text-green-800">
                                                Active
                                            </span>
                                        }
                                        <a href={ templ.SafeURL(fmt.Sprintf("https://github.com/settings/installations/%d", inst.InstallationID)) }
                                           target="_blank"
                                           class="text-sm text-gray-500 hover:text-gray-700">
                                            Manage
                                        </a>
                                    </div>
                                </li>
                            }
                        </ul>
                    } else {
                        <div class="mt-6 text-center py-8 bg-gray-50 rounded-lg">
                            <p class="text-sm text-gray-500">No GitHub accounts connected.</p>
                            <p class="mt-1 text-sm text-gray-500">
                                Connect a GitHub account to start deploying repositories.
                            </p>
                        </div>
                    }
                </div>
            </div>
        </div>
    }
}
```

---

## Routes

```go
// Add to cmd/rhone/main.go router setup

// GitHub App routes
r.Get("/github/connect", h.ConnectGitHub)
r.Get("/github/callback", h.GitHubCallback)

// Protected routes
r.Group(func(r chi.Router) {
    r.Use(middleware.RequireAuth)

    // ... existing routes ...

    // Repository API
    r.Get("/api/repos", h.ListRepositories)
    r.Get("/api/repos/search", h.SearchRepositories)
    r.Get("/api/repos/selector", h.RepoSelector)
})
```

---

## Testing Strategy

### Unit Tests

```go
// internal/auth/github_app_test.go
package auth_test

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/vangoframework/rhone/internal/auth"
)

func TestGitHubApp_GenerateJWT(t *testing.T) {
    // Test with sample key (don't use in production!)
    testKey := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA...
-----END RSA PRIVATE KEY-----`

    app, err := auth.NewGitHubApp(123456, testKey)
    require.NoError(t, err)

    jwt, err := app.GenerateJWT()
    require.NoError(t, err)
    assert.NotEmpty(t, jwt)

    // JWT should have three parts
    parts := strings.Split(jwt, ".")
    assert.Len(t, parts, 3)
}

func TestGitHubApp_CloneURL(t *testing.T) {
    app := &auth.GitHubApp{}

    url := app.CloneURL("ghp_xxxx", "owner/repo")
    assert.Equal(t, "https://x-access-token:ghp_xxxx@github.com/owner/repo.git", url)
}
```

### Integration Tests

```go
// internal/handlers/github_test.go
package handlers_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestConnectGitHubRedirect(t *testing.T) {
    h := setupTestHandlers(t)
    server := httptest.NewServer(h.Router())
    defer server.Close()

    client := &http.Client{
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse
        },
    }

    // Login first
    session := createTestSession(t, h)

    req, _ := http.NewRequest("GET", server.URL+"/github/connect", nil)
    req.AddCookie(session.Cookie)

    resp, err := client.Do(req)
    assert.NoError(t, err)
    assert.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)

    location := resp.Header.Get("Location")
    assert.Contains(t, location, "github.com/apps")
    assert.Contains(t, location, "installations/new")
}
```

---

## Security Considerations

### Private Key Storage

```go
// The private key should NEVER be:
// - Committed to version control
// - Logged
// - Exposed in error messages

// Store as Fly.io secret:
// fly secrets set GITHUB_APP_PRIVATE_KEY="$(cat private-key.pem)"

// Or base64 encode for easier handling:
// fly secrets set GITHUB_APP_PRIVATE_KEY_BASE64="$(base64 < private-key.pem)"
```

### Token Handling

```go
// Installation tokens are short-lived (1 hour) but still sensitive
// - Never log tokens
// - Never store in database
// - Always use HTTPS for cloning
// - Clear from memory after use

func (h *Handlers) cloneRepo(ctx context.Context, installationID int64, repoFullName string) error {
    token, err := h.githubApp.GetInstallationToken(ctx, installationID)
    if err != nil {
        return err
    }

    // Use token for clone
    cloneURL := h.githubApp.CloneURL(token.Token, repoFullName)

    // Clone operation...

    // Token automatically expires after 1 hour
    // No need to explicitly revoke
}
```

---

## File Structure

```
internal/
├── auth/
│   ├── github_oauth.go      # User authentication (Phase 1)
│   ├── github_app.go        # Repository access (this phase)
│   └── session.go
├── database/
│   ├── migrations/
│   │   ├── 001_initial.up.sql
│   │   ├── 001_initial.down.sql
│   │   ├── 002_github_apps.up.sql
│   │   └── 002_github_apps.down.sql
│   └── queries/
│       ├── queries.sql
│       └── github.sql        # GitHub-specific queries
├── handlers/
│   ├── auth.go
│   └── github.go             # GitHub installation handlers
└── templates/
    ├── components/
    │   ├── github.templ      # Connect prompt
    │   └── repo_selector.templ
    └── pages/
        └── settings.templ    # Settings with GitHub section
```

---

## Exit Criteria

Phase 2 is complete when:

1. [ ] GitHub App created and configured on github.com
2. [ ] Private key securely stored as Fly.io secret
3. [ ] Installation flow redirects to GitHub correctly
4. [ ] Callback receives and stores installation_id
5. [ ] Installation tokens can be exchanged
6. [ ] Repositories can be listed for installations
7. [ ] Settings page shows connected accounts
8. [ ] Repository selector component works
9. [ ] Suspended installations are handled
10. [ ] Unit tests pass
11. [ ] Integration tests pass

---

## Dependencies

- **Requires**: Phase 1 (authentication, database)
- **Required by**: Phase 3 (app creation needs repo selection), Phase 4 (builds need repo access)

---

*Phase 2 Specification - Version 1.0*
