# Phase 4: Sizing Utilities

## Overview

Phase 4 implements all sizing utilities—width, height, min/max dimensions, and aspect ratio. These utilities demonstrate handling of multiple value types: spacing scale, percentages, fractions, and keywords.

**Prerequisites:** Phase 1 (Core Infrastructure)

**Files to create/modify:**
- `utilities/sizing.go` - All sizing utility implementations

---

## 1. Value Types for Sizing

Sizing utilities accept several types of values:

1. **Spacing scale**: `w-4` → `1rem`
2. **Fractions**: `w-1/2` → `50%`
3. **Keywords**: `w-full`, `w-screen`, `w-auto`, `w-fit`
4. **Arbitrary**: `w-[300px]`, `w-[50vw]`

```go
// utilities/sizing.go

package utilities

import (
    "fmt"
    "strings"

    "github.com/vango-dev/vgo-tailwind"
)

// resolveSizing resolves a sizing value from theme, keywords, or arbitrary.
func resolveSizing(value *tailwind.Value, theme *tailwind.Theme) (string, bool) {
    if value == nil {
        return "", false
    }

    switch value.Kind {
    case tailwind.ValueArbitrary:
        return value.Content, true

    case tailwind.ValueNamed:
        content := value.Content

        // Check for keywords first
        switch content {
        case "auto":
            return "auto", true
        case "full":
            return "100%", true
        case "screen":
            return "100vw", true // or 100vh for height
        case "svw":
            return "100svw", true
        case "lvw":
            return "100lvw", true
        case "dvw":
            return "100dvw", true
        case "min":
            return "min-content", true
        case "max":
            return "max-content", true
        case "fit":
            return "fit-content", true
        case "px":
            return "1px", true
        }

        // Check for fractions
        if value.Fraction != "" {
            return fractionToPercent(value.Fraction)
        }

        // Check spacing scale
        if v, ok := theme.Resolve(content, "--spacing"); ok {
            return v, true
        }
    }

    return "", false
}

// fractionToPercent converts "1/2" to "50%", "2/3" to "66.666667%", etc.
func fractionToPercent(fraction string) (string, bool) {
    parts := strings.Split(fraction, "/")
    if len(parts) != 2 {
        return "", false
    }

    // Parse numerator and denominator
    var num, denom int
    _, err := fmt.Sscanf(fraction, "%d/%d", &num, &denom)
    if err != nil || denom == 0 {
        return "", false
    }

    percent := float64(num) / float64(denom) * 100
    return fmt.Sprintf("%g%%", percent), true
}
```

---

## 2. Width Utilities

### 2.1 Basic Width: `w-*`

```go
func (r *Registry) registerSizingUtilities() {
    // w-* (width)
    r.RegisterFunctional("w", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSizing(c.Value, theme)
        if !ok {
            return nil
        }
        // Handle screen keyword for width specifically
        if c.Value.Kind == tailwind.ValueNamed && c.Value.Content == "screen" {
            value = "100vw"
        }
        return []tailwind.Declaration{
            {Property: "width", Value: value},
        }
    })
```

### 2.2 Min/Max Width: `min-w-*`, `max-w-*`

```go
    // min-w-* (min-width)
    r.RegisterFunctional("min-w", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSizingMinMax(c.Value, theme, "width")
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "min-width", Value: value},
        }
    })

    // max-w-* (max-width)
    r.RegisterFunctional("max-w", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSizingMinMax(c.Value, theme, "width")
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "max-width", Value: value},
        }
    })
```

### 2.3 Width Reference Table

