# Phase 9: Logs & Monitoring

> **Real-time log streaming and application metrics**

**Status**: Not Started

---

## Overview

Phase 9 implements log streaming and monitoring for deployed applications. Users can view real-time logs, search historical logs, and monitor basic metrics from the Rhone dashboard.

### Goals

1. **Real-time streaming**: Live log output via SSE
2. **Log history**: Searchable historical logs
3. **Basic metrics**: CPU, memory, request count
4. **Multiple sources**: Application logs, system logs, build logs
5. **Log filtering**: By level, time range, search term

### Non-Goals

1. Log aggregation service (use Fly's built-in)
2. Advanced APM (Application Performance Monitoring)
3. Custom alerting rules
4. Log export to external services (initially)

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                       LOG STREAMING ARCHITECTURE                        │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  LOG SOURCES                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │                                                                     ││
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              ││
│  │  │  App Logs    │  │  System Logs │  │  Build Logs  │              ││
│  │  │  (stdout/    │  │  (Fly VM     │  │  (BuildKit   │              ││
│  │  │   stderr)    │  │   events)    │  │   output)    │              ││
│  │  └──────────────┘  └──────────────┘  └──────────────┘              ││
│  │         │                 │                 │                       ││
│  │         └─────────────────┴─────────────────┘                       ││
│  │                           │                                         ││
│  │                           ▼                                         ││
│  │                   Fly.io NATS (internal)                            ││
│  │                           │                                         ││
│  └───────────────────────────┼─────────────────────────────────────────┘│
│                              │                                          │
│  RHONE LOG SERVICE                                                      │
│  ┌───────────────────────────┼─────────────────────────────────────────┐│
│  │                           ▼                                         ││
│  │  ┌──────────────────────────────────────────────────────────────┐  ││
│  │  │                    Log Aggregator                             │  ││
│  │  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │  ││
│  │  │  │  Fly Logs    │  │  Parser      │  │  Broadcaster │        │  ││
│  │  │  │  API Client  │──│  (JSON/text) │──│  (SSE hub)   │        │  ││
│  │  │  └──────────────┘  └──────────────┘  └──────────────┘        │  ││
│  │  └──────────────────────────────────────────────────────────────┘  ││
│  │                              │                                      ││
│  │         ┌────────────────────┼────────────────────┐                ││
│  │         ▼                    ▼                    ▼                ││
│  │  ┌──────────────┐  ┌──────────────────┐  ┌──────────────┐         ││
│  │  │  SSE Stream  │  │  Log Search API  │  │  Metrics     │         ││
│  │  │  /logs/live  │  │  /logs/search    │  │  /metrics    │         ││
│  │  └──────────────┘  └──────────────────┘  └──────────────┘         ││
│  │         │                    │                    │                 ││
│  └─────────┼────────────────────┼────────────────────┼─────────────────┘│
│            │                    │                    │                  │
│            ▼                    ▼                    ▼                  │
│  ┌─────────────────────────────────────────────────────────────────────┐│
│  │                         DASHBOARD UI                                ││
│  │  ┌──────────────────────────────────────────────────────────────┐  ││
│  │  │  Real-time log viewer with auto-scroll                        │  ││
│  │  │  Search bar with filters (level, time, term)                  │  ││
│  │  │  Metrics charts (CPU, memory, requests)                       │  ││
│  │  └──────────────────────────────────────────────────────────────┘  ││
│  └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Fly.io Log Access

Fly.io provides log access via the NATS protocol internally and a REST API externally. We'll use the REST API for simplicity.

### Fly Logs API

```go
// internal/fly/logs.go
package fly

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// LogEntry represents a single log line from Fly
type LogEntry struct {
    Timestamp time.Time `json:"timestamp"`
    Message   string    `json:"message"`
    Level     string    `json:"level"`
    Region    string    `json:"region"`
    Instance  string    `json:"instance"`
    Meta      LogMeta   `json:"meta"`
}

type LogMeta struct {
    Event struct {
        Provider string `json:"provider"`
    } `json:"event"`
    Error struct {
        Code    int    `json:"code"`
        Message string `json:"message"`
    } `json:"error"`
    HTTP struct {
        Request struct {
            Method string `json:"method"`
            URL    string `json:"url"`
        } `json:"request"`
        Response struct {
            StatusCode int `json:"status_code"`
        } `json:"response"`
    } `json:"http"`
}

// LogStream streams logs from a Fly app
func (c *Client) LogStream(ctx context.Context, appName string, opts LogStreamOptions) (<-chan LogEntry, error) {
    url := fmt.Sprintf("https://api.fly.io/v1/apps/%s/logs?region=%s&instance=%s",
        appName,
        opts.Region,
        opts.Instance,
    )

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+c.token)
    req.Header.Set("Accept", "text/event-stream")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }

    if resp.StatusCode != http.StatusOK {
        resp.Body.Close()
        return nil, fmt.Errorf("log stream failed: %s", resp.Status)
    }

    logs := make(chan LogEntry, 100)

    go func() {
        defer resp.Body.Close()
        defer close(logs)

        reader := bufio.NewReader(resp.Body)
        for {
            select {
            case <-ctx.Done():
                return
            default:
            }

            line, err := reader.ReadBytes('\n')
            if err != nil {
                if err != io.EOF {
                    c.logger.Error("log stream read error", "error", err)
                }
                return
            }

            // Skip empty lines and SSE comments
            if len(line) <= 1 || line[0] == ':' {
                continue
            }

            // Parse SSE data
            if len(line) > 6 && string(line[:6]) == "data: " {
                var entry LogEntry
                if err := json.Unmarshal(line[6:], &entry); err != nil {
                    continue
                }
                select {
                case logs <- entry:
                default:
                    // Drop log if channel full
                }
            }
        }
    }()

    return logs, nil
}

type LogStreamOptions struct {
    Region   string
    Instance string
}

// GetLogs retrieves historical logs
func (c *Client) GetLogs(ctx context.Context, appName string, opts GetLogsOptions) ([]LogEntry, error) {
    url := fmt.Sprintf("https://api.fly.io/v1/apps/%s/logs?limit=%d",
        appName,
        opts.Limit,
    )

    if !opts.Since.IsZero() {
        url += fmt.Sprintf("&since=%s", opts.Since.Format(time.RFC3339))
    }
    if !opts.Until.IsZero() {
        url += fmt.Sprintf("&until=%s", opts.Until.Format(time.RFC3339))
    }

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+c.token)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("get logs failed: %s", resp.Status)
    }

    var result struct {
        Data []LogEntry `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return result.Data, nil
}

