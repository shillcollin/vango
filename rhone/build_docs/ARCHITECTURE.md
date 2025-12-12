# Rhone System Architecture

> **Complete technical architecture for the Rhone platform**

---

## Overview

Rhone is a control plane that orchestrates Fly.io resources to provide a seamless deployment experience for Vango applications. It does not manage servers directly—it manages Fly.io primitives (Machines, Certificates, Volumes) via their APIs.

---

## System Components

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              INTERNET                                       │
│                                                                             │
│    Users                   GitHub                    Stripe                 │
│      │                        │                         │                   │
│      │ HTTPS                  │ Webhooks                │ Webhooks          │
│      ▼                        ▼                         ▼                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                         RHONE CONTROL PLANE                                 │
│                         (rhone.app on Fly.io)                               │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                           Load Balancer                                 ││
│  │                    (Fly.io Anycast + Edge TLS)                          ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                    │                                        │
│                                    ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                         Go Application Server                           ││
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐    ││
│  │  │   Chi        │ │   Templ      │ │   Auth       │ │   Handlers   │    ││
│  │  │   Router     │ │   Templates  │ │   Middleware │ │   (HTMX)     │    ││
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘    ││
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐    ││
│  │  │   Fly.io     │ │   Build      │ │   Deploy     │ │   Billing    │    ││
│  │  │   Client     │ │   Service    │ │   Service    │ │   Service    │    ││
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘    ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                │                │                │                          │
│                ▼                ▼                ▼                          │
│  ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐             │
│  │   Neon Postgres  │ │   Fly Machines   │ │   Stripe API     │             │
│  │   (Database)     │ │   API            │ │                  │             │
│  └──────────────────┘ └──────────────────┘ └──────────────────┘             │
│                                │                                            │
└────────────────────────────────┼────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           BUILD INFRASTRUCTURE                              │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                     BuildKit Daemon (Rootless)                          ││
│  │                     rhone-builder.fly.dev                               ││
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐                     ││
│  │  │  Railpack    │ │  BuildKit    │ │  Registry    │                     ││
│  │  │  Analyzer    │─│  Builder     │─│  Push        │                     ││
│  │  └──────────────┘ └──────────────┘ └──────────────┘                     ││
│  │         │                │                │                             ││
│  │         ▼                ▼                ▼                             ││
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐                     ││
│  │  │  GitHub      │ │  Build       │ │  registry    │                     ││
│  │  │  Clone       │ │  Cache Vol   │ │  .fly.io     │                     ││
│  │  └──────────────┘ └──────────────┘ └──────────────┘                     ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           USER APP FLEET                                    │
│                         (*.rhone.app on Fly.io)                             │
│                                                                             │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐        │
│  │ app-1        │ │ app-2        │ │ app-3        │ │ app-N        │        │
│  │ .rhone.app   │ │ .rhone.app   │ │ .rhone.app   │ │ .rhone.app   │        │
│  │              │ │              │ │              │ │              │        │
│  │ Firecracker  │ │ Firecracker  │ │ Firecracker  │ │ Firecracker  │        │
│  │ MicroVM      │ │ MicroVM      │ │ MicroVM      │ │ MicroVM      │        │
│  │              │ │              │ │              │ │              │        │
│  │ Vango App    │ │ Vango App    │ │ Vango App    │ │ Vango App    │        │
│  │ WebSocket    │ │ WebSocket    │ │ WebSocket    │ │ WebSocket    │        │
│  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Request Flow

### 1. User Visits Dashboard (rhone.app)

```
Browser ──HTTPS──▶ Fly.io Edge ──▶ Rhone Go Server
                                          │
                    ┌─────────────────────┴─────────────────────┐
                    │                                           │
                    ▼                                           ▼
            Session Cookie?                              GitHub OAuth
                    │                                           │
              ┌─────┴─────┐                                     │
              │           │                                     │
              ▼           ▼                                     ▼
          Valid       Invalid                           Redirect to
          Session     Session                           github.com
              │           │                                     │
              ▼           ▼                                     │
         Render       Redirect                                  │
         Dashboard    to Login ◀────────────────────────────────┘
```

### 2. User Deploys App

