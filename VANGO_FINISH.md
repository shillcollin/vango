# Flaw and idea for better design:

# Design Deep Dive: Client-Side Persistence in Vango

> **Problem**: How do we persist user preferences (Theme, Sidebar State) across sessions in a framework where state lives on the server?

---

## 1. The Context: How Vango Works

Vango is a **Server-Driven UI** framework. This means:
1.  **State is on the Server**: All `Signal[T]` values live in process memory (RAM) on the Go server.
2.  **UI is on the Client**: The browser is a "Thin Client" that just renders DOM updates sent by the server.
3.  **The Bridge is Volatile**: The connection is a WebSocket. If the user refreshes the page, the WebSocket disconnects, the server-side session is destroyed, and all state in RAM is lost.

### The Challenge
Users expect certain UI states to survive a refresh:
- "Dark Mode"
- "Sidebar Collapsed"
- "Table Sort Order"
- "Language Preference"

In a traditional SPA (React), these are stored in `localStorage`.
In Vango, the server (where the logic lives) cannot synchronously read `localStorage` (where the data lives).

---

## 2. The Initial Idea: `.Persist()`

The initial spec proposed a "Magic" API inspired by client-side reactivity libraries:

```go
// The aspirational API
var SidebarOpen = vango.Signal(false).Persist(vango.LocalStorage, "sidebar")
```

### Why it was attractive
- **DX**: One liner. Looks just like a standard signal.
- **Familiarity**: Similar to MobX or Zustand middlewares.

### Why it is Flawed (The "Context Gap")

This API implies that a global or component-level variable can "know" the value of `localStorage` at initialization time.

**Fail 1: Timing (The Lie)**
When `vango.Signal(false)` executes on the server, the client hasn't connected yet.
- Server: "I am initializing this component."
- Client: *Still downloading `vango.js`...*
- Result: The signal *must* initialize with `false`. Later, when the client connects, it *might* update to `true`. This causes logic bugs where the code assumes the stored value is available immediately.

**Fail 2: Context (The Void)**
```go
// Global definition
var Theme = vango.Signal("light").Persist(LocalStorage, "theme")
```
Whose localStorage? User A's? User B's?
Without a `vango.Ctx` reference, the server doesn't know *which* connection to talk to.

**Fail 3: Performance (The Flood)**
If `.Persist()` works by sending every `.Set()` to the client, and we have 100 persisted signals, we risk spamming the WebSocket channel with minor state updates that might not even be needed.

---

## 3. The Architecture Constraint

We are bound by the laws of physics:
1.  **Server cannot read Client synchronousy.**
2.  **Cookies** are the only data sent *before* the WebSocket connects (via HTTP headers).
3.  **Handshake** is the first moment the Client can send arbitrary data (via WebSocket).

## 4. A potential solution

This feedback is gold. Both responses identify the same pressure points, which validates that these are the real edge cases to solve. Let me synthesize and propose concrete solutions for each.

---

## Synthesis: The Gaps We Need to Close

| Gap | Identified By | Severity |
|-----|---------------|----------|
| Server restart loses all state | Both | High (production blocker) |
| Sticky sessions are a hard constraint | Both | High (limits deployment options) |
| 30s window may be too short/rigid | Doc 11 | Medium |
| Memory protection for detached sessions | Doc 10 | Medium |
| "Reconnecting" UI feedback | Doc 10 | Medium |
| URLParam history spam | Doc 10 | Medium |
| URLParam vs path param conflict | Doc 10 | Low (documentation) |
| Pref merge strategy edge cases | Both | Medium |
| Cross-device sync race conditions | Doc 11 | Medium |
| Form draft persistence pattern | Both | Medium (documentation) |
| Testing utilities | Doc 11 | High (DX) |
| init() global state for testing | Doc 10 | Low |

---

## Proposed Solutions

### 1. Session Serialization (The "Graceful Restart" Problem)

Both reviewers flagged this. The solution is an **optional serialization interface**:

```go
// For teams that need server restarts without losing sessions
type SessionStore interface {
    Save(sessionID string, data []byte) error
    Load(sessionID string) ([]byte, error)
    Delete(sessionID string) error
    // Called on graceful shutdown
    SaveAll(sessions map[string][]byte) error
}

// Built-in implementations
vango.MemoryStore()           // Default: no persistence
vango.RedisStore(client)      // Redis-backed
vango.SQLStore(db, "sessions") // Database-backed
```

**What gets serialized?**

Not goroutines or channels—just the **Signal values**:

```go
type SerializableSession struct {
    ID        string
    Token     string
    Signals   map[string]json.RawMessage  // Signal key → JSON value
    Prefs     map[string]json.RawMessage  // Pref cache
    URLParams map[string]string
    CreatedAt time.Time
    UserID    *string  // If authenticated
}
```

**Lifecycle with serialization:**

```
┌─────────────────────────────────────────────────────────────────┐
│                    SESSION LIFECYCLE (WITH STORE)                │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. User connects                                                │
│     ├── Check store for existing session token                  │
│     ├── If found: deserialize → rehydrate component tree        │
│     └── If not: create new session                              │
│                                                                  │
│  2. User disconnects                                             │
│     ├── Move to Detached state                                  │
│     ├── Serialize signal values to store (async)                │
│     └── Start grace period timer                                │
│                                                                  │
│  3. Server receives SIGTERM (graceful shutdown)                 │
│     ├── Stop accepting new connections                          │
│     ├── Serialize ALL active sessions to store                  │
│     └── Exit                                                     │
│                                                                  │
│  4. New server instance starts                                   │
│     ├── Sessions exist in store (Redis/DB)                      │
│     └── Clients reconnect → rehydrate from store                │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**The constraint**: Signals must be JSON-serializable. This is already true for most state. For non-serializable state (channels, functions), those Signals are marked as `Transient` and excluded:

```go
// This survives restart
count := vango.Signal(0)

// This does NOT survive restart (channels can't serialize)
updates := vango.Signal(make(chan Event)).Transient()
```

**Config:**

```go
app := vango.New(vango.Config{
    Session: vango.SessionConfig{
        ResumeWindow: 30 * time.Second,
        
        // Optional: Enable session persistence
        Store: vango.RedisStore(redisClient, vango.StoreConfig{
            Prefix:     "vango:session:",
            Expiration: 5 * time.Minute,  // Longer than ResumeWindow
        }),
    },
})
```

---

### 2. Memory Protection for Detached Sessions

Both reviewers raised the "10,000 tabs" attack vector.

**Solution**: Separate limits + LRU eviction:

```go
Session: vango.SessionConfig{
    // Total sessions (connected + detached)
    MaxSessions: 10000,
    
    // Detached sessions can't exceed this
    MaxDetachedSessions: 1000,
    
    // If at limit, evict oldest detached first
    EvictionPolicy: vango.LRU,
    
    // Per-IP rate limiting for new sessions
    MaxSessionsPerIP: 50,
}
```

**Behavior when limits are hit:**

1. New connection when at `MaxSessions`: Reject with 503
2. New detach when at `MaxDetachedSessions`: Evict oldest detached session
3. Memory pressure (configurable threshold): Aggressively evict detached sessions

---

### 3. Configurable Resume Window

Doc 11 correctly points out that 30s is arbitrary. Different use cases need different windows.

**Solution**: Route-level overrides:

```go
// Global default
app := vango.New(vango.Config{
    Session: vango.SessionConfig{
        ResumeWindow: 30 * time.Second,
    },
})

// Route-specific override for complex wizards
app.Route("/checkout/wizard", WizardHandler, vango.RouteConfig{
    ResumeWindow: 5 * time.Minute,
})

// Shorter window for simple pages
app.Route("/dashboard", DashboardHandler, vango.RouteConfig{
    ResumeWindow: 10 * time.Second,
})
```

---

### 4. "Reconnecting" UI Feedback

Doc 10 correctly identifies that users need visual feedback during disconnect.

**Solution**: Built-in CSS classes + optional toast:

```go
// vango.js automatically manages these classes on <html>
// .vango-connected    - Normal state
// .vango-connecting   - Initial connection in progress  
// .vango-reconnecting - Disconnected, attempting to reconnect
// .vango-disconnected - Gave up (session expired or error)
```

**CSS usage:**

```css
/* Show reconnecting overlay */
.vango-reconnecting::after {
    content: "Reconnecting...";
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    padding: 8px;
    background: #fbbf24;
    text-align: center;
    z-index: 9999;
}

/* Disable interactions while reconnecting */
.vango-reconnecting main {
    pointer-events: none;
    opacity: 0.7;
}
```

**Optional toast helper:**

```go
Head(
    // Injects minimal JS for toast notifications
    vango.ReconnectToast(vango.ToastConfig{
        Reconnecting: "Connection lost. Reconnecting...",
        Reconnected:  "Connected!",
        Failed:       "Connection failed. Please refresh.",
    }),
)
```

---

### 5. URLParam History Mode

Doc 10 correctly identifies the history spam problem.

**Solution**: `Push` vs `Replace` modes:

```go
// Default: Push (creates history entry)
page := vango.URLParam[int](ctx, "page", 1)
// Clicking page 2, then page 3 = two history entries
// Back button goes: 3 → 2 → 1

// Replace mode (no history entry)
search := vango.URLParam[string](ctx, "q", "", vango.Replace)
// Typing "hello" doesn't create history entries
// Only creates entry on blur/submit

// Replace with debounce (common pattern for search)
search := vango.URLParam[string](ctx, "q", "", 
    vango.Replace,
    vango.Debounce(300*time.Millisecond))
```

---

### 6. URLParam vs Path Parameter Clarification

Doc 10 asks about the relationship between URLParam and route parameters.

**Answer**: They're distinct concepts:

```go
// Route definition: path parameters
app.Route("/projects/{id}", ProjectHandler)

// Inside handler: path param from router
func ProjectHandler(ctx vango.Ctx) vango.Component {
    // Path parameter - from the router
    projectID := ctx.Param("id")  // e.g., "123" from /projects/123
    
    // Query parameter - from URLParam
    tab := vango.URLParam[string](ctx, "tab", "overview")  // ?tab=settings
    
    // Full URL: /projects/123?tab=settings
}
```

**Documentation should clarify:**
- Path parameters (`/projects/{id}`) → `ctx.Param()`
- Query parameters (`?tab=settings`) → `vango.URLParam()`

---

### 7. Complex URLParam Encoding

Both reviewers noted that Base64 JSON is ugly.

**Solution**: Multiple encoding strategies:

```go
// Simple types: direct encoding (default)
page := vango.URLParam[int](ctx, "page", 1)
// URL: ?page=5

// String arrays: comma-separated
tags := vango.URLParam[[]string](ctx, "tags", nil, vango.CommaSeparated)
// URL: ?tags=go,web,api

// Complex structs: compressed encoding
filters := vango.URLParam[Filters](ctx, "f", Filters{}, vango.CompressedJSON)
// URL: ?f=eJxLz... (shorter than base64)

// Alternative: named params for common struct fields
type Filters struct {
    Category string `url:"cat"`
    MinPrice int    `url:"min"`
    MaxPrice int    `url:"max"`
}
filters := vango.URLParam[Filters](ctx, "", Filters{}, vango.FlattenStruct)
// URL: ?cat=electronics&min=100&max=500
```

The `FlattenStruct` option is particularly useful—it spreads struct fields into individual query params, making URLs human-readable.

---

### 8. Pref Merge Strategy (Refined)

Both reviewers asked about the multi-device scenario. Here's the precise behavior:

```go
// The merge strategy, fully specified
type MergeStrategy struct {
    // When both DB and localStorage have a value for the same key
    OnConflict: ConflictResolution  // DBWins (default), LocalWins, Prompt
    
    // When DB is missing a key that localStorage has
    OnMissingInDB: MissingResolution  // Adopt (default), Ignore
    
    // When localStorage is missing a key that DB has
    OnMissingInLocal: MissingResolution  // Adopt (default), Ignore
    
    // Notify user when their local pref was overwritten
    NotifyOnOverwrite: bool
}

