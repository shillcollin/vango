# Phase 5: Deployment

> **Deploying built images to Fly.io Machines**

**Status**: Not Started

---

## Overview

Phase 5 implements the deployment system that takes built Docker images from Phase 4 and deploys them as Fly.io Machines. This includes creating Fly apps, configuring machines, health checks, and zero-downtime updates.

### Goals

1. **Fly app creation**: Create Fly app for each user app on first deploy
2. **Machine deployment**: Deploy images as Fly Machines
3. **Zero-downtime updates**: Blue/green deployments
4. **Health checks**: Verify app is healthy before routing traffic
5. **Rollback**: Revert to previous working deployment
6. **Vango-aware config**: Special configuration for stateful Vango apps

### Non-Goals

1. Custom domains (Phase 6)
2. Multi-region deployment (Phase 11)
3. Horizontal scaling (future)
4. Volume management (future)

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                       DEPLOYMENT FLOW                                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  STEP 1: CREATE FLY APP (First Deploy Only)                             │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ POST https://api.machines.dev/v1/apps                               ││
│  │ {                                                                   ││
│  │   "app_name": "rhone-{team}-{slug}",                               ││
│  │   "org_slug": "rhone"                                               ││
│  │ }                                                                   ││
│  │                                                                     ││
│  │ Store fly_app_id in apps table                                      ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  STEP 2: DEPLOY MACHINE                                                 │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ New App (no existing machine):                                      ││
│  │   POST /v1/apps/{app}/machines                                      ││
│  │                                                                     ││
│  │ Update (existing machine):                                          ││
│  │   POST /v1/apps/{app}/machines/{id}                                 ││
│  │   - Starts new machine with new image                               ││
│  │   - Waits for health check                                          ││
│  │   - Fly proxy routes traffic to new machine                         ││
│  │   - Old machine stops                                               ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  STEP 3: CONFIGURE NETWORKING                                           │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ Automatic:                                                          ││
│  │   - {fly_app}.fly.dev → Machine port 8080                          ││
│  │                                                                     ││
│  │ Later (Phase 6):                                                    ││
│  │   - {slug}.rhone.app → CNAME to {fly_app}.fly.dev                  ││
│  │   - custom.domain.com → Certificate + CNAME                         ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  STEP 4: HEALTH CHECK                                                   │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ HTTP health check:                                                  ││
│  │   GET http://machine:8080/health                                    ││
│  │   - Must return 200 within timeout                                  ││
│  │   - Retries with backoff                                            ││
│  │   - Fails deployment if unhealthy                                   ││
│  │                                                                     ││
│  │ Vango-specific:                                                     ││
│  │   - Also check WebSocket endpoint available                         ││
│  │   - Longer grace period for state initialization                    ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Fly.io API Integration

### Fly API Client

```go
// internal/fly/client.go
package fly

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

const (
    MachinesAPIURL = "https://api.machines.dev"
    FlyAPIURL      = "https://api.fly.io"
)

type Client struct {
    token      string
    orgSlug    string
    httpClient *http.Client
}

func NewClient(token, orgSlug string) *Client {
    return &Client{
        token:   token,
        orgSlug: orgSlug,
        httpClient: &http.Client{
            Timeout: 60 * time.Second,
        },
    }
}

func (c *Client) do(ctx context.Context, method, url string, body any) (*http.Response, error) {
    var bodyReader io.Reader
    if body != nil {
        data, err := json.Marshal(body)
        if err != nil {
            return nil, err
        }
        bodyReader = bytes.NewReader(data)
    }

    req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+c.token)
    req.Header.Set("Content-Type", "application/json")

    return c.httpClient.Do(req)
}

func (c *Client) doJSON(ctx context.Context, method, url string, body, result any) error {
    resp, err := c.do(ctx, method, url, body)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("fly api error %d: %s", resp.StatusCode, string(bodyBytes))
    }

    if result != nil {
        return json.NewDecoder(resp.Body).Decode(result)
    }
    return nil
}
```

### App Management