type GetLogsOptions struct {
    Limit int
    Since time.Time
    Until time.Time
}
```

---

## Log Service

```go
// internal/logs/service.go
package logs

import (
    "context"
    "log/slog"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/vangoframework/rhone/internal/database/queries"
    "github.com/vangoframework/rhone/internal/fly"
)

type Service struct {
    flyClient *fly.Client
    queries   *queries.Queries
    logger    *slog.Logger

    // Active log streams per app
    streams map[uuid.UUID]*AppLogStream
    mu      sync.RWMutex
}

type AppLogStream struct {
    AppID       uuid.UUID
    FlyAppName  string
    Subscribers map[string]chan LogEntry
    Cancel      context.CancelFunc
    mu          sync.RWMutex
}

type LogEntry struct {
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    Level     string    `json:"level"`
    Message   string    `json:"message"`
    Source    string    `json:"source"` // app, system, build
    Region    string    `json:"region"`
    Instance  string    `json:"instance"`
}

func NewService(flyClient *fly.Client, queries *queries.Queries, logger *slog.Logger) *Service {
    return &Service{
        flyClient: flyClient,
        queries:   queries,
        logger:    logger,
        streams:   make(map[uuid.UUID]*AppLogStream),
    }
}

// Subscribe creates a new subscription to app logs
func (s *Service) Subscribe(ctx context.Context, appID uuid.UUID) (<-chan LogEntry, string, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    stream, exists := s.streams[appID]
    if !exists {
        // Get app details
        app, err := s.queries.GetApp(ctx, appID)
        if err != nil {
            return nil, "", err
        }

        if app.FlyAppID == nil {
            return nil, "", fmt.Errorf("app not deployed")
        }

        // Create new stream
        streamCtx, cancel := context.WithCancel(context.Background())
        stream = &AppLogStream{
            AppID:       appID,
            FlyAppName:  *app.FlyAppID,
            Subscribers: make(map[string]chan LogEntry),
            Cancel:      cancel,
        }
        s.streams[appID] = stream

        // Start streaming from Fly
        go s.streamFromFly(streamCtx, stream)
    }

    // Add subscriber
    subID := uuid.New().String()
    ch := make(chan LogEntry, 100)

    stream.mu.Lock()
    stream.Subscribers[subID] = ch
    stream.mu.Unlock()

    s.logger.Info("log subscriber added",
        "app_id", appID,
        "subscriber_id", subID,
        "total_subscribers", len(stream.Subscribers),
    )

    return ch, subID, nil
}

