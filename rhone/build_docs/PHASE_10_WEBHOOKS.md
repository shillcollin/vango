# Phase 10: Webhooks & Auto-Deploy

> **Automatic deployments triggered by GitHub push events**

**Status**: Not Started

---

## Overview

Phase 10 implements GitHub webhook handling for automatic deployments. When users push to their configured branch, Rhone automatically triggers a build and deploy. This also includes GitHub commit status updates and deployment notifications.

### Goals

1. **Webhook receiver**: Secure endpoint for GitHub events
2. **Auto-deploy**: Trigger builds on push to configured branch
3. **Commit status**: Update GitHub with build/deploy status
4. **Branch filtering**: Only deploy on configured branch
5. **Deployment notifications**: In-app and optional email notifications

### Non-Goals

1. Slack/Discord integrations (future)
2. Pull request preview deployments (future)
3. Manual approval gates
4. Scheduled deployments

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                       WEBHOOK FLOW ARCHITECTURE                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  GITHUB                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │                                                                     ││
│  │  Developer pushes to main                                           ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │  GitHub generates push event                                        ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │  POST https://rhone.app/webhooks/github                            ││
│  │  Headers:                                                           ││
│  │    X-GitHub-Event: push                                             ││
│  │    X-Hub-Signature-256: sha256=abc123...                           ││
│  │    X-GitHub-Delivery: unique-id                                     ││
│  │                                                                     ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                              │                                          │
│                              ▼                                          │
│  RHONE WEBHOOK HANDLER                                                  │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │                                                                     ││
│  │  1. Verify signature (HMAC-SHA256 with webhook secret)             ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │  2. Parse event payload                                             ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │  3. Find matching apps (by repository + branch)                     ││
│  │        │                                                            ││
│  │        ├──── No matching apps? Log and return 200                   ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │  4. For each matching app:                                          ││
│  │        │                                                            ││
│  │        ├─── Check auto_deploy enabled                               ││
│  │        │                                                            ││
│  │        ├─── Create deployment record                                ││
│  │        │                                                            ││
│  │        ├─── Set GitHub commit status: "pending"                     ││
│  │        │                                                            ││
│  │        └─── Queue build job                                         ││
│  │                    │                                                ││
│  └────────────────────┼────────────────────────────────────────────────┘│
│                       │                                                  │
│                       ▼                                                  │
│  BUILD & DEPLOY PIPELINE                                                │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │                                                                     ││
│  │  Build starts → Update status: "pending" (building)                 ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │  Build completes                                                    ││
│  │        │                                                            ││
│  │        ├─── Success → Deploy → Status: "success"                    ││
│  │        │                                                            ││
│  │        └─── Failure → Status: "failure"                             ││
│  │                                                                     ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                       │                                                  │
│                       ▼                                                  │
│  GITHUB COMMIT STATUS                                                   │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │                                                                     ││
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐             ││
│  │  │   pending   │───▶│  building   │───▶│   success   │             ││
│  │  │   ⏳        │    │   🔨        │    │   ✅        │             ││
│  │  └─────────────┘    └─────────────┘    └─────────────┘             ││
│  │                            │                                        ││
│  │                            ▼                                        ││
│  │                      ┌─────────────┐                                ││
│  │                      │   failure   │                                ││
│  │                      │   ❌        │                                ││
│  │                      └─────────────┘                                ││
│  │                                                                     ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Webhook Handler

