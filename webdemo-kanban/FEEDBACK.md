# Vango Framework Feedback & Learnings

From building the Collaborative Kanban Board demo application.

> **Status Update (Dec 2024):** Several issues documented here have been resolved in the DX overhaul. Look for ✅ RESOLVED markers. Items marked ⚠️ OUTSTANDING still need work.

---

## Summary

While building a real-time Kanban board to showcase Vango's server-driven architecture, we encountered several framework-level issues that affect developer experience and application reliability.

---

## Issue 1: Session Resume with Stale Handlers

**Severity**: Critical  
**Status**: **FIXED** (disabled session resume)

### Description
After page refresh, the server was resuming old sessions with stale handler maps instead of creating fresh sessions. This caused click handlers to fail because the old session's handlers were from a different page view.

### Symptoms
- First page load: clicks work fine
- Navigate to another view (e.g., board detail)
- Refresh page → back to dashboard HTML
- **Clicks don't work** - no events received by server

### Root Cause
```go
// server.go HandleWebSocket (lines 177-188)
if hello.SessionID != "" {
    session = s.sessions.Get(hello.SessionID)
    if session != nil {
        session.Resume(conn, ...)
        return  // BUG: Old session has handlers from board view!
    }
}
```

After refresh, SSR renders dashboard (h14 = board card), but resumed session has board view handlers (h14 = different element). Client sends click on h14, server looks for handler in stale map → not found.

### Fix Applied
Disabled session resume temporarily to ensure fresh handlers on every page load:
```go
if false && hello.SessionID != "" { // Disabled: stale handlers cause click failures
```

### Proper Fix (TODO)
On session resume, remount the root component to sync handlers with fresh SSR:
```go
if session != nil {
    session.Resume(conn, ...)
    if s.rootComponent != nil {
        session.MountRoot(s.rootComponent())  // Re-register handlers
    }
    return
}
```

---

## Issue 2: Event Bubbling Through Nested HID Elements

**Severity**: High  
**Status**: **FIXED** (client-side event bubbling)

### Description
Click events on nested elements (like text inside a clickable card) were stopping at the first HID element instead of bubbling up through HID ancestors to find the actual click handler.

### Symptoms
- Clicking **on** the card text → no response
- Clicking **above/below** the text (on parent div) → works
- User must click in exact "dead zones" to trigger handlers

### Root Cause
```html
<div data-hid="h14" data-on-click>  ← Handler here
  <div data-hid="h15">Sample Board</div>  ← No handler
  <div data-hid="h16">3 columns...</div>  ← No handler
</div>
```

Client code (events.js):
```javascript
_handleClick(event) {
    const el = this._findHidElement(event.target);  // Returns h15!
    if (!el.hasAttribute('data-on-click')) return;  // h15 has no handler → STOP
}
```

### Fix Applied
Added `_findHidElementWithHandler()` that bubbles up through HID ancestors:
```javascript
_findHidElementWithHandler(target, handlerAttr) {
    let el = target.closest('[data-hid]');
    while (el) {
        if (el.hasAttribute(handlerAttr)) return el;  // Found handler!
        const parent = el.parentElement;
        if (!parent) break;
        el = parent.closest('[data-hid]');  // Check next HID ancestor
    }
    return null;
}
```

Now clicks on nested text properly bubble up to find the parent's click handler.

---

## Issue 3: SSR/WebSocket Hydration ID Mismatch (Investigation)

**Severity**: Medium (investigated but not the root cause)  
**Status**: Verified synchronized

### Description
Initially suspected SSR and WS used different HID assignment order, but debugging confirmed both assign identical HIDs (h1-h16) to identical elements in identical order.

### Verification
Debug logging showed:
```
[SSR HID] h14 -> div (board card)
[WS HID] h14 -> div (board card)
[HANDLER] Registered onclick on h14 (div)
```

HIDs are **synchronized correctly**. The actual bugs were Issues #1 (session resume) and #2 (event bubbling).

---

## Issue 2: Navigate Events Without Router

**Severity**: Medium  
**Status**: ✅ **RESOLVED** (NavLink helper + documentation)

### Description
When using `<a href="">` tags, the thin client sends `Navigate` events to HID "nav", but without Vango's file-based router configured, there's no handler registered.

### Symptoms
- `WARN handler not found ... hid=nav type=Navigate` in logs
- Links don't navigate

### Resolution (Applied)
- Added `NavLink()` helper in `pkg/vdom/helpers.go` for SPA navigation
- Documented in `docs/reference/01-elements.md` and `docs/concepts/00-common-pitfalls.md`