// Unsubscribe removes a log subscription
func (s *Service) Unsubscribe(appID uuid.UUID, subID string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    stream, exists := s.streams[appID]
    if !exists {
        return
    }

    stream.mu.Lock()
    if ch, ok := stream.Subscribers[subID]; ok {
        close(ch)
        delete(stream.Subscribers, subID)
    }
    subscriberCount := len(stream.Subscribers)
    stream.mu.Unlock()

    s.logger.Info("log subscriber removed",
        "app_id", appID,
        "subscriber_id", subID,
        "remaining_subscribers", subscriberCount,
    )

    // Clean up stream if no subscribers
    if subscriberCount == 0 {
        stream.Cancel()
        delete(s.streams, appID)
        s.logger.Info("log stream closed", "app_id", appID)
    }
}

// streamFromFly reads logs from Fly and broadcasts to subscribers
func (s *Service) streamFromFly(ctx context.Context, stream *AppLogStream) {
    flyLogs, err := s.flyClient.LogStream(ctx, stream.FlyAppName, fly.LogStreamOptions{})
    if err != nil {
        s.logger.Error("failed to start fly log stream",
            "app_id", stream.AppID,
            "error", err,
        )
        return
    }

    for {
        select {
        case <-ctx.Done():
            return
        case flyEntry, ok := <-flyLogs:
            if !ok {
                // Reconnect after delay
                time.Sleep(5 * time.Second)
                flyLogs, err = s.flyClient.LogStream(ctx, stream.FlyAppName, fly.LogStreamOptions{})
                if err != nil {
                    s.logger.Error("failed to reconnect log stream",
                        "app_id", stream.AppID,
                        "error", err,
                    )
                    return
                }
                continue
            }

            // Convert to our format
            entry := LogEntry{
                ID:        uuid.New().String(),
                Timestamp: flyEntry.Timestamp,
                Level:     parseLogLevel(flyEntry.Level, flyEntry.Message),
                Message:   flyEntry.Message,
                Source:    "app",
                Region:    flyEntry.Region,
                Instance:  flyEntry.Instance,
            }

            // Broadcast to all subscribers
            stream.mu.RLock()
            for _, ch := range stream.Subscribers {
                select {
                case ch <- entry:
                default:
                    // Subscriber too slow, skip
                }
            }
            stream.mu.RUnlock()
        }
    }
}

