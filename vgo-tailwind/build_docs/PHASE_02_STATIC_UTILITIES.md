# Phase 2: Static Utilities

## Overview

Phase 2 implements all static utilities—classes that map directly to fixed CSS declarations without any value parameters. These include display, position, flexbox, and grid utilities.

**Prerequisites:** Phase 1 (Core Infrastructure)

**Files to create/modify:**
- `utilities/static.go` - Static utility registrations
- `utilities/layout.go` - Display, position, overflow utilities
- `utilities/flexbox.go` - Flexbox utilities
- `utilities/grid.go` - Grid utilities

---

## 1. Static Utility Pattern

Static utilities have no value—they map directly to CSS:

```go
// utilities/static.go

package utilities

import "github.com/vango-dev/vgo-tailwind"

// registerStatic is a helper for simple static utilities.
func registerStatic(r *Registry, name string, decls ...tailwind.Declaration) {
    r.RegisterStatic(name, func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        return decls
    })
}

// decl is a helper to create a declaration.
func decl(property, value string) tailwind.Declaration {
    return tailwind.Declaration{Property: property, Value: value}
}
```

---

## 2. Display Utilities

**Reference:** `/tailwind/packages/tailwindcss/src/utilities.ts` (search for "display")

### 2.1 Basic Display

```go
// utilities/layout.go

package utilities

func (r *Registry) registerDisplayUtilities() {
    // Block
    registerStatic(r, "block", decl("display", "block"))
    registerStatic(r, "inline-block", decl("display", "inline-block"))
    registerStatic(r, "inline", decl("display", "inline"))

    // Flex
    registerStatic(r, "flex", decl("display", "flex"))
    registerStatic(r, "inline-flex", decl("display", "inline-flex"))

    // Grid
    registerStatic(r, "grid", decl("display", "grid"))
    registerStatic(r, "inline-grid", decl("display", "inline-grid"))

    // Table
    registerStatic(r, "table", decl("display", "table"))
    registerStatic(r, "inline-table", decl("display", "inline-table"))
    registerStatic(r, "table-caption", decl("display", "table-caption"))
    registerStatic(r, "table-cell", decl("display", "table-cell"))
    registerStatic(r, "table-column", decl("display", "table-column"))
    registerStatic(r, "table-column-group", decl("display", "table-column-group"))
    registerStatic(r, "table-footer-group", decl("display", "table-footer-group"))
    registerStatic(r, "table-header-group", decl("display", "table-header-group"))
    registerStatic(r, "table-row-group", decl("display", "table-row-group"))
    registerStatic(r, "table-row", decl("display", "table-row"))

    // Flow-root
    registerStatic(r, "flow-root", decl("display", "flow-root"))

    // Contents
    registerStatic(r, "contents", decl("display", "contents"))

    // List-item
    registerStatic(r, "list-item", decl("display", "list-item"))

    // Hidden
    registerStatic(r, "hidden", decl("display", "none"))
}
```

### 2.2 Complete Display List

| Class | CSS |
|-------|-----|
| `block` | `display: block;` |
| `inline-block` | `display: inline-block;` |
| `inline` | `display: inline;` |
| `flex` | `display: flex;` |
| `inline-flex` | `display: inline-flex;` |
| `grid` | `display: grid;` |
| `inline-grid` | `display: inline-grid;` |
| `table` | `display: table;` |
| `inline-table` | `display: inline-table;` |
| `table-caption` | `display: table-caption;` |
| `table-cell` | `display: table-cell;` |
| `table-column` | `display: table-column;` |
| `table-column-group` | `display: table-column-group;` |
| `table-footer-group` | `display: table-footer-group;` |
| `table-header-group` | `display: table-header-group;` |
| `table-row-group` | `display: table-row-group;` |
| `table-row` | `display: table-row;` |
| `flow-root` | `display: flow-root;` |
| `contents` | `display: contents;` |
| `list-item` | `display: list-item;` |
| `hidden` | `display: none;` |

---

## 3. Position Utilities

