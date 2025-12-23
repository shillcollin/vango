# Phase 5: Typography Utilities

## Overview

Phase 5 implements typography utilities—font size, weight, family, line height, letter spacing, and text color. Note: Text colors are technically part of Phase 6 (Colors), but functional typography utilities are covered here.

**Prerequisites:** Phase 1 (Core Infrastructure)

**Files to create/modify:**
- `utilities/typography.go` - Typography utility implementations

---

## 1. Font Size: `text-*`

Font size utilities set the font size and optionally line height.

**Reference:** `/tailwind/packages/tailwindcss/src/utilities.ts` (search for "font-size")

```go
// utilities/typography.go

package utilities

import "github.com/vango-dev/vgo-tailwind"

func (r *Registry) registerTypographyUtilities() {
    r.registerFontSizeUtilities()
    r.registerFontWeightUtilities()
    r.registerFontFamilyUtilities()
    r.registerLineHeightUtilities()
    r.registerLetterSpacingUtilities()
    r.registerTextIndentUtilities()
}

func (r *Registry) registerFontSizeUtilities() {
    // text-* for font size (conflicts with text-* for color, handled by trying font-size first)
    r.RegisterFunctional("text", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        // Try to resolve as font size first
        if c.Value.Kind == tailwind.ValueArbitrary {
            return []tailwind.Declaration{
                {Property: "font-size", Value: c.Value.Content},
            }
        }

        // Named font sizes
        fontSize, ok := theme.Resolve(c.Value.Content, "--font-size")
        if !ok {
            // Not a font size, might be a color (handled elsewhere)
            return nil
        }

        // Font sizes in Tailwind often have associated line heights
        lineHeight, _ := theme.Resolve(c.Value.Content, "--font-size-line-height")

        decls := []tailwind.Declaration{
            {Property: "font-size", Value: fontSize},
        }

        if lineHeight != "" {
            decls = append(decls, tailwind.Declaration{
                Property: "line-height", Value: lineHeight,
            })
        }

        return decls
    })
}
```

### 1.1 Font Size Scale

| Class | Font Size | Line Height |
|-------|-----------|-------------|
| `text-xs` | `0.75rem` (12px) | `1rem` |
| `text-sm` | `0.875rem` (14px) | `1.25rem` |
| `text-base` | `1rem` (16px) | `1.5rem` |
| `text-lg` | `1.125rem` (18px) | `1.75rem` |
| `text-xl` | `1.25rem` (20px) | `1.75rem` |
| `text-2xl` | `1.5rem` (24px) | `2rem` |
| `text-3xl` | `1.875rem` (30px) | `2.25rem` |
| `text-4xl` | `2.25rem` (36px) | `2.5rem` |
| `text-5xl` | `3rem` (48px) | `1` |
| `text-6xl` | `3.75rem` (60px) | `1` |
| `text-7xl` | `4.5rem` (72px) | `1` |
| `text-8xl` | `6rem` (96px) | `1` |
| `text-9xl` | `8rem` (128px) | `1` |
| `text-[17px]` | `17px` | (none) |

### 1.2 Theme Font Size Values

```go
func (t *Theme) loadFontSizes() {
    // Each font size has associated line height
    fontSizes := map[string]struct {
        size       string
        lineHeight string
    }{
        "xs":   {"0.75rem", "1rem"},
        "sm":   {"0.875rem", "1.25rem"},
        "base": {"1rem", "1.5rem"},
        "lg":   {"1.125rem", "1.75rem"},
        "xl":   {"1.25rem", "1.75rem"},
        "2xl":  {"1.5rem", "2rem"},
        "3xl":  {"1.875rem", "2.25rem"},
        "4xl":  {"2.25rem", "2.5rem"},
        "5xl":  {"3rem", "1"},
        "6xl":  {"3.75rem", "1"},
        "7xl":  {"4.5rem", "1"},
        "8xl":  {"6rem", "1"},
        "9xl":  {"8rem", "1"},
    }

    for k, v := range fontSizes {
        t.Set("--font-size", k, v.size)
        t.Set("--font-size-line-height", k, v.lineHeight)
    }
}
```

---

## 2. Font Weight: `font-*`