```go
// internal/handlers/webhooks.go
package handlers

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
)

// GitHubWebhook handles incoming GitHub webhook events
func (h *Handlers) GitHubWebhook(w http.ResponseWriter, r *http.Request) {
    // Read body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        h.logger.Error("failed to read webhook body", "error", err)
        http.Error(w, "Bad request", http.StatusBadRequest)
        return
    }

    // Verify signature
    signature := r.Header.Get("X-Hub-Signature-256")
    if !h.verifyGitHubSignature(body, signature) {
        h.logger.Warn("invalid webhook signature",
            "delivery_id", r.Header.Get("X-GitHub-Delivery"),
        )
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // Parse event type
    eventType := r.Header.Get("X-GitHub-Event")
    deliveryID := r.Header.Get("X-GitHub-Delivery")

    h.logger.Info("received github webhook",
        "event", eventType,
        "delivery_id", deliveryID,
    )

    // Handle different event types
    switch eventType {
    case "push":
        h.handlePushEvent(r.Context(), body)
    case "installation":
        h.handleInstallationEvent(r.Context(), body)
    case "installation_repositories":
        h.handleInstallationReposEvent(r.Context(), body)
    case "ping":
        // GitHub sends ping when webhook is first created
        h.logger.Info("received ping event")
    default:
        h.logger.Debug("ignoring event type", "type", eventType)
    }

    // Always return 200 to acknowledge receipt
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

// verifyGitHubSignature verifies the webhook signature
func (h *Handlers) verifyGitHubSignature(body []byte, signature string) bool {
    if signature == "" {
        return false
    }

    // Signature format: sha256=abc123...
    parts := strings.SplitN(signature, "=", 2)
    if len(parts) != 2 || parts[0] != "sha256" {
        return false
    }

    expected, err := hex.DecodeString(parts[1])
    if err != nil {
        return false
    }

    mac := hmac.New(sha256.New, []byte(h.config.GitHubWebhookSecret))
    mac.Write(body)
    actual := mac.Sum(nil)

    return hmac.Equal(expected, actual)
}
```

---

## Push Event Handler

```go
// internal/handlers/webhooks_push.go
package handlers

import (
    "context"
    "encoding/json"
    "strings"
)

// PushEvent represents a GitHub push webhook payload
type PushEvent struct {
    Ref        string `json:"ref"`          // refs/heads/main
    Before     string `json:"before"`       // Previous commit SHA
    After      string `json:"after"`        // New commit SHA
    Created    bool   `json:"created"`      // Was branch created?
    Deleted    bool   `json:"deleted"`      // Was branch deleted?
    Forced     bool   `json:"forced"`       // Was it a force push?
    Repository struct {
        ID       int64  `json:"id"`
        FullName string `json:"full_name"` // owner/repo
        CloneURL string `json:"clone_url"`
        SSHURL   string `json:"ssh_url"`
    } `json:"repository"`
    Pusher struct {
        Name  string `json:"name"`
        Email string `json:"email"`
    } `json:"pusher"`
    Sender struct {
        ID        int64  `json:"id"`
        Login     string `json:"login"`
        AvatarURL string `json:"avatar_url"`
    } `json:"sender"`
    HeadCommit *struct {
        ID        string `json:"id"`
        Message   string `json:"message"`
        Timestamp string `json:"timestamp"`
        URL       string `json:"url"`
        Author    struct {
            Name     string `json:"name"`
            Email    string `json:"email"`
            Username string `json:"username"`
        } `json:"author"`
    } `json:"head_commit"`
    Installation struct {
        ID int64 `json:"id"`
    } `json:"installation"`
}

// handlePushEvent processes push webhook events
func (h *Handlers) handlePushEvent(ctx context.Context, body []byte) {
    var event PushEvent
    if err := json.Unmarshal(body, &event); err != nil {
        h.logger.Error("failed to parse push event", "error", err)
        return
    }

    // Ignore branch deletions
    if event.Deleted {
        h.logger.Debug("ignoring branch deletion",
            "repo", event.Repository.FullName,
            "ref", event.Ref,
        )
        return
    }

    // Extract branch name from ref (refs/heads/main -> main)
    branch := strings.TrimPrefix(event.Ref, "refs/heads/")

    h.logger.Info("processing push event",
        "repo", event.Repository.FullName,
        "branch", branch,
        "commit", event.After,
    )

    // Find apps connected to this repository and branch
    apps, err := h.queries.GetAppsByRepo(ctx, event.Repository.FullName)
    if err != nil {
        h.logger.Error("failed to find apps for repo",
            "repo", event.Repository.FullName,
            "error", err,
        )
        return
    }

    if len(apps) == 0 {
        h.logger.Debug("no apps found for repo",
            "repo", event.Repository.FullName,
        )
        return
    }

    // Process each matching app
    for _, app := range apps {
        // Check if branch matches
        if app.GitHubBranch != branch {
            h.logger.Debug("branch mismatch, skipping",
                "app", app.ID,
                "app_branch", app.GitHubBranch,
                "push_branch", branch,
            )
            continue
        }

        // Check if auto-deploy is enabled
        if !app.AutoDeploy {
            h.logger.Debug("auto-deploy disabled, skipping",
                "app", app.ID,
            )
            continue
        }

        // Trigger deployment
        h.triggerAutoDeploy(ctx, app, event)
    }
}

// triggerAutoDeploy initiates an automatic deployment
func (h *Handlers) triggerAutoDeploy(ctx context.Context, app queries.App, event PushEvent) {
    h.logger.Info("triggering auto-deploy",
        "app_id", app.ID,
        "app_name", app.Name,
        "commit", event.After,
    )

    // Extract commit info
    var commitMessage string
    if event.HeadCommit != nil {
        commitMessage = event.HeadCommit.Message
        // Truncate long messages
        if len(commitMessage) > 200 {
            commitMessage = commitMessage[:197] + "..."
        }
    }

    // Create deployment record
    deployment, err := h.queries.CreateDeployment(ctx, queries.CreateDeploymentParams{
        AppID:         app.ID,
        CommitSHA:     &event.After,
        CommitMessage: &commitMessage,
        Status:        "pending",
        Trigger:       "webhook",
    })
    if err != nil {
        h.logger.Error("failed to create deployment",
            "app_id", app.ID,
            "error", err,
        )
        return
    }

    // Set initial GitHub commit status
    if err := h.setCommitStatus(ctx, app, event.After, "pending", "Deployment queued", deployment.ID); err != nil {
        h.logger.Error("failed to set commit status",
            "error", err,
        )
    }

    // Queue build job (async)
    go func() {
        buildCtx := context.Background()
        if err := h.buildService.TriggerBuild(buildCtx, app, deployment.ID, event.After); err != nil {
            h.logger.Error("auto-deploy build failed",
                "app_id", app.ID,
                "deployment_id", deployment.ID,
                "error", err,
            )

            // Update status to failed
            h.queries.UpdateDeploymentStatus(buildCtx, queries.UpdateDeploymentStatusParams{
                ID:     deployment.ID,
                Status: "failed",
            })

            h.setCommitStatus(buildCtx, app, event.After, "failure", "Build failed", deployment.ID)
        }
    }()
}
```