```go
func (r *Registry) registerPositionUtilities() {
    registerStatic(r, "static", decl("position", "static"))
    registerStatic(r, "fixed", decl("position", "fixed"))
    registerStatic(r, "absolute", decl("position", "absolute"))
    registerStatic(r, "relative", decl("position", "relative"))
    registerStatic(r, "sticky", decl("position", "sticky"))
}
```

| Class | CSS |
|-------|-----|
| `static` | `position: static;` |
| `fixed` | `position: fixed;` |
| `absolute` | `position: absolute;` |
| `relative` | `position: relative;` |
| `sticky` | `position: sticky;` |

---

## 4. Visibility Utilities

```go
func (r *Registry) registerVisibilityUtilities() {
    registerStatic(r, "visible", decl("visibility", "visible"))
    registerStatic(r, "invisible", decl("visibility", "hidden"))
    registerStatic(r, "collapse", decl("visibility", "collapse"))
}
```

| Class | CSS |
|-------|-----|
| `visible` | `visibility: visible;` |
| `invisible` | `visibility: hidden;` |
| `collapse` | `visibility: collapse;` |

---

## 5. Flexbox Utilities

### 5.1 Flex Direction

```go
// utilities/flexbox.go

func (r *Registry) registerFlexboxUtilities() {
    // Flex Direction
    registerStatic(r, "flex-row", decl("flex-direction", "row"))
    registerStatic(r, "flex-row-reverse", decl("flex-direction", "row-reverse"))
    registerStatic(r, "flex-col", decl("flex-direction", "column"))
    registerStatic(r, "flex-col-reverse", decl("flex-direction", "column-reverse"))
}
```

### 5.2 Flex Wrap

```go
    // Flex Wrap
    registerStatic(r, "flex-wrap", decl("flex-wrap", "wrap"))
    registerStatic(r, "flex-wrap-reverse", decl("flex-wrap", "wrap-reverse"))
    registerStatic(r, "flex-nowrap", decl("flex-wrap", "nowrap"))
```

### 5.3 Flex Grow/Shrink

```go
    // Flex (shorthand)
    registerStatic(r, "flex-1", decl("flex", "1 1 0%"))
    registerStatic(r, "flex-auto", decl("flex", "1 1 auto"))
    registerStatic(r, "flex-initial", decl("flex", "0 1 auto"))
    registerStatic(r, "flex-none", decl("flex", "none"))

    // Flex Grow
    registerStatic(r, "grow", decl("flex-grow", "1"))
    registerStatic(r, "grow-0", decl("flex-grow", "0"))

    // Flex Shrink
    registerStatic(r, "shrink", decl("flex-shrink", "1"))
    registerStatic(r, "shrink-0", decl("flex-shrink", "0"))
```

### 5.4 Justify Content

```go
    // Justify Content
    registerStatic(r, "justify-normal", decl("justify-content", "normal"))
    registerStatic(r, "justify-start", decl("justify-content", "flex-start"))
    registerStatic(r, "justify-end", decl("justify-content", "flex-end"))
    registerStatic(r, "justify-center", decl("justify-content", "center"))
    registerStatic(r, "justify-between", decl("justify-content", "space-between"))
    registerStatic(r, "justify-around", decl("justify-content", "space-around"))
    registerStatic(r, "justify-evenly", decl("justify-content", "space-evenly"))
    registerStatic(r, "justify-stretch", decl("justify-content", "stretch"))
```

### 5.5 Justify Items

```go
    // Justify Items
    registerStatic(r, "justify-items-start", decl("justify-items", "start"))
    registerStatic(r, "justify-items-end", decl("justify-items", "end"))
    registerStatic(r, "justify-items-center", decl("justify-items", "center"))
    registerStatic(r, "justify-items-stretch", decl("justify-items", "stretch"))
```

### 5.6 Justify Self

```go
    // Justify Self
    registerStatic(r, "justify-self-auto", decl("justify-self", "auto"))
    registerStatic(r, "justify-self-start", decl("justify-self", "start"))
    registerStatic(r, "justify-self-end", decl("justify-self", "end"))
    registerStatic(r, "justify-self-center", decl("justify-self", "center"))
    registerStatic(r, "justify-self-stretch", decl("justify-self", "stretch"))
```

