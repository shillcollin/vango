# Phase 11: Multi-Region Support

> **Deploy applications to multiple Fly.io regions worldwide**

**Status**: Not Started

---

## Overview

Phase 11 enables users to deploy their applications to multiple Fly.io regions for better latency and redundancy. Users can select primary and secondary regions, and Rhone manages machine placement across regions.

### Goals

1. **Region selection**: Users choose deployment regions
2. **Primary region**: Designate one region as primary
3. **Multi-region deployments**: Deploy to multiple regions simultaneously
4. **Region-aware routing**: Fly handles automatic request routing
5. **Region management UI**: Visual region selector

### Non-Goals

1. Database replication (users handle their own DB)
2. Custom routing rules (use Fly's built-in)
3. Region-specific configurations (env vars are global)
4. Regional pricing differences

---

## Fly.io Regions

```
┌─────────────────────────────────────────────────────────────────────────┐
│                       FLY.IO GLOBAL REGIONS                             │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  NORTH AMERICA                                                          │
│  ├── iad  - Ashburn, Virginia (US East)                                │
│  ├── ord  - Chicago, Illinois                                          │
│  ├── dfw  - Dallas, Texas                                              │
│  ├── den  - Denver, Colorado                                           │
│  ├── lax  - Los Angeles, California                                    │
│  ├── sjc  - San Jose, California                                       │
│  ├── sea  - Seattle, Washington                                        │
│  ├── yyz  - Toronto, Canada                                            │
│  └── yul  - Montreal, Canada                                           │
│                                                                          │
│  SOUTH AMERICA                                                          │
│  ├── gru  - São Paulo, Brazil                                          │
│  ├── gig  - Rio de Janeiro, Brazil                                     │
│  ├── eze  - Buenos Aires, Argentina                                    │
│  └── scl  - Santiago, Chile                                            │
│                                                                          │
│  EUROPE                                                                 │
│  ├── lhr  - London, UK                                                 │
│  ├── cdg  - Paris, France                                              │
│  ├── ams  - Amsterdam, Netherlands                                     │
│  ├── fra  - Frankfurt, Germany                                         │
│  ├── waw  - Warsaw, Poland                                             │
│  ├── mad  - Madrid, Spain                                              │
│  └── arn  - Stockholm, Sweden                                          │
│                                                                          │
│  ASIA PACIFIC                                                           │
│  ├── nrt  - Tokyo, Japan                                               │
│  ├── hkg  - Hong Kong                                                  │
│  ├── sin  - Singapore                                                  │
│  ├── syd  - Sydney, Australia                                          │
│  ├── bom  - Mumbai, India                                              │
│  └── maa  - Chennai, India                                             │
│                                                                          │
│  MIDDLE EAST / AFRICA                                                   │
│  ├── jnb  - Johannesburg, South Africa                                 │
│  └── bah  - Bahrain                                                    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    MULTI-REGION DEPLOYMENT                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  USER REQUEST FLOW                                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │                                                                     ││
│  │  User in Europe                     User in Asia                    ││
│  │       │                                  │                          ││
│  │       ▼                                  ▼                          ││
│  │  Fly.io Anycast                    Fly.io Anycast                   ││
│  │       │                                  │                          ││
│  │       ▼                                  ▼                          ││
│  │  Edge: cdg (Paris)                 Edge: nrt (Tokyo)                ││
│  │       │                                  │                          ││
│  │       ▼                                  ▼                          ││
│  │  Machine: fra                      Machine: nrt                     ││
│  │  (closest region)                  (closest region)                 ││
│  │                                                                     ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  DEPLOYMENT TOPOLOGY                                                     │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │                                                                     ││
│  │  App: my-vango-app                                                  ││
│  │  Primary Region: iad (US East)                                      ││
│  │  Additional Regions: fra (Europe), nrt (Asia)                       ││
│  │                                                                     ││
│  │  ┌─────────────────────────────────────────────────────────────┐   ││
│  │  │                       Fly.io Network                         │   ││
│  │  │                                                              │   ││
│  │  │   ┌─────────┐       ┌─────────┐       ┌─────────┐           │   ││
│  │  │   │   iad   │       │   fra   │       │   nrt   │           │   ││
│  │  │   │ PRIMARY │       │ REPLICA │       │ REPLICA │           │   ││
│  │  │   │         │       │         │       │         │           │   ││
│  │  │   │ Machine │       │ Machine │       │ Machine │           │   ││
│  │  │   │  x 1    │       │  x 1    │       │  x 1    │           │   ││
│  │  │   └─────────┘       └─────────┘       └─────────┘           │   ││
│  │  │        │                 │                 │                 │   ││
│  │  │        └─────────────────┴─────────────────┘                 │   ││
│  │  │                          │                                   │   ││
│  │  │                   WireGuard Mesh                             │   ││
│  │  │                  (private network)                           │   ││
│  │  │                                                              │   ││
│  │  └──────────────────────────────────────────────────────────────┘   ││
│  │                                                                     ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Database Schema

```sql
-- internal/database/migrations/008_regions.up.sql

-- Add region columns to apps
ALTER TABLE apps ADD COLUMN IF NOT EXISTS primary_region VARCHAR(10) DEFAULT 'iad';
ALTER TABLE apps ADD COLUMN IF NOT EXISTS regions VARCHAR(255) DEFAULT 'iad'; -- Comma-separated

-- Region definitions (static data)
CREATE TABLE IF NOT EXISTS regions (
    code VARCHAR(10) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    city VARCHAR(100) NOT NULL,
    country VARCHAR(100) NOT NULL,
    continent VARCHAR(50) NOT NULL,
    latitude DECIMAL(10, 6),
    longitude DECIMAL(10, 6),
    available BOOLEAN DEFAULT true
);

-- Insert region data
INSERT INTO regions (code, name, city, country, continent, latitude, longitude) VALUES
-- North America
('iad', 'US East', 'Ashburn', 'United States', 'North America', 38.9519, -77.4480),
('ord', 'US Central', 'Chicago', 'United States', 'North America', 41.9742, -87.9073),
('dfw', 'US South', 'Dallas', 'United States', 'North America', 32.8998, -97.0403),
('den', 'US Mountain', 'Denver', 'United States', 'North America', 39.8561, -104.6737),
('lax', 'US West', 'Los Angeles', 'United States', 'North America', 33.9416, -118.4085),
('sjc', 'US West', 'San Jose', 'United States', 'North America', 37.3639, -121.9289),
('sea', 'US Northwest', 'Seattle', 'United States', 'North America', 47.4502, -122.3088),
('yyz', 'Canada East', 'Toronto', 'Canada', 'North America', 43.6777, -79.6248),
('yul', 'Canada East', 'Montreal', 'Canada', 'North America', 45.4657, -73.7455),
-- South America
('gru', 'South America', 'São Paulo', 'Brazil', 'South America', -23.4356, -46.4731),
('gig', 'South America', 'Rio de Janeiro', 'Brazil', 'South America', -22.8100, -43.2505),
('eze', 'South America', 'Buenos Aires', 'Argentina', 'South America', -34.8150, -58.5348),
('scl', 'South America', 'Santiago', 'Chile', 'South America', -33.3930, -70.7858),
-- Europe
('lhr', 'Europe West', 'London', 'United Kingdom', 'Europe', 51.4700, -0.4543),
('cdg', 'Europe West', 'Paris', 'France', 'Europe', 49.0097, 2.5479),
('ams', 'Europe West', 'Amsterdam', 'Netherlands', 'Europe', 52.3105, 4.7683),
('fra', 'Europe Central', 'Frankfurt', 'Germany', 'Europe', 50.0379, 8.5622),
('waw', 'Europe East', 'Warsaw', 'Poland', 'Europe', 52.1672, 20.9679),
('mad', 'Europe South', 'Madrid', 'Spain', 'Europe', 40.4983, -3.5676),
('arn', 'Europe North', 'Stockholm', 'Sweden', 'Europe', 59.6519, 17.9186),
-- Asia Pacific
('nrt', 'Asia East', 'Tokyo', 'Japan', 'Asia', 35.7720, 140.3929),
('hkg', 'Asia East', 'Hong Kong', 'Hong Kong', 'Asia', 22.3080, 113.9185),
('sin', 'Asia Southeast', 'Singapore', 'Singapore', 'Asia', 1.3644, 103.9915),
('syd', 'Oceania', 'Sydney', 'Australia', 'Oceania', -33.9399, 151.1753),
('bom', 'Asia South', 'Mumbai', 'India', 'Asia', 19.0896, 72.8656),
('maa', 'Asia South', 'Chennai', 'India', 'Asia', 12.9941, 80.1709),
-- Middle East / Africa
('jnb', 'Africa', 'Johannesburg', 'South Africa', 'Africa', -26.1367, 28.2411),
('bah', 'Middle East', 'Bahrain', 'Bahrain', 'Middle East', 26.2708, 50.6336)
ON CONFLICT (code) DO NOTHING;

-- Track machine regions
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS regions_deployed VARCHAR(255);
```

---

## Region Service

```go
// internal/regions/service.go
package regions

import (
    "context"
    "fmt"
    "strings"

    "github.com/google/uuid"
    "github.com/vangoframework/rhone/internal/database/queries"
    "github.com/vangoframework/rhone/internal/fly"
)

type Service struct {
    flyClient *fly.Client
    queries   *queries.Queries
    logger    *slog.Logger
}

func NewService(flyClient *fly.Client, queries *queries.Queries, logger *slog.Logger) *Service {
    return &Service{
        flyClient: flyClient,
        queries:   queries,
        logger:    logger,
    }
}

// Region represents a deployment region
type Region struct {
    Code      string  `json:"code"`
    Name      string  `json:"name"`
    City      string  `json:"city"`
    Country   string  `json:"country"`
    Continent string  `json:"continent"`
    Latitude  float64 `json:"latitude"`
    Longitude float64 `json:"longitude"`
    Available bool    `json:"available"`
}

// GetAvailableRegions returns all available regions
func (s *Service) GetAvailableRegions(ctx context.Context) ([]Region, error) {
    regions, err := s.queries.GetAvailableRegions(ctx)
    if err != nil {
        return nil, err
    }

    result := make([]Region, len(regions))
    for i, r := range regions {
        result[i] = Region{
            Code:      r.Code,
            Name:      r.Name,
            City:      r.City,
            Country:   r.Country,
            Continent: r.Continent,
            Latitude:  r.Latitude,
            Longitude: r.Longitude,
            Available: r.Available,
        }
    }

    return result, nil
}

// GetRegionsByContinent groups regions by continent
func (s *Service) GetRegionsByContinent(ctx context.Context) (map[string][]Region, error) {
    regions, err := s.GetAvailableRegions(ctx)
    if err != nil {
        return nil, err
    }

    byContinent := make(map[string][]Region)
    for _, r := range regions {
        byContinent[r.Continent] = append(byContinent[r.Continent], r)
    }

    return byContinent, nil
}

// UpdateAppRegions updates the regions for an app
func (s *Service) UpdateAppRegions(ctx context.Context, appID uuid.UUID, primaryRegion string, additionalRegions []string) error {
    // Validate primary region
    if !s.isValidRegion(ctx, primaryRegion) {
        return fmt.Errorf("invalid primary region: %s", primaryRegion)
    }

    // Validate additional regions
    allRegions := []string{primaryRegion}
    for _, r := range additionalRegions {
        if r != primaryRegion && s.isValidRegion(ctx, r) {
            allRegions = append(allRegions, r)
        }
    }

    // Update app
    return s.queries.UpdateAppRegions(ctx, queries.UpdateAppRegionsParams{
        ID:            appID,
        PrimaryRegion: primaryRegion,
        Regions:       strings.Join(allRegions, ","),
    })
}

func (s *Service) isValidRegion(ctx context.Context, code string) bool {
    region, err := s.queries.GetRegion(ctx, code)
    return err == nil && region.Available
}

// GetAppRegions returns the configured regions for an app
func (s *Service) GetAppRegions(ctx context.Context, appID uuid.UUID) (primary string, additional []string, err error) {
    app, err := s.queries.GetApp(ctx, appID)
    if err != nil {
        return "", nil, err
    }

    primary = app.PrimaryRegion
    if app.Regions != "" {
        all := strings.Split(app.Regions, ",")
        for _, r := range all {
            if r != primary {
                additional = append(additional, r)
            }
        }
    }

    return primary, additional, nil
}
```

---

## Multi-Region Deployer

```go
// internal/deploy/multiregion.go
package deploy

import (
    "context"
    "fmt"
    "strings"
    "sync"

    "github.com/google/uuid"
    "github.com/vangoframework/rhone/internal/database/queries"
    "github.com/vangoframework/rhone/internal/fly"
)

// DeployMultiRegion deploys an app to multiple regions
func (d *Deployer) DeployMultiRegion(ctx context.Context, app queries.App, deploymentID uuid.UUID, imageTag string) error {
    if app.FlyAppID == nil {
        return fmt.Errorf("app not initialized on Fly")
    }

    // Parse regions
    regions := strings.Split(app.Regions, ",")
    if len(regions) == 0 {
        regions = []string{app.PrimaryRegion}
    }

    d.logger.Info("deploying to multiple regions",
        "app_id", app.ID,
        "regions", regions,
        "primary", app.PrimaryRegion,
    )

    // Get existing machines by region
    existingMachines, err := d.flyClient.ListMachines(ctx, *app.FlyAppID)
    if err != nil {
        return fmt.Errorf("list machines: %w", err)
    }

    machinesByRegion := make(map[string][]fly.Machine)
    for _, m := range existingMachines {
        machinesByRegion[m.Region] = append(machinesByRegion[m.Region], m)
    }

    // Deploy to each region concurrently
    var wg sync.WaitGroup
    errors := make(chan error, len(regions))
    deployedRegions := make(chan string, len(regions))

    for _, region := range regions {
        wg.Add(1)
        go func(region string) {
            defer wg.Done()

            isPrimary := region == app.PrimaryRegion
            err := d.deployToRegion(ctx, app, deploymentID, imageTag, region, isPrimary, machinesByRegion[region])
            if err != nil {
                errors <- fmt.Errorf("region %s: %w", region, err)
                return
            }
            deployedRegions <- region
        }(region)
    }

    wg.Wait()
    close(errors)
    close(deployedRegions)

    // Collect results
    var deployed []string
    for r := range deployedRegions {
        deployed = append(deployed, r)
    }

    var deployErrors []error
    for err := range errors {
        deployErrors = append(deployErrors, err)
    }

    // Update deployment record
    d.queries.UpdateDeploymentRegions(ctx, queries.UpdateDeploymentRegionsParams{
        ID:              deploymentID,
        RegionsDeployed: strings.Join(deployed, ","),
    })

    if len(deployErrors) > 0 {
        // Partial success - some regions deployed
        if len(deployed) > 0 {
            d.logger.Warn("partial multi-region deployment",
                "deployed", deployed,
                "errors", len(deployErrors),
            )
            return nil
        }
        return fmt.Errorf("all regions failed: %v", deployErrors[0])
    }

    d.logger.Info("multi-region deployment complete",
        "app_id", app.ID,
        "regions", deployed,
    )

    return nil
}

// deployToRegion deploys to a single region
func (d *Deployer) deployToRegion(ctx context.Context, app queries.App, deploymentID uuid.UUID, imageTag, region string, isPrimary bool, existingMachines []fly.Machine) error {
    d.logger.Info("deploying to region",
        "app_id", app.ID,
        "region", region,
        "is_primary", isPrimary,
        "existing_machines", len(existingMachines),
    )

    // Determine machine config
    config := d.buildMachineConfig(app, imageTag, region, isPrimary)

    if len(existingMachines) == 0 {
        // Create new machine in region
        machine, err := d.flyClient.CreateMachine(ctx, *app.FlyAppID, fly.CreateMachineRequest{
            Region: region,
            Config: config,
        })
        if err != nil {
            return fmt.Errorf("create machine: %w", err)
        }

        // Wait for health
        return d.waitForMachineHealth(ctx, *app.FlyAppID, machine.ID)
    }

    // Blue/green deployment for existing machine
    oldMachine := existingMachines[0]

    // Create new machine
    newMachine, err := d.flyClient.CreateMachine(ctx, *app.FlyAppID, fly.CreateMachineRequest{
        Region: region,
        Config: config,
    })
    if err != nil {
        return fmt.Errorf("create new machine: %w", err)
    }

    // Wait for health
    if err := d.waitForMachineHealth(ctx, *app.FlyAppID, newMachine.ID); err != nil {
        // Rollback: destroy new machine
        d.flyClient.DestroyMachine(ctx, *app.FlyAppID, newMachine.ID)
        return fmt.Errorf("health check failed: %w", err)
    }

    // Stop and destroy old machine
    d.flyClient.StopMachine(ctx, *app.FlyAppID, oldMachine.ID)
    d.flyClient.DestroyMachine(ctx, *app.FlyAppID, oldMachine.ID)

    return nil
}

// buildMachineConfig creates the machine configuration
func (d *Deployer) buildMachineConfig(app queries.App, imageTag, region string, isPrimary bool) fly.MachineConfig {
    // Primary region gets full resources, replicas get smaller
    cpuKind := "shared"
    cpus := 1
    memoryMB := 256

    if isPrimary {
        // Primary can be larger if needed
        memoryMB = 512
    }

    return fly.MachineConfig{
        Image: imageTag,
        Guest: fly.GuestConfig{
            CPUKind:  cpuKind,
            CPUs:     cpus,
            MemoryMB: memoryMB,
        },
        Services: []fly.MachineService{
            {
                Protocol:     "tcp",
                InternalPort: 8080,
                Ports: []fly.MachinePort{
                    {Port: 80, Handlers: []string{"http"}},
                    {Port: 443, Handlers: []string{"http", "tls"}},
                },
                Concurrency: fly.MachineConcurrency{
                    Type:      "connections",
                    HardLimit: 100,
                    SoftLimit: 80,
                },
            },
        },
        Checks: map[string]fly.MachineCheck{
            "health": {
                Type:     "http",
                Port:     8080,
                Path:     "/health",
                Interval: "10s",
                Timeout:  "5s",
            },
        },
        Env: map[string]string{
            "FLY_REGION":     region,
            "PRIMARY_REGION": app.PrimaryRegion,
        },
        AutoDestroy: false,
        Restart: fly.RestartConfig{
            Policy: "always",
        },
    }
}

// ScaleRegion adds or removes a region from an app
func (d *Deployer) ScaleRegion(ctx context.Context, app queries.App, region string, add bool) error {
    if app.FlyAppID == nil {
        return fmt.Errorf("app not deployed")
    }

    if add {
        // Deploy to new region
        // Get current image
        machines, err := d.flyClient.ListMachines(ctx, *app.FlyAppID)
        if err != nil || len(machines) == 0 {
            return fmt.Errorf("no existing machines")
        }

        imageTag := machines[0].Config.Image
        isPrimary := region == app.PrimaryRegion

        return d.deployToRegion(ctx, app, uuid.Nil, imageTag, region, isPrimary, nil)
    }

    // Remove region - destroy all machines in region
    machines, err := d.flyClient.ListMachines(ctx, *app.FlyAppID)
    if err != nil {
        return err
    }

    for _, m := range machines {
        if m.Region == region {
            if err := d.flyClient.StopMachine(ctx, *app.FlyAppID, m.ID); err != nil {
                d.logger.Warn("failed to stop machine", "machine_id", m.ID, "error", err)
            }
            if err := d.flyClient.DestroyMachine(ctx, *app.FlyAppID, m.ID); err != nil {
                d.logger.Warn("failed to destroy machine", "machine_id", m.ID, "error", err)
            }
        }
    }

    return nil
}
```

---

## HTTP Handlers

```go
// internal/handlers/regions.go
package handlers

import (
    "encoding/json"
    "net/http"
    "strings"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
)

// ListRegions returns all available regions
func (h *Handlers) ListRegions(w http.ResponseWriter, r *http.Request) {
    regions, err := h.regionService.GetRegionsByContinent(r.Context())
    if err != nil {
        h.logger.Error("failed to list regions", "error", err)
        http.Error(w, "Failed to get regions", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(regions)
}

// GetAppRegions returns regions for an app
func (h *Handlers) GetAppRegions(w http.ResponseWriter, r *http.Request) {
    appID, err := uuid.Parse(chi.URLParam(r, "appID"))
    if err != nil {
        http.Error(w, "Invalid app ID", http.StatusBadRequest)
        return
    }

    app, err := h.getAppWithAccess(r.Context(), appID)
    if err != nil {
        http.Error(w, "App not found", http.StatusNotFound)
        return
    }

    primary, additional, err := h.regionService.GetAppRegions(r.Context(), app.ID)
    if err != nil {
        http.Error(w, "Failed to get regions", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "primary":    primary,
        "additional": additional,
    })
}

// UpdateAppRegions updates regions for an app
func (h *Handlers) UpdateAppRegions(w http.ResponseWriter, r *http.Request) {
    appID, err := uuid.Parse(chi.URLParam(r, "appID"))
    if err != nil {
        http.Error(w, "Invalid app ID", http.StatusBadRequest)
        return
    }

    app, err := h.getAppWithAccess(r.Context(), appID)
    if err != nil {
        http.Error(w, "App not found", http.StatusNotFound)
        return
    }

    // Check permissions (admin or owner required for region changes)
    membership := GetTeamMembership(r.Context())
    if !domain.Role(membership.Role).CanManageEnvVars() {
        http.Error(w, "Insufficient permissions", http.StatusForbidden)
        return
    }

    var req struct {
        PrimaryRegion     string   `json:"primary_region"`
        AdditionalRegions []string `json:"additional_regions"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // Update database
    if err := h.regionService.UpdateAppRegions(r.Context(), app.ID, req.PrimaryRegion, req.AdditionalRegions); err != nil {
        h.logger.Error("failed to update regions", "error", err)
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // If app is deployed, scale regions
    if app.FlyAppID != nil {
        currentRegions := strings.Split(app.Regions, ",")
        newRegions := append([]string{req.PrimaryRegion}, req.AdditionalRegions...)

        // Add new regions
        for _, r := range newRegions {
            if !contains(currentRegions, r) {
                go h.deployer.ScaleRegion(r.Context(), *app, r, true)
            }
        }

        // Remove old regions
        for _, r := range currentRegions {
            if !contains(newRegions, r) {
                go h.deployer.ScaleRegion(r.Context(), *app, r, false)
            }
        }
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
```

---

## Region Selector UI

```go
// internal/templates/components/region_selector.templ
package components

import "github.com/vangoframework/rhone/internal/regions"

templ RegionSelector(appID string, primaryRegion string, selectedRegions []string, regionsByContinent map[string][]regions.Region) {
    <div class="region-selector" id="region-selector">
        <h3 class="text-lg font-semibold mb-4">Deployment Regions</h3>

        <div class="mb-6">
            <label class="block text-sm font-medium text-gray-700 mb-2">
                Primary Region
            </label>
            <p class="text-sm text-gray-500 mb-2">
                The main region where your app runs. This region handles the primary workload.
            </p>
            <select
                name="primary_region"
                id="primary-region"
                class="w-full px-3 py-2 border rounded-lg"
            >
                for continent, regs := range regionsByContinent {
                    <optgroup label={ continent }>
                        for _, r := range regs {
                            <option
                                value={ r.Code }
                                selected?={ r.Code == primaryRegion }
                            >
                                { r.City }, { r.Country } ({ r.Code })
                            </option>
                        }
                    </optgroup>
                }
            </select>
        </div>

        <div class="mb-6">
            <label class="block text-sm font-medium text-gray-700 mb-2">
                Additional Regions
            </label>
            <p class="text-sm text-gray-500 mb-2">
                Deploy to additional regions for lower latency worldwide. Each region adds to your billing.
            </p>

            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                for continent, regs := range regionsByContinent {
                    <div class="border rounded-lg p-4">
                        <h4 class="font-medium text-sm text-gray-700 mb-2">{ continent }</h4>
                        <div class="space-y-2">
                            for _, r := range regs {
                                <label class="flex items-center gap-2 cursor-pointer">
                                    <input
                                        type="checkbox"
                                        name="additional_regions"
                                        value={ r.Code }
                                        class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                                        checked?={ isSelected(r.Code, selectedRegions) }
                                        disabled?={ r.Code == primaryRegion }
                                    />
                                    <span class={ "text-sm", templ.KV("text-gray-400", r.Code == primaryRegion) }>
                                        { r.City } ({ r.Code })
                                        if r.Code == primaryRegion {
                                            <span class="text-xs text-blue-600">(primary)</span>
                                        }
                                    </span>
                                </label>
                            }
                        </div>
                    </div>
                }
            </div>
        </div>

        <!-- World map visualization -->
        <div class="mb-6 p-4 bg-gray-50 rounded-lg">
            <div class="text-sm font-medium text-gray-700 mb-2">Region Map</div>
            <div id="region-map" class="h-64 bg-gray-200 rounded flex items-center justify-center text-gray-500">
                <!-- Could add an interactive map here -->
                <div class="text-center">
                    <div class="text-2xl mb-2">🌍</div>
                    <div>
                        { fmt.Sprintf("%d region(s) selected", 1 + len(selectedRegions)) }
                    </div>
                </div>
            </div>
        </div>

        <div class="flex justify-end gap-3">
            <button
                type="button"
                class="px-4 py-2 border rounded-lg hover:bg-gray-50"
                onclick="resetRegions()"
            >
                Reset
            </button>
            <button
                type="button"
                class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                hx-post={ fmt.Sprintf("/apps/%s/regions", appID) }
                hx-include="#primary-region, [name=additional_regions]:checked"
                hx-swap="none"
                hx-on::after-request="showToast('Regions updated')"
            >
                Save Regions
            </button>
        </div>
    </div>

    <script>
        function resetRegions() {
            document.getElementById('primary-region').value = '{ primaryRegion }';
            document.querySelectorAll('[name=additional_regions]').forEach(cb => {
                cb.checked = { fmt.Sprintf("%v", selectedRegions) }.includes(cb.value);
            });
        }
    </script>
}

func isSelected(code string, selected []string) bool {
    for _, s := range selected {
        if s == code {
            return true
        }
    }
    return false
}
```

### Region Status Component

```go
// internal/templates/components/region_status.templ
package components

templ RegionStatus(appID string, regions []MachineRegionStatus) {
    <div class="region-status">
        <h4 class="text-sm font-medium text-gray-700 mb-3">Active Regions</h4>

        <div class="space-y-2">
            for _, r := range regions {
                <div class="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
                    <div class="flex items-center gap-3">
                        <div class={ "w-3 h-3 rounded-full", statusColor(r.Status) }></div>
                        <div>
                            <div class="font-medium text-sm">{ r.City } ({ r.Code })</div>
                            <div class="text-xs text-gray-500">
                                { r.MachineCount } machine(s)
                                if r.IsPrimary {
                                    · Primary
                                }
                            </div>
                        </div>
                    </div>
                    <div class="text-right text-sm">
                        <div class="text-gray-600">{ r.Status }</div>
                        if r.AvgLatencyMs > 0 {
                            <div class="text-xs text-gray-400">{ fmt.Sprintf("%.0fms", r.AvgLatencyMs) } avg</div>
                        }
                    </div>
                </div>
            }
        </div>

        if len(regions) == 0 {
            <div class="text-center text-gray-500 py-4">
                No regions deployed yet
            </div>
        }
    </div>
}

type MachineRegionStatus struct {
    Code         string
    City         string
    Status       string
    MachineCount int
    IsPrimary    bool
    AvgLatencyMs float64
}

func statusColor(status string) string {
    switch status {
    case "running":
        return "bg-green-500"
    case "stopped":
        return "bg-gray-400"
    case "starting":
        return "bg-yellow-500"
    default:
        return "bg-red-500"
    }
}
```

---

## Routes

```go
// Add to router setup

// Region routes
r.Route("/regions", func(r chi.Router) {
    r.Get("/", h.ListRegions)
})

r.Route("/apps/{appID}/regions", func(r chi.Router) {
    r.Use(h.RequireAuth)
    r.Get("/", h.GetAppRegions)
    r.Post("/", h.UpdateAppRegions)
    r.Get("/status", h.GetRegionStatus)
})
```

---

## Exit Criteria

Phase 11 is complete when:

1. [ ] Region selector UI displays all Fly regions
2. [ ] Primary region can be selected
3. [ ] Additional regions can be toggled
4. [ ] Deployments go to all selected regions
5. [ ] Blue/green works per-region
6. [ ] Adding a region deploys existing image
7. [ ] Removing a region destroys machines
8. [ ] Region status shows machine health
9. [ ] Primary region env var set correctly
10. [ ] Pro plan required for multi-region (billing check)

---

## Dependencies

- **Requires**: Phase 5 (deployment), Phase 8 (billing for plan check)
- **Required by**: Phase 12 (production readiness)

---

*Phase 11 Specification - Version 1.0*
