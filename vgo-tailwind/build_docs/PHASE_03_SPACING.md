# Phase 3: Spacing Utilities

## Overview

Phase 3 implements all spacing utilities—padding, margin, gap, and space-between. These are among the most commonly used Tailwind utilities and demonstrate the functional utility pattern.

**Prerequisites:** Phase 1 (Core Infrastructure)

**Files to create/modify:**
- `utilities/spacing.go` - All spacing utility implementations

---

## 1. Functional Utility Pattern

Functional utilities take a value and generate CSS based on that value:

```go
// utilities/spacing.go

package utilities

import (
    "fmt"
    "strings"

    "github.com/vango-dev/vgo-tailwind"
)

// registerSpacingUtilities registers all spacing utilities.
func (r *Registry) registerSpacingUtilities() {
    r.registerPaddingUtilities()
    r.registerMarginUtilities()
    r.registerGapUtilities()
    r.registerSpaceBetweenUtilities()
}
```

---

## 2. Value Resolution

Spacing utilities resolve values from the theme:

```go
// resolveSpacing resolves a spacing value from theme or arbitrary.
func resolveSpacing(value *tailwind.Value, theme *tailwind.Theme, negative bool) (string, bool) {
    if value == nil {
        return "", false
    }

    var resolved string

    switch value.Kind {
    case tailwind.ValueArbitrary:
        // Arbitrary value: p-[17px]
        resolved = value.Content

    case tailwind.ValueNamed:
        // Named value: p-4
        var ok bool
        resolved, ok = theme.Resolve(value.Content, "--spacing")
        if !ok {
            return "", false
        }
    }

    // Apply negative
    if negative && resolved != "" && resolved != "0" && resolved != "0px" {
        // Prepend minus sign, handling calc() if needed
        if strings.HasPrefix(resolved, "calc(") {
            resolved = "calc(-1 * " + resolved[5:]
        } else {
            resolved = "-" + resolved
        }
    }

    return resolved, true
}
```

---

## 3. Padding Utilities

### 3.1 All Sides: `p-*`

```go
func (r *Registry) registerPaddingUtilities() {
    // p-* (all sides)
    r.RegisterFunctional("p", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "padding", Value: value},
        }
    })
```

### 3.2 Horizontal/Vertical: `px-*`, `py-*`

```go
    // px-* (horizontal: left and right)
    r.RegisterFunctional("px", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "padding-left", Value: value},
            {Property: "padding-right", Value: value},
        }
    })

    // py-* (vertical: top and bottom)
    r.RegisterFunctional("py", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "padding-top", Value: value},
            {Property: "padding-bottom", Value: value},
        }
    })
```

### 3.3 Inline/Block: `ps-*`, `pe-*`

```go
    // ps-* (padding-inline-start)
    r.RegisterFunctional("ps", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "padding-inline-start", Value: value},
        }
    })

    // pe-* (padding-inline-end)
    r.RegisterFunctional("pe", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "padding-inline-end", Value: value},
        }
    })
```

### 3.4 Individual Sides: `pt-*`, `pr-*`, `pb-*`, `pl-*`

```go
    // pt-* (top)
    r.RegisterFunctional("pt", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "padding-top", Value: value},
        }
    })

    // pr-* (right)
    r.RegisterFunctional("pr", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "padding-right", Value: value},
        }
    })

    // pb-* (bottom)
    r.RegisterFunctional("pb", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "padding-bottom", Value: value},
        }
    })

    // pl-* (left)
    r.RegisterFunctional("pl", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "padding-left", Value: value},
        }
    })
}
```

### 3.5 Complete Padding Reference

| Class | CSS |
|-------|-----|
| `p-0` | `padding: 0px;` |
| `p-px` | `padding: 1px;` |
| `p-0.5` | `padding: 0.125rem;` |
| `p-1` | `padding: 0.25rem;` |
| `p-2` | `padding: 0.5rem;` |
| `p-4` | `padding: 1rem;` |
| `p-8` | `padding: 2rem;` |
| `p-[17px]` | `padding: 17px;` |
| `px-4` | `padding-left: 1rem; padding-right: 1rem;` |
| `py-4` | `padding-top: 1rem; padding-bottom: 1rem;` |
| `pt-4` | `padding-top: 1rem;` |
| `pr-4` | `padding-right: 1rem;` |
| `pb-4` | `padding-bottom: 1rem;` |
| `pl-4` | `padding-left: 1rem;` |
| `ps-4` | `padding-inline-start: 1rem;` |
| `pe-4` | `padding-inline-end: 1rem;` |