// GetHistory retrieves historical logs
func (s *Service) GetHistory(ctx context.Context, appID uuid.UUID, opts HistoryOptions) ([]LogEntry, error) {
    app, err := s.queries.GetApp(ctx, appID)
    if err != nil {
        return nil, err
    }

    if app.FlyAppID == nil {
        return nil, fmt.Errorf("app not deployed")
    }

    flyOpts := fly.GetLogsOptions{
        Limit: opts.Limit,
        Since: opts.Since,
        Until: opts.Until,
    }

    flyLogs, err := s.flyClient.GetLogs(ctx, *app.FlyAppID, flyOpts)
    if err != nil {
        return nil, err
    }

    entries := make([]LogEntry, 0, len(flyLogs))
    for _, fl := range flyLogs {
        // Filter by level if specified
        level := parseLogLevel(fl.Level, fl.Message)
        if opts.Level != "" && level != opts.Level {
            continue
        }

        // Filter by search term if specified
        if opts.Search != "" && !containsIgnoreCase(fl.Message, opts.Search) {
            continue
        }

        entries = append(entries, LogEntry{
            ID:        uuid.New().String(),
            Timestamp: fl.Timestamp,
            Level:     level,
            Message:   fl.Message,
            Source:    "app",
            Region:    fl.Region,
            Instance:  fl.Instance,
        })
    }

    return entries, nil
}

type HistoryOptions struct {
    Limit  int
    Since  time.Time
    Until  time.Time
    Level  string
    Search string
}

// parseLogLevel attempts to extract log level from message
func parseLogLevel(level, message string) string {
    if level != "" {
        return level
    }

    // Try to parse from common log formats
    lowerMsg := strings.ToLower(message)
    switch {
    case strings.Contains(lowerMsg, "[error]") || strings.Contains(lowerMsg, "level=error"):
        return "error"
    case strings.Contains(lowerMsg, "[warn]") || strings.Contains(lowerMsg, "level=warn"):
        return "warn"
    case strings.Contains(lowerMsg, "[info]") || strings.Contains(lowerMsg, "level=info"):
        return "info"
    case strings.Contains(lowerMsg, "[debug]") || strings.Contains(lowerMsg, "level=debug"):
        return "debug"
    default:
        return "info"
    }
}

func containsIgnoreCase(s, substr string) bool {
    return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
```

---

## Metrics Service

```go
// internal/logs/metrics.go
package logs

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/vangoframework/rhone/internal/fly"
)

type MetricsService struct {
    flyClient *fly.Client
    logger    *slog.Logger
}

type AppMetrics struct {
    Timestamp   time.Time        `json:"timestamp"`
    CPU         float64          `json:"cpu_percent"`
    Memory      MemoryMetrics    `json:"memory"`
    Network     NetworkMetrics   `json:"network"`
    Requests    RequestMetrics   `json:"requests"`
    Instances   []InstanceStatus `json:"instances"`
}

type MemoryMetrics struct {
    UsedMB    float64 `json:"used_mb"`
    TotalMB   float64 `json:"total_mb"`
    UsedPct   float64 `json:"used_percent"`
}

type NetworkMetrics struct {
    BytesInPerSec  float64 `json:"bytes_in_per_sec"`
    BytesOutPerSec float64 `json:"bytes_out_per_sec"`
}

type RequestMetrics struct {
    TotalRequests  int64   `json:"total_requests"`
    RequestsPerSec float64 `json:"requests_per_sec"`
    AvgLatencyMs   float64 `json:"avg_latency_ms"`
    ErrorRate      float64 `json:"error_rate"`
}

type InstanceStatus struct {
    ID      string `json:"id"`
    Region  string `json:"region"`
    State   string `json:"state"`
    CPU     float64 `json:"cpu_percent"`
    MemoryMB float64 `json:"memory_mb"`
}