### 5.7 Align Content

```go
    // Align Content
    registerStatic(r, "content-normal", decl("align-content", "normal"))
    registerStatic(r, "content-center", decl("align-content", "center"))
    registerStatic(r, "content-start", decl("align-content", "flex-start"))
    registerStatic(r, "content-end", decl("align-content", "flex-end"))
    registerStatic(r, "content-between", decl("align-content", "space-between"))
    registerStatic(r, "content-around", decl("align-content", "space-around"))
    registerStatic(r, "content-evenly", decl("align-content", "space-evenly"))
    registerStatic(r, "content-baseline", decl("align-content", "baseline"))
    registerStatic(r, "content-stretch", decl("align-content", "stretch"))
```

### 5.8 Align Items

```go
    // Align Items
    registerStatic(r, "items-start", decl("align-items", "flex-start"))
    registerStatic(r, "items-end", decl("align-items", "flex-end"))
    registerStatic(r, "items-center", decl("align-items", "center"))
    registerStatic(r, "items-baseline", decl("align-items", "baseline"))
    registerStatic(r, "items-stretch", decl("align-items", "stretch"))
```

### 5.9 Align Self

```go
    // Align Self
    registerStatic(r, "self-auto", decl("align-self", "auto"))
    registerStatic(r, "self-start", decl("align-self", "flex-start"))
    registerStatic(r, "self-end", decl("align-self", "flex-end"))
    registerStatic(r, "self-center", decl("align-self", "center"))
    registerStatic(r, "self-stretch", decl("align-self", "stretch"))
    registerStatic(r, "self-baseline", decl("align-self", "baseline"))
```

### 5.10 Place Content

```go
    // Place Content (shorthand for align-content + justify-content)
    registerStatic(r, "place-content-center", decl("place-content", "center"))
    registerStatic(r, "place-content-start", decl("place-content", "start"))
    registerStatic(r, "place-content-end", decl("place-content", "end"))
    registerStatic(r, "place-content-between", decl("place-content", "space-between"))
    registerStatic(r, "place-content-around", decl("place-content", "space-around"))
    registerStatic(r, "place-content-evenly", decl("place-content", "space-evenly"))
    registerStatic(r, "place-content-baseline", decl("place-content", "baseline"))
    registerStatic(r, "place-content-stretch", decl("place-content", "stretch"))
```

### 5.11 Place Items

```go
    // Place Items (shorthand for align-items + justify-items)
    registerStatic(r, "place-items-start", decl("place-items", "start"))
    registerStatic(r, "place-items-end", decl("place-items", "end"))
    registerStatic(r, "place-items-center", decl("place-items", "center"))
    registerStatic(r, "place-items-baseline", decl("place-items", "baseline"))
    registerStatic(r, "place-items-stretch", decl("place-items", "stretch"))
```

### 5.12 Place Self

```go
    // Place Self (shorthand for align-self + justify-self)
    registerStatic(r, "place-self-auto", decl("place-self", "auto"))
    registerStatic(r, "place-self-start", decl("place-self", "start"))
    registerStatic(r, "place-self-end", decl("place-self", "end"))
    registerStatic(r, "place-self-center", decl("place-self", "center"))
    registerStatic(r, "place-self-stretch", decl("place-self", "stretch"))
```

---

## 6. Grid Utilities (Static)

```go
// utilities/grid.go

func (r *Registry) registerGridUtilities() {
    // Grid Auto Flow
    registerStatic(r, "grid-flow-row", decl("grid-auto-flow", "row"))
    registerStatic(r, "grid-flow-col", decl("grid-auto-flow", "column"))
    registerStatic(r, "grid-flow-dense", decl("grid-auto-flow", "dense"))
    registerStatic(r, "grid-flow-row-dense", decl("grid-auto-flow", "row dense"))
    registerStatic(r, "grid-flow-col-dense", decl("grid-auto-flow", "column dense"))
}
```

---

## 7. Overflow Utilities

