# Phase 4: Build System

> **Building Docker images from user repositories using Railpack + BuildKit**

**Status**: Not Started

---

## Overview

Phase 4 implements the build system that transforms user source code into Docker images. We use Railpack to analyze code and generate build plans, and BuildKit (in rootless mode) to execute builds and push to Fly's registry.

### Goals

1. **Railpack integration**: Auto-detect language and generate build plans
2. **BuildKit daemon**: Run rootless BuildKit on Fly.io
3. **Repository cloning**: Clone repos using GitHub App tokens
4. **Image building**: Build Docker images from source
5. **Registry push**: Push images to registry.fly.io
6. **Build logs**: Stream build output to the dashboard

### Non-Goals

1. Deployment (Phase 5)
2. Custom Dockerfiles (future enhancement)
3. Build caching across users (security concern)
4. Multi-architecture builds (future)

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         BUILD PIPELINE                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  STEP 1: TRIGGER BUILD                                                   │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ POST /apps/{slug}/deploy                                            ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ Create deployment record (status: "pending")                        ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ Queue build job                                                     ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                         │
│  STEP 2: CLONE REPOSITORY                                               │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ Get GitHub App installation token (1 hour validity)                 ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ git clone https://x-access-token:{token}@github.com/{repo}.git      ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ git checkout {commit_sha or branch}                                 ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                         │
│  STEP 3: ANALYZE WITH RAILPACK                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ railpack prepare .                                                  ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ Outputs: railpack-plan.json                                         ││
│  │   - Detected language (Go, Node, Python, etc.)                      ││
│  │   - Build commands                                                  ││
│  │   - Runtime configuration                                           ││
│  │   - Environment setup                                               ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                         │
│  STEP 4: BUILD WITH BUILDKIT                                            │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ Connect to BuildKit daemon (rootless)                               ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ Execute build using Railpack frontend                               ││
│  │   buildctl build \                                                  ││
│  │     --frontend gateway.v0 \                                         ││
│  │     --opt source=ghcr.io/railwayapp/railpack-frontend \            ││
│  │     --local context=. \                                             ││
│  │     --output type=image,name=registry.fly.io/{app}:{tag},push=true ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ Stream build logs to deployment record                              ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  STEP 5: COMPLETE BUILD                                                 │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ Update deployment record:                                           ││
│  │   - status: "built" (or "build_failed")                            ││
│  │   - image_tag: registry.fly.io/{app}:{tag}                         ││
│  │   - build_logs: [captured output]                                   ││
│  │        │                                                            ││
│  │        ▼                                                            ││
│  │ Trigger deployment (Phase 5)                                        ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## BuildKit Infrastructure

### BuildKit Fly App

Create a dedicated Fly app for the BuildKit daemon:

```toml
# rhone-builder/fly.toml
app = "rhone-builder"
primary_region = "iad"

[build]
  dockerfile = "Dockerfile"

[env]
  BUILDKIT_STEP_LOG_MAX_SIZE = "10485760"
  BUILDKIT_STEP_LOG_MAX_SPEED = "10485760"

[mounts]
  source = "buildkit_cache"
  destination = "/var/lib/buildkit"

[[services]]
  internal_port = 8080
  protocol = "tcp"
  auto_stop_machines = false   # Keep running
  auto_start_machines = true
  min_machines_running = 1

[[vm]]
  cpu_kind = "shared"
  cpus = 4
  memory_mb = 8192
```

```dockerfile
# rhone-builder/Dockerfile
FROM moby/buildkit:rootless

# Run as non-root user
USER 1000:1000

# BuildKit configuration
COPY --chown=1000:1000 buildkitd.toml /home/user/.config/buildkit/buildkitd.toml

EXPOSE 8080

ENTRYPOINT ["buildkitd", "--addr", "tcp://0.0.0.0:8080"]
```

```toml
# rhone-builder/buildkitd.toml
[worker.oci]
  gc = true
  gckeep = "48h"
  max-parallelism = 4

[worker.containerd]
  enabled = false

[registry."registry.fly.io"]
  # Fly registry doesn't need explicit auth when running on Fly
```

---

## Database Schema

