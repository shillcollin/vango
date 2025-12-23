# Phase 8: Variants

## Overview

Phase 8 implements the variant system—the mechanism that applies conditions like `hover:`, `focus:`, `md:`, and `dark:` to utilities. This is one of the most complex parts of vgo-tailwind because variants modify selectors and wrap rules in at-rules.

**Prerequisites:** Phases 1-7 (Core Infrastructure + All Utilities)

**Files to create/modify:**
- `variant.go` - Variant system implementation

---

## 1. Variant Architecture

Variants modify how a utility is applied by:
1. **Selector modification**: `hover:` → `.hover\:bg-blue-500:hover`
2. **At-rule wrapping**: `md:` → `@media (min-width: 768px) { ... }`
3. **Both**: `md:hover:` → `@media (min-width: 768px) { .md\:hover\:...:hover { ... } }`

```go
// variant.go

package tailwind

// VariantDef defines how a variant modifies a rule.
type VariantDef struct {
    Kind         VariantDefKind
    Selector     string   // For selector-modifying variants
    AtRule       string   // For at-rule variants: "@media", "@supports"
    AtRuleParams string   // For at-rule variants: "(min-width: 768px)"
    Order        int      // For sorting (lower = earlier in output)
}

type VariantDefKind uint8

const (
    VariantDefSelector VariantDefKind = iota + 1 // Modifies selector
    VariantDefAtRule                              // Wraps in at-rule
    VariantDefBoth                                // Both
)

// ApplyVariant applies a variant to a cached rule.
func (e *Engine) applyVariant(rule *CachedRule, v Variant) *CachedRule {
    def, ok := e.getVariantDef(v)
    if !ok {
        return rule
    }

    newRule := &CachedRule{
        Selector:     rule.Selector,
        Declarations: rule.Declarations,
        AtRules:      append([]string{}, rule.AtRules...),
        Order:        rule.Order + def.Order,
    }

    // Apply selector modification
    if def.Kind == VariantDefSelector || def.Kind == VariantDefBoth {
        newRule.Selector = applySelectorVariant(rule.Selector, def.Selector)
    }

    // Apply at-rule wrapping
    if def.Kind == VariantDefAtRule || def.Kind == VariantDefBoth {
        atRule := def.AtRule
        if def.AtRuleParams != "" {
            atRule += " " + def.AtRuleParams
        }
        newRule.AtRules = append(newRule.AtRules, atRule)
    }

    return newRule
}

// applySelectorVariant modifies a selector with a variant pattern.
// & is replaced with the original selector.
func applySelectorVariant(selector, pattern string) string {
    return strings.Replace(pattern, "&", selector, -1)
}
```

---

## 2. Pseudo-Class Variants

Pseudo-class variants modify the selector by appending a pseudo-class.

```go
func (e *Engine) registerPseudoClassVariants() {
    pseudoClasses := map[string]string{
        // Interactive states
        "hover":         "&:hover",
        "focus":         "&:focus",
        "focus-within":  "&:focus-within",
        "focus-visible": "&:focus-visible",
        "active":        "&:active",
        "visited":       "&:visited",
        "target":        "&:target",

        // Form states
        "disabled":       "&:disabled",
        "enabled":        "&:enabled",
        "checked":        "&:checked",
        "indeterminate":  "&:indeterminate",
        "default":        "&:default",
        "required":       "&:required",
        "valid":          "&:valid",
        "invalid":        "&:invalid",
        "in-range":       "&:in-range",
        "out-of-range":   "&:out-of-range",
        "placeholder-shown": "&:placeholder-shown",
        "autofill":       "&:autofill",
        "read-only":      "&:read-only",

        // Structural
        "first":          "&:first-child",
        "last":           "&:last-child",
        "only":           "&:only-child",
        "odd":            "&:nth-child(odd)",
        "even":           "&:nth-child(even)",
        "first-of-type":  "&:first-of-type",
        "last-of-type":   "&:last-of-type",
        "only-of-type":   "&:only-of-type",
        "empty":          "&:empty",

        // Special
        "open":           "&[open]",  // for <details>
    }

    for name, pattern := range pseudoClasses {
        e.variants.Register(name, VariantDef{
            Kind:     VariantDefSelector,
            Selector: pattern,
            Order:    1000, // Base order for pseudo-classes
        })
    }
}
```

