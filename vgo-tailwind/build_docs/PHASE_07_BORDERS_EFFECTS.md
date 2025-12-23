# Phase 7: Borders & Effects

## Overview

Phase 7 implements border utilities (width, radius, style) and visual effects (shadows, opacity, rings). These utilities frequently combine static and functional patterns.

**Prerequisites:** Phase 1 (Core Infrastructure)

**Files to create/modify:**
- `utilities/borders.go` - Border utilities
- `utilities/effects.go` - Shadow, opacity, ring utilities

---

## 1. Border Width

### 1.1 All Sides: `border`, `border-*`

```go
// utilities/borders.go

package utilities

import "github.com/vango-dev/vgo-tailwind"

func (r *Registry) registerBorderUtilities() {
    r.registerBorderWidthUtilities()
    r.registerBorderRadiusUtilities()
    r.registerBorderStyleUtilities()
    r.registerDivideUtilities()
    r.registerOutlineUtilities()
    r.registerRingUtilities()
}

func (r *Registry) registerBorderWidthUtilities() {
    // border (default 1px)
    registerStatic(r, "border", decl("border-width", "1px"))

    // border-0, border-2, border-4, border-8
    borderWidths := map[string]string{
        "0": "0px",
        "2": "2px",
        "4": "4px",
        "8": "8px",
    }

    // Functional for values
    r.RegisterFunctional("border", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        var width string
        if c.Value.Kind == tailwind.ValueArbitrary {
            width = c.Value.Content
        } else {
            var ok bool
            width, ok = borderWidths[c.Value.Content]
            if !ok {
                // Might be a color (handled in Phase 6)
                return nil
            }
        }

        return []tailwind.Declaration{
            {Property: "border-width", Value: width},
        }
    })

    // Individual sides
    sides := map[string]string{
        "t": "top",
        "r": "right",
        "b": "bottom",
        "l": "left",
    }

    for short, full := range sides {
        shortName := short
        fullName := full

        // border-t, border-r, etc. (default 1px)
        registerStatic(r, "border-"+shortName,
            decl("border-"+fullName+"-width", "1px"))

        // border-t-0, border-t-2, etc.
        r.RegisterFunctional("border-"+shortName, func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
            if c.Value == nil {
                return nil
            }

            var width string
            if c.Value.Kind == tailwind.ValueArbitrary {
                width = c.Value.Content
            } else {
                var ok bool
                width, ok = borderWidths[c.Value.Content]
                if !ok {
                    return nil
                }
            }

            return []tailwind.Declaration{
                {Property: "border-" + fullName + "-width", Value: width},
            }
        })
    }

    // border-x, border-y
    registerStatic(r, "border-x",
        decl("border-left-width", "1px"),
        decl("border-right-width", "1px"))
    registerStatic(r, "border-y",
        decl("border-top-width", "1px"),
        decl("border-bottom-width", "1px"))

    r.RegisterFunctional("border-x", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }
        var width string
        if c.Value.Kind == tailwind.ValueArbitrary {
            width = c.Value.Content
        } else {
            var ok bool
            width, ok = borderWidths[c.Value.Content]
            if !ok {
                return nil
            }
        }
        return []tailwind.Declaration{
            {Property: "border-left-width", Value: width},
            {Property: "border-right-width", Value: width},
        }
    })

    r.RegisterFunctional("border-y", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }
        var width string
        if c.Value.Kind == tailwind.ValueArbitrary {
            width = c.Value.Content
        } else {
            var ok bool
            width, ok = borderWidths[c.Value.Content]
            if !ok {
                return nil
            }
        }
        return []tailwind.Declaration{
            {Property: "border-top-width", Value: width},
            {Property: "border-bottom-width", Value: width},
        }
    })
}
```

### 1.2 Border Width Reference

| Class | CSS |
|-------|-----|
| `border` | `border-width: 1px;` |
| `border-0` | `border-width: 0px;` |
| `border-2` | `border-width: 2px;` |
| `border-4` | `border-width: 4px;` |
| `border-8` | `border-width: 8px;` |
| `border-[3px]` | `border-width: 3px;` |
| `border-t` | `border-top-width: 1px;` |
| `border-t-2` | `border-top-width: 2px;` |
| `border-x` | `border-left-width: 1px; border-right-width: 1px;` |
| `border-y-4` | `border-top-width: 4px; border-bottom-width: 4px;` |

