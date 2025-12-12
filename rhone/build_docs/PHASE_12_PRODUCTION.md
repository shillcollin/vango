# Phase 12: Production Hardening

> **Security, performance, monitoring, and operational excellence**

**Status**: Not Started

---

## Overview

Phase 12 focuses on making Rhone production-ready. This includes security hardening, rate limiting, comprehensive monitoring, error handling, and operational tooling.

### Goals

1. **Security hardening**: Input validation, CSRF protection, security headers
2. **Rate limiting**: Protect against abuse
3. **Monitoring & alerting**: System health visibility
4. **Error handling**: User-friendly errors, error tracking
5. **Performance optimization**: Caching, query optimization
6. **Operational tooling**: Admin dashboard, maintenance mode
7. **Documentation**: User guides, API docs

### Non-Goals

1. SOC 2 compliance (future)
2. HIPAA compliance (future)
3. Single sign-on (SSO) for teams
4. Dedicated support tooling

---

## Security Checklist

```
┌─────────────────────────────────────────────────────────────────────────┐
│                       SECURITY HARDENING CHECKLIST                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  AUTHENTICATION & AUTHORIZATION                                         │
│  ├── [x] GitHub OAuth with state parameter (CSRF protection)           │
│  ├── [x] Secure session cookies (HTTPOnly, Secure, SameSite=Lax)       │
│  ├── [x] Session expiration and renewal                                 │
│  ├── [x] Team-based access control                                      │
│  ├── [x] Role-based permissions (owner/admin/member)                   │
│  ├── [ ] API token authentication for CLI (future)                      │
│  └── [ ] Account deletion capability                                    │
│                                                                          │
│  INPUT VALIDATION                                                       │
│  ├── [ ] All user input validated and sanitized                        │
│  ├── [ ] SQL injection prevention (parameterized queries via sqlc)     │
│  ├── [ ] XSS prevention (templ auto-escapes)                           │
│  ├── [ ] Path traversal prevention                                      │
│  ├── [ ] File upload validation (if applicable)                         │
│  └── [ ] JSON schema validation for API endpoints                       │
│                                                                          │
│  SECRETS MANAGEMENT                                                     │
│  ├── [x] Environment variables encrypted at rest (AES-256-GCM)         │
│  ├── [x] Secrets in Fly.io secrets (not in code/config)                │
│  ├── [x] GitHub webhook signature verification                          │
│  ├── [x] Stripe webhook signature verification                          │
│  └── [ ] Secret rotation capability                                     │
│                                                                          │
│  NETWORK SECURITY                                                       │
│  ├── [x] TLS 1.3 everywhere (Fly handles this)                         │
│  ├── [x] HTTPS redirects                                                │
│  ├── [ ] Security headers (CSP, HSTS, etc.)                            │
│  ├── [ ] Rate limiting per IP and per user                             │
│  └── [ ] DDoS protection (Fly provides basic)                          │
│                                                                          │
│  DATA PROTECTION                                                        │
│  ├── [x] Database encryption at rest (Neon provides)                   │
│  ├── [x] Database TLS required                                          │
│  ├── [ ] PII handling documentation                                     │
│  ├── [ ] Data retention policies                                        │
│  └── [ ] Audit logging for sensitive operations                        │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Security Headers Middleware

```go
// internal/middleware/security.go
package middleware

import (
    "net/http"
)

// SecurityHeaders adds security headers to responses
func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Prevent XSS attacks
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-XSS-Protection", "1; mode=block")

        // Prevent clickjacking
        w.Header().Set("X-Frame-Options", "DENY")

        // Enable HSTS (browser remembers to use HTTPS)
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

        // Referrer policy
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

        // Permissions policy (formerly Feature-Policy)
        w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

        // Content Security Policy
        csp := "default-src 'self'; " +
            "script-src 'self' 'unsafe-inline' https://unpkg.com; " + // HTMX from CDN
            "style-src 'self' 'unsafe-inline'; " +
            "img-src 'self' data: https:; " +
            "font-src 'self'; " +
            "connect-src 'self' wss:; " + // WebSocket for logs
            "frame-ancestors 'none';"
        w.Header().Set("Content-Security-Policy", csp)

        next.ServeHTTP(w, r)
    })
}

