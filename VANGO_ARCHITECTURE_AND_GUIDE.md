---
title: "Vango Architecture & Guide" #just web, we've built most of this.
slug: vango-architecture-guide
version: 2.0
status: RFC
---

# Vango Architecture & Guide

> **The Go Framework for Modern Web Applications**

---

## Table of Contents

1. [Vision & Philosophy](#1-vision--philosophy)
2. [Architecture Overview](#2-architecture-overview)
3. [Component Model](#3-component-model)
   - [3.9 Frontend API Reference](#39-frontend-api-reference) — Complete reference for all APIs:
     - Elements, Attributes, Event Handlers
     - Signal, Memo, Effect, Resource, Ref
     - Helpers (If, Range, Fragment, Key, Text)
     - Context, Form, URL State
4. [The Server-Driven Runtime](#4-the-server-driven-runtime)
5. [The Thin Client](#5-the-thin-client)
6. [The WASM Runtime](#6-the-wasm-runtime)
7. [State Management](#7-state-management)
8. [Interaction Primitives](#8-interaction-primitives)
   - [8.2 Client Hooks](#82-client-hooks) — 60fps interactions (drag-drop, sortable)
   - [8.4 Standard Hooks](#84-standard-hooks) — Sortable, Draggable, Tooltip, etc.
   - [8.5 Custom Hooks](#85-custom-hooks) — Define your own client behaviors
9. [Routing & Navigation](#9-routing--navigation)
10. [Data & APIs](#10-data--apis)
11. [Forms & Validation](#11-forms--validation)
   - [11.4 Toast Notifications](#114-toast-notifications)
   - [11.5 File Uploads](#115-file-uploads)
12. [JavaScript Islands](#12-javascript-islands)
13. [Styling](#13-styling)
14. [Performance & Scaling](#14-performance--scaling)
15. [Security](#15-security)
   - [14.6 Authentication & Middleware](#146-authentication--middleware)
16. [Testing](#16-testing)
17. [Developer Experience](#17-developer-experience)
18. [Migration Guide](#18-migration-guide)
19. [Examples](#19-examples)
20. [FAQ](#20-faq)
21. [Appendix: Protocol Specification](#21-appendix-protocol-specification)

---

## 1. Vision & Philosophy

### 1.1 The Problem with Modern Web Development

Modern web development is fragmented:

- **Two languages**: JavaScript on frontend, another language on backend
- **Two data models**: DTOs, JSON serialization, API contracts
- **Two state systems**: Client state, server state, synchronization hell
- **Heavy bundles**: 200KB+ of JavaScript before interactivity
- **Complex toolchains**: Webpack, Babel, TypeScript, bundlers, transpilers

What if we could write web applications in a single language, with direct access to the database, instant interactivity, and no bundle size concerns?

### 1.2 The Vango Approach

Vango is a **server-driven web framework** where:

1. **Components run on the server** by default
2. **UI updates flow as binary patches** over WebSocket
3. **The client is a thin renderer** (~12KB)
4. **You write Go everywhere** — no JavaScript required
5. **WASM is available** for offline or latency-sensitive features

This is similar to Phoenix LiveView (Elixir) or Laravel Livewire (PHP), but with Go's performance, type safety, and concurrency model.

### 1.3 Design Principles

| Principle | Meaning |
|-----------|---------|
| **Server-First** | Most code runs on the server. Client is minimal. |
| **One Language** | Go from database to DOM. No context switching. |
| **Type-Safe** | Compiler catches errors. No runtime surprises. |
| **Instant Interactive** | SSR means no waiting for bundles. |
| **Progressive Enhancement** | Works without JS, enhanced with WebSocket. |
| **Escape Hatches** | WASM and JS islands when you need them. |

### 1.4 When to Use Vango

**Ideal for:**
- CRUD applications (admin panels, dashboards)
- Collaborative apps (project management, documents)
- Data-heavy interfaces (analytics, reporting)
- Real-time features (chat, notifications, live updates)
- Internal tools (where Go backend teams own the frontend)

**Consider alternatives for:**
- Offline-first applications (use WASM mode or different framework)
- Extremely latency-sensitive UIs (drawing apps, games)
- Static content sites (use a static site generator)

---

## 2. Architecture Overview

### 2.1 The Three Modes

Vango supports three rendering modes. Choose based on your needs:

```
┌─────────────────────────────────────────────────────────────────┐
│                        VANGO MODES                              │
├─────────────────┬─────────────────┬─────────────────────────────┤
│  SERVER-DRIVEN  │     HYBRID      │           WASM              │
│   (Default)     │                 │                             │
├─────────────────┼─────────────────┼─────────────────────────────┤
│ Components run  │ Most on server, │ Components run in           │
│ on server       │ some on client  │ browser WASM                │
├─────────────────┼─────────────────┼─────────────────────────────┤
│ 12KB client     │ 12KB + partial  │ ~300KB WASM                 │
│                 │ WASM            │                             │
├─────────────────┼─────────────────┼─────────────────────────────┤
│ Requires        │ Requires        │ Works offline               │
│ connection      │ connection      │                             │
├─────────────────┼─────────────────┼─────────────────────────────┤
│ Best for: most  │ Best for: apps  │ Best for: offline,          │
│ web apps        │ with some       │ latency-critical            │
│                 │ latency needs   │                             │
└─────────────────┴─────────────────┴─────────────────────────────┘
```

### 2.2 Server-Driven Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                           BROWSER                                │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                    Thin Client (12KB)                      │  │
│  │  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐    │  │
│  │  │   Event     │  │   Patch      │  │   Optimistic    │    │  │
│  │  │   Capture   │──│   Applier    │──│   Updates       │    │  │
│  │  └─────────────┘  └──────────────┘  └─────────────────┘    │  │
│  └──────────────────────────┬─────────────────────────────────┘  │
│                             │ WebSocket (Binary)                 │
└─────────────────────────────┼────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────┴────────────────────────────────────┐
│                           SERVER                                 │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                    Vango Runtime                           │  │
│  │  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐    │  │
│  │  │   Session   │  │   Component  │  │   Diff          │    │  │
│  │  │   Manager   │──│   Tree       │──│   Engine        │    │  │
│  │  └─────────────┘  └──────────────┘  └─────────────────┘    │  │
│  │         │                │                   │             │  │
│  │         ▼                ▼                   ▼             │  │
│  │  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐    │  │
│  │  │   Signal    │  │   Event      │  │   Patch         │    │  │
│  │  │   Store     │  │   Router     │  │   Encoder       │    │  │
│  │  └─────────────┘  └──────────────┘  └─────────────────┘    │  │
│  └────────────────────────────────────────────────────────────┘  │
│                             │                                    │
│                             ▼                                    │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │              Direct Access (No HTTP/JSON)                  │  │
│  │  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐    │  │
│  │  │  Database   │  │    Cache     │  │   Services      │    │  │
│  │  └─────────────┘  └──────────────┘  └─────────────────┘    │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 2.3 Request Lifecycle

**Initial Page Load:**
```
1. Browser requests GET /projects/123
2. Server matches route → ProjectPage(id=123)
3. Component renders → VNode tree
4. VNode tree → HTML string (SSR)
5. HTML sent to browser (user sees content immediately)
6. Thin client JS loads (~12KB)
7. WebSocket connection established
8. Page is now interactive
```

**User Interaction:**
```
1. User clicks "Complete Task" button
2. Thin client captures click event
3. Binary event sent: {type: CLICK, hid: "h42"}
4. Server finds session, finds handler for hid="h42"
5. Handler runs: task.Complete()
6. Affected signals update, component re-renders
7. Diff: old VNode vs new VNode → patches
8. Binary patches sent: [{SET_TEXT, "h17", "✓ Done"}]
9. Thin client applies patches to DOM
10. User sees "✓ Done" (~50-80ms total)
```

### 2.4 Same Components, Different Modes

The same component code works in all modes:

```go
func Counter(initial int) vango.Component {
    return vango.Func(func() *vango.VNode {
        count := vango.Signal(initial)

        return Div(Class("counter"),
            H1(Textf("Count: %d", count())),
            Button(OnClick(count.Inc), Text("+")),
            Button(OnClick(count.Dec), Text("-")),
        )
    })
}
```

| Mode | Where `Signal` lives | Where `OnClick` runs | How DOM updates |
|------|---------------------|---------------------|-----------------|
| Server-Driven | Server memory | Server | Binary patches over WS |
| WASM | Browser WASM memory | Browser WASM | Direct DOM manipulation |
| Hybrid | Depends on component | Depends on component | Mixed |

---

## 3. Component Model

### 3.1 Core Concepts

Vango has five core concepts:

| Concept | Purpose | Example |
|---------|---------|---------|
| **Element** | UI structure | `Div(Class("card"), ...)` |
| **Signal** | Reactive state | `count := vango.Signal(0)` |
| **Memo** | Derived state | `doubled := vango.Memo(...)` |
| **Effect** | Side effects | `vango.Effect(func() {...})` |
| **Component** | Composition | `func Card() vango.Component` |

### 3.2 Elements

Elements are functions that accept mixed attributes and children:

```go
import (
    . "vango/el"  // Dot import for concise syntax
    "vango"
)

// Basic element
Div(Class("card"), Text("Hello"))

// Nested elements
Div(Class("container"),
    H1(Text("Title")),
    P(Class("subtitle"), Text("Description")),
    Button(OnClick(handleClick), Text("Click me")),
)

// Attributes and children can be mixed freely
Form(
    Method("POST"),
    Class("login-form"),
    Input(Type("email"), Name("email"), Placeholder("Email")),
    Input(Type("password"), Name("password")),
    Button(Type("submit"), Text("Login")),
)
```

**Why this syntax?**
- Pure Go — no custom parser, standard tooling works
- Type-safe — compiler catches errors
- Flexible — attributes and children intermix naturally
- Readable — structure mirrors HTML output

### 3.3 Signals

Signals are reactive values that trigger re-renders when changed:

```go
func Counter(initial int) vango.Component {
    return vango.Func(func() *vango.VNode {
        // Create a signal
        count := vango.Signal(initial)

        // Read the value (subscribes this component)
        currentValue := count()

        // Update the value (triggers re-render)
        increment := func() {
            count.Set(count() + 1)
        }

        return Div(
            Text(fmt.Sprintf("Count: %d", count())),
            Button(OnClick(increment), Text("+")),
        )
    })
}
```

**Signal API:**
```go
// Create
count := vango.Signal(0)           // Signal[int]
user := vango.Signal[*User](nil)   // Signal[*User]
items := vango.Signal([]Item{})    // Signal[[]Item]

// Read (subscribes component to changes)
value := count()

// Write
count.Set(5)
count.Update(func(n int) int { return n + 1 })

// Convenience methods
count.Inc()       // +1 (integers only)
count.Dec()       // -1 (integers only)
enabled.Toggle()  // !current (booleans only)
```

### 3.4 Memos

Memos are cached computations that update when dependencies change:

```go
func ShoppingCart() vango.Component {
    return vango.Func(func() *vango.VNode {
        items := vango.Signal([]CartItem{})
        taxRate := vango.Signal(0.08)

        // Recomputes only when items() changes
        subtotal := vango.Memo(func() float64 {
            total := 0.0
            for _, item := range items() {
                total += item.Price * float64(item.Qty)
            }
            return total
        })

        // Recomputes when subtotal() or taxRate() changes
        tax := vango.Memo(func() float64 {
            return subtotal() * taxRate()
        })

        // Memos can depend on other memos
        total := vango.Memo(func() float64 {
            return subtotal() + tax()
        })

        return Div(
            CartItems(items),
            Div(Class("totals"),
                Row("Subtotal", subtotal()),
                Row("Tax", tax()),
                Row("Total", total()),
            ),
        )
    })
}
```

### 3.5 Effects

Effects run after render and handle side effects:

```go
func UserProfile(userID int) vango.Component {
    return vango.Func(func() *vango.VNode {
        user := vango.Signal[*User](nil)
        loading := vango.Signal(true)

        // Effect runs after mount, and when dependencies change
        vango.Effect(func() vango.Cleanup {
            loading.Set(true)

            // Direct database access! (server-driven mode)
            u, err := db.Users.FindByID(userID)
            if err != nil {
                // Handle error...
            }

            user.Set(u)
            loading.Set(false)

            // Optional cleanup function
            return func() {
                // Runs before next effect or unmount
            }
        })

        if loading() {
            return LoadingSpinner()
        }

        return Div(
            H1(Text(user().Name)),
            P(Text(user().Email)),
        )
    })
}
```

**Effect Timing:**
| When | What Happens |
|------|--------------|
| After first render | Effect runs |
| Signal dependency changes | Effect re-runs (after cleanup) |
| Component unmounts | Cleanup runs |

### 3.6 Component Types

**Stateless Components** — Pure functions returning VNodes:
```go
func Greeting(name string) *vango.VNode {
    return H1(Class("greeting"), Textf("Hello, %s!", name))
}

// Usage
Div(
    Greeting("Alice"),
    Greeting("Bob"),
)
```

**Stateful Components** — Functions returning `vango.Component`:
```go
func Counter(initial int) vango.Component {
    return vango.Func(func() *vango.VNode {
        count := vango.Signal(initial)
        return Div(
            Text(fmt.Sprintf("%d", count())),
            Button(OnClick(count.Inc), Text("+")),
        )
    })
}

// Usage
Div(
    Counter(0),
    Counter(100),
)
```

**Components with Children:**
```go
func Card(title string, children ...any) *vango.VNode {
    return Div(Class("card"),
        H2(Class("card-title"), Text(title)),
        Div(Class("card-body"), children...),
    )
}

// Usage
Card("Settings",
    Form(
        Input(Type("text"), Name("name")),
        Button(Type("submit"), Text("Save")),
    ),
)
```

### 3.7 Conditional Rendering

```go
// Simple conditional
If(isLoggedIn,
    UserMenu(user),
)

// If-else
IfElse(isLoggedIn,
    UserMenu(user),
    LoginButton(),
)

// Inline conditional (when you need the else to be nil)
func() *vango.VNode {
    if isLoggedIn {
        return UserMenu(user)
    }
    return nil
}()

// Switch-like patterns
func StatusBadge(status string) *vango.VNode {
    switch status {
    case "active":
        return Badge("success", "Active")
    case "pending":
        return Badge("warning", "Pending")
    default:
        return Badge("gray", "Unknown")
    }
}
```

### 3.8 List Rendering

```go
// With keys (required for efficient updates)
func TaskList(tasks []Task) *vango.VNode {
    return Ul(Class("task-list"),
        Range(tasks, func(task Task, i int) *vango.VNode {
            return Li(
                Key(task.ID),  // Stable key for reconciliation
                Class("task"),
                Text(task.Title),
            )
        }),
    )
}

// Without Range helper (manual approach)
func TaskList(tasks []Task) *vango.VNode {
    items := make([]any, len(tasks))
    for i, task := range tasks {
        items[i] = Li(Key(task.ID), Text(task.Title))
    }
    return Ul(items...)
}
```

---

## 3.9 Frontend API Reference

This section provides a complete reference for all Vango frontend APIs. For quick lookup, use the following categories:

- [3.9.1 HTML Elements](#391-html-elements)
- [3.9.2 Attributes](#392-attributes)
- [3.9.3 Event Handlers](#393-event-handlers)
- [3.9.4 Signal API](#394-signal-api)
- [3.9.5 Memo API](#395-memo-api)
- [3.9.6 Effect API](#396-effect-api)
- [3.9.7 Resource API](#397-resource-api)
- [3.9.8 Ref API](#398-ref-api)
- [3.9.9 Helper Functions](#399-helper-functions)
- [3.9.10 Context API](#3910-context-api)

---

### 3.9.1 HTML Elements

All standard HTML elements are available as functions. Import with dot notation for concise syntax:

```go
import . "vango/el"
```

#### Element Signatures

Every element function accepts variadic `any` arguments that can be:
- **Attributes**: `Class("card")`, `ID("main")`, `Style("color: red")`
- **Event handlers**: `OnClick(fn)`, `OnInput(fn)`
- **Children**: Other elements, `Text("...")`, `*vango.VNode`, `vango.Component`
- **nil**: Safely ignored

```go
func Div(args ...any) *vango.VNode
func Span(args ...any) *vango.VNode
func Button(args ...any) *vango.VNode
// ... all HTML elements follow this pattern
```

#### Document Structure

| Element | Description | Common Attributes |
|---------|-------------|-------------------|
| `Html()` | Root element | `Lang("en")` |
| `Head()` | Document head | - |
| `Body()` | Document body | `Class()` |
| `Title()` | Page title | - |
| `Meta()` | Metadata | `Name()`, `Content()`, `Charset()` |
| `Link()` | External resource | `Rel()`, `Href()`, `Type()` |
| `Script()` | Script element | `Src()`, `Type()`, `Defer()`, `Async()` |
| `Style()` | Inline styles | `Type()` |

#### Content Sectioning

| Element | Description | Semantic Use |
|---------|-------------|--------------|
| `Header()` | Header section | Page/section header |
| `Footer()` | Footer section | Page/section footer |
| `Main()` | Main content | Primary page content |
| `Nav()` | Navigation | Navigation links |
| `Section()` | Generic section | Thematic content grouping |
| `Article()` | Article | Self-contained content |
| `Aside()` | Sidebar | Tangentially related content |
| `H1()` - `H6()` | Headings | Section headings |
| `Hgroup()` | Heading group | Multi-level heading |
| `Address()` | Contact info | Contact information |

#### Text Content

| Element | Description | Common Use |
|---------|-------------|------------|
| `Div()` | Generic container | Layout, grouping |
| `P()` | Paragraph | Text paragraphs |
| `Span()` | Inline container | Inline styling |
| `Pre()` | Preformatted | Code blocks |
| `Blockquote()` | Quote block | Quotations |
| `Ul()` | Unordered list | Bullet lists |
| `Ol()` | Ordered list | Numbered lists |
| `Li()` | List item | List items |
| `Dl()` | Description list | Term-description pairs |
| `Dt()` | Description term | Term in dl |
| `Dd()` | Description details | Description in dl |
| `Figure()` | Figure | Illustrations, diagrams |
| `Figcaption()` | Figure caption | Caption for figure |
| `Hr()` | Horizontal rule | Thematic break |

#### Inline Text

| Element | Description | Renders As |
|---------|-------------|------------|
| `A()` | Anchor/link | Hyperlink |
| `Strong()` | Strong importance | Bold |
| `Em()` | Emphasis | Italic |
| `B()` | Bring attention | Bold (no semantic) |
| `I()` | Alternate voice | Italic (no semantic) |
| `U()` | Unarticulated | Underline |
| `S()` | Strikethrough | Strikethrough |
| `Small()` | Side comment | Smaller text |
| `Mark()` | Highlight | Highlighted |
| `Sub()` | Subscript | Subscript |
| `Sup()` | Superscript | Superscript |
| `Code()` | Code | Monospace |
| `Kbd()` | Keyboard input | Key styling |
| `Samp()` | Sample output | Output styling |
| `Var()` | Variable | Variable styling |
| `Abbr()` | Abbreviation | With title tooltip |
| `Time()` | Time | Datetime value |
| `Br()` | Line break | Newline |
| `Wbr()` | Word break | Optional break |

#### Forms

| Element | Description | Key Attributes |
|---------|-------------|----------------|
| `Form()` | Form container | `Action()`, `Method()`, `OnSubmit()` |
| `Input()` | Input field | `Type()`, `Name()`, `Value()`, `Placeholder()` |
| `Textarea()` | Multi-line input | `Name()`, `Rows()`, `Cols()` |
| `Select()` | Dropdown | `Name()`, `Multiple()` |
| `Option()` | Select option | `Value()`, `Selected()` |
| `Optgroup()` | Option group | `Label()` |
| `Button()` | Button | `Type()`, `Disabled()` |
| `Label()` | Form label | `For()` |
| `Fieldset()` | Field group | - |
| `Legend()` | Fieldset legend | - |
| `Datalist()` | Input suggestions | `ID()` |
| `Output()` | Calculation result | `Name()`, `For()` |
| `Progress()` | Progress bar | `Value()`, `Max()` |
| `Meter()` | Scalar measurement | `Value()`, `Min()`, `Max()` |

#### Tables

| Element | Description | Key Attributes |
|---------|-------------|----------------|
| `Table()` | Table container | - |
| `Thead()` | Table header | - |
| `Tbody()` | Table body | - |
| `Tfoot()` | Table footer | - |
| `Tr()` | Table row | - |
| `Th()` | Header cell | `Scope()`, `Colspan()`, `Rowspan()` |
| `Td()` | Data cell | `Colspan()`, `Rowspan()` |
| `Caption()` | Table caption | - |
| `Colgroup()` | Column group | - |
| `Col()` | Column | `Span()` |

#### Media

| Element | Description | Key Attributes |
|---------|-------------|----------------|
| `Img()` | Image | `Src()`, `Alt()`, `Width()`, `Height()` |
| `Picture()` | Responsive image | - |
| `Source()` | Media source | `Srcset()`, `Media()`, `Type()` |
| `Video()` | Video | `Src()`, `Controls()`, `Autoplay()` |
| `Audio()` | Audio | `Src()`, `Controls()` |
| `Track()` | Text track | `Src()`, `Kind()`, `Srclang()` |
| `Iframe()` | Inline frame | `Src()`, `Width()`, `Height()` |
| `Embed()` | External content | `Src()`, `Type()` |
| `Object()` | External object | `Data()`, `Type()` |
| `Canvas()` | Drawing canvas | `Width()`, `Height()` |
| `Svg()` | SVG container | `Viewbox()`, `Width()`, `Height()` |

#### Interactive

| Element | Description | Key Attributes |
|---------|-------------|----------------|
| `Details()` | Disclosure widget | `Open()` |
| `Summary()` | Details summary | - |
| `Dialog()` | Dialog box | `Open()` |
| `Menu()` | Menu container | - |

---

### 3.9.2 Attributes

Attributes are functions that return attribute values. They can be mixed with children in element calls.

#### Global Attributes

These work on any HTML element:

```go
// Identity
ID("main-content")          // id="main-content"
Class("card", "active")     // class="card active"
Style("color: red")         // style="color: red"

// Data attributes
Data("id", "123")           // data-id="123"
Data("user-role", "admin")  // data-user-role="admin"

// Accessibility
Role("button")              // role="button"
AriaLabel("Close")          // aria-label="Close"
AriaHidden(true)            // aria-hidden="true"
AriaExpanded(false)         // aria-expanded="false"
AriaDescribedBy("desc")     // aria-describedby="desc"
AriaLabelledBy("title")     // aria-labelledby="title"
AriaLive("polite")          // aria-live="polite"
AriaAtomic(true)            // aria-atomic="true"
AriaBusy(false)             // aria-busy="false"
AriaControls("menu")        // aria-controls="menu"
AriaCurrent("page")         // aria-current="page"
AriaDisabled(true)          // aria-disabled="true"
AriaHasPopup("menu")        // aria-haspopup="menu"
AriaPressed("true")         // aria-pressed="true"
AriaSelected(true)          // aria-selected="true"

// Keyboard
TabIndex(0)                 // tabindex="0"
TabIndex(-1)                // tabindex="-1"
AccessKey("s")              // accesskey="s"

// Visibility
Hidden()                    // hidden
Title("Tooltip text")       // title="Tooltip text"

// Behavior
ContentEditable(true)       // contenteditable="true"
Draggable()                 // draggable="true"
Spellcheck(false)           // spellcheck="false"
Translate(false)            // translate="no"

// Language/Direction
Lang("en")                  // lang="en"
Dir("ltr")                  // dir="ltr"
```

#### Link Attributes

```go
// Anchor
Href("/users")              // href="/users"
Href(router.User(123))      // href="/users/123" (type-safe)
Target("_blank")            // target="_blank"
Rel("noopener")             // rel="noopener"
Download()                  // download
Download("file.pdf")        // download="file.pdf"
Hreflang("en")              // hreflang="en"
Ping("/track")              // ping="/track"
ReferrerPolicy("origin")    // referrerpolicy="origin"
```

#### Form Input Attributes

```go
// Common
Name("email")               // name="email"
Value("hello")              // value="hello"
Type("email")               // type="email"
Placeholder("Enter email")  // placeholder="Enter email"

// Input Types
Type("text")                // type="text"
Type("password")            // type="password"
Type("email")               // type="email"
Type("number")              // type="number"
Type("tel")                 // type="tel"
Type("url")                 // type="url"
Type("search")              // type="search"
Type("date")                // type="date"
Type("time")                // type="time"
Type("datetime-local")      // type="datetime-local"
Type("month")               // type="month"
Type("week")                // type="week"
Type("color")               // type="color"
Type("file")                // type="file"
Type("hidden")              // type="hidden"
Type("checkbox")            // type="checkbox"
Type("radio")               // type="radio"
Type("range")               // type="range"
Type("submit")              // type="submit"
Type("reset")               // type="reset"
Type("button")              // type="button"
Type("image")               // type="image"

// States
Disabled()                  // disabled
Readonly()                  // readonly
Required()                  // required
Checked()                   // checked
Selected()                  // selected
Multiple()                  // multiple
Autofocus()                 // autofocus
Autocomplete("email")       // autocomplete="email"

// Validation
Pattern(`[0-9]+`)           // pattern="[0-9]+"
MinLength(3)                // minlength="3"
MaxLength(100)              // maxlength="100"
Min("0")                    // min="0"
Max("100")                  // max="100"
Step("0.01")                // step="0.01"

// Files
Accept("image/*")           // accept="image/*"
Capture("user")             // capture="user"

// Text areas
Rows(5)                     // rows="5"
Cols(40)                    // cols="40"
Wrap("soft")                // wrap="soft"
```

#### Form Attributes

```go
// Form element
Action("/submit")           // action="/submit"
Method("POST")              // method="POST"
Enctype("multipart/form-data")  // enctype="..."
Novalidate()                // novalidate
Autocomplete("off")         // autocomplete="off"

// Label
For("input-id")             // for="input-id"

// Button
FormAction("/other")        // formaction="/other"
FormMethod("GET")           // formmethod="GET"
FormNovalidate()            // formnovalidate
```

#### Media Attributes

```go
// Image
Src("/img/photo.jpg")       // src="/img/photo.jpg"
Alt("Description")          // alt="Description"
Width(300)                  // width="300"
Height(200)                 // height="200"
Loading("lazy")             // loading="lazy"
Decoding("async")           // decoding="async"
Srcset("...")               // srcset="..."
Sizes("...")                // sizes="..."

// Video/Audio
Controls()                  // controls
Autoplay()                  // autoplay
Loop()                      // loop
Muted()                     // muted
Preload("metadata")         // preload="metadata"
Poster("/poster.jpg")       // poster="/poster.jpg"
Playsinline()               // playsinline

// Iframe
Sandbox("allow-scripts")    // sandbox="allow-scripts"
Allow("fullscreen")         // allow="fullscreen"
Allowfullscreen()           // allowfullscreen
```

#### Table Attributes

```go
Colspan(2)                  // colspan="2"
Rowspan(3)                  // rowspan="3"
Scope("col")                // scope="col"
Headers("h1 h2")            // headers="h1 h2"
```

#### Miscellaneous Attributes

```go
// Lists
Start(5)                    // start="5" (ol)
Reversed()                  // reversed (ol)

// Details
Open()                      // open

// Meta
Charset("utf-8")            // charset="utf-8"
Content("...")              // content="..."
HttpEquiv("refresh")        // http-equiv="refresh"

// Link
Rel("stylesheet")           // rel="stylesheet"
As("style")                 // as="style"
Crossorigin("anonymous")    // crossorigin="anonymous"
Integrity("sha384-...")     // integrity="..."

// Custom/raw attribute
Attr("x-custom", "value")   // x-custom="value"
```

---

### 3.9.3 Event Handlers

Event handlers trigger server-side callbacks or client-side behavior.

#### Mouse Events

```go
// Click
OnClick(func() {
    // Handle click
})

OnClick(func(e vango.MouseEvent) {
    // Access event details
    fmt.Println(e.ClientX, e.ClientY)
})

// Double click
OnDblClick(func() { })

// Mouse buttons
OnMouseDown(func(e vango.MouseEvent) { })
OnMouseUp(func(e vango.MouseEvent) { })

// Mouse movement
OnMouseMove(func(e vango.MouseEvent) { })
OnMouseEnter(func() { })
OnMouseLeave(func() { })
OnMouseOver(func() { })
OnMouseOut(func() { })

// Context menu
OnContextMenu(func(e vango.MouseEvent) { })

// Wheel
OnWheel(func(e vango.WheelEvent) { })
```

**MouseEvent:**
```go
type MouseEvent struct {
    ClientX   int     // X relative to viewport
    ClientY   int     // Y relative to viewport
    PageX     int     // X relative to document
    PageY     int     // Y relative to document
    OffsetX   int     // X relative to target
    OffsetY   int     // Y relative to target
    Button    int     // 0=left, 1=middle, 2=right
    Buttons   int     // Bitmask of pressed buttons
    CtrlKey   bool    // Ctrl held
    ShiftKey  bool    // Shift held
    AltKey    bool    // Alt held
    MetaKey   bool    // Meta/Cmd held
}
```

#### Keyboard Events

```go
OnKeyDown(func(e vango.KeyboardEvent) {
    if e.Key == "Enter" && !e.ShiftKey {
        submit()
    }
})

OnKeyUp(func(e vango.KeyboardEvent) { })

OnKeyPress(func(e vango.KeyboardEvent) { })  // Deprecated, use KeyDown
```

**KeyboardEvent:**
```go
type KeyboardEvent struct {
    Key       string  // "Enter", "a", "Escape", etc.
    Code      string  // "Enter", "KeyA", "Escape", etc.
    CtrlKey   bool
    ShiftKey  bool
    AltKey    bool
    MetaKey   bool
    Repeat    bool    // True if key is held down
    Location  int     // 0=standard, 1=left, 2=right, 3=numpad
}
```

**Common Key Values:**
| Key Constant | Value |
|--------------|-------|
| `vango.KeyEnter` | "Enter" |
| `vango.KeyEscape` | "Escape" |
| `vango.KeySpace` | " " |
| `vango.KeyTab` | "Tab" |
| `vango.KeyBackspace` | "Backspace" |
| `vango.KeyDelete` | "Delete" |
| `vango.KeyArrowUp` | "ArrowUp" |
| `vango.KeyArrowDown` | "ArrowDown" |
| `vango.KeyArrowLeft` | "ArrowLeft" |
| `vango.KeyArrowRight` | "ArrowRight" |
| `vango.KeyHome` | "Home" |
| `vango.KeyEnd` | "End" |
| `vango.KeyPageUp` | "PageUp" |
| `vango.KeyPageDown` | "PageDown" |

#### Form Events

```go
// Input changes (fires on each keystroke)
OnInput(func(value string) {
    searchQuery.Set(value)
})

OnInput(func(e vango.InputEvent) {
    fmt.Println(e.Value)
})

// Change (fires on blur/commit)
OnChange(func(value string) {
    filter.Set(value)
})

// Form submission
OnSubmit(func(data vango.FormData) {
    email := data.Get("email")
    password := data.Get("password")
    handleLogin(email, password)
})

// Prevent default (the callback returning is implicit prevention)
// To explicitly prevent:
OnSubmit(func(data vango.FormData) {
    // Submitting via Vango already prevents browser default
})

// Focus
OnFocus(func() { })
OnBlur(func() { })
OnFocusIn(func() { })
OnFocusOut(func() { })

// Selection
OnSelect(func() { })

// Invalid (form validation)
OnInvalid(func() { })

// Reset
OnReset(func() { })
```

**FormData:**
```go
type FormData struct {
    values map[string][]string
}

func (f FormData) Get(key string) string          // First value
func (f FormData) GetAll(key string) []string     // All values
func (f FormData) Has(key string) bool            // Key exists
func (f FormData) Keys() []string                 // All keys
```

#### Drag Events

```go
// Draggable element
OnDragStart(func(e vango.DragEvent) {
    e.SetData("text/plain", item.ID)
})
OnDrag(func(e vango.DragEvent) { })
OnDragEnd(func(e vango.DragEvent) { })

// Drop target
OnDragEnter(func(e vango.DragEvent) { })
OnDragOver(func(e vango.DragEvent) bool {
    return true  // Allow drop
})
OnDragLeave(func(e vango.DragEvent) { })
OnDrop(func(e vango.DropEvent) {
    data := e.GetData("text/plain")
})
```

#### Touch Events

```go
OnTouchStart(func(e vango.TouchEvent) { })
OnTouchMove(func(e vango.TouchEvent) { })
OnTouchEnd(func(e vango.TouchEvent) { })
OnTouchCancel(func(e vango.TouchEvent) { })
```

**TouchEvent:**
```go
type TouchEvent struct {
    Touches        []Touch  // All current touches
    TargetTouches  []Touch  // Touches on this element
    ChangedTouches []Touch  // Touches that changed
}

type Touch struct {
    Identifier int
    ClientX    int
    ClientY    int
    PageX      int
    PageY      int
}
```

#### Animation/Transition Events

```go
OnAnimationStart(func(e vango.AnimationEvent) { })
OnAnimationEnd(func(e vango.AnimationEvent) { })
OnAnimationIteration(func(e vango.AnimationEvent) { })
OnAnimationCancel(func(e vango.AnimationEvent) { })

OnTransitionStart(func(e vango.TransitionEvent) { })
OnTransitionEnd(func(e vango.TransitionEvent) { })
OnTransitionRun(func(e vango.TransitionEvent) { })
OnTransitionCancel(func(e vango.TransitionEvent) { })
```

#### Media Events

```go
OnPlay(func() { })
OnPause(func() { })
OnEnded(func() { })
OnTimeUpdate(func(currentTime float64) { })
OnDurationChange(func(duration float64) { })
OnVolumeChange(func(volume float64, muted bool) { })
OnSeeking(func() { })
OnSeeked(func() { })
OnLoadStart(func() { })
OnLoadedData(func() { })
OnLoadedMetadata(func() { })
OnCanPlay(func() { })
OnCanPlayThrough(func() { })
OnWaiting(func() { })
OnPlaying(func() { })
OnProgress(func() { })
OnStalled(func() { })
OnSuspend(func() { })
OnError(func(err error) { })
```

#### Scroll Events

```go
OnScroll(func(e vango.ScrollEvent) {
    fmt.Println(e.ScrollTop, e.ScrollLeft)
})

// Throttled version (recommended for performance)
OnScroll(vango.Throttle(100*time.Millisecond, func(e vango.ScrollEvent) {
    if e.ScrollTop > 100 {
        showBackToTop.Set(true)
    }
}))
```

#### Window/Document Events

These are used at the layout or page level:

```go
OnLoad(func() { })
OnUnload(func() { })
OnBeforeUnload(func() string {
    return "You have unsaved changes"  // Browser shows confirmation
})
OnResize(func(width, height int) { })
OnPopState(func(state any) { })
OnHashChange(func(oldURL, newURL string) { })
OnOnline(func() { })
OnOffline(func() { })
OnVisibilityChange(func(hidden bool) { })
```

#### Event Modifiers

Modify event behavior:

```go
// Prevent default browser behavior
OnClick(vango.PreventDefault(func() {
    // Click handled, default prevented
}))

// Stop event propagation
OnClick(vango.StopPropagation(func() {
    // Click won't bubble up
}))

// Both
OnClick(vango.PreventDefault(vango.StopPropagation(func() {
    // ...
})))

// Self-only (only fire if target is this element)
OnClick(vango.Self(func() {
    // Only fires if clicked element is this exact element
}))

// Once (remove after first trigger)
OnClick(vango.Once(func() {
    // Only fires once
}))

// Passive (for scroll performance)
OnScroll(vango.Passive(func(e vango.ScrollEvent) {
    // Cannot call preventDefault
}))

// Capture phase
OnClick(vango.Capture(func() {
    // Fires during capture phase
}))

// Debounce
OnInput(vango.Debounce(300*time.Millisecond, func(value string) {
    search(value)
}))

// Throttle
OnMouseMove(vango.Throttle(100*time.Millisecond, func(e vango.MouseEvent) {
    updatePosition(e.ClientX, e.ClientY)
}))

// Key modifiers
OnKeyDown(vango.Key("Enter", func() {
    submit()
}))

OnKeyDown(vango.Keys([]string{"Enter", "NumpadEnter"}, func() {
    submit()
}))

OnKeyDown(vango.KeyWithModifiers("s", vango.Ctrl, func() {
    save()  // Ctrl+S
}))

OnKeyDown(vango.KeyWithModifiers("s", vango.Ctrl|vango.Shift, func() {
    saveAs()  // Ctrl+Shift+S
}))
```

---

### 3.9.4 Signal API

Signals are reactive values that trigger re-renders when changed.

#### Creating Signals

```go
// Basic signal with initial value
count := vango.Signal(0)                    // Signal[int]
name := vango.Signal("Alice")               // Signal[string]
user := vango.Signal[*User](nil)            // Signal[*User] with nil
items := vango.Signal([]Item{})             // Signal[[]Item]
prefs := vango.Signal(Preferences{})        // Signal[Preferences]

// Session-scoped signal (shared within a user session)
var CartItems = vango.SharedSignal([]CartItem{})

// Global signal (shared across ALL sessions)
var OnlineUsers = vango.GlobalSignal([]User{})
```

#### Reading Values

```go
// Call the signal to get current value
// This also subscribes the current component to changes
currentCount := count()
userName := name()

// Read without subscribing (rarely needed)
value := count.Peek()
```

#### Writing Values

```go
// Set new value
count.Set(5)
name.Set("Bob")

// Update with function
count.Update(func(n int) int {
    return n + 1
})

// For structs, use Update to avoid mutation
user.Update(func(u *User) *User {
    return &User{
        ID:   u.ID,
        Name: newName,
        Age:  u.Age,
    }
})
```

#### Convenience Methods

For numeric signals:
```go
count.Inc()              // Increment by 1
count.Dec()              // Decrement by 1
count.Add(5)             // Add value
count.Sub(3)             // Subtract value
count.Mul(2)             // Multiply
count.Div(2)             // Divide
```

For boolean signals:
```go
visible.Toggle()         // Flip true/false
visible.SetTrue()        // Set to true
visible.SetFalse()       // Set to false
```

For string signals:
```go
text.Append(" world")    // Append string
text.Prepend("Hello ")   // Prepend string
text.Clear()             // Set to ""
```

For slice signals:
```go
items.Append(newItem)                           // Add to end
items.Prepend(newItem)                          // Add to start
items.InsertAt(2, newItem)                      // Insert at index
items.RemoveAt(0)                               // Remove by index
items.RemoveLast()                              // Remove last
items.RemoveFirst()                             // Remove first
items.RemoveWhere(func(i Item) bool {           // Remove matching
    return i.Done
})
items.UpdateAt(0, func(i Item) Item {           // Update at index
    return Item{...i, Done: true}
})
items.UpdateWhere(                              // Update matching
    func(i Item) bool { return i.ID == id },
    func(i Item) Item { return Item{...i, Done: true} },
)
items.Clear()                                   // Remove all
items.SetAt(0, newItem)                         // Replace at index
```

For map signals:
```go
users.SetKey("123", user)                       // Set key
users.RemoveKey("123")                          // Remove key
users.UpdateKey("123", func(u User) User {      // Update key
    return User{...u, LastSeen: time.Now()}
})
users.HasKey("123")                             // Check key exists
users.Clear()                                   // Remove all
```

#### Signal Metadata

```go
// Check if signal has been modified
if count.IsDirty() {
    // Signal changed since last render
}

// Get subscriber count (debugging)
fmt.Println(count.SubscriberCount())

// Named signals (for debugging)
count := vango.Signal(0).Named("counter")
```

#### Persistence

```go
// Browser session storage (cleared when tab closes)
tabState := vango.Signal(State{}).Persist(vango.SessionStorage, "key")

// Browser local storage (persists across sessions)
prefs := vango.Signal(Prefs{}).Persist(vango.LocalStorage, "user-prefs")

// Server database (permanent, syncs across devices)
settings := vango.Signal(Settings{}).Persist(vango.Database, "settings:123")

// Custom persistence
data := vango.Signal(Data{}).Persist(vango.Custom(redisStore), "key")
```

#### Batching Updates

```go
// Multiple updates trigger single re-render
vango.Batch(func() {
    count.Set(5)
    name.Set("Bob")
    items.Append(newItem)
})
```

---

### 3.9.5 Memo API

Memos are cached computations that update when dependencies change.

#### Creating Memos

```go
// Basic memo
doubled := vango.Memo(func() int {
    return count() * 2  // Re-runs when count changes
})

// Memo depending on multiple signals
fullName := vango.Memo(func() string {
    return firstName() + " " + lastName()
})

// Session-scoped memo
var CartTotal = vango.SharedMemo(func() float64 {
    total := 0.0
    for _, item := range CartItems() {
        total += item.Price * float64(item.Qty)
    }
    return total
})

// Global memo
var ActiveUserCount = vango.GlobalMemo(func() int {
    return len(OnlineUsers())
})
```

#### Reading Memos

```go
// Call to get cached value
value := doubled()

// Read without subscribing
value := doubled.Peek()
```

#### Memo Chains

Memos can depend on other memos:

```go
var FilteredItems = vango.SharedMemo(func() []Item {
    return filterItems(Items(), Filter())
})

var SortedItems = vango.SharedMemo(func() []Item {
    return sortItems(FilteredItems(), SortOrder())
})

var PagedItems = vango.SharedMemo(func() []Item {
    items := SortedItems()
    start := (Page() - 1) * PageSize()
    end := min(start + PageSize(), len(items))
    return items[start:end]
})
```

#### Memo Options

```go
// Equality function (for complex types)
items := vango.Memo(func() []Item {
    return fetchItems()
}).Equals(func(a, b []Item) bool {
    return reflect.DeepEqual(a, b)
})

// Named memo (for debugging)
total := vango.Memo(func() float64 {
    return calculate()
}).Named("cart-total")
```

---

### 3.9.6 Effect API

Effects run after render and handle side effects.

#### Creating Effects

```go
// Basic effect
vango.Effect(func() vango.Cleanup {
    fmt.Println("Component mounted")

    return func() {
        fmt.Println("Component unmounting")
    }
})

// Effect with dependencies (runs when dependencies change)
vango.Effect(func() vango.Cleanup {
    fmt.Println("User changed to", userID())

    user, _ := db.Users.FindByID(userID())
    currentUser.Set(user)

    return nil  // No cleanup needed
})

// Effect that runs only once (on mount)
vango.OnMount(func() {
    analytics.TrackPageView()
})

// Effect that runs on unmount only
vango.OnUnmount(func() {
    cleanup()
})
```

#### Effect Timing

| Lifecycle | Function | When It Runs |
|-----------|----------|--------------|
| Mount | `vango.OnMount(fn)` | After first render |
| Update | `vango.OnUpdate(fn)` | After each re-render |
| Unmount | `vango.OnUnmount(fn)` | Before component removes |
| Effect | `vango.Effect(fn)` | After render, re-runs on dep change |

```go
func Timer() vango.Component {
    return vango.Func(func() *vango.VNode {
        elapsed := vango.Signal(0)

        vango.OnMount(func() {
            fmt.Println("Timer started")
        })

        vango.Effect(func() vango.Cleanup {
            ticker := time.NewTicker(1 * time.Second)
            go func() {
                for range ticker.C {
                    elapsed.Inc()
                }
            }()

            return func() {
                ticker.Stop()
            }
        })

        vango.OnUnmount(func() {
            fmt.Println("Timer stopped")
        })

        return Div(Textf("Elapsed: %d seconds", elapsed()))
    })
}
```

#### Effect Dependencies

Effects automatically track signal dependencies:

```go
vango.Effect(func() vango.Cleanup {
    // This effect re-runs when userID() changes
    user, err := fetchUser(userID())
    if err != nil {
        errorState.Set(err)
        return nil
    }
    userData.Set(user)
    return nil
})
```

#### Untracked Reads

To read a signal without creating a dependency:

```go
vango.Effect(func() vango.Cleanup {
    userId := userID()  // Tracked - effect re-runs on change

    vango.Untracked(func() {
        config := globalConfig()  // Not tracked
        // Effect won't re-run when globalConfig changes
    })

    return nil
})
```

---

### 3.9.7 Resource API

Resources handle async data loading with loading/error/success states.

#### Creating Resources

```go
// Basic resource
user := vango.Resource(func() (*User, error) {
    return db.Users.FindByID(userID)
})

// Resource with key (re-fetches when key changes)
user := vango.Resource(userID, func(id int) (*User, error) {
    return db.Users.FindByID(id)
})

// Multiple resources
type PageData struct {
    User     *User
    Projects []Project
}

data := vango.Resource(func() (PageData, error) {
    user, err := db.Users.FindByID(userID)
    if err != nil {
        return PageData{}, err
    }
    projects, err := db.Projects.FindByUser(userID)
    if err != nil {
        return PageData{}, err
    }
    return PageData{User: user, Projects: projects}, nil
})
```

#### Resource States

```go
type ResourceState int

const (
    vango.Pending ResourceState = iota  // Not started
    vango.Loading                       // In progress
    vango.Ready                         // Success
    vango.Error                         // Failed
)
```

#### Reading Resources

```go
// Pattern 1: Switch on state
switch user.State() {
case vango.Pending, vango.Loading:
    return LoadingSpinner()
case vango.Error:
    return ErrorMessage(user.Error())
case vango.Ready:
    return UserCard(user.Data())
}

// Pattern 2: Match helper
return user.Match(
    vango.OnLoading(func() *vango.VNode {
        return LoadingSpinner()
    }),
    vango.OnError(func(err error) *vango.VNode {
        return ErrorMessage(err)
    }),
    vango.OnReady(func(u *User) *vango.VNode {
        return UserCard(u)
    }),
)

// Pattern 3: With defaults
u := user.DataOr(&User{Name: "Loading..."})
return UserCard(u)
```

#### Resource Methods

```go
// Get current data (nil if not ready)
data := user.Data()

// Get data with default
data := user.DataOr(defaultValue)

// Get error (nil if no error)
err := user.Error()

// Get state
state := user.State()

// Manual refetch
user.Refetch()

// Mutate local data (optimistic update)
user.Mutate(func(u *User) *User {
    return &User{...u, Name: newName}
})

// Invalidate (marks stale, refetches)
user.Invalidate()
```

#### Resource Options

```go
// Initial data (show while loading)
user := vango.Resource(fetchUser).InitialData(cachedUser)

// Stale time (how long data is considered fresh)
user := vango.Resource(fetchUser).StaleTime(5 * time.Minute)

// Retry on error
user := vango.Resource(fetchUser).Retry(3, 1*time.Second)

// On success/error callbacks
user := vango.Resource(fetchUser).
    OnSuccess(func(u *User) {
        analytics.TrackUserLoad()
    }).
    OnError(func(err error) {
        logger.Error("Failed to load user", "error", err)
    })
```

---

### 3.9.8 Ref API

Refs provide direct access to DOM elements or component instances.

#### Creating Refs

```go
// DOM element ref
inputRef := vango.Ref[js.Value](nil)

// Use in component
Input(
    Ref(inputRef),
    Type("text"),
)

// Access after mount
vango.OnMount(func() {
    inputRef.Current().Call("focus")
})
```

#### Ref Methods

```go
// Get current value
el := inputRef.Current()

// Check if set
if inputRef.IsSet() {
    // ...
}
```

#### Forward Refs

```go
// Child component that exposes a ref
func FancyInput(ref *vango.Ref[js.Value]) *vango.VNode {
    return Input(
        Ref(ref),
        Class("fancy"),
    )
}

// Parent usage
inputRef := vango.Ref[js.Value](nil)
FancyInput(inputRef)

vango.OnMount(func() {
    inputRef.Current().Call("focus")
})
```

---

### 3.9.9 Helper Functions

#### Conditional Rendering

```go
// If: Render if condition true
If(isLoggedIn, UserMenu())

// IfElse: Render one or the other
IfElse(isLoggedIn, UserMenu(), LoginButton())

// When: Like If but lazy (closure)
When(isLoggedIn, func() *vango.VNode {
    return UserMenu()  // Only evaluated if true
})

// Unless: Render if condition false
Unless(isLoading, Content())

// Switch: Multiple conditions
Switch(status,
    Case("active", ActiveBadge()),
    Case("pending", PendingBadge()),
    Default(UnknownBadge()),
)
```

#### List Rendering

```go
// Range: Map slice to elements
Range(items, func(item Item, index int) *vango.VNode {
    return Li(Key(item.ID), Text(item.Name))
})

// RangeMap: Map map to elements
RangeMap(users, func(id string, user User) *vango.VNode {
    return Li(Key(id), Text(user.Name))
})

// Repeat: Render n times
Repeat(5, func(i int) *vango.VNode {
    return Star()
})
```

#### Fragment and Key

```go
// Fragment: Group without wrapper element
Fragment(
    Header(),
    Main(),
    Footer(),
)

// Key: Stable identity for reconciliation
Li(Key(item.ID), Text(item.Name))

// Key with multiple parts
Li(Key(item.Type, item.ID), Text(item.Name))
```

#### Text Helpers

```go
// Text: Static text node
Text("Hello, world!")

// Textf: Formatted text
Textf("Count: %d", count)

// Raw: Unescaped HTML (use carefully!)
Raw("<strong>Bold</strong>")
```

#### Slots and Children

```go
// Slot: Named slot placeholder
func Layout(slots map[string]*vango.VNode) *vango.VNode {
    return Div(
        Header(slots["header"]),
        Main(slots["content"]),
        Footer(slots["footer"]),
    )
}

// Usage
Layout(map[string]*vango.VNode{
    "header":  H1(Text("Title")),
    "content": ArticleContent(),
    "footer":  Copyright(),
})

// Children: Variadic children
func Card(title string, children ...any) *vango.VNode {
    return Div(Class("card"),
        H2(Text(title)),
        Div(Class("card-body"), children...),
    )
}
```

#### Null and Empty

```go
// Null: Explicit nothing
func MaybeShow() *vango.VNode {
    if !shouldShow {
        return vango.Null()
    }
    return Content()
}

// Empty: Empty fragment
vango.Empty()  // Renders nothing
```

---

### 3.9.10 Context API

Context provides dependency injection through the component tree.

#### Creating Context

```go
// Define context with default value
var ThemeContext = vango.CreateContext("light")

// Context with type
var UserContext = vango.CreateContext[*User](nil)
```

#### Providing Values

```go
func App() vango.Component {
    return vango.Func(func() *vango.VNode {
        theme := vango.Signal("dark")

        return ThemeContext.Provider(theme(),
            Header(),
            Main(),
            Footer(),
        )
    })
}
```

#### Consuming Values

```go
func Button() *vango.VNode {
    theme := ThemeContext.Use()  // "light" or "dark"

    return ButtonEl(
        Class("btn", theme+"-theme"),
        Text("Click"),
    )
}
```

#### Multiple Contexts

```go
func App() vango.Component {
    return vango.Func(func() *vango.VNode {
        user := getCurrentUser()
        theme := "dark"
        locale := "en"

        return UserContext.Provider(user,
            ThemeContext.Provider(theme,
                LocaleContext.Provider(locale,
                    Router(),
                ),
            ),
        )
    })
}
```

---

### 3.9.11 Form API

Vango provides a structured form system for complex forms with validation.

#### UseForm Hook

```go
// Define form structure
type ContactForm struct {
    Name    string `form:"name" validate:"required,min=2"`
    Email   string `form:"email" validate:"required,email"`
    Message string `form:"message" validate:"required,max=1000"`
}

func ContactPage() vango.Component {
    return vango.Func(func() *vango.VNode {
        form := vango.UseForm(ContactForm{})

        submit := func() {
            if !form.Validate() {
                return  // Errors shown automatically
            }
            sendEmail(form.Values())
            form.Reset()
        }

        return Form(OnSubmit(submit),
            form.Field("Name", Input(Type("text"))),
            form.Field("Email", Input(Type("email"))),
            form.Field("Message", Textarea()),
            Button(Type("submit"), Text("Send")),
        )
    })
}
```

#### Form Methods

```go
form := vango.UseForm(MyForm{})

// Read values
values := form.Values()           // MyForm struct
value := form.Get("fieldName")    // Single field value

// Write values
form.Set("fieldName", value)      // Set single field
form.SetValues(MyForm{...})       // Set all values
form.Reset()                      // Reset to initial

// Validation
isValid := form.Validate()        // Run all validators
errors := form.Errors()           // map[string][]string
fieldErrs := form.FieldErrors("name")  // []string
hasError := form.HasError("name") // bool
form.ClearErrors()                // Clear all errors

// State
form.IsDirty()                    // Has any field changed
form.FieldDirty("name")           // Has specific field changed
form.IsSubmitting()               // Currently submitting
form.SetSubmitting(true)          // Set submitting state
```

#### Field Method

```go
// form.Field returns a configured input with:
// - Value binding
// - Error display
// - Validation on blur

form.Field("Email",
    Input(Type("email"), Placeholder("you@example.com")),
    vango.Required("Email is required"),
    vango.Email("Invalid email format"),
    vango.MaxLength(100, "Email too long"),
)

// Renders as:
// <div class="field">
//   <input type="email" name="email" value="..." />
//   <span class="error">Invalid email format</span>
// </div>
```

#### Built-in Validators

```go
// String validators
vango.Required("Field is required")
vango.MinLength(n, "Too short")
vango.MaxLength(n, "Too long")
vango.Pattern(regex, "Invalid format")
vango.Email("Invalid email")
vango.URL("Invalid URL")
vango.UUID("Invalid UUID")

// Numeric validators
vango.Min(n, "Too small")
vango.Max(n, "Too large")
vango.Between(min, max, "Out of range")
vango.Positive("Must be positive")
vango.NonNegative("Must be non-negative")

// Comparison validators
vango.EqualTo("password", "Passwords must match")
vango.NotEqualTo("oldPassword", "Must be different")

// Date validators
vango.DateAfter(time, "Must be after...")
vango.DateBefore(time, "Must be before...")
vango.Future("Must be in the future")
vango.Past("Must be in the past")

// Custom validator
vango.Custom(func(value any) error {
    if !isUnique(value.(string)) {
        return errors.New("Already taken")
    }
    return nil
})

// Async validator
vango.Async(func(value any) (error, bool) {
    // Returns (error, isComplete)
    // isComplete=false while loading
    available := checkUsername(value.(string))
    if !available {
        return errors.New("Username taken"), true
    }
    return nil, true
})
```

#### Form Arrays

```go
type OrderForm struct {
    CustomerName string
    Items        []OrderItem
}

type OrderItem struct {
    ProductID int
    Quantity  int
}

func OrderFormPage() vango.Component {
    return vango.Func(func() *vango.VNode {
        form := vango.UseForm(OrderForm{})

        addItem := func() {
            form.AppendTo("Items", OrderItem{})
        }

        return Form(
            form.Field("CustomerName", Input(Type("text"))),

            // Render array items
            form.Array("Items", func(item vango.FormArrayItem, i int) *vango.VNode {
                return Div(Class("item-row"),
                    item.Field("ProductID", ProductSelect()),
                    item.Field("Quantity", Input(Type("number"))),
                    Button(OnClick(item.Remove), Text("Remove")),
                )
            }),

            Button(OnClick(addItem), Text("Add Item")),
            Button(Type("submit"), Text("Submit Order")),
        )
    })
}
```

---

### 3.9.12 URL State API

Synchronize component state with URL query parameters.

#### UseURLState Hook

```go
func ProductList() vango.Component {
    return vango.Func(func() *vango.VNode {
        // State is synced with URL: ?search=...&page=...&sort=...
        search := vango.UseURLState("search", "")
        page := vango.UseURLState("page", 1)
        sort := vango.UseURLState("sort", "newest")

        products := vango.Resource(func() ([]Product, error) {
            return db.Products.Search(search(), page(), sort())
        })

        return Div(
            // Search input updates URL
            Input(
                Type("search"),
                Value(search()),
                OnInput(search.Set),
                Placeholder("Search products..."),
            ),

            // Sort dropdown updates URL
            Select(
                Value(sort()),
                OnChange(sort.Set),
                Option(Value("newest"), Text("Newest")),
                Option(Value("price_asc"), Text("Price: Low to High")),
                Option(Value("price_desc"), Text("Price: High to Low")),
            ),

            // Product grid
            ProductGrid(products),

            // Pagination
            Pagination(page(), products.Meta().TotalPages, page.Set),
        )
    })
}
```

#### URLState Methods

```go
// Create URL state with default
search := vango.UseURLState("query", "")     // String
page := vango.UseURLState("page", 1)         // Int (auto-converts)
filters := vango.UseURLState("tags", []string{})  // Slice

// Read value
currentSearch := search()

// Set value (updates URL)
search.Set("new value")

// Set without pushing history (replaces current URL)
search.Replace("new value")

// Reset to default
search.Reset()

// Check if has non-default value
if search.IsSet() {
    // ...
}
```

#### URL State Options

```go
// Debounce updates
search := vango.UseURLState("q", "").Debounce(300 * time.Millisecond)

// Transform values
page := vango.UseURLState("p", 1).
    Serialize(func(p int) string { return fmt.Sprintf("%d", p) }).
    Deserialize(func(s string) int {
        n, _ := strconv.Atoi(s)
        return max(1, n)
    })

// Validation
page := vango.UseURLState("page", 1).
    Validate(func(p int) bool { return p >= 1 && p <= 100 })

// Multiple values (arrays)
tags := vango.UseURLState("tags", []string{}).Multi()
// URL: ?tags=go&tags=wasm&tags=vango
```

#### Navigation with State

```go
// Navigate with URL params
vango.Navigate("/products", vango.WithParams(map[string]any{
    "search": "laptop",
    "page":   1,
    "sort":   "price_asc",
}))
// Navigates to: /products?search=laptop&page=1&sort=price_asc

// Get current URL params
params := vango.URLParams()
search := params.Get("search")

// Preserve params during navigation
vango.Navigate("/products/123", vango.PreserveParams("search", "sort"))
```

#### Hash State (for modals, tabs)

```go
// Sync state with URL hash
activeTab := vango.UseHashState("settings")
// URL: /settings#billing

return Div(
    TabList(
        Tab("general", "General", activeTab),
        Tab("billing", "Billing", activeTab),
        Tab("security", "Security", activeTab),
    ),
    TabPanel(activeTab()),
)
```

---

## 4. The Server-Driven Runtime

### 4.1 Session Management

Each browser tab creates a WebSocket connection with its own session:

```go
// Internal session structure (simplified)
type Session struct {
    ID          string
    Conn        *websocket.Conn
    Signals     map[uint32]*SignalBase    // All signals for this session
    Components  map[uint32]*ComponentInst // Mounted component instances
    LastTree    *vdom.VNode               // For diffing
    Handlers    map[string]func()         // hid → handler
    CreatedAt   time.Time
    LastActive  time.Time
}
```

**Session Lifecycle:**
```
1. WebSocket handshake (validates CSRF, creates session)
2. Initial render (components mount, effects run)
3. Interaction loop (events → updates → patches)
4. Disconnect (cleanup, session evicted after timeout)
```

### 4.2 The Event Loop

```go
// Simplified server event loop (per session)
func (s *Session) eventLoop() {
    for {
        select {
        case event := <-s.events:
            // Find and run handler
            handler := s.Handlers[event.HID]
            if handler != nil {
                handler()
            }

            // Re-render affected components
            s.renderDirtyComponents()

            // Diff and send patches
            patches := vdom.Diff(s.LastTree, s.CurrentTree)
            s.sendPatches(patches)
            s.LastTree = s.CurrentTree

        case <-s.done:
            return
        }
    }
}
```

### 4.3 Binary Protocol Overview

The protocol is optimized for minimal bandwidth:

**Client → Server (Events):**
```
┌─────────┬──────────────┬─────────────────┐
│ Type    │ HID          │ Payload         │
│ 1 byte  │ varint       │ varies          │
└─────────┴──────────────┴─────────────────┘

Event Types:
  0x01: CLICK         (no payload)
  0x02: INPUT         (varint length + utf8 string)
  0x03: SUBMIT        (form data encoding)
  0x04: FOCUS         (no payload)
  0x05: BLUR          (no payload)
  0x06: KEYDOWN       (key code + modifiers)
  0x07: CUSTOM        (varint type + payload)
```

**Server → Client (Patches):**
```
┌─────────────┬───────────────────────────────┐
│ Patch Count │ Patches...                    │
│ varint      │                               │
└─────────────┴───────────────────────────────┘

Each Patch:
┌─────────┬──────────────┬─────────────────┐
│ Type    │ Target HID   │ Payload         │
│ 1 byte  │ varint       │ varies          │
└─────────┴──────────────┴─────────────────┘

Patch Types:
  0x01: SET_TEXT       (varint length + utf8 string)
  0x02: SET_ATTR       (key + value)
  0x03: REMOVE_ATTR    (key)
  0x04: INSERT_NODE    (index + encoded vnode)
  0x05: REMOVE_NODE    (no payload)
  0x06: MOVE_NODE      (new parent + index)
  0x07: REPLACE_NODE   (encoded vnode)
  0x08: SET_STYLE      (property + value)
```

### 4.4 Hydration IDs

Every interactive element gets a hydration ID during SSR:

```html
<!-- Server-rendered HTML -->
<div class="counter">
    <h1 data-hid="h1">Count: 5</h1>
    <button data-hid="h2">+</button>
    <button data-hid="h3">-</button>
</div>
```

The mapping is stored server-side:
```go
session.Handlers["h2"] = count.Inc  // + button
session.Handlers["h3"] = count.Dec  // - button
```

When the button is clicked, the client sends `{type: CLICK, hid: "h2"}`, and the server runs the mapped handler.

### 4.5 Component Mounting

```go
// When a route matches, the page component mounts
func (s *Session) mountPage(route Route, params Params) {
    // Create component instance
    component := route.Component(params)

    // Set up signal scope
    s.currentComponent = component.ID

    // Run the component function (creates signals, effects)
    vtree := component.Render()

    // Collect handlers
    s.collectHandlers(vtree)

    // Initial render to HTML (for SSR)
    html := s.renderToHTML(vtree)

    // Store for future diffs
    s.LastTree = vtree
}
```

---

## 5. The Thin Client

### 5.1 Responsibilities

The thin client (~12KB gzipped) handles:

1. **WebSocket Connection** — Connect, reconnect, heartbeat
2. **Event Capture** — Click, input, submit, keyboard, etc.
3. **Patch Application** — Apply DOM updates from server
4. **Optimistic Updates** — Optional client-side predictions

It does NOT handle:
- Component logic
- State management
- Routing decisions
- Data fetching

### 5.2 Core Implementation

```javascript
// Simplified thin client (~200 lines total)
class VangoClient {
    constructor() {
        this.ws = null;
        this.reconnectAttempts = 0;
        this.connect();
        this.attachEventListeners();
    }

    connect() {
        this.ws = new WebSocket(`wss://${location.host}/_vango/live`);
        this.ws.binaryType = 'arraybuffer';

        this.ws.onopen = () => {
            this.reconnectAttempts = 0;
            this.sendHandshake();
        };

        this.ws.onmessage = (e) => {
            this.handleMessage(new Uint8Array(e.data));
        };

        this.ws.onclose = () => {
            this.scheduleReconnect();
        };
    }

    attachEventListeners() {
        // Click events
        document.addEventListener('click', (e) => {
            const el = e.target.closest('[data-hid]');
            if (el) {
                e.preventDefault();
                this.sendEvent(0x01, el.dataset.hid);
            }
        });

        // Input events (debounced)
        document.addEventListener('input', debounce((e) => {
            const el = e.target.closest('[data-hid]');
            if (el) {
                this.sendEvent(0x02, el.dataset.hid, el.value);
            }
        }, 100));

        // Form submit
        document.addEventListener('submit', (e) => {
            const form = e.target.closest('[data-hid]');
            if (form) {
                e.preventDefault();
                this.sendEvent(0x03, form.dataset.hid, new FormData(form));
            }
        });
    }

    sendEvent(type, hid, payload) {
        const buffer = encodeEvent(type, hid, payload);
        this.ws.send(buffer);
    }

    handleMessage(buffer) {
        const patches = decodePatches(buffer);
        patches.forEach(patch => this.applyPatch(patch));
    }

    applyPatch(patch) {
        const el = document.querySelector(`[data-hid="${patch.hid}"]`);
        if (!el) return;

        switch (patch.type) {
            case 0x01: // SET_TEXT
                el.textContent = patch.text;
                break;
            case 0x02: // SET_ATTR
                el.setAttribute(patch.key, patch.value);
                break;
            case 0x03: // REMOVE_ATTR
                el.removeAttribute(patch.key);
                break;
            case 0x04: // INSERT_NODE
                const node = createNode(patch.vnode);
                el.insertBefore(node, el.childNodes[patch.index] || null);
                break;
            case 0x05: // REMOVE_NODE
                el.remove();
                break;
            // ... etc
        }
    }
}

// Initialize on DOM ready
new VangoClient();
```

### 5.3 Optimistic Updates

For instant feedback without waiting for server round-trip:

```go
// Server-side component
Button(
    OnClick(count.Inc),
    // Optimistic update hint
    Optimistic("#count", "textContent", func() string {
        return fmt.Sprintf("%d", count() + 1)
    }),
    Text("+"),
)
```

Rendered HTML:
```html
<button data-hid="h5" data-optimistic='{"target":"#count","prop":"textContent","expr":"+1"}'>
    +
</button>
```

Client behavior:
```javascript
document.addEventListener('click', (e) => {
    const el = e.target.closest('[data-hid]');
    if (el && el.dataset.optimistic) {
        // Apply optimistic update immediately
        const opt = JSON.parse(el.dataset.optimistic);
        applyOptimisticUpdate(opt);
    }
    // Still send to server for confirmation
    this.sendEvent(0x01, el.dataset.hid);
});
```

This gives 0ms perceived latency for common operations while maintaining server authority.

### 5.4 Reconnection Strategy

```javascript
scheduleReconnect() {
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
    this.reconnectAttempts++;

    setTimeout(() => {
        this.connect();
    }, delay);
}

// On reconnect, server sends full page state
// Client replaces content, no need for complex sync
```

---

## 6. The WASM Runtime

### 6.1 When to Use WASM Mode

Use WASM for:
- **Offline-first apps** — PWAs that must work without network
- **Latency-critical interactions** — Drawing, music production, games
- **Heavy client computation** — Image processing, data visualization
- **Specific components** — Within otherwise server-driven app

### 6.2 Enabling WASM Mode

**Full WASM mode** (entire app runs in browser):
```go
// vango.json
{
    "mode": "wasm"
}
```

**Hybrid mode** (specific components run client-side):
```go
// Mark a component as client-required
func DrawingCanvas() vango.Component {
    return vango.ClientRequired(func() *vango.VNode {
        // This code runs in WASM, not on server
        canvas := vango.Ref[js.Value](nil)

        vango.Effect(func() vango.Cleanup {
            ctx := canvas.Current().Call("getContext", "2d")
            // Set up drawing...
            return nil
        })

        return Canvas(
            Ref(canvas),
            Width(800),
            Height(600),
            OnMouseMove(handleDraw),
        )
    })
}
```

### 6.3 Hybrid Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        BROWSER                               │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                  Server-Driven UI                      │  │
│  │   ┌─────────┐  ┌─────────┐  ┌─────────────────────┐   │  │
│  │   │ Header  │  │ Sidebar │  │     Main Content    │   │  │
│  │   │ (12KB)  │  │ (12KB)  │  │       (12KB)        │   │  │
│  │   └─────────┘  └─────────┘  └──────────┬──────────┘   │  │
│  │                                        │               │  │
│  │   ┌────────────────────────────────────▼───────────┐  │  │
│  │   │         WASM Island (ClientRequired)           │  │  │
│  │   │   ┌─────────────────────────────────────────┐  │  │  │
│  │   │   │          DrawingCanvas (~50KB)          │  │  │  │
│  │   │   │        Runs entirely in WASM            │  │  │  │
│  │   │   └─────────────────────────────────────────┘  │  │  │
│  │   └────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

The WASM bundle only includes `ClientRequired` components, not the whole app.

### 6.4 Client vs Server Signals

```go
// Regular signal: lives on server (server-driven) or WASM (WASM mode)
count := vango.Signal(0)

// Local signal: always lives on client (for latency-sensitive state)
cursorPos := vango.LocalSignal(Position{0, 0})

// Synced signal: client + server, with optimistic updates
savedValue := vango.SyncedSignal(0)
```

**LocalSignal** in server-driven mode:
- Requires minimal WASM runtime (~20KB) or JS implementation
- State never leaves the browser
- Great for UI state (hover, focus, scroll position)

**SyncedSignal**:
- Client updates immediately (optimistic)
- Server confirms or rejects
- Automatic reconciliation

---

## 7. State Management

Vango's state management is designed around the server-driven architecture. Unlike client-side frameworks that need complex solutions (Redux, MobX, Zustand) to synchronize state, Vango keeps state on the server where the data already lives.

### 7.1 Why Vango Doesn't Need Redux

Traditional SPAs face a fundamental problem: state lives in the browser but data lives on the server. This creates:

- **Synchronization hell** — Keep client state in sync with server
- **Cache invalidation** — When is cached data stale?
- **Optimistic updates** — Show changes before server confirms
- **Conflict resolution** — What if two tabs modify the same data?

Vango sidesteps most of this by keeping state on the server:

```
Traditional SPA:                     Vango Server-Driven:

┌─────────────────────┐              ┌─────────────────────┐
│  Browser            │              │  Browser            │
│  ┌───────────────┐  │              │                     │
│  │ Redux Store   │  │              │  (minimal state)    │
│  │ - users       │  │              │  - UI-only state    │
│  │ - products    │  │              │  - optimistic vals  │
│  │ - cart        │  │              │                     │
│  │ - filters     │  │              └──────────┬──────────┘
│  └───────────────┘  │                         │
│         ↕ sync      │                         │ WebSocket
└─────────┬───────────┘              ┌──────────▼──────────┐
          │                          │  Server             │
┌─────────▼───────────┐              │  ┌───────────────┐  │
│  Server             │              │  │ All State     │  │
│  (source of truth)  │              │  │ Lives Here    │  │
└─────────────────────┘              │  └───────────────┘  │
                                     └─────────────────────┘
```

### 7.2 Signal Scopes

Vango provides three signal scopes for different use cases:

```go
// ┌─────────────────────────────────────────────────────────────────┐
// │                     SIGNAL SCOPES                                │
// ├─────────────────┬─────────────────┬─────────────────────────────┤
// │    Signal       │  SharedSignal   │      GlobalSignal           │
// ├─────────────────┼─────────────────┼─────────────────────────────┤
// │ One component   │ One session     │ All sessions                │
// │ instance        │ (one user/tab)  │ (all users)                 │
// ├─────────────────┼─────────────────┼─────────────────────────────┤
// │ Form input,     │ Shopping cart,  │ Live cursors,               │
// │ local UI state  │ user prefs,     │ collaborative editing,      │
// │                 │ filters         │ presence indicators         │
// └─────────────────┴─────────────────┴─────────────────────────────┘

// Component-local: created per instance, GC'd on unmount
func Counter(initial int) vango.Component {
    return vango.Func(func() *vango.VNode {
        count := vango.Signal(initial)  // Local to this Counter
        return Div(Text(fmt.Sprintf("%d", count())))
    })
}

// Session-shared: defined at package level, scoped to user session
var CartItems = vango.SharedSignal([]CartItem{})

// Global: shared across ALL connected users
var OnlineUsers = vango.GlobalSignal([]User{})
```

### 7.3 Shared State Patterns

#### Basic Shared State (Store Pattern)

```go
// store/cart.go — Define your store
package store

import "vango"

// State
var CartItems = vango.SharedSignal([]CartItem{})

// Derived state (automatically updates when CartItems changes)
var CartTotal = vango.SharedMemo(func() float64 {
    total := 0.0
    for _, item := range CartItems() {
        total += item.Price * float64(item.Qty)
    }
    return total
})

var CartCount = vango.SharedMemo(func() int {
    count := 0
    for _, item := range CartItems() {
        count += item.Qty
    }
    return count
})

// Actions (functions that modify state)
func AddItem(product Product, qty int) {
    CartItems.Update(func(items []CartItem) []CartItem {
        // Check if already in cart
        for i, item := range items {
            if item.ProductID == product.ID {
                items[i].Qty += qty
                return items
            }
        }
        // Add new item
        return append(items, CartItem{
            ProductID: product.ID,
            Product:   product,
            Qty:       qty,
        })
    })
}

func RemoveItem(productID int) {
    CartItems.Update(func(items []CartItem) []CartItem {
        result := make([]CartItem, 0, len(items))
        for _, item := range items {
            if item.ProductID != productID {
                result = append(result, item)
            }
        }
        return result
    })
}

func ClearCart() {
    CartItems.Set([]CartItem{})
}
```

```go
// components/header.go — Read from store
func Header() *vango.VNode {
    return Nav(Class("header"),
        Logo(),
        NavLinks(),
        // Reads from shared state — auto-updates when cart changes
        CartBadge(store.CartCount()),
    )
}
```

```go
// components/product_card.go — Write to store
func ProductCard(p Product) *vango.VNode {
    return Div(Class("product-card"),
        Img(Src(p.ImageURL)),
        H3(Text(p.Name)),
        Price(p.Price),
        Button(
            OnClick(func() { store.AddItem(p, 1) }),
            Text("Add to Cart"),
        ),
    )
}
```

#### Complex State with Nested Objects

```go
// store/project.go
package store

type ProjectState struct {
    Project  *Project
    Tasks    []Task
    Filter   TaskFilter
    Selected map[int]bool
}

var State = vango.SharedSignal(ProjectState{})

// Selectors (derived state for specific parts)
var FilteredTasks = vango.SharedMemo(func() []Task {
    s := State()
    return filterTasks(s.Tasks, s.Filter)
})

var SelectedCount = vango.SharedMemo(func() int {
    count := 0
    for _, selected := range State().Selected {
        if selected {
            count++
        }
    }
    return count
})

// Actions with targeted updates
func ToggleTask(taskID int) {
    State.Update(func(s ProjectState) ProjectState {
        for i := range s.Tasks {
            if s.Tasks[i].ID == taskID {
                s.Tasks[i].Done = !s.Tasks[i].Done
                break
            }
        }
        return s
    })
}

func SetFilter(filter TaskFilter) {
    State.Update(func(s ProjectState) ProjectState {
        s.Filter = filter
        return s
    })
}

func SelectTask(taskID int, selected bool) {
    State.Update(func(s ProjectState) ProjectState {
        if s.Selected == nil {
            s.Selected = make(map[int]bool)
        }
        s.Selected[taskID] = selected
        return s
    })
}
```

### 7.4 Global State (Real-time Collaboration)

GlobalSignals synchronize across all connected sessions:

```go
// store/presence.go
package store

// Shared across ALL users
var OnlineUsers = vango.GlobalSignal([]User{})
var CursorPositions = vango.GlobalSignal(map[string]Position{})

// When a user connects
func UserJoined(user User) {
    OnlineUsers.Update(func(users []User) []User {
        return append(users, user)
    })
}

// When a user disconnects (called automatically by Vango)
func UserLeft(userID string) {
    OnlineUsers.Update(func(users []User) []User {
        result := make([]User, 0, len(users))
        for _, u := range users {
            if u.ID != userID {
                result = append(result, u)
            }
        }
        return result
    })

    CursorPositions.Update(func(pos map[string]Position) map[string]Position {
        delete(pos, userID)
        return pos
    })
}

// Broadcast cursor movement
func MoveCursor(userID string, pos Position) {
    CursorPositions.Update(func(positions map[string]Position) map[string]Position {
        positions[userID] = pos
        return positions
    })
}
```

```go
// components/collaborative_canvas.go
func CollaborativeCanvas() vango.Component {
    return vango.Func(func() *vango.VNode {
        cursors := store.CursorPositions()
        currentUser := ctx.User()

        return Div(Class("canvas"),
            // Render other users' cursors
            Range(cursors, func(userID string, pos Position) *vango.VNode {
                if userID == currentUser.ID {
                    return nil  // Don't show own cursor
                }
                return CursorIndicator(userID, pos)
            }),

            // Track mouse movement
            Div(
                Class("canvas-area"),
                OnMouseMove(func(x, y int) {
                    store.MoveCursor(currentUser.ID, Position{x, y})
                }),
            ),
        )
    })
}
```

### 7.5 Resource Pattern for Async Data

For loading data with loading/error/success states:

```go
// Resource wraps async data with loading state
func UserProfile(userID int) vango.Component {
    return vango.Func(func() *vango.VNode {
        // Resource handles loading/error/ready states
        user := vango.Resource(func() (*User, error) {
            return db.Users.FindByID(userID)
        })

        // Pattern 1: Switch on state
        switch user.State() {
        case vango.Loading:
            return Skeleton("user-profile")
        case vango.Error:
            return ErrorCard(user.Error())
        case vango.Ready:
            return UserCard(user.Data())
        }
        return nil
    })
}
```

```go
// Pattern 2: Match helper for cleaner code
func UserProfile(userID int) vango.Component {
    return vango.Func(func() *vango.VNode {
        user := vango.Resource(func() (*User, error) {
            return db.Users.FindByID(userID)
        })

        return user.Match(
            vango.OnLoading(func() *vango.VNode {
                return Skeleton("user-profile")
            }),
            vango.OnError(func(err error) *vango.VNode {
                return ErrorCard(err)
            }),
            vango.OnReady(func(u *User) *vango.VNode {
                return UserCard(u)
            }),
        )
    })
}
```

```go
// Pattern 3: With refetch capability
func ProjectPage(projectID int) vango.Component {
    return vango.Func(func() *vango.VNode {
        project := vango.Resource(func() (*Project, error) {
            return db.Projects.FindByID(projectID)
        })

        return Div(
            Header(
                H1(Text(project.Data().Name)),
                Button(
                    OnClick(project.Refetch),  // Manually trigger reload
                    Text("Refresh"),
                ),
            ),
            // ...
        )
    })
}
```

### 7.6 Immutable Update Helpers

To avoid accidental mutations, use these helper patterns:

```go
// DANGEROUS: Mutating in place
items.Update(func(i []Item) []Item {
    i[0].Done = true  // Mutation! Other references see this change
    return i
})

// SAFE: Create new slice/struct
items.Update(func(items []Item) []Item {
    result := make([]Item, len(items))
    copy(result, items)
    result[0] = Item{...result[0], Done: true}
    return result
})
```

**Built-in helpers for common operations:**

```go
// Array operations
items.Append(newItem)                           // Add to end
items.Prepend(newItem)                          // Add to start
items.InsertAt(index, newItem)                  // Insert at position
items.RemoveAt(index)                           // Remove by index
items.RemoveWhere(func(i Item) bool { ... })    // Remove by predicate
items.UpdateAt(index, func(i Item) Item { ... }) // Update single item
items.UpdateWhere(predicate, updater)           // Update matching items

// Map operations
users.SetKey(id, user)                          // Set key
users.RemoveKey(id)                             // Remove key
users.UpdateKey(id, func(u User) User { ... })  // Update key

// Examples:
tasks.UpdateAt(0, func(t Task) Task {
    return Task{...t, Done: true}
})

tasks.RemoveWhere(func(t Task) bool {
    return t.Done
})

userMap.UpdateKey(userID, func(u User) User {
    return User{...u, LastSeen: time.Now()}
})
```

### 7.7 Computed Chains (Derived State)

Memos can depend on other memos, creating a computation graph:

```go
// store/analytics.go
package store

var RawData = vango.SharedSignal([]DataPoint{})
var DateRange = vango.SharedSignal(DateRange{Start: weekAgo, End: now})
var Grouping = vango.SharedSignal("day")  // day, week, month

// Level 1: Filter by date
var FilteredData = vango.SharedMemo(func() []DataPoint {
    data := RawData()
    range_ := DateRange()
    return filterByDate(data, range_.Start, range_.End)
})

// Level 2: Group filtered data
var GroupedData = vango.SharedMemo(func() map[string][]DataPoint {
    data := FilteredData()  // Depends on FilteredData
    grouping := Grouping()
    return groupByPeriod(data, grouping)
})

// Level 3: Aggregate grouped data
var ChartData = vango.SharedMemo(func() []ChartPoint {
    grouped := GroupedData()  // Depends on GroupedData
    return aggregateForChart(grouped)
})

// Level 3 (parallel): Summary stats
var SummaryStats = vango.SharedMemo(func() Stats {
    data := FilteredData()  // Also depends on FilteredData
    return Stats{
        Total:   sum(data),
        Average: avg(data),
        Max:     max(data),
        Min:     min(data),
    }
})
```

```
Dependency Graph:

RawData ──┬──► FilteredData ──┬──► GroupedData ──► ChartData
          │                   │
DateRange─┘                   └──► SummaryStats

Grouping ─────────────────────────► GroupedData
```

When `DateRange` changes:
1. `FilteredData` recomputes
2. `GroupedData` recomputes (depends on FilteredData)
3. `ChartData` recomputes (depends on GroupedData)
4. `SummaryStats` recomputes (depends on FilteredData)

Memos that don't depend on changed values are NOT recomputed.

### 7.8 Batching Updates

Multiple signal updates in sequence trigger multiple re-renders. Use `Batch` to combine them:

```go
// Without batch: 3 re-renders
func resetFilters() {
    store.SearchQuery.Set("")     // Re-render 1
    store.Category.Set("all")     // Re-render 2
    store.SortOrder.Set("newest") // Re-render 3
}

// With batch: 1 re-render
func resetFilters() {
    vango.Batch(func() {
        store.SearchQuery.Set("")
        store.Category.Set("all")
        store.SortOrder.Set("newest")
    })
    // Single re-render after batch completes
}
```

### 7.9 Persistence

Signals can be automatically persisted to various backends:

```go
// Browser Session Storage (per tab, cleared on close)
tabState := vango.Signal(TabState{}).Persist(vango.SessionStorage, "tab-state")

// Browser Local Storage (persists across sessions)
userPrefs := vango.Signal(Preferences{
    Theme:    "system",
    Compact:  false,
    Language: "en",
}).Persist(vango.LocalStorage, "user-prefs")

// Server Database (persists permanently, syncs across devices)
userSettings := vango.Signal(Settings{}).Persist(
    vango.Database,
    fmt.Sprintf("settings:%s", userID),
)

// Custom persistence (e.g., Redis, S3)
largeData := vango.Signal(Data{}).Persist(
    vango.Custom(redisStore),
    "large-data-key",
)
```

### 7.10 Debugging Signals

In development mode, Vango logs all signal changes:

```go
// Optional: Add action names for clearer logs
count.Set(count() + 1, "increment button clicked")
```

```
[12:34:56.789] Signal store.CartItems: [] → [{id:1, qty:1}]
               Action: "add to cart"
               Source: components/product_card.go:42
               Subscribers: [Header, Sidebar, CartPage]

[12:34:56.801] Memo store.CartTotal: recomputed 0 → 29.99
               Dependencies: [store.CartItems]

[12:34:56.802] Re-render: Header (1 patch), Sidebar (2 patches)
```

**DevTools integration:**

```go
// vango.json
{
    "devTools": {
        "signalLogging": true,
        "dependencyGraph": true,
        "performanceMetrics": true
    }
}
```

Opens a browser panel showing:
- All signals and current values
- Dependency graph visualization
- Re-render triggers and timing
- Time-travel through signal history

### 7.11 When to Use Each Pattern

| Scenario | Pattern | Example |
|----------|---------|---------|
| Form input | Local Signal | `input := vango.Signal("")` |
| UI state (modals, tabs) | Local Signal | `isOpen := vango.Signal(false)` |
| Shopping cart | SharedSignal | `var Cart = vango.SharedSignal(...)` |
| User preferences | SharedSignal + Persist | `var Prefs = vango.SharedSignal(...).Persist(...)` |
| Filter/search state | SharedSignal | `var Filter = vango.SharedSignal(...)` |
| Async data loading | Resource | `user := vango.Resource(...)` |
| Derived calculations | Memo | `var Total = vango.SharedMemo(...)` |
| Real-time presence | GlobalSignal | `var OnlineUsers = vango.GlobalSignal(...)` |
| Collaborative editing | GlobalSignal | `var DocContent = vango.GlobalSignal(...)` |

### 7.12 Anti-Patterns to Avoid

```go
// ❌ DON'T: Create signals outside component context
var badSignal = vango.Signal(0)  // Package-level local signal

func MyComponent() vango.Component {
    return vango.Func(func() *vango.VNode {
        // badSignal is shared across ALL instances!
        return Text(fmt.Sprintf("%d", badSignal()))
    })
}

// ✅ DO: Create local signals inside the component
func MyComponent() vango.Component {
    return vango.Func(func() *vango.VNode {
        goodSignal := vango.Signal(0)  // Per-instance
        return Text(fmt.Sprintf("%d", goodSignal()))
    })
}

// ✅ OR: Use SharedSignal explicitly for intentional sharing
var intentionallyShared = vango.SharedSignal(0)
```

```go
// ❌ DON'T: Read signals conditionally
func BadComponent() vango.Component {
    return vango.Func(func() *vango.VNode {
        if someCondition {
            value := mySignal()  // Subscription depends on condition!
        }
        return Div()
    })
}

// ✅ DO: Read signals unconditionally, use value conditionally
func GoodComponent() vango.Component {
    return vango.Func(func() *vango.VNode {
        value := mySignal()  // Always subscribe
        if someCondition {
            return Div(Text(fmt.Sprintf("%d", value)))
        }
        return Div()
    })
}
```

```go
// ❌ DON'T: Heavy computation in signal update
items.Update(func(i []Item) []Item {
    // This runs synchronously, blocking the event loop
    result := veryExpensiveOperation(i)  // Bad!
    return result
})

// ✅ DO: Use Effect for heavy async work
vango.Effect(func() vango.Cleanup {
    go func() {
        result := veryExpensiveOperation(items())
        processedItems.Set(result)
    }()
    return nil
})
```

---

## 8. Interaction Primitives

Vango provides a spectrum of interaction patterns, from simple server events to rich client-side behaviors. The key insight is that **the server doesn't need to know about every drag pixel—it only needs to know the final result**.

### 8.1 Design Philosophy

Vango uses a three-tier interaction model:

```
┌─────────────────────────────────────────────────────────────────┐
│                    INTERACTION SPECTRUM                          │
├───────────────────┬─────────────────────┬───────────────────────┤
│   Server Events   │    Client Hooks     │     JS Islands        │
├───────────────────┼─────────────────────┼───────────────────────┤
│   OnClick         │   Hook("Sortable")  │  JSIsland("editor")   │
│   OnSubmit        │   Hook("Draggable") │  JSIsland("chart")    │
│   OnInput         │   Hook("Tooltip")   │  JSIsland("map")      │
├───────────────────┼─────────────────────┼───────────────────────┤
│   Server runs     │   Client runs the   │  Client runs          │
│   the handler     │   behavior, server  │  everything,          │
│                   │   owns state        │  bridges to server    │
├───────────────────┼─────────────────────┼───────────────────────┤
│   ~0 client KB    │   ~15KB (bundled)   │  Variable (lazy)      │
├───────────────────┼─────────────────────┼───────────────────────┤
│   50-100ms        │   60fps interaction │  60fps interaction    │
│   latency OK      │   + single event    │  + bridge events      │
└───────────────────┴─────────────────────┴───────────────────────┘
```

**When to use each tier:**

| Tier | Use When | Examples |
|------|----------|----------|
| **Server Events** | Latency is acceptable (buttons, forms, navigation) | Click handlers, form submission, toggles |
| **Client Hooks** | Need 60fps feedback but server owns state | Drag-and-drop, sortable lists, tooltips, dropdowns |
| **JS Islands** | Full third-party library or complex client logic | Rich text editors, charts, maps, video players |

**The Hook pattern** is the key innovation here. It delegates client-side interaction physics to specialized JavaScript while keeping state management on the server. This gives you:

- **60fps animations** during drag operations
- **Zero network traffic** during interactions
- **Simple Go code** (just handle the final result)
- **Graceful failure** (interaction works, sync fails gracefully)

### 8.2 Client Hooks

Client Hooks are the recommended way to handle interactions that need 60fps visual feedback (drag-and-drop, sortable lists, tooltips, etc.). The hook handles all client-side animation and behavior, then sends a single event to the server when the interaction completes.

#### The Hook Attribute

```go
// Attach a hook to any element
Div(
    Hook("Sortable", map[string]any{
        "group":     "tasks",
        "animation": 150,
        "handle":    ".drag-handle",
    }),

    // Handle events from the hook
    OnEvent("reorder", func(e vango.HookEvent) {
        // This fires ONCE when drag ends, not during drag
        fromIndex := e.Int("fromIndex")
        toIndex := e.Int("toIndex")
        db.Tasks.Reorder(fromIndex, toIndex)
    }),

    // Children...
)
```

#### Why Hooks Instead of Server Events

Consider drag-and-drop. With server events:

```
User drags card → Stream of dragover events → Server processes each →
Client predicts DOM changes → Server sends patches → Reconciliation

Problems:
- Network during drag (latency spikes visible)
- Complex prediction logic on client
- Server CPU processing drag events
- Not truly 60fps
```

With hooks:

```
User drags card → Hook handles animation at 60fps →
User drops card → ONE event to server → Server updates DB

Benefits:
- Zero network during drag
- Native 60fps from specialized library
- Simple server code (just handle result)
- Works even with high latency
```

#### HookEvent API

```go
type HookEvent struct {
    Name string         // Event name (e.g., "reorder", "drop")
    Data map[string]any // Event data from the hook
}

// Type-safe accessors
func (e HookEvent) String(key string) string
func (e HookEvent) Int(key string) int
func (e HookEvent) Float(key string) float64
func (e HookEvent) Bool(key string) bool
func (e HookEvent) Strings(key string) []string
func (e HookEvent) Raw(key string) any
```

#### Complete Example: Sortable List

```go
func SortableList(items []Item, onReorder func(fromIdx, toIdx int)) *vango.VNode {
    return Ul(
        Class("sortable-list"),

        // Hook handles all drag animation at 60fps
        Hook("Sortable", map[string]any{
            "animation":  150,
            "ghostClass": "sortable-ghost",
        }),

        // Only fires when drag completes
        OnEvent("reorder", func(e vango.HookEvent) {
            onReorder(e.Int("fromIndex"), e.Int("toIndex"))
        }),

        Range(items, func(item Item, i int) *vango.VNode {
            return Li(
                Key(item.ID),
                Data("id", item.ID),
                Text(item.Name),
            )
        }),
    )
}
```

#### Complete Example: Kanban Board

```go
func KanbanBoard(columns []Column) vango.Component {
    return vango.Func(func() *vango.VNode {
        return Div(Class("kanban-board"),
            Range(columns, func(col Column) *vango.VNode {
                return Div(
                    Key(col.ID),
                    Class("kanban-column"),
                    Data("column-id", col.ID),

                    // Hook handles all drag visuals at 60fps
                    Hook("Sortable", map[string]any{
                        "group":      "cards",
                        "animation":  150,
                        "ghostClass": "card-ghost",
                    }),

                    // Only fires when drag ends
                    OnEvent("reorder", func(e vango.HookEvent) {
                        cardID := e.String("id")
                        toColumn := e.String("toColumn")
                        toIndex := e.Int("newIndex")

                        // Update database
                        err := db.Cards.Move(cardID, toColumn, toIndex)
                        if err != nil {
                            // Hook can revert the visual change
                            e.Revert()
                            toast.Error("Failed to move card")
                        }
                    }),

                    H3(Class("column-title"), Text(col.Name)),

                    Div(Class("card-list"),
                        Range(col.Cards, func(card Card, i int) *vango.VNode {
                            return Div(
                                Key(card.ID),
                                Data("id", card.ID),
                                Class("kanban-card"),
                                CardContent(card),
                            )
                        }),
                    ),
                )
            }),
        )
    })
}
```

**What happens during a drag:**

1. User starts dragging a card
2. SortableJS (bundled in thin client) handles animation at 60fps
3. Cards shuffle smoothly as cursor moves
4. User drops the card
5. Client sends ONE event: `{id: "card-123", toColumn: "done", newIndex: 2}`
6. Server updates database
7. Server re-renders and sends confirmation patch
8. If server fails, client can revert (`e.Revert()`)

**Network traffic during drag:** Zero.
**Frames per second:** 60.
**Go code complexity:** Minimal.

### 8.3 Optimistic Updates

For simple interactions like button clicks and toggles, optimistic updates provide instant visual feedback while the server processes the action.

> **Note:** For complex interactions like drag-and-drop, use [Client Hooks](#82-client-hooks) instead. Hooks handle the animation natively and don't need prediction logic.

#### When to Use Optimistic Updates

| Use Case | Recommended Approach |
|----------|---------------------|
| Toggle checkbox | Optimistic update |
| Like button | Optimistic update |
| Increment counter | Optimistic update |
| Delete item (simple) | Optimistic update |
| Drag-and-drop | Client Hook (§8.2) |
| Sortable list | Client Hook (§8.2) |
| Complex animations | Client Hook (§8.2) |

#### Simple Optimistic Attributes

```go
// Toggle a class optimistically
Button(
    OnClick(toggleComplete),
    OptimisticClass("completed", !task.Done),  // Toggle class instantly
    Text("Complete"),
)

// Update text optimistically
Button(
    OnClick(incrementLikes),
    OptimisticText(fmt.Sprintf("%d", likes + 1)),  // Show new count instantly
    Textf("%d likes", likes),
)

// Toggle attribute optimistically
Button(
    OnClick(toggleDisabled),
    OptimisticAttr("disabled", "true"),
    Text("Submit"),
)
```

#### How It Works

```
1. User clicks button
2. Client applies optimistic change immediately (class, text, etc.)
3. Client sends event to server
4. Server processes and sends confirmation
5a. If success: Server patch matches optimistic state (no visual change)
5b. If failure: Server patch reverts to original state
```

#### Complete Example: Task Toggle

```go
func TaskItem(task Task) *vango.VNode {
    return Li(
        Key(task.ID),
        Class("task-item"),
        ClassIf(task.Done, "completed"),

        Input(
            Type("checkbox"),
            Checked(task.Done),
            OnChange(func() {
                db.Tasks.Toggle(task.ID)
            }),
            // Toggle the parent's class optimistically
            OptimisticParentClass("completed", !task.Done),
        ),

        Span(Text(task.Title)),
    )
}
```

#### Complete Example: Like Button

```go
func LikeButton(postID string, likes int, userLiked bool) *vango.VNode {
    return Button(
        Class("like-button"),
        ClassIf(userLiked, "liked"),

        OnClick(func() {
            if userLiked {
                db.Posts.Unlike(postID)
            } else {
                db.Posts.Like(postID)
            }
        }),

        // Optimistic visual feedback
        OptimisticClass("liked", !userLiked),
        OptimisticText(func() string {
            if userLiked {
                return fmt.Sprintf("%d", likes-1)
            }
            return fmt.Sprintf("%d", likes+1)
        }()),

        Icon("heart"),
        Span(Textf("%d", likes)),
    )
}
```

#### Signal-Based Optimistic Updates

For more control, update signals optimistically with manual rollback:

```go
func TaskList() vango.Component {
    return vango.Func(func() *vango.VNode {
        tasks := vango.Signal(initialTasks)

        toggleTask := func(taskID string) {
            // Capture original state for rollback
            originalTasks := tasks()

            // Optimistically update signal (triggers re-render immediately)
            tasks.Update(func(t []Task) []Task {
                for i := range t {
                    if t[i].ID == taskID {
                        t[i].Done = !t[i].Done
                        break
                    }
                }
                return t
            })

            // Server action
            go func() {
                err := db.Tasks.Toggle(taskID)
                if err != nil {
                    // Revert on failure
                    tasks.Set(originalTasks)
                    toast.Error("Failed to update task")
                }
            }()
        }

        return Ul(
            Range(tasks(), func(task Task, i int) *vango.VNode {
                return Li(
                    Key(task.ID),
                    ClassIf(task.Done, "completed"),
                    OnClick(func() { toggleTask(task.ID) }),
                    Text(task.Title),
                )
            }),
        )
    })
}
```

### 8.4 Standard Hooks

Vango bundles a set of standard hooks for common interaction patterns. These are included in the thin client (~15KB total with hooks).

#### Available Standard Hooks

| Hook | Purpose | Events |
|------|---------|--------|
| `Sortable` | Drag-to-reorder lists and grids | `reorder` |
| `Draggable` | Free-form element dragging | `dragend` |
| `Droppable` | Drop zones for draggable elements | `drop` |
| `Resizable` | Resize handles on elements | `resize` |
| `Tooltip` | Hover tooltips | (none - visual only) |
| `Dropdown` | Click-outside-to-close behavior | `close` |
| `Collapsible` | Expand/collapse with animation | `toggle` |

#### Sortable Hook

For drag-to-reorder lists:

```go
Ul(
    Class("task-list"),

    Hook("Sortable", map[string]any{
        "animation":  150,           // Animation duration (ms)
        "handle":     ".drag-handle", // Optional: restrict to handle
        "ghostClass": "ghost",       // Class for placeholder
        "group":      "tasks",       // Allow drag between lists with same group
    }),

    OnEvent("reorder", func(e vango.HookEvent) {
        fromIndex := e.Int("fromIndex")
        toIndex := e.Int("toIndex")
        db.Tasks.Reorder(fromIndex, toIndex)
    }),

    Range(tasks, TaskItem),
)
```

**Sortable Event Data:**
```go
e.Int("fromIndex")      // Original index
e.Int("toIndex")        // New index
e.String("id")          // data-id of moved element
e.String("fromGroup")   // Group moved from (if cross-list)
e.String("toGroup")     // Group moved to (if cross-list)
```

#### Draggable Hook

For free-form dragging:

```go
Div(
    Class("floating-panel"),

    Hook("Draggable", map[string]any{
        "handle":  ".panel-header",  // Drag handle
        "bounds":  "parent",         // Constrain to parent
        "axis":    "both",           // "x", "y", or "both"
        "grid":    []int{10, 10},    // Snap to grid
    }),

    OnEvent("dragend", func(e vango.HookEvent) {
        x := e.Int("x")
        y := e.Int("y")
        db.Panels.UpdatePosition(panelID, x, y)
    }),

    PanelContent(),
)
```

#### Droppable Hook

For drop zones:

```go
Div(
    Class("upload-zone"),

    Hook("Droppable", map[string]any{
        "accept":     ".draggable-file",  // CSS selector
        "hoverClass": "drag-over",        // Class when dragging over
    }),

    OnEvent("drop", func(e vango.HookEvent) {
        itemID := e.String("id")
        handleDrop(itemID)
    }),

    Text("Drop files here"),
)
```

#### Resizable Hook

For resizable elements:

```go
Div(
    Class("resizable-panel"),
    Style(fmt.Sprintf("width: %dpx; height: %dpx", width, height)),

    Hook("Resizable", map[string]any{
        "handles":   "e,s,se",       // Which edges: n,e,s,w,ne,se,sw,nw
        "minWidth":  200,
        "maxWidth":  800,
        "minHeight": 100,
    }),

    OnEvent("resize", func(e vango.HookEvent) {
        width := e.Int("width")
        height := e.Int("height")
        db.Panels.UpdateSize(panelID, width, height)
    }),

    PanelContent(),
)
```

#### Tooltip Hook

For hover tooltips (visual only, no events):

```go
Button(
    Hook("Tooltip", map[string]any{
        "content":   "Click to save",
        "placement": "top",          // top, bottom, left, right
        "delay":     200,            // Delay before showing (ms)
    }),

    Text("Save"),
)

// Dynamic tooltip content
Button(
    Hook("Tooltip", map[string]any{
        "content":   fmt.Sprintf("Last saved: %s", lastSaved.Format(time.Kitchen)),
        "placement": "bottom",
    }),

    Text("Status"),
)
```

#### Dropdown Hook

For click-outside-to-close behavior:

```go
func DropdownMenu() vango.Component {
    return vango.Func(func() *vango.VNode {
        open := vango.Signal(false)

        return Div(
            Class("dropdown"),

            Button(OnClick(open.Toggle), Text("Menu")),

            If(open(),
                Div(
                    Class("dropdown-content"),

                    Hook("Dropdown", map[string]any{
                        "closeOnEscape": true,
                        "closeOnClick":  true,  // Close when clicking inside
                    }),

                    OnEvent("close", func(e vango.HookEvent) {
                        open.Set(false)
                    }),

                    MenuItem("Edit"),
                    MenuItem("Delete"),
                ),
            ),
        )
    })
}
```

#### Collapsible Hook

For animated expand/collapse:

```go
Div(
    Class("accordion-item"),

    Button(
        Class("accordion-header"),
        OnClick(func() { /* toggle handled by hook */ }),
        Text("Section Title"),
    ),

    Div(
        Class("accordion-content"),

        Hook("Collapsible", map[string]any{
            "open":     isOpen,
            "duration": 200,
        }),

        OnEvent("toggle", func(e vango.HookEvent) {
            isNowOpen := e.Bool("open")
            // Update state if needed
        }),

        SectionContent(),
    ),
)
```

### 8.5 Custom Hooks

For behaviors not covered by standard hooks, you can define custom hooks in JavaScript.

#### Creating a Custom Hook

```javascript
// public/js/hooks.js
export default {
    // Custom color picker hook
    ColorPicker: {
        mounted(el, config, pushEvent) {
            // Initialize when element mounts
            this.picker = new Pickr({
                el: el,
                default: config.color || '#000000',
                components: {
                    preview: true,
                    hue: true,
                },
            });

            // Send events to server
            this.picker.on('change', (color) => {
                pushEvent('color-changed', {
                    color: color.toHEXA().toString()
                });
            });
        },

        updated(el, config, pushEvent) {
            // Called when config changes
            if (config.color) {
                this.picker.setColor(config.color);
            }
        },

        destroyed(el) {
            // Cleanup when element unmounts
            this.picker.destroy();
        }
    },

    // Custom chart hook
    Chart: {
        mounted(el, config, pushEvent) {
            this.chart = new Chart(el.getContext('2d'), {
                type: config.type,
                data: config.data,
            });
        },

        updated(el, config, pushEvent) {
            this.chart.data = config.data;
            this.chart.update();
        },

        destroyed(el) {
            this.chart.destroy();
        }
    }
};
```

#### Registering Custom Hooks

```json
// vango.json
{
    "hooks": "./public/js/hooks.js"
}
```

Or programmatically:

```go
// main.go
func main() {
    app := vango.New()
    app.RegisterHooks("./public/js/hooks.js")
    app.Run()
}
```

#### Using Custom Hooks

```go
// Use just like standard hooks
Div(
    Hook("ColorPicker", map[string]any{
        "color": currentColor,
    }),

    OnEvent("color-changed", func(e vango.HookEvent) {
        newColor := e.String("color")
        db.Settings.SetColor(newColor)
    }),
)
```

#### Hook Lifecycle

| Method | When Called |
|--------|-------------|
| `mounted(el, config, pushEvent)` | Element added to DOM |
| `updated(el, config, pushEvent)` | Hook config changed |
| `destroyed(el)` | Element removed from DOM |

**Parameters:**
- `el` — The DOM element the hook is attached to
- `config` — The configuration map passed from Go
- `pushEvent(name, data)` — Function to send events to server

#### pushEvent API

```javascript
// Send event to server
pushEvent('event-name', {
    key: 'value',
    number: 42,
    array: [1, 2, 3]
});

// In Go, handle with OnEvent
OnEvent("event-name", func(e vango.HookEvent) {
    value := e.String("key")
    number := e.Int("number")
    array := e.Strings("array")  // Returns []string
})
```

#### Revert Capability

Hooks can support revert for failed server operations:

```javascript
// In hook
ColorPicker: {
    mounted(el, config, pushEvent) {
        this.picker = new Pickr({...});
        this.lastColor = config.color;

        this.picker.on('change', (color) => {
            const newColor = color.toHEXA().toString();

            pushEvent('color-changed', {
                color: newColor,
                // Include revert info
                _revert: () => this.picker.setColor(this.lastColor)
            });

            this.lastColor = newColor;
        });
    }
}
```

```go
// In Go - revert on error
OnEvent("color-changed", func(e vango.HookEvent) {
    err := db.Settings.SetColor(e.String("color"))
    if err != nil {
        e.Revert()  // Calls the _revert function
        toast.Error("Failed to save color")
    }
})
```

### 8.6 Keyboard Navigation

> **Note:** Keyboard navigation uses server events since latency is acceptable for key presses.

#### Scoped Keyboard Handlers

```go
func BoardPage() vango.Component {
    return vango.Func(func() *vango.VNode {
        selected := vango.Signal[*Card](nil)

        return KeyboardScope(
            // Arrow navigation
            Key("ArrowDown", func() { selectNext(selected) }),
            Key("ArrowUp", func() { selectPrev(selected) }),
            Key("ArrowRight", func() { moveToNextColumn(selected) }),
            Key("ArrowLeft", func() { moveToPrevColumn(selected) }),

            // Actions
            Key("Enter", func() { openCard(selected()) }),
            Key("e", func() { editCard(selected()) }),
            Key("d", func() { deleteCard(selected()) }),
            Key("Escape", func() { selected.Set(nil) }),

            // With modifiers
            Key("Cmd+k", openCommandPalette),
            Key("Cmd+Enter", saveAndClose),
            Key("Shift+?", showKeyboardShortcuts),

            // The actual content
            BoardContent(selected),
        )
    })
}
```

#### Key Modifiers

```go
// Available modifiers
Key("Cmd+k", handler)       // Cmd on Mac, Ctrl on Windows/Linux
Key("Ctrl+k", handler)      // Always Ctrl
Key("Alt+k", handler)       // Alt/Option
Key("Shift+k", handler)     // Shift
Key("Cmd+Shift+k", handler) // Combinations

// Special keys
Key("Enter", handler)
Key("Escape", handler)
Key("Tab", handler)
Key("Backspace", handler)
Key("Delete", handler)
Key("Space", handler)
Key("ArrowUp", handler)
Key("ArrowDown", handler)
Key("ArrowLeft", handler)
Key("ArrowRight", handler)
```

#### Global Keyboard Shortcuts

```go
// Register global shortcuts (work anywhere on the page)
func App() vango.Component {
    return vango.Func(func() *vango.VNode {
        // Global shortcuts
        vango.GlobalKey("Cmd+k", openCommandPalette)
        vango.GlobalKey("/", focusSearch)
        vango.GlobalKey("?", showHelp)
        vango.GlobalKey("Escape", closeModals)

        return Div(
            Header(),
            Main(children...),
            Footer(),
        )
    })
}
```

#### Nested Scopes

```go
// Outer scope
KeyboardScope(
    Key("Escape", closePanel),

    // Inner scope (takes priority when focused)
    KeyboardScope(
        Key("Escape", clearSelection),  // This fires first
        Key("Enter", confirmSelection),

        SelectionList(...),
    ),
)
```

### 8.7 Focus Management

```go
// Auto-focus on mount
Input(
    Autofocus(),
    Type("text"),
)

// Programmatic focus
func SearchModal() vango.Component {
    return vango.Func(func() *vango.VNode {
        inputRef := vango.Ref[js.Value](nil)

        vango.Effect(func() vango.Cleanup {
            // Focus input when modal opens
            inputRef.Current().Call("focus")
            return nil
        })

        return Div(Class("modal"),
            Input(
                Ref(inputRef),
                Type("search"),
                Placeholder("Search..."),
            ),
        )
    })
}

// Focus trap (keep focus within modal)
FocusTrap(
    Div(Class("modal"),
        Input(Type("text")),
        Button(Text("Cancel")),
        Button(Text("Submit")),
    ),
)
```

### 8.8 Scroll Management

```go
// Scroll into view
func scrollToCard(cardID string) {
    vango.ScrollIntoView(
        fmt.Sprintf("[data-card-id='%s']", cardID),
        ScrollConfig{
            Behavior: "smooth",      // "smooth" | "instant"
            Block:    "center",      // "start" | "center" | "end"
            Inline:   "nearest",
        },
    )
}

// Scroll position tracking
func InfiniteList() vango.Component {
    return vango.Func(func() *vango.VNode {
        items := vango.Signal(initialItems)
        loading := vango.Signal(false)

        return Div(
            Class("list-container"),
            OnScroll(func(e vango.ScrollEvent) {
                // Load more when near bottom
                if e.ScrollTop + e.ClientHeight >= e.ScrollHeight - 100 {
                    if !loading() {
                        loading.Set(true)
                        loadMore(items, loading)
                    }
                }
            }),

            Range(items(), ItemComponent),
            If(loading(), Spinner()),
        )
    })
}

// Preserve scroll position
func PreserveScroll(children ...any) *vango.VNode {
    return Div(
        ScrollRestore("list-scroll"),  // Key for storage
        children...,
    )
}
```

### 8.9 Touch Gestures (Mobile)

```go
// Swipe actions
Div(
    Swipeable(SwipeConfig{
        OnSwipeLeft: func() {
            showActions()
        },
        OnSwipeRight: func() {
            archive()
        },
        Threshold: 50,  // Minimum pixels to trigger
    }),

    ListItem(item),
)

// Long press
Div(
    OnLongPress(func() {
        showContextMenu()
    }, 500),  // 500ms threshold

    CardContent(card),
)

// Pinch zoom (for WASM components)
Canvas(
    OnPinch(func(e vango.PinchEvent) {
        setZoom(e.Scale)
    }),
)
```

### 8.10 Thin Client Implementation

The thin client handles events, patches, and hooks:

```javascript
// Approximate size breakdown
// Base thin client: ~12KB
// + Standard hooks (Sortable, Draggable, etc.): ~3KB
// + Simple optimistic updates: ~0.5KB
// + Keyboard/scroll utilities: ~0.5KB
// Total: ~16KB gzipped

class VangoClient {
    hooks = {};        // Registered hook implementations
    hookInstances = {} // Active hook instances per element

    constructor() {
        // Register standard hooks
        this.registerHook('Sortable', SortableHook);
        this.registerHook('Draggable', DraggableHook);
        this.registerHook('Droppable', DroppableHook);
        this.registerHook('Resizable', ResizableHook);
        this.registerHook('Tooltip', TooltipHook);
        this.registerHook('Dropdown', DropdownHook);
        this.registerHook('Collapsible', CollapsibleHook);
    }

    // Hook lifecycle management
    mountHook(el, hookName, config) {
        const Hook = this.hooks[hookName];
        if (!Hook) {
            console.warn(`Unknown hook: ${hookName}`);
            return;
        }

        const hid = el.dataset.hid;
        const pushEvent = (event, data) => {
            this.sendEvent(HOOK_EVENT, hid, { event, ...data });
        };

        const instance = new Hook();
        instance.mounted(el, config, pushEvent);
        this.hookInstances[hid] = { instance, hookName };
    }

    updateHook(el, config) {
        const hid = el.dataset.hid;
        const entry = this.hookInstances[hid];
        if (entry) {
            entry.instance.updated(el, config, (event, data) => {
                this.sendEvent(HOOK_EVENT, hid, { event, ...data });
            });
        }
    }

    destroyHook(hid) {
        const entry = this.hookInstances[hid];
        if (entry) {
            const el = document.querySelector(`[data-hid="${hid}"]`);
            entry.instance.destroyed(el);
            delete this.hookInstances[hid];
        }
    }

    // Simple optimistic updates (class, text, attr)
    applyOptimistic(el, type, value) {
        switch (type) {
            case 'class':
                el.classList.toggle(value.class, value.add);
                break;
            case 'text':
                el.textContent = value;
                break;
            case 'attr':
                el.setAttribute(value.name, value.value);
                break;
        }
    }

    // Keyboard scopes (server events)
    keyboardScopes = [];

    attachKeyboardListeners() {
        document.addEventListener('keydown', (e) => {
            const key = this.normalizeKey(e);

            // Check scopes from innermost to outermost
            for (const scope of [...this.keyboardScopes].reverse()) {
                if (scope.handlers[key]) {
                    e.preventDefault();
                    this.sendEvent(KEY, scope.hid, { key });
                    return;
                }
            }
        });
    }
}

// Standard hook implementation (simplified)
class SortableHook {
    mounted(el, config, pushEvent) {
        this.sortable = new Sortable(el, {
            animation: config.animation || 150,
            handle: config.handle,
            ghostClass: config.ghostClass || 'sortable-ghost',
            group: config.group,
            onEnd: (evt) => {
                pushEvent('reorder', {
                    id: evt.item.dataset.id,
                    fromIndex: evt.oldIndex,
                    toIndex: evt.newIndex,
                    fromGroup: evt.from.dataset.group,
                    toGroup: evt.to.dataset.group,
                });
            }
        });
    }

    updated(el, config, pushEvent) {
        // Update config if needed
    }

    destroyed(el) {
        this.sortable?.destroy();
    }
}
```

**Key architectural difference from the old design:**

| Old (Complex Predictions) | New (Hook Pattern) |
|--------------------------|-------------------|
| Client predicts DOM changes | Hook library handles DOM |
| Server streams drag events | Server only gets final result |
| Complex reconciliation logic | No reconciliation needed |
| ~2KB prediction engine | ~3KB proven libraries |

### 8.11 When to Use WASM Instead

Server events and client hooks handle most cases. Use WASM components when you need:

| Scenario | Why WASM |
|----------|----------|
| Physics simulation | Continuous calculation, not event-driven |
| Canvas drawing | <16ms feedback required |
| Complex gestures | Multi-touch, pressure, custom recognition |
| Heavy data processing | Filter/sort large datasets client-side |
| Offline computation | Work without server connection |

```go
// Example: Physics-based graph (WASM)
func ForceGraph(nodes []Node, edges []Edge) vango.Component {
    return vango.ClientRequired(func() *vango.VNode {
        // This runs entirely in WASM
        // because it needs continuous physics simulation

        canvasRef := vango.Ref[js.Value](nil)

        vango.Effect(func() vango.Cleanup {
            sim := physics.NewSimulation(canvasRef.Current())
            sim.SetNodes(nodes)
            sim.SetEdges(edges)
            sim.Start()  // Runs at 60fps

            return sim.Stop
        })

        return Canvas(
            Ref(canvasRef),
            Width(800),
            Height(600),
        )
    })
}
```

---

## 9. Routing & Navigation

### 9.1 File-Based Routing

```
app/routes/
├── index.go              → /
├── about.go              → /about
├── projects/
│   ├── index.go          → /projects
│   ├── new.go            → /projects/new
│   └── [id].go           → /projects/:id
├── api/
│   └── projects.go       → /api/projects (JSON)
└── _layout.go            → Wraps all routes
```

### 9.2 Page Components

```go
// app/routes/projects/[id].go
package routes

import (
    . "vango/el"
    "vango"
)

// Params are automatically typed from filename
type Params struct {
    ID int `param:"id"`
}

func Page(ctx vango.Ctx, p Params) vango.Component {
    return vango.Func(func() *vango.VNode {
        project := vango.Signal[*Project](nil)

        vango.Effect(func() vango.Cleanup {
            p, _ := db.Projects.FindByID(p.ID)
            project.Set(p)
            return nil
        })

        if project() == nil {
            return Loading()
        }

        return Div(Class("project-page"),
            H1(Text(project().Name)),
            P(Text(project().Description)),
            TaskBoard(project().Tasks),
        )
    })
}
```

### 9.3 Layouts

```go
// app/routes/_layout.go
package routes

func Layout(ctx vango.Ctx, children vango.Slot) *vango.VNode {
    return Html(
        Head(
            Title(Text("My App")),
            Link(Rel("stylesheet"), Href("/styles.css")),
        ),
        Body(
            Navbar(ctx.User()),
            Main(Class("container"), children),
            Footer(),
            VangoScripts(),  // Injects thin client
        ),
    )
}
```

### 9.4 Navigation

```go
// Programmatic navigation
func handleSave() {
    project := saveProject()
    vango.Navigate("/projects/" + project.ID)
}

// Link component
A(Href("/projects/123"), Text("View Project"))

// With prefetch (loads data before navigation)
A(
    Href("/projects/123"),
    Prefetch(),  // Preloads on hover
    Text("View Project"),
)
```

### 9.5 How Navigation Works

**Server-Driven Mode:**
```
1. User clicks link to /projects/123
2. Thin client intercepts, sends NAVIGATE event
3. Server:
   - Matches route
   - Mounts new page component
   - Renders to VNode
   - Diffs against current page
   - Sends patches (often just replacing <main> content)
4. Client applies patches
5. URL updates via History API
```

No full page reload, no WASM download, minimal data transfer.

---

## 10. Data & APIs

### 10.1 Direct Database Access

In server-driven mode, components have direct access to backend:

```go
func UserList() vango.Component {
    return vango.Func(func() *vango.VNode {
        users := vango.Signal([]User{})
        search := vango.Signal("")

        vango.Effect(func() vango.Cleanup {
            // Direct database query - no HTTP, no JSON!
            results, _ := db.Users.Search(search())
            users.Set(results)
            return nil
        })

        return Div(
            Input(
                Type("search"),
                Value(search()),
                OnInput(search.Set),
                Placeholder("Search users..."),
            ),
            Ul(
                Range(users(), func(u User, i int) *vango.VNode {
                    return Li(Key(u.ID), Text(u.Name))
                }),
            ),
        )
    })
}
```

### 10.2 API Routes

For external clients (mobile apps, third parties):

```go
// app/routes/api/projects.go
package api

func GET(ctx vango.Ctx) ([]Project, error) {
    return db.Projects.All()
}

func POST(ctx vango.Ctx, input CreateProjectInput) (*Project, error) {
    if err := validate(input); err != nil {
        return nil, vango.BadRequest(err)
    }
    return db.Projects.Create(input)
}
```

Generated endpoints:
- `GET /api/projects` → Returns JSON array
- `POST /api/projects` → Creates project, returns JSON

### 10.3 External API Calls

```go
func WeatherWidget(city string) vango.Component {
    return vango.Func(func() *vango.VNode {
        weather := vango.Signal[*Weather](nil)

        vango.Effect(func() vango.Cleanup {
            // HTTP call from server (not browser!)
            resp, _ := http.Get("https://api.weather.com/v1/" + city)
            var w Weather
            json.NewDecoder(resp.Body).Decode(&w)
            weather.Set(&w)
            return nil
        })

        if weather() == nil {
            return Loading()
        }

        return Div(Class("weather"),
            Text(weather().Description),
            Text(fmt.Sprintf("%.1f°C", weather().Temp)),
        )
    })
}
```

Benefits:
- API keys stay on server (secure)
- No CORS issues
- Server can cache responses

---

## 11. Forms & Validation

### 11.1 Basic Forms

```go
func LoginForm() vango.Component {
    return vango.Func(func() *vango.VNode {
        email := vango.Signal("")
        password := vango.Signal("")
        error := vango.Signal("")

        submit := func() {
            user, err := auth.Login(email(), password())
            if err != nil {
                error.Set(err.Error())
                return
            }
            vango.SetSession(user)
            vango.Navigate("/dashboard")
        }

        return Form(OnSubmit(submit),
            If(error() != "",
                Div(Class("error"), Text(error())),
            ),

            Label(Text("Email")),
            Input(
                Type("email"),
                Value(email()),
                OnInput(email.Set),
                Required(),
            ),

            Label(Text("Password")),
            Input(
                Type("password"),
                Value(password()),
                OnInput(password.Set),
                Required(),
            ),

            Button(Type("submit"), Text("Login")),
        )
    })
}
```

### 11.2 Form Library

```go
func ContactForm() vango.Component {
    return vango.Func(func() *vango.VNode {
        form := vango.UseForm(ContactInput{})

        submit := func() {
            if !form.Validate() {
                return
            }
            sendEmail(form.Values())
            form.Reset()
        }

        return Form(OnSubmit(submit),
            form.Field("Name",
                Input(Type("text")),
                vango.Required("Name is required"),
                vango.MinLength(2, "Name too short"),
            ),

            form.Field("Email",
                Input(Type("email")),
                vango.Required("Email is required"),
                vango.Email("Invalid email"),
            ),

            form.Field("Message",
                Textarea(),
                vango.Required("Message is required"),
                vango.MaxLength(1000, "Message too long"),
            ),

            Button(
                Type("submit"),
                Disabled(form.Submitting()),
                Text("Send"),
            ),
        )
    })
}
```

### 11.3 Progressive Enhancement

Forms work without JavaScript:

```go
Form(
    Method("POST"),
    Action("/api/contact"),  // Fallback for no-JS
    OnSubmit(handleSubmit),  // Enhanced with WS when available
    // ...
)
```

### 11.4 Toast Notifications

Since Vango uses persistent WebSocket connections, traditional HTTP flash cookies don't work. Instead, use the toast package:

```go
import "github.com/vango-dev/vango/v2/pkg/toast"

func DeleteProject(ctx vango.Ctx, id int) error {
    if err := db.Projects.Delete(id); err != nil {
        toast.Error(ctx, "Failed to delete project")
        return err
    }
    
    toast.Success(ctx, "Project deleted")
    ctx.Navigate("/projects")
    return nil
}
```

**Client-Side Handler** (user provides):
```javascript
// Listen for toast events and render with your preferred library
window.addEventListener("vango:toast", (e) => {
    Toastify({ text: e.detail.message, className: e.detail.level }).showToast();
});
```

### 11.5 File Uploads

Large file uploads over WebSocket block the event loop. Use the hybrid HTTP+WS approach:

```go
import "github.com/vango-dev/vango/v2/pkg/upload"

// 1. Mount upload handler (main.go)
r.Post("/upload", upload.Handler(uploadStore))

// 2. Handle in component (after client POSTs and receives temp_id via WS form)
func CreatePost(ctx vango.Ctx, formData vango.FormData) error {
    tempID := formData.Get("attachment_temp_id")
    
    if tempID != "" {
        file, err := upload.Claim(uploadStore, tempID)
        if err != nil {
            return err
        }
        // Use file.Path or file.URL
    }
    
    toast.Success(ctx, "Post created!")
    return nil
}
```

---

## 12. JavaScript Islands

### 12.1 When to Use

Use JS islands for:
- Third-party libraries (charts, rich text editors, maps)
- Browser APIs not exposed to WASM
- Existing JS widgets during migration

### 12.2 Basic Usage

```go
func AnalyticsDashboard(data []DataPoint) *vango.VNode {
    return Div(Class("dashboard"),
        H1(Text("Analytics")),

        // JavaScript island for chart library
        JSIsland("revenue-chart",
            JSModule("/js/charts.js"),
            JSProps{
                "data":   data,
                "type":   "line",
                "height": 400,
            },
        ),
    )
}
```

### 12.3 JavaScript Side

```javascript
// public/js/charts.js
import { Chart } from 'chart.js';

export function mount(container, props) {
    const chart = new Chart(container, {
        type: props.type,
        data: formatData(props.data),
        options: { maintainAspectRatio: false }
    });

    // Return cleanup function
    return () => chart.destroy();
}

// Called when props change (optional)
export function update(container, props, chart) {
    chart.data = formatData(props.data);
    chart.update();
}
```

### 12.4 Communication Bridge

```go
// Send data to JS island
vango.SendToIsland("revenue-chart", map[string]any{
    "action": "highlight",
    "series": "revenue",
})

// Receive events from JS island
vango.OnIslandMessage("revenue-chart", func(msg map[string]any) {
    if msg["event"] == "point-click" {
        showDetails(msg["dataIndex"].(int))
    }
})
```

```javascript
// In charts.js
import { sendToVango, onVangoMessage } from '@vango/bridge';

chart.on('click', (e) => {
    sendToVango('revenue-chart', {
        event: 'point-click',
        dataIndex: e.dataIndex
    });
});

onVangoMessage('revenue-chart', (msg) => {
    if (msg.action === 'highlight') {
        chart.highlightSeries(msg.series);
    }
});
```

### 12.5 SSR Behavior

Islands render as placeholders during SSR:

```html
<div id="revenue-chart"
     data-island="true"
     data-island-module="/js/charts.js"
     data-island-props='{"type":"line","height":400}'>
    <!-- Optional loading skeleton -->
    <div class="chart-skeleton"></div>
</div>
```

The thin client hydrates islands after connecting:

```javascript
// In thin client
document.querySelectorAll('[data-island]').forEach(async (el) => {
    const mod = await import(el.dataset.islandModule);
    const props = JSON.parse(el.dataset.islandProps);
    el._cleanup = mod.mount(el, props);
});
```

---

## 12. Styling

### 12.1 Global CSS

```go
// In layout
Head(
    Link(Rel("stylesheet"), Href("/styles.css")),
)
```

### 12.2 Tailwind CSS

Vango integrates with Tailwind automatically:

```go
// Just use Tailwind classes
Div(Class("flex items-center justify-between p-4 bg-white shadow"),
    H1(Class("text-2xl font-bold text-gray-900"), Text("Title")),
    Button(Class("px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"),
        Text("Action"),
    ),
)
```

```bash
$ vango dev
→ Detected tailwind.config.js
→ Running Tailwind CSS in watch mode...
```

### 12.3 CSS Variables for Theming

```go
func ThemeProvider(theme Theme, children ...any) *vango.VNode {
    return Div(
        Style(fmt.Sprintf(`
            --color-primary: %s;
            --color-secondary: %s;
            --color-background: %s;
        `, theme.Primary, theme.Secondary, theme.Background)),
        children...,
    )
}
```

### 12.4 Dynamic Styles

```go
func ProgressBar(percent int) *vango.VNode {
    return Div(Class("progress-bar"),
        Div(
            Class("progress-fill"),
            Style(fmt.Sprintf("width: %d%%", percent)),
        ),
    )
}
```

---

## 13. Performance & Scaling

### 13.1 Server Resource Usage

**Memory per session:**
| App Complexity | Typical Memory |
|----------------|----------------|
| Simple (blog, marketing) | 10-50 KB |
| Medium (dashboard, CRUD) | 50-200 KB |
| Complex (project management) | 200 KB - 1 MB |

**Scaling calculation:**
```
1,000 concurrent users × 200 KB = 200 MB
10,000 concurrent users × 200 KB = 2 GB
100,000 concurrent users × 200 KB = 20 GB
```

This is manageable. A single 32 GB server can handle 100k+ concurrent users.

### 13.2 Reducing Memory Usage

**Stateless pages:**
```go
// For read-only pages, don't maintain session state
func BlogPost(slug string) *vango.VNode {
    post := db.Posts.FindBySlug(slug)
    return Article(
        H1(Text(post.Title)),
        Div(DangerouslySetInnerHTML(post.Content)),
    )
}
```

**Automatic cleanup:**
```go
// Sessions are evicted after inactivity
vango.Config{
    SessionTimeout: 5 * time.Minute,
    SessionMaxAge:  1 * time.Hour,
}
```

**State externalization:**
```go
// Store large state in Redis, not memory
tasks := vango.Signal([]Task{}).Store(redis.Store)
```

### 13.3 WebSocket Scaling

Go handles WebSocket connections efficiently:

```go
// Using efficient connection pooling
vango.Config{
    MaxConnsPerSession: 1,  // One WS per tab
    ReadBufferSize:     1024,
    WriteBufferSize:    1024,
    EnableCompression:  true,
}
```

For horizontal scaling:
- Use sticky sessions (route by session ID)
- Or use Redis pub/sub for cross-server messaging

### 13.4 Latency Optimization

**Server location matters:**
| User Location | Server Location | Round-trip |
|---------------|-----------------|------------|
| NYC | NYC | 5-10ms |
| NYC | SF | 40-60ms |
| NYC | London | 70-90ms |
| NYC | Tokyo | 150-200ms |

**Recommendations:**
- Deploy in regions close to users
- Use edge locations for static assets
- Consider optimistic updates for high-latency scenarios

### 13.5 Bundle Size

| Mode | Client Size (gzip) |
|------|-------------------|
| Server-Driven | ~12 KB |
| Server-Driven + Optimistic | ~15 KB |
| Hybrid (partial WASM) | 12 KB + WASM components |
| Full WASM | ~250-400 KB |

---

## 14. Security

Vango provides **security by design** with secure defaults that protect against common vulnerabilities.

### 14.0 Secure Defaults (v2.1+)

| Setting | Default | Notes |
|---------|---------|-------|
| `CheckOrigin` | Same-origin only | Cross-origin WS rejected |
| CSRF | Warning if disabled | Required in v3.0 |
| `on*` attributes | Stripped unless handler | Prevents XSS injection |
| Protocol limits | 4MB max allocation | Prevents DoS |

### 14.1 XSS Prevention

#### Text Escaping

All text content is escaped by default:

```go
// Safe - content is escaped
Div(Text(userInput))  // <script> becomes &lt;script&gt;

// Explicit opt-in for raw HTML
Div(DangerouslySetInnerHTML(trustedHTML))
```

#### Attribute Sanitization

Event handler attributes (`onclick`, `onmouseover`, etc.) are automatically filtered:

```go
// This is BLOCKED - attribute stripped during render
Attr("onclick", "alert(1)")

// This is SAFE - uses internal event handler
OnClick(myHandler)
```

> **Note**: The filter is case-insensitive. `ONCLICK`, `onClick`, and `onclick` are all blocked.

### 14.2 CSRF Protection

Enable CSRF protection in production:

```go
vango.Config{
    CSRFSecret: []byte("your-32-byte-secret-key-here!!"),
}
```

CSRF uses the **Double Submit Cookie** pattern:
1. Server sets `__vango_csrf` cookie via `server.SetCSRFCookie()`
2. Server embeds token in HTML as `window.__VANGO_CSRF__`
3. Client sends token in WebSocket handshake
4. Server validates handshake token matches cookie

```go
// In your page handler
func ServePage(w http.ResponseWriter, r *http.Request) {
    token := server.GenerateCSRFToken()
    server.SetCSRFCookie(w, token)
    // Embed token in page for client
}
```

> **Warning**: If `CSRFSecret` is nil, a warning is logged on startup. This will become a hard error in v3.0.

### 14.3 WebSocket Origin Validation

By default, Vango rejects cross-origin WebSocket connections (prevents CSWSH):

```go
// Default behavior - same-origin only
config := server.DefaultServerConfig()
// config.CheckOrigin = SameOriginCheck (secure default)

// Explicit cross-origin (dev only!)
config.CheckOrigin = func(r *http.Request) bool { return true }
```

### 14.4 Session Security

```go
vango.Config{
    SessionCookie: http.Cookie{
        Name:     "vango_session",
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
    },
}
```

### 14.5 Protocol Security

The binary protocol includes allocation limits to prevent DoS attacks:

| Limit | Value | Purpose |
|-------|-------|---------|
| Max string/bytes | 4MB | Prevent OOM |
| Max collection | 100K items | Prevent CPU exhaustion |
| Hard cap | 16MB | Absolute ceiling |

### 14.6 Event Handler Safety

Handlers are server-side function references, not code strings:

```go
// This creates a server-side handler mapping
Button(OnClick(func() {
    doSensitiveAction()  // Runs on server
}))
```

The client only sends `{hid: "h42", type: 0x01}`. It cannot:
- Execute arbitrary functions
- Access handlers from other sessions
- Inject JavaScript

### 14.5 Input Validation

```go
func CreateProject(ctx vango.Ctx, input CreateInput) (*Project, error) {
    // Validate on server (always!)
    if err := validate.Struct(input); err != nil {
        return nil, vango.BadRequest(err)
    }

    // Sanitize
    input.Name = sanitize.String(input.Name)

    return db.Projects.Create(input)
}
```

### 14.6 Authentication & Middleware

Vango uses a **dual-layer architecture** that separates HTTP middleware from Vango's event-loop middleware:

**Layer 1: HTTP Stack** (runs once per session):
- Standard `func(http.Handler) http.Handler` middleware
- Authentication, CORS, logging, panic recovery
- Compatible with Chi, Gorilla, rs/cors, etc.

**Layer 2: Vango Event Stack** (runs on every interaction):
- Lightweight `func(ctx vango.Ctx, next func() error) error` middleware  
- Authorization guards (RBAC), event validation
- No HTTP overhead on the hot path

#### The Context Bridge

The WebSocket upgrade presents a challenge: HTTP request context dies after the upgrade. The Context Bridge solves this:

```go
app := vango.New(vango.Config{
    // This runs ONCE during WebSocket upgrade
    OnSessionStart: func(httpCtx context.Context, s *vango.Session) {
        // Copy user from HTTP context to Vango session
        if user := myauth.UserFromContext(httpCtx); user != nil {
            auth.Set(s, user)
        }
    },
})
```

#### Type-Safe Auth Package

```go
import "github.com/vango-dev/vango/v2/pkg/auth"

// Require auth (returns error if not logged in)
func Dashboard(ctx vango.Ctx) (vango.Component, error) {
    user, err := auth.Require[*User](ctx)
    if err != nil {
        return nil, err  // ErrorBoundary handles redirect
    }
    return renderDashboard(user), nil
}

// Optional auth (guest allowed)
func HomePage(ctx vango.Ctx) vango.Component {
    user, ok := auth.Get[*User](ctx)
    if ok {
        return LoggedInHome(user)
    }
    return GuestHome()
}
```

#### Integration with Chi Router

Vango exposes itself as a standard `http.Handler` for ecosystem compatibility:

```go
func main() {
    app := vango.New(vango.Config{
        OnSessionStart: hydrateSession,
    })

    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(AuthMiddleware)  // Your auth middleware
    
    r.Get("/api/health", healthHandler)
    r.Handle("/*", app.Handler())  // Vango handles the rest
    
    http.ListenAndServe(":3000", r)
}
```

#### Route-Level Auth Guards

```go
// app/routes/admin/_layout.go
func Middleware() []router.Middleware {
    return []router.Middleware{
        auth.RequireRole(func(u *User) bool {
            return u.IsAdmin
        }),
    }
}
```

---

## 15. Testing

### 15.1 Unit Testing Components

```go
func TestCounter(t *testing.T) {
    // Create test context
    ctx := vango.TestContext()

    // Mount component
    c := Counter(5)
    tree := ctx.Mount(c)

    // Assert initial render
    assert.Contains(t, tree.Text(), "Count: 5")

    // Simulate click
    ctx.Click("[data-testid=increment]")

    // Assert update
    assert.Contains(t, tree.Text(), "Count: 6")
}
```

### 15.2 Testing with Signals

```go
func TestSignalUpdates(t *testing.T) {
    ctx := vango.TestContext()

    count := ctx.Signal(0)
    tree := ctx.Render(func() *vango.VNode {
        return Div(Textf("Count: %d", count()))
    })

    assert.Equal(t, "Count: 0", tree.Text())

    count.Set(10)
    ctx.Flush()  // Process signal updates

    assert.Equal(t, "Count: 10", tree.Text())
}
```

### 15.3 Integration Testing

```go
func TestLoginFlow(t *testing.T) {
    app := vango.TestApp()

    // Navigate to login
    page := app.Navigate("/login")

    // Fill form
    page.Fill("[name=email]", "test@example.com")
    page.Fill("[name=password]", "password123")
    page.Click("[type=submit]")

    // Assert redirect
    assert.Equal(t, "/dashboard", page.URL())
    assert.Contains(t, page.Text(), "Welcome back")
}
```

### 15.4 E2E Testing (Playwright)

```typescript
// tests/login.spec.ts
test('user can log in', async ({ page }) => {
    await page.goto('/login');

    await page.fill('[name=email]', 'test@example.com');
    await page.fill('[name=password]', 'password123');
    await page.click('[type=submit]');

    await expect(page).toHaveURL('/dashboard');
    await expect(page.locator('h1')).toContainText('Welcome');
});
```

---

## 16. Developer Experience

### 16.1 Project Structure

```
my-app/
├── app/
│   ├── routes/           # Page components (file-based routing)
│   │   ├── index.go
│   │   ├── about.go
│   │   └── projects/
│   │       ├── index.go
│   │       └── [id].go
│   ├── components/       # Shared components
│   │   ├── button.go
│   │   ├── card.go
│   │   └── navbar.go
│   ├── layouts/          # Layout components
│   │   └── main.go
│   └── store/            # Shared state
│       └── user.go
├── public/               # Static assets
│   ├── styles.css
│   └── images/
├── db/                   # Database layer
│   └── models.go
├── go.mod
├── go.sum
└── vango.json            # Configuration
```

### 16.2 CLI Commands

```bash
# Create new project
vango create my-app

# Development server (hot reload)
vango dev

# Production build
vango build

# Run tests
vango test

# Generate routes
vango gen routes
```

### 16.3 Hot Reload

```bash
$ vango dev
→ Server starting on http://localhost:3000
→ Watching for changes...

[12:34:56] Changed: app/components/button.go
[12:34:56] Rebuilding... (42ms)
[12:34:56] Reloaded 2 connected clients
```

Changes are instant:
1. File change detected
2. Go recompilation (~50ms for incremental)
3. Connected browsers receive refresh signal
4. Only affected components re-render

### 16.4 Error Messages

**Compile-time errors:**
```
app/routes/projects/[id].go:23:15: cannot use string as int in argument
    project := db.Projects.FindByID(params.ID)
                                    ^^^^^^^^^
    Hint: params.ID is string, but FindByID expects int
          Use: strconv.Atoi(params.ID)
```

**Runtime errors:**
```
ERROR in /projects/123
  app/routes/projects/[id].go:45

  Signal read outside component context

    count := vango.Signal(0)
    value := count()  // ← Error: no active component

  Hint: Signal reads must happen inside a component's render function
        or an Effect. Move this code inside vango.Func(func() {...})
```

**Hydration mismatches (dev mode):**
```
HYDRATION MISMATCH at /dashboard

  Server rendered:
    <div class="status">Offline</div>

  Client expected:
    <div class="status">Online</div>

  Difference: text content

  Location: app/components/status.go:12

  Hint: This component reads browser-only state (navigator.onLine)
        during render. Use an Effect instead:

        vango.Effect(func() vango.Cleanup {
            status.Set(getOnlineStatus())
            return nil
        })
```

### 16.5 VS Code Extension

- Syntax highlighting for Vango components
- Go to definition for components
- Autocomplete for element attributes
- Error highlighting
- Hot reload integration

---

## 17. Migration Guide

### 17.1 From React

**React:**
```jsx
function Counter({ initial }) {
    const [count, setCount] = useState(initial);

    return (
        <div className="counter">
            <h1>Count: {count}</h1>
            <button onClick={() => setCount(c => c + 1)}>+</button>
        </div>
    );
}
```

**Vango:**
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

**Key differences:**
| React | Vango |
|-------|-------|
| `useState` | `vango.Signal` |
| `useEffect` | `vango.Effect` |
| `useMemo` | `vango.Memo` |
| JSX | Function calls |
| Runs in browser | Runs on server |

### 17.2 From Vue

**Vue:**
```vue
<template>
    <div class="counter">
        <h1>Count: {{ count }}</h1>
        <button @click="count++">+</button>
    </div>
</template>

<script setup>
import { ref } from 'vue'
const count = ref(0)
</script>
```

**Vango:**
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

### 17.3 Gradual Migration

You can migrate incrementally:

1. **Add Vango to existing Go backend**
2. **Create new pages in Vango**
3. **Use JS islands for existing React components**
4. **Gradually rewrite components in Go**

```go
// During migration: wrap existing React component
func LegacyDashboard() *vango.VNode {
    return JSIsland("dashboard",
        JSModule("/js/legacy/dashboard.js"),  // Existing React code
        JSProps{"user": currentUser},
    )
}
```

---

## 18. Examples

### 18.1 Todo App

```go
// app/routes/todos.go
package routes

func Page(ctx vango.Ctx) vango.Component {
    return vango.Func(func() *vango.VNode {
        todos := vango.Signal([]Todo{})
        newTodo := vango.Signal("")

        // Load todos from database
        vango.Effect(func() vango.Cleanup {
            items, _ := db.Todos.ForUser(ctx.UserID())
            todos.Set(items)
            return nil
        })

        addTodo := func() {
            if newTodo() == "" {
                return
            }
            todo := db.Todos.Create(ctx.UserID(), newTodo())
            todos.Update(func(t []Todo) []Todo {
                return append(t, todo)
            })
            newTodo.Set("")
        }

        toggleTodo := func(id int) func() {
            return func() {
                db.Todos.Toggle(id)
                todos.Update(func(t []Todo) []Todo {
                    for i := range t {
                        if t[i].ID == id {
                            t[i].Done = !t[i].Done
                        }
                    }
                    return t
                })
            }
        }

        return Div(Class("todo-app"),
            H1(Text("My Todos")),

            Form(OnSubmit(addTodo), Class("add-form"),
                Input(
                    Type("text"),
                    Value(newTodo()),
                    OnInput(newTodo.Set),
                    Placeholder("What needs to be done?"),
                ),
                Button(Type("submit"), Text("Add")),
            ),

            Ul(Class("todo-list"),
                Range(todos(), func(todo Todo, i int) *vango.VNode {
                    return Li(
                        Key(todo.ID),
                        Class("todo-item"),
                        ClassIf(todo.Done, "completed"),

                        Input(
                            Type("checkbox"),
                            Checked(todo.Done),
                            OnChange(toggleTodo(todo.ID)),
                        ),
                        Span(Text(todo.Text)),
                    )
                }),
            ),
        )
    })
}
```

### 18.2 Real-time Chat

```go
func ChatRoom(roomID string) vango.Component {
    return vango.Func(func() *vango.VNode {
        messages := vango.GlobalSignal([]Message{})  // Shared across all users
        input := vango.Signal("")

        sendMessage := func() {
            if input() == "" {
                return
            }
            msg := Message{
                User:    currentUser(),
                Text:    input(),
                Time:    time.Now(),
            }
            messages.Update(func(m []Message) []Message {
                return append(m, msg)
            })
            input.Set("")
        }

        return Div(Class("chat-room"),
            Div(Class("messages"),
                Range(messages(), func(msg Message, i int) *vango.VNode {
                    return Div(Class("message"),
                        Strong(Text(msg.User.Name)),
                        Span(Text(msg.Text)),
                        Time_(Text(msg.Time.Format("3:04 PM"))),
                    )
                }),
            ),

            Form(OnSubmit(sendMessage), Class("input-area"),
                Input(
                    Type("text"),
                    Value(input()),
                    OnInput(input.Set),
                    Placeholder("Type a message..."),
                ),
                Button(Type("submit"), Text("Send")),
            ),
        )
    })
}
```

### 18.3 Dashboard with Charts

```go
func Dashboard() vango.Component {
    return vango.Func(func() *vango.VNode {
        stats := vango.Signal[*Stats](nil)
        period := vango.Signal("week")

        vango.Effect(func() vango.Cleanup {
            s, _ := analytics.GetStats(period())
            stats.Set(s)
            return nil
        })

        if stats() == nil {
            return Loading()
        }

        return Div(Class("dashboard"),
            Header(
                H1(Text("Dashboard")),
                Select(
                    Value(period()),
                    OnChange(period.Set),
                    Option(Value("day"), Text("Today")),
                    Option(Value("week"), Text("This Week")),
                    Option(Value("month"), Text("This Month")),
                ),
            ),

            Div(Class("stats-grid"),
                StatCard("Revenue", stats().Revenue, "+12%"),
                StatCard("Users", stats().Users, "+5%"),
                StatCard("Orders", stats().Orders, "+8%"),
            ),

            // JS island for complex chart
            JSIsland("revenue-chart",
                JSModule("/js/charts.js"),
                JSProps{
                    "data": stats().RevenueHistory,
                    "type": "area",
                },
            ),
        )
    })
}
```

---

## 19. FAQ

### General

**Q: Is this like Phoenix LiveView?**
A: Yes! Vango is inspired by LiveView but for Go. Server-driven UI with binary patches over WebSocket.

**Q: Do I need to know JavaScript?**
A: For most apps, no. You only need JS for islands (third-party libraries) or very latency-sensitive features.

**Q: What about SEO?**
A: SSR is built-in. Search engines see fully-rendered HTML. No JavaScript required for content.

**Q: Can I use this for mobile apps?**
A: Vango is for web apps. For mobile, consider using the API routes with a native app, or a WebView wrapper.

### Performance

**Q: What's the latency for interactions?**
A: Typically 50-100ms (network round-trip + processing). Use optimistic updates for instant feel.

**Q: How many concurrent users can one server handle?**
A: Depends on complexity, but typically 10,000-100,000+ with proper session management.

**Q: Is the 12KB client cached?**
A: Yes. After first load, it's served from browser cache. Only the WebSocket connection is new.

### Architecture

**Q: What happens if WebSocket disconnects?**
A: The client auto-reconnects. Server sends full state on reconnect. No manual sync needed.

**Q: Can I use this with an existing Go backend?**
A: Yes! Vango integrates with `net/http`. Mount it alongside your existing API routes.

**Q: How does authentication work?**
A: Use your existing auth. Vango reads session cookies. User is available via `ctx.User()`.

### Development

**Q: Is hot reload fast?**
A: Yes. Go incremental compilation is ~50ms. Changes appear instantly in browser.

**Q: Can I debug server-side code?**
A: Yes. Use Delve or your IDE's debugger. Set breakpoints in event handlers.

**Q: How do I deploy?**
A: Single binary. Deploy like any Go server. No Node.js, no build step in production.

---

## 20. Appendix: Protocol Specification

### 20.1 WebSocket Handshake

```
Client → Server:
{
    "type": "HANDSHAKE",
    "version": "1.0",
    "csrf": "<token>",
    "session": "<session-id>",  // From cookie, if reconnecting
    "viewport": {"width": 1920, "height": 1080}
}

Server → Client:
{
    "type": "HANDSHAKE_ACK",
    "session": "<new-or-existing-session-id>",
    "serverTime": 1699999999999
}
```

### 20.2 Binary Event Format

```
┌─────────────────────────────────────────────────────────────┐
│ Byte 0    │ Bytes 1-N      │ Remaining bytes               │
│ EventType │ HID (varint)   │ Payload (type-specific)       │
└─────────────────────────────────────────────────────────────┘

EventType values:
  0x01: CLICK
  0x02: DBLCLICK
  0x03: INPUT          Payload: [varint: length][utf8: value]
  0x04: CHANGE         Payload: [varint: length][utf8: value]
  0x05: SUBMIT         Payload: [form encoding]
  0x06: FOCUS
  0x07: BLUR
  0x08: KEYDOWN        Payload: [key: uint16][modifiers: uint8]
  0x09: KEYUP          Payload: [key: uint16][modifiers: uint8]
  0x0A: MOUSEENTER
  0x0B: MOUSELEAVE
  0x0C: SCROLL         Payload: [scrollX: int32][scrollY: int32]
  0x0D: NAVIGATE       Payload: [varint: length][utf8: path]
  0x0E: CUSTOM         Payload: [varint: type][varint: length][data]
```

### 20.3 Binary Patch Format

```
┌─────────────────────────────────────────────────────────────┐
│ Bytes 0-N        │ Patches...                               │
│ Count (varint)   │                                          │
└─────────────────────────────────────────────────────────────┘

Each Patch:
┌─────────────────────────────────────────────────────────────┐
│ Byte 0     │ Bytes 1-N      │ Remaining bytes              │
│ PatchType  │ HID (varint)   │ Payload (type-specific)      │
└─────────────────────────────────────────────────────────────┘

PatchType values:
  0x01: SET_TEXT       Payload: [varint: length][utf8: text]
  0x02: SET_ATTR       Payload: [varint: key-len][key][varint: val-len][val]
  0x03: REMOVE_ATTR    Payload: [varint: key-len][key]
  0x04: ADD_CLASS      Payload: [varint: length][utf8: class]
  0x05: REMOVE_CLASS   Payload: [varint: length][utf8: class]
  0x06: SET_STYLE      Payload: [varint: prop-len][prop][varint: val-len][val]
  0x07: INSERT_BEFORE  Payload: [varint: ref-hid][encoded-vnode]
  0x08: INSERT_AFTER   Payload: [varint: ref-hid][encoded-vnode]
  0x09: APPEND_CHILD   Payload: [encoded-vnode]
  0x0A: REMOVE_NODE    Payload: (none)
  0x0B: REPLACE_NODE   Payload: [encoded-vnode]
  0x0C: SET_VALUE      Payload: [varint: length][utf8: value]
  0x0D: SET_CHECKED    Payload: [bool: checked]
  0x0E: SET_SELECTED   Payload: [bool: selected]
  0x0F: FOCUS          Payload: (none)
  0x10: BLUR           Payload: (none)
  0x11: SCROLL_TO      Payload: [int32: x][int32: y]
```

### 20.4 VNode Encoding

```
┌─────────────────────────────────────────────────────────────┐
│ Byte 0    │ Remaining bytes (type-specific)                 │
│ NodeType  │                                                 │
└─────────────────────────────────────────────────────────────┘

NodeType 0x01: Element
  [varint: tag-length][tag]
  [varint: hid]  // 0 if no hid
  [varint: attr-count]
  for each attr:
    [varint: key-length][key][varint: val-length][val]
  [varint: child-count]
  for each child:
    [encoded-vnode]  // Recursive

NodeType 0x02: Text
  [varint: length][utf8: text]

NodeType 0x03: Fragment
  [varint: child-count]
  for each child:
    [encoded-vnode]
```

### 20.5 Varint Encoding

Unsigned variable-length integer (same as Protocol Buffers):

```
Value 0-127:        1 byte   [0xxxxxxx]
Value 128-16383:    2 bytes  [1xxxxxxx] [0xxxxxxx]
Value 16384+:       3+ bytes [1xxxxxxx] [1xxxxxxx] [0xxxxxxx] ...
```

This keeps small numbers (most HIDs, lengths) as single bytes.

---

## Changelog

| Date | Version | Notes |
|------|---------|-------|
| 2024-12-06 | 2.0 | Complete rewrite: server-driven primary architecture |

---

*This document is the authoritative reference for Vango's architecture. For implementation details, see the source code and inline documentation.*


------------------
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

The architecture is now production-ready. The main remaining work is implementation and documentation.


---
Making Vango Mobile #still in design phase and not implemented.
---

**Vango-Native** is not just viable—it is the logical endpoint of the Vango architecture.



The mobile industry is currently struggling with the "updates problem": you have to recompile, re-sign, and wait for App Store review for every UI tweak. React Native solves the cross-platform part but still bundles logic on the device.



By moving the **logic** to the server (Go) and leaving only a **rendering engine** (The Player) on the device, you get "Over-The-Air" updates by default for the entire app.



Here is the deep dive into how **Vango-Native** would work, its architecture, and the Developer Experience (DX).



-----



### 1\. Architecture: The "Standard Player"



Unlike React Native, which ships a JavaScript VM (Hermes) and your business logic in a JS bundle, Vango-Native ships a **generic native shell**.



We can call this the **Vango Player**.



#### The Stack



* **Server (Go):** Runs all business logic, state management, and API calls.

* **Protocol (Binary):** The exact same encoded binary stream from Vango V2.

* **Client (iOS/Android):** A thin native app that does three things:

1. **Decode:** Deserializes `SET_ATTR` or `INSERT_NODE` instructions.

2. **Layout:** Uses **Yoga** (the C++ layout engine used by React Native) to calculate Flexbox layouts.

3. **Render:** Maps Vango Nodes to real native views (`UIView` on iOS, `android.view.View` on Android).



#### Why this is faster than React Native



React Native has a "Bridge" bottleneck: Serializing JSON ↔ JavaScript VM ↔ Native Modules.

Vango-Native removes the JS VM entirely. The binary stream talks directly to the Native/C++ layer.



-----



### 2\. The Component Model



Vango-Native needs a new package, `vango/native`, which provides type-safe Go wrappers for standard mobile UI elements.



```go

package main



import (

"vango"

. "vango/native" // The Native Component Library

)



func UserProfile(u User) *vango.VNode {

return View(

Style(

FlexDirection("row"),

Padding(16),

BackgroundColor("#FFFFFF"),

CornerRadius(8),

),

// Image is a native UIImageView / ImageView

Image(

Source(u.AvatarURL),

Style(Width(50), Height(50), BorderRadius(25)),

),

View(

Style(MarginLeft(12), JustifyContent("center")),

Text(

Content(u.Name),

Style(FontSize(18), FontWeight("bold"), Color("#333")),

),

Text(

Content("Status: Online"),

Style(FontSize(14), Color("green")),

),

),

)

}

```



**Key Difference:** On the web, `Div()` renders a `<div>`. In Vango-Native, `View()` sends a binary instruction that the iOS Player interprets as `new UIStackView()`.



-----



### 3\. Developer Experience (DX)



This is where Vango-Native shines. The feedback loop is instantaneous because there is no compilation step for the client.



#### The "Vango Go" App



Imagine a generic app in the App Store called **"Vango Go"** (similar to the Expo Go app for React Native).



**The Workflow:**



1. **Start Server:** You run `go run main.go` on your laptop.

2. **Open App:** You open "Vango Go" on your iPhone.

3. **Scan:** You scan a QR code on your terminal.

4. **Develop:**

* You change `Color("green")` to `Color("blue")` in Go.

* The server recompiles (50ms).

* The server pushes a binary patch to the phone.

* The phone updates **instantly** without reloading the full app context.



#### No "Native Builds"



For 95% of development, you never touch Xcode or Android Studio. You are just writing Go code. The "Standard Player" already has the map, camera, and video components pre-compiled.



-----



### 4\. Handling Complex Interactions (The "Native Hook")



Latency is the enemy of Server-Driven UI. If a user swipes a card, you can't wait 100ms for the server to say "move pixel by pixel."



Vango-Native solves this with **Declarative Gestures** (Client Hooks adaptation):



**Scenario: A Tinder-like Swipe Card**

You define the *rules* of the interaction in Go, but the *physics* run on the device.



```go

View(

// This hook runs on the native thread (120fps on iPad Pro)

Hook("SwipeGesture", map[string]any{

"directions": []string{"left", "right"},

"threshold": 100, // pixels

"rotation": 15, // degrees rotation while dragging

}),

// Server only cares about the FINAL outcome

OnEvent("swipeRight", func() {

db.Matches.Create(currentUser, targetUser)

}),

OnEvent("swipeLeft", func() {

// Load next profile

}),

CardContent(user),

)

```



The Native Player sees `Hook("SwipeGesture")` and attaches a native `UIPanGestureRecognizer`. It handles the smooth movement, rotation, and spring animation locally. It only pings the server when the action is **committed**.



-----



### 5\. Native Islands (Escape Hatch)



What if you need a specific native SDK, like a specialized AR library or a biometric scanner that isn't in the Standard Player?



You build a **"Custom Player."**



1. **Fork** the Vango Player repo (iOS/Android).

2. **Add** your native Swift/Kotlin code (e.g., `MyARView`).

3. **Register** it in the Player's registry map.

4. **Use** it in Go:



<!-- end list -->



```go

// In Go

NativeIsland("MyARView", map[string]any{

"model": "chair.usdz",

})

```



This is similar to "ejecting" in Expo. You now have your own binary, but you still keep the Server-Driven architecture for the rest of the app.



### Summary Comparison



| Metric | React Native | Vango-Native |

| :--- | :--- | :--- |

| **Language** | TypeScript | Go |

| **Logic Runtime** | On Device (JS Bundle) | On Server (Go Binary) |

| **Update Speed** | CodePush / App Store | Instant (Server Deploy) |

| **App Size** | \~20MB+ (JS Engine + Bundle) | \~5MB (Thin Player) |

| **Offline** | Yes | Needs WASM mode (cached) |

| **DX** | Hot Reload (Fast) | Hot Reload (Instant - no bundler) |

Apple's App Store Review Guidelines (specifically Section 2.5.2 and 3.3.2) are the "Law of the Land" for mobile frameworks.

To mitigate this risk, you must explicitly distinguish between Data (Interpreted) and Code (Executable). Vango Native is compliant because it technically never downloads "Code."

Here is the risk mitigation strategy to include in your architecture guide.

26. App Store Compliance Strategy
Risk: Apple Guideline 2.5.2 prohibits apps that "download, install, or execute code which introduces or changes features or functionality of the app."

Solution: Vango Native strictly separates the Execution Engine (The Player) from the Instruction Stream (The Logic).

26.1 The "Browser" Argument (Compliance via Precedent)
Apple allows apps like Chrome or Figma to download and render new interfaces because they classify HTML/JS as "Interpreted Code" (Guideline 3.3.2), not "Executable Binary Code."

Vango operates on the same principle:

The Vango Player is a specialized browser. It is a static binary compiled once and reviewed by Apple.

The Protocol is the HTML. It is a passive binary stream of instructions (INSERT_NODE, UPDATE_ATTR), not machine code (x86/ARM).

Precedent: This is exactly how React Native (CodePush) and Expo operate. They download new JavaScript bundles OTA. Apple allows this provided the update does not "significantly change the primary purpose of the application".

26.2 The "No OTA Logic" Constraint (For Embedded Mode)
There is a critical constraint for the Vango Embedded (Offline) mode:

Allowed: Syncing Data (SQLite rows, JSON, VNode trees) to the embedded Go engine.

Prohibited: Updating the Go Engine itself (.dylib, .so, or compiled Go binary) OTA.

The Rule: If you need to change the Go Logic (func main()) in Embedded Mode, you must issue a full App Store update. If you need to change the UI or State in Cloud Mode, you can do it OTA instantly.

26.3 The "Interpreter" Safeguard
To strictly comply with Guideline 3.3.2 ("Interpreted code may be downloaded... if run by an interpreter packaged in the Application"), we define the Vango Player as a Virtual Machine:

Fixed Instruction Set: The Player only understands a finite set of opcodes (0x01: CREATE_VIEW, 0x02: ADD_CHILD).

No Arbitrary Execution: The protocol cannot allocate memory or access OS APIs (Camera, Contacts) directly. It must request them via the Player's registered "Native Islands".

Sandboxing: The logic lives on the Server (or sandboxed Thread). The Player is just a dumb renderer.

Verdict: As long as Vango does not download compiled ARM64 binaries, it complies with current App Store policies.

22. Vango NativeVango Native is the logical endpoint of the Vango architecture. By moving logic to the server (or an embedded Go engine) and leaving only a rendering shell on the device, we solve the mobile "updates problem" and eliminate the JavaScript bridge entirely.22.1 Architecture: The "Player" ModelUnlike React Native, which ships a JS VM and business logic on the device, Vango Native ships a generic native shell called the Vango Player.The Stack:Server (Go): Runs logic, state, and API calls.Protocol: The exact same binary stream used in V2 (INSERT_NODE, SET_ATTR).Client (The Player): A thin native app (Swift/Kotlin) that:Decodes the binary stream.Layouts using Yoga (C++ Flexbox engine).Renders to native views (UIView, android.view.View).Performance:Because the binary stream talks directly to the Native/C++ layer, we bypass the JavaScript bridge bottleneck completely.22.2 The Component Model (vango/native)Mobile development requires different primitives than the web. We introduce the vango/native package.Gopackage main

import (
    "vango"
    . "vango/native" // Native Primitives
)

func UserProfile(u User) *vango.VNode {
    return View(
        Style(
            FlexDirection("row"),
            Padding(16),
            BackgroundColor("#FFFFFF"),
            CornerRadius(8),
        ),
        // Renders as UIImageView on iOS
        Image(
            Source(u.AvatarURL),
            Style(Width(50), Height(50), BorderRadius(25)),
        ),
        View(
            Style(MarginLeft(12), JustifyContent("center")),
            // Renders as UILabel on iOS
            Text(
                Content(u.Name),
                Style(FontSize(18), FontWeight("bold")),
            ),
        ),
    )
}
22.3 Native Hooks (Declarative Gestures)Latency is the enemy of mobile interaction. We cannot wait 100ms for the server to confirm a swipe. We solve this with Native Hooks—defining the rules in Go, but running the physics on the device.GoView(
    // The "SwipeGesture" hook attaches a UIPanGestureRecognizer
    // It runs at 120fps on the device (Main Thread)
    Hook("SwipeGesture", map[string]any{
        "directions": []string{"left", "right"},
        "threshold":  100, // pixels
        "rotation":   15,  // degrees
    }),
    
    // Server only receives the FINAL committed event
    OnEvent("swipeRight", func() {
        db.Matches.Create(currentUser, targetUser)
    }),
)
22.4 Offline Support: Vango EmbeddedFor offline capabilities, we do not fall back to caching HTML. We run the Go Engine on the device using gomobile.The "In-App Server" Architecture:Compile: The Vango app is compiled to a native library (.framework/.aar) using gomobile bind.Runtime: The Go runtime lives in a background thread on the phone.Bridge: Instead of WebSockets, the Player communicates with the Go engine via Direct Memory Calls (FFI).ModeLogic LocationTransportLatencyCloud ModeData CenterWebSocket50-100msEmbedded ModeDevice (Background Thread)Memory Pointer~0msThis creates a "Localhost" pattern where the app works 100% offline because the server is in the user's pocket.23. Vango UniversalWe can unify Web and Mobile into a single codebase using Abstract Primitives.23.1 The Universal Package (vango/uni)Instead of writing Div (Web) or View (Mobile), you use Stack.Goimport . "vango/uni"

func ProductCard(p Product) *vango.VNode {
    // This component renders natively on ALL platforms
    return Stack(
        Direction("vertical"),
        Padding(16),
        
        Text(Content(p.Name), Style(Bold())),
        
        Button(
            Label("Buy Now"),
            OnTap(func() { cart.Add(p) }),
        ),
    )
}
Context-Aware Encoding:The server checks the connection context (ctx.Client):If Web: Serializes Stack → <div style="display:flex">If Mobile: Serializes Stack → INSERT_NODE (Type: STACK)23.2 Capability NegotiationFor platform-specific features, use server-side branching based on the client's handshake capabilities:Gofunc UploadControl(ctx vango.Ctx) *vango.VNode {
    // Branching based on capabilities, not just user agent
    if ctx.HasCapability("CAMERA") {
        return Button(
            Label("Take Selfie"),
            OnTap(func() { ctx.Send(NativeCommand("OPEN_CAMERA")) }),
        )
    }

    return Input(Type("file"), Accept("image/*"))
}

25. Developer Experience (DX)
25.1 Vango Go
Development does not require Xcode or Android Studio.

Download "Vango Go" from the App Store.

Run vango dev on your laptop.

Scan the QR code.

Instant Updates: Changing Go code sends a patch to the phone instantly. No compilation, no app restart.

25.2 The "Polyglot Edge" Deployment
Vango Cloud allows you to treat the App Store like a CDN:

You push to Git.

Vango Cloud updates the Web PWA.

Vango Cloud instantly updates the Logic for all Native App users (OTA).

Zero App Store Review time for logic or UI changes.
"""


### RHONE ### (The Go-To deployment/hosting/obs platform for Vango apps.)

Lots of ideas here. Seems like using fly.io may be a good way to get an mvp out there, they run on firecracker and have some cool features. Worth research more deeply. As well as considering other options.

Vango will be an open-source framework devs can deploy however they want, but making Rhone a go-to platform for Vango apps is our goal either for increased DX, functionality, performance, cost efficiency, or any other reason.