---

## 2. Border Radius

```go
func (r *Registry) registerBorderRadiusUtilities() {
    // All corners
    r.RegisterFunctional("rounded", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value := "0.25rem" // default
        if c.Value != nil {
            if c.Value.Kind == tailwind.ValueArbitrary {
                value = c.Value.Content
            } else if v, ok := theme.Resolve(c.Value.Content, "--radius"); ok {
                value = v
            } else {
                return nil
            }
        }
        return []tailwind.Declaration{
            {Property: "border-radius", Value: value},
        }
    })

    // Corners: t, r, b, l, tl, tr, br, bl, s, e, ss, se, es, ee
    corners := map[string][]string{
        "t":  {"border-top-left-radius", "border-top-right-radius"},
        "r":  {"border-top-right-radius", "border-bottom-right-radius"},
        "b":  {"border-bottom-right-radius", "border-bottom-left-radius"},
        "l":  {"border-top-left-radius", "border-bottom-left-radius"},
        "tl": {"border-top-left-radius"},
        "tr": {"border-top-right-radius"},
        "br": {"border-bottom-right-radius"},
        "bl": {"border-bottom-left-radius"},
        "s":  {"border-start-start-radius", "border-end-start-radius"},
        "e":  {"border-start-end-radius", "border-end-end-radius"},
        "ss": {"border-start-start-radius"},
        "se": {"border-start-end-radius"},
        "es": {"border-end-start-radius"},
        "ee": {"border-end-end-radius"},
    }

    for corner, props := range corners {
        cornerName := corner
        properties := props

        r.RegisterFunctional("rounded-"+cornerName, func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
            value := "0.25rem"
            if c.Value != nil {
                if c.Value.Kind == tailwind.ValueArbitrary {
                    value = c.Value.Content
                } else if v, ok := theme.Resolve(c.Value.Content, "--radius"); ok {
                    value = v
                } else {
                    return nil
                }
            }

            decls := make([]tailwind.Declaration, len(properties))
            for i, prop := range properties {
                decls[i] = tailwind.Declaration{Property: prop, Value: value}
            }
            return decls
        })
    }
}
```

### 2.1 Border Radius Scale

| Class | CSS |
|-------|-----|
| `rounded-none` | `border-radius: 0px;` |
| `rounded-sm` | `border-radius: 0.125rem;` |
| `rounded` | `border-radius: 0.25rem;` |
| `rounded-md` | `border-radius: 0.375rem;` |
| `rounded-lg` | `border-radius: 0.5rem;` |
| `rounded-xl` | `border-radius: 0.75rem;` |
| `rounded-2xl` | `border-radius: 1rem;` |
| `rounded-3xl` | `border-radius: 1.5rem;` |
| `rounded-full` | `border-radius: 9999px;` |
| `rounded-[10px]` | `border-radius: 10px;` |
| `rounded-t-lg` | `border-top-left-radius: 0.5rem; border-top-right-radius: 0.5rem;` |
| `rounded-tl-lg` | `border-top-left-radius: 0.5rem;` |

---

## 3. Border Style

```go
func (r *Registry) registerBorderStyleUtilities() {
    styles := []string{"solid", "dashed", "dotted", "double", "hidden", "none"}

    for _, style := range styles {
        styleName := style
        registerStatic(r, "border-"+styleName, decl("border-style", styleName))
    }
}
```

| Class | CSS |
|-------|-----|
| `border-solid` | `border-style: solid;` |
| `border-dashed` | `border-style: dashed;` |
| `border-dotted` | `border-style: dotted;` |
| `border-double` | `border-style: double;` |
| `border-hidden` | `border-style: hidden;` |
| `border-none` | `border-style: none;` |

---

## 4. Divide Utilities

Divide utilities add borders between children.