// GetMetrics retrieves current metrics for an app
func (s *MetricsService) GetMetrics(ctx context.Context, app queries.App) (*AppMetrics, error) {
    if app.FlyAppID == nil {
        return nil, fmt.Errorf("app not deployed")
    }

    // Get machine statuses
    machines, err := s.flyClient.ListMachines(ctx, *app.FlyAppID)
    if err != nil {
        return nil, fmt.Errorf("list machines: %w", err)
    }

    metrics := &AppMetrics{
        Timestamp: time.Now(),
        Instances: make([]InstanceStatus, 0, len(machines)),
    }

    var totalCPU, totalMemory float64
    var runningCount int

    for _, m := range machines {
        instance := InstanceStatus{
            ID:     m.ID,
            Region: m.Region,
            State:  m.State,
        }

        // Get detailed metrics for running machines
        if m.State == "started" {
            runningCount++

            // Fly provides metrics via the machines API
            machineMetrics, err := s.flyClient.GetMachineMetrics(ctx, *app.FlyAppID, m.ID)
            if err == nil {
                instance.CPU = machineMetrics.CPUPercent
                instance.MemoryMB = machineMetrics.MemoryUsedMB
                totalCPU += machineMetrics.CPUPercent
                totalMemory += machineMetrics.MemoryUsedMB
            }
        }

        metrics.Instances = append(metrics.Instances, instance)
    }

    // Calculate averages
    if runningCount > 0 {
        metrics.CPU = totalCPU / float64(runningCount)
        metrics.Memory = MemoryMetrics{
            UsedMB:  totalMemory,
            TotalMB: float64(runningCount * 256), // Assume 256MB per machine
            UsedPct: (totalMemory / float64(runningCount*256)) * 100,
        }
    }

    // Get request metrics from Fly (if available)
    requestMetrics, err := s.flyClient.GetAppMetrics(ctx, *app.FlyAppID)
    if err == nil {
        metrics.Requests = RequestMetrics{
            TotalRequests:  requestMetrics.TotalRequests,
            RequestsPerSec: requestMetrics.RequestsPerSec,
            AvgLatencyMs:   requestMetrics.AvgLatencyMs,
            ErrorRate:      requestMetrics.ErrorRate,
        }
        metrics.Network = NetworkMetrics{
            BytesInPerSec:  requestMetrics.BytesInPerSec,
            BytesOutPerSec: requestMetrics.BytesOutPerSec,
        }
    }

    return metrics, nil
}

// GetMetricsHistory retrieves historical metrics
func (s *MetricsService) GetMetricsHistory(ctx context.Context, app queries.App, duration time.Duration) ([]AppMetrics, error) {
    if app.FlyAppID == nil {
        return nil, fmt.Errorf("app not deployed")
    }

    // Fly provides historical metrics via Prometheus endpoints
    // For now, we return current metrics only
    // Future: integrate with Fly's Prometheus API

    current, err := s.GetMetrics(ctx, app)
    if err != nil {
        return nil, err
    }

    return []AppMetrics{*current}, nil
}
```

---

## HTTP Handlers

```go
// internal/handlers/logs.go
package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
)

// LogsLive streams logs via SSE
func (h *Handlers) LogsLive(w http.ResponseWriter, r *http.Request) {
    appID, err := uuid.Parse(chi.URLParam(r, "appID"))
    if err != nil {
        http.Error(w, "Invalid app ID", http.StatusBadRequest)
        return
    }

    // Verify access
    app, err := h.getAppWithAccess(r.Context(), appID)
    if err != nil {
        http.Error(w, "App not found", http.StatusNotFound)
        return
    }

    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming not supported", http.StatusInternalServerError)
        return
    }

    // Subscribe to logs
    logs, subID, err := h.logService.Subscribe(r.Context(), app.ID)
    if err != nil {
        fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
        flusher.Flush()
        return
    }
    defer h.logService.Unsubscribe(app.ID, subID)

    // Send initial connection message
    fmt.Fprintf(w, "event: connected\ndata: {\"app_id\":\"%s\"}\n\n", app.ID)
    flusher.Flush()

    // Stream logs
    for {
        select {
        case <-r.Context().Done():
            return
        case entry, ok := <-logs:
            if !ok {
                fmt.Fprintf(w, "event: disconnected\ndata: {}\n\n")
                flusher.Flush()
                return
            }

            data, _ := json.Marshal(entry)
            fmt.Fprintf(w, "event: log\ndata: %s\n\n", data)
            flusher.Flush()
        }
    }
}