---

## 4. Margin Utilities

Margin utilities follow the same pattern as padding, but support negative values.

### 4.1 Implementation

```go
func (r *Registry) registerMarginUtilities() {
    // m-* (all sides)
    r.RegisterFunctional("m", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacingOrAuto(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "margin", Value: value},
        }
    })

    // mx-* (horizontal)
    r.RegisterFunctional("mx", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacingOrAuto(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "margin-left", Value: value},
            {Property: "margin-right", Value: value},
        }
    })

    // my-* (vertical)
    r.RegisterFunctional("my", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacingOrAuto(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "margin-top", Value: value},
            {Property: "margin-bottom", Value: value},
        }
    })

    // ms-* (margin-inline-start)
    r.RegisterFunctional("ms", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacingOrAuto(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "margin-inline-start", Value: value},
        }
    })

    // me-* (margin-inline-end)
    r.RegisterFunctional("me", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacingOrAuto(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "margin-inline-end", Value: value},
        }
    })

    // mt-* (top)
    r.RegisterFunctional("mt", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacingOrAuto(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "margin-top", Value: value},
        }
    })

    // mr-* (right)
    r.RegisterFunctional("mr", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacingOrAuto(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "margin-right", Value: value},
        }
    })

    // mb-* (bottom)
    r.RegisterFunctional("mb", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacingOrAuto(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "margin-bottom", Value: value},
        }
    })

    // ml-* (left)
    r.RegisterFunctional("ml", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacingOrAuto(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "margin-left", Value: value},
        }
    })
}

// resolveSpacingOrAuto handles "auto" in addition to spacing values.
func resolveSpacingOrAuto(value *tailwind.Value, theme *tailwind.Theme, negative bool) (string, bool) {
    if value == nil {
        return "", false
    }

    // Special case: auto
    if value.Kind == tailwind.ValueNamed && value.Content == "auto" {
        return "auto", true
    }

    return resolveSpacing(value, theme, negative)
}
```

### 4.2 Complete Margin Reference

| Class | CSS |
|-------|-----|
| `m-0` | `margin: 0px;` |
| `m-4` | `margin: 1rem;` |
| `m-auto` | `margin: auto;` |
| `-m-4` | `margin: -1rem;` |
| `mx-auto` | `margin-left: auto; margin-right: auto;` |
| `my-4` | `margin-top: 1rem; margin-bottom: 1rem;` |
| `mt-4` | `margin-top: 1rem;` |
| `-mt-4` | `margin-top: -1rem;` |
| `mr-4` | `margin-right: 1rem;` |
| `mb-4` | `margin-bottom: 1rem;` |
| `ml-4` | `margin-left: 1rem;` |
| `ms-4` | `margin-inline-start: 1rem;` |
| `me-4` | `margin-inline-end: 1rem;` |
| `m-[17px]` | `margin: 17px;` |
| `-m-[17px]` | `margin: -17px;` |

---

## 5. Gap Utilities

Gap utilities control spacing between grid/flex children.

```go
func (r *Registry) registerGapUtilities() {
    // gap-* (both axes)
    r.RegisterFunctional("gap", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "gap", Value: value},
        }
    })

    // gap-x-* (column gap)
    r.RegisterFunctional("gap-x", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "column-gap", Value: value},
        }
    })

    // gap-y-* (row gap)
    r.RegisterFunctional("gap-y", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "row-gap", Value: value},
        }
    })
}
```

### 5.1 Complete Gap Reference

| Class | CSS |
|-------|-----|
| `gap-0` | `gap: 0px;` |
| `gap-4` | `gap: 1rem;` |
| `gap-x-4` | `column-gap: 1rem;` |
| `gap-y-4` | `row-gap: 1rem;` |
| `gap-[17px]` | `gap: 17px;` |

---

## 6. Space Between Utilities

Space-between utilities add margin to children (except the first).

**Reference:** `/tailwind/packages/tailwindcss/src/utilities.ts` (search for "space")