```go
func (r *Registry) registerDivideUtilities() {
    // divide-x, divide-y (default 1px)
    // These require special selectors: > :not([hidden]) ~ :not([hidden])

    r.RegisterFunctional("divide-x", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        width := "1px"
        if c.Value != nil {
            if c.Value.Kind == tailwind.ValueArbitrary {
                width = c.Value.Content
            } else {
                switch c.Value.Content {
                case "0":
                    width = "0px"
                case "2":
                    width = "2px"
                case "4":
                    width = "4px"
                case "8":
                    width = "8px"
                default:
                    return nil
                }
            }
        }
        return []tailwind.Declaration{
            {Property: "border-left-width", Value: width},
        }
    })

    r.RegisterFunctional("divide-y", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        width := "1px"
        if c.Value != nil {
            if c.Value.Kind == tailwind.ValueArbitrary {
                width = c.Value.Content
            } else {
                switch c.Value.Content {
                case "0":
                    width = "0px"
                case "2":
                    width = "2px"
                case "4":
                    width = "4px"
                case "8":
                    width = "8px"
                default:
                    return nil
                }
            }
        }
        return []tailwind.Declaration{
            {Property: "border-top-width", Value: width},
        }
    })

    // divide-x-reverse, divide-y-reverse
    registerStatic(r, "divide-x-reverse", decl("--tw-divide-x-reverse", "1"))
    registerStatic(r, "divide-y-reverse", decl("--tw-divide-y-reverse", "1"))

    // divide-style
    styles := []string{"solid", "dashed", "dotted", "double", "none"}
    for _, style := range styles {
        registerStatic(r, "divide-"+style, decl("border-style", style))
    }
}
```

---

## 5. Outline Utilities

```go
func (r *Registry) registerOutlineUtilities() {
    // outline-none (special)
    r.RegisterStatic("outline-none", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        return []tailwind.Declaration{
            {Property: "outline", Value: "2px solid transparent"},
            {Property: "outline-offset", Value: "2px"},
        }
    })

    // outline (default)
    registerStatic(r, "outline", decl("outline-style", "solid"))

    // outline-dashed, outline-dotted, outline-double
    registerStatic(r, "outline-dashed", decl("outline-style", "dashed"))
    registerStatic(r, "outline-dotted", decl("outline-style", "dotted"))
    registerStatic(r, "outline-double", decl("outline-style", "double"))

    // outline-width: outline-0, outline-1, outline-2, outline-4, outline-8
    outlineWidths := map[string]string{
        "0": "0px",
        "1": "1px",
        "2": "2px",
        "4": "4px",
        "8": "8px",
    }

    r.RegisterFunctional("outline", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        if c.Value.Kind == tailwind.ValueArbitrary {
            return []tailwind.Declaration{
                {Property: "outline-width", Value: c.Value.Content},
            }
        }

        if width, ok := outlineWidths[c.Value.Content]; ok {
            return []tailwind.Declaration{
                {Property: "outline-width", Value: width},
            }
        }

        // Could be a color (handled in Phase 6)
        return nil
    })

    // outline-offset
    r.RegisterFunctional("outline-offset", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        var value string
        if c.Value.Kind == tailwind.ValueArbitrary {
            value = c.Value.Content
        } else {
            switch c.Value.Content {
            case "0":
                value = "0px"
            case "1":
                value = "1px"
            case "2":
                value = "2px"
            case "4":
                value = "4px"
            case "8":
                value = "8px"
            default:
                return nil
            }
        }

        return []tailwind.Declaration{
            {Property: "outline-offset", Value: value},
        }
    })
}
```

---

## 6. Ring Utilities