```sql
-- internal/database/migrations/004_deployments.up.sql

-- Deployments table
CREATE TABLE deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    triggered_by UUID REFERENCES users(id),

    -- Git info
    commit_sha VARCHAR(40),
    commit_message TEXT,
    commit_author VARCHAR(255),
    branch VARCHAR(255),

    -- Build info
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    image_tag VARCHAR(500),
    build_started_at TIMESTAMPTZ,
    build_finished_at TIMESTAMPTZ,
    build_logs TEXT,
    build_duration_ms INT,

    -- Deploy info (Phase 5)
    deploy_started_at TIMESTAMPTZ,
    deploy_finished_at TIMESTAMPTZ,
    deploy_logs TEXT,
    machine_id VARCHAR(255),

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Status values:
-- pending      - Created, waiting to start
-- cloning      - Cloning repository
-- analyzing    - Running Railpack
-- building     - BuildKit executing
-- pushing      - Pushing to registry
-- built        - Build complete, ready to deploy
-- deploying    - Deployment in progress (Phase 5)
-- live         - Successfully deployed (Phase 5)
-- build_failed - Build error
-- deploy_failed - Deployment error (Phase 5)
-- cancelled    - User cancelled

CREATE INDEX idx_deployments_app_id ON deployments(app_id);
CREATE INDEX idx_deployments_status ON deployments(status);
CREATE INDEX idx_deployments_created_at ON deployments(created_at DESC);

CREATE TRIGGER update_deployments_updated_at
    BEFORE UPDATE ON deployments
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

---

## Core Types

### Build Service

```go
// internal/build/builder.go
package build

import (
    "context"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "time"

    "github.com/google/uuid"
    "github.com/vangoframework/rhone/internal/auth"
    "github.com/vangoframework/rhone/internal/database/queries"
)

type Builder struct {
    githubApp    *auth.GitHubApp
    buildkitAddr string
    workDir      string
    registry     string
    logger       *slog.Logger
}

type BuildConfig struct {
    AppID          uuid.UUID
    AppSlug        string
    GitHubRepo     string
    GitHubBranch   string
    InstallationID int64
    CommitSHA      string // Optional: specific commit
    EnvVars        map[string]string
}

type BuildResult struct {
    Success    bool
    ImageTag   string
    Logs       string
    Duration   time.Duration
    CommitSHA  string
    CommitMsg  string
    Error      error
}

func NewBuilder(githubApp *auth.GitHubApp, buildkitAddr, workDir, registry string, logger *slog.Logger) *Builder {
    return &Builder{
        githubApp:    githubApp,
        buildkitAddr: buildkitAddr,
        workDir:      workDir,
        registry:     registry,
        logger:       logger,
    }
}

// Build executes the full build pipeline
func (b *Builder) Build(ctx context.Context, cfg BuildConfig, logWriter io.Writer) (*BuildResult, error) {
    startTime := time.Now()
    result := &BuildResult{}

    // Create temporary directory for this build
    buildDir := filepath.Join(b.workDir, cfg.AppSlug+"-"+uuid.New().String()[:8])
    if err := os.MkdirAll(buildDir, 0755); err != nil {
        return nil, fmt.Errorf("create build dir: %w", err)
    }
    defer os.RemoveAll(buildDir) // Cleanup

    // Step 1: Clone repository
    fmt.Fprintln(logWriter, "==> Cloning repository...")
    commitSHA, commitMsg, err := b.cloneRepo(ctx, cfg, buildDir, logWriter)
    if err != nil {
        result.Error = fmt.Errorf("clone failed: %w", err)
        result.Logs = captureOutput(logWriter)
        return result, nil
    }
    result.CommitSHA = commitSHA
    result.CommitMsg = commitMsg

    // Step 2: Analyze with Railpack
    fmt.Fprintln(logWriter, "==> Analyzing project...")
    if err := b.analyzeProject(ctx, buildDir, logWriter); err != nil {
        result.Error = fmt.Errorf("analysis failed: %w", err)
        result.Logs = captureOutput(logWriter)
        return result, nil
    }

    // Step 3: Build with BuildKit
    imageTag := b.imageTag(cfg.AppSlug, commitSHA)
    fmt.Fprintf(logWriter, "==> Building image %s...\n", imageTag)
    if err := b.buildImage(ctx, buildDir, imageTag, cfg.EnvVars, logWriter); err != nil {
        result.Error = fmt.Errorf("build failed: %w", err)
        result.Logs = captureOutput(logWriter)
        return result, nil
    }

    // Success!
    result.Success = true
    result.ImageTag = imageTag
    result.Duration = time.Since(startTime)
    result.Logs = captureOutput(logWriter)

    fmt.Fprintf(logWriter, "==> Build complete in %s\n", result.Duration.Round(time.Second))

    return result, nil
}