```go
func (r *Registry) registerOverflowUtilities() {
    // Overflow
    registerStatic(r, "overflow-auto", decl("overflow", "auto"))
    registerStatic(r, "overflow-hidden", decl("overflow", "hidden"))
    registerStatic(r, "overflow-clip", decl("overflow", "clip"))
    registerStatic(r, "overflow-visible", decl("overflow", "visible"))
    registerStatic(r, "overflow-scroll", decl("overflow", "scroll"))

    // Overflow X
    registerStatic(r, "overflow-x-auto", decl("overflow-x", "auto"))
    registerStatic(r, "overflow-x-hidden", decl("overflow-x", "hidden"))
    registerStatic(r, "overflow-x-clip", decl("overflow-x", "clip"))
    registerStatic(r, "overflow-x-visible", decl("overflow-x", "visible"))
    registerStatic(r, "overflow-x-scroll", decl("overflow-x", "scroll"))

    // Overflow Y
    registerStatic(r, "overflow-y-auto", decl("overflow-y", "auto"))
    registerStatic(r, "overflow-y-hidden", decl("overflow-y", "hidden"))
    registerStatic(r, "overflow-y-clip", decl("overflow-y", "clip"))
    registerStatic(r, "overflow-y-visible", decl("overflow-y", "visible"))
    registerStatic(r, "overflow-y-scroll", decl("overflow-y", "scroll"))
}
```

---

## 8. Overscroll Behavior

```go
func (r *Registry) registerOverscrollUtilities() {
    registerStatic(r, "overscroll-auto", decl("overscroll-behavior", "auto"))
    registerStatic(r, "overscroll-contain", decl("overscroll-behavior", "contain"))
    registerStatic(r, "overscroll-none", decl("overscroll-behavior", "none"))

    registerStatic(r, "overscroll-x-auto", decl("overscroll-behavior-x", "auto"))
    registerStatic(r, "overscroll-x-contain", decl("overscroll-behavior-x", "contain"))
    registerStatic(r, "overscroll-x-none", decl("overscroll-behavior-x", "none"))

    registerStatic(r, "overscroll-y-auto", decl("overscroll-behavior-y", "auto"))
    registerStatic(r, "overscroll-y-contain", decl("overscroll-behavior-y", "contain"))
    registerStatic(r, "overscroll-y-none", decl("overscroll-behavior-y", "none"))
}
```

---

## 9. Box Sizing

```go
func (r *Registry) registerBoxSizingUtilities() {
    registerStatic(r, "box-border", decl("box-sizing", "border-box"))
    registerStatic(r, "box-content", decl("box-sizing", "content-box"))
}
```

---

## 10. Float and Clear

```go
func (r *Registry) registerFloatUtilities() {
    registerStatic(r, "float-start", decl("float", "inline-start"))
    registerStatic(r, "float-end", decl("float", "inline-end"))
    registerStatic(r, "float-right", decl("float", "right"))
    registerStatic(r, "float-left", decl("float", "left"))
    registerStatic(r, "float-none", decl("float", "none"))

    registerStatic(r, "clear-start", decl("clear", "inline-start"))
    registerStatic(r, "clear-end", decl("clear", "inline-end"))
    registerStatic(r, "clear-left", decl("clear", "left"))
    registerStatic(r, "clear-right", decl("clear", "right"))
    registerStatic(r, "clear-both", decl("clear", "both"))
    registerStatic(r, "clear-none", decl("clear", "none"))
}
```

---

## 11. Isolation

```go
func (r *Registry) registerIsolationUtilities() {
    registerStatic(r, "isolate", decl("isolation", "isolate"))
    registerStatic(r, "isolation-auto", decl("isolation", "auto"))
}
```

---

## 12. Object Fit and Position