```
┌─────────────────────────────────────────────────────────────────────────┐
│ DEPLOYMENT PIPELINE                                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  1. TRIGGER                                                             │
│     ┌─────────────┐                                                     │
│     │ User clicks │───▶ POST /apps/{id}/deploy                          │
│     │ "Deploy"    │                                                     │
│     └─────────────┘                                                     │
│            │                                                            │
│            ▼                                                            │
│  2. CLONE                                                               │
│     ┌─────────────┐     ┌─────────────┐                                 │
│     │ Get GitHub  │────▶│ Clone repo  │                                 │
│     │ App Token   │     │ (specific   │                                 │
│     │ (1hr valid) │     │  commit)    │                                 │
│     └─────────────┘     └─────────────┘                                 │
│                                │                                        │
│                                ▼                                        │
│  3. ANALYZE                                                             │
│     ┌─────────────┐                                                     │
│     │ Railpack    │                                                     │
│     │ detect lang │────▶ Generate build plan                            │
│     │ & framework │      (railpack-plan.json)                           │
│     └─────────────┘                                                     │
│                                │                                        │
│                                ▼                                        │
│  4. BUILD                                                               │
│     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐             │
│     │ BuildKit    │────▶│ Execute     │────▶│ Push image  │             │
│     │ (rootless)  │     │ Dockerfile  │     │ to registry │             │
│     │             │     │ layers      │     │ .fly.io     │             │
│     └─────────────┘     └─────────────┘     └─────────────┘             │
│                                                     │                   │
│                                                     ▼                   │
│  5. DEPLOY                                                              │
│     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐             │
│     │ Create/     │────▶│ Wait for    │────▶│ Route       │             │
│     │ Update      │     │ health      │     │ traffic     │             │
│     │ Fly Machine │     │ check pass  │     │ to new VM   │             │
│     └─────────────┘     └─────────────┘     └─────────────┘             │
│                                                     │                   │
│                                                     ▼                   │
│  6. LIVE                                                                │
│     ┌─────────────────────────────────────────────────────┐             │
│     │ App available at {slug}.rhone.app                   │             │
│     │ Old machine terminated (if blue/green)              │             │
│     │ Deployment marked as "live"                         │             │
│     └─────────────────────────────────────────────────────┘             │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3. User App Receives Traffic

```
Browser ──HTTPS──▶ *.rhone.app
                        │
                        ▼
               Fly.io Edge (Anycast)
                        │
                        ├── TLS Termination
                        ├── HTTP/2 Upgrade
                        │
                        ▼
               Fly.io Proxy
                        │
                        ├── Route by Host header
                        ├── Sticky sessions (for WebSocket)
                        │
                        ▼
               User's Fly Machine
                        │
                        ├── Vango Server (port 8080)
                        ├── WebSocket at /_vango/ws
                        │
                        ▼
               Vango Session (in RAM)
```

---

## Data Model

### Entity Relationships

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           ENTITY RELATIONSHIPS                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────┐         ┌──────────┐         ┌──────────┐                │
│  │   User   │────────▶│  Team    │◀────────│  User    │                │
│  │          │  owns   │  Member  │  member │          │                │
│  └──────────┘         └──────────┘         └──────────┘                │
│       │                    │                                             │
│       │                    │ has many                                    │
│       │                    ▼                                             │
│       │              ┌──────────┐                                       │
│       │              │   Team   │                                       │
│       │              │          │                                       │
│       │              └──────────┘                                       │
│       │                    │                                             │
│       │                    │ has many                                    │
│       │                    ▼                                             │
│       │    ┌──────────────────────────────┐                             │
│       │    │                              │                              │
│       │    ▼                              ▼                              │
│       │  ┌──────────┐              ┌──────────┐                         │
│       │  │   App    │              │  GitHub  │                         │
│       │  │          │              │  Install │                         │
│       │  └──────────┘              └──────────┘                         │
│       │       │                                                          │
│       │       │ has many                                                 │
│       │       ├──────────────┬──────────────┬──────────────┐            │
│       │       ▼              ▼              ▼              ▼            │
│       │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│       │  │ Deploy   │  │  Env     │  │  Domain  │  │  Usage   │        │
│       │  │ ment     │  │  Var     │  │          │  │  Record  │        │
│       │  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
│       │                                                                  │
│       │  triggers                                                        │
│       └──────────────────────────────────────────────────────────────▶  │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Database Tables

```sql
-- Core entities
users              -- GitHub-authenticated users
teams              -- Organizations/workspaces
team_members       -- User-team relationship with roles

-- GitHub integration
github_installations  -- GitHub App installations per team

-- Application management
apps               -- Vango applications
env_vars           -- Encrypted environment variables
domains            -- Custom domain configurations
deployments        -- Deployment history and status

