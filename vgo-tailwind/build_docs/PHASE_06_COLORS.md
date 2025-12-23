# Phase 6: Color Utilities

## Overview

Phase 6 implements color utilities—text color, background color, border color, and related utilities with opacity modifier support. This is one of the more complex phases due to the size of the color palette and the opacity modifier system.

**Prerequisites:** Phase 1 (Core Infrastructure)

**Files to create/modify:**
- `utilities/colors.go` - Color utility implementations
- Update `theme.go` with full color palette

---

## 1. Color Resolution

Colors can be specified as:
1. **Named colors**: `red-500`, `blue-600`, `gray-100`
2. **Special values**: `inherit`, `current`, `transparent`, `white`, `black`
3. **Arbitrary values**: `[#ff0000]`, `[rgb(255,0,0)]`, `[hsl(0,100%,50%)]`
4. **With opacity modifier**: `red-500/50`, `blue-600/[.25]`

```go
// utilities/colors.go

package utilities

import (
    "fmt"
    "strings"

    "github.com/vango-dev/vgo-tailwind"
)

// resolveColor resolves a color value, optionally with opacity modifier.
func resolveColor(value *tailwind.Value, modifier *tailwind.Modifier, theme *tailwind.Theme) (string, bool) {
    if value == nil {
        return "", false
    }

    var color string

    switch value.Kind {
    case tailwind.ValueArbitrary:
        color = value.Content

    case tailwind.ValueNamed:
        // Check special values first
        switch value.Content {
        case "inherit":
            return "inherit", true
        case "current":
            return "currentColor", true
        case "transparent":
            return "transparent", true
        case "white":
            color = "#ffffff"
        case "black":
            color = "#000000"
        default:
            // Look up in theme
            var ok bool
            color, ok = theme.Resolve(value.Content, "--color")
            if !ok {
                return "", false
            }
        }
    }

    // Apply opacity modifier if present
    if modifier != nil {
        color = applyOpacity(color, modifier)
    }

    return color, true
}

// applyOpacity applies an opacity modifier to a color.
func applyOpacity(color string, mod *tailwind.Modifier) string {
    var opacity string

    switch mod.Kind {
    case tailwind.ModifierArbitrary:
        // Arbitrary: /[.25] or /[25%]
        opacity = mod.Value
    case tailwind.ModifierNamed:
        // Named: /50 means 50% opacity
        opacity = mod.Value + "%"
    }

    // Convert color to rgba/color-mix format
    // For hex colors, use color-mix
    if strings.HasPrefix(color, "#") || strings.HasPrefix(color, "rgb") || strings.HasPrefix(color, "hsl") {
        return fmt.Sprintf("color-mix(in srgb, %s %s, transparent)", color, opacity)
    }

    // For oklch colors (Tailwind v4 default), use the alpha channel
    if strings.HasPrefix(color, "oklch(") {
        // Insert opacity into oklch
        return strings.TrimSuffix(color, ")") + " / " + opacity + ")"
    }

    return color
}
```

---

## 2. Text Color: `text-*`

**Note:** `text-*` is shared between font size and text color. The utility checks font size first, then falls back to color.

```go
func (r *Registry) registerTextColorUtility() {
    // This is registered as part of the "text" functional utility
    // The compile function tries font-size first, then color

    // We need to update the existing text utility to handle both
    r.RegisterFunctional("text", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        // Try font size first (from Phase 5)
        if fontSize, ok := theme.Resolve(c.Value.Content, "--font-size"); ok {
            lineHeight, _ := theme.Resolve(c.Value.Content, "--font-size-line-height")
            decls := []tailwind.Declaration{{Property: "font-size", Value: fontSize}}
            if lineHeight != "" {
                decls = append(decls, tailwind.Declaration{Property: "line-height", Value: lineHeight})
            }
            return decls
        }

        // Try as color
        color, ok := resolveColor(c.Value, c.Modifier, theme)
        if !ok {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "color", Value: color},
        }
    })
}
```

### 2.1 Text Color Examples

| Class | CSS |
|-------|-----|
| `text-inherit` | `color: inherit;` |
| `text-current` | `color: currentColor;` |
| `text-transparent` | `color: transparent;` |
| `text-black` | `color: #000000;` |
| `text-white` | `color: #ffffff;` |
| `text-red-500` | `color: #ef4444;` |
| `text-blue-600` | `color: #2563eb;` |
| `text-gray-900` | `color: #111827;` |
| `text-red-500/50` | `color: color-mix(in srgb, #ef4444 50%, transparent);` |
| `text-[#ff0000]` | `color: #ff0000;` |
| `text-[rgb(255,0,0)]` | `color: rgb(255,0,0);` |

---

## 3. Background Color: `bg-*`