```go
func (r *Registry) registerObjectUtilities() {
    // Object Fit
    registerStatic(r, "object-contain", decl("object-fit", "contain"))
    registerStatic(r, "object-cover", decl("object-fit", "cover"))
    registerStatic(r, "object-fill", decl("object-fit", "fill"))
    registerStatic(r, "object-none", decl("object-fit", "none"))
    registerStatic(r, "object-scale-down", decl("object-fit", "scale-down"))

    // Object Position
    registerStatic(r, "object-bottom", decl("object-position", "bottom"))
    registerStatic(r, "object-center", decl("object-position", "center"))
    registerStatic(r, "object-left", decl("object-position", "left"))
    registerStatic(r, "object-left-bottom", decl("object-position", "left bottom"))
    registerStatic(r, "object-left-top", decl("object-position", "left top"))
    registerStatic(r, "object-right", decl("object-position", "right"))
    registerStatic(r, "object-right-bottom", decl("object-position", "right bottom"))
    registerStatic(r, "object-right-top", decl("object-position", "right top"))
    registerStatic(r, "object-top", decl("object-position", "top"))
}
```

---

## 13. Pointer Events and User Select

```go
func (r *Registry) registerInteractivityUtilities() {
    // Pointer Events
    registerStatic(r, "pointer-events-none", decl("pointer-events", "none"))
    registerStatic(r, "pointer-events-auto", decl("pointer-events", "auto"))

    // User Select
    registerStatic(r, "select-none", decl("user-select", "none"))
    registerStatic(r, "select-text", decl("user-select", "text"))
    registerStatic(r, "select-all", decl("user-select", "all"))
    registerStatic(r, "select-auto", decl("user-select", "auto"))

    // Resize
    registerStatic(r, "resize-none", decl("resize", "none"))
    registerStatic(r, "resize-y", decl("resize", "vertical"))
    registerStatic(r, "resize-x", decl("resize", "horizontal"))
    registerStatic(r, "resize", decl("resize", "both"))

    // Scroll Behavior
    registerStatic(r, "scroll-auto", decl("scroll-behavior", "auto"))
    registerStatic(r, "scroll-smooth", decl("scroll-behavior", "smooth"))

    // Touch Action
    registerStatic(r, "touch-auto", decl("touch-action", "auto"))
    registerStatic(r, "touch-none", decl("touch-action", "none"))
    registerStatic(r, "touch-pan-x", decl("touch-action", "pan-x"))
    registerStatic(r, "touch-pan-left", decl("touch-action", "pan-left"))
    registerStatic(r, "touch-pan-right", decl("touch-action", "pan-right"))
    registerStatic(r, "touch-pan-y", decl("touch-action", "pan-y"))
    registerStatic(r, "touch-pan-up", decl("touch-action", "pan-up"))
    registerStatic(r, "touch-pan-down", decl("touch-action", "pan-down"))
    registerStatic(r, "touch-pinch-zoom", decl("touch-action", "pinch-zoom"))
    registerStatic(r, "touch-manipulation", decl("touch-action", "manipulation"))
}
```

---

## 14. Cursor

```go
func (r *Registry) registerCursorUtilities() {
    cursors := []string{
        "auto", "default", "pointer", "wait", "text", "move", "help",
        "not-allowed", "none", "context-menu", "progress", "cell",
        "crosshair", "vertical-text", "alias", "copy", "no-drop",
        "grab", "grabbing", "all-scroll", "col-resize", "row-resize",
        "n-resize", "e-resize", "s-resize", "w-resize",
        "ne-resize", "nw-resize", "se-resize", "sw-resize",
        "ew-resize", "ns-resize", "nesw-resize", "nwse-resize", "zoom-in", "zoom-out",
    }

    for _, cursor := range cursors {
        registerStatic(r, "cursor-"+cursor, decl("cursor", cursor))
    }
}
```

---

## 15. Screen Readers

```go
func (r *Registry) registerScreenReaderUtilities() {
    // sr-only: Visually hide but keep accessible
    r.RegisterStatic("sr-only", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        return []tailwind.Declaration{
            decl("position", "absolute"),
            decl("width", "1px"),
            decl("height", "1px"),
            decl("padding", "0"),
            decl("margin", "-1px"),
            decl("overflow", "hidden"),
            decl("clip", "rect(0, 0, 0, 0)"),
            decl("white-space", "nowrap"),
            decl("border-width", "0"),
        }
    })

    // not-sr-only: Undo sr-only
    r.RegisterStatic("not-sr-only", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        return []tailwind.Declaration{
            decl("position", "static"),
            decl("width", "auto"),
            decl("height", "auto"),
            decl("padding", "0"),
            decl("margin", "0"),
            decl("overflow", "visible"),
            decl("clip", "auto"),
            decl("white-space", "normal"),
        }
    })
}
```