```go
func (r *Registry) registerRingUtilities() {
    // ring (default 3px)
    r.RegisterStatic("ring", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        return []tailwind.Declaration{
            {Property: "box-shadow", Value: "var(--tw-ring-offset-shadow), var(--tw-ring-shadow), var(--tw-shadow, 0 0 #0000)"},
            {Property: "--tw-ring-offset-shadow", Value: "var(--tw-ring-inset) 0 0 0 var(--tw-ring-offset-width) var(--tw-ring-offset-color)"},
            {Property: "--tw-ring-shadow", Value: "var(--tw-ring-inset) 0 0 0 calc(3px + var(--tw-ring-offset-width)) var(--tw-ring-color)"},
        }
    })

    // ring-0, ring-1, ring-2, ring-4, ring-8
    ringWidths := map[string]string{
        "0": "0px",
        "1": "1px",
        "2": "2px",
        "4": "4px",
        "8": "8px",
    }

    r.RegisterFunctional("ring", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        var width string
        if c.Value.Kind == tailwind.ValueArbitrary {
            width = c.Value.Content
        } else if w, ok := ringWidths[c.Value.Content]; ok {
            width = w
        } else {
            // Could be a color (handled in Phase 6)
            return nil
        }

        return []tailwind.Declaration{
            {Property: "box-shadow", Value: "var(--tw-ring-offset-shadow), var(--tw-ring-shadow), var(--tw-shadow, 0 0 #0000)"},
            {Property: "--tw-ring-offset-shadow", Value: "var(--tw-ring-inset) 0 0 0 var(--tw-ring-offset-width) var(--tw-ring-offset-color)"},
            {Property: "--tw-ring-shadow", Value: fmt.Sprintf("var(--tw-ring-inset) 0 0 0 calc(%s + var(--tw-ring-offset-width)) var(--tw-ring-color)", width)},
        }
    })

    // ring-inset
    registerStatic(r, "ring-inset", decl("--tw-ring-inset", "inset"))

    // ring-offset-* (width)
    r.RegisterFunctional("ring-offset", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        var width string
        if c.Value.Kind == tailwind.ValueArbitrary {
            width = c.Value.Content
        } else if w, ok := ringWidths[c.Value.Content]; ok {
            width = w
        } else {
            // Could be a color
            return nil
        }

        return []tailwind.Declaration{
            {Property: "--tw-ring-offset-width", Value: width},
        }
    })
}
```

---

## 7. Box Shadow

```go
// utilities/effects.go

package utilities

func (r *Registry) registerEffectsUtilities() {
    r.registerBoxShadowUtilities()
    r.registerOpacityUtilities()
}

func (r *Registry) registerBoxShadowUtilities() {
    shadows := map[string]string{
        "sm":    "0 1px 2px 0 rgb(0 0 0 / 0.05)",
        "":      "0 1px 3px 0 rgb(0 0 0 / 0.1), 0 1px 2px -1px rgb(0 0 0 / 0.1)",
        "md":    "0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)",
        "lg":    "0 10px 15px -3px rgb(0 0 0 / 0.1), 0 4px 6px -4px rgb(0 0 0 / 0.1)",
        "xl":    "0 20px 25px -5px rgb(0 0 0 / 0.1), 0 8px 10px -6px rgb(0 0 0 / 0.1)",
        "2xl":   "0 25px 50px -12px rgb(0 0 0 / 0.25)",
        "inner": "inset 0 2px 4px 0 rgb(0 0 0 / 0.05)",
        "none":  "0 0 #0000",
    }

    // shadow (default)
    registerStatic(r, "shadow",
        decl("box-shadow", "var(--tw-ring-offset-shadow, 0 0 #0000), var(--tw-ring-shadow, 0 0 #0000), var(--tw-shadow)"),
        decl("--tw-shadow", shadows[""]),
        decl("--tw-shadow-colored", "0 1px 3px 0 var(--tw-shadow-color), 0 1px 2px -1px var(--tw-shadow-color)"),
    )

    // shadow-sm, shadow-md, etc.
    for name, value := range shadows {
        if name == "" {
            continue
        }
        shadowValue := value
        registerStatic(r, "shadow-"+name,
            decl("box-shadow", "var(--tw-ring-offset-shadow, 0 0 #0000), var(--tw-ring-shadow, 0 0 #0000), var(--tw-shadow)"),
            decl("--tw-shadow", shadowValue),
        )
    }

    // Arbitrary shadow
    r.RegisterFunctional("shadow", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        if c.Value.Kind == tailwind.ValueArbitrary {
            return []tailwind.Declaration{
                {Property: "box-shadow", Value: c.Value.Content},
            }
        }

        // Could be a color for shadow-color
        return nil
    })
}
```

### 7.1 Shadow Reference

| Class | Description |
|-------|-------------|
| `shadow-sm` | Small shadow |
| `shadow` | Default shadow |
| `shadow-md` | Medium shadow |
| `shadow-lg` | Large shadow |
| `shadow-xl` | Extra large shadow |
| `shadow-2xl` | 2x extra large shadow |
| `shadow-inner` | Inner shadow |
| `shadow-none` | No shadow |
| `shadow-[0_4px_6px_rgba(0,0,0,0.1)]` | Arbitrary shadow |

---

## 8. Opacity