```go
func (r *Registry) registerFontWeightUtilities() {
    r.RegisterFunctional("font", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        switch c.Value.Kind {
        case tailwind.ValueArbitrary:
            return []tailwind.Declaration{
                {Property: "font-weight", Value: c.Value.Content},
            }

        case tailwind.ValueNamed:
            // First try font-weight
            if weight, ok := theme.Resolve(c.Value.Content, "--font-weight"); ok {
                return []tailwind.Declaration{
                    {Property: "font-weight", Value: weight},
                }
            }

            // Then try font-family
            if family, ok := theme.Resolve(c.Value.Content, "--font-family"); ok {
                return []tailwind.Declaration{
                    {Property: "font-family", Value: family},
                }
            }
        }

        return nil
    })
}
```

### 2.1 Font Weight Scale

| Class | CSS |
|-------|-----|
| `font-thin` | `font-weight: 100;` |
| `font-extralight` | `font-weight: 200;` |
| `font-light` | `font-weight: 300;` |
| `font-normal` | `font-weight: 400;` |
| `font-medium` | `font-weight: 500;` |
| `font-semibold` | `font-weight: 600;` |
| `font-bold` | `font-weight: 700;` |
| `font-extrabold` | `font-weight: 800;` |
| `font-black` | `font-weight: 900;` |
| `font-[550]` | `font-weight: 550;` |

---

## 3. Font Family: `font-*`

Font family uses the same `font-*` utility but resolves from a different theme namespace.

### 3.1 Default Font Families

| Class | CSS |
|-------|-----|
| `font-sans` | `font-family: ui-sans-serif, system-ui, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji";` |
| `font-serif` | `font-family: ui-serif, Georgia, Cambria, "Times New Roman", Times, serif;` |
| `font-mono` | `font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;` |

### 3.2 Theme Font Family Values

