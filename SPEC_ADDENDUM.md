# Vango Spec Addendum: Structured Side Effects and Effect Helpers

**Version:** 2.3.1  
**Status:** Normative  
**Date:** January 4, 2026

---

## Overview

This addendum updates Vango's side-effect model to provide one obvious, hard-to-misuse answer for each common task:

| Task | Primitive | Notes |
|------|-----------|-------|
| Async data loading | `Resource` | Existing (§3.9.7) |
| User mutation | `Action` | New (§A.1) |
| Periodic work | `Interval` | New (§A.2.1) |
| Event streams | `Subscribe` | New (§A.2.2) |
| Async with reactive deps | `Effect` + `GoLatest` | New (§A.2.3) |
| Pure wiring | `Effect` | Existing, refined (§A.3) |

The design minimizes ambiguous choices and removes boilerplate with subtle correctness hazards, while preserving Vango's deterministic commit model and runtime safety mechanisms.

---

## Integration with Main Spec (§3.9.6)

The following callout MUST be added to §3.9.6 (Effect API) immediately after the introductory paragraph:

> ### Prefer Structured Helpers Over Raw Effect
>
> Before writing a raw Effect with goroutines, use the appropriate structured helper:
>
> | Need | Use | Reference |
> |------|-----|-----------|
> | Async data loading | `Resource` | §3.9.7 |
> | User-triggered mutation | `Action` | §A.1 |
> | Periodic work (timers, polling) | `Interval` | §A.2.1 |
> | Event streams (WebSocket, SSE) | `Subscribe` | §A.2.2 |
> | Async work with reactive deps | `Effect` + `GoLatest` | §A.2.3 |
>
> Raw Effect is for pure wiring when none of the above fit. The helpers handle cancellation, stale suppression, dispatch, and transaction naming automatically.

---

## A.1 Action API

Action is the structured primitive for async mutations, complementing Resource (async queries).

### A.1.1 Motivation

Mutations currently require ad-hoc boilerplate: manual loading/error/success state signals, cancellation handling, and dispatch ceremony. Action bundles this into a standard, auditable unit.

### A.1.2 Type Definitions

```go
package vango

type ActionState int

const (
    ActionIdle ActionState = iota
    ActionRunning
    ActionSuccess
    ActionError
)

type Action[A any, R any] struct {
    // unexported fields
}

func (a *Action[A, R]) Run(arg A) (accepted bool)
func (a *Action[A, R]) State() ActionState
func (a *Action[A, R]) Result() (R, bool)
func (a *Action[A, R]) Error() error
func (a *Action[A, R]) Reset()
```

### A.1.3 Constructor

```go
func NewAction[A any, R any](
    do func(ctx context.Context, a A) (R, error),
    opts ...ActionOption,
) *Action[A, R]
```

**Normative**: `NewAction` MUST be called during render, effect, or handler execution (i.e., when `UseCtx()` is valid). The Action obtains runtime context internally.

**Normative**: `UseCtx()` is valid during component render, event handler execution, Effect execution, and any callback invoked on the session loop via `ctx.Dispatch(...)` (including Resource/Action/GoLatest apply callbacks). Calling `NewAction`, `Interval`, `Subscribe`, or `GoLatest` when `UseCtx()` is invalid MUST panic with a diagnostic error.

**Normative**: `NewAction` returns a stable pointer that persists across re-renders for a given component instance (hook-slot semantics). Like other hook-order primitives (`NewSignal`, `NewMemo`, `Effect`, `NewResource`), it MUST be called unconditionally and in a consistent order on every render. Conditional creation or creation inside loops with variable iteration counts is a hook-order violation.

**Normative**: Options passed to `NewAction` are applied at creation time. If `NewAction` is invoked in subsequent renders at the same hook slot with different options, the runtime MUST ignore subsequent option changes and SHOULD emit a dev-mode warning.

### A.1.4 Options

```go
type ActionOption interface{ isActionOption() }

// Concurrency policies
func CancelLatest() ActionOption   // Cancel prior in-flight on new Run
func DropWhileRunning() ActionOption // Ignore Run while Running
func Queue(max int) ActionOption   // Buffer up to max, execute sequentially

// Naming and observability
func ActionTxName(name string) ActionOption
func OnActionStart(fn func()) ActionOption
func OnActionSuccess[R any](fn func(R)) ActionOption
func OnActionError(fn func(error)) ActionOption

// Storm budget override
func OnActionBudgetExceeded(mode BudgetExceededMode) ActionOption
```

### A.1.5 Normative Semantics

1. **Off-loop execution**: The `do` function MUST execute off the session event loop.

2. **On-loop state transitions**: All Action state transitions (Idle→Running, Running→Success, Running→Error) MUST be applied on the session loop via `ctx.Dispatch(...)`, executing inside an implicit transaction.