### 2.1 Pseudo-Class Reference

| Variant | Selector |
|---------|----------|
| `hover:` | `&:hover` |
| `focus:` | `&:focus` |
| `focus-within:` | `&:focus-within` |
| `focus-visible:` | `&:focus-visible` |
| `active:` | `&:active` |
| `visited:` | `&:visited` |
| `disabled:` | `&:disabled` |
| `checked:` | `&:checked` |
| `first:` | `&:first-child` |
| `last:` | `&:last-child` |
| `odd:` | `&:nth-child(odd)` |
| `even:` | `&:nth-child(even)` |
| `empty:` | `&:empty` |

---

## 3. Pseudo-Element Variants

```go
func (e *Engine) registerPseudoElementVariants() {
    pseudoElements := map[string]string{
        "before":      "&::before",
        "after":       "&::after",
        "first-line":  "&::first-line",
        "first-letter": "&::first-letter",
        "marker":      "&::marker",
        "selection":   "&::selection",
        "file":        "&::file-selector-button",
        "backdrop":    "&::backdrop",
        "placeholder": "&::placeholder",
    }

    for name, pattern := range pseudoElements {
        e.variants.Register(name, VariantDef{
            Kind:     VariantDefSelector,
            Selector: pattern,
            Order:    2000, // After pseudo-classes
        })
    }

    // before: and after: also need content: "" to work
    // This is handled specially in the compile step
}
```

### 3.1 Content for before/after

The `before:` and `after:` variants automatically add `content: ""` if no content is specified:

```go
func (e *Engine) handleBeforeAfter(rule *CachedRule, variantName string) *CachedRule {
    if variantName != "before" && variantName != "after" {
        return rule
    }

    // Check if content is already declared
    hasContent := false
    for _, decl := range rule.Declarations {
        if decl.Property == "content" {
            hasContent = true
            break
        }
    }

    if !hasContent {
        // Prepend content: ""
        rule.Declarations = append(
            []Declaration{{Property: "content", Value: `""`}},
            rule.Declarations...,
        )
    }

    return rule
}
```

---

## 4. Responsive Variants

Responsive variants wrap rules in media queries.

```go
func (e *Engine) registerResponsiveVariants() {
    breakpoints := map[string]string{
        "sm":  "640px",
        "md":  "768px",
        "lg":  "1024px",
        "xl":  "1280px",
        "2xl": "1536px",
    }

    for name, minWidth := range breakpoints {
        e.variants.Register(name, VariantDef{
            Kind:         VariantDefAtRule,
            AtRule:       "@media",
            AtRuleParams: fmt.Sprintf("(min-width: %s)", minWidth),
            Order:        100, // Responsive variants come early
        })
    }

    // max-* variants
    maxBreakpoints := map[string]string{
        "max-sm":  "639.9px",
        "max-md":  "767.9px",
        "max-lg":  "1023.9px",
        "max-xl":  "1279.9px",
        "max-2xl": "1535.9px",
    }

    for name, maxWidth := range maxBreakpoints {
        e.variants.Register(name, VariantDef{
            Kind:         VariantDefAtRule,
            AtRule:       "@media",
            AtRuleParams: fmt.Sprintf("(max-width: %s)", maxWidth),
            Order:        100,
        })
    }

    // min-[*] and max-[*] arbitrary variants are handled separately
}
```

### 4.1 Responsive Reference

| Variant | Media Query |
|---------|-------------|
| `sm:` | `@media (min-width: 640px)` |
| `md:` | `@media (min-width: 768px)` |
| `lg:` | `@media (min-width: 1024px)` |
| `xl:` | `@media (min-width: 1280px)` |
| `2xl:` | `@media (min-width: 1536px)` |
| `max-sm:` | `@media (max-width: 639.9px)` |
| `max-md:` | `@media (max-width: 767.9px)` |
| `min-[800px]:` | `@media (min-width: 800px)` |
| `max-[1000px]:` | `@media (max-width: 1000px)` |

---

## 5. Dark Mode Variant