const (
    DBWins    ConflictResolution = iota  // Server is authoritative
    LocalWins                             // Client is authoritative (rare)
    Prompt                                // Ask user (most UX-friendly)
)
```

**Scenario walkthrough:**

```
User's journey:
───────────────────────────────────────────────────────────────────

1. Desktop (anonymous): Set theme = "dark"
   └── localStorage["theme"] = "dark"

2. Create account on Desktop
   └── DB["theme"] = "dark" (adopted from localStorage)

3. Phone (anonymous): Has localStorage["theme"] = "light" (default)

4. Login on Phone
   ├── DB has: theme = "dark"
   ├── Local has: theme = "light"  
   ├── Conflict! OnConflict = DBWins
   ├── Result: Phone shows "dark"
   └── If NotifyOnOverwrite: Toast "Synced your dark mode preference"

5. User changes to "light" on Phone
   └── DB updates to "light", syncs to Desktop (if Realtime)
```

**The key insight**: DB is authoritative for authenticated users. localStorage is just a cache. On login, DB wins any conflict. This is the least surprising behavior.

---

### 9. Cross-Device Sync Consistency

Doc 11 raised the race condition concern.

**Solution**: Last-Write-Wins with timestamps:

```go
type PrefValue struct {
    Value     json.RawMessage
    UpdatedAt time.Time  // Millisecond precision
    DeviceID  string     // For debugging/audit
}

// When two devices update simultaneously:
// 1. Both send updates with their local timestamp
// 2. Server keeps the one with the later timestamp
// 3. Server broadcasts the winner to all sessions
// 4. "Loser" device sees its change reverted

// This is eventually consistent, not strongly consistent
// For prefs (theme, sidebar), this is fine
```

**Documentation should state**: "Pref sync uses last-write-wins. If you change a pref on two devices within the same second, the result is non-deterministic. For prefs like theme/language, this is acceptable UX."

For apps that need stronger consistency, they should use server state (Signals backed by DB), not Prefs.

---

### 10. Form Draft Persistence Pattern

Both reviewers identified this as a common need.

**Solution**: Document as a pattern + optional helper:

```go
// Pattern 1: Use Pref directly for long-lived drafts
var ApplicationDraft = vango.Pref("app_draft", vango.PrefConfig[ApplicationForm]{
    Default: ApplicationForm{},
    TTL:     7 * 24 * time.Hour,  // Auto-expire after 7 days
})

func ApplicationWizard(ctx vango.Ctx) vango.Component {
    draft := ApplicationDraft.Use(ctx)
    
    return Form(
        OnInput(func(data ApplicationForm) {
            draft.Set(data)  // Auto-saves on every change (debounced)
        }),
        OnSubmit(func(data ApplicationForm) {
            submitApplication(data)
            draft.Clear()  // Clear draft on successful submit
        }),
    )
}

// Pattern 2: UseForm hook with built-in persistence
form := vango.UseForm(ctx, ApplicationSchema, vango.FormConfig{
    Persist: "application_draft",  // Saves to Pref automatically
    PersistDebounce: 500 * time.Millisecond,
    PersistTTL: 7 * 24 * time.Hour,
})
```

---

### 11. Testing Utilities

Doc 11 correctly identifies this as critical for DX.

**Solution**: First-class test helpers:

```go
func TestCartSurvivesRefresh(t *testing.T) {
    // Create test session
    session := vango.NewTestSession(t)
    
    // Mount component
    session.Mount(CartPage)
    
    // Interact
    session.Click("#add-item")
    session.Click("#add-item")
    
    // Assert state
    cart := session.GetSharedSignal(CartItems)
    assert.Equal(t, 2, len(cart))
    
    // Simulate refresh (disconnect + reconnect within grace period)
    session.SimulateRefresh()
    
    // Assert state survived
    cart = session.GetSharedSignal(CartItems)
    assert.Equal(t, 2, len(cart))
}

func TestURLParamUpdatesURL(t *testing.T) {
    session := vango.NewTestSession(t)
    session.Mount(ProductList)
    
    // Check initial URL
    assert.Equal(t, "1", session.URL().Query().Get("page"))
    
    // Click next page
    session.Click("#next-page")
    
    // Assert URL updated
    assert.Equal(t, "2", session.URL().Query().Get("page"))
    
    // Simulate back button
    session.SimulateBack()
    
    // Assert state reverted
    assert.Equal(t, "1", session.URL().Query().Get("page"))
}

func TestPrefPersistsAcrossLogins(t *testing.T) {
    session := vango.NewTestSession(t)
    
    // Anonymous: set theme
    session.Mount(App)
    session.SetPref(prefs.Theme, "dark")
    
    // Create account
    session.SimulateLogin("user123")
    
    // Assert theme persisted to DB
    dbTheme := testDB.GetPref("user123", "theme")
    assert.Equal(t, "dark", dbTheme)
    
    // New session, same user
    session2 := vango.NewTestSession(t)
    session2.SimulateLogin("user123")
    session2.Mount(App)
    
    // Assert theme loaded from DB
    assert.Equal(t, "dark", session2.GetPref(prefs.Theme))
}
```

**Test utilities provided:**

```go
// Session lifecycle
session.Mount(component)
session.SimulateRefresh()           // Disconnect + reconnect
session.SimulateDisconnect()        // Just disconnect
session.SimulateReconnect()         // Just reconnect  
session.SimulateGracePeriodExpiry() // Force session death
session.SimulateServerRestart()     // Serialize + deserialize

// User actions
session.Click(selector)
session.Type(selector, text)
session.Submit(selector)

// Navigation
session.SimulateBack()
session.SimulateForward()
session.Navigate(path)

// State inspection
session.GetSignal(signal)
session.GetSharedSignal(signal)
session.GetPref(pref)
session.URL()

// Auth
session.SimulateLogin(userID)
session.SimulateLogout()
```

---

## Updated Configuration (Complete)

```go
app := vango.New(vango.Config{
    // ════════════════════════════════════════════════════════════
    // SESSION MANAGEMENT
    // ════════════════════════════════════════════════════════════
    Session: vango.SessionConfig{
        // Grace period for reconnection
        ResumeWindow: 30 * time.Second,
        
        // Memory limits
        MaxSessions:          10000,
        MaxDetachedSessions:  1000,
        MaxSessionsPerIP:     50,
        EvictionPolicy:       vango.LRU,
        
        // Optional: Persistence for non-sticky deployments
        Store: vango.RedisStore(redisClient, vango.StoreConfig{
            Prefix:     "vango:session:",
            Expiration: 5 * time.Minute,
        }),
        
        // Token configuration
        TokenLength:   32,
        TokenCookie:   "vango_session",
        TokenStorage:  vango.SessionStorage,  // sessionStorage (per-tab)
    },
    
    // ════════════════════════════════════════════════════════════
    // USER PREFERENCES
    // ════════════════════════════════════════════════════════════
    Prefs: vango.PrefsConfig{
        // Database for authenticated users (nil = anon-only mode)
        Store: vango.PostgresStore(db, "user_preferences"),
        
        // Merge strategy for login
        MergeStrategy: vango.MergeStrategy{
            OnConflict:        vango.DBWins,
            OnMissingInDB:     vango.Adopt,
            NotifyOnOverwrite: true,
        },
        
        // Cookie settings
        Cookie: vango.CookieConfig{
            Name:     "vango_prefs",
            MaxAge:   365 * 24 * time.Hour,
            Secure:   true,
            SameSite: http.SameSiteLaxMode,
        },
        
        // Sync settings
        RealtimeSync: true,  // Broadcast pref changes to all user sessions
        SyncConsistency: vango.LastWriteWins,
    },
    
    // ════════════════════════════════════════════════════════════
    // URL PARAMETERS
    // ════════════════════════════════════════════════════════════
    URLParams: vango.URLParamConfig{
        MaxURLLength:    2048,
        MaxParamLength:  500,
        DefaultMode:     vango.Push,      // vs vango.Replace
        DefaultDebounce: 100 * time.Millisecond,
        StructEncoding:  vango.FlattenStruct,  // vs CompressedJSON
    },
    
    // ════════════════════════════════════════════════════════════
    // UI FEEDBACK
    // ════════════════════════════════════════════════════════════
    UI: vango.UIConfig{
        // CSS classes for connection state
        ConnectionClasses: true,  // Adds .vango-connected, etc. to <html>
        
        // Optional built-in toast
        ReconnectToast: &vango.ToastConfig{
            Reconnecting: "Reconnecting...",
            Reconnected:  "Connected!",
            Failed:       "Connection lost. Please refresh.",
        },
    },
})
```

---

## Updated Decision Flowchart

Incorporating Doc 10's suggestion:

```
                    "Where should this state live?"
                                 │
                                 ▼
                    ┌────────────────────────────┐
                    │ Must it be shareable via   │
                    │ URL? (pagination, search,  │
                    │ filters, deep links)       │
                    └────────────────────────────┘
                                 │
                    YES ─────────┴───────── NO
                     │                       │
                     ▼                       ▼
                 URLParam       ┌────────────────────────────┐
                                │ Must it survive closing    │
                                │ the browser tab?           │
                                └────────────────────────────┘
                                             │
                                YES ─────────┴───────── NO
                                 │                       │
                                 ▼                       ▼
                    ┌────────────────────────┐    Session Resumption
                    │ Is it purely client-   │    handles this!
                    │ side visual state?     │    Just use Signal.
                    │ (scroll, focus)        │
                    └────────────────────────┘
                                 │
                    YES ─────────┴───────── NO
                     │                       │
                     ▼                       ▼
                   Hook                    Pref
              (with localStorage)    (Cookie + DB + localStorage)
