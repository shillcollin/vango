# Phase 8: Billing & Usage

> **Stripe integration for subscriptions and usage-based billing**

**Status**: Not Started

---

## Overview

Phase 8 implements billing via Stripe, including subscription plans, usage metering, and invoice generation.

### Goals

1. **Stripe integration**: Customer and subscription management
2. **Subscription plans**: Free, Starter, Pro tiers
3. **Usage metering**: Track machine hours, bandwidth, builds
4. **Invoice generation**: Monthly billing with usage details
5. **Payment methods**: Card management
6. **Billing dashboard**: Usage visualization

### Non-Goals

1. Prepaid credits
2. Annual plans (initially)
3. Custom enterprise pricing

---

## Pricing Model

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        PRICING TIERS                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  FREE ($0/month)                                                         │
│  ├── 100 machine hours                                                   │
│  ├── 50 builds                                                           │
│  ├── 10 GB bandwidth                                                     │
│  ├── 1 team member                                                       │
│  └── No custom domains                                                   │
│                                                                          │
│  STARTER ($20/month)                                                     │
│  ├── 500 machine hours                                                   │
│  ├── 200 builds                                                          │
│  ├── 100 GB bandwidth                                                    │
│  ├── 5 team members                                                      │
│  └── Custom domains                                                      │
│                                                                          │
│  PRO ($100/month)                                                        │
│  ├── 2000 machine hours                                                  │
│  ├── Unlimited builds                                                    │
│  ├── 500 GB bandwidth                                                    │
│  ├── Unlimited team members                                              │
│  └── Multi-region                                                        │
│                                                                          │
│  OVERAGE RATES                                                           │
│  ├── Machine hours: $0.05/hr                                             │
│  ├── Bandwidth: $0.02/GB                                                 │
│  └── Builds (Starter): $0.02/build                                       │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Database Schema

```sql
-- internal/database/migrations/006_billing.up.sql

-- Usage records for metering
CREATE TABLE usage_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    app_id UUID REFERENCES apps(id) ON DELETE SET NULL,
    metric VARCHAR(50) NOT NULL, -- machine_hours, bandwidth_gb, builds
    quantity DECIMAL(20, 6) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    billing_period DATE NOT NULL, -- First day of billing month
    stripe_usage_record_id VARCHAR(255)
);

CREATE INDEX idx_usage_records_team_period ON usage_records(team_id, billing_period);
CREATE INDEX idx_usage_records_metric ON usage_records(metric, billing_period);

-- Add Stripe fields to teams
ALTER TABLE teams ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(255);
ALTER TABLE teams ADD COLUMN IF NOT EXISTS stripe_subscription_id VARCHAR(255);
```

---

## Stripe Integration

```go
// internal/billing/stripe.go
package billing

import (
    "context"
    "fmt"
    "time"

    "github.com/stripe/stripe-go/v80"
    "github.com/stripe/stripe-go/v80/customer"
    "github.com/stripe/stripe-go/v80/subscription"
    "github.com/stripe/stripe-go/v80/usagerecord"
)

type BillingService struct {
    queries *queries.Queries
    logger  *slog.Logger
}

// Plan configuration
var Plans = map[string]Plan{
    "free": {
        Name:          "Free",
        PriceID:       "", // Free has no Stripe price
        MachineHours:  100,
        Builds:        50,
        BandwidthGB:   10,
        TeamMembers:   1,
        CustomDomains: false,
    },
    "starter": {
        Name:          "Starter",
        PriceID:       "price_starter_xxx", // Set from Stripe
        MachineHours:  500,
        Builds:        200,
        BandwidthGB:   100,
        TeamMembers:   5,
        CustomDomains: true,
    },
    "pro": {
        Name:          "Pro",
        PriceID:       "price_pro_xxx",
        MachineHours:  2000,
        Builds:        -1, // Unlimited
        BandwidthGB:   500,
        TeamMembers:   -1,
        CustomDomains: true,
    },
}

type Plan struct {
    Name          string
    PriceID       string
    MachineHours  int
    Builds        int
    BandwidthGB   int
    TeamMembers   int
    CustomDomains bool
}

// CreateCustomer creates a Stripe customer for a team
func (s *BillingService) CreateCustomer(ctx context.Context, team queries.Team, email string) (string, error) {
    params := &stripe.CustomerParams{
        Email: stripe.String(email),
        Name:  stripe.String(team.Name),
        Metadata: map[string]string{
            "team_id":   team.ID.String(),
            "team_slug": team.Slug,
        },
    }

    c, err := customer.New(params)
    if err != nil {
        return "", fmt.Errorf("create customer: %w", err)
    }

    // Store customer ID
    s.queries.UpdateTeamStripeCustomer(ctx, queries.UpdateTeamStripeCustomerParams{
        ID:               team.ID,
        StripeCustomerID: &c.ID,
    })

    return c.ID, nil
}

// CreateSubscription creates a subscription for a team
func (s *BillingService) CreateSubscription(ctx context.Context, team queries.Team, planID string) error {
    plan, ok := Plans[planID]
    if !ok {
        return fmt.Errorf("unknown plan: %s", planID)
    }

    if plan.PriceID == "" {
        // Free plan - just update the team
        return s.queries.UpdateTeamPlan(ctx, queries.UpdateTeamPlanParams{
            ID:   team.ID,
            Plan: planID,
        })
    }

    // Ensure customer exists
    if team.StripeCustomerID == nil {
        return fmt.Errorf("team has no Stripe customer")
    }

    params := &stripe.SubscriptionParams{
        Customer: team.StripeCustomerID,
        Items: []*stripe.SubscriptionItemsParams{
            {Price: stripe.String(plan.PriceID)},
        },
    }

    sub, err := subscription.New(params)
    if err != nil {
        return fmt.Errorf("create subscription: %w", err)
    }

    // Update team
    return s.queries.UpdateTeamSubscription(ctx, queries.UpdateTeamSubscriptionParams{
        ID:                   team.ID,
        Plan:                 planID,
        StripeSubscriptionID: &sub.ID,
    })
}

// RecordUsage records usage for billing
func (s *BillingService) RecordUsage(ctx context.Context, teamID, appID uuid.UUID, metric string, quantity float64) error {
    billingPeriod := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -time.Now().Day()+1)

    _, err := s.queries.CreateUsageRecord(ctx, queries.CreateUsageRecordParams{
        TeamID:        teamID,
        AppID:         &appID,
        Metric:        metric,
        Quantity:      quantity,
        BillingPeriod: billingPeriod,
    })
    return err
}

// GetUsageSummary returns usage for current billing period
func (s *BillingService) GetUsageSummary(ctx context.Context, teamID uuid.UUID) (*UsageSummary, error) {
    billingPeriod := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -time.Now().Day()+1)

    records, err := s.queries.GetTeamUsage(ctx, queries.GetTeamUsageParams{
        TeamID:        teamID,
        BillingPeriod: billingPeriod,
    })
    if err != nil {
        return nil, err
    }

    summary := &UsageSummary{
        BillingPeriod: billingPeriod,
    }

    for _, r := range records {
        switch r.Metric {
        case "machine_hours":
            summary.MachineHours += r.Total
        case "bandwidth_gb":
            summary.BandwidthGB += r.Total
        case "builds":
            summary.Builds += int(r.Total)
        }
    }

    return summary, nil
}

type UsageSummary struct {
    BillingPeriod time.Time
    MachineHours  float64
    BandwidthGB   float64
    Builds        int
}
```

