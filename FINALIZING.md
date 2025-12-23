# Vango Pre-Release Finalization

This document tracks DX improvements, API refinements, and architectural decisions to finalize before release.

---

## 1. Reactive Primitive Naming: `Signal()` vs `NewSignal()`

### Problem

The spec (`VANGO_ARCHITECTURE_AND_GUIDE.md`) promises a clean API:

```go
count := vango.Signal(0)
doubled := vango.Memo(func() int { ... })
vango.Effect(func() vango.Cleanup { ... })
```

But the implementation uses verbose constructors:

```go
count := vango.NewSignal(0)       // Because Signal is the type name
doubled := vango.NewMemo(...)     // Because Memo is the type name
vango.CreateEffect(...)           // Because Effect is the type name
```

**Root cause:** Go doesn't allow a type and function to share the same name in a package.

### Decision

**Rename the types** to free up the short function names:

| Current Type | New Type | Constructor |
|--------------|----------|-------------|
| `Signal[T]` | `SignalValue[T]` | `Signal[T](initial) *SignalValue[T]` |
| `Memo[T]` | `MemoValue[T]` | `Memo[T](fn) *MemoValue[T]` |
| `Effect` | `EffectHandle` | `Effect(fn) *EffectHandle` |

### Rationale

1. **Semantic clarity**: Signal and Memo *hold values* - the "Value" suffix is accurate. Effect doesn't hold a value; you get a *handle* to control it (dispose, check state).

2. **Error message readability**:
   ```
   cannot use string as int in argument to (*vango.SignalValue[int]).Set
   ```
   Reads naturally as "Signal Value of int".

3. **Matches the spec**: The entire architecture guide uses `Signal()`, `Memo()`, `Effect()`.

4. **Better DX**: When coding, `vango.Signal(0)` flows naturally. `vango.NewSignal(0)` feels like boilerplate.

### Migration

- Rename types in `signal.go`, `memo.go`, `effect.go`
- Add `Signal()`, `Memo()`, `Effect()` constructor functions
- Update all internal usages
- Update all documentation

### Alternatives Considered

- **`SignalRef`/`MemoRef`/`EffectRef`**: Shorter, Vue-familiar, but "Ref" for Effect is awkward since it's not referencing a value
- **Keep `NewSignal`/`NewMemo`/`CreateEffect`**: Standard Go idiom, but doesn't match the spec and feels verbose for such a fundamental API

---

## 2. Client Storage API (Replacing `.Persist()`)

### Problem

The spec shows a `.Persist()` API on Signals:

```go
// From VANGO_ARCHITECTURE_AND_GUIDE.md
prefs := vango.Signal(Prefs{}).Persist(vango.LocalStorage, "user-prefs")
settings := vango.Signal(Settings{}).Persist(vango.Database, "settings:123")
```

**This API is fundamentally broken** in a server-driven architecture.

### Why `.Persist()` Cannot Work

**1. The API lies about timing.**

```go
// This executes on the SERVER during component setup
darkMode := vango.Signal(false).Persist(vango.LocalStorage, "dark")
```

At this moment:
- The user's localStorage is on their browser
- This code is running on your server
- There's a network between them

The API *pretends* synchronous access to client-side storage. That's not "magic" - it's deception.

**2. No context = no user.**

```go
// Wrong: Whose localStorage? Which user?
var Theme = vango.Signal("light").Persist(LocalStorage, "theme")
```

User-specific data requires knowing which user. That means you need `ctx`.

**3. Database persistence is application logic.**

Auto-saving to a database on every `.Set()` means:
- Hammering your DB on every keystroke
- No transaction boundaries
- No validation layer
- Race conditions between tabs/sessions

This is domain logic, not framework magic.

### Decision

**Remove `.Persist()` entirely.** Replace with explicit, context-aware APIs.

| Remove | Add |
|--------|-----|
| `.Persist(LocalStorage, key)` | `UseLocalStorage(ctx, key, default)` |
| `.Persist(SessionStorage, key)` | `UseSessionStorage(ctx, key, default)` |
| `.Persist(Database, key)` | Nothing - use explicit DB calls |