func (b *Builder) imageTag(appSlug, commitSHA string) string {
    tag := commitSHA
    if len(tag) > 12 {
        tag = tag[:12]
    }
    return fmt.Sprintf("%s/%s:%s", b.registry, appSlug, tag)
}
```

### Repository Cloning

```go
// internal/build/clone.go
package build

import (
    "bufio"
    "context"
    "fmt"
    "io"
    "os/exec"
    "strings"
)

func (b *Builder) cloneRepo(ctx context.Context, cfg BuildConfig, dir string, w io.Writer) (commitSHA, commitMsg string, err error) {
    // Get installation token
    token, err := b.githubApp.GetInstallationToken(ctx, cfg.InstallationID)
    if err != nil {
        return "", "", fmt.Errorf("get token: %w", err)
    }

    // Clone URL with token
    cloneURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token.Token, cfg.GitHubRepo)

    // Clone with depth 1 for speed
    cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", cfg.GitHubBranch, cloneURL, dir)
    cmd.Stdout = w
    cmd.Stderr = w

    if err := cmd.Run(); err != nil {
        return "", "", fmt.Errorf("git clone: %w", err)
    }

    // If specific commit requested, fetch and checkout
    if cfg.CommitSHA != "" {
        fetchCmd := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "origin", cfg.CommitSHA)
        fetchCmd.Stdout = w
        fetchCmd.Stderr = w
        if err := fetchCmd.Run(); err != nil {
            return "", "", fmt.Errorf("git fetch: %w", err)
        }

        checkoutCmd := exec.CommandContext(ctx, "git", "-C", dir, "checkout", cfg.CommitSHA)
        checkoutCmd.Stdout = w
        checkoutCmd.Stderr = w
        if err := checkoutCmd.Run(); err != nil {
            return "", "", fmt.Errorf("git checkout: %w", err)
        }
    }

    // Get commit info
    shaCmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")
    shaOutput, err := shaCmd.Output()
    if err != nil {
        return "", "", fmt.Errorf("get commit sha: %w", err)
    }
    commitSHA = strings.TrimSpace(string(shaOutput))

    msgCmd := exec.CommandContext(ctx, "git", "-C", dir, "log", "-1", "--pretty=%B")
    msgOutput, err := msgCmd.Output()
    if err != nil {
        return "", "", fmt.Errorf("get commit message: %w", err)
    }
    commitMsg = strings.TrimSpace(string(msgOutput))

    fmt.Fprintf(w, "Cloned %s at %s\n", cfg.GitHubRepo, commitSHA[:12])

    return commitSHA, commitMsg, nil
}
```

### Railpack Analysis

```go
// internal/build/railpack.go
package build

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
)

// RailpackPlan represents the output of railpack prepare
type RailpackPlan struct {
    Language  string            `json:"language"`
    Framework string            `json:"framework"`
    Build     RailpackBuild     `json:"build"`
    Runtime   RailpackRuntime   `json:"runtime"`
}

type RailpackBuild struct {
    Commands []string `json:"commands"`
    Packages []string `json:"packages"`
}

type RailpackRuntime struct {
    Command string            `json:"command"`
    Env     map[string]string `json:"env"`
}

func (b *Builder) analyzeProject(ctx context.Context, dir string, w io.Writer) error {
    // Run railpack prepare
    cmd := exec.CommandContext(ctx, "railpack", "prepare", ".")
    cmd.Dir = dir
    cmd.Stdout = w
    cmd.Stderr = w

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("railpack prepare: %w", err)
    }

    // Check if plan was created
    planPath := filepath.Join(dir, "railpack-plan.json")
    if _, err := os.Stat(planPath); os.IsNotExist(err) {
        return fmt.Errorf("railpack did not generate a build plan")
    }

    // Parse and log the plan
    planData, err := os.ReadFile(planPath)
    if err != nil {
        return fmt.Errorf("read plan: %w", err)
    }

    var plan RailpackPlan
    if err := json.Unmarshal(planData, &plan); err != nil {
        return fmt.Errorf("parse plan: %w", err)
    }

    fmt.Fprintf(w, "Detected: %s", plan.Language)
    if plan.Framework != "" {
        fmt.Fprintf(w, " (%s)", plan.Framework)
    }
    fmt.Fprintln(w)

    return nil
}
```

### BuildKit Integration

```go
// internal/build/buildkit.go
package build