// CORS middleware for API endpoints
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")

            // Check if origin is allowed
            allowed := false
            for _, o := range allowedOrigins {
                if o == "*" || o == origin {
                    allowed = true
                    break
                }
            }

            if allowed {
                w.Header().Set("Access-Control-Allow-Origin", origin)
                w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
                w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
                w.Header().Set("Access-Control-Allow-Credentials", "true")
                w.Header().Set("Access-Control-Max-Age", "86400")
            }

            // Handle preflight
            if r.Method == "OPTIONS" {
                w.WriteHeader(http.StatusNoContent)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

---

## Rate Limiting

```go
// internal/middleware/ratelimit.go
package middleware

import (
    "net/http"
    "sync"
    "time"

    "golang.org/x/time/rate"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
    cleanup  time.Duration
}

// NewRateLimiter creates a new rate limiter
// rate: requests per second
// burst: maximum burst size
func NewRateLimiter(r float64, burst int) *RateLimiter {
    rl := &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rate:     rate.Limit(r),
        burst:    burst,
        cleanup:  time.Minute * 5,
    }

    // Cleanup old entries periodically
    go rl.cleanupLoop()

    return rl
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    limiter, exists := rl.limiters[key]
    if !exists {
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.limiters[key] = limiter
    }

    return limiter
}

func (rl *RateLimiter) cleanupLoop() {
    ticker := time.NewTicker(rl.cleanup)
    for range ticker.C {
        rl.mu.Lock()
        // Simple cleanup: just clear old entries
        // In production, track last access time
        if len(rl.limiters) > 10000 {
            rl.limiters = make(map[string]*rate.Limiter)
        }
        rl.mu.Unlock()
    }
}

// RateLimit middleware
func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Use IP as key (could also use user ID if authenticated)
            key := getClientIP(r)

            limiter := rl.getLimiter(key)
            if !limiter.Allow() {
                w.Header().Set("Retry-After", "1")
                http.Error(w, "Too many requests", http.StatusTooManyRequests)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

// Stricter rate limit for sensitive endpoints
func StrictRateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
    strictLimiter := NewRateLimiter(0.1, 5) // 0.1 req/sec, burst 5

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            key := getClientIP(r)

            limiter := strictLimiter.getLimiter(key)
            if !limiter.Allow() {
                w.Header().Set("Retry-After", "10")
                http.Error(w, "Too many requests", http.StatusTooManyRequests)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

func getClientIP(r *http.Request) string {
    // Check Fly-Client-IP header first (Fly.io sets this)
    if ip := r.Header.Get("Fly-Client-IP"); ip != "" {
        return ip
    }
    // Fallback to X-Forwarded-For
    if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
        return ip
    }
    // Last resort
    return r.RemoteAddr
}

// Rate limit configurations for different endpoint types
var (
    // General API: 100 req/sec per IP
    GeneralLimiter = NewRateLimiter(100, 200)

    // Auth endpoints: 5 req/sec per IP
    AuthLimiter = NewRateLimiter(5, 10)

    // Deploy endpoints: 10 req/min per IP
    DeployLimiter = NewRateLimiter(0.16, 5)

    // Webhook endpoints: 100 req/sec (from GitHub)
    WebhookLimiter = NewRateLimiter(100, 500)
)
```

---

## Error Handling

```go
// internal/errors/errors.go
package errors

import (
    "errors"
    "fmt"
    "net/http"
)

// AppError represents an application error
type AppError struct {
    Code       string `json:"code"`
    Message    string `json:"message"`
    Detail     string `json:"detail,omitempty"`
    StatusCode int    `json:"-"`
    Err        error  `json:"-"`
}

func (e *AppError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Err)
    }
    return e.Message
}

func (e *AppError) Unwrap() error {
    return e.Err
}

// Common error codes
const (
    ErrCodeNotFound       = "NOT_FOUND"
    ErrCodeUnauthorized   = "UNAUTHORIZED"
    ErrCodeForbidden      = "FORBIDDEN"
    ErrCodeBadRequest     = "BAD_REQUEST"
    ErrCodeConflict       = "CONFLICT"
    ErrCodeInternal       = "INTERNAL_ERROR"
    ErrCodeRateLimit      = "RATE_LIMIT"
    ErrCodeValidation     = "VALIDATION_ERROR"
    ErrCodeBuildFailed    = "BUILD_FAILED"
    ErrCodeDeployFailed   = "DEPLOY_FAILED"
    ErrCodePaymentFailed  = "PAYMENT_FAILED"
    ErrCodeQuotaExceeded  = "QUOTA_EXCEEDED"
)

// Error constructors
func NotFound(resource string) *AppError {
    return &AppError{
        Code:       ErrCodeNotFound,
        Message:    fmt.Sprintf("%s not found", resource),
        StatusCode: http.StatusNotFound,
    }
}

func Unauthorized(detail string) *AppError {
    return &AppError{
        Code:       ErrCodeUnauthorized,
        Message:    "Authentication required",
        Detail:     detail,
        StatusCode: http.StatusUnauthorized,
    }
}

func Forbidden(detail string) *AppError {
    return &AppError{
        Code:       ErrCodeForbidden,
        Message:    "Access denied",
        Detail:     detail,
        StatusCode: http.StatusForbidden,
    }
}

func BadRequest(message string) *AppError {
    return &AppError{
        Code:       ErrCodeBadRequest,
        Message:    message,
        StatusCode: http.StatusBadRequest,
    }
}

func Validation(field, message string) *AppError {
    return &AppError{
        Code:       ErrCodeValidation,
        Message:    "Validation error",
        Detail:     fmt.Sprintf("%s: %s", field, message),
        StatusCode: http.StatusBadRequest,
    }
}

func Internal(err error) *AppError {
    return &AppError{
        Code:       ErrCodeInternal,
        Message:    "An unexpected error occurred",
        StatusCode: http.StatusInternalServerError,
        Err:        err,
    }
}

func BuildFailed(detail string, err error) *AppError {
    return &AppError{
        Code:       ErrCodeBuildFailed,
        Message:    "Build failed",
        Detail:     detail,
        StatusCode: http.StatusInternalServerError,
        Err:        err,
    }
}

func DeployFailed(detail string, err error) *AppError {
    return &AppError{
        Code:       ErrCodeDeployFailed,
        Message:    "Deployment failed",
        Detail:     detail,
        StatusCode: http.StatusInternalServerError,
        Err:        err,
    }
}

func QuotaExceeded(resource string) *AppError {
    return &AppError{
        Code:       ErrCodeQuotaExceeded,
        Message:    fmt.Sprintf("%s quota exceeded", resource),
        Detail:     "Please upgrade your plan for additional capacity",
        StatusCode: http.StatusPaymentRequired,
    }
}

// AsAppError tries to convert an error to AppError
func AsAppError(err error) *AppError {
    var appErr *AppError
    if errors.As(err, &appErr) {
        return appErr
    }
    return Internal(err)
}
```

### Error Handler Middleware

```go
// internal/middleware/error_handler.go
package middleware

import (
    "encoding/json"
    "log/slog"
    "net/http"
    "runtime/debug"

    apperrors "github.com/vangoframework/rhone/internal/errors"
)

// ErrorHandler wraps handlers to catch panics and errors
func ErrorHandler(logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if rec := recover(); rec != nil {
                    logger.Error("panic recovered",
                        "panic", rec,
                        "stack", string(debug.Stack()),
                        "path", r.URL.Path,
                    )

                    err := apperrors.Internal(nil)
                    writeError(w, err)
                }
            }()

            // Wrap response writer to capture status
            wrapped := &responseCapture{ResponseWriter: w, status: http.StatusOK}
            next.ServeHTTP(wrapped, r)
        })
    }
}

type responseCapture struct {
    http.ResponseWriter
    status int
}

func (w *responseCapture) WriteHeader(status int) {
    w.status = status
    w.ResponseWriter.WriteHeader(status)
}

// WriteError writes an AppError as JSON response
func writeError(w http.ResponseWriter, err *apperrors.AppError) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(err.StatusCode)
    json.NewEncoder(w).Encode(map[string]any{
        "error": map[string]any{
            "code":    err.Code,
            "message": err.Message,
            "detail":  err.Detail,
        },
    })
}

// HandleError is a helper for handlers to return errors
func HandleError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
    appErr := apperrors.AsAppError(err)

    // Log internal errors
    if appErr.StatusCode >= 500 {
        logger.Error("internal error",
            "error", err,
            "code", appErr.Code,
            "path", r.URL.Path,
        )
    }

    writeError(w, appErr)
}
```

---

## Monitoring & Metrics

```go
// internal/monitoring/metrics.go
package monitoring

import (
    "net/http"
    "strconv"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    // HTTP request metrics
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "rhone_http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "rhone_http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )

    // Business metrics
    deploymentsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "rhone_deployments_total",
            Help: "Total number of deployments",
        },
        []string{"status", "trigger"},
    )

    buildsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "rhone_builds_total",
            Help: "Total number of builds",
        },
        []string{"status"},
    )

    buildDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "rhone_build_duration_seconds",
            Help:    "Build duration in seconds",
            Buckets: []float64{10, 30, 60, 120, 300, 600},
        },
        []string{"status"},
    )

    activeApps = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "rhone_active_apps",
            Help: "Number of active apps",
        },
    )

    activeTeams = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "rhone_active_teams",
            Help: "Number of active teams",
        },
    )
)

func init() {
    prometheus.MustRegister(
        httpRequestsTotal,
        httpRequestDuration,
        deploymentsTotal,
        buildsTotal,
        buildDuration,
        activeApps,
        activeTeams,
    )
}

// MetricsMiddleware records HTTP metrics
func MetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Wrap response writer
        wrapped := &statusRecorder{ResponseWriter: w, status: 200}
        next.ServeHTTP(wrapped, r)

        duration := time.Since(start).Seconds()
        path := normalizePath(r.URL.Path)

        httpRequestsTotal.WithLabelValues(
            r.Method,
            path,
            strconv.Itoa(wrapped.status),
        ).Inc()

        httpRequestDuration.WithLabelValues(
            r.Method,
            path,
        ).Observe(duration)
    })
}

type statusRecorder struct {
    http.ResponseWriter
    status int
}

func (r *statusRecorder) WriteHeader(status int) {
    r.status = status
    r.ResponseWriter.WriteHeader(status)
}

// normalizePath normalizes URL paths for metrics (avoid high cardinality)
func normalizePath(path string) string {
    // Replace UUIDs with placeholder
    // /apps/123e4567-e89b-12d3-a456-426614174000 -> /apps/{id}
    // This is a simplified version; in production use a more robust approach
    return path
}

// MetricsHandler returns the Prometheus metrics handler
func MetricsHandler() http.Handler {
    return promhttp.Handler()
}

// RecordDeployment records a deployment metric
func RecordDeployment(status, trigger string) {
    deploymentsTotal.WithLabelValues(status, trigger).Inc()
}

// RecordBuild records a build metric
func RecordBuild(status string, duration time.Duration) {
    buildsTotal.WithLabelValues(status).Inc()
    buildDuration.WithLabelValues(status).Observe(duration.Seconds())
}

// UpdateActiveApps updates the active apps gauge
func UpdateActiveApps(count int) {
    activeApps.Set(float64(count))
}

// UpdateActiveTeams updates the active teams gauge
func UpdateActiveTeams(count int) {
    activeTeams.Set(float64(count))
}
```

---

## Health Checks

```go
// internal/handlers/health.go
package handlers

import (
    "context"
    "encoding/json"
    "net/http"
    "time"
)

type HealthStatus struct {
    Status    string            `json:"status"`
    Timestamp time.Time         `json:"timestamp"`
    Services  map[string]string `json:"services"`
    Version   string            `json:"version"`
}

// Health returns basic health status
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "status": "ok",
    })
}

// HealthDetailed returns detailed health with dependency checks
func (h *Handlers) HealthDetailed(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    status := HealthStatus{
        Status:    "ok",
        Timestamp: time.Now(),
        Services:  make(map[string]string),
        Version:   h.config.Version,
    }

    // Check database
    if err := h.db.PingContext(ctx); err != nil {
        status.Services["database"] = "unhealthy"
        status.Status = "degraded"
    } else {
        status.Services["database"] = "healthy"
    }

    // Check Fly.io API (optional)
    if err := h.flyClient.Ping(ctx); err != nil {
        status.Services["fly"] = "unhealthy"
        // Don't mark as degraded - non-critical for serving requests
    } else {
        status.Services["fly"] = "healthy"
    }

    // Check Stripe API (optional)
    if err := h.stripeClient.Ping(ctx); err != nil {
        status.Services["stripe"] = "unhealthy"
    } else {
        status.Services["stripe"] = "healthy"
    }

    w.Header().Set("Content-Type", "application/json")

    if status.Status != "ok" {
        w.WriteHeader(http.StatusServiceUnavailable)
    }

    json.NewEncoder(w).Encode(status)
}

// Ready checks if the service is ready to receive traffic
func (h *Handlers) Ready(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()

    // Must have database connection
    if err := h.db.PingContext(ctx); err != nil {
        http.Error(w, "not ready", http.StatusServiceUnavailable)
        return
    }

    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ready"))
}
```

---

## Audit Logging

```go
// internal/audit/audit.go
package audit

import (
    "context"
    "encoding/json"
    "log/slog"
    "time"

    "github.com/google/uuid"
    "github.com/vangoframework/rhone/internal/database/queries"
)

type Logger struct {
    queries *queries.Queries
    logger  *slog.Logger
}

type Event struct {
    Type      string         `json:"type"`
    UserID    *uuid.UUID     `json:"user_id,omitempty"`
    TeamID    *uuid.UUID     `json:"team_id,omitempty"`
    AppID     *uuid.UUID     `json:"app_id,omitempty"`
    Action    string         `json:"action"`
    Resource  string         `json:"resource"`
    Details   map[string]any `json:"details,omitempty"`
    IP        string         `json:"ip,omitempty"`
    UserAgent string         `json:"user_agent,omitempty"`
}

// Common event types
const (
    EventTypeAuth       = "auth"
    EventTypeTeam       = "team"
    EventTypeApp        = "app"
    EventTypeDeployment = "deployment"
    EventTypeBilling    = "billing"
    EventTypeAdmin      = "admin"
)

// Common actions
const (
    ActionCreate = "create"
    ActionUpdate = "update"
    ActionDelete = "delete"
    ActionLogin  = "login"
    ActionLogout = "logout"
    ActionDeploy = "deploy"
    ActionInvite = "invite"
)

func NewLogger(queries *queries.Queries, logger *slog.Logger) *Logger {
    return &Logger{
        queries: queries,
        logger:  logger,
    }
}

// Log records an audit event
func (l *Logger) Log(ctx context.Context, event Event) {
    // Always log to structured logger
    l.logger.Info("audit event",
        "type", event.Type,
        "action", event.Action,
        "resource", event.Resource,
        "user_id", event.UserID,
        "team_id", event.TeamID,
        "app_id", event.AppID,
    )

    // Store in database for queryable audit trail
    detailsJSON, _ := json.Marshal(event.Details)

    _, err := l.queries.CreateAuditLog(ctx, queries.CreateAuditLogParams{
        EventType: event.Type,
        UserID:    event.UserID,
        TeamID:    event.TeamID,
        AppID:     event.AppID,
        Action:    event.Action,
        Resource:  event.Resource,
        Details:   detailsJSON,
        IP:        &event.IP,
        UserAgent: &event.UserAgent,
    })
    if err != nil {
        l.logger.Error("failed to write audit log", "error", err)
    }
}

// LogAuth logs authentication events
func (l *Logger) LogAuth(ctx context.Context, userID uuid.UUID, action, ip, userAgent string) {
    l.Log(ctx, Event{
        Type:      EventTypeAuth,
        UserID:    &userID,
        Action:    action,
        Resource:  "session",
        IP:        ip,
        UserAgent: userAgent,
    })
}

// LogTeamAction logs team-related events
func (l *Logger) LogTeamAction(ctx context.Context, userID, teamID uuid.UUID, action string, details map[string]any) {
    l.Log(ctx, Event{
        Type:     EventTypeTeam,
        UserID:   &userID,
        TeamID:   &teamID,
        Action:   action,
        Resource: "team",
        Details:  details,
    })
}

// LogAppAction logs app-related events
func (l *Logger) LogAppAction(ctx context.Context, userID, teamID, appID uuid.UUID, action string, details map[string]any) {
    l.Log(ctx, Event{
        Type:     EventTypeApp,
        UserID:   &userID,
        TeamID:   &teamID,
        AppID:    &appID,
        Action:   action,
        Resource: "app",
        Details:  details,
    })
}

// LogDeployment logs deployment events
func (l *Logger) LogDeployment(ctx context.Context, userID *uuid.UUID, teamID, appID uuid.UUID, action string, details map[string]any) {
    l.Log(ctx, Event{
        Type:     EventTypeDeployment,
        UserID:   userID,
        TeamID:   &teamID,
        AppID:    &appID,
        Action:   action,
        Resource: "deployment",
        Details:  details,
    })
}
```

### Audit Log Schema

```sql
-- internal/database/migrations/009_audit.up.sql

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(50) NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    app_id UUID REFERENCES apps(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,
    resource VARCHAR(100) NOT NULL,
    details JSONB,
    ip VARCHAR(50),
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_team_id ON audit_logs(team_id);
CREATE INDEX idx_audit_logs_app_id ON audit_logs(app_id);
CREATE INDEX idx_audit_logs_event_type ON audit_logs(event_type);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
```

---

## Maintenance Mode

```go
// internal/middleware/maintenance.go
package middleware

import (
    "net/http"
    "sync/atomic"
)

var maintenanceMode atomic.Bool

// SetMaintenanceMode enables or disables maintenance mode
func SetMaintenanceMode(enabled bool) {
    maintenanceMode.Store(enabled)
}

// IsMaintenanceMode returns true if maintenance mode is enabled
func IsMaintenanceMode() bool {
    return maintenanceMode.Load()
}

// Maintenance middleware blocks requests during maintenance
func Maintenance(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Always allow health checks
        if r.URL.Path == "/health" || r.URL.Path == "/ready" {
            next.ServeHTTP(w, r)
            return
        }

        // Always allow metrics
        if r.URL.Path == "/metrics" {
            next.ServeHTTP(w, r)
            return
        }

        if maintenanceMode.Load() {
            w.Header().Set("Retry-After", "300")
            w.WriteHeader(http.StatusServiceUnavailable)
            w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Rhone - Maintenance</title>
    <style>
        body {
            font-family: system-ui, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background: #f5f5f5;
        }
        .container {
            text-align: center;
            padding: 40px;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 { color: #333; }
        p { color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔧 Under Maintenance</h1>
        <p>We're performing scheduled maintenance. We'll be back shortly.</p>
        <p>Your apps are still running and serving traffic.</p>
    </div>
</body>
</html>`))
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