// LogsHistory returns historical logs
func (h *Handlers) LogsHistory(w http.ResponseWriter, r *http.Request) {
    appID, err := uuid.Parse(chi.URLParam(r, "appID"))
    if err != nil {
        http.Error(w, "Invalid app ID", http.StatusBadRequest)
        return
    }

    // Verify access
    app, err := h.getAppWithAccess(r.Context(), appID)
    if err != nil {
        http.Error(w, "App not found", http.StatusNotFound)
        return
    }

    // Parse query params
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit <= 0 || limit > 1000 {
        limit = 100
    }

    opts := logs.HistoryOptions{
        Limit:  limit,
        Level:  r.URL.Query().Get("level"),
        Search: r.URL.Query().Get("search"),
    }

    if since := r.URL.Query().Get("since"); since != "" {
        if t, err := time.Parse(time.RFC3339, since); err == nil {
            opts.Since = t
        }
    }
    if until := r.URL.Query().Get("until"); until != "" {
        if t, err := time.Parse(time.RFC3339, until); err == nil {
            opts.Until = t
        }
    }

    entries, err := h.logService.GetHistory(r.Context(), app.ID, opts)
    if err != nil {
        h.logger.Error("failed to get log history", "error", err)
        http.Error(w, "Failed to get logs", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "logs":  entries,
        "count": len(entries),
    })
}