3. **Concurrency policies**:
   - `CancelLatest()`: Calling `Run` while Running MUST cancel the prior in-flight call (via context cancellation) before starting the new one. Returns `true`.
   - `DropWhileRunning()`: Calling `Run` while Running MUST be a no-op. Returns `false`.
   - `Queue(max)`: Calls to `Run` while Running MUST be queued up to `max`. Returns `true` if queued successfully, `false` if queue is full. Exceeding `max` MUST also set the Action to `ActionError` with `ErrQueueFull`.

4. **Default policy**: If no concurrency policy is specified, the default MUST be `CancelLatest()`.

5. **Multiple policies**: At most one concurrency policy MAY be specified. If multiple are provided, the runtime MUST panic in dev mode and MUST deterministically select the first policy in prod while emitting telemetry.

6. **Run return value**: `Run` MUST return `true` if the call was accepted (started or queued), `false` if rejected (dropped due to `DropWhileRunning` or queue full). This enables immediate UI feedback without waiting for state propagation.

7. **State transitions**: Calling `Run` from any state transitions the Action to `ActionRunning` (subject to concurrency policy). A successful completion MUST set `ActionSuccess` and overwrite the stored result; a failed completion MUST set `ActionError` and overwrite the stored error. `Reset()` MUST set `ActionIdle` and clear stored result/error.

8. **Transaction naming**: State transitions MUST appear as named transactions in DevTools:
   - With `ActionTxName("save_profile")`: `Action:save_profile:running`, `Action:save_profile:success`, etc.
   - Default: `Action:<component>:<instance>:<state>` (dev mode MAY include file:line).

### A.1.6 Example Usage

```go
func ProfileEditor() vango.Component {
    return vango.Func(func() *vango.VNode {
        profile := vango.NewSignal(Profile{})
        
        save := vango.NewAction(
            func(ctx context.Context, p Profile) (Profile, error) {
                return api.SaveProfile(ctx, p)
            },
            vango.CancelLatest(),
            vango.ActionTxName("profile:save"),
            vango.OnActionSuccess(func(p Profile) {
                profile.Set(p)
                toast.Success("Profile saved")
            }),
            vango.OnActionError(func(err error) {
                toast.Error("Failed to save: " + err.Error())
            }),
        )

        return Form(
            OnSubmit(func() {
                save.Run(profile.Get())
            }),
            
            // ... form fields ...
            
            Button(
                Type("submit"),
                Disabled(save.State() == vango.ActionRunning),
                IfElse(save.State() == vango.ActionRunning,
                    Text("Saving..."),
                    Text("Save"),
                ),
            ),
        )
    })
}
```

---

## A.2 Effect Helpers

These helpers standardize common Effect patterns, handling cleanup, dispatch, and transaction naming automatically. They are **not new primitives**—they are ergonomic wrappers that encode correctness patterns for use inside Effect.

### A.2.0 Call-Site and Lifetime Semantics

**Normative**: `Interval`, `Subscribe`, and `GoLatest` SHOULD be called inside an Effect (or lifecycle hook that accepts cleanup), and their returned Cleanup SHOULD be returned from that Effect.

**Normative**: If invoked outside an Effect (e.g., from an event handler), the caller MUST retain and eventually invoke the returned Cleanup. The runtime MAY warn in dev mode if a helper's returned Cleanup is dropped without being wired into a lifecycle.

**Canonical pattern:**
```go
vango.Effect(func() vango.Cleanup {
    return vango.Interval(time.Second, tick)
})
```

**Anti-pattern (leak):**
```go
// DON'T: cleanup is dropped
vango.Effect(func() vango.Cleanup {
    vango.Interval(time.Second, tick) // leaked!
    return nil
})
```

### A.2.1 Interval

#### Signature

```go
func Interval(
    d time.Duration,
    fn func(),
    opts ...IntervalOption,
) Cleanup

type IntervalOption interface{ isIntervalOption() }
func IntervalTxName(name string) IntervalOption
func IntervalImmediate() IntervalOption // first tick occurs immediately
```

#### Normative Semantics

1. **Scheduling**: `Interval` MUST schedule ticks off the session loop.

2. **Dispatch**: Each tick MUST invoke `fn` on the session loop via `ctx.Dispatch(fn)`.

3. **Cleanup**: The returned Cleanup MUST stop future ticks and MUST be called when the owning component unmounts. User code MUST NOT need to manually stop tickers.

4. **Start timing**: By default, the first tick MUST occur after `d` duration from the Interval creation (not immediately). If `IntervalImmediate()` is provided, the first tick MUST occur as soon as possible after creation (still dispatched through the session loop).