---

## GitHub Commit Status API

```go
// internal/github/status.go
package github

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/google/uuid"
)

// CommitStatus represents a GitHub commit status
type CommitStatus struct {
    State       string `json:"state"`       // pending, success, failure, error
    TargetURL   string `json:"target_url"`  // Link to deployment
    Description string `json:"description"` // Short description
    Context     string `json:"context"`     // Status context (e.g., "rhone/deploy")
}

// SetCommitStatus updates the status on a GitHub commit
func (c *Client) SetCommitStatus(ctx context.Context, owner, repo, sha string, status CommitStatus) error {
    url := fmt.Sprintf("https://api.github.com/repos/%s/%s/statuses/%s", owner, repo, sha)

    body, err := json.Marshal(status)
    if err != nil {
        return err
    }

    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return err
    }

    // Use installation token for auth
    req.Header.Set("Authorization", "Bearer "+c.installationToken)
    req.Header.Set("Accept", "application/vnd.github+json")
    req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("github status API error: %s", resp.Status)
    }

    return nil
}

// Handler helper to set commit status
func (h *Handlers) setCommitStatus(ctx context.Context, app queries.App, sha, state, description string, deploymentID uuid.UUID) error {
    if app.GitHubRepo == nil {
        return fmt.Errorf("app has no github repo")
    }

    // Parse owner/repo
    parts := strings.SplitN(*app.GitHubRepo, "/", 2)
    if len(parts) != 2 {
        return fmt.Errorf("invalid repo format: %s", *app.GitHubRepo)
    }
    owner, repo := parts[0], parts[1]

    // Get installation token
    installation, err := h.queries.GetInstallationByTeam(ctx, app.TeamID)
    if err != nil {
        return fmt.Errorf("no github installation: %w", err)
    }

    token, err := h.githubApp.GetInstallationToken(ctx, installation.InstallationID)
    if err != nil {
        return fmt.Errorf("get installation token: %w", err)
    }

    // Build status
    status := github.CommitStatus{
        State:       state,
        TargetURL:   fmt.Sprintf("%s/apps/%s/deployments/%s", h.config.BaseURL, app.ID, deploymentID),
        Description: description,
        Context:     "rhone/deploy",
    }

    client := github.NewClient(token)
    return client.SetCommitStatus(ctx, owner, repo, sha, status)
}
```

