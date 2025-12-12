# Rhone Build Roadmap

> **A cloud platform for deploying Vango applications**

---

## Executive Summary

Rhone is a Platform-as-a-Service (PaaS) built specifically for Vango applications. It handles the complexity of building, deploying, and managing stateful WebSocket applications on Fly.io infrastructure.

### Core Value Proposition

1. **One-Click Deploys**: Connect GitHub repo → Deploy → Live in minutes
2. **Vango-Aware**: Understands Vango's stateful nature (sticky sessions, no scale-to-zero while connected)
3. **Usage-Based Billing**: Pay for what you use, not fixed tiers
4. **Zero-Config Builds**: Railpack auto-detects and builds Go/Vango apps

---

## Technology Stack

| Layer | Technology | Purpose |
|-------|------------|---------|
| **Backend** | Go + Chi | API server, orchestration |
| **Frontend** | HTMX + Templ | Server-rendered UI |
| **Database** | Neon (Postgres) | User data, app state |
| **Infrastructure** | Fly.io | VMs, networking, registry |
| **Builds** | Railpack + BuildKit | Docker image creation |
| **Billing** | Stripe | Subscriptions, usage metering |
| **Auth** | GitHub OAuth + App | User login, repo access |

---

## Phase Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           RHONE BUILD PHASES                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Phase 1: Foundation          Phase 2: GitHub           Phase 3: Apps   │
│  ┌─────────────────┐          ┌─────────────────┐      ┌──────────────┐ │
│  │ Go + Chi Server │          │ GitHub App      │      │ App CRUD     │ │
│  │ Neon Database   │────────▶ │ Repo Access     │─────▶│ Env Vars     │ │
│  │ GitHub OAuth    │          │ Installation    │      │ Settings     │ │
│  │ HTMX + Templ    │          │ Token Exchange  │      │ Slug System  │ │
│  └─────────────────┘          └─────────────────┘      └──────────────┘ │
│           │                                                    │        │
│           ▼                                                    ▼        │
│  Phase 4: Build System         Phase 5: Deploy          Phase 6: Domain │
│  ┌─────────────────┐          ┌─────────────────┐      ┌──────────────┐ │
│  │ Railpack        │          │ Fly Machines    │      │ Subdomains   │ │
│  │ BuildKit        │────────▶ │ Zero-Downtime   │─────▶│ Custom DNS   │ │
│  │ Registry Push   │          │ Health Checks   │      │ SSL Certs    │ │
│  │ Log Streaming   │          │ Rollback        │      │ Verification │ │
│  └─────────────────┘          └─────────────────┘      └──────────────┘ │
│           │                                                    │        │
│           ▼                                                    ▼        │
│  Phase 7: Teams               Phase 8: Billing          Phase 9: Logs   │
│  ┌─────────────────┐          ┌─────────────────┐      ┌──────────────┐ │
│  │ Organizations   │          │ Stripe          │      │ Log Stream   │ │
│  │ Member Invite   │────────▶ │ Usage Metering  │─────▶│ Metrics      │ │
│  │ Role Perms      │          │ Invoices        │      │ Alerts       │ │
│  │ Team Switch     │          │ Overage         │      │ History      │ │
│  └─────────────────┘          └─────────────────┘      └──────────────┘ │
│           │                                                    │        │
│           ▼                                                    ▼        │
│  Phase 10: Webhooks           Phase 11: Regions        Phase 12: Prod   │
│  ┌─────────────────┐          ┌─────────────────┐      ┌──────────────┐ │
│  │ GitHub Webhooks │          │ Multi-Region    │      │ Rate Limits  │ │
│  │ Auto Deploy     │────────▶ │ Region Selector │─────▶│ Security     │ │
│  │ Commit Status   │          │ Latency Routing │      │ Monitoring   │ │
│  │ Branch Config   │          │ Primary Region  │      │ Documentation│ │
│  └─────────────────┘          └─────────────────┘      └──────────────┘ │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Phase Dependencies

```
Phase 1: Foundation
    │
    ├──▶ Phase 2: GitHub Integration
    │        │
    │        └──▶ Phase 3: App Management
    │                 │
    │                 ├──▶ Phase 4: Build System
    │                 │        │
    │                 │        └──▶ Phase 5: Deployment
    │                 │                 │
    │                 │                 └──▶ Phase 6: Domains
    │                 │
    │                 └──▶ Phase 7: Teams (parallel with 4-6)
    │
    └──▶ Phase 8: Billing (can start after Phase 1)
             │
             └──▶ Integrates with Phase 5 (usage tracking)

Phase 9: Logs ──▶ Requires Phase 5 (deployment)
Phase 10: Webhooks ──▶ Requires Phase 4 (builds)
Phase 11: Multi-Region ──▶ Requires Phase 5-6 (deploy + domains)
Phase 12: Production ──▶ Requires all previous phases
```

---

## Phase Status

