# VangoUI Implementation Plan

## Overview

Create `vango-ui/` as a standalone registry/source for the VangoUI component library. This will be the canonical source that `vango add` copies from.

## Directory Structure

```
/Users/collinshill/Documents/vango/vango-ui/
├── go.mod                    # Module: github.com/vango-dev/vango-ui
├── registry.json             # Component manifest for CLI
├── components/
│   ├── base.go               # BaseConfig, ConfigProvider, generic options
│   ├── utils.go              # CN utility, class helpers
│   ├── button.go             # Button component
│   ├── card.go               # Card component
│   ├── input.go              # Input component
│   ├── badge.go              # Badge component
│   └── dialog.go             # Dialog (interactive, uses hooks)
└── components_test.go        # Tests for all components
```

## Implementation Steps

### Step 1: Initialize Go Module

Create `go.mod` with dependency on vango_v2 core.

### Step 2: Implement Foundation (`base.go`)

```go
// BaseConfig is embedded in every component config
type BaseConfig struct {
    Classes []string
    Attrs   []vdom.Attr
    Children []*vdom.VNode
}

// ConfigProvider interface allows generic options
type ConfigProvider interface {
    GetBase() *BaseConfig
}

// Option[T] is the generic option type
type Option[T ConfigProvider] func(T)

// Class adds utility classes
func Class[T ConfigProvider](classes ...string) Option[T]

// Attr adds raw attributes
func Attr[T ConfigProvider](attrs ...vdom.Attr) Option[T]

// Child adds children
func Child[T ConfigProvider](nodes ...*vdom.VNode) Option[T]

// On adds event handlers
func On[T ConfigProvider](event string, handler any) Option[T]
```

### Step 3: Implement Utils (`utils.go`)

```go
// CN merges class strings, deduplicating and preserving order
func CN(inputs ...string) string

// Internal helper for variant class generation
func variantClasses(base string, variants map[string]map[string]string, ...) string
```

### Step 4: Implement Button (`button.go`)

- Typed enums: `ButtonVariant`, `ButtonSize`
- Config: `ButtonConfig` with BaseConfig embedded
- Options: `Variant()`, `Size()`, `Disabled()`
- Render: Merges variant classes with user classes

### Step 5: Implement Card (`card.go`)

- Simple container component
- Parts: `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter`
- Each part is a separate function sharing style conventions

### Step 6: Implement Input (`input.go`)

- Typed enums: `InputType` (text, email, password, etc.)
- Integration with `vango.Signal[string]` for two-way binding
- Options: `Type()`, `Placeholder()`, `Value()`, `OnChange()`

### Step 7: Implement Badge (`badge.go`)

- Typed enums: `BadgeVariant` (default, secondary, destructive, outline)
- Simple inline display component

### Step 8: Implement Dialog (`dialog.go`)

- Interactive component using hooks
- Hook integration: `FocusTrap`, `Portal`
- Options: `Open(*vango.Signal[bool])`, `OnClose()`, `CloseOnEscape()`
- Event wiring pattern from spec
- Parts: `Dialog`, `DialogTrigger`, `DialogContent`, `DialogHeader`, `DialogTitle`, `DialogDescription`, `DialogFooter`, `DialogClose`

### Step 9: Create Registry Manifest (`registry.json`)

```json
{
  "version": "0.1.0",
  "components": {
    "base": {
      "files": ["base.go"],
      "dependsOn": []
    },
    "utils": {
      "files": ["utils.go"],
      "dependsOn": []
    },
    "button": {
      "files": ["button.go"],
      "dependsOn": ["base", "utils"]
    },
    ...
  }
}
```

### Step 10: Write Tests

- Test each component renders correctly
- Test option application
- Test class merging behavior
- Test dialog hook event wiring

## Key Design Decisions

1. **Generic Options with Type Inference**: Go 1.21+ allows `Class[*ButtonConfig]("foo")` to be written as just `button.Class("foo")` when used in context.

2. **Attr vs Children separation**: Unlike the current vdom `...any` pattern, VangoUI explicitly separates attributes and children for type safety.

3. **Hook Constants**: All hook names and event names are constants to prevent typos.

4. **No Magic Strings**: All variants, sizes, and options use typed enums.

5. **vdom Compatibility**: Components return `*vdom.VNode` and can be composed with raw vdom elements.

## Dependencies

- `github.com/vango-dev/vango/v2/pkg/vdom` - Element creation
- `github.com/vango-dev/vango/v2/pkg/vango` - Signals (for Dialog)

## Testing Strategy

Each component has tests verifying:
1. Default rendering (no options)
2. Variant application
3. Class merging (user classes + variant classes)
4. Children rendering
5. Attribute passthrough
6. For Dialog: hook config serialization and event wiring