```go
// internal/fly/apps.go
package fly

import (
    "context"
    "fmt"
)

type App struct {
    ID           string `json:"id"`
    Name         string `json:"name"`
    Organization struct {
        Slug string `json:"slug"`
    } `json:"organization"`
    Status string `json:"status"`
}

type CreateAppRequest struct {
    AppName string `json:"app_name"`
    OrgSlug string `json:"org_slug"`
}

// CreateApp creates a new Fly app
func (c *Client) CreateApp(ctx context.Context, name string) (*App, error) {
    req := CreateAppRequest{
        AppName: name,
        OrgSlug: c.orgSlug,
    }

    var app App
    err := c.doJSON(ctx, "POST", MachinesAPIURL+"/v1/apps", req, &app)
    if err != nil {
        return nil, fmt.Errorf("create app: %w", err)
    }

    return &app, nil
}

// GetApp retrieves an app by name
func (c *Client) GetApp(ctx context.Context, name string) (*App, error) {
    var app App
    err := c.doJSON(ctx, "GET", MachinesAPIURL+"/v1/apps/"+name, nil, &app)
    if err != nil {
        return nil, fmt.Errorf("get app: %w", err)
    }
    return &app, nil
}

// DeleteApp deletes an app
func (c *Client) DeleteApp(ctx context.Context, name string) error {
    _, err := c.do(ctx, "DELETE", MachinesAPIURL+"/v1/apps/"+name, nil)
    return err
}

// GenerateFlyAppName creates a unique Fly app name
func GenerateFlyAppName(teamSlug, appSlug string) string {
    // Fly app names must be globally unique and max 63 chars
    // Format: rhone-{team}-{app}-{random}
    name := fmt.Sprintf("rhone-%s-%s", teamSlug, appSlug)
    if len(name) > 55 {
        name = name[:55]
    }
    return name + "-" + randomString(6)
}

func randomString(n int) string {
    const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
    b := make([]byte, n)
    rand.Read(b)
    for i := range b {
        b[i] = chars[int(b[i])%len(chars)]
    }
    return string(b)
}
```

### Machine Management