---

## 16. Text Utilities (Static)

```go
func (r *Registry) registerTextStaticUtilities() {
    // Text Alignment
    registerStatic(r, "text-left", decl("text-align", "left"))
    registerStatic(r, "text-center", decl("text-align", "center"))
    registerStatic(r, "text-right", decl("text-align", "right"))
    registerStatic(r, "text-justify", decl("text-align", "justify"))
    registerStatic(r, "text-start", decl("text-align", "start"))
    registerStatic(r, "text-end", decl("text-align", "end"))

    // Text Decoration
    registerStatic(r, "underline", decl("text-decoration-line", "underline"))
    registerStatic(r, "overline", decl("text-decoration-line", "overline"))
    registerStatic(r, "line-through", decl("text-decoration-line", "line-through"))
    registerStatic(r, "no-underline", decl("text-decoration-line", "none"))

    // Text Decoration Style
    registerStatic(r, "decoration-solid", decl("text-decoration-style", "solid"))
    registerStatic(r, "decoration-double", decl("text-decoration-style", "double"))
    registerStatic(r, "decoration-dotted", decl("text-decoration-style", "dotted"))
    registerStatic(r, "decoration-dashed", decl("text-decoration-style", "dashed"))
    registerStatic(r, "decoration-wavy", decl("text-decoration-style", "wavy"))

    // Text Transform
    registerStatic(r, "uppercase", decl("text-transform", "uppercase"))
    registerStatic(r, "lowercase", decl("text-transform", "lowercase"))
    registerStatic(r, "capitalize", decl("text-transform", "capitalize"))
    registerStatic(r, "normal-case", decl("text-transform", "none"))

    // Text Overflow
    registerStatic(r, "truncate",
        decl("overflow", "hidden"),
        decl("text-overflow", "ellipsis"),
        decl("white-space", "nowrap"),
    )
    registerStatic(r, "text-ellipsis", decl("text-overflow", "ellipsis"))
    registerStatic(r, "text-clip", decl("text-overflow", "clip"))

    // Text Wrap
    registerStatic(r, "text-wrap", decl("text-wrap", "wrap"))
    registerStatic(r, "text-nowrap", decl("text-wrap", "nowrap"))
    registerStatic(r, "text-balance", decl("text-wrap", "balance"))
    registerStatic(r, "text-pretty", decl("text-wrap", "pretty"))

    // Word Break
    registerStatic(r, "break-normal",
        decl("overflow-wrap", "normal"),
        decl("word-break", "normal"),
    )
    registerStatic(r, "break-words", decl("overflow-wrap", "break-word"))
    registerStatic(r, "break-all", decl("word-break", "break-all"))
    registerStatic(r, "break-keep", decl("word-break", "keep-all"))

    // Hyphens
    registerStatic(r, "hyphens-none", decl("hyphens", "none"))
    registerStatic(r, "hyphens-manual", decl("hyphens", "manual"))
    registerStatic(r, "hyphens-auto", decl("hyphens", "auto"))

    // Whitespace
    registerStatic(r, "whitespace-normal", decl("white-space", "normal"))
    registerStatic(r, "whitespace-nowrap", decl("white-space", "nowrap"))
    registerStatic(r, "whitespace-pre", decl("white-space", "pre"))
    registerStatic(r, "whitespace-pre-line", decl("white-space", "pre-line"))
    registerStatic(r, "whitespace-pre-wrap", decl("white-space", "pre-wrap"))
    registerStatic(r, "whitespace-break-spaces", decl("white-space", "break-spaces"))

    // Vertical Align
    registerStatic(r, "align-baseline", decl("vertical-align", "baseline"))
    registerStatic(r, "align-top", decl("vertical-align", "top"))
    registerStatic(r, "align-middle", decl("vertical-align", "middle"))
    registerStatic(r, "align-bottom", decl("vertical-align", "bottom"))
    registerStatic(r, "align-text-top", decl("vertical-align", "text-top"))
    registerStatic(r, "align-text-bottom", decl("vertical-align", "text-bottom"))
    registerStatic(r, "align-sub", decl("vertical-align", "sub"))
    registerStatic(r, "align-super", decl("vertical-align", "super"))
}
```