import (
    "context"
    "fmt"
    "io"
    "os"
    "os/exec"
    "strings"
)

func (b *Builder) buildImage(ctx context.Context, dir, imageTag string, envVars map[string]string, w io.Writer) error {
    // Prepare build args for environment variables
    var buildArgs []string
    for key, value := range envVars {
        buildArgs = append(buildArgs, "--opt", fmt.Sprintf("build-arg:%s=%s", key, value))
    }

    // Build command using buildctl
    args := []string{
        "build",
        "--addr", b.buildkitAddr,
        "--frontend", "gateway.v0",
        "--opt", "source=ghcr.io/railwayapp/railpack-frontend",
        "--local", "context=" + dir,
        "--local", "dockerfile=" + dir,
        "--output", fmt.Sprintf("type=image,name=%s,push=true", imageTag),
    }
    args = append(args, buildArgs...)

    cmd := exec.CommandContext(ctx, "buildctl", args...)
    cmd.Stdout = w
    cmd.Stderr = w

    // Set Fly registry auth if available
    if token := os.Getenv("FLY_API_TOKEN"); token != "" {
        cmd.Env = append(os.Environ(),
            "DOCKER_CONFIG=/tmp/docker-config",
        )
        // Create docker config with Fly auth
        if err := b.setupFlyAuth(token); err != nil {
            return fmt.Errorf("setup fly auth: %w", err)
        }
    }

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("buildctl: %w", err)
    }

    return nil
}

func (b *Builder) setupFlyAuth(token string) error {
    configDir := "/tmp/docker-config"
    if err := os.MkdirAll(configDir, 0700); err != nil {
        return err
    }

    config := fmt.Sprintf(`{
        "auths": {
            "registry.fly.io": {
                "auth": "%s"
            }
        }
    }`, base64.StdEncoding.EncodeToString([]byte("x:"+token)))

    return os.WriteFile(filepath.Join(configDir, "config.json"), []byte(config), 0600)
}
```

---

## Deployment Record Management

```go
// internal/build/deployment.go
package build

import (
    "context"
    "time"

    "github.com/google/uuid"
    "github.com/vangoframework/rhone/internal/database/queries"
)

type DeploymentService struct {
    queries *queries.Queries
    builder *Builder
    logger  *slog.Logger
}

func NewDeploymentService(q *queries.Queries, b *Builder, logger *slog.Logger) *DeploymentService {
    return &DeploymentService{
        queries: q,
        builder: b,
        logger:  logger,
    }
}

// StartDeployment creates a deployment record and triggers the build
func (s *DeploymentService) StartDeployment(ctx context.Context, app queries.App, userID uuid.UUID, commitSHA string) (*queries.Deployment, error) {
    // Create deployment record
    deployment, err := s.queries.CreateDeployment(ctx, queries.CreateDeploymentParams{
        AppID:       app.ID,
        TriggeredBy: &userID,
        Branch:      app.GithubBranch,
        CommitSha:   &commitSHA,
        Status:      "pending",
    })
    if err != nil {
        return nil, fmt.Errorf("create deployment: %w", err)
    }

    // Start build in background
    go s.runBuild(context.Background(), deployment, app)

    return &deployment, nil
}