```go
// internal/fly/machines.go
package fly

import (
    "context"
    "fmt"
    "time"
)

type Machine struct {
    ID         string        `json:"id"`
    Name       string        `json:"name"`
    State      string        `json:"state"`
    Region     string        `json:"region"`
    InstanceID string        `json:"instance_id"`
    Config     MachineConfig `json:"config"`
    CreatedAt  time.Time     `json:"created_at"`
    UpdatedAt  time.Time     `json:"updated_at"`
}

type MachineConfig struct {
    Image    string            `json:"image"`
    Env      map[string]string `json:"env"`
    Guest    GuestConfig       `json:"guest"`
    Services []ServiceConfig   `json:"services"`
    Checks   map[string]Check  `json:"checks,omitempty"`
    Restart  RestartConfig     `json:"restart,omitempty"`
    Init     InitConfig        `json:"init,omitempty"`
    StopConfig StopConfig      `json:"stop_config,omitempty"`
    AutoDestroy bool           `json:"auto_destroy,omitempty"`
}

type GuestConfig struct {
    CPUKind  string `json:"cpu_kind"`
    CPUs     int    `json:"cpus"`
    MemoryMB int    `json:"memory_mb"`
}

type ServiceConfig struct {
    Protocol     string           `json:"protocol"`
    InternalPort int              `json:"internal_port"`
    Ports        []PortConfig     `json:"ports"`
    Concurrency  ConcurrencyConfig `json:"concurrency,omitempty"`
}

type PortConfig struct {
    Port     int      `json:"port"`
    Handlers []string `json:"handlers"`
}

type ConcurrencyConfig struct {
    Type      string `json:"type"`
    HardLimit int    `json:"hard_limit"`
    SoftLimit int    `json:"soft_limit"`
}

type Check struct {
    Type        string        `json:"type"`
    Port        int           `json:"port,omitempty"`
    Path        string        `json:"path,omitempty"`
    Interval    string        `json:"interval,omitempty"`
    Timeout     string        `json:"timeout,omitempty"`
    GracePeriod string        `json:"grace_period,omitempty"`
}

type RestartConfig struct {
    Policy     string `json:"policy"`
    MaxRetries int    `json:"max_retries,omitempty"`
}

type InitConfig struct {
    Cmd []string `json:"cmd,omitempty"`
}

type StopConfig struct {
    Timeout string `json:"timeout,omitempty"`
    Signal  string `json:"signal,omitempty"`
}

// CreateMachine creates a new machine
func (c *Client) CreateMachine(ctx context.Context, appName string, config MachineConfig) (*Machine, error) {
    req := struct {
        Config MachineConfig `json:"config"`
        Region string        `json:"region,omitempty"`
    }{
        Config: config,
    }

    var machine Machine
    url := fmt.Sprintf("%s/v1/apps/%s/machines", MachinesAPIURL, appName)
    err := c.doJSON(ctx, "POST", url, req, &machine)
    if err != nil {
        return nil, fmt.Errorf("create machine: %w", err)
    }

    return &machine, nil
}

// UpdateMachine updates an existing machine (blue/green deploy)
func (c *Client) UpdateMachine(ctx context.Context, appName, machineID string, config MachineConfig) (*Machine, error) {
    req := struct {
        Config MachineConfig `json:"config"`
    }{
        Config: config,
    }

    var machine Machine
    url := fmt.Sprintf("%s/v1/apps/%s/machines/%s", MachinesAPIURL, appName, machineID)
    err := c.doJSON(ctx, "POST", url, req, &machine)
    if err != nil {
        return nil, fmt.Errorf("update machine: %w", err)
    }

    return &machine, nil
}

// ListMachines lists all machines for an app
func (c *Client) ListMachines(ctx context.Context, appName string) ([]Machine, error) {
    var machines []Machine
    url := fmt.Sprintf("%s/v1/apps/%s/machines", MachinesAPIURL, appName)
    err := c.doJSON(ctx, "GET", url, nil, &machines)
    if err != nil {
        return nil, fmt.Errorf("list machines: %w", err)
    }
    return machines, nil
}

// GetMachine retrieves a machine by ID
func (c *Client) GetMachine(ctx context.Context, appName, machineID string) (*Machine, error) {
    var machine Machine
    url := fmt.Sprintf("%s/v1/apps/%s/machines/%s", MachinesAPIURL, appName, machineID)
    err := c.doJSON(ctx, "GET", url, nil, &machine)
    if err != nil {
        return nil, fmt.Errorf("get machine: %w", err)
    }
    return &machine, nil
}

// WaitForMachine waits for machine to reach desired state
func (c *Client) WaitForMachine(ctx context.Context, appName, machineID string, desiredState string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)

    for time.Now().Before(deadline) {
        machine, err := c.GetMachine(ctx, appName, machineID)
        if err != nil {
            return err
        }

        if machine.State == desiredState {
            return nil
        }

        if machine.State == "failed" || machine.State == "destroyed" {
            return fmt.Errorf("machine entered state: %s", machine.State)
        }

        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(2 * time.Second):
        }
    }

    return fmt.Errorf("timeout waiting for machine state: %s", desiredState)
}

// StopMachine stops a running machine
func (c *Client) StopMachine(ctx context.Context, appName, machineID string) error {
    url := fmt.Sprintf("%s/v1/apps/%s/machines/%s/stop", MachinesAPIURL, appName, machineID)
    _, err := c.do(ctx, "POST", url, nil)
    return err
}

// DestroyMachine destroys a machine
func (c *Client) DestroyMachine(ctx context.Context, appName, machineID string) error {
    url := fmt.Sprintf("%s/v1/apps/%s/machines/%s?force=true", MachinesAPIURL, appName, machineID)
    _, err := c.do(ctx, "DELETE", url, nil)
    return err
}
```

---

## Vango-Specific Machine Configuration