5. **Transaction naming**:
   - With `IntervalTxName("heartbeat")`: `Interval:heartbeat`
   - Default: `Interval:<component>:<instance>` (dev mode MAY include file:line)

#### Example

```go
func Timer() vango.Component {
    return vango.Func(func() *vango.VNode {
        elapsed := vango.NewSignal(0)

        vango.Effect(func() vango.Cleanup {
            return vango.Interval(time.Second, func() {
                elapsed.Inc() // auto-dispatched, no manual ceremony
            }, vango.IntervalTxName("timer:tick"))
        })

        return Div(Textf("Elapsed: %d seconds", elapsed.Get()))
    })
}
```

### A.2.2 Subscribe

#### Signature

```go
type Stream[T any] interface {
    Subscribe(handler func(T)) (unsubscribe func())
}

func Subscribe[T any](
    stream Stream[T],
    fn func(T),
    opts ...SubscribeOption,
) Cleanup

type SubscribeOption interface{ isSubscribeOption() }
func SubscribeTxName(name string) SubscribeOption
```

#### Normative Semantics

1. **Dispatch**: Upon receiving a message, `Subscribe` MUST invoke `fn(msg)` on the session loop via `ctx.Dispatch`.

2. **Cleanup**: The returned Cleanup MUST unsubscribe from the stream.

3. **Transaction naming**:
   - With `SubscribeTxName("ws_messages")`: `Subscribe:ws_messages`
   - Default: `Subscribe:<component>:<instance>` (dev mode MAY include file:line)

#### Example

```go
func ChatRoom(roomID string) vango.Component {
    return vango.Func(func() *vango.VNode {
        messages := vango.NewSignal([]Message{})
        ws := websocket.Connect(roomID)

        vango.Effect(func() vango.Cleanup {
            return vango.Subscribe(ws.Messages, func(msg Message) {
                messages.Append(msg) // auto-dispatched
            }, vango.SubscribeTxName("chat:message"))
        })

        return Div(
            Range(messages.Get(), func(m Message, i int) *vango.VNode {
                return Div(Key(m.ID), Text(m.Text))
            }),
        )
    })
}
```

### A.2.3 GoLatest

`GoLatest` is the standard helper for async integration work inside Effect when Resource or Action do not fit.

#### Signature

```go
func GoLatest[K comparable, R any](
    key K,
    work func(ctx context.Context, key K) (R, error),
    apply func(result R, err error),
    opts ...GoLatestOption,
) Cleanup

type GoLatestOption interface{ isGoLatestOption() }
func GoLatestTxName(name string) GoLatestOption
func GoLatestForceRestart() GoLatestOption // restart even when key unchanged
func OnGoLatestBudgetExceeded(mode BudgetExceededMode) GoLatestOption
```

#### Normative Semantics

1. **Call-site identity**: "Call site" refers to the specific invocation position (hook slot) within a given component instance. For a stable Effect slot, successive executions of that Effect share the same GoLatest call-site identity if `GoLatest` is invoked at the same lexical position during that Effect.

2. **Key coalescing (default)**: If `GoLatest` is invoked with a key equal to the previous invocation at the same call site, the runtime MUST NOT cancel, restart, or start new work. This prevents unnecessary refetches when effects re-run for unrelated reasons.
   
   If `GoLatestForceRestart()` is specified, the runtime MUST cancel any in-flight work and MUST start new work even when keys are equal.
   
   **Rationale**: "Same key = same data = no work" is a simpler mental model and reduces storm-budget triggers. Developers who need refresh semantics can use `GoLatestForceRestart()`, include a nonce in the key, or use `Action` for explicit user-triggered refresh.

3. **Cancel-latest (different keys)**: Invoking `GoLatest` with a new key (different from prior) MUST cancel the prior in-flight work for that call site via context cancellation.

4. **Off-loop execution**: The `work` function MUST execute off the session event loop.

5. **On-loop apply**: The `apply` callback MUST be invoked on the session loop via `ctx.Dispatch(...)`.

6. **Stale suppression**: If the key has changed since `work` started (i.e., another `GoLatest` call occurred with a different key), `apply` MUST NOT be invoked. The runtime MUST emit telemetry for stale-ignored results:
   ```go
   // Telemetry event (structured, not user-facing callback)
   {
       "event": "golatest.stale_ignored",
       "key_started": <redacted_hash>,  // Privacy: hashed by default
       "work_duration_ms": <duration>,
       "tx_name": "<name>"
   }
   ```
   **Privacy**: `key_started` MUST be redacted by default (e.g., hashed). Raw keys MUST NOT be logged unless explicitly enabled via `DebugConfig.LogRawKeys`, which MUST default to `false` in production.

7. **Cleanup**: The returned Cleanup MUST cancel in-flight work and prevent future `apply` invocations.