```

**Key insight**: The middle path (survives tab close = NO) leads to "just use Signal" because Session Resumption makes refresh survival automatic.

---

## Summary: What Changed

| Original Design | Refined Design |
|-----------------|----------------|
| Sticky sessions required | Sticky sessions OR Redis/DB-backed session store |
| Fixed 30s grace period | Configurable per-route |
| No memory protection | MaxDetachedSessions + LRU eviction + per-IP limits |
| No reconnection UI | Built-in CSS classes + optional toast |
| URLParam always pushes history | Push vs Replace modes |
| Base64 JSON for complex types | FlattenStruct + CommaSeparated + CompressedJSON options |
| Simple merge strategy | Full merge config with NotifyOnOverwrite |
| LWW for sync implied | LWW explicit + documented |
| No test utilities | First-class TestSession API |


-----------------

Vango V2.1 Design Report: The Unified Context & Resilience LayerVersion: 2.1 (Draft)Status: Approved for ImplementationDate: October 26, 2023Target Audience: Core Contributors, System Architects1. Executive SummaryVango V2 established the paradigm of Server-Driven UI via WebSocket patching. Vango V2.1 evolves this foundation to address the critical challenges of production deployment: Persistence, Latency, Observability, and Universality.This report specifies the design of the "Unified Context" (vango.Ctx), a reimagined central nervous system for the framework. By elevating the Context from a simple request carrier to a stateful orchestrator, Vango V2.1 bridges the gap between ephemeral server processes and the user's need for continuous, persistent experiences.Key Deliverablesctx.Async: Native support for out-of-order streaming and async UI rendering, solving the "waterfall" problem inherent in server-side rendering.ctx.Client: A capability negotiation layer enabling a single Go codebase to drive Web, iOS, and Android interfaces adaptively.ctx.Session: A robust, backend-agnostic persistence layer (Redis, SQL, NATS) ensuring session survival across server restarts and network disconnects.ctx.Sync: An offline-first data synchronization primitive for embedded mobile deployments.vango.NatsStore: A high-performance, edge-ready session store implementation using NATS JetStream.Observability Stack: A middleware-first approach to distributed tracing using OpenTelemetry.2. Core Architecture: The Unified ContextIn V2.0, vango.Ctx was primarily a wrapper around the HTTP Request and the WebSocket connection. In V2.1, it becomes the unified interface for the four pillars of modern application delivery.2.1 The Four Pillarstype Ctx interface {
    // 1. Latency Management (Streaming)
    Async(fn func() (*VNode, error), config AsyncConfig) *VNode

    // 2. Platform Adaptation (Native/Web)
    Client ClientContext

    // 3. Persistence (Session Survival)
    Session SessionContext

    // 4. Data Synchronization (Offline)
    Sync(key string, fetcher func() any) *SyncResource

    // Standard Context Compliance
    context.Context
}
This design ensures that developers have immediate access to advanced capabilities without importing disparate packages or managing complex dependency injection graphs.3. Deep Dive: Intelligent Streaming (ctx.Async)3.1 Problem StatementServer-Side Rendering (SSR) traditionally suffers from the "waterfall" problem. If a dashboard requires three database queries taking 100ms, 200ms, and 500ms respectively, the entire page is blocked for 500ms. This degrades the Time-to-First-Byte (TTFB) and perceived performance.React attempts to solve this with Suspense and client-side hydration. Vango V2.1 solves this natively via the WebSocket connection, eliminating the need for complex hydration logic.3.2 Architecturectx.Async leverages the persistent nature of the Vango connection.Immediate Render: When ctx.Async is called, it immediately returns a Fallback VNode (e.g., a Skeleton or Spinner). This node is assigned a temporary, deterministic Hydration ID (HID).Goroutine Dispatch: The framework spawns a managed Goroutine to execute the provided data-fetching closure.Out-of-Order Patching: Upon completion of the closure:The resulting VNode is rendered to a virtual DOM tree.A REPLACE_NODE binary patch is generated targeting the temporary HID.The patch is pushed down the WebSocket.3.3 API Specification// AsyncConfig controls the behavior of the async operation
type AsyncConfig struct {
    Fallback *VNode        // Rendered immediately
    Timeout  time.Duration // Max time to wait before error
    OnError  func(error) *VNode
}

// Usage Example
func Dashboard(ctx vango.Ctx) *vango.VNode {
    return Div(
        Class("dashboard-grid"),
        
        // Fast static content
        Sidebar(),

        // Slow dynamic content
        ctx.Async(
            func() (*vango.VNode, error) {
                // This blocks ONLY this goroutine, not the page render
                data := db.HeavyAnalyticsQuery() 
                return Chart(data), nil
            },
            vango.AsyncConfig{
                Fallback: SkeletonCard(),
                Timeout:  5 * time.Second,
                OnError: func(err error) *vango.VNode {
                    return ErrorAlert("Failed to load analytics", err)
                },
            },
        ),
    )
}
3.4 Implementation GuidanceConcurrency Safety: The Session object is not thread-safe. The async Goroutine must not mutate the Session's CurrentTree directly. Instead, it must send the result back to the Session's main event loop via a buffered channel (asyncResults chan AsyncResult).Panic Recovery: Every async Goroutine must be wrapped in a defer/recover block. A panic in a widget must not crash the entire server. It should be caught and rendered as the OnError state.Context Propagation: The ctx passed to the closure must be a child context of the request context, ensuring that if the user navigates away (canceling the parent context), the heavy database query is also canceled via context.Done().4. Deep Dive: Universal Adaptation (ctx.Client)4.1 Problem StatementMaintaining separate codebases for Web, iOS, and Android is prohibitively expensive. "React Native" unifies the logic but bifurcates the rendering. Vango V2.1's "Polyglot Edge" strategy unifies both by treating the native app as a generic "Player" that interprets high-level instructions.4.2 ArchitectureThe ctx.Client exposes a capability registry populated during the initial WebSocket handshake.The Player: A thin native shell (Swift/Kotlin) that includes standard UI components and a set of "Native Islands" (Camera, Map, Haptics).The Handshake: When connecting, the Player sends a bitmask or list of available capabilities (e.g., CAP_CAMERA, CAP_BIOMETRICS, CAP_AR).The Branching: The server-side component checks these capabilities to decide whether to render a standard HTML <input type="file"> or a Native instruction INSERT_NODE { type: "NATIVE_BUTTON", action: "OPEN_CAMERA" }.4.3 API Specificationtype ClientContext interface {
    // Has checks if the connected client supports a specific capability
    Has(capability ClientCapability) bool

    // Platform returns "web", "ios", "android", or "terminal"
    Platform() string

    // SendNative sends a direct command to the native bridge
    SendNative(command string, payload any)
}

// Usage Example
func UploadControl(ctx vango.Ctx) *vango.VNode {
    // Imperative branching - "It's just Go"
    if ctx.Client.Has(vango.CapCamera) {
        return NativeButton(
            Label("Take Photo"),
            OnTap(func() { 
                ctx.Client.SendNative("OPEN_CAMERA", nil) 
            }),
        )
    }
    
    // Web Fallback
    return Input(Type("file"), Accept("image/*"))
}
4.4 App Store Compliance StrategyTo strictly adhere to Apple Guideline 2.5.2 (which prohibits downloading executable code), Vango V2.1 strictly separates Data from Logic.Logic (Go): Runs entirely on the server (Cloud Mode) or in a sandboxed background thread (Embedded Mode). It is never "downloaded" to the main executable memory space.Data (View Tree): The VNode tree sent over the wire is purely data—a description of the UI state. It contains no executable logic, only references (HIDs) to server-side handlers.This architecture mirrors standard web browsers (which download HTML/CSS) and existing compliant apps like Figma or Notion.5. Deep Dive: Resilience & Persistence (ctx.Session)5.1 Problem StatementIn V2.0, session state lived exclusively in server RAM. A server deployment or crash resulted in all users losing their state (form drafts, UI toggles). V2.1 introduces the SessionStore interface to decouple state from the process lifecycle.5.2 ArchitectureThe core concept is Session Serialization. When a user disconnects, or periodically during interaction, the session's Signal graph is serialized to JSON and flushed to a durable store.The Boundary: Not all state is serializable. Channels, functions, and mutexes cannot be saved.The Transient Marker: Signals containing non-serializable data must be marked as Transient. The serializer will skip these signals during the save process.5.3 API Specificationtype SessionContext interface {
    // Get retrieves a value, populating 'dest' if found. 
    // If not found, 'dest' remains at default.
    Get(key string, dest any) error

    // Set saves a value to the session store.
    Set(key string, value any) error

    // ID returns the persistent session identifier
    ID() string
}

// Signal Integration
// Signals can be automatically backed by the session
var Draft = vango.Signal(Form{}).Persist("draft_id")
5.4 The SessionStore Interfacetype SessionStore interface {
    // Save writes the serialized session blob
    Save(sessionID string, data []byte) error

    // Load retrieves the blob
    Load(sessionID string) ([]byte, error)

    // Delete removes the session (logout/expiry)
    Delete(sessionID string) error

    // SaveAll is called during graceful shutdown
    SaveAll(sessions map[string][]byte) error
}
5.5 Implementation Guidance: VersioningSerialized data effectively becomes a database schema. If the struct definition of a Signal changes between deployments, deserialization will fail.Strategy: The SerializableSession struct must include a SchemaVersion.On Load(), check data.Version.If data.Version < CurrentVersion, discard the session (graceful reset) or attempt migration if critical.V2.1 will default to "Discard on Mismatch" to prevent runtime panics.6. Feature Focus: NATS JetStream Persistence (vango.NatsStore)6.1 Why NATS?While Redis is the standard for session storage, NATS JetStream offers distinct advantages for Vango's architecture:Go Synergy: NATS is written in Go and can be embedded directly into the Vango binary for single-binary deployments that still support clustering.Key-Value Buckets: JetStream's KV layer provides immediate consistency and watchability, simpler than managing Redis Keyspace Notifications.Edge Replication: NATS Leaf Nodes allow session state to be replicated to edge locations seamlessly.6.2 Implementation Sketchpackage vango

import (
    "[github.com/nats-io/nats.go](https://github.com/nats-io/nats.go)"
    "[github.com/nats-io/nats.go/jetstream](https://github.com/nats-io/nats.go/jetstream)"
)

type NatsStore struct {
    kv jetstream.KeyValue
}

func NewNatsStore(nc *nats.Conn, bucketName string) (*NatsStore, error) {
    js, _ := jetstream.New(nc)
    
    // Create or bind to the KV bucket
    kv, err := js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{
        Bucket:      bucketName,
        Description: "Vango Session Storage",
        TTL:         24 * time.Hour, 
        History:     1, // We only need the latest state
    })
    
    return &NatsStore{kv: kv}, err
}

func (s *NatsStore) Save(sessionID string, data []byte) error {
    _, err := s.kv.Put(context.Background(), sessionID, data)
    return err
}

func (s *NatsStore) Load(sessionID string) ([]byte, error) {
    entry, err := s.kv.Get(context.Background(), sessionID)
    if err == jetstream.ErrKeyNotFound {
        return nil, nil // Valid miss
    }
    if err != nil {
        return nil, err
    }
    return entry.Value(), nil
}

func (s *NatsStore) Delete(sessionID string) error {
    return s.kv.Delete(context.Background(), sessionID)
}

func (s *NatsStore) SaveAll(sessions map[string][]byte) error {
    // NATS handles concurrency well, but we can pipeline this if needed
    for id, data := range sessions {
        if err := s.Save(id, data); err != nil {
            return err
        }
    }
    return nil
}
7. Deep Dive: Offline-First Data (ctx.Sync)7.1 Problem StatementVango Mobile apps ("Embedded Mode") run the Go engine on the device. They need access to data even when the device is offline (e.g., in an airplane). Relying solely on http.Get fails here.7.2 Architecturectx.Sync is an abstraction layer that behaves differently based on the deployment mode:Cloud Mode: It acts as a transparent pass-through to the database or API.Embedded Mode: It binds to a local SQLite database. It includes a background "Syncer" that:Push: Queues local mutations (CREATE/UPDATE/DELETE) and replays them to the cloud API when connectivity is restored.Pull: Periodically fetches changes from the cloud and updates the local SQLite replica.7.3 API Specificationtype SyncResource[T any] struct {
    // Data access
    Items() []T
    
    // Mutation
    Add(item T)
    Remove(id string)
}

// Usage
func TodoList(ctx vango.Ctx) *vango.VNode {
    // Declarative data dependency
    todos := vango.Sync(ctx, "todos", db.ListTodos)

    return Div(
        Range(todos.Items(), func(t Todo, i int) *vango.VNode {
            return TodoItem(t)
        }),
    )
}
7.4 Conflict ResolutionV2.1 implements Last-Write-Wins (LWW) based on timestamps. While rudimentary compared to CRDTs, LWW is sufficient for 95% of CRUD applications and drastically reduces implementation complexity. Advanced users can inject custom conflict resolution logic via the SyncConfig.8. Deep Dive: Observability (Middleware & Tracing)8.1 StrategyWe reject the idea of ctx.Trace. Observability should be infrastructure, not application logic. Vango V2.1 adopts a "Middleware-First" approach using OpenTelemetry (OTel).8.2 ArchitectureThe Vango session loop is wrapped in a Tracer. Every event (Click, Submit, Mount) starts a new Span.// Middleware Registration
app.Use(otel.Middleware("vango-service", otel.Config{
    SampleRate: 1.0, // 100% in dev
}))
8.3 Context PropagationThe critical requirement is ensuring the trace_id propagates from the WebSocket event to the database driver.ctx.StdContext() returns a standard context.Context that carries the OTel span.Developers must pass this context to DB calls.func SaveHandler(ctx vango.Ctx) {
    // ctx.StdContext() has the Trace ID
    // DB driver (pgx/sql) extracts it and adds it to the query headers
    user, err := db.Users.Get(ctx.StdContext(), id)
}
9. Security & Scaling Considerations9.1 Memory Protection (The "10,000 Tabs" Attack)A malicious actor could open 10,000 tabs, disconnect them, and force the server to hold 10,000 "Detached" sessions in memory, causing an OOM (Out of Memory) crash.Defense: MaxDetachedSessions + LRU.Vango V2.1 introduces a bounded LRU cache for detached sessions.Config: MaxDetachedSessions: 1000.Behavior: When the 1001st session disconnects, the least recently used detached session is serialized to the SessionStore (Redis/NATS) and evicted from RAM.Impact: RAM usage remains bounded regardless of client behavior.9.2 Protocol SecurityThe binary protocol parser must be hardened against fuzzing.Allocation Limits: Strict caps on string/byte slice allocation (e.g., max 4MB).Depth Limits: Max VNode tree depth to prevent stack overflow attacks.10. ConclusionThe Vango V2.1 specification transforms the framework from a promising prototype into a production-grade platform.By introducing Session Serialization and NATS Persistence, we solve the deployment and scaling story.By introducing ctx.Async and ctx.Client, we solve the UX and Cross-Platform story.By adopting Standard Context Tracing, we solve the Day-2 Operations story.The design phase is essentially complete. The focus now shifts entirely to implementation, starting with the SessionStore interface, as it is the foundation upon which the other features rely.


# Vango UI

# VangoUI: Component Library Specification

**Version:** 1.0  
**Status:** Final  
**Last Updated:** December 2024

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Architectural Principles](#2-architectural-principles)
3. [Distribution Model](#3-distribution-model)
4. [Component API Design](#4-component-api-design)
5. [The Client Hook Protocol](#5-the-client-hook-protocol)
6. [Styling System](#6-styling-system)
7. [Theming](#7-theming)
8. [Component Categories](#8-component-categories)
9. [Implementation Patterns](#9-implementation-patterns)
10. [Standard Hooks Library](#10-standard-hooks-library)
11. [Hook Loading Strategy](#11-hook-loading-strategy)
12. [Security Model](#12-security-model)
13. [Testing Strategy](#13-testing-strategy)
14. [Component Reference](#14-component-reference)
15. [Migration & Versioning](#15-migration--versioning)

---

## 1. Executive Summary

VangoUI is a component library designed specifically for Vango's server-driven architecture. It provides production-ready UI components that leverage Vango's unique strengths: server-side rendering, direct database access, and minimal client-side JavaScript.

### Core Principles

| Principle | Implementation |
|-----------|----------------|
| **Code Ownership** | CLI copies source files into your project. You own and modify them. |
| **Type Safety** | Functional options with compile-time checking. No stringly-typed props. |
| **Server-First** | Business logic runs on the server. Client handles only interaction physics. |
| **Zero Bloat** | Primitive components add 0KB to bundle. Hooks load on-demand. |
| **AI-Optimized DX** | Fully discoverable API via language server. Self-documenting types. |

### The Logic/Physics Split

```
┌─────────────────────────────────────────────────────────────────┐
│                         SERVER (Go)                              │
│  • State management (Signals)                                    │
│  • Data fetching (direct DB queries)                             │
│  • Business logic (validation, permissions)                      │
│  • HTML rendering                                                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ WebSocket / SSE
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      CLIENT (Thin Client)                        │
│  • DOM patching                                                  │
│  • Event delegation                                              │
│  • Hook execution (focus traps, drag-drop, positioning)          │
│  • 60fps animations                                              │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Architectural Principles