---

## Build Trigger Integration

```go
// internal/build/trigger.go
package build

import (
    "context"

    "github.com/google/uuid"
    "github.com/vangoframework/rhone/internal/database/queries"
)

// TriggerBuild initiates a build for a deployment
func (s *BuildService) TriggerBuild(ctx context.Context, app queries.App, deploymentID uuid.UUID, commitSHA string) error {
    s.logger.Info("triggering build",
        "app_id", app.ID,
        "deployment_id", deploymentID,
        "commit", commitSHA,
    )

    // Update deployment status
    s.queries.UpdateDeploymentStatus(ctx, queries.UpdateDeploymentStatusParams{
        ID:     deploymentID,
        Status: "building",
    })

    // Get installation for repo access
    installation, err := s.queries.GetInstallationByTeam(ctx, app.TeamID)
    if err != nil {
        return fmt.Errorf("no github installation: %w", err)
    }

    // Get installation token
    token, err := s.githubApp.GetInstallationToken(ctx, installation.InstallationID)
    if err != nil {
        return fmt.Errorf("get installation token: %w", err)
    }

    // Run build pipeline
    result, err := s.Build(ctx, BuildRequest{
        App:          app,
        DeploymentID: deploymentID,
        CommitSHA:    commitSHA,
        GitHubToken:  token,
    })
    if err != nil {
        s.queries.UpdateDeploymentStatus(ctx, queries.UpdateDeploymentStatusParams{
            ID:     deploymentID,
            Status: "failed",
        })
        return err
    }

    // Update deployment with image
    s.queries.UpdateDeploymentImage(ctx, queries.UpdateDeploymentImageParams{
        ID:       deploymentID,
        ImageTag: &result.ImageTag,
    })

    // Trigger deploy
    return s.deployer.Deploy(ctx, app, deploymentID, result.ImageTag)
}
```

---

## Installation Events