| Class | CSS |
|-------|-----|
| `w-0` | `width: 0px;` |
| `w-px` | `width: 1px;` |
| `w-0.5` | `width: 0.125rem;` |
| `w-1` | `width: 0.25rem;` |
| `w-4` | `width: 1rem;` |
| `w-64` | `width: 16rem;` |
| `w-auto` | `width: auto;` |
| `w-1/2` | `width: 50%;` |
| `w-1/3` | `width: 33.333333%;` |
| `w-2/3` | `width: 66.666667%;` |
| `w-1/4` | `width: 25%;` |
| `w-3/4` | `width: 75%;` |
| `w-1/5` | `width: 20%;` |
| `w-2/5` | `width: 40%;` |
| `w-3/5` | `width: 60%;` |
| `w-4/5` | `width: 80%;` |
| `w-1/6` | `width: 16.666667%;` |
| `w-5/6` | `width: 83.333333%;` |
| `w-1/12` | `width: 8.333333%;` |
| `w-full` | `width: 100%;` |
| `w-screen` | `width: 100vw;` |
| `w-svw` | `width: 100svw;` |
| `w-lvw` | `width: 100lvw;` |
| `w-dvw` | `width: 100dvw;` |
| `w-min` | `width: min-content;` |
| `w-max` | `width: max-content;` |
| `w-fit` | `width: fit-content;` |
| `w-[300px]` | `width: 300px;` |

---

## 3. Height Utilities

### 3.1 Basic Height: `h-*`

```go
    // h-* (height)
    r.RegisterFunctional("h", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSizing(c.Value, theme)
        if !ok {
            return nil
        }
        // Handle screen keyword for height specifically
        if c.Value.Kind == tailwind.ValueNamed && c.Value.Content == "screen" {
            value = "100vh"
        }
        return []tailwind.Declaration{
            {Property: "height", Value: value},
        }
    })

    // min-h-* (min-height)
    r.RegisterFunctional("min-h", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSizingMinMax(c.Value, theme, "height")
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "min-height", Value: value},
        }
    })

    // max-h-* (max-height)
    r.RegisterFunctional("max-h", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSizingMinMax(c.Value, theme, "height")
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "max-height", Value: value},
        }
    })
```

### 3.2 Height Reference Table

| Class | CSS |
|-------|-----|
| `h-0` | `height: 0px;` |
| `h-4` | `height: 1rem;` |
| `h-64` | `height: 16rem;` |
| `h-auto` | `height: auto;` |
| `h-1/2` | `height: 50%;` |
| `h-full` | `height: 100%;` |
| `h-screen` | `height: 100vh;` |
| `h-svh` | `height: 100svh;` |
| `h-lvh` | `height: 100lvh;` |
| `h-dvh` | `height: 100dvh;` |
| `h-min` | `height: min-content;` |
| `h-max` | `height: max-content;` |
| `h-fit` | `height: fit-content;` |
| `h-[300px]` | `height: 300px;` |

---

## 4. Size Utility (Width + Height)

The `size-*` utility sets both width and height simultaneously.

```go
    // size-* (width and height)
    r.RegisterFunctional("size", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSizing(c.Value, theme)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "width", Value: value},
            {Property: "height", Value: value},
        }
    })
```

| Class | CSS |
|-------|-----|
| `size-4` | `width: 1rem; height: 1rem;` |
| `size-full` | `width: 100%; height: 100%;` |
| `size-[50px]` | `width: 50px; height: 50px;` |

---

## 5. Min/Max Sizing Helper

```go
// resolveSizingMinMax handles min-w, max-w, min-h, max-h with additional keywords.
func resolveSizingMinMax(value *tailwind.Value, theme *tailwind.Theme, dimension string) (string, bool) {
    if value == nil {
        return "", false
    }

    switch value.Kind {
    case tailwind.ValueArbitrary:
        return value.Content, true

    case tailwind.ValueNamed:
        content := value.Content

        // Min/max specific keywords
        switch content {
        case "none":
            return "none", true
        case "full":
            return "100%", true
        case "min":
            return "min-content", true
        case "max":
            return "max-content", true
        case "fit":
            return "fit-content", true
        case "prose":
            return "65ch", true // Tailwind's prose width
        case "0":
            return "0px", true
        }

        // Screen variants
        if strings.HasPrefix(content, "screen") || content == "screen" {
            if dimension == "width" {
                switch content {
                case "screen":
                    return "100vw", true
                }
            } else { // height
                switch content {
                case "screen":
                    return "100vh", true
                }
            }
        }

        // Container breakpoints for max-w
        // max-w-sm, max-w-md, max-w-lg, etc.
        if bp, ok := theme.Resolve(content, "--breakpoint"); ok {
            return bp, true
        }

        // Check spacing scale
        if v, ok := theme.Resolve(content, "--spacing"); ok {
            return v, true
        }

        // Check fractions
        if value.Fraction != "" {
            return fractionToPercent(value.Fraction)
        }
    }

    return "", false
}
```

