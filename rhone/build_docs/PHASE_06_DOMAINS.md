# Phase 6: Domains & Networking

> **Subdomains, custom domains, and SSL certificates**

**Status**: Not Started

---

## Overview

Phase 6 implements the domain management system. Each app automatically gets a `{slug}.rhone.app` subdomain, and users can add custom domains with automatic SSL certificate provisioning via Fly.io.

### Goals

1. **Automatic subdomains**: `{slug}.rhone.app` for every app
2. **Custom domains**: User-provided domains with verification
3. **SSL certificates**: Automatic provisioning via Fly's ACME
4. **DNS verification**: Guide users through DNS configuration
5. **Domain status tracking**: Monitor certificate health

### Non-Goals

1. DNS hosting (users manage their own DNS)
2. Wildcard certificates
3. Certificate pinning

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                       DOMAIN ROUTING                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  AUTOMATIC SUBDOMAINS                                                    │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ DNS: *.rhone.app → Fly.io Anycast IPs                               ││
│  │                                                                     ││
│  │ Request: my-app.rhone.app                                           ││
│  │     │                                                               ││
│  │     ▼                                                               ││
│  │ Fly.io Edge                                                         ││
│  │     │                                                               ││
│  │     ▼                                                               ││
│  │ Rhone Proxy (future) or direct to Fly App                          ││
│  │     │                                                               ││
│  │     ▼                                                               ││
│  │ User's Fly Machine                                                  ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
│  CUSTOM DOMAINS                                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │ 1. User adds custom.example.com in Rhone dashboard                  ││
│  │                                                                     ││
│  │ 2. Rhone calls Fly GraphQL API to add certificate:                  ││
│  │    mutation { addCertificate(appId: "...", hostname: "...") }       ││
│  │                                                                     ││
│  │ 3. User configures DNS:                                             ││
│  │    CNAME custom.example.com → {fly-app}.fly.dev                    ││
│  │    OR                                                               ││
│  │    A custom.example.com → Fly Anycast IP                           ││
│  │                                                                     ││
│  │ 4. Fly automatically provisions Let's Encrypt certificate          ││
│  │                                                                     ││
│  │ 5. Rhone polls certificate status until verified                    ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Database Schema

```sql
-- internal/database/migrations/005_domains.up.sql

CREATE TABLE domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    hostname VARCHAR(255) NOT NULL,
    is_primary BOOLEAN DEFAULT false,

    -- Verification status
    dns_configured BOOLEAN DEFAULT false,
    ssl_status VARCHAR(50) DEFAULT 'pending', -- pending, provisioning, active, failed
    ssl_issued_at TIMESTAMPTZ,
    ssl_expires_at TIMESTAMPTZ,

    -- Verification details
    verification_method VARCHAR(50), -- cname, a_record
    verification_target VARCHAR(255), -- What they should point to
    last_check_at TIMESTAMPTZ,
    last_error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(hostname)
);

CREATE INDEX idx_domains_app_id ON domains(app_id);
CREATE INDEX idx_domains_hostname ON domains(hostname);
CREATE INDEX idx_domains_ssl_status ON domains(ssl_status);

CREATE TRIGGER update_domains_updated_at
    BEFORE UPDATE ON domains
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

---

## Fly Certificate API

```go
// internal/fly/certificates.go
package fly

import (
    "context"
    "fmt"
)

type Certificate struct {
    ID                 string `json:"id"`
    Hostname           string `json:"hostname"`
    Configured         bool   `json:"configured"`
    AcmeDnsConfigured  bool   `json:"acmeDnsConfigured"`
    AcmeAlpnConfigured bool   `json:"acmeAlpnConfigured"`
    CertificateAuthority string `json:"certificateAuthority"`
    CreatedAt          string `json:"createdAt"`
    DnsValidationHostname string `json:"dnsValidationHostname"`
    DnsValidationTarget   string `json:"dnsValidationTarget"`
    Source             string `json:"source"`
    ClientStatus       string `json:"clientStatus"`
    IsApex             bool   `json:"isApex"`
    IsWildcard         bool   `json:"isWildcard"`
}

// GraphQL mutation for adding certificates
const addCertificateMutation = `
mutation AddCertificate($appId: ID!, $hostname: String!) {
    addCertificate(appId: $appId, hostname: $hostname) {
        certificate {
            id
            hostname
            configured
            acmeDnsConfigured
            acmeAlpnConfigured
            dnsValidationHostname
            dnsValidationTarget
            clientStatus
            isApex
        }
    }
}
`