```go
func (r *Registry) registerOpacityUtilities() {
    opacities := map[string]string{
        "0":   "0",
        "5":   "0.05",
        "10":  "0.1",
        "15":  "0.15",
        "20":  "0.2",
        "25":  "0.25",
        "30":  "0.3",
        "35":  "0.35",
        "40":  "0.4",
        "45":  "0.45",
        "50":  "0.5",
        "55":  "0.55",
        "60":  "0.6",
        "65":  "0.65",
        "70":  "0.7",
        "75":  "0.75",
        "80":  "0.8",
        "85":  "0.85",
        "90":  "0.9",
        "95":  "0.95",
        "100": "1",
    }

    r.RegisterFunctional("opacity", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        var value string
        if c.Value.Kind == tailwind.ValueArbitrary {
            value = c.Value.Content
        } else if v, ok := opacities[c.Value.Content]; ok {
            value = v
        } else {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "opacity", Value: value},
        }
    })
}
```

### 8.1 Opacity Reference

| Class | CSS |
|-------|-----|
| `opacity-0` | `opacity: 0;` |
| `opacity-25` | `opacity: 0.25;` |
| `opacity-50` | `opacity: 0.5;` |
| `opacity-75` | `opacity: 0.75;` |
| `opacity-100` | `opacity: 1;` |
| `opacity-[.33]` | `opacity: .33;` |

---

## 9. Testing

```go
// utilities/borders_test.go

func TestBorderWidthUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        property string
        value    string
    }{
        {"border", "border-width", "1px"},
        {"border-0", "border-width", "0px"},
        {"border-2", "border-width", "2px"},
        {"border-t", "border-top-width", "1px"},
        {"border-t-4", "border-top-width", "4px"},
        {"border-[3px]", "border-width", "3px"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            // Test implementation
        })
    }
}

func TestBorderRadiusUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        expected string
    }{
        {"rounded", "0.25rem"},
        {"rounded-lg", "0.5rem"},
        {"rounded-full", "9999px"},
        {"rounded-none", "0px"},
        {"rounded-[10px]", "10px"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            // Test implementation
        })
    }
}

func TestShadowUtilities(t *testing.T) {
    registry := NewRegistry()
    engine := tailwind.New()

    css := engine.Generate([]string{"shadow", "shadow-lg", "shadow-none"})

    assert.Contains(t, css, ".shadow")
    assert.Contains(t, css, "box-shadow")
}

func TestOpacityUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        expected string
    }{
        {"opacity-0", "0"},
        {"opacity-50", "0.5"},
        {"opacity-100", "1"},
        {"opacity-[.33]", ".33"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            c, err := tailwind.ParseCandidate(tt.class, registry)
            assert.NoError(t, err)

            util, ok := registry.Get("opacity", UtilityFunctional)
            assert.True(t, ok)

            decls := util.Compile(c, theme)
            assert.Len(t, decls, 1)
            assert.Equal(t, "opacity", decls[0].Property)
            assert.Equal(t, tt.expected, decls[0].Value)
        })
    }
}
```

---

## 10. Files to Create

| File | Purpose | Lines (est.) |
|------|---------|--------------|
| `utilities/borders.go` | Border, divide, outline, ring | 400 |
| `utilities/borders_test.go` | Border tests | 200 |
| `utilities/effects.go` | Shadow, opacity | 150 |
| `utilities/effects_test.go` | Effects tests | 100 |

---

## 11. Completion Criteria

Phase 7 is complete when:

1. ✅ Border width works (`border`, `border-2`, `border-t-4`)
2. ✅ Border radius works (`rounded`, `rounded-lg`, `rounded-t-xl`)
3. ✅ Border style works (`border-solid`, `border-dashed`)
4. ✅ Divide utilities work (`divide-x`, `divide-y-2`)
5. ✅ Outline utilities work (`outline`, `outline-2`, `outline-offset-2`)
6. ✅ Ring utilities work (`ring`, `ring-2`, `ring-offset-2`)
7. ✅ Shadow utilities work (`shadow`, `shadow-lg`, `shadow-none`)
8. ✅ Opacity utilities work (`opacity-50`, `opacity-[.33]`)
9. ✅ Arbitrary values work for all utilities
10. ✅ All tests pass

---

*Last Updated: 2024-12-12*