### 2.1 Server-Driven Components

Every VangoUI component is a Go function that returns a `*vango.VNode`. There are no "client components" in the React sense. The server controls all state; the client executes physics.

```go
// This entire component runs on the server
func UserCard(user *User) *vango.VNode {
    return ui.Card(
        ui.CardHeader(
            ui.Avatar(ui.Src(user.AvatarURL)),  // Direct DB data
            ui.Title(user.Name),
        ),
        ui.CardContent(
            ui.Text(user.Bio),
        ),
        ui.CardFooter(
            ui.Button(
                ui.Primary,
                ui.OnClick(func() { followUser(user.ID) }),  // Server function
                ui.Children(ui.Text("Follow")),
            ),
        ),
    )
}
```

### 2.2 No API Glue

Traditional SPAs require:
1. Server endpoint (`GET /api/users/:id`)
2. Client fetch logic
3. Loading states
4. Error handling
5. Data transformation

VangoUI components query data directly:

```go
func UserProfile(ctx vango.Context) *vango.VNode {
    user := db.Users.Find(ctx.Param("id"))  // Direct query
    posts := db.Posts.Where("user_id = ?", user.ID).Limit(10)
    
    return ui.Card(
        ui.CardContent(
            ui.Text(user.Bio),
            Map(posts, PostCard),  // Render directly
        ),
    )
}
```

### 2.3 Compound Components Without Providers

React compound components require Context Providers for state sharing:

```jsx
// React: Provider hell
<Accordion.Root>
  <Accordion.Item>
    <Accordion.Trigger />
    <Accordion.Content />
  </Accordion.Item>
</Accordion.Root>
```

VangoUI passes signals directly—they're just function arguments:

```go
// Vango: Just functions
func Accordion(items []AccordionItemData) *vango.VNode {
    openIndex := vango.Signal(-1)  // Shared state
    
    return Div(
        Class("space-y-2"),
        Map(items, func(item AccordionItemData, i int) *vango.VNode {
            return AccordionItem(item, openIndex, i)  // Pass signal directly
        }),
    )
}

func AccordionItem(item AccordionItemData, openIndex *vango.Signal[int], index int) *vango.VNode {
    isOpen := openIndex.Get() == index
    
    return Div(
        Button(
            ui.Ghost,
            ui.OnClick(func() { openIndex.Set(index) }),
            ui.Children(ui.Text(item.Title)),
        ),
        If(isOpen, 
            Div(Class("p-4"), Text(item.Content)),
        ),
    )
}
```

---

## 3. Distribution Model

### 3.1 CLI-First Distribution

VangoUI is not imported as a Go module. Components are copied into your project.

```bash
# Initialize VangoUI in your project
vango add init

# Add specific components
vango add button card dialog

# Add multiple components
vango add button input label textarea select checkbox radio
```

### 3.2 Project Structure

After initialization:

```
app/
├── components/
│   └── ui/
│       ├── utils.go        # CN utility, shared helpers
│       ├── button.go       # You own this
│       ├── card.go         # You own this
│       ├── dialog.go       # You own this
│       └── ...
├── routes/
└── ...
```

### 3.3 The `vango add init` Command

Creates the foundation:

```go
// app/components/ui/utils.go

package ui

import "github.com/vango-dev/vango"

// CN merges Tailwind classes using a robust merge library (e.g. tailwind-merge-go).
func CN(classes ...string) string {
    return tailwind.Merge(classes...) 
}

// Common type aliases
type VNode = vango.VNode
type Signal[T any] = vango.Signal[T]
```

Configures Tailwind:

```javascript
// tailwind.config.js (created/updated)
module.exports = {
  content: ["./app/**/*.go"],
  theme: {
    extend: {
      colors: {
        border: "hsl(var(--border))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        // ... full color system
      },
    },
  },
}
```

Creates base CSS:

```css
/* app/static/globals.css */
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 0 0% 3.9%;
    --primary: 0 0% 9%;
    --primary-foreground: 0 0% 98%;
    /* ... full variable set */
  }

  }
}
```

Configures VS Code:
To ensure a smooth developer experience, `vango add init` generates a `.vscode/settings.json` file to enable Tailwind IntelliSense within Go files.

```json
{
  "tailwindCSS.experimental.classRegex": [
    ["Class\\(\"([^\"]*)\"\\)", "\"([^\"]*)\""],
    ["CN\\(([^)]*)\\)", "\"([^\"]*)\""]
  ],
  "tailwindCSS.includeLanguages": {
    "go": "html"
  }
}
```
```

### 3.4 Component Registry

The CLI fetches from a registry manifest:

```json
{
  "version": "1.0.0",
  "components": {
    "button": {
      "files": ["button.go"],
      "dependencies": [],
      "hooks": []
    },
    "skeleton": {
      "files": ["skeleton.go"],
      "dependencies": [],
      "hooks": []
    },
    "dialog": {
      "files": ["dialog.go"],
      "dependencies": ["button"],
      "hooks": ["Dialog"]
    },
    "combobox": {
      "files": ["combobox.go"],
      "dependencies": ["input", "popover"],
      "hooks": ["Combobox", "Popover"]
    }
  }
}
```

When you run `vango add dialog`:
1. Fetches `dialog.go` from registry
2. Checks for `button` dependency, prompts if missing
3. Ensures `Dialog` hook is available in thin client

### 3.5 Icon Strategy

VangoUI does not abstract icons behind a string map (e.g. `ui.Icon("menu")`). This prevents binary bloat and hidden dependencies.

**The Strategy:**
- Use a dedicated icon package (e.g., `github.com/lucide-icons/lucide-go`).
- Pass icon components directly as children.
- Component generators (`vango add`) will import the chosen icon library.

```go
// Good: Type-safe, tree-shakable
ui.Button(
    ui.Children(lucide.Trash),
)
```

---

## 4. Component API Design

### 4.1 Functional Options Pattern

Every component accepts typed options:

```go
func Button(opts ...ButtonOption) *vango.VNode
```

This provides:
- **Compile-time safety**: Invalid options don't compile
- **Discoverability**: Language server shows all valid options
- **Flexibility**: Options can be combined in any order

### 4.2 Option Type Definition

Each component defines its own option interface:

```go
// button.go

// ButtonOption is any option that can be applied to a Button
type ButtonOption interface {
    applyButton(*buttonConfig)
}

// Internal config struct
type buttonConfig struct {
    variant   string
    size      string
    disabled  bool
    class     string
    onClick   func()
    children  []*vango.VNode
    attrs     map[string]string
}

// Default configuration
func defaultButtonConfig() *buttonConfig {
    return &buttonConfig{
        variant: "default",
        size:    "md",
    }
}
```

### 4.3 Option Implementations

**Variant Options:**

```go
type buttonVariant string

func (v buttonVariant) applyButton(c *buttonConfig) {
    c.variant = string(v)
}

var (
    Primary     ButtonOption = buttonVariant("primary")
    Secondary   ButtonOption = buttonVariant("secondary")
    Destructive ButtonOption = buttonVariant("destructive")
    Outline     ButtonOption = buttonVariant("outline")
    Ghost       ButtonOption = buttonVariant("ghost")
    Link        ButtonOption = buttonVariant("link")
)
```

**Size Options:**

```go
type buttonSize string