8. **Transaction naming**:
   - With `GoLatestTxName("user_fetch")`: `GoLatest:user_fetch`
   - Default: `GoLatest:<component>:<instance>` (dev mode MAY include file:line)

#### Example

```go
func UserSearch() vango.Component {
    return vango.Func(func() *vango.VNode {
        query := vango.NewSignal("")
        results := vango.NewSignal([]User{})
        searchErr := vango.NewSignal[error](nil)

        vango.Effect(func() vango.Cleanup {
            q := query.Get()
            if q == "" {
                results.Set([]User{})
                searchErr.Set(nil)
                return nil
            }
            
            return vango.GoLatest(q,
                func(ctx context.Context, q string) ([]User, error) {
                    return api.SearchUsers(ctx, q)
                },
                func(users []User, err error) {
                    // Already dispatched, already stale-checked
                    if err != nil {
                        searchErr.Set(err)
                        results.Set([]User{})
                    } else {
                        searchErr.Set(nil)
                        results.Set(users)
                    }
                },
                vango.GoLatestTxName("user:search"),
            )
        })

        return Div(
            Input(
                Type("search"),
                Value(query.Get()),
                OnInput(query.Set),
            ),
            // ... render results ...
        )
    })
}
```

#### Comparison: Before and After

**Before (manual ceremony, ~25 lines, 5+ correctness hazards):**
```go
vango.Effect(func() vango.Cleanup {
    ctx := vango.UseCtx()
    cctx, cancel := context.WithCancel(ctx.StdContext())
    q := query.Get()

    go func(q string) {
        users, err := api.SearchUsers(cctx, q)
        if err != nil && cctx.Err() != nil {
            return
        }
        ctx.Dispatch(func() {
            if cctx.Err() != nil {
                return
            }
            if query.Peek() != q {
                return  // Easy to forget!
            }
            if err != nil {
                searchErr.Set(err)
                results.Set([]User{})
            } else {
                searchErr.Set(nil)
                results.Set(users)
            }
        })
    }(q)

    return cancel
})
```

**After (GoLatest, ~15 lines, correct by construction):**
```go
vango.Effect(func() vango.Cleanup {
    q := query.Get()
    return vango.GoLatest(q,
        func(ctx context.Context, q string) ([]User, error) {
            return api.SearchUsers(ctx, q)
        },
        func(users []User, err error) {
            if err != nil {
                searchErr.Set(err)
                results.Set([]User{})
            } else {
                searchErr.Set(nil)
                results.Set(users)
            }
        },
    )
})
```

---

## A.3 Effect Enforcement

### A.3.1 Motivation

Effect is necessary as an escape hatch for wiring and integration, but should be "wiring-only" by default to prevent common misuse patterns (accidental signal writes that cause amplification loops).

### A.3.2 Options

```go
type EffectOption interface{ isEffectOption() }
func AllowWrites() EffectOption
func EffectTxName(name string) EffectOption
```

**EffectTxName semantics**: `EffectTxName(name)` sets the identifier used in Effect warnings/telemetry and in DevTools entries related to that Effect (e.g., effect-time write warnings). It does NOT propagate to helper transactions (Interval/Subscribe/GoLatest); those use their own `*TxName` options or fall back to default naming.

### A.3.3 Strictness Configuration

```go
type StrictEffectMode int

const (
    StrictEffectOff StrictEffectMode = iota
    StrictEffectWarn   // Default in dev
    StrictEffectPanic
)

type EffectConfig struct {
    Mode StrictEffectMode // default: Warn in dev, Off in prod
}
```

### A.3.4 Normative Semantics

1. **Effect-time write definition**: A mutation is an "effect-time write" if it occurs synchronously during execution of the Effect callback on the session loop, before the Effect callback returns. Mutations occurring in goroutines spawned by the Effect, or inside `ctx.Dispatch(...)` callbacks, are NOT effect-time writes.

2. **Covered mutations**: Effect-time writes include all signal mutation APIs: `Set`, `Update`, `Inc`, `Dec`, `Toggle`, `Append`, `Prepend`, `Clear`, `SetKey`, `RemoveKey`, `UpdateKey`, `InsertAt`, `RemoveAt`, `UpdateAt`, `RemoveWhere`, `UpdateWhere`, and any other operation that adds entries to the transaction write set.

3. **Enforcement behavior**:

   | Condition | StrictEffectWarn | StrictEffectPanic |
   |-----------|-------------------|-------------------|
   | Write without `AllowWrites()` | Warning emitted | Panic |
   | Write with `AllowWrites()` | No warning | No panic |

