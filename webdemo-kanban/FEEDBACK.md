# Vango Framework Feedback & Learnings

From building the Collaborative Kanban Board demo application.

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
**Status**: Expected behavior, needs documentation

### Description
When using `<a href="">` tags, the thin client sends `Navigate` events to HID "nav", but without Vango's file-based router configured, there's no handler registered.

### Symptoms
- `WARN handler not found ... hid=nav type=Navigate` in logs
- Links don't navigate

### Workaround
Use `<button OnClick={...}>` instead of `<a href>` for in-app navigation, manually updating a path Signal.

### Proposed Fix
- Document that `<a>` tags require the router package
- OR: Provide a simple navigation helper that doesn't require full router

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

| Area | Issue | Suggestion |
|------|-------|------------|
| Developer Experience | Hydration failures are silent | Add dev-mode warnings when HID mismatch detected |
| Documentation | No guidance on SSR+WS alignment | Add "Common Pitfalls" section |
| Error Messages | "handler not found" doesn't explain why | Include expected vs actual HID |
| Tooling | No hydration debugger | Browser devtools extension showing HID mappings |

### Missing Features for Production Apps
1. **URL State Sync** - Browser URL doesn't update during SPA navigation
2. **Session Persistence** - State lost on page reload
3. **Auth Integration** - No standard pattern for authenticated sessions
4. **Error Boundaries** - Panics in handlers crash entire session

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