| Phase | Name | Status | Description |
|-------|------|--------|-------------|
| 1 | Foundation | 🔴 Not Started | Go server, auth, database |
| 2 | GitHub Integration | 🔴 Not Started | GitHub App, repo access |
| 3 | App Management | 🔴 Not Started | CRUD, env vars, settings |
| 4 | Build System | 🔴 Not Started | Railpack, BuildKit |
| 5 | Deployment | 🔴 Not Started | Fly Machines, health checks |
| 6 | Domains | 🔴 Not Started | Subdomains, SSL, custom DNS |
| 7 | Teams | 🔴 Not Started | Organizations, permissions |
| 8 | Billing | 🔴 Not Started | Stripe, usage metering |
| 9 | Logs | 🔴 Not Started | Log streaming, metrics |
| 10 | Webhooks | 🔴 Not Started | Auto-deploy, commit status |
| 11 | Multi-Region | 🔴 Not Started | Region selection, routing |
| 12 | Production | 🔴 Not Started | Hardening, monitoring |

---

## Milestone Checkpoints

### Milestone 1: "Hello Rhone"
**Goal**: User can log in and see empty dashboard

- [ ] Phase 1 complete
- [ ] User authenticates with GitHub
- [ ] Dashboard renders with navigation
- [ ] Rhone deployed on Fly.io

### Milestone 2: "First Deploy"
**Goal**: User can deploy a Vango app from GitHub

- [ ] Phases 2-5 complete
- [ ] Connect GitHub repo
- [ ] Build completes successfully
- [ ] App live at {slug}.rhone.app

### Milestone 3: "Custom Domain"
**Goal**: User can use their own domain

- [ ] Phase 6 complete
- [ ] Custom domain configured
- [ ] SSL certificate provisioned
- [ ] DNS verified

### Milestone 4: "Team Collaboration"
**Goal**: Multiple users can work on same apps

- [ ] Phase 7 complete
- [ ] Create team
- [ ] Invite members
- [ ] Role-based access working

### Milestone 5: "Production Billing"
**Goal**: Real usage-based billing

- [ ] Phase 8 complete
- [ ] Payment method added
- [ ] Usage tracked accurately
- [ ] Invoices generated

### Milestone 6: "Auto-Deploy"
**Goal**: Push to main triggers automatic deploy

- [ ] Phases 9-10 complete
- [ ] GitHub webhook received
- [ ] Build triggered automatically
- [ ] Commit status updated

### Milestone 7: "Global Scale"
**Goal**: Deploy to multiple regions

- [ ] Phase 11 complete
- [ ] Multiple regions available
- [ ] Traffic routed correctly

### Milestone 8: "Production Ready"
**Goal**: Ready for public launch

- [ ] Phase 12 complete
- [ ] Security audit passed
- [ ] Rate limiting active
- [ ] Monitoring in place

---

## Critical Path

The minimum path to a working product:

```
Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5
```

This gets you: Login → Connect Repo → Create App → Build → Deploy

Everything else (billing, teams, custom domains) can be added incrementally.

---

## File Structure

```
rhone/
├── build_docs/
│   ├── BUILD_ROADMAP.md          # This file
│   ├── ARCHITECTURE.md           # System architecture
│   ├── PHASE_01_FOUNDATION.md    # Foundation
│   ├── PHASE_02_GITHUB.md        # GitHub integration
│   ├── PHASE_03_APPS.md          # App management
│   ├── PHASE_04_BUILD.md         # Build system
│   ├── PHASE_05_DEPLOY.md        # Deployment
│   ├── PHASE_06_DOMAINS.md       # Domains & SSL
│   ├── PHASE_07_TEAMS.md         # Teams & orgs
│   ├── PHASE_08_BILLING.md       # Stripe billing
│   ├── PHASE_09_LOGS.md          # Logs & monitoring
│   ├── PHASE_10_WEBHOOKS.md      # Auto-deploy
│   ├── PHASE_11_REGIONS.md       # Multi-region
│   └── PHASE_12_PRODUCTION.md    # Production hardening
├── cmd/
│   └── rhone/
│       └── main.go
├── internal/
│   ├── config/
│   ├── database/
│   ├── auth/
│   ├── billing/
│   ├── fly/
│   ├── build/
│   ├── deploy/
│   ├── domain/
│   ├── handlers/
│   ├── middleware/
│   └── templates/
├── static/
├── Dockerfile
├── fly.toml
└── go.mod
```

---

## External Services Setup

Before development begins, set up:

1. **Neon Database**
   - Create project at neon.tech
   - Get connection string
   - Note: Use connection pooler for production

2. **GitHub OAuth App** (for user login)
   - Settings → Developer Settings → OAuth Apps
   - Callback: `https://rhone.app/auth/callback`
   - Scopes: `read:user`, `user:email`

3. **GitHub App** (for repo access)
   - Settings → Developer Settings → GitHub Apps
   - Permissions: Contents (Read), Metadata (Read)
   - Webhook: Push events
   - Installation callback: `https://rhone.app/github/callback`

4. **Stripe Account**
   - Create products for each plan (Free, Starter, Pro)
   - Set up usage-based metering
   - Configure webhook endpoint

5. **Fly.io Organization**
   - Create `rhone` organization
   - Generate API token
   - Reserve `rhone.app` domain (or chosen domain)

6. **Domain & DNS**
   - Register/configure `rhone.app`
   - Point to Fly.io
   - Set up wildcard for `*.rhone.app`

---

## Development Workflow

1. **Read phase doc** before starting any phase
2. **Write tests** alongside implementation
3. **Update phase status** when complete
4. **Document decisions** in phase doc
5. **Verify milestone** before moving on

---

*Build Roadmap v1.0 - Created 2024-12-11*