func (s *DeploymentService) runBuild(ctx context.Context, deployment queries.Deployment, app queries.App) {
    // Update status to cloning
    s.updateStatus(ctx, deployment.ID, "cloning", nil)

    // Create log buffer
    var logBuffer strings.Builder

    // Get env vars
    envVars, err := s.getDecryptedEnvVars(ctx, app.ID)
    if err != nil {
        s.failBuild(ctx, deployment.ID, fmt.Sprintf("Failed to get env vars: %v", err), &logBuffer)
        return
    }

    // Build config
    cfg := BuildConfig{
        AppID:          app.ID,
        AppSlug:        app.Slug,
        GitHubRepo:     *app.GithubRepo,
        GitHubBranch:   app.GithubBranch,
        InstallationID: *app.GithubInstallationID,
        CommitSHA:      stringValue(deployment.CommitSha),
        EnvVars:        envVars,
    }

    // Mark build started
    s.queries.UpdateDeploymentBuildStarted(ctx, deployment.ID)

    // Run build
    result, err := s.builder.Build(ctx, cfg, &logBuffer)
    if err != nil {
        s.failBuild(ctx, deployment.ID, fmt.Sprintf("Build error: %v", err), &logBuffer)
        return
    }

    if !result.Success {
        s.failBuild(ctx, deployment.ID, result.Error.Error(), &logBuffer)
        return
    }

    // Update deployment with build result
    s.queries.UpdateDeploymentBuildComplete(ctx, queries.UpdateDeploymentBuildCompleteParams{
        ID:               deployment.ID,
        Status:           "built",
        ImageTag:         &result.ImageTag,
        BuildLogs:        &result.Logs,
        CommitSha:        &result.CommitSHA,
        CommitMessage:    &result.CommitMsg,
        BuildDurationMs:  int32(result.Duration.Milliseconds()),
    })

    s.logger.Info("build complete",
        "deployment_id", deployment.ID,
        "image", result.ImageTag,
        "duration", result.Duration,
    )

    // TODO: Trigger deployment (Phase 5)
}

func (s *DeploymentService) updateStatus(ctx context.Context, id uuid.UUID, status string, logs *strings.Builder) {
    var logsPtr *string
    if logs != nil {
        logsStr := logs.String()
        logsPtr = &logsStr
    }

    s.queries.UpdateDeploymentStatus(ctx, queries.UpdateDeploymentStatusParams{
        ID:        id,
        Status:    status,
        BuildLogs: logsPtr,
    })
}

func (s *DeploymentService) failBuild(ctx context.Context, id uuid.UUID, errMsg string, logs *strings.Builder) {
    logs.WriteString("\n\nBuild failed: " + errMsg)
    s.updateStatus(ctx, id, "build_failed", logs)

    s.logger.Error("build failed",
        "deployment_id", id,
        "error", errMsg,
    )
}
```

---

## Handlers

```go
// internal/handlers/deploy.go
package handlers

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/vangoframework/rhone/internal/middleware"
)

func (h *Handlers) TriggerDeploy(w http.ResponseWriter, r *http.Request) {
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

    // Optional: specific commit SHA
    commitSHA := r.FormValue("commit_sha")

    // Start deployment
    deployment, err := h.deployments.StartDeployment(ctx, app, session.UserID, commitSHA)
    if err != nil {
        h.logger.Error("failed to start deployment", "error", err)
        http.Error(w, "Failed to start deployment", http.StatusInternalServerError)
        return
    }

    h.logger.Info("deployment started",
        "deployment_id", deployment.ID,
        "app", slug,
    )

    // Redirect to deployment page
    http.Redirect(w, r, fmt.Sprintf("/apps/%s/deployments/%s", slug, deployment.ID), http.StatusSeeOther)
}

func (h *Handlers) ShowDeployment(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)
    slug := chi.URLParam(r, "slug")
    deploymentID := chi.URLParam(r, "id")

    app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
        TeamID: session.TeamID,
        Slug:   slug,
    })
    if err != nil {
        http.NotFound(w, r)
        return
    }

    id, err := uuid.Parse(deploymentID)
    if err != nil {
        http.NotFound(w, r)
        return
    }

    deployment, err := h.queries.GetDeployment(ctx, id)
    if err != nil || deployment.AppID != app.ID {
        http.NotFound(w, r)
        return
    }

    pages.Deployment(session, app, deployment).Render(ctx, w)
}

// StreamBuildLogs provides SSE stream of build logs
func (h *Handlers) StreamBuildLogs(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    deploymentID := chi.URLParam(r, "id")

    id, err := uuid.Parse(deploymentID)
    if err != nil {
        http.NotFound(w, r)
        return
    }

    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }

    // Poll for updates
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    lastLength := 0

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            deployment, err := h.queries.GetDeployment(ctx, id)
            if err != nil {
                return
            }

            // Send new logs
            if deployment.BuildLogs != nil && len(*deployment.BuildLogs) > lastLength {
                newContent := (*deployment.BuildLogs)[lastLength:]
                fmt.Fprintf(w, "data: %s\n\n", escapeSSE(newContent))
                flusher.Flush()
                lastLength = len(*deployment.BuildLogs)
            }

            // Check if complete
            if deployment.Status == "built" || deployment.Status == "build_failed" ||
               deployment.Status == "live" || deployment.Status == "deploy_failed" {
                fmt.Fprintf(w, "event: complete\ndata: %s\n\n", deployment.Status)
                flusher.Flush()
                return
            }
        }
    }
}