### 5.1 Max-Width Breakpoint Values

| Class | CSS |
|-------|-----|
| `max-w-none` | `max-width: none;` |
| `max-w-0` | `max-width: 0px;` |
| `max-w-xs` | `max-width: 20rem;` (320px) |
| `max-w-sm` | `max-width: 24rem;` (384px) |
| `max-w-md` | `max-width: 28rem;` (448px) |
| `max-w-lg` | `max-width: 32rem;` (512px) |
| `max-w-xl` | `max-width: 36rem;` (576px) |
| `max-w-2xl` | `max-width: 42rem;` (672px) |
| `max-w-3xl` | `max-width: 48rem;` (768px) |
| `max-w-4xl` | `max-width: 56rem;` (896px) |
| `max-w-5xl` | `max-width: 64rem;` (1024px) |
| `max-w-6xl` | `max-width: 72rem;` (1152px) |
| `max-w-7xl` | `max-width: 80rem;` (1280px) |
| `max-w-full` | `max-width: 100%;` |
| `max-w-min` | `max-width: min-content;` |
| `max-w-max` | `max-width: max-content;` |
| `max-w-fit` | `max-width: fit-content;` |
| `max-w-prose` | `max-width: 65ch;` |
| `max-w-screen-sm` | `max-width: 640px;` |
| `max-w-screen-md` | `max-width: 768px;` |
| `max-w-screen-lg` | `max-width: 1024px;` |
| `max-w-screen-xl` | `max-width: 1280px;` |
| `max-w-screen-2xl` | `max-width: 1536px;` |

---

## 6. Aspect Ratio

```go
    // aspect-* (aspect ratio)
    r.RegisterFunctional("aspect", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        var value string
        switch c.Value.Kind {
        case tailwind.ValueArbitrary:
            value = c.Value.Content
        case tailwind.ValueNamed:
            switch c.Value.Content {
            case "auto":
                value = "auto"
            case "square":
                value = "1 / 1"
            case "video":
                value = "16 / 9"
            default:
                // Try to parse as ratio like "4/3"
                if c.Value.Fraction != "" {
                    parts := strings.Split(c.Value.Fraction, "/")
                    if len(parts) == 2 {
                        value = parts[0] + " / " + parts[1]
                    }
                }
            }
        }

        if value == "" {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "aspect-ratio", Value: value},
        }
    })
}
```

| Class | CSS |
|-------|-----|
| `aspect-auto` | `aspect-ratio: auto;` |
| `aspect-square` | `aspect-ratio: 1 / 1;` |
| `aspect-video` | `aspect-ratio: 16 / 9;` |
| `aspect-4/3` | `aspect-ratio: 4 / 3;` |
| `aspect-[4/3]` | `aspect-ratio: 4/3;` |

---

## 7. Theme Values for Sizing

Add these to the theme's loadDefaults:

```go
func (t *Theme) loadMaxWidthScale() {
    maxWidths := map[string]string{
        "xs":   "20rem",   // 320px
        "sm":   "24rem",   // 384px
        "md":   "28rem",   // 448px
        "lg":   "32rem",   // 512px
        "xl":   "36rem",   // 576px
        "2xl":  "42rem",   // 672px
        "3xl":  "48rem",   // 768px
        "4xl":  "56rem",   // 896px
        "5xl":  "64rem",   // 1024px
        "6xl":  "72rem",   // 1152px
        "7xl":  "80rem",   // 1280px
    }

    for k, v := range maxWidths {
        t.Set("--max-width", k, v)
    }
}
```

---

## 8. Complete Fraction Values

The default fraction scale:

| Fraction | Percentage |
|----------|------------|
| `1/2` | `50%` |
| `1/3` | `33.333333%` |
| `2/3` | `66.666667%` |
| `1/4` | `25%` |
| `2/4` | `50%` |
| `3/4` | `75%` |
| `1/5` | `20%` |
| `2/5` | `40%` |
| `3/5` | `60%` |
| `4/5` | `80%` |
| `1/6` | `16.666667%` |
| `2/6` | `33.333333%` |
| `3/6` | `50%` |
| `4/6` | `66.666667%` |
| `5/6` | `83.333333%` |
| `1/12` | `8.333333%` |
| `2/12` | `16.666667%` |
| `3/12` | `25%` |
| `4/12` | `33.333333%` |
| `5/12` | `41.666667%` |
| `6/12` | `50%` |
| `7/12` | `58.333333%` |
| `8/12` | `66.666667%` |
| `9/12` | `75%` |
| `10/12` | `83.333333%` |
| `11/12` | `91.666667%` |