4. **Warning format** (normative UX):
   ```
   Warning: Effect wrote signal 'elapsed' via Inc() at components/timer.go:42
     → For periodic updates, use vango.Interval()
     → For event streams, use vango.Subscribe()
     → For async work, use Effect + vango.GoLatest()
     → For intentional writes, add vango.AllowWrites()
   ```

5. **Default strictness**:
   - Development: `StrictEffectWarn`
   - Production: `StrictEffectOff` (unless explicitly configured)

6. **Telemetry**: Even when `AllowWrites()` is set, the runtime SHOULD capture telemetry that the effect performed writes, for observability purposes.

### A.3.5 Example: Intentional Effect-Body Writes

Rare cases where synchronous initialization requires a direct effect-body write:

```go
// Syncing with external state that must be read synchronously on mount
vango.Effect(func() vango.Cleanup {
    // This is an effect-body write (not in Dispatch) - requires AllowWrites()
    syncedValue.Set(legacySystem.ReadCurrentSync())
    
    // Subsequent updates come through Dispatch (normal pattern)
    unsub := legacySystem.Subscribe(func(v any) {
        vango.UseCtx().Dispatch(func() {
            syncedValue.Set(v) // This is in Dispatch - doesn't require AllowWrites()
        })
    })
    return unsub
}, vango.AllowWrites())
```

**Note**: Most patterns do not require `AllowWrites()`. If your Effect only writes signals inside `ctx.Dispatch(...)` callbacks (as with `Interval`, `Subscribe`, `GoLatest`), no opt-in is needed.

---

## A.4 Storm Budgets

### A.4.1 Motivation

In server-driven architectures, amplification bugs (effects re-running excessively, triggering repeated I/O) consume shared server resources. Runtime budgets provide a safety net.

### A.4.2 Configuration

```go
type BudgetExceededMode int

const (
    BudgetThrottle BudgetExceededMode = iota // Default
    BudgetTripBreaker
)

type StormBudgetConfig struct {
    // Per-window start limits (0 = unlimited)
    MaxResourceStartsPerSecond int
    MaxActionStartsPerSecond   int
    MaxGoLatestStartsPerSecond int
    
    // Per-tick limits
    MaxEffectRunsPerTick int
    
    // Throttle window duration (default: 1s if zero)
    WindowDuration time.Duration
    
    // Default behavior when exceeded
    OnExceeded BudgetExceededMode
}

// In session config
type SessionConfig struct {
    // ... existing fields ...
    StormBudget StormBudgetConfig
}
```

### A.4.3 Normative Semantics

1. **Throttle behavior** (default):
   - The runtime MUST deny or delay new starts when budget is exceeded.
   - The runtime MUST surface an error state through the relevant primitive:
     - `Resource`: transitions to Error state with `ErrBudgetExceeded`
     - `Action`: transitions to Error state with `ErrBudgetExceeded`; `Run` returns `false`
     - `GoLatest`: When throttled, the runtime MUST NOT start `work`. The runtime MUST invoke `apply(zero, ErrBudgetExceeded)` at most once per throttle window per call site.
   - **Error surfacing scope**: Budget-exceeded error surfacing suppression MUST be tracked per primitive instance (Resource instance / Action instance / GoLatest call site) to avoid cross-component interference.
   - The runtime MUST emit telemetry with budget counters and callsite names.

2. **Trip breaker behavior**:
   - The runtime MUST terminate the session or transition it to a "session invalidated" state.
   - This is appropriate for protecting critical downstream services.

3. **Per-primitive override**: `Resource`, `Action`, and `GoLatest` MUST support budget-exceeded override options:
   ```go
   // Auth checks should trip breaker to protect auth service
   authCheck := vango.NewResource(fetchAuth,
       vango.OnResourceBudgetExceeded(vango.BudgetTripBreaker),
   )
   ```

4. **Telemetry fields**: Budget-related telemetry MUST include:
   - `budget_type`: `resource` | `action` | `golatest` | `effect`
   - `limit`: configured limit
   - `current`: current count in window
   - `tx_name`: transaction/primitive name
   - `action_taken`: `throttled` | `breaker_tripped`
   - `error_surfaced`: `true` | `false`

### A.4.4 Recommended Defaults

```go
StormBudgetConfig{
    MaxResourceStartsPerSecond: 100,
    MaxActionStartsPerSecond:   50,
    MaxGoLatestStartsPerSecond: 100,
    MaxEffectRunsPerTick:       50,
    WindowDuration:             time.Second,
    OnExceeded:                 BudgetThrottle,
}
```

---

## A.5 Transaction Naming

### A.5.1 Motivation

DevTools transaction timelines are the primary debugging tool. Anonymous transactions reduce debuggability at scale.

### A.5.2 Naming Rules

All helpers and Action state transitions MUST emit named transactions according to these rules:

| Primitive | With TxName option | Default (dev) | Default (prod) |
|-----------|-------------------|---------------|----------------|
| `Action` | `Action:<name>:<state>` | `Action:<file>:<line>:<state>` | `Action:<component>:<instance>:<state>` |
| `Interval` | `Interval:<name>` | `Interval:<file>:<line>` | `Interval:<component>:<instance>` |
| `Subscribe` | `Subscribe:<name>` | `Subscribe:<file>:<line>` | `Subscribe:<component>:<instance>` |
| `GoLatest` | `GoLatest:<name>` | `GoLatest:<file>:<line>` | `GoLatest:<component>:<instance>` |

### A.5.3 Source Location Configuration

```go
type DebugConfig struct {
    // Include file:line in transaction names (default: true in dev, false in prod)
    IncludeSourceLocations bool
    
    // Log unhashed keys in GoLatest telemetry (default: false; MUST be false in prod)
    LogRawKeys bool
}
```

---

## A.6 Agent Guidance

This section provides canonical decision patterns for AI coding agents generating Vango code.

### A.6.1 Primitive Selection

**If you are inside an Effect and think you need a goroutine, stop and use:**

| Need | Use | Why |
|------|-----|-----|
| Time-based repetition | `Interval` | Handles cleanup, dispatch, naming |
| Event stream | `Subscribe` | Handles unsubscribe, dispatch, naming |
| Async keyed work | `GoLatest` | Handles cancel, stale, dispatch, naming |

**If you are responding to user intent (button click, form submit):**
- Use `Action` for async mutations
- Use direct signal writes for sync updates

**If you are loading/rendering data:**
- Use `Resource` for async queries with loading/error/ready states
- Use `NewResourceKeyed` when the query depends on reactive state

### A.6.2 Anti-Pattern Recognition

**Never write this:**
```go
vango.Effect(func() vango.Cleanup {
    ticker := time.NewTicker(time.Second)
    go func() {
        for range ticker.C {
            ctx.Dispatch(...)  // Manual dispatch
        }
    }()
    return ticker.Stop
})
```

**Write this instead:**
```go
vango.Effect(func() vango.Cleanup {
    return vango.Interval(time.Second, func() {
        // Auto-dispatched
    })
})
```

**Never write this:**
```go
vango.Effect(func() vango.Cleanup {
    cctx, cancel := context.WithCancel(...)
    go func() {
        result, err := fetch(cctx, key)
        ctx.Dispatch(func() {
            if cctx.Err() != nil { return }
            if key != currentKey.Peek() { return }  // Manual stale check
            // ...
        })
    }()
    return cancel
})
```

**Write this instead:**
```go
vango.Effect(func() vango.Cleanup {
    return vango.GoLatest(key,
        func(ctx context.Context, k K) (R, error) {
            return fetch(ctx, k)
        },
        func(result R, err error) {
            // Auto-dispatched, auto-stale-checked
        },
    )
})
```

### A.6.3 Correctness Checklist

Before submitting Effect code, verify:

- [ ] No manual `time.Ticker` — use `Interval`
- [ ] No manual stream subscribe/unsubscribe — use `Subscribe`
- [ ] No manual goroutine + Dispatch + stale check — use `GoLatest`
- [ ] No manual loading/error/success signals — use `Resource` or `Action`
- [ ] Cleanup is returned from Effect (not dropped)
- [ ] Effect helpers (`Interval`/`Subscribe`/`GoLatest`) are inside an Effect

---

## A.7 Documentation Structure

### A.7.1 Required Reorganization

Documentation MUST present primitives in order of "reach for this first":

1. **Resource** (§3.9.7) — async queries, load-on-mount, key-based refetch
2. **Action** (§A.1) — user-triggered mutations
3. **Interval** (§A.2.1) — periodic work
4. **Subscribe** (§A.2.2) — event streams
5. **Effect + GoLatest** (§A.2.3) — async integration inside Effect
6. **Effect** (§3.9.6) — rare wiring, integration glue

### A.7.2 Pattern Matching Guide

Documentation MUST include a decision flowchart:

```
Need to load data on mount or when key changes?
  └─→ Use Resource

Need to perform a user-triggered mutation (save, delete, submit)?
  └─→ Use Action

Need periodic updates (polling, timers)?
  └─→ Use Effect + Interval

Need to react to an event stream (WebSocket, SSE, pub/sub)?
  └─→ Use Effect + Subscribe

Need async work triggered by reactive state that doesn't fit above?
  └─→ Use Effect + GoLatest

Need pure wiring (no async, just connecting things)?
  └─→ Use Effect (rare)
```

### A.7.3 Desugared Patterns Appendix

Documentation MUST include an appendix showing what helpers expand to, for understanding and advanced customization.