```go
func (r *Registry) registerSpaceBetweenUtilities() {
    // space-x-* (horizontal spacing between children)
    r.RegisterFunctional("space-x", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        // Note: This requires a special selector "> :not([hidden]) ~ :not([hidden])"
        // The variant system will handle this
        return []tailwind.Declaration{
            {Property: "margin-left", Value: value},
        }
    })

    // space-y-* (vertical spacing between children)
    r.RegisterFunctional("space-y", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        value, ok := resolveSpacing(c.Value, theme, c.Negative)
        if !ok {
            return nil
        }
        return []tailwind.Declaration{
            {Property: "margin-top", Value: value},
        }
    })

    // space-x-reverse (for RTL or reversed flex)
    registerStatic(r, "space-x-reverse", decl("--tw-space-x-reverse", "1"))
    registerStatic(r, "space-y-reverse", decl("--tw-space-y-reverse", "1"))
}
```

### 6.1 Space Between Selector Handling

Space-between utilities require special selector handling:

```go
// In the Engine's compileCandidateToRule method, handle space-* specially:

func (e *Engine) handleSpaceUtility(c *Candidate, decls []Declaration) *CachedRule {
    // Space utilities target "> :not([hidden]) ~ :not([hidden])"
    selector := "." + escapeSelector(c.Raw) + " > :not([hidden]) ~ :not([hidden])"

    return &CachedRule{
        Selector:     selector,
        Declarations: decls,
    }
}
```

### 6.2 Complete Space Reference

| Class | CSS (on child selector) |
|-------|-------------------------|
| `space-x-4` | `margin-left: 1rem;` |
| `space-y-4` | `margin-top: 1rem;` |
| `-space-x-4` | `margin-left: -1rem;` |
| `-space-y-4` | `margin-top: -1rem;` |
| `space-x-reverse` | `--tw-space-x-reverse: 1;` |
| `space-y-reverse` | `--tw-space-y-reverse: 1;` |

---

## 7. Complete Spacing Scale

The default Tailwind spacing scale (from theme):

| Value | Size |
|-------|------|
| `0` | `0px` |
| `px` | `1px` |
| `0.5` | `0.125rem` (2px) |
| `1` | `0.25rem` (4px) |
| `1.5` | `0.375rem` (6px) |
| `2` | `0.5rem` (8px) |
| `2.5` | `0.625rem` (10px) |
| `3` | `0.75rem` (12px) |
| `3.5` | `0.875rem` (14px) |
| `4` | `1rem` (16px) |
| `5` | `1.25rem` (20px) |
| `6` | `1.5rem` (24px) |
| `7` | `1.75rem` (28px) |
| `8` | `2rem` (32px) |
| `9` | `2.25rem` (36px) |
| `10` | `2.5rem` (40px) |
| `11` | `2.75rem` (44px) |
| `12` | `3rem` (48px) |
| `14` | `3.5rem` (56px) |
| `16` | `4rem` (64px) |
| `20` | `5rem` (80px) |
| `24` | `6rem` (96px) |
| `28` | `7rem` (112px) |
| `32` | `8rem` (128px) |
| `36` | `9rem` (144px) |
| `40` | `10rem` (160px) |
| `44` | `11rem` (176px) |
| `48` | `12rem` (192px) |
| `52` | `13rem` (208px) |
| `56` | `14rem` (224px) |
| `60` | `15rem` (240px) |
| `64` | `16rem` (256px) |
| `72` | `18rem` (288px) |
| `80` | `20rem` (320px) |
| `96` | `24rem` (384px) |

---

## 8. Arbitrary Value Support

Arbitrary values allow any CSS value:

```go
// Examples of arbitrary spacing:
// p-[17px]     → padding: 17px;
// m-[2rem]     → margin: 2rem;
// gap-[10%]    → gap: 10%;
// p-[clamp(1rem,5vw,3rem)] → padding: clamp(1rem,5vw,3rem);

// The resolveSpacing function handles arbitrary values by returning
// the content directly (already handled above).
```

---

## 9. Testing