func (s buttonSize) applyButton(c *buttonConfig) {
    c.size = string(s)
}

var (
    SizeSm ButtonOption = buttonSize("sm")
    SizeMd ButtonOption = buttonSize("md")
    SizeLg ButtonOption = buttonSize("lg")
    SizeIcon ButtonOption = buttonSize("icon")
)
```

**Behavior Options:**

```go
type buttonDisabled bool

func (d buttonDisabled) applyButton(c *buttonConfig) {
    c.disabled = bool(d)
}

func Disabled(v bool) ButtonOption {
    return buttonDisabled(v)
}
// Note: Be careful of naming collisions with `el.Disabled` (attribute).
// Users importing both packages should use named imports or selector syntax.

type buttonOnClick func()

func (f buttonOnClick) applyButton(c *buttonConfig) {
    c.onClick = f
}

func OnClick(f func()) ButtonOption {
    return buttonOnClick(f)
}
```

**Children Option:**

```go
type buttonChildren []*vango.VNode

func (ch buttonChildren) applyButton(c *buttonConfig) {
    c.children = ch
}

func Children(nodes ...*vango.VNode) ButtonOption {
    return buttonChildren(nodes)
}
```

**Class Override:**

```go
type buttonClass string

func (cl buttonClass) applyButton(c *buttonConfig) {
    c.class = string(cl)
}

func Class(s string) ButtonOption {
    return buttonClass(s)
}
```

### 4.4 Component Implementation

```go
func Button(opts ...ButtonOption) *vango.VNode {
    cfg := defaultButtonConfig()
    
    for _, opt := range opts {
        opt.applyButton(cfg)
    }

    // Optional: Runtime Safety
    if len(cfg.children) == 0 {
         panic("ui.Button: requires at least one child")
    }
    
    // Base classes
    base := "inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
    
    // Variant classes
    variants := map[string]string{
        "primary":     "bg-primary text-primary-foreground shadow hover:bg-primary/90",
        "secondary":   "bg-secondary text-secondary-foreground shadow-sm hover:bg-secondary/80",
        "destructive": "bg-destructive text-destructive-foreground shadow-sm hover:bg-destructive/90",
        "outline":     "border border-input bg-background shadow-sm hover:bg-accent hover:text-accent-foreground",
        "ghost":       "hover:bg-accent hover:text-accent-foreground",
        "link":        "text-primary underline-offset-4 hover:underline",
    }
    
    // Size classes
    sizes := map[string]string{
        "sm":   "h-8 px-3 text-xs",
        "md":   "h-9 px-4 py-2",
        "lg":   "h-10 px-8",
        "icon": "h-9 w-9",
    }
    
    classes := CN(base, variants[cfg.variant], sizes[cfg.size], cfg.class)
    
    return vango.Button(
        vango.Class(classes),
        vango.Disabled(cfg.disabled),
        vango.OnClick(cfg.onClick),
        vango.Fragment(cfg.children...),
    )
}
```

### 4.5 Usage Examples

```go
// Simple button
ui.Button(ui.Primary, ui.Children(ui.Text("Save")))

// With all options
ui.Button(
    ui.Destructive,
    ui.SizeLg,
    ui.Disabled(isSubmitting),
    ui.OnClick(handleDelete),
    ui.Class("w-full"),  // Override: full width
    ui.Children(
        icons.Trash, // from lucide-go or similar
        ui.Text("Delete Account"),
    ),
)

// Icon button
ui.Button(
    ui.Ghost,
    ui.SizeIcon,
    ui.OnClick(toggleSidebar),
    ui.Children(icons.Menu),
)
```

### 4.6 Shared Options

To avoid modifying `utils.go` every time a component is added, we use the "Shared Type, Local Method" pattern. `utils.go` defines the type, but the component file defines the method that satisfies the interface.

**`utils.go` (Static):**
Defines the type and the constructor. This file is rarely modified.

```go
type ClassOption string

func Class(s string) ClassOption {
    return ClassOption(s)
}

// Other shared types like Disabled, OnClick, etc.
```

**`button.go` (Component):**
Implements the adapter method locally. This allows `vango add button` to be purely additive (no AST manipulation of `utils.go`).

```go
// Makes ClassOption satisfy ButtonOption
func (c ClassOption) applyButton(cfg *buttonConfig) { 
    cfg.class = string(c) 
}
```

### 4.7 Form Patterns

Forms require special handling to ensure `e.PreventDefault()` runs on the client and values are serialized correctly to the server without a page reload.

**Implementation:**

Forms in VangoUI rely on the Thin Client's native event delegation. No specific "Form Hook" is required for standard submission.

```go
// ui/form.go
func Form(opts ...FormOption) *vango.VNode {
    // 1. Render a standard HTML form
    // 2. The Thin Client automatically intercepts 'submit'
    // 3. Serializes FormData and sends to server via WebSocket
    return HtmlForm(
        // No Hook("Form") needed
        OnEvent("submit", handleFormSubmit),
        // ... apply other options
    )
}
```

This ensures the server receives a clean map of values while maintaining the SPA feel.
```

---

## 5. The Client Hook Protocol

### 5.1 Overview

Hooks provide 60fps client-side interactions while keeping state on the server. They handle the "physics" of UI: focus traps, drag-and-drop, positioning, animations.

### 5.2 Wire Format

Single attribute, JSON object. Keys are hook names, values are configurations.

```html
<div 
  id="task-123"
  data-v-hooks='{
    "Sortable": { "group": "tasks", "handle": ".drag" },
    "Tooltip": { "text": "Drag to reorder" }
  }'
>
  <span class="drag">::</span> Task Name
</div>
```

### 5.3 JavaScript Contract

```typescript
type HookDefinition = {
    // REQUIRED: Called when element enters DOM
    // Must return an instance object for state preservation
    mounted: (
        el: HTMLElement,
        config: Config,
        pushEvent: PushEventFn
    ) => Instance | Promise<Instance>;

    // OPTIONAL: Called when server sends new config
    // Receives oldConfig for smart diffing
    // If undefined, hook is destroyed and re-mounted on config change
    updated?: (
        el: HTMLElement,
        newConfig: Config,
        oldConfig: Config,
        instance: Instance
    ) => void;

    // OPTIONAL: Called when element is removed from DOM
    destroyed?: (
        el: HTMLElement,
        instance: Instance
    ) => void;
}

type Config = Record<string, any>;
type Instance = any;
type PushEventFn = (eventName: string, payload: Record<string, any>) => void;
```

### 5.4 Go API

```go
// Attach a hook to an element
Hook(name string, config map[string]any)

// Listen for events from hooks
OnEvent(name string, handler func(HookEvent))

// HookEvent interface
type HookEvent interface {
    // Typed accessors
    String(key string) string
    Int(key string) int
    Float(key string) float64
    Bool(key string) bool
    Strings(key string) []string
    Raw(key string) any  // For complex nested data
    
    // Flow control
    Revert()  // Re-render from server state
}
```

### 5.5 Complete Hook Example

**JavaScript Definition:**

```javascript
// public/js/hooks.js
export default {
    Sortable: {
        mounted(el, config, pushEvent) {
            const sortable = new Sortable(el, {
                group: config.group,
                handle: config.handle,
                animation: 150,
                onEnd: (evt) => {
                    pushEvent("reorder", {
                        itemId: evt.item.dataset.id,
                        fromIndex: evt.oldIndex,
                        toIndex: evt.newIndex,
                    });
                },
            });
            
            return { sortable };
        },
        
        updated(el, newConfig, oldConfig, { sortable }) {
            if (newConfig.disabled !== oldConfig.disabled) {
                sortable.option("disabled", newConfig.disabled);
            }
        },
        
        destroyed(el, { sortable }) {
            sortable.destroy();
        },
    },
}
```

**Go Component:**

```go
func SortableList(items []Item) *vango.VNode {
    return Ul(
        Class("space-y-2"),
        Hook("Sortable", map[string]any{
            "group":  "items",
            "handle": ".handle",
        }),
        OnEvent("reorder", func(e vango.HookEvent) {
            itemId := e.String("itemId")
            from := e.Int("fromIndex")
            to := e.Int("toIndex")
            
            if err := db.Items.Reorder(itemId, from, to); err != nil {
                e.Revert()  // Snap back to server state
                toast.Error("Failed to reorder")
                return
            }
        }),
        Map(items, func(item Item) *vango.VNode {
            return Li(
                Data("id", item.ID),
                Span(Class("handle cursor-grab"), Text("::")),
                Text(item.Name),
            )
        }),
    )
}
```

### 5.6 The Revert Protocol

`Revert()` is not a client-side undo. It triggers a server re-render.

**Flow:**

```
1. User drags item A to position B
2. Hook moves DOM element (instant visual feedback)
3. Hook calls pushEvent("reorder", {from: 0, to: 2})
4. Server handler runs db.Items.Reorder(...)
5. Database operation fails
6. Handler calls e.Revert()
7. Server re-renders component from current DB state (A still at position 0)
8. Diff engine generates PATCH
9. Client applies patch, DOM snaps back
```

**Error Feedback:**

`Revert()` handles physics. UI feedback is separate:

```go
OnEvent("reorder", func(e vango.HookEvent) {
    if err := db.Items.Reorder(...); err != nil {
        e.Revert()                          // 1. Fix the physics
        toast.Error(ctx, "Permission denied") // 2. Inform the user
    }
})
```

### 5.7 Error Handling

**Load Failure:**

If a hook name in `data-v-hooks` is not found in the registry:
- Thin Client logs `console.warn("Vango: Unknown hook 'HookName'")`
- Element renders as static DOM (progressive enhancement)
- No error thrown

**Async Race Condition:**

```javascript
// Thin Client implementation
async mountHook(name, el, config) {
    const hook = registry[name];
    if (!hook) return;

    try {
        const instance = await hook.mounted(el, config, this.pushEvent);
        
        // Element might be gone by now
        if (!document.body.contains(el)) {
            hook.destroyed?.(el, instance);
            return;
        }
        
        storeInstance(el, name, instance);
    } catch (err) {
        console.error(`Vango: Hook '${name}' failed to mount`, err);
    }
}
```

### 5.8 Multi-Hook Support

Elements can have multiple hooks:

```go
Div(
    Hook("Sortable", map[string]any{"group": "tasks"}),
    Hook("Tooltip", map[string]any{"text": "Drag me"}),
    // ...
)
```

Rendered as:

```html
<div data-v-hooks='{"Sortable":{"group":"tasks"},"Tooltip":{"text":"Drag me"}}'>
```

The Thin Client manages each independently:

```javascript
// el._vInstances = {
//     "Sortable": { config: {...}, state: {...} },
//     "Tooltip": { config: {...}, state: {...} }
// }
```

### 5.9 Inter-Hook Communication

**Decision:** No client-side event bus for MVP.

**Reasoning:**
1. Server coordination is primary. When a Dropdown opens, the server knows and can close others.
2. DOM events suffice for edge cases. Hooks can use `el.dispatchEvent(new CustomEvent(...))`.

---

## 6. Styling System

### 6.1 Tailwind CSS Foundation

VangoUI uses Tailwind CSS for all styling. No CSS-in-JS, no styled-components.

**Why Tailwind:**
- Works with Go templates (class strings)
- Generates minimal CSS (only used classes)
- IDE support (Tailwind IntelliSense)
- Industry standard, well-documented

### 6.2 The CN Utility

The `CN` function merges Tailwind classes intelligently:

```go
// Later classes override earlier ones for conflicting utilities
CN("p-2", "p-4")           // → "p-4"
CN("bg-red-500", "bg-blue-500") // → "bg-blue-500"
CN("p-2 m-4", "p-8")       // → "m-4 p-8"
```

**Implementation:**

Tailwind conflict resolution is mathematically complex (e.g., `p-4` vs `px-2`). 

**Recommendation:** Do not write this from scratch. Use an existing Go port of `tailwind-merge` or bundle a tiny JS function in the thin client to handle class merging at hydration time. The `vango add init` command will set this up using a recommended library or utility.

### 6.3 Class Overrides

Every component accepts a `Class()` option:

```go
// Default padding is p-4
ui.Card(
    ui.Class("p-8"),  // Override to p-8
    ui.CardContent(...),
)

// Add custom classes alongside defaults
ui.Button(
    ui.Primary,
    ui.Class("w-full shadow-lg"),  // Extends button styles
    ui.Children(ui.Text("Submit")),
)
```

---

## 7. Theming

### 7.1 CSS Variables

Themes are defined via CSS custom properties:

```css
:root {
    /* Base */
    --background: 0 0% 100%;
    --foreground: 0 0% 3.9%;
    
    /* Primary */
    --primary: 0 0% 9%;
    --primary-foreground: 0 0% 98%;
    
    /* Secondary */
    --secondary: 0 0% 96.1%;
    --secondary-foreground: 0 0% 9%;
    
    /* Destructive */
    --destructive: 0 84.2% 60.2%;
    --destructive-foreground: 0 0% 98%;
    
    /* Muted */
    --muted: 0 0% 96.1%;
    --muted-foreground: 0 0% 45.1%;
    
    /* Accent */
    --accent: 0 0% 96.1%;
    --accent-foreground: 0 0% 9%;
    
    /* Card */
    --card: 0 0% 100%;
    --card-foreground: 0 0% 3.9%;
    
    /* Border & Input */
    --border: 0 0% 89.8%;
    --input: 0 0% 89.8%;
    --ring: 0 0% 3.9%;
    
    /* Radius */
    --radius: 0.5rem;
}

.dark {
    --background: 0 0% 3.9%;
    --foreground: 0 0% 98%;
    --primary: 0 0% 98%;
    --primary-foreground: 0 0% 9%;
    /* ... inverted values */
}
```

### 7.2 Server-Controlled Theme

Theme is a class on `<html>`, controlled by server state:

```go
func RootLayout(children ...*vango.VNode) *vango.VNode {
    theme := getThemeFromCookie(ctx)  // "light" or "dark"
    
    return Html(
        Class(theme),
        Head(...),
        Body(
            vango.Fragment(children...),
        ),
    )
}
```

### 7.3 Theme Toggle

```go
func ThemeToggle() *vango.VNode {
    theme := getThemeFromCookie(ctx)
    
    return ui.Button(
        ui.Ghost,
        ui.SizeIcon,
        ui.OnClick(func() {
            newTheme := "dark"
            if theme == "dark" {
                newTheme = "light"
            }
            ctx.SetCookie("theme", newTheme)
            // Re-render triggers from cookie change
        }),
        ui.Children(
            If(theme == "dark",
                icons.Sun,  // icons package
                icons.Moon, // icons package
            ),
        ),
    )
}
```

### 7.4 Custom Themes

Developers can define additional themes:

```css
.theme-ocean {
    --primary: 201 96% 32%;
    --primary-foreground: 0 0% 100%;
    /* ... */
}

.theme-forest {
    --primary: 142 76% 36%;
    --primary-foreground: 0 0% 100%;
    /* ... */
}
```

```go
Html(Class("theme-ocean"), ...)
```

---

## 8. Component Categories

### 8.1 Primitives (0KB JavaScript)

Pure server-rendered components. No hooks required.

| Component | Description |
|-----------|-------------|
| `Button` | Clickable button with variants |
| `Input` | Text input field |
| `Label` | Form label |
| `Textarea` | Multi-line text input |
| `Select` | Native select dropdown |
| `Checkbox` | Checkbox input |
| `Radio` | Radio button |
| `Switch` | Toggle switch (CSS-only) |
| `Card` | Container with header/content/footer |
| `Badge` | Status indicator |
| `Separator` | Visual divider |
| `Avatar` | User avatar image |
| `Skeleton` | Loading placeholder |
| `Progress` | Progress bar |
| `Alert` | Alert message box |

### 8.2 Interactive (Hooks Required)

Components that need client-side physics.

| Component | Hook | Purpose |
|-----------|------|---------|
| `Dialog` | `Dialog` | Modal with focus trap |
| `Sheet` | `Sheet` | Slide-out panel |
| `Dropdown` | `Dropdown` | Click-triggered menu |
| `Popover` | `Popover` | Positioned popup |
| `Tooltip` | `Tooltip` | Hover information |
| `Tabs` | `Tabs` | Tabbed interface |
| `Accordion` | `Collapsible` | Expandable sections |
| `Collapsible` | `Collapsible` | Single expandable |
| `Toast` | `Toast` | Notification system |
| `AlertDialog` | `Dialog` | Confirmation modal |

### 8.3 Data Components (Server-Powered)

Components that leverage direct database access.

| Component | Description |
|-----------|-------------|
| `Combobox` | Searchable select with server filtering |
| `Command` | Command palette with fuzzy search |
| `DataTable` | Sortable, filterable table |
| `Pagination` | Server-side pagination |
| `InfiniteScroll` | Load more on scroll |

### 8.4 Form Components

Integrated with server-side validation.

| Component | Description |
|-----------|-------------|
| `Form` | Form wrapper with validation |
| `FormField` | Field with label and error |
| `FormMessage` | Validation error message |

### 8.5 Layout Components

Structural components.

| Component | Description |
|-----------|-------------|
| `Sidebar` | Collapsible side navigation |
| `SidebarLayout` | Sidebar + main content |
| `Navbar` | Top navigation bar |
| `Breadcrumb` | Navigation breadcrumbs |

---

## 9. Implementation Patterns

### 9.1 Primitive Component Pattern

```go
// badge.go

type BadgeOption interface {
    applyBadge(*badgeConfig)
}

type badgeConfig struct {
    variant  string
    class    string
    children []*vango.VNode
}

// Variants
type badgeVariant string
func (v badgeVariant) applyBadge(c *badgeConfig) { c.variant = string(v) }

var (
    BadgeDefault     BadgeOption = badgeVariant("default")
    BadgeSecondary   BadgeOption = badgeVariant("secondary")
    BadgeDestructive BadgeOption = badgeVariant("destructive")
    BadgeOutline     BadgeOption = badgeVariant("outline")
)

func Badge(opts ...BadgeOption) *vango.VNode {
    cfg := &badgeConfig{variant: "default"}
    for _, opt := range opts {
        opt.applyBadge(cfg)
    }
    
    variants := map[string]string{
        "default":     "bg-primary text-primary-foreground shadow",
        "secondary":   "bg-secondary text-secondary-foreground",
        "destructive": "bg-destructive text-destructive-foreground",
        "outline":     "border border-input bg-background",
    }
    
    classes := CN(
        "inline-flex items-center rounded-md px-2.5 py-0.5 text-xs font-semibold",
        variants[cfg.variant],
        cfg.class,
    )
    
    return Span(
        Class(classes),
        Fragment(cfg.children...),
    )
}
```

### 9.2 Interactive Component Pattern

```go
// dialog.go

type DialogOption interface {
    applyDialog(*dialogConfig)
}

type dialogConfig struct {
    open        *vango.Signal[bool]
    onClose     func()
    class       string
    trigger     *vango.VNode
    content     *vango.VNode
    title       string
    description string
}

func Dialog(opts ...DialogOption) *vango.VNode {
    cfg := &dialogConfig{}
    for _, opt := range opts {
        opt.applyDialog(cfg)
    }
    
    // Create internal signal if not provided
    if cfg.open == nil {
        cfg.open = vango.Signal(false)
    }
    
    return Fragment(
        // Trigger
        Div(
            OnClick(cfg.open.SetTrue),
            cfg.trigger,
        ),
        
        // Portal content (rendered when open)
        If(cfg.open.Get(),
            // Backdrop
            Div(
                Class("fixed inset-0 z-50 bg-black/80"),
                OnClick(cfg.open.SetFalse),
            ),
            
            // Dialog
            Div(
                Class(CN(
                    "fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2",
                    "w-full max-w-lg border bg-background p-6 shadow-lg rounded-lg",
                    cfg.class,
                )),
                
                // Hook handles focus trap and escape key
                Hook("Dialog", map[string]any{}),
                OnEvent("close", func(e vango.HookEvent) {
                    cfg.open.SetFalse()
                    if cfg.onClose != nil {
                        cfg.onClose()
                    }
                }),
                
                // Header
                If(cfg.title != "",
                    Div(
                        Class("mb-4"),
                        H2(Class("text-lg font-semibold"), Text(cfg.title)),
                        If(cfg.description != "",
                            P(Class("text-sm text-muted-foreground"), Text(cfg.description)),
                        ),
                    ),
                ),
                
                // Content
                cfg.content,
                
                // Close button
                Button(
                    Class("absolute right-4 top-4"),
                    OnClick(cfg.open.SetFalse),
                    icons.X,
                ),
            ),
        ),
    )
}

// Option constructors
func DialogTrigger(node *vango.VNode) DialogOption { /* ... */ }
func DialogContent(node *vango.VNode) DialogOption { /* ... */ }
func DialogTitle(s string) DialogOption { /* ... */ }
func DialogDescription(s string) DialogOption { /* ... */ }
func DialogOpen(sig *vango.Signal[bool]) DialogOption { /* ... */ }
func DialogOnClose(f func()) DialogOption { /* ... */ }
```

### 9.3 Data Component Pattern

```go
// combobox.go

type ComboboxOption[T any] interface {
    applyCombobox(*comboboxConfig[T])
}

type comboboxConfig[T any] struct {
    selected    *vango.Signal[*T]
    search      func(query string) []T
    renderItem  func(T) *vango.VNode
    placeholder string
    class       string
}

func Combobox[T any](opts ...ComboboxOption[T]) *vango.VNode {
    cfg := &comboboxConfig[T]{}
    for _, opt := range opts {
        opt.applyCombobox(cfg)
    }
    
    query := vango.Signal("")
    open := vango.Signal(false)
    
    // Server-side search (runs on every query change)
    results := vango.Derived(func() []T {
        if query.Get() == "" {
            return nil
        }
        return cfg.search(query.Get())
    })
    
    return Div(
        Class(CN("relative", cfg.class)),
        
        // Input
        Input(
            Placeholder(cfg.placeholder),
            Value(query.Get()),
            OnInput(query.Set),
            OnFocus(open.SetTrue),
            Debounce(150),
        ),
        
        // Dropdown
        If(open.Get() && len(results.Get()) > 0,
            Div(
                Class("absolute top-full left-0 right-0 mt-1 border bg-popover rounded-md shadow-lg z-50"),
                Hook("Combobox", map[string]any{}),
                OnEvent("close", func(e vango.HookEvent) {
                    open.SetFalse()
                }),
                
                Ul(
                    Class("py-1"),
                    Map(results.Get(), func(item T) *vango.VNode {
                        return Li(
                            Class("px-3 py-2 cursor-pointer hover:bg-accent"),
                            OnClick(func() {
                                cfg.selected.Set(&item)
                                open.SetFalse()
                            }),
                            cfg.renderItem(item),
                        )
                    }),
                ),
            ),
        ),
    )
}
```

**Usage:**