func escapeSSE(s string) string {
    return strings.ReplaceAll(s, "\n", "\\n")
}
```

---

## Templates

### Deployment Page

```go
// internal/templates/pages/deployment.templ
package pages

import (
    "github.com/vangoframework/rhone/internal/auth"
    "github.com/vangoframework/rhone/internal/database/queries"
    "github.com/vangoframework/rhone/internal/templates/layouts"
)

templ Deployment(session *auth.SessionData, app queries.App, deployment queries.Deployment) {
    @layouts.App(app.Name + " - Deployment") {
        <div class="max-w-4xl">
            <!-- Header -->
            <div class="mb-8">
                <nav class="text-sm text-gray-500 mb-2">
                    <a href="/apps" class="hover:text-gray-700">Apps</a>
                    <span class="mx-2">/</span>
                    <a href={ templ.SafeURL("/apps/" + app.Slug) } class="hover:text-gray-700">{ app.Name }</a>
                    <span class="mx-2">/</span>
                    <span>Deployment</span>
                </nav>
                <div class="flex items-center justify-between">
                    <h1 class="text-2xl font-semibold text-gray-900">
                        Deployment { deployment.ID.String()[:8] }
                    </h1>
                    @DeploymentStatus(deployment.Status)
                </div>
            </div>

            <!-- Commit Info -->
            if deployment.CommitSha != nil {
                <div class="bg-white shadow rounded-lg p-4 mb-6">
                    <div class="flex items-center gap-3">
                        <div class="flex-shrink-0">
                            <svg class="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"/>
                            </svg>
                        </div>
                        <div>
                            <p class="text-sm font-mono text-gray-900">{ (*deployment.CommitSha)[:12] }</p>
                            if deployment.CommitMessage != nil {
                                <p class="text-sm text-gray-500">{ *deployment.CommitMessage }</p>
                            }
                        </div>
                    </div>
                </div>
            }

            <!-- Build Logs -->
            <div class="bg-gray-900 rounded-lg overflow-hidden">
                <div class="px-4 py-2 bg-gray-800 border-b border-gray-700">
                    <h3 class="text-sm font-medium text-gray-200">Build Logs</h3>
                </div>
                <div
                    id="build-logs"
                    class="p-4 h-96 overflow-y-auto font-mono text-sm text-gray-100 whitespace-pre-wrap"
                    if isBuilding(deployment.Status) {
                        hx-ext="sse"
                        sse-connect={ "/apps/" + app.Slug + "/deployments/" + deployment.ID.String() + "/logs" }
                        sse-swap="message"
                        hx-swap="beforeend"
                    }
                >
                    if deployment.BuildLogs != nil {
                        { *deployment.BuildLogs }
                    } else {
                        <span class="text-gray-500">Waiting for logs...</span>
                    }
                </div>
            </div>

            <!-- Actions -->
            <div class="mt-6 flex justify-end gap-3">
                <a href={ templ.SafeURL("/apps/" + app.Slug) } class="rounded-md bg-white px-3 py-2 text-sm font-semibold text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 hover:bg-gray-50">
                    Back to App
                </a>
                if deployment.Status == "build_failed" {
                    <form method="POST" action={ templ.SafeURL("/apps/" + app.Slug + "/deploy") }>
                        <button type="submit" class="rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500">
                            Retry Deploy
                        </button>
                    </form>
                }
            </div>
        </div>
    }
}

templ DeploymentStatus(status string) {
    switch status {
        case "pending", "cloning", "analyzing", "building", "pushing":
            <span class="inline-flex items-center gap-1.5 rounded-full bg-yellow-100 px-3 py-1 text-sm font-medium text-yellow-800">
                <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                { statusLabel(status) }
            </span>
        case "built":
            <span class="inline-flex items-center rounded-full bg-blue-100 px-3 py-1 text-sm font-medium text-blue-800">
                Built
            </span>
        case "deploying":
            <span class="inline-flex items-center gap-1.5 rounded-full bg-blue-100 px-3 py-1 text-sm font-medium text-blue-800">
                <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                Deploying
            </span>
        case "live":
            <span class="inline-flex items-center rounded-full bg-green-100 px-3 py-1 text-sm font-medium text-green-800">
                Live
            </span>
        case "build_failed", "deploy_failed":
            <span class="inline-flex items-center rounded-full bg-red-100 px-3 py-1 text-sm font-medium text-red-800">
                Failed
            </span>
        case "cancelled":
            <span class="inline-flex items-center rounded-full bg-gray-100 px-3 py-1 text-sm font-medium text-gray-800">
                Cancelled
            </span>
    }
}