```go
// internal/handlers/webhooks_installation.go
package handlers

import (
    "context"
    "encoding/json"
)

// InstallationEvent represents a GitHub App installation webhook
type InstallationEvent struct {
    Action       string `json:"action"` // created, deleted, suspend, unsuspend
    Installation struct {
        ID      int64  `json:"id"`
        Account struct {
            ID    int64  `json:"id"`
            Login string `json:"login"`
            Type  string `json:"type"` // User, Organization
        } `json:"account"`
    } `json:"installation"`
    Repositories []struct {
        ID       int64  `json:"id"`
        FullName string `json:"full_name"`
        Private  bool   `json:"private"`
    } `json:"repositories"`
    Sender struct {
        ID    int64  `json:"id"`
        Login string `json:"login"`
    } `json:"sender"`
}

// handleInstallationEvent processes installation webhooks
func (h *Handlers) handleInstallationEvent(ctx context.Context, body []byte) {
    var event InstallationEvent
    if err := json.Unmarshal(body, &event); err != nil {
        h.logger.Error("failed to parse installation event", "error", err)
        return
    }

    h.logger.Info("processing installation event",
        "action", event.Action,
        "installation_id", event.Installation.ID,
        "account", event.Installation.Account.Login,
    )

    switch event.Action {
    case "created":
        // Installation created - we might already have it from OAuth flow
        // Just log for now
        h.logger.Info("github app installed",
            "installation_id", event.Installation.ID,
            "account", event.Installation.Account.Login,
            "repos_count", len(event.Repositories),
        )

    case "deleted":
        // Installation removed - mark it as inactive
        err := h.queries.DeactivateInstallation(ctx, event.Installation.ID)
        if err != nil {
            h.logger.Error("failed to deactivate installation",
                "installation_id", event.Installation.ID,
                "error", err,
            )
        }

        // Disable auto-deploy for all apps using this installation
        err = h.queries.DisableAutoDeployForInstallation(ctx, event.Installation.ID)
        if err != nil {
            h.logger.Error("failed to disable auto-deploy",
                "installation_id", event.Installation.ID,
                "error", err,
            )
        }

    case "suspend":
        h.logger.Warn("github app suspended",
            "installation_id", event.Installation.ID,
            "account", event.Installation.Account.Login,
        )

    case "unsuspend":
        h.logger.Info("github app unsuspended",
            "installation_id", event.Installation.ID,
            "account", event.Installation.Account.Login,
        )
    }
}

// InstallationRepositoriesEvent for repo add/remove
type InstallationRepositoriesEvent struct {
    Action       string `json:"action"` // added, removed
    Installation struct {
        ID int64 `json:"id"`
    } `json:"installation"`
    RepositoriesAdded []struct {
        ID       int64  `json:"id"`
        FullName string `json:"full_name"`
    } `json:"repositories_added"`
    RepositoriesRemoved []struct {
        ID       int64  `json:"id"`
        FullName string `json:"full_name"`
    } `json:"repositories_removed"`
}

// handleInstallationReposEvent processes repo add/remove events
func (h *Handlers) handleInstallationReposEvent(ctx context.Context, body []byte) {
    var event InstallationRepositoriesEvent
    if err := json.Unmarshal(body, &event); err != nil {
        h.logger.Error("failed to parse installation repos event", "error", err)
        return
    }

    h.logger.Info("processing installation repos event",
        "action", event.Action,
        "installation_id", event.Installation.ID,
        "added", len(event.RepositoriesAdded),
        "removed", len(event.RepositoriesRemoved),
    )

    // If repos were removed, check if any apps use them
    for _, repo := range event.RepositoriesRemoved {
        apps, err := h.queries.GetAppsByRepo(ctx, repo.FullName)
        if err != nil {
            continue
        }

        for _, app := range apps {
            // Disable auto-deploy for removed repos
            h.queries.UpdateAppAutoDeploy(ctx, queries.UpdateAppAutoDeployParams{
                ID:         app.ID,
                AutoDeploy: false,
            })

            h.logger.Warn("disabled auto-deploy due to repo removal",
                "app_id", app.ID,
                "repo", repo.FullName,
            )
        }
    }
}
```

---

## Database Queries

```sql
-- internal/database/queries/webhooks.sql

-- name: GetAppsByRepo :many
SELECT * FROM apps
WHERE github_repo = $1;

-- name: DeactivateInstallation :exec
UPDATE github_installations
SET active = false, updated_at = NOW()
WHERE installation_id = $1;

-- name: DisableAutoDeployForInstallation :exec
UPDATE apps
SET auto_deploy = false, updated_at = NOW()
WHERE team_id IN (
    SELECT team_id FROM github_installations
    WHERE installation_id = $1
);

-- name: UpdateAppAutoDeploy :exec
UPDATE apps
SET auto_deploy = $2, updated_at = NOW()
WHERE id = $1;

-- name: CreateDeployment :one
INSERT INTO deployments (app_id, commit_sha, commit_message, status, trigger)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateDeploymentStatus :exec
UPDATE deployments
SET status = $2, updated_at = NOW()
WHERE id = $1;

-- name: UpdateDeploymentImage :exec
UPDATE deployments
SET image_tag = $2, updated_at = NOW()
WHERE id = $1;
```

---

## Webhook Configuration UI