```go
func (e *Engine) registerDarkModeVariant() {
    // Default: class-based dark mode
    e.variants.Register("dark", VariantDef{
        Kind:     VariantDefSelector,
        Selector: ".dark &",  // Assumes .dark class on ancestor
        Order:    50,
    })

    // Alternative: media query based
    // Can be configured via theme
}

// For media-query based dark mode:
func (e *Engine) registerMediaDarkMode() {
    e.variants.Register("dark", VariantDef{
        Kind:         VariantDefAtRule,
        AtRule:       "@media",
        AtRuleParams: "(prefers-color-scheme: dark)",
        Order:        50,
    })
}
```

---

## 6. Motion Variants

```go
func (e *Engine) registerMotionVariants() {
    e.variants.Register("motion-safe", VariantDef{
        Kind:         VariantDefAtRule,
        AtRule:       "@media",
        AtRuleParams: "(prefers-reduced-motion: no-preference)",
        Order:        200,
    })

    e.variants.Register("motion-reduce", VariantDef{
        Kind:         VariantDefAtRule,
        AtRule:       "@media",
        AtRuleParams: "(prefers-reduced-motion: reduce)",
        Order:        200,
    })
}
```

---

## 7. Print Variant

```go
func (e *Engine) registerPrintVariant() {
    e.variants.Register("print", VariantDef{
        Kind:         VariantDefAtRule,
        AtRule:       "@media",
        AtRuleParams: "print",
        Order:        300,
    })
}
```

---

## 8. Container Query Variants

```go
func (e *Engine) registerContainerVariants() {
    // @container variants
    containerBreakpoints := map[string]string{
        "@sm":  "640px",
        "@md":  "768px",
        "@lg":  "1024px",
        "@xl":  "1280px",
        "@2xl": "1536px",
    }

    for name, minWidth := range containerBreakpoints {
        e.variants.Register(name, VariantDef{
            Kind:         VariantDefAtRule,
            AtRule:       "@container",
            AtRuleParams: fmt.Sprintf("(min-width: %s)", minWidth),
            Order:        400,
        })
    }
}
```

---

## 9. Group and Peer Variants

Group and peer variants are compound variants that depend on a parent/sibling state.

```go
func (e *Engine) registerGroupVariants() {
    // group-hover, group-focus, etc.
    groupStates := []string{
        "hover", "focus", "active", "visited",
        "focus-within", "focus-visible",
    }

    for _, state := range groupStates {
        stateName := state
        e.variants.Register("group-"+stateName, VariantDef{
            Kind:     VariantDefSelector,
            Selector: fmt.Sprintf(".group:%s &", stateName),
            Order:    1500,
        })
    }

    // Named groups: group/name
    // group-hover/sidebar -> .group\/sidebar:hover &
    // This requires special parsing for the / separator
}

func (e *Engine) registerPeerVariants() {
    // peer-hover, peer-focus, etc.
    peerStates := []string{
        "hover", "focus", "active", "visited",
        "focus-within", "focus-visible",
        "checked", "disabled", "invalid", "valid",
    }

    for _, state := range peerStates {
        stateName := state
        e.variants.Register("peer-"+stateName, VariantDef{
            Kind:     VariantDefSelector,
            Selector: fmt.Sprintf(".peer:%s ~ &", stateName),
            Order:    1500,
        })
    }
}
```

### 9.1 Group/Peer Reference

| Variant | Selector |
|---------|----------|
| `group-hover:` | `.group:hover &` |
| `group-focus:` | `.group:focus &` |
| `peer-hover:` | `.peer:hover ~ &` |
| `peer-checked:` | `.peer:checked ~ &` |

---

## 10. Arbitrary Variants

Arbitrary variants allow any selector:

```go
func (e *Engine) handleArbitraryVariant(v Variant) VariantDef {
    if v.Kind != VariantArbitrary {
        return VariantDef{}
    }

    selector := v.Name
    // Arbitrary variants use & as placeholder
    // [&:nth-child(3)] -> &:nth-child(3)
    // [@media(min-width:800px)] -> @media wrapper

    if strings.HasPrefix(selector, "@") {
        // At-rule variant
        parts := strings.SplitN(selector, " ", 2)
        atRule := parts[0]
        params := ""
        if len(parts) > 1 {
            params = parts[1]
        }
        return VariantDef{
            Kind:         VariantDefAtRule,
            AtRule:       atRule,
            AtRuleParams: params,
            Order:        5000,
        }
    }

    // Selector variant
    return VariantDef{
        Kind:     VariantDefSelector,
        Selector: selector,
        Order:    5000,
    }
}
```