```go
// internal/fly/vango_config.go
package fly

// VangoMachineConfig returns the optimal machine config for Vango apps
func VangoMachineConfig(imageTag string, envVars map[string]string, region string) MachineConfig {
    // Merge user env vars with required vars
    env := make(map[string]string)
    for k, v := range envVars {
        env[k] = v
    }
    env["PORT"] = "8080"

    return MachineConfig{
        Image: imageTag,
        Env:   env,
        Guest: GuestConfig{
            CPUKind:  "shared",
            CPUs:     1,
            MemoryMB: 512, // Vango apps need RAM for sessions
        },
        Services: []ServiceConfig{
            {
                Protocol:     "tcp",
                InternalPort: 8080,
                Ports: []PortConfig{
                    {Port: 443, Handlers: []string{"tls", "http"}},
                    {Port: 80, Handlers: []string{"http"}},
                },
                Concurrency: ConcurrencyConfig{
                    Type:      "connections", // WebSocket-friendly
                    HardLimit: 1000,
                    SoftLimit: 800,
                },
            },
        },
        Checks: map[string]Check{
            "health": {
                Type:        "http",
                Port:        8080,
                Path:        "/health",
                Interval:    "10s",
                Timeout:     "5s",
                GracePeriod: "30s", // Longer grace for Vango state init
            },
        },
        Restart: RestartConfig{
            Policy:     "on-failure",
            MaxRetries: 3,
        },
        StopConfig: StopConfig{
            Timeout: "30s",  // Allow WebSocket connections to drain
            Signal:  "SIGTERM",
        },
        // Don't auto-destroy - we manage lifecycle explicitly
        AutoDestroy: false,
    }
}
```

---

## Deployment Service