```go
// internal/templates/components/webhook_settings.templ
package components

templ WebhookSettings(app queries.App) {
    <div class="webhook-settings">
        <h3 class="text-lg font-semibold mb-4">Auto-Deploy Settings</h3>

        <div class="space-y-4">
            <!-- Auto-deploy toggle -->
            <div class="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
                <div>
                    <div class="font-medium">Auto-deploy on push</div>
                    <div class="text-sm text-gray-500">
                        Automatically deploy when changes are pushed to the { app.GitHubBranch } branch
                    </div>
                </div>
                <label class="relative inline-flex items-center cursor-pointer">
                    <input
                        type="checkbox"
                        class="sr-only peer"
                        if app.AutoDeploy {
                            checked
                        }
                        hx-post={ fmt.Sprintf("/apps/%s/settings/auto-deploy", app.ID) }
                        hx-trigger="change"
                        hx-swap="none"
                        name="auto_deploy"
                    />
                    <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
                </label>
            </div>

            <!-- Branch selection -->
            <div class="p-4 bg-gray-50 rounded-lg">
                <label class="block font-medium mb-2">Deploy branch</label>
                <select
                    name="github_branch"
                    class="w-full px-3 py-2 border rounded-lg"
                    hx-post={ fmt.Sprintf("/apps/%s/settings/branch", app.ID) }
                    hx-trigger="change"
                    hx-swap="none"
                >
                    <option value="main" selected?={ app.GitHubBranch == "main" }>main</option>
                    <option value="master" selected?={ app.GitHubBranch == "master" }>master</option>
                    <option value="develop" selected?={ app.GitHubBranch == "develop" }>develop</option>
                    <option value="production" selected?={ app.GitHubBranch == "production" }>production</option>
                </select>
                <div class="text-sm text-gray-500 mt-1">
                    Only pushes to this branch will trigger auto-deploy
                </div>
            </div>

            <!-- Webhook status -->
            <div class="p-4 bg-gray-50 rounded-lg">
                <div class="flex items-center justify-between mb-2">
                    <span class="font-medium">Webhook Status</span>
                    if app.GitHubRepo != nil {
                        <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                            <span class="w-2 h-2 mr-1 bg-green-400 rounded-full"></span>
                            Connected
                        </span>
                    } else {
                        <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800">
                            Not connected
                        </span>
                    }
                </div>
                if app.GitHubRepo != nil {
                    <div class="text-sm text-gray-500">
                        Webhooks are automatically configured when you connect a GitHub repository.
                    </div>
                }
            </div>

            <!-- Recent webhook deliveries (optional) -->
            <div class="p-4 bg-gray-50 rounded-lg">
                <div class="font-medium mb-2">Recent Deployments from Webhooks</div>
                <div
                    hx-get={ fmt.Sprintf("/apps/%s/deployments/recent?trigger=webhook", app.ID) }
                    hx-trigger="load"
                    hx-swap="innerHTML"
                >
                    <div class="text-sm text-gray-500">Loading...</div>
                </div>
            </div>
        </div>
    </div>
}
```

---

## Routes

```go
// Add to router setup

// Webhook endpoint (no auth - uses signature verification)
r.Post("/webhooks/github", h.GitHubWebhook)

// Settings endpoints
r.Route("/apps/{appID}/settings", func(r chi.Router) {
    r.Use(h.RequireAuth)
    r.Post("/auto-deploy", h.ToggleAutoDeploy)
    r.Post("/branch", h.UpdateDeployBranch)
})
```

---

## Exit Criteria

Phase 10 is complete when:

1. [ ] Webhook endpoint receives GitHub events
2. [ ] Signature verification works correctly
3. [ ] Push events trigger builds for matching apps
4. [ ] Branch filtering works (only configured branch)
5. [ ] Auto-deploy toggle works
6. [ ] GitHub commit status updates (pending, success, failure)
7. [ ] Installation events handled (create, delete)
8. [ ] Repository removal disables affected apps' auto-deploy
9. [ ] Webhook settings UI shows correct state
10. [ ] Build/deploy failures properly reported to GitHub

---

## Dependencies

- **Requires**: Phase 2 (GitHub App), Phase 4 (Build), Phase 5 (Deploy)
- **Required by**: Phase 12 (production readiness)

---

*Phase 10 Specification - Version 1.0*