```go
NavLink("/settings", Text("Settings"))  // SPA navigation
A(Href("/external"), Text("External"))  // Full page reload
```

---

## Issue 3: Background Signal Updates Race Condition

**Severity**: Medium  
**Status**: Fixed in demo

### Description
Using `go func() { signal.Set(...) }()` in component constructors creates a race between SSR render and the goroutine completing.

### Symptoms
- SSR renders with stale state (e.g., `loading=true`)
- WS session renders with updated state
- Hydration mismatch

### Solution
Initialize demo/static data synchronously. Only use goroutines for actual async operations (DB queries, API calls).

---

## Issue 4: Component Re-creation in Render()

**Severity**: High  
**Status**: Fixed in demo

### Description
Creating child components inside `Render()` causes new instances on every render, losing signal connections and spawning duplicate goroutines.

### Bad Pattern
```go
func (r *Root) Render() *VNode {
    dash := NewDashboard(...)  // Creates new instance every render!
    return dash.Render()
}
```

### Good Pattern
```go
func (r *Root) Render() *VNode {
    return r.renderDashboard()  // Use methods that reuse parent's signals
}
```

---

## General Feedback

### What Works Well
1. **Reactive Signals** - Thread-safe, simple API
2. **Binary Protocol** - Efficient patch transmission
3. **Hub Pattern** - Shared state across sessions works as designed
4. **SSR Architecture** - Clean separation of concerns

### Areas for Improvement

| Area | Issue | Status |
|------|-------|--------|
| Developer Experience | Hydration failures are silent | ⚠️ Outstanding |
| Documentation | No guidance on SSR+WS alignment | ✅ Added `00-common-pitfalls.md` |
| Error Messages | "handler not found" doesn't explain why | ✅ Improved (wrapHandler now logs WARN) |
| Tooling | No hydration debugger | ⚠️ Outstanding |

### Missing Features for Production Apps
| Feature | Status |
|---------|--------|
| URL State Sync | ✅ `NavLink()` helper added |
| Session Persistence | ⚠️ Outstanding |
| Auth Integration | ⚠️ Outstanding |
| Error Boundaries | ⚠️ Outstanding |

---

## Files Created/Modified

### Demo App (`webdemo-kanban/`)
- `main.go` - Entry point with SSR + WS setup
- `pkg/app/app.go` - Root component with routing
- `pkg/hub/hub.go` - Shared BoardModel manager
- `pkg/hub/model.go` - Reactive board state
- `pkg/db/db.go` - pgx database client

### Vango Framework (potential fixes needed)
- `pkg/server/session.go` - HID generator initialization
- `pkg/render/renderer.go` - HID assignment during SSR
- `pkg/vdom/hydration.go` - HID assignment during WS mount

---

## Next Steps

1. **Short term**: Use full page navigation in demo to avoid HID issues
2. **Medium term**: Fix HID sync in framework
3. **Long term**: Add proper client-side routing support with History API integration

---

## Phase 3: Kanban State & Drag-and-Drop Limitations

During the implementation of drag-and-drop features, we encountered significant issues related to type safety, DOM manipulation conflicts, and API clarity.

### Issue 5: Hook Event Type Mismatch (Silent Failure)

**Severity**: High
**Status**: ✅ **RESOLVED** (framework + improved logging)

#### Description
The framework defined two distinct `HookEvent` types: one in `pkg/features/hooks` (user-facing) and one in `pkg/server` (internal). The `wrapHandler` function in the server runtime only recognized the internal type, causing user-defined handlers using `hooks.HookEvent` to be silently ignored.

#### Symptoms
- `OnEvent("onreorder", ...)` handler never triggers.
- No error logs, just silence.
- Debugging revealed the handler wrapper fell through to a default no-op case.

#### Resolution (Applied)
1. `pkg/server/handler.go` already handles both `hooks.HookEvent` and `server.HookEvent`
2. Unknown handler types now log a WARN with supported types list:
   ```
   [WARN] wrapHandler: Unrecognized handler type func(main.CustomEvent). 
   Handler will NOT be called. Supported types: func(), func(*Event), 
   func(string), func(hooks.HookEvent), func(FormData), etc.
   ```
3. Documented in `docs/concepts/00-common-pitfalls.md`

### Issue 6: Data Element vs. Data Attribute

**Severity**: Medium (DX confusion)
**Status**: **RESOLVED** (API renamed in framework)

#### Description
The VDOM API had a helper `Data("key", "value")` which created a `<data value="key">value</data>` HTML element. Developers expecting to create `data-*` attributes (e.g., `data-id="123"`) mistakenly used `Data()` instead of `DataAttr()`.