**Note**: Signal writes in these desugared examples occur inside `ctx.Dispatch(...)` transactions, not in the Effect body. They do not require `AllowWrites()`.

**Interval desugared:**
```go
// What Interval(d, fn) expands to:
vango.Effect(func() vango.Cleanup {
    ctx := vango.UseCtx()
    ticker := time.NewTicker(d)
    done := make(chan struct{})
    go func() {
        for {
            select {
            case <-ticker.C:
                ctx.Dispatch(func() {
                    vango.TxNamed("Interval:...", fn)
                })
            case <-done:
                return
            }
        }
    }()
    return func() {
        close(done)
        ticker.Stop()
    }
})
```

**GoLatest desugared (conceptual):**
```go
// Per-call-site state (managed by runtime)
var lastKey K
var lastSeq uint64

vango.Effect(func() vango.Cleanup {
    ctx := vango.UseCtx()
    
    // Key coalescing: if same key, do nothing (regardless of in-flight state)
    if key == lastKey {
        return nil  // No work, no restart
    }
    
    lastKey = key
    lastSeq++
    mySeq := lastSeq
    
    cctx, cancel := context.WithCancel(ctx.StdContext())

    go func(mySeq uint64, k K) {
        result, err := work(cctx, k)
        if cctx.Err() != nil {
            return
        }
        
        ctx.Dispatch(func() {
            if cctx.Err() != nil {
                return
            }
            if lastSeq != mySeq {
                // Stale - emit telemetry, don't apply
                return
            }
            vango.TxNamed("GoLatest:...", func() {
                apply(result, err)
            })
        })
    }(mySeq, key)

    return cancel
})
```

---

## A.8 Migration Guide

### A.8.1 From Effect + Ticker to Interval

**Before:**
```go
vango.Effect(func() vango.Cleanup {
    ctx := vango.UseCtx()
    ticker := time.NewTicker(time.Second)
    done := make(chan struct{})
    go func() {
        for {
            select {
            case <-ticker.C:
                ctx.Dispatch(elapsed.Inc)
            case <-done:
                return
            }
        }
    }()
    return func() {
        close(done)
        ticker.Stop()
    }
})
```

**After:**
```go
vango.Effect(func() vango.Cleanup {
    return vango.Interval(time.Second, elapsed.Inc)
})
```

### A.8.2 From Effect + Subscribe to Subscribe Helper

**Before:**
```go
vango.Effect(func() vango.Cleanup {
    ctx := vango.UseCtx()
    unsub := stream.Subscribe(func(msg Message) {
        ctx.Dispatch(func() {
            messages.Append(msg)
        })
    })
    return unsub
})
```

**After:**
```go
vango.Effect(func() vango.Cleanup {
    return vango.Subscribe(stream, func(msg Message) {
        messages.Append(msg)
    })
})
```

### A.8.3 From Manual Async to GoLatest

**Before:**
```go
vango.Effect(func() vango.Cleanup {
    ctx := vango.UseCtx()
    cctx, cancel := context.WithCancel(ctx.StdContext())
    id := userID.Get()
    go func(id int) {
        user, err := api.FetchUser(cctx, id)
        if cctx.Err() != nil { return }
        ctx.Dispatch(func() {
            if cctx.Err() != nil { return }
            if userID.Peek() != id { return }
            if err != nil {
                fetchErr.Set(err)
            } else {
                currentUser.Set(user)
            }
        })
    }(id)
    return cancel
})
```

**After:**
```go
vango.Effect(func() vango.Cleanup {
    return vango.GoLatest(userID.Get(),
        func(ctx context.Context, id int) (*User, error) {
            return api.FetchUser(ctx, id)
        },
        func(user *User, err error) {
            if err != nil {
                fetchErr.Set(err)
            } else {
                currentUser.Set(user)
            }
        },
    )
})
```

### A.8.4 From Ad-hoc Mutation to Action

**Before:**
```go
func ProfilePage() vango.Component {
    return vango.Func(func() *vango.VNode {
        ctx := vango.UseCtx()
        profile := vango.NewSignal(Profile{})
        saving := vango.NewSignal(false)
        saveErr := vango.NewSignal[error](nil)

        save := func() {
            saving.Set(true)
            saveErr.Set(nil)
            go func() {
                p := profile.Peek()
                result, err := api.SaveProfile(p)
                ctx.Dispatch(func() {
                    saving.Set(false)
                    if err != nil {
                        saveErr.Set(err)
                    } else {
                        profile.Set(result)
                        toast.Success("Saved!")
                    }
                })
            }()
        }
        // ...
    })
}
```