```go
func (r *Registry) registerBackgroundColorUtility() {
    r.RegisterFunctional("bg", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        // Resolve color
        color, ok := resolveColor(c.Value, c.Modifier, theme)
        if !ok {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "background-color", Value: color},
        }
    })
}
```

### 3.1 Background Color Examples

| Class | CSS |
|-------|-----|
| `bg-inherit` | `background-color: inherit;` |
| `bg-transparent` | `background-color: transparent;` |
| `bg-white` | `background-color: #ffffff;` |
| `bg-black` | `background-color: #000000;` |
| `bg-red-500` | `background-color: #ef4444;` |
| `bg-blue-100` | `background-color: #dbeafe;` |
| `bg-gray-50` | `background-color: #f9fafb;` |
| `bg-red-500/50` | `background-color: color-mix(in srgb, #ef4444 50%, transparent);` |
| `bg-[#ff0000]` | `background-color: #ff0000;` |

---

## 4. Border Color: `border-*`

```go
func (r *Registry) registerBorderColorUtility() {
    // Note: border-* is shared between border-width and border-color
    // The compile function checks if it's a width value first

    r.RegisterFunctional("border", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            // No value means border-width: 1px (static, handled elsewhere)
            return nil
        }

        // Check if this is a width value (number or px)
        if isWidthValue(c.Value) {
            return compileBorderWidth(c, theme)
        }

        // Try as color
        color, ok := resolveColor(c.Value, c.Modifier, theme)
        if !ok {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "border-color", Value: color},
        }
    })

    // Individual sides
    for _, side := range []string{"t", "r", "b", "l", "x", "y", "s", "e"} {
        sideName := side
        r.RegisterFunctional("border-"+side, func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
            if c.Value == nil {
                return nil
            }

            // Check if width value
            if isWidthValue(c.Value) {
                return compileBorderWidthSide(c, theme, sideName)
            }

            // Try as color
            color, ok := resolveColor(c.Value, c.Modifier, theme)
            if !ok {
                return nil
            }

            return borderColorForSide(sideName, color)
        })
    }
}

func isWidthValue(v *tailwind.Value) bool {
    if v.Kind == tailwind.ValueArbitrary {
        // Check if it looks like a length
        return strings.HasSuffix(v.Content, "px") ||
               strings.HasSuffix(v.Content, "rem") ||
               strings.HasSuffix(v.Content, "em")
    }
    // Named width values: 0, 2, 4, 8
    switch v.Content {
    case "0", "2", "4", "8":
        return true
    }
    return false
}

func borderColorForSide(side, color string) []tailwind.Declaration {
    switch side {
    case "t":
        return []tailwind.Declaration{{Property: "border-top-color", Value: color}}
    case "r":
        return []tailwind.Declaration{{Property: "border-right-color", Value: color}}
    case "b":
        return []tailwind.Declaration{{Property: "border-bottom-color", Value: color}}
    case "l":
        return []tailwind.Declaration{{Property: "border-left-color", Value: color}}
    case "x":
        return []tailwind.Declaration{
            {Property: "border-left-color", Value: color},
            {Property: "border-right-color", Value: color},
        }
    case "y":
        return []tailwind.Declaration{
            {Property: "border-top-color", Value: color},
            {Property: "border-bottom-color", Value: color},
        }
    case "s":
        return []tailwind.Declaration{{Property: "border-inline-start-color", Value: color}}
    case "e":
        return []tailwind.Declaration{{Property: "border-inline-end-color", Value: color}}
    }
    return nil
}
```

### 4.1 Border Color Examples

| Class | CSS |
|-------|-----|
| `border-transparent` | `border-color: transparent;` |
| `border-gray-300` | `border-color: #d1d5db;` |
| `border-red-500` | `border-color: #ef4444;` |
| `border-t-gray-200` | `border-top-color: #e5e7eb;` |
| `border-x-blue-500` | `border-left-color: #3b82f6; border-right-color: #3b82f6;` |
| `border-red-500/50` | `border-color: color-mix(in srgb, #ef4444 50%, transparent);` |

---

## 5. Ring Color: `ring-*`

```go
func (r *Registry) registerRingColorUtility() {
    r.RegisterFunctional("ring", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            // ring with no value is ring-width (handled in Phase 7)
            return nil
        }

        // Check if this is a width value
        if isRingWidthValue(c.Value) {
            return compileRingWidth(c, theme)
        }

        // Try as color
        color, ok := resolveColor(c.Value, c.Modifier, theme)
        if !ok {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "--tw-ring-color", Value: color},
        }
    })

    // Ring offset color
    r.RegisterFunctional("ring-offset", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        // Check if width
        if isRingWidthValue(c.Value) {
            return compileRingOffsetWidth(c, theme)
        }

        // Color
        color, ok := resolveColor(c.Value, c.Modifier, theme)
        if !ok {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "--tw-ring-offset-color", Value: color},
        }
    })
}
```