#### Symptoms
- Attributes like `data-id` missing from the target element.
- JavaScript client (SortableJS) failing to find IDs on elements (`dataset.id` is undefined), falling back to internal HIDs.
- Business logic failing (e.g., `MoveCard` logic expecting database IDs but receiving HIDs).

#### Resolution (Applied)
The framework API was updated:
- **`Data(key, value)`** now creates `data-*` attributes (what developers expect)
- **`DataElement()`** creates the rare `<data>` HTML element
- Documentation updated in `docs/reference/01-elements.md` and new `docs/concepts/00-common-pitfalls.md`

```go
Data("id", "123")           // → data-id="123" (attribute)
DataElement(Value("code"))  // → <data value="code"> (element)
```

### Issue 7: DOM Interference & Ghost Artifacts

**Severity**: Medium
**Status**: **FIXED** (layout workaround)

#### Description
Third-party libraries like `SortableJS` manipulate the DOM directly (moving elements). When a Vango component (like an "Add Card" button) resides inside the sortable container, Sortable treats it as a draggable item. If Vango tries to update the VDOM while Sortable has mutated the real DOM, synchronization is lost, leading to duplicate "ghost" buttons or missing elements.

#### Symptoms
- "Add Card" button duplicated after a drag operation.
- Use of internal HIDs mixed with Sortable's DOM manipulation caused Vango to patch the wrong nodes.

#### Solution
Moved the "Add Card" button **outside** the `.cards-container` managed by SortableJS, preventing the library from interfering with it.

#### Recommendation
- **Isolation**: Guide developers to isolate interactive/hook-managed zones from static Vango components.
- **Ignore Directive**: Future frameworks could support a `v-ignore` or `v-static` directive to tell Vango's diff engine to skip certain subtrees managed by external libraries.


----- NEW CODING SESSION CONTINUING THE KANBAN DEMO -----

### Issue 8: Eager Evaluation in VDOM Helpers Causes Nil Pointer Panics

**Severity**: High  
**Status**: ✅ **RESOLVED** (When/IfLazy/ShowWhen helpers + documentation)

#### Description
VDOM helpers like `If()`, `ClassIf()`, `Style()`, etc. are regular Go functions. Go evaluates **all function arguments before calling the function**, which means code inside an `If()` block executes even when the condition is false.

#### Symptoms
```go
// PANIC: card.DueDate is nil, but .Before() is called anyway!
If(card.DueDate != nil,
    Span(
        ClassIf(card.DueDate.Before(now()), "overdue"),  // ← Evaluated BEFORE If() runs
        Text(card.DueDate.Format("Jan 2")),              // ← Also panics
    ),
)
```

The developer expects the inner content to only evaluate when `card.DueDate != nil`, but Go's evaluation order means `card.DueDate.Before()` is called unconditionally, causing a nil pointer panic.

#### Root Cause
This is Go's semantics, not a Vango bug. However, the declarative VDOM API creates an expectation of lazy evaluation that Go doesn't provide.

#### Current Workaround
Precompute all values that depend on nullable pointers **before** the VNode construction:

```go
// ✅ CORRECT: Precompute outside VNode
hasDueDate := card.DueDate != nil
isOverdue := hasDueDate && card.DueDate.Before(now())
dueDateStr := ""
if hasDueDate {
    dueDateStr = card.DueDate.Format("Jan 2")
}

return Div(
    If(hasDueDate,
        Span(ClassIf(isOverdue, "overdue"), Text(dueDateStr)),
    ),
)
```

#### Proposed DX Improvements

1. **Document in Common Pitfalls** (minimum) - Add to `00-common-pitfalls.md`

2. **Add Lazy Evaluation Helper** (recommended):
   ```go
   // IfFunc only evaluates the node function when condition is true
   func IfFunc(condition bool, nodeFunc func() *VNode) *VNode {
       if condition {
           return nodeFunc()
       }
       return nil
   }
   
   // Usage - the inner func() is only called when DueDate != nil
   IfFunc(card.DueDate != nil, func() *VNode {
       return Span(ClassIf(card.DueDate.Before(now()), "overdue"))
   })
   ```

3. **Better Panic Recovery** (nice to have):
   - Wrap `ComponentInstance.Render()` in `recover()`
   - Log which component and approximate line caused the panic
   - Return an error VNode instead of crashing the session

#### Recommendation
- **Immediate**: Document this in `00-common-pitfalls.md` under "Nullable Pointer Access in If()"
- **Future**: Add `IfFunc()` lazy variant to `pkg/vdom/helpers.go`