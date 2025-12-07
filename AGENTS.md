# CLAUDE.md

This file provides guidance to Claude Code when working with the Vango framework.

---

## What is Vango?

**Vango is a server-driven web framework for Go**, inspired by Phoenix LiveView (Elixir) and Laravel Livewire (PHP). The core innovation is that components run on the server by default, and UI updates flow as binary patches over WebSocket to a thin ~12KB JavaScript client.

### Key Architecture Principles

1. **Server-First**: Components execute on the server with direct database access
2. **Binary Protocol**: Minimal bandwidth via varint-encoded events and patches
3. **Reactive Signals**: Fine-grained reactivity (like SolidJS, not React)
4. **Progressive Enhancement**: Works without JS, enhanced with WebSocket
5. **Type Safety**: Go's compiler catches errors at build time

### How It Works

```
User clicks button → Thin client captures event → Binary event sent via WebSocket →
Server finds handler → Handler updates signals → Component re-renders →
Diff algorithm generates patches → Binary patches sent to client →
Client applies patches to DOM → User sees update (~50-100ms total)
```

---

## Repository Structure

```
/Users/collinshill/Documents/vango/
├── VANGO_ARCHITECTURE_AND_GUIDE.md   # THE authoritative spec (5800+ lines)
├── CLAUDE.md                          # This file
├── vango/                             # V1 implementation (reference only, was never launched)
└── vango_v2/                          # V2 implementation (active development)
    └── docs/
        ├── BUILD_ROADMAP.md           # Phases, milestones, dependencies
        ├── PHASE_01_CORE.md           # Reactive system (Signal, Memo, Effect)
        ├── PHASE_02_VDOM.md           # Virtual DOM and diffing
        ├── PHASE_03_PROTOCOL.md       # Binary wire protocol
        ├── PHASE_04_RUNTIME.md        # Server session management
        ├── PHASE_05_CLIENT.md         # Thin JavaScript client
        ├── PHASE_06_SSR.md            # Server-side rendering
        ├── PHASE_07_ROUTING.md        # File-based routing
        ├── PHASE_08_FEATURES.md       # Forms, Resources, Hooks, etc.
        └── PHASE_09_DX.md             # CLI, hot reload, error messages
```

---

## Key Documentation References

### Primary Specification
- **`VANGO_ARCHITECTURE_AND_GUIDE.md`** - The complete framework specification
  - Section 3.9: Frontend API Reference (all elements, attributes, events)
  - Section 4: Server-Driven Runtime
  - Section 5: Thin Client
  - Section 7: State Management
  - Section 8: Interaction Primitives (Client Hooks)
  - Section 21: Binary Protocol Specification

### Build Documentation (vango_v2/docs/)
- **BUILD_ROADMAP.md** - Overview of all phases and dependencies
- **PHASE_XX_*.md** - Detailed specifications for each subsystem

### V1 Reference (vango/vango/)
- V1 is reference only - do NOT modify
- Key learnings documented in the Phase docs
- Useful for understanding existing patterns

---

## Development Rules

### 1. Maximum Reasoning
Always seek a thorough understanding before writing code. Read relevant documentation, explore existing patterns, understand the full context.

### 2. No Simplified or Placeholder Code
Every line of code should be production-ready. No `// TODO: implement this`, no stub functions, no shortcuts. If something is complex, implement it completely.

### 3. Take Your Time
We are never in a rush. Spend hours understanding a problem before solving it. When debugging, don't witch-hunt for solutions—build deep understanding first.

### 4. Document As You Go
Update documentation when making changes. Explain decisions. Future sessions need to understand what was done and why.

### 5. Reference Documentation Constantly
Before implementing anything, check:
1. `VANGO_ARCHITECTURE_AND_GUIDE.md` for the spec
2. The relevant `PHASE_XX_*.md` for implementation details
3. V1 code for existing patterns (if applicable)

---

## Testing Philosophy

Tests are critical for a production open-source framework, but:

1. **Tests serve the code, not vice versa** - Never modify source code just to pass a test
2. **No hardcoded test values** - Tests should verify behavior, not specific outputs
3. **No placeholder tests** - Every test should provide real verification value
4. **Test the contract** - Focus on public API behavior, not implementation details

---

## Building Vango V2

### Phase Dependencies

```
Phase 1: Reactive Core (Signal, Memo, Effect)
    └── Phase 2: Virtual DOM (VNode, Diff, Patch)
         └── Phase 3: Binary Protocol (Events, Patches)
              └── Phase 4: Server Runtime (Sessions, Handlers)
                   └── Phase 5: Thin Client (JavaScript)
                        └── Phase 6: SSR & Hydration
                             └── Phase 7: Routing
                                  └── Phase 8: Higher-Level Features
                                       └── Phase 9: Developer Experience
                                            └── Phase 10: Production Hardening
```

### Current Status
Check `vango_v2/docs/BUILD_ROADMAP.md` for current phase status and next steps.

### Implementation Approach

1. **Read the phase doc first** - Each phase has detailed specifications
2. **Build foundation before features** - Follow the dependency order
3. **Test incrementally** - Each phase should be fully tested before moving on
4. **Milestone checkpoints** - Verify end-to-end functionality at key points

---

## Common Patterns

### Component Structure
```go
func Counter(initial int) vango.Component {
    return vango.Func(func() *vango.VNode {
        count := vango.Signal(initial)

        return Div(Class("counter"),
            H1(Textf("Count: %d", count())),
            Button(OnClick(count.Inc), Text("+")),
        )
    })
}
```

### Element Creation (vango/el package)
```go
import . "vango/el"

Div(Class("card"), ID("main"),
    H1(Text("Title")),
    P(Text("Content")),
    Button(OnClick(handler), Text("Click")),
)
```

### Reactive State
```go
// Local signal (component-scoped)
count := vango.Signal(0)

// Read (subscribes component)
value := count()

// Write (triggers re-render)
count.Set(5)
count.Update(func(n int) int { return n + 1 })

// Derived state
doubled := vango.Memo(func() int { return count() * 2 })

// Side effects
vango.Effect(func() vango.Cleanup {
    fmt.Println("Count changed:", count())
    return func() { /* cleanup */ }
})
```

---

## When Starting a New Session

1. **Orient yourself**: Check `vango_v2/docs/BUILD_ROADMAP.md` for current status
2. **Read relevant phase doc**: Before working on any subsystem
3. **Check for existing code**: Look at what's already implemented in `vango_v2/`
4. **Understand the spec**: Reference `VANGO_ARCHITECTURE_AND_GUIDE.md` for expected behavior

---

## Important Context

- **28 companies** are waiting to use Vango - quality is paramount
- **Production open-source** - This will be used by real developers
- **V1 exists** but V2 is a ground-up rewrite with lessons learned
- **Server-driven is the default** - WASM mode is future/optional

---

## Quick Command Reference

```bash
# Development
cd vango_v2
go test ./...
go build ./...

# Check phase docs
cat vango_v2/docs/BUILD_ROADMAP.md
cat vango_v2/docs/PHASE_01_CORE.md  # etc.

# Reference V1 (read-only)
ls vango/vango/pkg/
```

---

## Questions to Ask Yourself

Before implementing anything:
1. What does the spec say? (Check VANGO_ARCHITECTURE_AND_GUIDE.md)
2. What does the phase doc say? (Check PHASE_XX_*.md)
3. How did V1 handle this? (Check vango/vango/ if relevant)
4. Is this the simplest solution that fully solves the problem?
5. Will this work for all the use cases in the spec?

---

*Last Updated: 2024-12-06*