---

## 6. Divide Color: `divide-*`

```go
func (r *Registry) registerDivideColorUtility() {
    r.RegisterFunctional("divide", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        // Check if width
        if isDivideWidthValue(c.Value) {
            return compileDivideWidth(c, theme)
        }

        // Color
        color, ok := resolveColor(c.Value, c.Modifier, theme)
        if !ok {
            return nil
        }

        // Divide uses the child selector like space-*
        return []tailwind.Declaration{
            {Property: "border-color", Value: color},
        }
    })
}
```

---

## 7. Outline Color: `outline-*`

```go
func (r *Registry) registerOutlineColorUtility() {
    r.RegisterFunctional("outline", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        // Check for outline-style values (none, dashed, etc.)
        if isOutlineStyleValue(c.Value) {
            return compileOutlineStyle(c)
        }

        // Check for outline-width values (0, 1, 2, 4, 8)
        if isOutlineWidthValue(c.Value) {
            return compileOutlineWidth(c, theme)
        }

        // Try as color
        color, ok := resolveColor(c.Value, c.Modifier, theme)
        if !ok {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "outline-color", Value: color},
        }
    })
}
```

---

## 8. Accent Color: `accent-*`

```go
func (r *Registry) registerAccentColorUtility() {
    r.RegisterFunctional("accent", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        // Special: accent-auto
        if c.Value.Kind == tailwind.ValueNamed && c.Value.Content == "auto" {
            return []tailwind.Declaration{
                {Property: "accent-color", Value: "auto"},
            }
        }

        color, ok := resolveColor(c.Value, c.Modifier, theme)
        if !ok {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "accent-color", Value: color},
        }
    })
}
```

---

## 9. Caret Color: `caret-*`

```go
func (r *Registry) registerCaretColorUtility() {
    r.RegisterFunctional("caret", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        color, ok := resolveColor(c.Value, c.Modifier, theme)
        if !ok {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "caret-color", Value: color},
        }
    })
}
```

---

## 10. Placeholder Color: `placeholder-*`

```go
func (r *Registry) registerPlaceholderColorUtility() {
    r.RegisterFunctional("placeholder", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        color, ok := resolveColor(c.Value, c.Modifier, theme)
        if !ok {
            return nil
        }

        // Note: This needs special selector handling for ::placeholder
        return []tailwind.Declaration{
            {Property: "color", Value: color},
        }
    })
}
```

---

## 11. Fill and Stroke Colors

```go
func (r *Registry) registerSVGColorUtilities() {
    // fill-*
    r.RegisterFunctional("fill", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        color, ok := resolveColor(c.Value, c.Modifier, theme)
        if !ok {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "fill", Value: color},
        }
    })

    // stroke-*
    r.RegisterFunctional("stroke", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        // Check if stroke-width value
        if isStrokeWidthValue(c.Value) {
            return compileStrokeWidth(c, theme)
        }

        color, ok := resolveColor(c.Value, c.Modifier, theme)
        if !ok {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "stroke", Value: color},
        }
    })
}
```

---

## 12. Complete Color Palette

The full Tailwind color palette must be loaded into the theme:

```go
// theme.go

func (t *Theme) loadColors() {
    // Each color has shades from 50-950
    t.loadColorPalette("slate", slateColors)
    t.loadColorPalette("gray", grayColors)
    t.loadColorPalette("zinc", zincColors)
    t.loadColorPalette("neutral", neutralColors)
    t.loadColorPalette("stone", stoneColors)
    t.loadColorPalette("red", redColors)
    t.loadColorPalette("orange", orangeColors)
    t.loadColorPalette("amber", amberColors)
    t.loadColorPalette("yellow", yellowColors)
    t.loadColorPalette("lime", limeColors)
    t.loadColorPalette("green", greenColors)
    t.loadColorPalette("emerald", emeraldColors)
    t.loadColorPalette("teal", tealColors)
    t.loadColorPalette("cyan", cyanColors)
    t.loadColorPalette("sky", skyColors)
    t.loadColorPalette("blue", blueColors)
    t.loadColorPalette("indigo", indigoColors)
    t.loadColorPalette("violet", violetColors)
    t.loadColorPalette("purple", purpleColors)
    t.loadColorPalette("fuchsia", fuchsiaColors)
    t.loadColorPalette("pink", pinkColors)
    t.loadColorPalette("rose", roseColors)
}

func (t *Theme) loadColorPalette(name string, colors map[string]string) {
    for shade, value := range colors {
        t.Set("--color", name+"-"+shade, value)
    }
}

// Example color definitions (use Tailwind's actual values)
var redColors = map[string]string{
    "50":  "#fef2f2",
    "100": "#fee2e2",
    "200": "#fecaca",
    "300": "#fca5a5",
    "400": "#f87171",
    "500": "#ef4444",
    "600": "#dc2626",
    "700": "#b91c1c",
    "800": "#991b1b",
    "900": "#7f1d1d",
    "950": "#450a0a",
}

var blueColors = map[string]string{
    "50":  "#eff6ff",
    "100": "#dbeafe",
    "200": "#bfdbfe",
    "300": "#93c5fd",
    "400": "#60a5fa",
    "500": "#3b82f6",
    "600": "#2563eb",
    "700": "#1d4ed8",
    "800": "#1e40af",
    "900": "#1e3a8a",
    "950": "#172554",
}

// ... define all other color palettes
```