```go
func UserSelector() *vango.VNode {
    selected := vango.Signal[*User](nil)
    
    return ui.Combobox(
        ui.ComboboxSelected(selected),
        ui.ComboboxSearch(func(q string) []User {
            return db.Users.Where("name ILIKE ?", "%"+q+"%").Limit(10).All()
        }),
        ui.ComboboxRenderItem(func(u User) *vango.VNode {
            return Div(
                Class("flex items-center gap-2"),
                ui.Avatar(ui.Src(u.AvatarURL), ui.SizeSm),
                Text(u.Name),
            )
        }),
        ui.ComboboxPlaceholder("Search users..."),
    )
}
```

---

## 10. Standard Hooks Library

### 10.1 Bundled Hooks

These hooks are included in the thin client's standard library:

| Hook | Size | Dependencies | Purpose |
|------|------|--------------|---------|
| `Dialog` | ~1.5KB | focus-trap | Modal focus management |
| `Popover` | ~2KB | Floating UI | Positioned popups |
| `Dropdown` | ~1KB | - | Click-outside detection |
| `Tooltip` | ~1.5KB | Floating UI | Hover positioning |
| `Collapsible` | ~0.5KB | - | Expand/collapse |
| `Tabs` | ~0.5KB | - | Tab navigation |
| `Toast` | ~1KB | - | Notification management |
| `Sortable` | ~3KB | SortableJS | Drag to reorder |
| `Resizable` | ~1KB | - | Resize handles |

### 10.2 Hook Implementations

**Dialog Hook:**

```javascript
// hooks/dialog.js
import { createFocusTrap } from 'focus-trap';

export default {
    mounted(el, config, pushEvent) {
        const trap = createFocusTrap(el, {
            escapeDeactivates: true,
            clickOutsideDeactivates: true,
            onDeactivate: () => pushEvent('close', {}),
        });
        
        trap.activate();
        
        return { trap };
    },
    
    destroyed(el, { trap }) {
        trap.deactivate();
    },
}
```

**Popover Hook:**

```javascript
// hooks/popover.js
import { computePosition, flip, offset, shift } from '@floating-ui/dom';

export default {
    mounted(el, config, pushEvent) {
        const reference = document.querySelector(config.reference);
        
        async function update() {
            const { x, y } = await computePosition(reference, el, {
                placement: config.placement || 'bottom',
                middleware: [offset(4), flip(), shift()],
            });
            
            Object.assign(el.style, {
                left: `${x}px`,
                top: `${y}px`,
            });
        }
        
        update();
        
        // Close on outside click
        function handleClick(e) {
            if (!el.contains(e.target) && !reference.contains(e.target)) {
                pushEvent('close', {});
            }
        }
        
        document.addEventListener('click', handleClick);
        
        return { 
            cleanup: () => document.removeEventListener('click', handleClick) 
        };
    },
    
    destroyed(el, { cleanup }) {
        cleanup();
    },
}
```

**Sortable Hook:**

```javascript
// hooks/sortable.js
import Sortable from 'sortablejs';

export default {
    mounted(el, config, pushEvent) {
        const sortable = Sortable.create(el, {
            group: config.group,
            handle: config.handle,
            animation: 150,
            ghostClass: 'opacity-50',
            
            onEnd(evt) {
                pushEvent('reorder', {
                    itemId: evt.item.dataset.id,
                    fromIndex: evt.oldIndex,
                    toIndex: evt.newIndex,
                    fromGroup: evt.from.dataset.group,
                    toGroup: evt.to.dataset.group,
                });
            },
        });
        
        return { sortable };
    },
    
    updated(el, newConfig, oldConfig, { sortable }) {
        if (newConfig.disabled !== oldConfig.disabled) {
            sortable.option('disabled', newConfig.disabled);
        }
    },
    
    destroyed(el, { sortable }) {
        sortable.destroy();
    },
}
```

### 10.3 Custom Hooks

Developers can create custom hooks:

```javascript
// public/js/hooks.js
export default {
    Chart: {
        mounted(el, config, pushEvent) {
            const chart = new Chart(el, {
                type: config.type,
                data: config.data,
                options: config.options,
            });
            
            el.addEventListener('click', (e) => {
                const points = chart.getElementsAtEventForMode(e, 'nearest', {}, false);
                if (points.length) {
                    pushEvent('point-click', {
                        datasetIndex: points[0].datasetIndex,
                        index: points[0].index,
                    });
                }
            });
            
            return { chart };
        },
        
        updated(el, newConfig, oldConfig, { chart }) {
            chart.data = newConfig.data;
            chart.update();
        },
        
        destroyed(el, { chart }) {
            chart.destroy();
        },
    },
}
```

Register in `vango.json`:

```json
{
    "hooks": "./public/js/hooks.js"
}
```

---

## 11. Hook Loading Strategy

### 11.1 Overview

Hooks are loaded on-demand after the thin client boots. This keeps the initial bundle small (~12KB) while allowing rich interactivity.

### 11.2 Loading Flow

```
1. Thin client loads (12KB)
2. App becomes interactive (links, server events work)
3. Thin client reads page content, finds hook references
4. Background fetch all needed hooks
5. Initialize hooks as they load
```

### 11.3 Implementation

```javascript
// Thin client hook loader

const hookCache = new Map();
const pendingLoads = new Map();

async function ensureHook(name) {
    // Already loaded
    if (hookCache.has(name)) {
        return hookCache.get(name);
    }
    
    // Already loading
    if (pendingLoads.has(name)) {
        return pendingLoads.get(name);
    }
    
    // Start loading
    const promise = import(`/_vango/hooks/${name.toLowerCase()}.js`)
        .then(module => {
            hookCache.set(name, module.default);
            pendingLoads.delete(name);
            return module.default;
        });
    
    pendingLoads.set(name, promise);
    return promise;
}

// On page load, preload all hooks mentioned in DOM
function preloadPageHooks() {
    const hookElements = document.querySelectorAll('[data-v-hooks]');
    const hookNames = new Set();
    
    hookElements.forEach(el => {
        const config = JSON.parse(el.getAttribute('data-v-hooks'));
        Object.keys(config).forEach(name => hookNames.add(name));
    });
    
    // Background fetch all
    hookNames.forEach(name => ensureHook(name));
}

// Call on DOMContentLoaded
preloadPageHooks();
```

### 11.4 Server Preload Hints

The server can emit preload hints for faster loading:

```html
<head>
    <link rel="modulepreload" href="/_vango/hooks/dialog.js">
    <link rel="modulepreload" href="/_vango/hooks/popover.js">
</head>
```

This is automatic—the server knows what hooks are on the page.

---

## 12. Security Model

### 12.1 No Public API Endpoints

Traditional SPAs expose endpoints:

```
GET  /api/users          (list all users - can be abused)
GET  /api/users/search   (search - can be enumerated)
POST /api/users/:id      (update - needs auth checks)
```

VangoUI components query data internally:

```go
func UserCombobox() *vango.VNode {
    // This never becomes a public endpoint
    results := db.Users.Search(query, 10)
    // ...
}
```

The search logic is internal. There's no URL to abuse.

### 12.2 Automatic XSS Prevention

Vango automatically escapes text content:

```go
// This is safe even if user.Name contains "<script>..."
Text(user.Name)
```

Developers must explicitly opt into raw HTML:

```go
// Only use when you trust the content
UnsafeHTML(trustedHTML)
```

### 12.3 Server-Side Validation

Form components integrate with server-side validation:

```go
func UpdateProfile(ctx vango.Context) {
    var input struct {
        Name  string `validate:"required,min=2,max=100"`
        Email string `validate:"required,email"`
    }
    
    if err := ctx.Bind(&input); err != nil {
        // Validation errors automatically populate form fields
        return
    }
    
    // Input is validated, safe to use
}
```

### 12.4 Data Exposure Control

With direct database access, developers control exactly what data reaches the client:

```go
// BAD: Exposes all user fields
func UserCard(user *User) *vango.VNode {
    return Pre(Text(fmt.Sprintf("%+v", user)))  // Leaks password hash, etc.
}

// GOOD: Only expose what's needed
func UserCard(user *User) *vango.VNode {
    return Div(
        Text(user.Name),
        Text(user.PublicBio),
        // user.PasswordHash, user.Email never sent to client
    )
}
```

---

## 13. Testing Strategy

### 13.1 Unit Testing Components

Components are functions that return VNodes. Test them directly:

```go
func TestButton(t *testing.T) {
    node := ui.Button(
        ui.Primary,
        ui.SizeLg,
        ui.Disabled(true),
        ui.Children(ui.Text("Submit")),
    )
    
    // Assert on VNode structure
    assert.Equal(t, "button", node.Tag)
    assert.Contains(t, node.Attrs["class"], "bg-primary")
    assert.Contains(t, node.Attrs["class"], "h-10")  // SizeLg
    assert.Equal(t, "true", node.Attrs["disabled"])
}
```

### 13.2 Snapshot Testing

Render to HTML and snapshot:

```go
func TestCard(t *testing.T) {
    node := ui.Card(
        ui.CardHeader(ui.Title("Test")),
        ui.CardContent(ui.Text("Content")),
    )
    
    html := vango.RenderToString(node)
    
    // Compare against saved snapshot
    snapshot.Assert(t, html)
}
```

### 13.3 Integration Testing

Test with actual signals and events:

```go
func TestDialog(t *testing.T) {
    open := vango.Signal(false)
    
    node := ui.Dialog(
        ui.DialogOpen(open),
        ui.DialogTrigger(ui.Button(ui.Children(ui.Text("Open")))),
        ui.DialogContent(ui.Text("Hello")),
    )
    
    // Initially closed
    html := vango.RenderToString(node)
    assert.NotContains(t, html, "Hello")
    
    // Open
    open.Set(true)
    html = vango.RenderToString(node)
    assert.Contains(t, html, "Hello")
}
```

### 13.4 Hook Testing

Test hooks in isolation with a mock environment:

```javascript
// hooks/dialog.test.js
import { Dialog } from './dialog.js';

test('Dialog calls pushEvent on escape', () => {
    const el = document.createElement('div');
    document.body.appendChild(el);
    
    const pushEvent = jest.fn();
    const instance = Dialog.mounted(el, {}, pushEvent);
    
    // Simulate escape key
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    
    expect(pushEvent).toHaveBeenCalledWith('close', {});
    
    Dialog.destroyed(el, instance);
});
```

---

## 14. Component Reference

### 14.1 Button

```go
ui.Button(opts ...ButtonOption) *vango.VNode

// Variants
ui.Primary      // Primary action
ui.Secondary    // Secondary action  
ui.Destructive  // Dangerous action
ui.Outline      // Bordered
ui.Ghost        // Minimal
ui.Link         // Looks like link

// Sizes
ui.SizeSm       // Small
ui.SizeMd       // Medium (default)
ui.SizeLg       // Large
ui.SizeIcon     // Square, for icons

// Behavior
ui.Disabled(bool)
ui.OnClick(func())

// Content
ui.Children(...*vango.VNode)

// Styling
ui.Class(string)
```

### 14.2 Card

```go
ui.Card(opts ...CardOption) *vango.VNode
ui.CardHeader(opts ...CardHeaderOption) *vango.VNode
ui.CardTitle(opts ...CardTitleOption) *vango.VNode
ui.CardDescription(opts ...CardDescriptionOption) *vango.VNode
ui.CardContent(opts ...CardContentOption) *vango.VNode
ui.CardFooter(opts ...CardFooterOption) *vango.VNode

// Example
ui.Card(
    ui.Class("w-96"),
    ui.Children(
        ui.CardHeader(
            ui.Children(
                ui.CardTitle(ui.Children(ui.Text("Account"))),
                ui.CardDescription(ui.Children(ui.Text("Manage settings"))),
            ),
        ),
        ui.CardContent(...),
        ui.CardFooter(...),
    ),
)
```