const getCertificateQuery = `
query GetCertificate($appId: ID!, $hostname: String!) {
    app(id: $appId) {
        certificate(hostname: $hostname) {
            id
            hostname
            configured
            acmeDnsConfigured
            acmeAlpnConfigured
            clientStatus
            createdAt
            source
        }
    }
}
`

const deleteCertificateMutation = `
mutation DeleteCertificate($appId: ID!, $hostname: String!) {
    deleteCertificate(appId: $appId, hostname: $hostname) {
        app {
            id
        }
    }
}
`

// AddCertificate adds a custom domain certificate
func (c *Client) AddCertificate(ctx context.Context, appID, hostname string) (*Certificate, error) {
    variables := map[string]any{
        "appId":    appID,
        "hostname": hostname,
    }

    var result struct {
        AddCertificate struct {
            Certificate Certificate `json:"certificate"`
        } `json:"addCertificate"`
    }

    if err := c.graphQL(ctx, addCertificateMutation, variables, &result); err != nil {
        return nil, fmt.Errorf("add certificate: %w", err)
    }

    return &result.AddCertificate.Certificate, nil
}

// GetCertificate retrieves certificate status
func (c *Client) GetCertificate(ctx context.Context, appID, hostname string) (*Certificate, error) {
    variables := map[string]any{
        "appId":    appID,
        "hostname": hostname,
    }

    var result struct {
        App struct {
            Certificate *Certificate `json:"certificate"`
        } `json:"app"`
    }

    if err := c.graphQL(ctx, getCertificateQuery, variables, &result); err != nil {
        return nil, fmt.Errorf("get certificate: %w", err)
    }

    return result.App.Certificate, nil
}

// DeleteCertificate removes a custom domain certificate
func (c *Client) DeleteCertificate(ctx context.Context, appID, hostname string) error {
    variables := map[string]any{
        "appId":    appID,
        "hostname": hostname,
    }

    var result struct{}
    return c.graphQL(ctx, deleteCertificateMutation, variables, &result)
}

// graphQL executes a GraphQL query against Fly's API
func (c *Client) graphQL(ctx context.Context, query string, variables map[string]any, result any) error {
    body := map[string]any{
        "query":     query,
        "variables": variables,
    }

    var response struct {
        Data   json.RawMessage `json:"data"`
        Errors []struct {
            Message string `json:"message"`
        } `json:"errors"`
    }

    if err := c.doJSON(ctx, "POST", FlyAPIURL+"/graphql", body, &response); err != nil {
        return err
    }

    if len(response.Errors) > 0 {
        return fmt.Errorf("graphql error: %s", response.Errors[0].Message)
    }

    return json.Unmarshal(response.Data, result)
}
```

---

## Domain Service

```go
// internal/domain/domains.go
package domain

import (
    "context"
    "fmt"
    "net"
    "strings"
    "time"

    "github.com/google/uuid"
    "github.com/vangoframework/rhone/internal/database/queries"
    "github.com/vangoframework/rhone/internal/fly"
)

type DomainService struct {
    flyClient *fly.Client
    queries   *queries.Queries
    logger    *slog.Logger
}

func NewDomainService(flyClient *fly.Client, queries *queries.Queries, logger *slog.Logger) *DomainService {
    return &DomainService{
        flyClient: flyClient,
        queries:   queries,
        logger:    logger,
    }
}

type AddDomainResult struct {
    Domain            queries.Domain
    VerificationTarget string
    IsApex            bool
}

// AddDomain adds a custom domain to an app
func (s *DomainService) AddDomain(ctx context.Context, app queries.App, hostname string) (*AddDomainResult, error) {
    // Validate hostname
    hostname = strings.ToLower(strings.TrimSpace(hostname))
    if err := validateHostname(hostname); err != nil {
        return nil, err
    }

    // Check if domain already exists
    existing, _ := s.queries.GetDomainByHostname(ctx, hostname)
    if existing != nil {
        return nil, fmt.Errorf("domain already in use")
    }

    // Add certificate to Fly
    cert, err := s.flyClient.AddCertificate(ctx, *app.FlyAppID, hostname)
    if err != nil {
        return nil, fmt.Errorf("fly certificate error: %w", err)
    }

    // Determine verification target
    verificationTarget := fmt.Sprintf("%s.fly.dev", *app.FlyAppID)
    verificationMethod := "cname"
    if cert.IsApex {
        // Apex domains need A record
        verificationMethod = "a_record"
        verificationTarget = "Fly.io Anycast IP (see docs)"
    }

    // Store domain
    domain, err := s.queries.CreateDomain(ctx, queries.CreateDomainParams{
        AppID:              app.ID,
        Hostname:           hostname,
        SSLStatus:          "pending",
        VerificationMethod: verificationMethod,
        VerificationTarget: verificationTarget,
    })
    if err != nil {
        return nil, err
    }

    s.logger.Info("domain added",
        "app_id", app.ID,
        "hostname", hostname,
    )

    return &AddDomainResult{
        Domain:            domain,
        VerificationTarget: verificationTarget,
        IsApex:            cert.IsApex,
    }, nil
}