---

## 9. Testing

```go
// utilities/sizing_test.go

func TestWidthUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        expected string
    }{
        {"w-4", "1rem"},
        {"w-full", "100%"},
        {"w-1/2", "50%"},
        {"w-2/3", "66.666667%"},
        {"w-screen", "100vw"},
        {"w-auto", "auto"},
        {"w-min", "min-content"},
        {"w-max", "max-content"},
        {"w-fit", "fit-content"},
        {"w-[300px]", "300px"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            c, err := tailwind.ParseCandidate(tt.class, registry)
            assert.NoError(t, err)

            util, ok := registry.Get("w", UtilityFunctional)
            assert.True(t, ok)

            decls := util.Compile(c, theme)
            assert.Len(t, decls, 1)
            assert.Equal(t, "width", decls[0].Property)
            assert.Equal(t, tt.expected, decls[0].Value)
        })
    }
}

func TestHeightUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        expected string
    }{
        {"h-4", "1rem"},
        {"h-full", "100%"},
        {"h-screen", "100vh"},
        {"h-[50vh]", "50vh"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            c, err := tailwind.ParseCandidate(tt.class, registry)
            assert.NoError(t, err)

            util, ok := registry.Get("h", UtilityFunctional)
            assert.True(t, ok)

            decls := util.Compile(c, theme)
            assert.Len(t, decls, 1)
            assert.Equal(t, "height", decls[0].Property)
            assert.Equal(t, tt.expected, decls[0].Value)
        })
    }
}

func TestSizeUtility(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    c, err := tailwind.ParseCandidate("size-4", registry)
    assert.NoError(t, err)

    util, ok := registry.Get("size", UtilityFunctional)
    assert.True(t, ok)

    decls := util.Compile(c, theme)
    assert.Len(t, decls, 2)
    assert.Equal(t, "width", decls[0].Property)
    assert.Equal(t, "1rem", decls[0].Value)
    assert.Equal(t, "height", decls[1].Property)
    assert.Equal(t, "1rem", decls[1].Value)
}

func TestFractionToPercent(t *testing.T) {
    tests := []struct {
        fraction string
        expected string
    }{
        {"1/2", "50%"},
        {"1/3", "33.333333%"},
        {"2/3", "66.666667%"},
        {"1/4", "25%"},
        {"3/4", "75%"},
        {"1/12", "8.333333%"},
    }

    for _, tt := range tests {
        t.Run(tt.fraction, func(t *testing.T) {
            result, ok := fractionToPercent(tt.fraction)
            assert.True(t, ok)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

---

## 10. Files to Create

| File | Purpose | Lines (est.) |
|------|---------|--------------|
| `utilities/sizing.go` | All sizing utilities | 300 |
| `utilities/sizing_test.go` | Sizing tests | 200 |

---

## 11. Completion Criteria

Phase 4 is complete when:

1. ✅ Width utilities work (`w-*`, `min-w-*`, `max-w-*`)
2. ✅ Height utilities work (`h-*`, `min-h-*`, `max-h-*`)
3. ✅ Size utility works (`size-*`)
4. ✅ Fractions work (`w-1/2`, `h-2/3`)
5. ✅ Keywords work (`w-full`, `w-screen`, `w-auto`, `w-min`, `w-max`, `w-fit`)
6. ✅ Max-width scale works (`max-w-sm`, `max-w-xl`, `max-w-prose`)
7. ✅ Screen variants work (`max-w-screen-md`)
8. ✅ Aspect ratio works (`aspect-video`, `aspect-square`, `aspect-4/3`)
9. ✅ Arbitrary values work (`w-[300px]`, `h-[50vh]`)
10. ✅ All tests pass

---

*Last Updated: 2024-12-12*