-- Billing
usage_records      -- Metered usage for billing
```

---

## Security Model

### Authentication Layers

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           AUTHENTICATION FLOW                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  LAYER 1: User Authentication (GitHub OAuth)                            │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ User ──▶ GitHub OAuth ──▶ Access Token ──▶ Fetch User Profile       ││
│  │                                              │                       ││
│  │                                              ▼                       ││
│  │                                    Create/Update User in DB         ││
│  │                                              │                       ││
│  │                                              ▼                       ││
│  │                                    Set Secure Session Cookie        ││
│  │                                    (HTTPOnly, Secure, SameSite)     ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  LAYER 2: Repository Access (GitHub App)                                │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ User installs GitHub App on account/org                             ││
│  │        │                                                             ││
│  │        ▼                                                             ││
│  │ GitHub sends installation_id to our webhook                         ││
│  │        │                                                             ││
│  │        ▼                                                             ││
│  │ When deploying: Exchange installation_id for temporary token        ││
│  │ POST /app/installations/{id}/access_tokens                          ││
│  │        │                                                             ││
│  │        ▼                                                             ││
│  │ Token valid for 1 hour, scoped ONLY to installed repos              ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  LAYER 3: Team Authorization                                            │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ Every request checks:                                                ││
│  │   1. Is user authenticated?                                          ││
│  │   2. Is user member of requested team?                               ││
│  │   3. Does user have required role? (owner/admin/member)              ││
│  │                                                                      ││
│  │ Permissions:                                                         ││
│  │   owner  - Full control, billing, delete team                        ││
│  │   admin  - Manage apps, invite members                               ││
│  │   member - Deploy apps, view logs                                    ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Encryption

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           ENCRYPTION STRATEGY                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  AT REST                                                                 │
│  ──────────────────────────────────────────────────────────────────     │
│  • Environment variables: AES-256-GCM encrypted before storage          │
│  • Database: Neon provides encryption at rest                            │
│  • Fly volumes: Encrypted with keys in secure storage                    │
│                                                                          │
│  IN TRANSIT                                                              │
│  ──────────────────────────────────────────────────────────────────     │
│  • All external traffic: TLS 1.3                                         │
│  • Fly internal network: WireGuard encrypted                             │
│  • Database connection: TLS required                                     │
│                                                                          │
│  SECRETS                                                                 │
│  ──────────────────────────────────────────────────────────────────     │
│  • GitHub App private key: Fly secrets                                   │
│  • Stripe keys: Fly secrets                                              │
│  • Encryption key: Fly secrets (never in code)                           │
│  • Session secret: Fly secrets                                           │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Vango-Specific Architecture

### Why Vango Apps Are Different

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    STATELESS VS STATEFUL APPS                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  NEXT.JS (Stateless)                     VANGO (Stateful)               │
│  ─────────────────────                   ─────────────────               │
│                                                                          │
│  Request ──▶ Any Server                  Connection ──▶ Specific Server │
│           │                                           │                  │
│           ▼                                           ▼                  │
│     Process Request                            Open WebSocket            │
│           │                                           │                  │
│           ▼                                           ▼                  │
│     Fetch from DB                              Session in RAM            │
│           │                                    (signals, state)          │
│           ▼                                           │                  │
│     Return HTML                                       │                  │
│           │                                    ┌──────┴──────┐           │
│           ▼                                    │             │           │
│     FORGET USER ◀────────┐                    ▼             ▼           │
│     (stateless)          │              User Events    Server Push      │
│                          │              (clicks, etc)  (patches)        │
│                          │                    │             │           │
│     Next request can     │                    └──────┬──────┘           │
│     go to ANY server ────┘                          │                   │
│                                                      │                   │
│                                              MAINTAIN SESSION            │
│                                              (keep connection open)      │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Rhone's Vango-Aware Configuration

```go
// Machine configuration for Vango apps
type VangoMachineConfig struct {
    // Never scale to zero while connections exist
    AutoStop: "off",

    // Start when first request arrives
    AutoStart: true,

    // Keep at least one machine running
    MinMachinesRunning: 1,

    // Health check must account for WebSocket
    HealthCheck: HealthCheckConfig{
        Type:     "http",
        Path:     "/health",
        Interval: "10s",
        Timeout:  "5s",
        // Don't kill machine just because health check fails once
        GracePeriod: "30s",
    },

    // Graceful shutdown for WebSocket drain
    StopConfig: StopConfig{
        // Give connections time to close gracefully
        Timeout: "30s",
        // Signal to use (SIGTERM allows cleanup)
        Signal: "SIGTERM",
    },
}
```

### Deployment "Blink" Handling

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    VANGO DEPLOYMENT STRATEGY                             │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  BEFORE DEPLOY                                                           │
│  ┌────────────────────────────────────────────────┐                     │
│  │  Machine v1 (running)                          │                     │
│  │  ├── 100 active WebSocket connections          │                     │
│  │  └── Session state in RAM                      │                     │
│  └────────────────────────────────────────────────┘                     │
│                                                                          │
│  DURING DEPLOY                                                           │
│  ┌────────────────────────────────────────────────┐                     │
│  │  1. Start Machine v2 with new image            │                     │
│  │  2. Wait for v2 health check to pass           │                     │
│  │  3. Send SIGTERM to v1                         │                     │
│  │  4. v1 enters drain mode:                      │                     │
│  │     - Stops accepting new connections          │                     │
│  │     - Sends "reconnect" signal to clients      │                     │
│  │     - Waits for connections to close (30s max) │                     │
│  │  5. v1 terminates                              │                     │
│  └────────────────────────────────────────────────┘                     │
│                                                                          │
│  AFTER DEPLOY                                                            │
│  ┌────────────────────────────────────────────────┐                     │
│  │  Machine v2 (running)                          │                     │
│  │  ├── Clients reconnect automatically           │                     │
│  │  ├── New sessions created                      │                     │
│  │  └── Any unsaved state is lost (expected)      │                     │
│  └────────────────────────────────────────────────┘                     │
│                                                                          │
│  USER EXPERIENCE                                                         │
│  ┌────────────────────────────────────────────────┐                     │
│  │  • Brief "reconnecting..." overlay (~1-3s)     │                     │
│  │  • Page reloads with fresh state               │                     │
│  │  • Any unsaved form data is lost               │                     │
│  │  • This is documented/expected behavior        │                     │
│  └────────────────────────────────────────────────┘                     │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Billing Architecture

### Usage Tracking

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           USAGE TRACKING                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  METERED RESOURCES                                                       │
│  ─────────────────────────────────────────────────────────────────────  │
│                                                                          │
│  1. Machine Hours                                                        │
│     ┌─────────────────────────────────────────────────────────────────┐ │
│     │ Fly reports machine uptime via Machines API                     │ │
│     │ Rhone polls every 5 minutes and records:                        │ │
│     │   - app_id                                                      │ │
│     │   - machine_size (shared-1x, performance-2x, etc)               │ │
│     │   - duration_seconds                                            │ │
│     │   - region                                                      │ │
│     │                                                                 │ │
│     │ Cost: ~$0.0028/hr (shared-1x) to $0.18/hr (performance-4x)     │ │
│     └─────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  2. Bandwidth                                                            │
│     ┌─────────────────────────────────────────────────────────────────┐ │
│     │ Fly provides bandwidth metrics per app                          │ │
│     │ Rhone aggregates monthly:                                       │ │
│     │   - bytes_in (usually free)                                     │ │
│     │   - bytes_out (charged)                                         │ │
│     │                                                                 │ │
│     │ Cost: ~$0.02-0.12/GB depending on region                       │ │
│     └─────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  3. Builds                                                               │
│     ┌─────────────────────────────────────────────────────────────────┐ │
│     │ Rhone tracks:                                                   │ │
│     │   - build_count per billing period                              │ │
│     │   - build_duration_seconds (for cost allocation)                │ │
│     │                                                                 │ │
│     │ Cost: ~$0.007/build (builder machine time)                     │ │
│     └─────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  STRIPE INTEGRATION                                                      │
│  ─────────────────────────────────────────────────────────────────────  │
│                                                                          │
│  ┌───────────┐     ┌───────────┐     ┌───────────┐                     │
│  │  Usage    │────▶│  Stripe   │────▶│  Invoice  │                     │
│  │  Records  │     │  Metering │     │  Created  │                     │
│  └───────────┘     └───────────┘     └───────────┘                     │
│        │                                                                 │
│        │ Daily batch upload to Stripe:                                  │
│        │ POST /v1/billing/meter_events                                  │
│        │                                                                 │
│        └─────────────────────────────────────────────────────────────▶  │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Pricing Tiers

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           PRICING TIERS                                  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  FREE TIER ($0/month)                                                    │
│  ──────────────────────────────────────────────────────────────────     │
│  • 100 machine hours                                                     │
│  • 50 builds                                                             │
│  • 10 GB bandwidth                                                       │
│  • 1 team member                                                         │
│  • Community support                                                     │
│  • No custom domains                                                     │
│                                                                          │
│  STARTER ($20/month)                                                     │
│  ──────────────────────────────────────────────────────────────────     │
│  • 500 machine hours                                                     │
│  • 200 builds                                                            │
│  • 100 GB bandwidth                                                      │
│  • 5 team members                                                        │
│  • Email support                                                         │
│  • Custom domains                                                        │
│                                                                          │
│  PRO ($100/month)                                                        │
│  ──────────────────────────────────────────────────────────────────     │
│  • 2000 machine hours                                                    │
│  • Unlimited builds                                                      │
│  • 500 GB bandwidth                                                      │
│  • Unlimited team members                                                │
│  • Priority support                                                      │
│  • Multi-region                                                          │
│                                                                          │
│  ENTERPRISE (Custom)                                                     │
│  ──────────────────────────────────────────────────────────────────     │
│  • Custom limits                                                         │
│  • SLA guarantees                                                        │
│  • Dedicated support                                                     │
│  • Private networking                                                    │
│  • Volume discounts                                                      │
│                                                                          │
│  OVERAGE RATES                                                           │
│  ──────────────────────────────────────────────────────────────────     │
│  • Machine hours: $0.05/hr                                               │
│  • Bandwidth: $0.02/GB                                                   │
│  • Builds (Starter only): $0.02/build                                   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Scalability Considerations

### Current Design (Single Region)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    SINGLE REGION ARCHITECTURE                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Rhone Control Plane                                                     │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │  1-3 Fly Machines (auto-scale based on load)                        ││
│  │  • Each machine is stateless                                         ││
│  │  • Session stored in secure cookie                                   ││
│  │  • Database provides consistency                                     ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  Expected Capacity                                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │  • 10,000+ teams                                                     ││
│  │  • 50,000+ apps                                                      ││
│  │  • 1,000+ concurrent dashboard users                                 ││
│  │  • 100+ concurrent deployments                                       ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  Bottlenecks & Mitigations                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │  Database:                                                           ││
│  │    • Neon connection pooler (PgBouncer)                             ││
│  │    • Read replicas for dashboard queries                             ││
│  │                                                                      ││
│  │  Build System:                                                       ││
│  │    • Multiple builder machines (1 per concurrent build)              ││
│  │    • Shared layer cache on Fly volume                                ││
│  │                                                                      ││
│  │  Fly API:                                                            ││
│  │    • Rate limiting (respect their limits)                            ││
│  │    • Batch operations where possible                                 ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Future Scaling (Multi-Region)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    FUTURE: MULTI-REGION                                  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  When Needed                                                             │
│  ──────────────────────────────────────────────────────────────────     │
│  • Latency-sensitive operations (log streaming)                          │
│  • Regional data residency requirements                                  │
│  • Disaster recovery                                                     │
│                                                                          │
│  Architecture Changes                                                    │
│  ──────────────────────────────────────────────────────────────────     │
│  • Rhone runs in multiple Fly regions                                    │
│  • Neon primary + read replicas per region                               │
│  • Redis for cross-region session cache (optional)                       │
│  • Background job queue (River or similar)                               │
│                                                                          │
│  Not Needed For MVP                                                      │
│  ──────────────────────────────────────────────────────────────────     │
│  • Single region handles significant scale                               │
│  • Fly's global anycast provides edge termination                        │
│  • User apps can be multi-region independently                           │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Technology Decisions

### Why Chi (not Fiber, Gin, Echo)?

```
Chi chosen because:
• Standard net/http compatible (middleware works everywhere)
• Minimal magic (explicit is better)
• Battle-tested at scale
• Easy to migrate to/from standard library
• Middleware composability
```

### Why Templ (not html/template, Gomponents)?

```
Templ chosen because:
• Type-safe templates (compile-time errors)
• Go syntax (not a new DSL)
• Excellent HTMX integration
• Hot reload in development
• Similar feel to Vango's future component model
```

### Why Neon (not Supabase, PlanetScale, Fly Postgres)?

```
Neon chosen because:
• Serverless Postgres (scale to zero)
• Branching for development
• Connection pooling built-in
• Generous free tier
• True Postgres (not MySQL pretending)
```

### Why Railpack + BuildKit (not Depot, Nixpacks)?

```
Railpack + BuildKit chosen because:
• Free (only compute costs)
• Railway-proven (14M+ apps)
• Rootless = secure
• Best caching and smallest images
• Full control over build process
```

---

*Architecture Document v1.0 - Created 2024-12-11*