**After:**
```go
func ProfilePage() vango.Component {
    return vango.Func(func() *vango.VNode {
        profile := vango.NewSignal(Profile{})

        save := vango.NewAction(
            func(ctx context.Context, p Profile) (Profile, error) {
                return api.SaveProfile(ctx, p)
            },
            vango.OnActionSuccess(func(p Profile) {
                profile.Set(p)
                toast.Success("Saved!")
            }),
        )

        // save.State() == ActionRunning replaces saving signal
        // save.Error() replaces saveErr signal
        // ...
    })
}
```

---

## A.9 Complete API Reference

### A.9.1 Action

```go
// Types
type ActionState int
const (ActionIdle, ActionRunning, ActionSuccess, ActionError)
type Action[A any, R any] struct{}

// Constructor
func NewAction[A, R any](do func(context.Context, A) (R, error), opts ...ActionOption) *Action[A, R]

// Methods
func (*Action[A, R]) Run(A) bool  // Returns true if accepted
func (*Action[A, R]) State() ActionState
func (*Action[A, R]) Result() (R, bool)
func (*Action[A, R]) Error() error
func (*Action[A, R]) Reset()

// Options
func CancelLatest() ActionOption
func DropWhileRunning() ActionOption
func Queue(max int) ActionOption
func ActionTxName(string) ActionOption
func OnActionStart(func()) ActionOption
func OnActionSuccess[R any](func(R)) ActionOption
func OnActionError(func(error)) ActionOption
func OnActionBudgetExceeded(BudgetExceededMode) ActionOption
```

### A.9.2 Effect Helpers

```go
// Interval
func Interval(d time.Duration, fn func(), opts ...IntervalOption) Cleanup
func IntervalTxName(string) IntervalOption
func IntervalImmediate() IntervalOption

// Subscribe
type Stream[T any] interface { Subscribe(handler func(T)) (unsubscribe func()) }
func Subscribe[T any](Stream[T], func(T), ...SubscribeOption) Cleanup
func SubscribeTxName(string) SubscribeOption

// GoLatest
func GoLatest[K comparable, R any](K, func(context.Context, K) (R, error), func(R, error), ...GoLatestOption) Cleanup
func GoLatestTxName(string) GoLatestOption
func GoLatestForceRestart() GoLatestOption
func OnGoLatestBudgetExceeded(BudgetExceededMode) GoLatestOption
```

### A.9.3 Effect Options

```go
func AllowWrites() EffectOption
func EffectTxName(string) EffectOption
```

### A.9.4 Storm Budgets

```go
type BudgetExceededMode int
const (BudgetThrottle, BudgetTripBreaker)

type StormBudgetConfig struct {
    MaxResourceStartsPerSecond int
    MaxActionStartsPerSecond   int
    MaxGoLatestStartsPerSecond int
    MaxEffectRunsPerTick       int
    WindowDuration             time.Duration // default: 1s if zero
    OnExceeded                 BudgetExceededMode
}

// Per-primitive budget override options
func OnResourceBudgetExceeded(BudgetExceededMode) ResourceOption
func OnActionBudgetExceeded(BudgetExceededMode) ActionOption
func OnGoLatestBudgetExceeded(BudgetExceededMode) GoLatestOption

// Sentinel errors - MUST support errors.Is() checks
var ErrBudgetExceeded = errors.New("vango: storm budget exceeded")
var ErrQueueFull = errors.New("vango: action queue full")
```

### A.9.5 Configuration

```go
type StrictEffectMode int
const (StrictEffectOff, StrictEffectWarn, StrictEffectPanic)

type EffectConfig struct {
    Mode StrictEffectMode
}

type DebugConfig struct {
    IncludeSourceLocations bool // file:line in tx names (default: true dev, false prod)
    LogRawKeys             bool // raw keys in GoLatest telemetry (MUST be false in prod)
}
```

---

## A.10 Cross-References

### A.10.1 Integration with Transactions (§7.8.1)

The "Effect-triggered writes are coalesced and bounded" example in §7.8.1 shows the manual async pattern. This pattern is encapsulated by `GoLatest` (§A.2.3), which handles cancellation, stale suppression, dispatch, and transaction naming automatically.

### A.10.2 Integration with Resource (§3.9.7)

Resource and Action are complementary:
- **Resource**: async *queries* with loading/error/ready states, automatic refetch on key change
- **Action**: async *mutations* with loading/error/success states, explicit trigger via `Run()`

Use Resource when data should load automatically; use Action when the operation requires explicit user intent.

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| 2.3 | 2026-01-04 | Initial: Action, Interval, Subscribe, GoLatest; Effect enforcement; Storm budgets |
| 2.3.1 | 2026-01-04 | GoLatest equal-key default tightened (never rerun unless forced); Action.Run returns bool; EffectTxName no inheritance; privacy-by-default for telemetry keys; Agent Guidance section; section numbering fixes |