### 10.1 Arbitrary Variant Examples

| Variant | Result |
|---------|--------|
| `[&:nth-child(3)]:` | `&:nth-child(3)` |
| `[&>*]:` | `& > *` |
| `[@media(min-width:800px)]:` | `@media (min-width: 800px)` |
| `[@supports(display:grid)]:` | `@supports (display: grid)` |

---

## 11. Has Variant

```go
func (e *Engine) registerHasVariants() {
    e.variants.Register("has", VariantDef{
        Kind:     VariantDefSelector,
        Selector: "&:has(*)", // Default, usually used with arbitrary
        Order:    1200,
    })

    // has-[selector] is handled as functional variant
}

func (e *Engine) handleHasFunctionalVariant(v Variant) VariantDef {
    if v.Name != "has" || v.Value == "" {
        return VariantDef{}
    }

    return VariantDef{
        Kind:     VariantDefSelector,
        Selector: fmt.Sprintf("&:has(%s)", v.Value),
        Order:    1200,
    }
}
```

---

## 12. Supports Variant

```go
func (e *Engine) registerSupportsVariant() {
    // supports-[display:grid] -> @supports (display: grid)
    // Handled as functional variant
}

func (e *Engine) handleSupportsFunctionalVariant(v Variant) VariantDef {
    if v.Name != "supports" || v.Value == "" {
        return VariantDef{}
    }

    value := v.Value
    // If it's just a property name, wrap as (property: var(--tw))
    if !strings.Contains(value, ":") {
        value = fmt.Sprintf("(%s: var(--tw))", value)
    } else {
        value = fmt.Sprintf("(%s)", value)
    }

    return VariantDef{
        Kind:         VariantDefAtRule,
        AtRule:       "@supports",
        AtRuleParams: value,
        Order:        250,
    }
}
```

---

## 13. RTL/LTR Variants

```go
func (e *Engine) registerDirectionVariants() {
    e.variants.Register("ltr", VariantDef{
        Kind:     VariantDefSelector,
        Selector: "[dir=\"ltr\"] &",
        Order:    60,
    })

    e.variants.Register("rtl", VariantDef{
        Kind:     VariantDefSelector,
        Selector: "[dir=\"rtl\"] &",
        Order:    60,
    })
}
```

---

## 14. Stacking Variants

Multiple variants can be stacked: `md:hover:focus:bg-blue-500`

```go
func (e *Engine) compileStackedVariants(variants []Variant) (*CachedRule, error) {
    // Start with base rule
    rule := &CachedRule{
        Selector:     "." + escapeSelector(rawClass),
        Declarations: decls,
    }

    // Apply variants in reverse order (inside-out)
    // md:hover:bg-blue-500 means:
    // 1. Apply hover: selector modification
    // 2. Wrap in md: media query

    for i := len(variants) - 1; i >= 0; i-- {
        rule = e.applyVariant(rule, variants[i])
    }

    return rule, nil
}
```

### 14.1 Stacking Example

For `md:hover:bg-blue-500`:

1. Start: `.bg-blue-500 { background-color: #3b82f6; }`
2. Apply `hover:`: `.hover\:bg-blue-500:hover { background-color: #3b82f6; }`
3. Apply `md:`:
```css
@media (min-width: 768px) {
    .md\:hover\:bg-blue-500:hover {
        background-color: #3b82f6;
    }
}
```

---

## 15. Variant Ordering

Variants must be output in a specific order for correct CSS specificity:

```go
// Variant order groups (lower = earlier in output)
const (
    OrderBase           = 0
    OrderDark           = 50
    OrderRTL            = 60
    OrderResponsive     = 100
    OrderMotion         = 200
    OrderSupports       = 250
    OrderPrint          = 300
    OrderContainer      = 400
    OrderPseudoClass    = 1000
    OrderPseudoElement  = 2000
    OrderGroupPeer      = 1500
    OrderArbitrary      = 5000
)
```