---

## 17. Font Utilities (Static)

```go
func (r *Registry) registerFontStaticUtilities() {
    // Font Style
    registerStatic(r, "italic", decl("font-style", "italic"))
    registerStatic(r, "not-italic", decl("font-style", "normal"))

    // Font Variant Numeric
    registerStatic(r, "normal-nums", decl("font-variant-numeric", "normal"))
    registerStatic(r, "ordinal", decl("font-variant-numeric", "ordinal"))
    registerStatic(r, "slashed-zero", decl("font-variant-numeric", "slashed-zero"))
    registerStatic(r, "lining-nums", decl("font-variant-numeric", "lining-nums"))
    registerStatic(r, "oldstyle-nums", decl("font-variant-numeric", "oldstyle-nums"))
    registerStatic(r, "proportional-nums", decl("font-variant-numeric", "proportional-nums"))
    registerStatic(r, "tabular-nums", decl("font-variant-numeric", "tabular-nums"))
    registerStatic(r, "diagonal-fractions", decl("font-variant-numeric", "diagonal-fractions"))
    registerStatic(r, "stacked-fractions", decl("font-variant-numeric", "stacked-fractions"))

    // Font Smoothing
    registerStatic(r, "antialiased",
        decl("-webkit-font-smoothing", "antialiased"),
        decl("-moz-osx-font-smoothing", "grayscale"),
    )
    registerStatic(r, "subpixel-antialiased",
        decl("-webkit-font-smoothing", "auto"),
        decl("-moz-osx-font-smoothing", "auto"),
    )
}
```

---

## 18. List Style Utilities (Static)

```go
func (r *Registry) registerListUtilities() {
    // List Style Type
    registerStatic(r, "list-none", decl("list-style-type", "none"))
    registerStatic(r, "list-disc", decl("list-style-type", "disc"))
    registerStatic(r, "list-decimal", decl("list-style-type", "decimal"))

    // List Style Position
    registerStatic(r, "list-inside", decl("list-style-position", "inside"))
    registerStatic(r, "list-outside", decl("list-style-position", "outside"))
}
```

---

## 19. Background Utilities (Static)

```go
func (r *Registry) registerBackgroundStaticUtilities() {
    // Background Attachment
    registerStatic(r, "bg-fixed", decl("background-attachment", "fixed"))
    registerStatic(r, "bg-local", decl("background-attachment", "local"))
    registerStatic(r, "bg-scroll", decl("background-attachment", "scroll"))

    // Background Clip
    registerStatic(r, "bg-clip-border", decl("background-clip", "border-box"))
    registerStatic(r, "bg-clip-padding", decl("background-clip", "padding-box"))
    registerStatic(r, "bg-clip-content", decl("background-clip", "content-box"))
    registerStatic(r, "bg-clip-text", decl("background-clip", "text"))

    // Background Origin
    registerStatic(r, "bg-origin-border", decl("background-origin", "border-box"))
    registerStatic(r, "bg-origin-padding", decl("background-origin", "padding-box"))
    registerStatic(r, "bg-origin-content", decl("background-origin", "content-box"))

    // Background Repeat
    registerStatic(r, "bg-repeat", decl("background-repeat", "repeat"))
    registerStatic(r, "bg-no-repeat", decl("background-repeat", "no-repeat"))
    registerStatic(r, "bg-repeat-x", decl("background-repeat", "repeat-x"))
    registerStatic(r, "bg-repeat-y", decl("background-repeat", "repeat-y"))
    registerStatic(r, "bg-repeat-round", decl("background-repeat", "round"))
    registerStatic(r, "bg-repeat-space", decl("background-repeat", "space"))

    // Background Size
    registerStatic(r, "bg-auto", decl("background-size", "auto"))
    registerStatic(r, "bg-cover", decl("background-size", "cover"))
    registerStatic(r, "bg-contain", decl("background-size", "contain"))

    // Background Position
    registerStatic(r, "bg-bottom", decl("background-position", "bottom"))
    registerStatic(r, "bg-center", decl("background-position", "center"))
    registerStatic(r, "bg-left", decl("background-position", "left"))
    registerStatic(r, "bg-left-bottom", decl("background-position", "left bottom"))
    registerStatic(r, "bg-left-top", decl("background-position", "left top"))
    registerStatic(r, "bg-right", decl("background-position", "right"))
    registerStatic(r, "bg-right-bottom", decl("background-position", "right bottom"))
    registerStatic(r, "bg-right-top", decl("background-position", "right top"))
    registerStatic(r, "bg-top", decl("background-position", "top"))
}
```