func statusLabel(status string) string {
    switch status {
    case "pending":
        return "Pending"
    case "cloning":
        return "Cloning"
    case "analyzing":
        return "Analyzing"
    case "building":
        return "Building"
    case "pushing":
        return "Pushing"
    default:
        return status
    }
}

func isBuilding(status string) bool {
    return status == "pending" || status == "cloning" || status == "analyzing" ||
           status == "building" || status == "pushing"
}
```

---

## Routes

```go
// Add to router

// Deployments
r.Post("/apps/{slug}/deploy", h.TriggerDeploy)
r.Get("/apps/{slug}/deployments", h.ListDeployments)
r.Get("/apps/{slug}/deployments/{id}", h.ShowDeployment)
r.Get("/apps/{slug}/deployments/{id}/logs", h.StreamBuildLogs)
```

---

## Testing Strategy

### Unit Tests

```go
// internal/build/builder_test.go
package build_test

import (
    "context"
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/vangoframework/rhone/internal/build"
)

func TestImageTag(t *testing.T) {
    b := &build.Builder{}

    tag := b.imageTag("my-app", "abc123def456")
    assert.Equal(t, "registry.fly.io/my-app:abc123def456", tag)
}

func TestImageTagLongSHA(t *testing.T) {
    b := &build.Builder{}

    tag := b.imageTag("my-app", "abc123def456789012345678901234567890")
    assert.Equal(t, "registry.fly.io/my-app:abc123def456", tag) // Truncated to 12 chars
}
```

### Integration Tests

```go
// internal/build/integration_test.go
// +build integration

package build_test

import (
    "context"
    "strings"
    "testing"

    "github.com/stretchr/testify/require"
    "github.com/vangoframework/rhone/internal/build"
)

func TestBuildGoProject(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    builder := setupTestBuilder(t)

    cfg := build.BuildConfig{
        AppSlug:    "test-app",
        GitHubRepo: "vangoframework/sample-app",
        GitHubBranch: "main",
        InstallationID: testInstallationID,
    }

    var logs strings.Builder
    result, err := builder.Build(context.Background(), cfg, &logs)
    require.NoError(t, err)

    assert.True(t, result.Success)
    assert.NotEmpty(t, result.ImageTag)
    assert.Contains(t, logs.String(), "Detected: Go")
}
```

---

## File Structure

```
internal/
├── build/
│   ├── builder.go       # Main build orchestration
│   ├── clone.go         # Git cloning
│   ├── railpack.go      # Railpack analysis
│   ├── buildkit.go      # BuildKit integration
│   ├── deployment.go    # Deployment record management
│   └── builder_test.go
├── database/
│   └── migrations/
│       ├── 004_deployments.up.sql
│       └── 004_deployments.down.sql
├── handlers/
│   └── deploy.go        # Deployment handlers
└── templates/
    └── pages/
        └── deployment.templ

rhone-builder/           # Separate Fly app for BuildKit
├── Dockerfile
├── fly.toml
└── buildkitd.toml
```

---

## Exit Criteria

Phase 4 is complete when:

1. [ ] BuildKit daemon running on Fly.io (rootless)
2. [ ] Repositories can be cloned with GitHub App tokens
3. [ ] Railpack analyzes Go projects correctly
4. [ ] BuildKit builds images successfully
5. [ ] Images pushed to registry.fly.io
6. [ ] Deployment records track build progress
7. [ ] Build logs stream to UI via SSE
8. [ ] Failed builds show error messages
9. [ ] Build can be retried after failure
10. [ ] Unit tests pass
11. [ ] Integration tests pass (with real build)

---

## Dependencies

- **Requires**: Phase 1-3 (apps, GitHub access)
- **Required by**: Phase 5 (deployment needs built images)

---

*Phase 4 Specification - Version 1.0*