```go
func (t *Theme) loadFontFamilies() {
    t.Set("--font-family", "sans", `ui-sans-serif, system-ui, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji"`)
    t.Set("--font-family", "serif", `ui-serif, Georgia, Cambria, "Times New Roman", Times, serif`)
    t.Set("--font-family", "mono", `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace`)
}
```

---

## 4. Line Height: `leading-*`

```go
func (r *Registry) registerLineHeightUtilities() {
    r.RegisterFunctional("leading", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        var value string
        switch c.Value.Kind {
        case tailwind.ValueArbitrary:
            value = c.Value.Content
        case tailwind.ValueNamed:
            var ok bool
            value, ok = theme.Resolve(c.Value.Content, "--leading")
            if !ok {
                // Try as a number (leading-5, leading-6, etc.)
                // These map to spacing scale in rem
                if v, ok := theme.Resolve(c.Value.Content, "--spacing"); ok {
                    value = v
                }
            }
        }

        if value == "" {
            return nil
        }

        return []tailwind.Declaration{
            {Property: "line-height", Value: value},
        }
    })
}
```

### 4.1 Line Height Scale

| Class | CSS |
|-------|-----|
| `leading-none` | `line-height: 1;` |
| `leading-tight` | `line-height: 1.25;` |
| `leading-snug` | `line-height: 1.375;` |
| `leading-normal` | `line-height: 1.5;` |
| `leading-relaxed` | `line-height: 1.625;` |
| `leading-loose` | `line-height: 2;` |
| `leading-3` | `line-height: 0.75rem;` |
| `leading-4` | `line-height: 1rem;` |
| `leading-5` | `line-height: 1.25rem;` |
| `leading-6` | `line-height: 1.5rem;` |
| `leading-7` | `line-height: 1.75rem;` |
| `leading-8` | `line-height: 2rem;` |
| `leading-9` | `line-height: 2.25rem;` |
| `leading-10` | `line-height: 2.5rem;` |
| `leading-[3rem]` | `line-height: 3rem;` |

---

## 5. Letter Spacing: `tracking-*`

```go
func (r *Registry) registerLetterSpacingUtilities() {
    r.RegisterFunctional("tracking", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        var value string
        switch c.Value.Kind {
        case tailwind.ValueArbitrary:
            value = c.Value.Content
        case tailwind.ValueNamed:
            var ok bool
            value, ok = theme.Resolve(c.Value.Content, "--tracking")
            if !ok {
                return nil
            }
        }

        return []tailwind.Declaration{
            {Property: "letter-spacing", Value: value},
        }
    })
}
```

### 5.1 Letter Spacing Scale

| Class | CSS |
|-------|-----|
| `tracking-tighter` | `letter-spacing: -0.05em;` |
| `tracking-tight` | `letter-spacing: -0.025em;` |
| `tracking-normal` | `letter-spacing: 0em;` |
| `tracking-wide` | `letter-spacing: 0.025em;` |
| `tracking-wider` | `letter-spacing: 0.05em;` |
| `tracking-widest` | `letter-spacing: 0.1em;` |
| `tracking-[.25em]` | `letter-spacing: 0.25em;` |

---

## 6. Text Indent: `indent-*`

```go
func (r *Registry) registerTextIndentUtilities() {
    r.RegisterFunctional("indent", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        var value string
        switch c.Value.Kind {
        case tailwind.ValueArbitrary:
            value = c.Value.Content
        case tailwind.ValueNamed:
            // Use spacing scale
            var ok bool
            value, ok = theme.Resolve(c.Value.Content, "--spacing")
            if !ok {
                return nil
            }
        }

        // Handle negative
        if c.Negative && value != "" {
            value = "-" + value
        }

        return []tailwind.Declaration{
            {Property: "text-indent", Value: value},
        }
    })
}
```

| Class | CSS |
|-------|-----|
| `indent-0` | `text-indent: 0px;` |
| `indent-4` | `text-indent: 1rem;` |
| `indent-8` | `text-indent: 2rem;` |
| `-indent-4` | `text-indent: -1rem;` |
| `indent-[50px]` | `text-indent: 50px;` |

---

## 7. Text Decoration Thickness: `decoration-*`

```go
func (r *Registry) registerDecorationUtilities() {
    // decoration-* for thickness (color is in Phase 6)
    r.RegisterFunctional("decoration", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        // Check if this is a thickness value
        switch c.Value.Content {
        case "auto":
            return []tailwind.Declaration{
                {Property: "text-decoration-thickness", Value: "auto"},
            }
        case "from-font":
            return []tailwind.Declaration{
                {Property: "text-decoration-thickness", Value: "from-font"},
            }
        case "0", "1", "2", "4", "8":
            return []tailwind.Declaration{
                {Property: "text-decoration-thickness", Value: c.Value.Content + "px"},
            }
        }

        if c.Value.Kind == tailwind.ValueArbitrary {
            return []tailwind.Declaration{
                {Property: "text-decoration-thickness", Value: c.Value.Content},
            }
        }

        // Otherwise might be a color (handled in Phase 6)
        return nil
    })
}
```

| Class | CSS |
|-------|-----|
| `decoration-auto` | `text-decoration-thickness: auto;` |
| `decoration-from-font` | `text-decoration-thickness: from-font;` |
| `decoration-0` | `text-decoration-thickness: 0px;` |
| `decoration-1` | `text-decoration-thickness: 1px;` |
| `decoration-2` | `text-decoration-thickness: 2px;` |
| `decoration-4` | `text-decoration-thickness: 4px;` |
| `decoration-8` | `text-decoration-thickness: 8px;` |
| `decoration-[3px]` | `text-decoration-thickness: 3px;` |

---

## 8. Underline Offset: `underline-offset-*`

```go
func (r *Registry) registerUnderlineOffsetUtilities() {
    r.RegisterFunctional("underline-offset", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
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
            case "0", "1", "2", "4", "8":
                value = c.Value.Content + "px"
            default:
                return nil
            }
        }

        return []tailwind.Declaration{
            {Property: "text-underline-offset", Value: value},
        }
    })
}
```

| Class | CSS |
|-------|-----|
| `underline-offset-auto` | `text-underline-offset: auto;` |
| `underline-offset-0` | `text-underline-offset: 0px;` |
| `underline-offset-1` | `text-underline-offset: 1px;` |
| `underline-offset-2` | `text-underline-offset: 2px;` |
| `underline-offset-4` | `text-underline-offset: 4px;` |
| `underline-offset-8` | `text-underline-offset: 8px;` |
| `underline-offset-[3px]` | `text-underline-offset: 3px;` |

---

## 9. List Style Type (Functional)

While basic list styles are static, custom list markers are functional:

```go
func (r *Registry) registerListStyleTypeUtility() {
    r.RegisterFunctional("list", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        if c.Value.Kind == tailwind.ValueArbitrary {
            return []tailwind.Declaration{
                {Property: "list-style-type", Value: c.Value.Content},
            }
        }

        return nil // Named values handled as static
    })
}
```

| Class | CSS |
|-------|-----|
| `list-[square]` | `list-style-type: square;` |
| `list-['→']` | `list-style-type: '→';` |

---

## 10. Content: `content-*`

For `::before` and `::after` pseudo-elements:

```go
func (r *Registry) registerContentUtility() {
    // content-none is static, but arbitrary content is functional
    r.RegisterFunctional("content", func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration {
        if c.Value == nil {
            return nil
        }

        if c.Value.Kind == tailwind.ValueArbitrary {
            // Wrap in quotes if not already a CSS function
            content := c.Value.Content
            return []tailwind.Declaration{
                {Property: "content", Value: content},
            }
        }

        return nil
    })

    // Static content-none
    registerStatic(r, "content-none", decl("content", "none"))
}
```

| Class | CSS |
|-------|-----|
| `content-none` | `content: none;` |
| `content-['Hello']` | `content: 'Hello';` |
| `content-[attr(data-label)]` | `content: attr(data-label);` |

---

## 11. Testing

```go
// utilities/typography_test.go

func TestFontSizeUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class        string
        expectedSize string
        hasLineHeight bool
    }{
        {"text-xs", "0.75rem", true},
        {"text-base", "1rem", true},
        {"text-2xl", "1.5rem", true},
        {"text-[17px]", "17px", false},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            c, err := tailwind.ParseCandidate(tt.class, registry)
            assert.NoError(t, err)

            util, ok := registry.Get("text", UtilityFunctional)
            assert.True(t, ok)

            decls := util.Compile(c, theme)
            assert.NotEmpty(t, decls)
            assert.Equal(t, "font-size", decls[0].Property)
            assert.Equal(t, tt.expectedSize, decls[0].Value)

            if tt.hasLineHeight {
                assert.Len(t, decls, 2)
                assert.Equal(t, "line-height", decls[1].Property)
            }
        })
    }
}

func TestFontWeightUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        expected string
    }{
        {"font-thin", "100"},
        {"font-normal", "400"},
        {"font-bold", "700"},
        {"font-black", "900"},
        {"font-[550]", "550"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            c, err := tailwind.ParseCandidate(tt.class, registry)
            assert.NoError(t, err)

            util, ok := registry.Get("font", UtilityFunctional)
            assert.True(t, ok)

            decls := util.Compile(c, theme)
            assert.Len(t, decls, 1)
            assert.Equal(t, "font-weight", decls[0].Property)
            assert.Equal(t, tt.expected, decls[0].Value)
        })
    }
}

func TestLineHeightUtilities(t *testing.T) {
    registry := NewRegistry()
    theme := tailwind.NewDefaultTheme()

    tests := []struct {
        class    string
        expected string
    }{
        {"leading-none", "1"},
        {"leading-normal", "1.5"},
        {"leading-loose", "2"},
        {"leading-6", "1.5rem"},
        {"leading-[3rem]", "3rem"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            c, err := tailwind.ParseCandidate(tt.class, registry)
            assert.NoError(t, err)

            util, ok := registry.Get("leading", UtilityFunctional)
            assert.True(t, ok)

            decls := util.Compile(c, theme)
            assert.Len(t, decls, 1)
            assert.Equal(t, "line-height", decls[0].Property)
            assert.Equal(t, tt.expected, decls[0].Value)
        })
    }
}
```

---

## 12. Files to Create

| File | Purpose | Lines (est.) |
|------|---------|--------------|
| `utilities/typography.go` | All typography utilities | 350 |
| `utilities/typography_test.go` | Typography tests | 250 |

---

## 13. Completion Criteria

Phase 5 is complete when:

1. ✅ Font size utilities work (`text-sm`, `text-lg`, `text-[17px]`)
2. ✅ Font size includes line height when appropriate
3. ✅ Font weight utilities work (`font-bold`, `font-[550]`)
4. ✅ Font family utilities work (`font-sans`, `font-serif`, `font-mono`)
5. ✅ Line height utilities work (`leading-tight`, `leading-6`, `leading-[3rem]`)
6. ✅ Letter spacing utilities work (`tracking-wide`, `tracking-[.25em]`)
7. ✅ Text indent utilities work (`indent-4`, `-indent-4`)
8. ✅ Decoration thickness works (`decoration-2`)
9. ✅ Underline offset works (`underline-offset-4`)
10. ✅ Arbitrary values work for all utilities
11. ✅ All tests pass

---

*Last Updated: 2024-12-12*