```go
// utilities/spacing_test.go

package utilities

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/vango-dev/vgo-tailwind"
)

func TestPaddingUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        expected []tailwind.Declaration
    }{
        {
            class: "p-4",
            expected: []tailwind.Declaration{
                {Property: "padding", Value: "1rem"},
            },
        },
        {
            class: "px-4",
            expected: []tailwind.Declaration{
                {Property: "padding-left", Value: "1rem"},
                {Property: "padding-right", Value: "1rem"},
            },
        },
        {
            class: "pt-0",
            expected: []tailwind.Declaration{
                {Property: "padding-top", Value: "0px"},
            },
        },
        {
            class: "p-[17px]",
            expected: []tailwind.Declaration{
                {Property: "padding", Value: "17px"},
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            c, err := tailwind.ParseCandidate(tt.class, registry)
            assert.NoError(t, err)

            util, ok := registry.Get(c.Root, UtilityFunctional)
            assert.True(t, ok)

            decls := util.Compile(c, theme)
            assert.Equal(t, tt.expected, decls)
        })
    }
}

func TestMarginUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        expected []tailwind.Declaration
    }{
        {
            class: "m-4",
            expected: []tailwind.Declaration{
                {Property: "margin", Value: "1rem"},
            },
        },
        {
            class: "-m-4",
            expected: []tailwind.Declaration{
                {Property: "margin", Value: "-1rem"},
            },
        },
        {
            class: "mx-auto",
            expected: []tailwind.Declaration{
                {Property: "margin-left", Value: "auto"},
                {Property: "margin-right", Value: "auto"},
            },
        },
        {
            class: "-mt-[10px]",
            expected: []tailwind.Declaration{
                {Property: "margin-top", Value: "-10px"},
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            c, err := tailwind.ParseCandidate(tt.class, registry)
            assert.NoError(t, err)

            util, ok := registry.Get(c.Root, UtilityFunctional)
            assert.True(t, ok)

            decls := util.Compile(c, theme)
            assert.Equal(t, tt.expected, decls)
        })
    }
}

func TestGapUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        property string
        value    string
    }{
        {"gap-4", "gap", "1rem"},
        {"gap-x-4", "column-gap", "1rem"},
        {"gap-y-4", "row-gap", "1rem"},
        {"gap-0", "gap", "0px"},
        {"gap-[20px]", "gap", "20px"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            c, err := tailwind.ParseCandidate(tt.class, registry)
            assert.NoError(t, err)

            util, ok := registry.Get(c.Root, UtilityFunctional)
            assert.True(t, ok)

            decls := util.Compile(c, theme)
            assert.Len(t, decls, 1)
            assert.Equal(t, tt.property, decls[0].Property)
            assert.Equal(t, tt.value, decls[0].Value)
        })
    }
}
```

---

## 10. Integration Test

```go
func TestSpacingEndToEnd(t *testing.T) {
    engine := tailwind.New()

    css := engine.Generate([]string{
        "p-4",
        "m-2",
        "mx-auto",
        "-mt-4",
        "gap-4",
        "p-[17px]",
    })

    // Verify each class is in the output
    assert.Contains(t, css, ".p-4")
    assert.Contains(t, css, "padding: 1rem")

    assert.Contains(t, css, ".m-2")
    assert.Contains(t, css, "margin: 0.5rem")

    assert.Contains(t, css, ".mx-auto")
    assert.Contains(t, css, "margin-left: auto")
    assert.Contains(t, css, "margin-right: auto")

    assert.Contains(t, css, ".-mt-4")
    assert.Contains(t, css, "margin-top: -1rem")

    assert.Contains(t, css, ".gap-4")
    assert.Contains(t, css, "gap: 1rem")

    assert.Contains(t, css, ".p-\\[17px\\]")
    assert.Contains(t, css, "padding: 17px")
}
```

---

## 11. Files to Create

| File | Purpose | Lines (est.) |
|------|---------|--------------|
| `utilities/spacing.go` | All spacing utilities | 250 |
| `utilities/spacing_test.go` | Spacing tests | 200 |

---

## 12. Completion Criteria

Phase 3 is complete when:

1. ✅ All padding utilities work (`p-*`, `px-*`, `py-*`, `pt-*`, `pr-*`, `pb-*`, `pl-*`, `ps-*`, `pe-*`)
2. ✅ All margin utilities work (`m-*`, `mx-*`, `my-*`, `mt-*`, `mr-*`, `mb-*`, `ml-*`, `ms-*`, `me-*`)
3. ✅ Negative margins work (`-m-4`, `-mt-4`)
4. ✅ Auto margins work (`m-auto`, `mx-auto`)
5. ✅ Gap utilities work (`gap-*`, `gap-x-*`, `gap-y-*`)
6. ✅ Space-between utilities work (`space-x-*`, `space-y-*`)
7. ✅ Arbitrary values work (`p-[17px]`, `m-[2rem]`)
8. ✅ All spacing scale values resolve correctly
9. ✅ All tests pass

---

*Last Updated: 2024-12-12*