// Metrics returns current app metrics
func (h *Handlers) Metrics(w http.ResponseWriter, r *http.Request) {
    appID, err := uuid.Parse(chi.URLParam(r, "appID"))
    if err != nil {
        http.Error(w, "Invalid app ID", http.StatusBadRequest)
        return
    }

    // Verify access
    app, err := h.getAppWithAccess(r.Context(), appID)
    if err != nil {
        http.Error(w, "App not found", http.StatusNotFound)
        return
    }

    metrics, err := h.metricsService.GetMetrics(r.Context(), *app)
    if err != nil {
        h.logger.Error("failed to get metrics", "error", err)
        http.Error(w, "Failed to get metrics", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(metrics)
}
```

---

## Templates

### Log Viewer Component

```go
// internal/templates/components/logs.templ
package components

templ LogViewer(appID string) {
    <div class="logs-container" id="logs-container">
        <!-- Controls -->
        <div class="logs-controls flex items-center gap-4 mb-4 p-4 bg-gray-50 rounded-lg">
            <div class="flex items-center gap-2">
                <label class="text-sm font-medium">Level:</label>
                <select
                    id="log-level-filter"
                    class="px-3 py-1.5 border rounded text-sm"
                    hx-get={ "/apps/" + appID + "/logs/history" }
                    hx-target="#logs-list"
                    hx-include="[name='search']"
                    hx-trigger="change"
                    name="level"
                >
                    <option value="">All</option>
                    <option value="error">Error</option>
                    <option value="warn">Warning</option>
                    <option value="info">Info</option>
                    <option value="debug">Debug</option>
                </select>
            </div>

            <div class="flex-1">
                <input
                    type="search"
                    name="search"
                    placeholder="Search logs..."
                    class="w-full px-3 py-1.5 border rounded text-sm"
                    hx-get={ "/apps/" + appID + "/logs/history" }
                    hx-target="#logs-list"
                    hx-include="[name='level']"
                    hx-trigger="keyup changed delay:300ms"
                />
            </div>

            <div class="flex items-center gap-2">
                <button
                    id="live-toggle"
                    class="px-4 py-1.5 bg-green-500 text-white rounded text-sm font-medium hover:bg-green-600"
                    onclick="toggleLiveStream()"
                >
                    <span id="live-status">● Live</span>
                </button>

                <button
                    class="px-4 py-1.5 border rounded text-sm hover:bg-gray-100"
                    onclick="clearLogs()"
                >
                    Clear
                </button>
            </div>
        </div>

        <!-- Log Output -->
        <div
            id="logs-list"
            class="logs-output bg-gray-900 text-gray-100 rounded-lg p-4 font-mono text-sm h-96 overflow-y-auto"
        >
            <div class="text-gray-500 text-center py-8">
                Connecting to log stream...
            </div>
        </div>

        <!-- Auto-scroll toggle -->
        <div class="mt-2 flex items-center gap-2 text-sm text-gray-600">
            <input type="checkbox" id="auto-scroll" checked />
            <label for="auto-scroll">Auto-scroll</label>
        </div>
    </div>

    <script>
        let eventSource = null;
        let isLive = true;
        const appId = '{ appID }';

        function connectLogStream() {
            if (eventSource) {
                eventSource.close();
            }

            eventSource = new EventSource(`/apps/${appId}/logs/live`);

            eventSource.addEventListener('connected', function(e) {
                console.log('Log stream connected');
            });

            eventSource.addEventListener('log', function(e) {
                const entry = JSON.parse(e.data);
                appendLogEntry(entry);
            });

            eventSource.addEventListener('error', function(e) {
                console.error('Log stream error');
                document.getElementById('live-status').textContent = '○ Disconnected';
                document.getElementById('live-toggle').classList.remove('bg-green-500');
                document.getElementById('live-toggle').classList.add('bg-red-500');
            });
        }

        function appendLogEntry(entry) {
            const container = document.getElementById('logs-list');
            const div = document.createElement('div');
            div.className = `log-entry log-${entry.level}`;

            const time = new Date(entry.timestamp).toLocaleTimeString();
            const levelClass = {
                'error': 'text-red-400',
                'warn': 'text-yellow-400',
                'info': 'text-blue-400',
                'debug': 'text-gray-400'
            }[entry.level] || 'text-gray-300';

            div.innerHTML = `
                <span class="text-gray-500">${time}</span>
                <span class="${levelClass} uppercase font-bold">[${entry.level}]</span>
                <span class="text-gray-200">${escapeHtml(entry.message)}</span>
            `;

            container.appendChild(div);

            // Auto-scroll if enabled
            if (document.getElementById('auto-scroll').checked) {
                container.scrollTop = container.scrollHeight;
            }

            // Limit entries
            while (container.children.length > 1000) {
                container.removeChild(container.firstChild);
            }
        }

        function toggleLiveStream() {
            isLive = !isLive;
            const btn = document.getElementById('live-toggle');
            const status = document.getElementById('live-status');

            if (isLive) {
                connectLogStream();
                status.textContent = '● Live';
                btn.classList.remove('bg-gray-500');
                btn.classList.add('bg-green-500');
            } else {
                if (eventSource) {
                    eventSource.close();
                    eventSource = null;
                }
                status.textContent = '○ Paused';
                btn.classList.remove('bg-green-500');
                btn.classList.add('bg-gray-500');
            }
        }

        function clearLogs() {
            const container = document.getElementById('logs-list');
            container.innerHTML = '<div class="text-gray-500">Logs cleared</div>';
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        // Start streaming on load
        document.addEventListener('DOMContentLoaded', connectLogStream);

        // Cleanup on page leave
        window.addEventListener('beforeunload', function() {
            if (eventSource) {
                eventSource.close();
            }
        });
    </script>

    <style>
        .log-entry {
            padding: 2px 0;
            border-bottom: 1px solid rgba(255,255,255,0.05);
        }
        .log-entry:hover {
            background: rgba(255,255,255,0.05);
        }
        .log-error {
            background: rgba(239, 68, 68, 0.1);
        }
        .log-warn {
            background: rgba(234, 179, 8, 0.1);
        }
    </style>
}
```

### Metrics Dashboard Component

```go
// internal/templates/components/metrics.templ
package components

templ MetricsDashboard(appID string) {
    <div
        class="metrics-dashboard grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4"
        hx-get={ "/apps/" + appID + "/metrics" }
        hx-trigger="load, every 10s"
        hx-swap="innerHTML"
    >
        <div class="text-center text-gray-500 py-8">
            Loading metrics...
        </div>
    </div>
}

templ MetricsCards(metrics AppMetrics) {
    <!-- CPU -->
    <div class="bg-white rounded-lg shadow p-4">
        <div class="text-sm text-gray-500 mb-1">CPU Usage</div>
        <div class="text-2xl font-bold">{ fmt.Sprintf("%.1f%%", metrics.CPU) }</div>
        <div class="mt-2 h-2 bg-gray-200 rounded-full">
            <div
                class="h-full bg-blue-500 rounded-full"
                style={ fmt.Sprintf("width: %.1f%%", min(metrics.CPU, 100)) }
            ></div>
        </div>
    </div>

    <!-- Memory -->
    <div class="bg-white rounded-lg shadow p-4">
        <div class="text-sm text-gray-500 mb-1">Memory</div>
        <div class="text-2xl font-bold">
            { fmt.Sprintf("%.0f", metrics.Memory.UsedMB) } MB
        </div>
        <div class="text-sm text-gray-500">
            of { fmt.Sprintf("%.0f", metrics.Memory.TotalMB) } MB
        </div>
        <div class="mt-2 h-2 bg-gray-200 rounded-full">
            <div
                class="h-full bg-green-500 rounded-full"
                style={ fmt.Sprintf("width: %.1f%%", metrics.Memory.UsedPct) }
            ></div>
        </div>
    </div>

    <!-- Requests -->
    <div class="bg-white rounded-lg shadow p-4">
        <div class="text-sm text-gray-500 mb-1">Requests/sec</div>
        <div class="text-2xl font-bold">
            { fmt.Sprintf("%.1f", metrics.Requests.RequestsPerSec) }
        </div>
        <div class="text-sm text-gray-500">
            { fmt.Sprintf("%.0fms avg latency", metrics.Requests.AvgLatencyMs) }
        </div>
    </div>

    <!-- Instances -->
    <div class="bg-white rounded-lg shadow p-4">
        <div class="text-sm text-gray-500 mb-1">Instances</div>
        <div class="text-2xl font-bold">
            { fmt.Sprintf("%d", countRunning(metrics.Instances)) }
            <span class="text-sm font-normal text-gray-500">running</span>
        </div>
        <div class="mt-2 flex gap-1">
            for _, inst := range metrics.Instances {
                <div
                    class={ "w-3 h-3 rounded-full", instanceColor(inst.State) }
                    title={ fmt.Sprintf("%s (%s)", inst.ID[:8], inst.Region) }
                ></div>
            }
        </div>
    </div>
}

func countRunning(instances []InstanceStatus) int {
    count := 0
    for _, i := range instances {
        if i.State == "started" {
            count++
        }
    }
    return count
}

func instanceColor(state string) string {
    switch state {
    case "started":
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

// Log routes
r.Route("/apps/{appID}/logs", func(r chi.Router) {
    r.Use(h.RequireAuth)
    r.Get("/live", h.LogsLive)
    r.Get("/history", h.LogsHistory)
})

// Metrics routes
r.Route("/apps/{appID}/metrics", func(r chi.Router) {
    r.Use(h.RequireAuth)
    r.Get("/", h.Metrics)
})
```

---

## Exit Criteria

Phase 9 is complete when:

1. [ ] Real-time log streaming works via SSE
2. [ ] Logs display with proper formatting (level colors)
3. [ ] Log filtering by level works
4. [ ] Log search works
5. [ ] Auto-scroll can be toggled
6. [ ] Historical logs can be retrieved
7. [ ] Metrics dashboard shows CPU/memory
8. [ ] Metrics refresh automatically
9. [ ] Instance count and status displayed
10. [ ] Graceful handling of disconnects/reconnects

---

## Dependencies

- **Requires**: Phase 5 (deployed apps to get logs from)
- **Required by**: Phase 12 (monitoring is part of production readiness)

---

*Phase 9 Specification - Version 1.0*