---

## 13. Opacity Modifier Deep Dive

The opacity modifier (`/50`, `/[.25]`) applies opacity to colors:

### 13.1 Named Opacity Values

| Modifier | Opacity |
|----------|---------|
| `/0` | 0% |
| `/5` | 5% |
| `/10` | 10% |
| `/20` | 20% |
| `/25` | 25% |
| `/30` | 30% |
| `/40` | 40% |
| `/50` | 50% |
| `/60` | 60% |
| `/70` | 70% |
| `/75` | 75% |
| `/80` | 80% |
| `/90` | 90% |
| `/95` | 95% |
| `/100` | 100% |

### 13.2 Arbitrary Opacity

| Modifier | Opacity |
|----------|---------|
| `/[.25]` | 25% |
| `/[0.5]` | 50% |
| `/[50%]` | 50% |

---

## 14. Testing

```go
// utilities/colors_test.go

func TestTextColorUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        expected string
    }{
        {"text-inherit", "inherit"},
        {"text-current", "currentColor"},
        {"text-transparent", "transparent"},
        {"text-black", "#000000"},
        {"text-white", "#ffffff"},
        {"text-red-500", "#ef4444"},
        {"text-[#ff0000]", "#ff0000"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            c, err := tailwind.ParseCandidate(tt.class, registry)
            assert.NoError(t, err)

            util, ok := registry.Get("text", UtilityFunctional)
            assert.True(t, ok)

            decls := util.Compile(c, theme)
            assert.NotEmpty(t, decls)
            assert.Equal(t, "color", decls[0].Property)
            assert.Equal(t, tt.expected, decls[0].Value)
        })
    }
}

func TestColorWithOpacity(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    c, err := tailwind.ParseCandidate("text-red-500/50", registry)
    assert.NoError(t, err)
    assert.NotNil(t, c.Modifier)
    assert.Equal(t, "50", c.Modifier.Value)

    util, ok := registry.Get("text", UtilityFunctional)
    assert.True(t, ok)

    decls := util.Compile(c, theme)
    assert.NotEmpty(t, decls)
    assert.Equal(t, "color", decls[0].Property)
    assert.Contains(t, decls[0].Value, "50%")
}

func TestBackgroundColorUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        expected string
    }{
        {"bg-white", "#ffffff"},
        {"bg-gray-100", "#f3f4f6"},
        {"bg-blue-500", "#3b82f6"},
        {"bg-transparent", "transparent"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            c, err := tailwind.ParseCandidate(tt.class, registry)
            assert.NoError(t, err)

            util, ok := registry.Get("bg", UtilityFunctional)
            assert.True(t, ok)

            decls := util.Compile(c, theme)
            assert.Len(t, decls, 1)
            assert.Equal(t, "background-color", decls[0].Property)
            assert.Equal(t, tt.expected, decls[0].Value)
        })
    }
}
```

---

## 15. Files to Create

| File | Purpose | Lines (est.) |
|------|---------|--------------|
| `utilities/colors.go` | All color utilities | 400 |
| `utilities/colors_test.go` | Color tests | 250 |
| Update `theme.go` | Color palette data | 500 |

---

## 16. Completion Criteria

Phase 6 is complete when:

1. ✅ Text color works (`text-red-500`, `text-[#ff0000]`)
2. ✅ Background color works (`bg-blue-500`, `bg-[rgb(0,0,255)]`)
3. ✅ Border color works (`border-gray-300`, `border-t-red-500`)
4. ✅ Ring color works (`ring-blue-500`)
5. ✅ Divide color works (`divide-gray-200`)
6. ✅ Outline color works (`outline-red-500`)
7. ✅ Accent color works (`accent-pink-500`)
8. ✅ Caret color works (`caret-blue-500`)
9. ✅ Fill/stroke colors work (`fill-current`, `stroke-gray-500`)
10. ✅ Special values work (`text-inherit`, `text-current`, `text-transparent`)
11. ✅ Opacity modifiers work (`text-red-500/50`, `bg-blue-500/[.25]`)
12. ✅ Full color palette is loaded (all colors from slate to rose)
13. ✅ All tests pass

---

*Last Updated: 2024-12-12*
