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