### 14.3 Dialog

```go
ui.Dialog(opts ...DialogOption) *vango.VNode

ui.DialogOpen(*vango.Signal[bool])  // Control open state
ui.DialogOnClose(func())            // Called when closed
ui.DialogTrigger(*vango.VNode)      // Element that opens dialog
ui.DialogContent(*vango.VNode)      // Dialog body
ui.DialogTitle(string)              // Optional title
ui.DialogDescription(string)        // Optional description

// Example
open := vango.Signal(false)

ui.Dialog(
    ui.DialogOpen(open),
    ui.DialogTitle("Confirm Delete"),
    ui.DialogDescription("This action cannot be undone."),
    ui.DialogTrigger(
        ui.Button(ui.Destructive, ui.Children(ui.Text("Delete"))),
    ),
    ui.DialogContent(
        ui.Div(
            ui.Class("flex gap-2 justify-end"),
            ui.Button(ui.Outline, ui.OnClick(open.SetFalse), ui.Children(ui.Text("Cancel"))),
            ui.Button(ui.Destructive, ui.OnClick(handleDelete), ui.Children(ui.Text("Delete"))),
        ),
    ),
)
```

### 14.4 Input

```go
ui.Input(opts ...InputOption) *vango.VNode

ui.InputType(string)        // "text", "email", "password", etc.
ui.Placeholder(string)
ui.Value(string)
ui.Disabled(bool)
ui.OnInput(func(string))
ui.OnChange(func(string))
ui.Debounce(int)            // Milliseconds

// Example
ui.Input(
    ui.InputType("email"),
    ui.Placeholder("Enter email"),
    ui.Value(email.Get()),
    ui.OnInput(email.Set),
    ui.Class("w-full"),
)
```

### 14.5 Combobox

```go
ui.Combobox[T any](opts ...ComboboxOption[T]) *vango.VNode

ui.ComboboxSelected(*vango.Signal[*T])
ui.ComboboxSearch(func(string) []T)
ui.ComboboxRenderItem(func(T) *vango.VNode)
ui.ComboboxPlaceholder(string)
ui.ComboboxEmpty(*vango.VNode)  // Shown when no results

// Example
selected := vango.Signal[*User](nil)

ui.Combobox(
    ui.ComboboxSelected(selected),
    ui.ComboboxSearch(func(q string) []User {
        return db.Users.Search(q, 10)
    }),
    ui.ComboboxRenderItem(func(u User) *vango.VNode {
        return ui.Div(
            ui.Class("flex items-center gap-2"),
            ui.Avatar(ui.Src(u.Avatar)),
            ui.Text(u.Name),
        )
    }),
    ui.ComboboxPlaceholder("Search users..."),
    ui.ComboboxEmpty(ui.Text("No users found")),
)
```

---

## 15. Migration & Versioning

### 15.1 Updating Components

Since you own the source files, updates are manual but controlled:

```bash
# See what changed
vango add button --diff

# Output:
# --- app/components/ui/button.go
# +++ registry/button.go
# @@ -45,6 +45,10 @@
# +    "loading": "opacity-50 cursor-wait",
```

Apply updates selectively or regenerate:

```bash
# Overwrite with latest (backs up existing)
vango add button --force

# Backup created at app/components/ui/button.go.bak
```

### 15.2 Component Versioning

The registry tracks versions:

```json
{
    "version": "1.2.0",
    "components": {
        "button": {
            "version": "1.1.0",
            "files": ["button.go"],
            "changelog": "Added loading state"
        }
    }
}
```

Check for updates:

```bash
vango add --check

# Output:
# button: 1.0.0 → 1.1.0 (Added loading state)
# card: up to date
# dialog: 1.0.0 → 1.2.0 (Breaking: DialogContent API changed)
```

### 15.3 Breaking Changes

Breaking changes are documented in the registry:

```json
{
    "button": {
        "version": "2.0.0",
        "breaking": true,
        "migration": "Button no longer accepts positional variant. Use ui.Primary instead of passing 'primary' as first arg."
    }
}
```

```bash
vango add button

# Warning: button 2.0.0 has breaking changes:
# Button no longer accepts positional variant. Use ui.Primary instead.
# 
# Continue? [y/N]
```

---

## Appendix A: Full CSS Variables Reference

```css
:root {
    /* Colors - HSL values without hsl() wrapper */
    --background: 0 0% 100%;
    --foreground: 0 0% 3.9%;
    
    --card: 0 0% 100%;
    --card-foreground: 0 0% 3.9%;
    
    --popover: 0 0% 100%;
    --popover-foreground: 0 0% 3.9%;
    
    --primary: 0 0% 9%;
    --primary-foreground: 0 0% 98%;
    
    --secondary: 0 0% 96.1%;
    --secondary-foreground: 0 0% 9%;
    
    --muted: 0 0% 96.1%;
    --muted-foreground: 0 0% 45.1%;
    
    --accent: 0 0% 96.1%;
    --accent-foreground: 0 0% 9%;
    
    --destructive: 0 84.2% 60.2%;
    --destructive-foreground: 0 0% 98%;
    
    --border: 0 0% 89.8%;
    --input: 0 0% 89.8%;
    --ring: 0 0% 3.9%;
    
    /* Radius */
    --radius: 0.5rem;
    
    /* Sidebar (for sidebar components) */
    --sidebar-background: 0 0% 98%;
    --sidebar-foreground: 0 0% 9%;
    --sidebar-primary: 0 0% 9%;
    --sidebar-primary-foreground: 0 0% 98%;
    --sidebar-accent: 0 0% 96.1%;
    --sidebar-accent-foreground: 0 0% 9%;
    --sidebar-border: 0 0% 89.8%;
    --sidebar-ring: 0 0% 3.9%;
}

.dark {
    --background: 0 0% 3.9%;
    --foreground: 0 0% 98%;
    
    --card: 0 0% 3.9%;
    --card-foreground: 0 0% 98%;
    
    --popover: 0 0% 3.9%;
    --popover-foreground: 0 0% 98%;
    
    --primary: 0 0% 98%;
    --primary-foreground: 0 0% 9%;
    
    --secondary: 0 0% 14.9%;
    --secondary-foreground: 0 0% 98%;
    
    --muted: 0 0% 14.9%;
    --muted-foreground: 0 0% 63.9%;
    
    --accent: 0 0% 14.9%;
    --accent-foreground: 0 0% 98%;
    
    --destructive: 0 62.8% 30.6%;
    --destructive-foreground: 0 0% 98%;
    
    --border: 0 0% 14.9%;
    --input: 0 0% 14.9%;
    --ring: 0 0% 83.1%;
    
    --sidebar-background: 0 0% 5%;
    --sidebar-foreground: 0 0% 98%;
    --sidebar-primary: 0 0% 98%;
    --sidebar-primary-foreground: 0 0% 9%;
    --sidebar-accent: 0 0% 14.9%;
    --sidebar-accent-foreground: 0 0% 98%;
    --sidebar-border: 0 0% 14.9%;
    --sidebar-ring: 0 0% 83.1%;
}
```

---

## Appendix B: Tailwind Configuration

```javascript
// tailwind.config.js
const { fontFamily } = require("tailwindcss/defaultTheme");

module.exports = {
    darkMode: ["class"],
    content: ["./app/**/*.go"],
    theme: {
        container: {
            center: true,
            padding: "2rem",
            screens: {
                "2xl": "1400px",
            },
        },
        extend: {
            colors: {
                border: "hsl(var(--border))",
                input: "hsl(var(--input))",
                ring: "hsl(var(--ring))",
                background: "hsl(var(--background))",
                foreground: "hsl(var(--foreground))",
                primary: {
                    DEFAULT: "hsl(var(--primary))",
                    foreground: "hsl(var(--primary-foreground))",
                },
                secondary: {
                    DEFAULT: "hsl(var(--secondary))",
                    foreground: "hsl(var(--secondary-foreground))",
                },
                destructive: {
                    DEFAULT: "hsl(var(--destructive))",
                    foreground: "hsl(var(--destructive-foreground))",
                },
                muted: {
                    DEFAULT: "hsl(var(--muted))",
                    foreground: "hsl(var(--muted-foreground))",
                },
                accent: {
                    DEFAULT: "hsl(var(--accent))",
                    foreground: "hsl(var(--accent-foreground))",
                },
                popover: {
                    DEFAULT: "hsl(var(--popover))",
                    foreground: "hsl(var(--popover-foreground))",
                },
                card: {
                    DEFAULT: "hsl(var(--card))",
                    foreground: "hsl(var(--card-foreground))",
                },
                sidebar: {
                    DEFAULT: "hsl(var(--sidebar-background))",
                    foreground: "hsl(var(--sidebar-foreground))",
                    primary: "hsl(var(--sidebar-primary))",
                    "primary-foreground": "hsl(var(--sidebar-primary-foreground))",
                    accent: "hsl(var(--sidebar-accent))",
                    "accent-foreground": "hsl(var(--sidebar-accent-foreground))",
                    border: "hsl(var(--sidebar-border))",
                    ring: "hsl(var(--sidebar-ring))",
                },
            },
            borderRadius: {
                lg: "var(--radius)",
                md: "calc(var(--radius) - 2px)",
                sm: "calc(var(--radius) - 4px)",
            },
            fontFamily: {
                sans: ["Inter", ...fontFamily.sans],
            },
            keyframes: {
                "accordion-down": {
                    from: { height: "0" },
                    to: { height: "var(--radix-accordion-content-height)" },
                },
                "accordion-up": {
                    from: { height: "var(--radix-accordion-content-height)" },
                    to: { height: "0" },
                },
            },
            animation: {
                "accordion-down": "accordion-down 0.2s ease-out",
                "accordion-up": "accordion-up 0.2s ease-out",
            },
        },
    },
    plugins: [require("tailwindcss-animate")],
};
```

---

## Appendix C: Implementation Checklist

### Phase 1: Foundation
- [ ] Implement `CN` utility (class merger)
- [ ] Create `utils.go` with shared types
- [ ] Set up Tailwind configuration
- [ ] Create CSS variables file
- [ ] Build CLI `vango add init` command

### Phase 2: Primitives
- [ ] Button
- [ ] Input
- [ ] Label
- [ ] Textarea
- [ ] Select
- [ ] Checkbox
- [ ] Radio
- [ ] Card (Card, CardHeader, CardContent, CardFooter)
- [ ] Badge
- [ ] Separator
- [ ] Avatar
- [ ] Skeleton

### Phase 3: Hooks
- [ ] Implement hook loader in thin client
- [ ] Dialog hook (focus-trap)
- [ ] Popover hook (Floating UI)
- [ ] Dropdown hook (click-outside)
- [ ] Tooltip hook
- [ ] Collapsible hook
- [ ] Toast hook
- [ ] Sortable hook

### Phase 4: Interactive Components
- [ ] Dialog
- [ ] Sheet
- [ ] Dropdown
- [ ] Popover
- [ ] Tooltip
- [ ] Tabs
- [ ] Accordion
- [ ] Toast

### Phase 5: Data Components
- [ ] Combobox
- [ ] Command
- [ ] DataTable

### Phase 6: CLI
- [ ] `vango add [component]` command
- [ ] Component registry
- [ ] Dependency resolution
- [ ] `--diff` flag
- [ ] `--force` flag
- [ ] `--check` flag

---

*End of Specification*