```go
// internal/deploy/deployer.go
package deploy

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/vangoframework/rhone/internal/database/queries"
    "github.com/vangoframework/rhone/internal/fly"
)

type Deployer struct {
    flyClient *fly.Client
    queries   *queries.Queries
    logger    *slog.Logger
}

func NewDeployer(flyClient *fly.Client, queries *queries.Queries, logger *slog.Logger) *Deployer {
    return &Deployer{
        flyClient: flyClient,
        queries:   queries,
        logger:    logger,
    }
}

type DeployConfig struct {
    App        queries.App
    Deployment queries.Deployment
    EnvVars    map[string]string
}

// Deploy deploys a built image to Fly
func (d *Deployer) Deploy(ctx context.Context, cfg DeployConfig) error {
    startTime := time.Now()

    // Update deployment status
    d.updateStatus(ctx, cfg.Deployment.ID, "deploying", nil)

    // Ensure Fly app exists
    flyAppID, err := d.ensureFlyApp(ctx, cfg.App)
    if err != nil {
        d.failDeploy(ctx, cfg.Deployment.ID, fmt.Sprintf("Failed to create Fly app: %v", err))
        return err
    }

    // Get or create machine
    machineID, err := d.deployMachine(ctx, flyAppID, cfg)
    if err != nil {
        d.failDeploy(ctx, cfg.Deployment.ID, fmt.Sprintf("Failed to deploy machine: %v", err))
        return err
    }

    // Wait for machine to be running
    d.logger.Info("waiting for machine to start", "machine_id", machineID)
    if err := d.flyClient.WaitForMachine(ctx, flyAppID, machineID, "started", 5*time.Minute); err != nil {
        d.failDeploy(ctx, cfg.Deployment.ID, fmt.Sprintf("Machine failed to start: %v", err))
        return err
    }

    // Wait for health check
    d.logger.Info("waiting for health check", "machine_id", machineID)
    if err := d.waitForHealthy(ctx, flyAppID, machineID); err != nil {
        d.failDeploy(ctx, cfg.Deployment.ID, fmt.Sprintf("Health check failed: %v", err))
        return err
    }

    // Success!
    duration := time.Since(startTime)
    d.queries.UpdateDeploymentLive(ctx, queries.UpdateDeploymentLiveParams{
        ID:                 cfg.Deployment.ID,
        Status:             "live",
        MachineID:          &machineID,
        DeployDurationMs:   int32(duration.Milliseconds()),
    })

    d.logger.Info("deployment complete",
        "deployment_id", cfg.Deployment.ID,
        "machine_id", machineID,
        "duration", duration,
    )

    return nil
}

func (d *Deployer) ensureFlyApp(ctx context.Context, app queries.App) (string, error) {
    // Already has Fly app ID
    if app.FlyAppID != nil && *app.FlyAppID != "" {
        return *app.FlyAppID, nil
    }

    // Create new Fly app
    flyAppName := fly.GenerateFlyAppName(app.TeamSlug, app.Slug)

    flyApp, err := d.flyClient.CreateApp(ctx, flyAppName)
    if err != nil {
        return "", err
    }

    // Store Fly app ID
    d.queries.UpdateApp(ctx, queries.UpdateAppParams{
        ID:       app.ID,
        FlyAppID: &flyApp.Name,
    })

    d.logger.Info("created fly app", "fly_app", flyApp.Name)

    return flyApp.Name, nil
}

func (d *Deployer) deployMachine(ctx context.Context, flyAppID string, cfg DeployConfig) (string, error) {
    // Build machine config
    machineConfig := fly.VangoMachineConfig(
        *cfg.Deployment.ImageTag,
        cfg.EnvVars,
        cfg.App.Region,
    )

    // Check for existing machine
    machines, err := d.flyClient.ListMachines(ctx, flyAppID)
    if err != nil {
        return "", err
    }

    if len(machines) > 0 {
        // Update existing machine (blue/green)
        existingMachine := machines[0]
        d.logger.Info("updating existing machine", "machine_id", existingMachine.ID)

        machine, err := d.flyClient.UpdateMachine(ctx, flyAppID, existingMachine.ID, machineConfig)
        if err != nil {
            return "", err
        }
        return machine.ID, nil
    }

    // Create new machine
    d.logger.Info("creating new machine")
    machine, err := d.flyClient.CreateMachine(ctx, flyAppID, machineConfig)
    if err != nil {
        return "", err
    }

    return machine.ID, nil
}

func (d *Deployer) waitForHealthy(ctx context.Context, flyAppID, machineID string) error {
    // Fly handles health checks automatically
    // We poll the machine status to see when it passes

    deadline := time.Now().Add(3 * time.Minute)

    for time.Now().Before(deadline) {
        machine, err := d.flyClient.GetMachine(ctx, flyAppID, machineID)
        if err != nil {
            return err
        }

        // Check if machine is started and has passed checks
        if machine.State == "started" {
            // Additional HTTP health check from our side
            if err := d.checkAppHealth(ctx, flyAppID); err == nil {
                return nil
            }
        }

        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(5 * time.Second):
        }
    }

    return fmt.Errorf("health check timeout")
}

func (d *Deployer) checkAppHealth(ctx context.Context, flyAppID string) error {
    url := fmt.Sprintf("https://%s.fly.dev/health", flyAppID)

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return err
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("health check returned %d", resp.StatusCode)
    }

    return nil
}

func (d *Deployer) updateStatus(ctx context.Context, id uuid.UUID, status string, logs *string) {
    d.queries.UpdateDeploymentStatus(ctx, queries.UpdateDeploymentStatusParams{
        ID:         id,
        Status:     status,
        DeployLogs: logs,
    })
}

func (d *Deployer) failDeploy(ctx context.Context, id uuid.UUID, errMsg string) {
    d.queries.UpdateDeploymentStatus(ctx, queries.UpdateDeploymentStatusParams{
        ID:         id,
        Status:     "deploy_failed",
        DeployLogs: &errMsg,
    })
}
```

---

## Rollback Support

```go
// internal/deploy/rollback.go
package deploy

import (
    "context"
    "fmt"

    "github.com/google/uuid"
)

// Rollback reverts to a previous deployment
func (d *Deployer) Rollback(ctx context.Context, app queries.App, targetDeploymentID uuid.UUID) error {
    // Get target deployment
    targetDeploy, err := d.queries.GetDeployment(ctx, targetDeploymentID)
    if err != nil {
        return fmt.Errorf("deployment not found: %w", err)
    }

    if targetDeploy.ImageTag == nil || *targetDeploy.ImageTag == "" {
        return fmt.Errorf("target deployment has no image")
    }

    if targetDeploy.Status != "live" {
        return fmt.Errorf("can only rollback to previously live deployments")
    }

    // Get env vars (use current, not from old deployment)
    envVars, err := d.getDecryptedEnvVars(ctx, app.ID)
    if err != nil {
        return err
    }

    // Create new deployment record for rollback
    rollbackDeploy, err := d.queries.CreateDeployment(ctx, queries.CreateDeploymentParams{
        AppID:         app.ID,
        Status:        "deploying",
        ImageTag:      targetDeploy.ImageTag,
        CommitSha:     targetDeploy.CommitSha,
        CommitMessage: ptr("Rollback to " + targetDeploymentID.String()[:8]),
    })
    if err != nil {
        return err
    }

    // Deploy the old image
    cfg := DeployConfig{
        App:        app,
        Deployment: rollbackDeploy,
        EnvVars:    envVars,
    }

    return d.Deploy(ctx, cfg)
}

// GetRollbackTargets returns deployments that can be rolled back to
func (d *Deployer) GetRollbackTargets(ctx context.Context, appID uuid.UUID) ([]queries.Deployment, error) {
    return d.queries.GetSuccessfulDeployments(ctx, queries.GetSuccessfulDeploymentsParams{
        AppID: appID,
        Limit: 10,
    })
}
```