---

## Usage Tracking

```go
// internal/billing/tracker.go
package billing

import (
    "context"
    "time"
)

type UsageTracker struct {
    billing *BillingService
    logger  *slog.Logger
}

// TrackMachineHours records machine uptime
// Called periodically by a background job
func (t *UsageTracker) TrackMachineHours(ctx context.Context) error {
    // Get all running apps
    apps, err := t.billing.queries.GetRunningApps(ctx)
    if err != nil {
        return err
    }

    for _, app := range apps {
        // Calculate hours since last check (5 minutes = 0.0833 hours)
        hours := 5.0 / 60.0

        if err := t.billing.RecordUsage(ctx, app.TeamID, app.ID, "machine_hours", hours); err != nil {
            t.logger.Error("failed to record machine hours",
                "app_id", app.ID,
                "error", err,
            )
        }
    }

    return nil
}

// TrackBuild records a build
func (t *UsageTracker) TrackBuild(ctx context.Context, teamID, appID uuid.UUID) error {
    return t.billing.RecordUsage(ctx, teamID, appID, "builds", 1)
}

// TrackBandwidth records bandwidth usage (called from Fly metrics)
func (t *UsageTracker) TrackBandwidth(ctx context.Context, teamID, appID uuid.UUID, gigabytes float64) error {
    return t.billing.RecordUsage(ctx, teamID, appID, "bandwidth_gb", gigabytes)
}
```

---

## Webhook Handler

```go
// internal/handlers/webhooks.go
package handlers

import (
    "encoding/json"
    "io"
    "net/http"

    "github.com/stripe/stripe-go/v80/webhook"
)

func (h *Handlers) StripeWebhook(w http.ResponseWriter, r *http.Request) {
    payload, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Error reading request body", http.StatusBadRequest)
        return
    }

    sig := r.Header.Get("Stripe-Signature")
    event, err := webhook.ConstructEvent(payload, sig, h.config.StripeWebhookSecret)
    if err != nil {
        h.logger.Error("webhook signature verification failed", "error", err)
        http.Error(w, "Invalid signature", http.StatusBadRequest)
        return
    }

    switch event.Type {
    case "customer.subscription.created":
        var sub stripe.Subscription
        json.Unmarshal(event.Data.Raw, &sub)
        h.handleSubscriptionCreated(r.Context(), &sub)

    case "customer.subscription.updated":
        var sub stripe.Subscription
        json.Unmarshal(event.Data.Raw, &sub)
        h.handleSubscriptionUpdated(r.Context(), &sub)

    case "customer.subscription.deleted":
        var sub stripe.Subscription
        json.Unmarshal(event.Data.Raw, &sub)
        h.handleSubscriptionDeleted(r.Context(), &sub)

    case "invoice.payment_succeeded":
        var invoice stripe.Invoice
        json.Unmarshal(event.Data.Raw, &invoice)
        h.handlePaymentSucceeded(r.Context(), &invoice)

    case "invoice.payment_failed":
        var invoice stripe.Invoice
        json.Unmarshal(event.Data.Raw, &invoice)
        h.handlePaymentFailed(r.Context(), &invoice)
    }

    w.WriteHeader(http.StatusOK)
}
```

---

## Exit Criteria

Phase 8 is complete when:

1. [ ] Stripe customers created for teams
2. [ ] Subscription plans work (Free, Starter, Pro)
3. [ ] Plan upgrades/downgrades work
4. [ ] Usage tracking records machine hours
5. [ ] Usage tracking records builds
6. [ ] Usage tracking records bandwidth
7. [ ] Usage dashboard shows current period
8. [ ] Billing page shows invoices
9. [ ] Payment methods can be added/removed
10. [ ] Stripe webhooks handled correctly
11. [ ] Overage billing works

---

## Dependencies

- **Requires**: Phase 5 (deployments to track), Phase 7 (teams for billing)
- **Required by**: None (but affects all features via limits)

---

*Phase 8 Specification - Version 1.0*