---

## Admin Dashboard

```go
// internal/handlers/admin.go
package handlers

import (
    "net/http"
)

// AdminDashboard shows system overview for admins
func (h *Handlers) AdminDashboard(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Get system stats
    stats, err := h.getSystemStats(ctx)
    if err != nil {
        h.HandleError(w, r, err)
        return
    }

    // Render admin dashboard
    h.render(w, r, templates.AdminDashboard(stats))
}

type SystemStats struct {
    TotalUsers       int64
    TotalTeams       int64
    TotalApps        int64
    TotalDeployments int64
    ActiveBuilds     int
    RecentErrors     []ErrorSummary
    TopApps          []AppUsage
}

type ErrorSummary struct {
    Code  string
    Count int
    Last  time.Time
}

type AppUsage struct {
    AppID        uuid.UUID
    AppName      string
    TeamName     string
    MachineHours float64
    Deployments  int
}

func (h *Handlers) getSystemStats(ctx context.Context) (*SystemStats, error) {
    stats := &SystemStats{}

    // Get counts (in parallel for performance)
    var wg sync.WaitGroup
    wg.Add(4)

    go func() {
        defer wg.Done()
        stats.TotalUsers, _ = h.queries.CountUsers(ctx)
    }()

    go func() {
        defer wg.Done()
        stats.TotalTeams, _ = h.queries.CountTeams(ctx)
    }()

    go func() {
        defer wg.Done()
        stats.TotalApps, _ = h.queries.CountApps(ctx)
    }()

    go func() {
        defer wg.Done()
        stats.TotalDeployments, _ = h.queries.CountDeployments(ctx)
    }()

    wg.Wait()

    // Get top apps by usage
    topApps, _ := h.queries.GetTopAppsByUsage(ctx, 10)
    stats.TopApps = make([]AppUsage, len(topApps))
    for i, a := range topApps {
        stats.TopApps[i] = AppUsage{
            AppID:        a.ID,
            AppName:      a.Name,
            TeamName:     a.TeamName,
            MachineHours: a.MachineHours,
            Deployments:  int(a.DeploymentCount),
        }
    }

    return stats, nil
}

// RequireAdmin middleware checks for admin privileges
func (h *Handlers) RequireAdmin(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        session := GetSession(r.Context())
        if session == nil {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }

        // Check if user is an admin (could use a database flag or specific user IDs)
        user, err := h.queries.GetUser(r.Context(), session.UserID)
        if err != nil || !user.IsAdmin {
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

---

## Database Migrations for Admin

```sql
-- internal/database/migrations/010_admin.up.sql