---

## 20. Appearance

```go
func (r *Registry) registerAppearanceUtilities() {
    registerStatic(r, "appearance-none", decl("appearance", "none"))
    registerStatic(r, "appearance-auto", decl("appearance", "auto"))
}
```

---

## 21. Complete Registration

```go
// utilities/registry.go

func (r *Registry) registerDefaults() {
    // Phase 2: Static utilities
    r.registerDisplayUtilities()
    r.registerPositionUtilities()
    r.registerVisibilityUtilities()
    r.registerFlexboxUtilities()
    r.registerGridUtilities()
    r.registerOverflowUtilities()
    r.registerOverscrollUtilities()
    r.registerBoxSizingUtilities()
    r.registerFloatUtilities()
    r.registerIsolationUtilities()
    r.registerObjectUtilities()
    r.registerInteractivityUtilities()
    r.registerCursorUtilities()
    r.registerScreenReaderUtilities()
    r.registerTextStaticUtilities()
    r.registerFontStaticUtilities()
    r.registerListUtilities()
    r.registerBackgroundStaticUtilities()
    r.registerAppearanceUtilities()

    // Phases 3-7: Functional utilities (registered in their respective phases)
}
```

---

## 22. Testing

```go
// utilities/static_test.go

func TestStaticUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        property string
        value    string
    }{
        {"flex", "display", "flex"},
        {"hidden", "display", "none"},
        {"relative", "position", "relative"},
        {"items-center", "align-items", "center"},
        {"justify-between", "justify-content", "space-between"},
        {"flex-col", "flex-direction", "column"},
        {"overflow-hidden", "overflow", "hidden"},
        {"cursor-pointer", "cursor", "pointer"},
        {"uppercase", "text-transform", "uppercase"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            util, ok := registry.Get(tt.class, UtilityStatic)
            assert.True(t, ok, "utility should exist")

            c := &tailwind.Candidate{Kind: tailwind.KindStatic, Root: tt.class}
            decls := util.Compile(c, theme)

            found := false
            for _, d := range decls {
                if d.Property == tt.property && d.Value == tt.value {
                    found = true
                    break
                }
            }
            assert.True(t, found, "expected %s: %s", tt.property, tt.value)
        })
    }
}
```

---

## 23. Summary

### Total Static Utilities: ~200

| Category | Count |
|----------|-------|
| Display | 21 |
| Position | 5 |
| Visibility | 3 |
| Flexbox | 65+ |
| Grid | 5 |
| Overflow | 15 |
| Overscroll | 9 |
| Box Sizing | 2 |
| Float/Clear | 11 |
| Isolation | 2 |
| Object | 14 |
| Interactivity | 30+ |
| Cursor | 35 |
| Screen Reader | 2 |
| Text | 40+ |
| Font | 15 |
| List | 5 |
| Background | 25 |
| Appearance | 2 |

---

## 24. Completion Criteria

Phase 2 is complete when:

1. ✅ All static utilities are registered
2. ✅ Each utility generates correct CSS
3. ✅ Tests cover all utilities
4. ✅ Integration with Engine works

---

*Last Updated: 2024-12-12*