// RemoveDomain removes a custom domain
func (s *DomainService) RemoveDomain(ctx context.Context, app queries.App, domainID uuid.UUID) error {
    domain, err := s.queries.GetDomain(ctx, domainID)
    if err != nil {
        return err
    }

    if domain.AppID != app.ID {
        return fmt.Errorf("domain not found")
    }

    // Remove from Fly
    if app.FlyAppID != nil {
        if err := s.flyClient.DeleteCertificate(ctx, *app.FlyAppID, domain.Hostname); err != nil {
            s.logger.Warn("failed to delete fly certificate", "error", err)
        }
    }

    // Delete from database
    return s.queries.DeleteDomain(ctx, domainID)
}

// CheckDomainStatus verifies DNS and SSL status
func (s *DomainService) CheckDomainStatus(ctx context.Context, app queries.App, domain queries.Domain) error {
    if app.FlyAppID == nil {
        return fmt.Errorf("app not deployed")
    }

    // Check certificate status from Fly
    cert, err := s.flyClient.GetCertificate(ctx, *app.FlyAppID, domain.Hostname)
    if err != nil {
        return err
    }

    // Update status
    sslStatus := "pending"
    dnsConfigured := false

    if cert != nil {
        dnsConfigured = cert.Configured || cert.AcmeDnsConfigured || cert.AcmeAlpnConfigured

        switch cert.ClientStatus {
        case "Ready":
            sslStatus = "active"
        case "Awaiting configuration":
            sslStatus = "pending"
        case "Provisioning":
            sslStatus = "provisioning"
        default:
            sslStatus = "pending"
        }
    }

    // Update database
    return s.queries.UpdateDomainStatus(ctx, queries.UpdateDomainStatusParams{
        ID:            domain.ID,
        DNSConfigured: dnsConfigured,
        SSLStatus:     sslStatus,
        LastCheckAt:   time.Now(),
    })
}

// VerifyDNS checks if DNS is properly configured
func (s *DomainService) VerifyDNS(ctx context.Context, hostname, expectedTarget string) (bool, error) {
    // Try CNAME lookup
    cname, err := net.LookupCNAME(hostname)
    if err == nil && strings.Contains(cname, expectedTarget) {
        return true, nil
    }

    // Try A record lookup and compare IPs
    ips, err := net.LookupIP(hostname)
    if err != nil {
        return false, nil
    }

    targetIPs, _ := net.LookupIP(expectedTarget)
    for _, ip := range ips {
        for _, targetIP := range targetIPs {
            if ip.Equal(targetIP) {
                return true, nil
            }
        }
    }

    return false, nil
}

func validateHostname(hostname string) error {
    if len(hostname) == 0 {
        return fmt.Errorf("hostname cannot be empty")
    }
    if len(hostname) > 253 {
        return fmt.Errorf("hostname too long")
    }
    if strings.HasSuffix(hostname, ".rhone.app") {
        return fmt.Errorf("cannot use rhone.app subdomains as custom domains")
    }
    // Add more validation as needed
    return nil
}
```

---

## Exit Criteria

Phase 6 is complete when:

1. [ ] Apps accessible at `{slug}.rhone.app` via wildcard DNS
2. [ ] Custom domains can be added via UI
3. [ ] Fly certificates provisioned automatically
4. [ ] DNS verification shows correct instructions
5. [ ] SSL status tracked and displayed
6. [ ] Custom domains can be removed
7. [ ] Primary domain can be set
8. [ ] Certificate renewal handled by Fly automatically

---

## Dependencies

- **Requires**: Phase 5 (apps must be deployed first)
- **Required by**: None (standalone feature)

---

*Phase 6 Specification - Version 1.0*