---

## Database Queries

```sql
-- internal/database/queries/deployments.sql

-- name: CreateDeployment :one
INSERT INTO deployments (app_id, triggered_by, commit_sha, commit_message, branch, status, image_tag)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetDeployment :one
SELECT * FROM deployments WHERE id = $1;

-- name: GetAppDeployments :many
SELECT * FROM deployments
WHERE app_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: GetSuccessfulDeployments :many
SELECT * FROM deployments
WHERE app_id = $1 AND status = 'live'
ORDER BY created_at DESC
LIMIT $2;

-- name: GetLatestLiveDeployment :one
SELECT * FROM deployments
WHERE app_id = $1 AND status = 'live'
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateDeploymentStatus :exec
UPDATE deployments SET
    status = $2,
    deploy_logs = COALESCE($3, deploy_logs),
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateDeploymentBuildStarted :exec
UPDATE deployments SET
    build_started_at = NOW(),
    status = 'building',
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateDeploymentBuildComplete :exec
UPDATE deployments SET
    status = $2,
    image_tag = $3,
    build_logs = $4,
    commit_sha = $5,
    commit_message = $6,
    build_duration_ms = $7,
    build_finished_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateDeploymentLive :exec
UPDATE deployments SET
    status = 'live',
    machine_id = $2,
    deploy_finished_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: MarkPreviousDeploymentsSuperseded :exec
UPDATE deployments SET
    status = 'superseded',
    updated_at = NOW()
WHERE app_id = $1 AND status = 'live' AND id != $2;
```

---

## Handlers

```go
// internal/handlers/deploy.go (additions)

func (h *Handlers) Rollback(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    session := middleware.GetSession(ctx)
    slug := chi.URLParam(r, "slug")
    targetID := r.FormValue("deployment_id")

    app, err := h.queries.GetAppBySlug(ctx, queries.GetAppBySlugParams{
        TeamID: session.TeamID,
        Slug:   slug,
    })
    if err != nil {
        http.NotFound(w, r)
        return
    }

    deploymentID, err := uuid.Parse(targetID)
    if err != nil {
        http.Error(w, "Invalid deployment ID", http.StatusBadRequest)
        return
    }

    if err := h.deployer.Rollback(ctx, app, deploymentID); err != nil {
        h.logger.Error("rollback failed", "error", err)
        http.Error(w, "Rollback failed: "+err.Error(), http.StatusInternalServerError)
        return
    }

    http.Redirect(w, r, "/apps/"+slug, http.StatusSeeOther)
}
```

---

## Exit Criteria

Phase 5 is complete when:

1. [ ] Fly apps created automatically on first deploy
2. [ ] Machines deployed with correct configuration
3. [ ] Vango-specific settings (no auto-stop, WebSocket support)
4. [ ] Health checks work correctly
5. [ ] Zero-downtime updates (blue/green) work
6. [ ] Deployment status tracked in database
7. [ ] Rollback to previous deployment works
8. [ ] Failed deployments marked correctly
9. [ ] Apps accessible at {fly-app}.fly.dev
10. [ ] Unit tests pass
11. [ ] Integration tests pass

---

## Dependencies

- **Requires**: Phase 4 (build produces images)
- **Required by**: Phase 6 (domains route to deployed apps), Phase 8 (billing tracks deployments)

---

*Phase 5 Specification - Version 1.0*