---

## 16. Variant Registry

```go
type VariantRegistry struct {
    variants map[string]VariantDef
    mu       sync.RWMutex
}

func NewVariantRegistry() *VariantRegistry {
    return &VariantRegistry{
        variants: make(map[string]VariantDef),
    }
}

func (r *VariantRegistry) Register(name string, def VariantDef) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.variants[name] = def
}

func (r *VariantRegistry) Get(name string) (VariantDef, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    def, ok := r.variants[name]
    return def, ok
}

func (r *VariantRegistry) RegisterDefaults() {
    // Register all built-in variants
}
```

---

## 17. Testing

```go
// variant_test.go

func TestPseudoClassVariants(t *testing.T) {
    engine := New()

    tests := []struct {
        class            string
        expectedSelector string
    }{
        {"hover:bg-blue-500", ".hover\\:bg-blue-500:hover"},
        {"focus:bg-blue-500", ".focus\\:bg-blue-500:focus"},
        {"active:bg-blue-500", ".active\\:bg-blue-500:active"},
        {"first:bg-blue-500", ".first\\:bg-blue-500:first-child"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            css := engine.Generate([]string{tt.class})
            assert.Contains(t, css, tt.expectedSelector)
        })
    }
}

func TestResponsiveVariants(t *testing.T) {
    engine := New()

    tests := []struct {
        class         string
        expectedMedia string
    }{
        {"sm:bg-blue-500", "@media (min-width: 640px)"},
        {"md:bg-blue-500", "@media (min-width: 768px)"},
        {"lg:bg-blue-500", "@media (min-width: 1024px)"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            css := engine.Generate([]string{tt.class})
            assert.Contains(t, css, tt.expectedMedia)
        })
    }
}

func TestStackedVariants(t *testing.T) {
    engine := New()

    css := engine.Generate([]string{"md:hover:bg-blue-500"})

    assert.Contains(t, css, "@media (min-width: 768px)")
    assert.Contains(t, css, ":hover")
    assert.Contains(t, css, "background-color")
}

func TestDarkModeVariant(t *testing.T) {
    engine := New()

    css := engine.Generate([]string{"dark:bg-gray-900"})

    assert.Contains(t, css, ".dark")
    assert.Contains(t, css, "background-color")
}

func TestGroupVariants(t *testing.T) {
    engine := New()

    css := engine.Generate([]string{"group-hover:bg-blue-500"})

    assert.Contains(t, css, ".group:hover")
}

func TestArbitraryVariants(t *testing.T) {
    engine := New()

    tests := []struct {
        class    string
        expected string
    }{
        {"[&:nth-child(3)]:bg-blue-500", ":nth-child(3)"},
        {"[@media(min-width:800px)]:bg-blue-500", "@media"},
    }

    for _, tt := range tests {
        t.Run(tt.class, func(t *testing.T) {
            css := engine.Generate([]string{tt.class})
            assert.Contains(t, css, tt.expected)
        })
    }
}
```

---

## 18. Files to Create/Modify

| File | Purpose | Lines (est.) |
|------|---------|--------------|
| `variant.go` | Variant system | 500 |
| `variant_test.go` | Variant tests | 300 |
| Update `tailwind.go` | Integrate variants | 50 |

---

## 19. Completion Criteria

Phase 8 is complete when:

1. ✅ Pseudo-class variants work (`hover:`, `focus:`, `active:`, `disabled:`)
2. ✅ Pseudo-element variants work (`before:`, `after:`, `placeholder:`)
3. ✅ Responsive variants work (`sm:`, `md:`, `lg:`, `xl:`, `2xl:`)
4. ✅ Dark mode variant works (`dark:`)
5. ✅ Motion variants work (`motion-safe:`, `motion-reduce:`)
6. ✅ Print variant works (`print:`)
7. ✅ Group variants work (`group-hover:`, `group-focus:`)
8. ✅ Peer variants work (`peer-hover:`, `peer-checked:`)
9. ✅ Stacked variants work (`md:hover:bg-blue-500`)
10. ✅ Arbitrary variants work (`[&:nth-child(3)]:`)
11. ✅ Variant ordering is correct
12. ✅ All tests pass

---

*Last Updated: 2024-12-12*