-- Add admin flag to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN DEFAULT false;

-- Add indices for common admin queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_apps_created_at ON apps(created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_deployments_created_at ON deployments(created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_deployments_status ON deployments(status);
```

---

## Production Routes

```go
// Router setup with all production middleware

func SetupRouter(h *Handlers, config *Config) http.Handler {
    r := chi.NewRouter()

    // Global middleware (order matters!)
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger(h.logger))
    r.Use(middleware.Recoverer)
    r.Use(middleware.Maintenance)
    r.Use(middleware.SecurityHeaders)
    r.Use(middleware.MetricsMiddleware)
    r.Use(middleware.RateLimit(middleware.GeneralLimiter))

    // Health endpoints (no auth required)
    r.Get("/health", h.Health)
    r.Get("/health/detailed", h.HealthDetailed)
    r.Get("/ready", h.Ready)
    r.Handle("/metrics", monitoring.MetricsHandler())

    // Auth routes with stricter rate limiting
    r.Group(func(r chi.Router) {
        r.Use(middleware.RateLimit(middleware.AuthLimiter))
        r.Get("/login", h.Login)
        r.Get("/auth/callback", h.AuthCallback)
        r.Post("/logout", h.Logout)
    })

    // Webhook routes (signature verified, different rate limit)
    r.Group(func(r chi.Router) {
        r.Use(middleware.RateLimit(middleware.WebhookLimiter))
        r.Post("/webhooks/github", h.GitHubWebhook)
        r.Post("/webhooks/stripe", h.StripeWebhook)
    })

    // Protected routes
    r.Group(func(r chi.Router) {
        r.Use(h.RequireAuth)

        // Dashboard
        r.Get("/", h.Dashboard)
        r.Get("/dashboard", h.Dashboard)

        // Apps
        r.Route("/apps", func(r chi.Router) {
            r.Get("/", h.ListApps)
            r.Post("/", h.CreateApp)
            r.Get("/new", h.NewAppForm)

            r.Route("/{appID}", func(r chi.Router) {
                r.Get("/", h.GetApp)
                r.Put("/", h.UpdateApp)
                r.Delete("/", h.DeleteApp)

                // Deploy with stricter rate limit
                r.Group(func(r chi.Router) {
                    r.Use(middleware.RateLimit(middleware.DeployLimiter))
                    r.Post("/deploy", h.Deploy)
                })

                // Other app routes...
            })
        })

        // Teams, Settings, Billing...
    })

    // Admin routes
    r.Route("/admin", func(r chi.Router) {
        r.Use(h.RequireAuth)
        r.Use(h.RequireAdmin)
        r.Get("/", h.AdminDashboard)
        r.Post("/maintenance", h.ToggleMaintenance)
    })

    return r
}
```

---

## Exit Criteria

Phase 12 is complete when:

1. [ ] Security headers applied to all responses
2. [ ] Rate limiting active on all endpoints
3. [ ] Prometheus metrics exposed at /metrics
4. [ ] Health check endpoints working
5. [ ] Detailed health shows all dependencies
6. [ ] Audit logging captures sensitive operations
7. [ ] Error handling returns user-friendly messages
8. [ ] Admin dashboard shows system stats
9. [ ] Maintenance mode can be toggled
10. [ ] All security checklist items addressed
11. [ ] Load testing completed successfully
12. [ ] Security review completed

---

## Production Deployment Checklist

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    PRODUCTION DEPLOYMENT CHECKLIST                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  PRE-DEPLOYMENT                                                         │
│  ├── [ ] All tests passing                                              │
│  ├── [ ] Database migrations tested                                     │
│  ├── [ ] Environment variables configured                               │
│  ├── [ ] Secrets rotated since staging                                 │
│  ├── [ ] Stripe webhooks configured for production                      │
│  ├── [ ] GitHub App configured for production                           │
│  ├── [ ] DNS configured for rhone.app and *.rhone.app                  │
│  └── [ ] SSL certificates provisioned                                   │
│                                                                          │
│  MONITORING                                                             │
│  ├── [ ] Prometheus/Grafana dashboards set up                          │
│  ├── [ ] Alerts configured for critical metrics                        │
│  ├── [ ] Error tracking (Sentry) configured                            │
│  ├── [ ] Log aggregation configured                                     │
│  └── [ ] Uptime monitoring enabled                                      │
│                                                                          │
│  SCALING                                                                │
│  ├── [ ] Auto-scaling configured                                        │
│  ├── [ ] Database connection pooling enabled                           │
│  ├── [ ] CDN for static assets (optional)                              │
│  └── [ ] Load tested at 2x expected traffic                            │
│                                                                          │
│  DOCUMENTATION                                                          │
│  ├── [ ] User documentation complete                                    │
│  ├── [ ] API documentation complete                                     │
│  ├── [ ] Runbook for common operations                                  │
│  ├── [ ] Incident response plan documented                              │
│  └── [ ] On-call rotation established                                   │
│                                                                          │
│  POST-DEPLOYMENT                                                        │
│  ├── [ ] Smoke tests passing                                           │
│  ├── [ ] Monitoring dashboards showing healthy metrics                  │
│  ├── [ ] First deployment successful                                    │
│  └── [ ] Rollback procedure verified                                    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Dependencies

- **Requires**: All previous phases (1-11)
- **Required by**: None (final phase)

---

*Phase 12 Specification - Version 1.0*