### The New Design: "Sync-on-Connect"

**1. Configuration (security + performance allowlist)**

```go
vango.Config{
    // Only these keys are sent during WebSocket handshake
    // Prevents malicious/bloated localStorage from hitting your server
    ClientStorageKeys: []string{"theme", "sidebar-collapsed", "locale"},
}
```

**2. Handshake Protocol**

The thin client reads *only* the allowlisted keys and sends them with the initial connection:

```
Client → Server: CONNECT { storage: { "theme": "dark", "sidebar-collapsed": "true" } }
```

Data is available **before first render** - no flicker, no "flash of wrong content."

**3. The Hook API (high-level, recommended)**

```go
func Dashboard(ctx vango.Ctx) vango.Component {
    // Returns a signal that:
    // - Initializes from handshake data (instant, already on server)
    // - Sends MSG_STORAGE_SET to client on every Set()
    sidebarOpen := vango.UseLocalStorage(ctx, "sidebar-collapsed", false)
    theme := vango.UseLocalStorage(ctx, "theme", "light")

    return Div(
        Class(theme.Get()), // Correct on first render!
        Button(OnClick(func() { sidebarOpen.Toggle() })), // Persists automatically
    )
}
```

**4. Low-Level Primitives (for custom needs)**

```go
// Raw read (from handshake data, already in memory)
value := ctx.ClientStorage("some-key") // returns string, empty if not set

// Raw write (sends to client)
ctx.WriteClientStorage("some-key", "some-value")
```

### Rationale

| Concern | `.Persist()` API | Sync-on-Connect |
|---------|------------------|-----------------|
| **Timing** | Unclear when values are available | Explicit: available after handshake |
| **What syncs** | Implicit (whatever has `.Persist()`) | Explicit (configured allowlist) |
| **First render** | Wrong value, then flash-correction | Correct from the start |
| **Security** | Client controls what's sent | Server controls allowlist |
| **Debugging** | "Why isn't my value there?" | Clear data flow |
| **Context** | None - whose data? | Explicit `ctx` - this user's data |

### Use Cases This Serves

| Use Case | Solution |
|----------|----------|
| Dark mode preference | `UseLocalStorage(ctx, "theme", "light")` |
| Sidebar collapsed state | `UseLocalStorage(ctx, "sidebar", false)` |
| Table column widths | `UseLocalStorage(ctx, "columns", defaultWidths)` |
| Filter preferences | `UseLocalStorage(ctx, "filters", defaultFilters)` |
| Form draft (ephemeral) | `UseSessionStorage(ctx, "draft", "")` |
| User settings (permanent) | Explicit DB call in your domain layer |

### What About Database Persistence?

**Not a framework concern.** Application data belongs in your domain logic:

```go
// Clear, debuggable, controllable
func SavePreferences(ctx vango.Ctx) {
    prefs := getCurrentPrefs(ctx)
    if err := db.SaveUserPrefs(ctx.UserID(), prefs); err != nil {
        showError(ctx, "Failed to save preferences")
        return
    }
    showSuccess(ctx, "Saved!")
}
```

This gives you:
- Transaction control
- Validation
- Error handling
- Audit logging
- Whatever your app needs

### Migration

1. Remove `.Persist()` method from `SignalValue[T]`
2. Remove `vango.LocalStorage`, `vango.SessionStorage`, `vango.Database` constants
3. Add `ClientStorageKeys` to `vango.Config`
4. Modify handshake protocol to include storage data
5. Add `UseLocalStorage()` and `UseSessionStorage()` hooks
6. Add `ctx.ClientStorage()` and `ctx.WriteClientStorage()` methods
7. Update all documentation to reflect new patterns

---

## 3. UI Components

*TODO: Document blessed UI component decisions*

---

## 4. Other Considerations

*TODO: Additional items to address before release*
