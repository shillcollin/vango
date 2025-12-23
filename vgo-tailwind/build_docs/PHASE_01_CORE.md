# Phase 1: Core Infrastructure

## Overview

Phase 1 establishes the foundation for vgo-tailwind. It implements the parsing, caching, theme system, and CSS generation pipeline that all subsequent phases build upon.

**Files to create:**
- `tailwind.go` - Engine type and public API
- `candidate.go` - Class string parsing
- `theme.go` - Design tokens (colors, spacing, breakpoints)
- `cache.go` - Thread-safe CSS caching
- `render.go` - CSS serialization
- `utilities/registry.go` - Utility registration system

---

## 1. Core Data Structures

### 1.1 Candidate (Parsed Class)

A `Candidate` represents a parsed Tailwind class string. This is the central data structure.

**Reference:** `/tailwind/packages/tailwindcss/src/candidate.ts`

```go
// candidate.go

package tailwind

// CandidateKind represents the type of utility.
type CandidateKind uint8

const (
    KindStatic CandidateKind = iota + 1     // e.g., "flex", "hidden"
    KindFunctional                           // e.g., "p-4", "bg-red-500"
    KindArbitrary                            // e.g., "[color:red]"
)

// Candidate represents a parsed Tailwind class.
type Candidate struct {
    Kind      CandidateKind
    Root      string      // Utility name: "bg", "p", "flex"
    Value     *Value      // nil for static, set for functional/arbitrary
    Modifier  *Modifier   // Optional opacity/modifier like "/50"
    Variants  []Variant   // Applied variants: hover:, sm:, dark:
    Important bool        // Has ! suffix or prefix
    Negative  bool        // Has - prefix (e.g., "-m-4")
    Raw       string      // Original class string for cache key
}

// Value represents the value part of a functional utility.
type Value struct {
    Kind     ValueKind
    Content  string    // "red-500" or "17px" (without brackets for arbitrary)
    DataType string    // For arbitrary: "color", "length", "percentage", etc.
    Fraction string    // For fractions: "1/2" in "w-1/2"
}

// ValueKind distinguishes named vs arbitrary values.
type ValueKind uint8

const (
    ValueNamed ValueKind = iota + 1     // e.g., "red-500", "4", "full"
    ValueArbitrary                       // e.g., "[17px]", "[#ff0000]"
)

// Modifier represents an opacity or other modifier.
type Modifier struct {
    Kind  ModifierKind
    Value string        // "50" or "0.5" or "[50%]"
}

// ModifierKind distinguishes named vs arbitrary modifiers.
type ModifierKind uint8

const (
    ModifierNamed ModifierKind = iota + 1    // e.g., "/50"
    ModifierArbitrary                         // e.g., "/[50%]"
)

// Variant represents a variant like hover:, sm:, or dark:.
type Variant struct {
    Kind  VariantKind
    Name  string        // "hover", "sm", "dark", or arbitrary selector
    Value string        // For functional variants like "min-[800px]"
}

// VariantKind distinguishes variant types.
type VariantKind uint8

const (
    VariantStatic VariantKind = iota + 1    // hover, focus, active
    VariantFunctional                        // sm, md, lg (responsive)
    VariantArbitrary                         // [&:nth-child(3)]
    VariantCompound                          // group-hover, peer-focus
)
```

### 1.2 Declaration and CachedRule

These represent the CSS output.

```go
// render.go

package tailwind

// Declaration is a single CSS property-value pair.
type Declaration struct {
    Property  string
    Value     string
    Important bool
}

// CachedRule represents a compiled CSS rule ready for output.
type CachedRule struct {
    Selector     string        // ".p-4", ".hover\\:bg-blue-500:hover"
    Declarations []Declaration // CSS properties
    AtRules      []string      // Wrapping at-rules: "@media (min-width: 768px)"
    Order        int64         // For sorting (variant order, then property order)
}

// String renders the rule as CSS.
func (r *CachedRule) String() string {
    // Implementation renders CSS with proper formatting
}
```

### 1.3 Theme

The theme holds all design tokens.

```go
// theme.go

package tailwind

import "sync"

// Theme holds design tokens organized by namespace.
type Theme struct {
    mu     sync.RWMutex
    values map[string]string  // Full key -> value, e.g., "--color-red-500" -> "oklch(...)"
}

// NewDefaultTheme creates a theme with Tailwind's default values.
func NewDefaultTheme() *Theme {
    t := &Theme{
        values: make(map[string]string),
    }
    t.loadDefaults()
    return t
}

// Resolve looks up a value from the theme.
// Example: Resolve("red-500", "--color") returns the color value.
func (t *Theme) Resolve(value string, namespace string) (string, bool) {
    t.mu.RLock()
    defer t.mu.RUnlock()

    key := namespace + "-" + value
    v, ok := t.values[key]
    return v, ok
}

// ResolveAny tries multiple namespaces in order.
func (t *Theme) ResolveAny(value string, namespaces ...string) (string, bool) {
    for _, ns := range namespaces {
        if v, ok := t.Resolve(value, ns); ok {
            return v, true
        }
    }
    return "", false
}

// Set adds or updates a theme value.
func (t *Theme) Set(namespace, key, value string) {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.values[namespace+"-"+key] = value
}
```

### 1.4 Cache

Thread-safe caching for compiled rules.

```go
// cache.go

package tailwind

import "sync"

// Cache provides thread-safe caching of compiled CSS rules.
type Cache struct {
    rules   sync.Map           // string -> *CachedRule
    invalid sync.Map           // string -> struct{} (negative cache)
}

// NewCache creates a new cache.
func NewCache() *Cache {
    return &Cache{}
}

// Get retrieves a cached rule.
func (c *Cache) Get(class string) (*CachedRule, bool) {
    if v, ok := c.rules.Load(class); ok {
        return v.(*CachedRule), true
    }
    return nil, false
}

// Set stores a compiled rule.
func (c *Cache) Set(class string, rule *CachedRule) {
    c.rules.Store(class, rule)
}

// IsInvalid checks if a class is known to be invalid.
func (c *Cache) IsInvalid(class string) bool {
    _, ok := c.invalid.Load(class)
    return ok
}

// MarkInvalid marks a class as invalid (won't try to compile again).
func (c *Cache) MarkInvalid(class string) {
    c.invalid.Store(class, struct{}{})
}

// Stats returns cache statistics.
func (c *Cache) Stats() CacheStats {
    var valid, invalid int
    c.rules.Range(func(_, _ any) bool { valid++; return true })
    c.invalid.Range(func(_, _ any) bool { invalid++; return true })
    return CacheStats{ValidRules: valid, InvalidClasses: invalid}
}

type CacheStats struct {
    ValidRules     int
    InvalidClasses int
}
```

---

## 2. Parsing Algorithm

### 2.1 Overview

The parsing algorithm transforms a class string like `hover:bg-red-500/50` into a `Candidate` struct.

**Reference:** `/tailwind/packages/tailwindcss/src/candidate.ts` (parseCandidate function)

### 2.2 Parsing Steps

```
Input: "hover:md:bg-red-500/50!"

Step 1: Check for important flag
        "hover:md:bg-red-500/50!" → important=true, rest="hover:md:bg-red-500/50"

Step 2: Split by colons to extract variants
        "hover:md:bg-red-500/50" → variants=["hover", "md"], base="bg-red-500/50"

Step 3: Split by slash to extract modifier
        "bg-red-500/50" → modifier="50", utility="bg-red-500"

Step 4: Check for negative prefix
        "bg-red-500" → negative=false (no leading -)
        "-m-4" → negative=true, utility="m-4"

Step 5: Check for arbitrary property [property:value]
        "[color:red]" → kind=arbitrary, property="color", value="red"

Step 6: Parse utility root and value
        "bg-red-500" → Try progressively shorter prefixes:
            "bg-red-500" - not a utility
            "bg-red" - not a utility
            "bg" - IS a utility! → root="bg", value="red-500"

Step 7: Parse value details
        "red-500" → kind=named, content="red-500"
        "[17px]" → kind=arbitrary, content="17px"
        "1/2" → kind=named, content="1/2", fraction="1/2"
```

### 2.3 Implementation

```go
// candidate.go

// ParseCandidate parses a class string into a Candidate.
func ParseCandidate(input string, registry *Registry) (*Candidate, error) {
    if input == "" {
        return nil, fmt.Errorf("empty class string")
    }

    c := &Candidate{Raw: input}

    // Step 1: Check for important flag (! at start or end)
    remaining := input
    if strings.HasSuffix(remaining, "!") {
        c.Important = true
        remaining = remaining[:len(remaining)-1]
    } else if strings.HasPrefix(remaining, "!") {
        c.Important = true
        remaining = remaining[1:]
    }

    // Step 2: Split by colons to extract variants
    parts := splitByUnescapedColon(remaining)
    if len(parts) == 0 {
        return nil, fmt.Errorf("invalid class: %s", input)
    }

    base := parts[len(parts)-1]
    variantStrs := parts[:len(parts)-1]

    // Parse variants (in reverse order - outermost first)
    for i := len(variantStrs) - 1; i >= 0; i-- {
        v, err := parseVariant(variantStrs[i])
        if err != nil {
            return nil, err
        }
        c.Variants = append(c.Variants, v)
    }

    // Step 3: Split by slash to extract modifier
    base, modifierStr := splitByLastSlash(base)
    if modifierStr != "" {
        c.Modifier = parseModifier(modifierStr)
    }

    // Step 4: Check for negative prefix
    if strings.HasPrefix(base, "-") && len(base) > 1 {
        c.Negative = true
        base = base[1:]
    }

    // Step 5: Check for arbitrary property [property:value]
    if isArbitraryProperty(base) {
        return parseArbitraryProperty(base, c)
    }

    // Step 6: Find utility root by trying progressively shorter prefixes
    root, valueStr, found := findUtilityRoot(base, registry)
    if !found {
        return nil, fmt.Errorf("unknown utility: %s", base)
    }

    c.Root = root

    // Step 7: Parse value if present
    if valueStr != "" {
        c.Value = parseValue(valueStr)
        c.Kind = KindFunctional
    } else {
        c.Kind = KindStatic
    }

    return c, nil
}

// splitByUnescapedColon splits on colons that aren't inside brackets.
func splitByUnescapedColon(s string) []string {
    var parts []string
    var current strings.Builder
    depth := 0

    for _, r := range s {
        switch r {
        case '[':
            depth++
            current.WriteRune(r)
        case ']':
            depth--
            current.WriteRune(r)
        case ':':
            if depth == 0 {
                parts = append(parts, current.String())
                current.Reset()
            } else {
                current.WriteRune(r)
            }
        default:
            current.WriteRune(r)
        }
    }

    if current.Len() > 0 {
        parts = append(parts, current.String())
    }

    return parts
}

// splitByLastSlash splits on the last slash not inside brackets.
func splitByLastSlash(s string) (string, string) {
    depth := 0
    lastSlash := -1

    for i := len(s) - 1; i >= 0; i-- {
        switch s[i] {
        case ']':
            depth++
        case '[':
            depth--
        case '/':
            if depth == 0 {
                lastSlash = i
                break
            }
        }
        if lastSlash >= 0 {
            break
        }
    }

    if lastSlash < 0 {
        return s, ""
    }
    return s[:lastSlash], s[lastSlash+1:]
}

// findUtilityRoot tries progressively shorter prefixes to find a registered utility.
func findUtilityRoot(base string, registry *Registry) (root, value string, found bool) {
    // First, check if the entire base is a static utility
    if registry.HasStatic(base) {
        return base, "", true
    }

    // Try progressively shorter prefixes
    for i := len(base) - 1; i >= 0; i-- {
        if base[i] == '-' {
            prefix := base[:i]
            if registry.HasFunctional(prefix) {
                return prefix, base[i+1:], true
            }
        }
    }

    // Try the whole thing as a functional utility with no value
    if registry.HasFunctional(base) {
        return base, "", true
    }

    return "", "", false
}

// parseValue parses a utility value.
func parseValue(s string) *Value {
    v := &Value{}

    // Check for arbitrary value [...]
    if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
        v.Kind = ValueArbitrary
        inner := s[1 : len(s)-1]

        // Check for data type hint like [length:17px]
        if idx := strings.Index(inner, ":"); idx > 0 {
            v.DataType = inner[:idx]
            v.Content = inner[idx+1:]
        } else {
            v.Content = inner
        }
        return v
    }

    // Named value
    v.Kind = ValueNamed
    v.Content = s

    // Check for fraction
    if strings.Contains(s, "/") {
        v.Fraction = s
    }

    return v
}

// parseModifier parses an opacity or other modifier.
func parseModifier(s string) *Modifier {
    m := &Modifier{}

    if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
        m.Kind = ModifierArbitrary
        m.Value = s[1 : len(s)-1]
    } else {
        m.Kind = ModifierNamed
        m.Value = s
    }

    return m
}

// isArbitraryProperty checks for [property:value] syntax.
func isArbitraryProperty(s string) bool {
    if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
        return false
    }
    inner := s[1 : len(s)-1]
    return strings.Contains(inner, ":")
}

// parseArbitraryProperty parses [property:value] syntax.
func parseArbitraryProperty(s string, c *Candidate) (*Candidate, error) {
    inner := s[1 : len(s)-1]
    idx := strings.Index(inner, ":")
    if idx < 0 {
        return nil, fmt.Errorf("invalid arbitrary property: %s", s)
    }

    c.Kind = KindArbitrary
    c.Root = inner[:idx]
    c.Value = &Value{
        Kind:    ValueArbitrary,
        Content: inner[idx+1:],
    }

    return c, nil
}

// parseVariant parses a variant string.
func parseVariant(s string) (Variant, error) {
    v := Variant{}

    // Arbitrary variant [&:nth-child(3)]
    if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
        v.Kind = VariantArbitrary
        v.Name = s[1 : len(s)-1]
        return v, nil
    }

    // Compound variant (group-hover, peer-focus)
    if strings.HasPrefix(s, "group-") || strings.HasPrefix(s, "peer-") {
        v.Kind = VariantCompound
        v.Name = s
        return v, nil
    }

    // Functional variant with value (min-[800px])
    if idx := strings.Index(s, "-["); idx > 0 {
        v.Kind = VariantFunctional
        v.Name = s[:idx]
        v.Value = s[idx+1:]
        return v, nil
    }

    // Static or functional variant (hover, sm, md)
    v.Name = s

    // Check if it's a responsive variant
    if isResponsiveVariant(s) {
        v.Kind = VariantFunctional
    } else {
        v.Kind = VariantStatic
    }

    return v, nil
}

func isResponsiveVariant(s string) bool {
    switch s {
    case "sm", "md", "lg", "xl", "2xl":
        return true
    }
    return false
}
```

---

## 3. Theme System

### 3.1 Default Theme Values

The theme must include Tailwind's complete default values.

**Reference:** `/tailwind/packages/tailwindcss/theme.css`

```go
// theme.go

func (t *Theme) loadDefaults() {
    // Spacing scale (used for padding, margin, gap, etc.)
    // Based on 0.25rem = 4px base
    t.loadSpacing()

    // Colors (full Tailwind palette)
    t.loadColors()

    // Breakpoints
    t.loadBreakpoints()

    // Typography
    t.loadTypography()

    // Shadows
    t.loadShadows()

    // Border radius
    t.loadBorderRadius()
}

func (t *Theme) loadSpacing() {
    spacing := map[string]string{
        "0":    "0px",
        "px":   "1px",
        "0.5":  "0.125rem",  // 2px
        "1":    "0.25rem",   // 4px
        "1.5":  "0.375rem",  // 6px
        "2":    "0.5rem",    // 8px
        "2.5":  "0.625rem",  // 10px
        "3":    "0.75rem",   // 12px
        "3.5":  "0.875rem",  // 14px
        "4":    "1rem",      // 16px
        "5":    "1.25rem",   // 20px
        "6":    "1.5rem",    // 24px
        "7":    "1.75rem",   // 28px
        "8":    "2rem",      // 32px
        "9":    "2.25rem",   // 36px
        "10":   "2.5rem",    // 40px
        "11":   "2.75rem",   // 44px
        "12":   "3rem",      // 48px
        "14":   "3.5rem",    // 56px
        "16":   "4rem",      // 64px
        "20":   "5rem",      // 80px
        "24":   "6rem",      // 96px
        "28":   "7rem",      // 112px
        "32":   "8rem",      // 128px
        "36":   "9rem",      // 144px
        "40":   "10rem",     // 160px
        "44":   "11rem",     // 176px
        "48":   "12rem",     // 192px
        "52":   "13rem",     // 208px
        "56":   "14rem",     // 224px
        "60":   "15rem",     // 240px
        "64":   "16rem",     // 256px
        "72":   "18rem",     // 288px
        "80":   "20rem",     // 320px
        "96":   "24rem",     // 384px
    }

    for k, v := range spacing {
        t.Set("--spacing", k, v)
    }
}

func (t *Theme) loadBreakpoints() {
    breakpoints := map[string]string{
        "sm":  "640px",
        "md":  "768px",
        "lg":  "1024px",
        "xl":  "1280px",
        "2xl": "1536px",
    }

    for k, v := range breakpoints {
        t.Set("--breakpoint", k, v)
    }
}

func (t *Theme) loadColors() {
    // This is a large function - load all Tailwind colors
    // Reference: /tailwind/packages/tailwindcss/theme.css

    // Example for red palette (Tailwind uses oklch in v4)
    reds := map[string]string{
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
    for k, v := range reds {
        t.Set("--color", "red-"+k, v)
    }

    // ... repeat for all colors:
    // gray, slate, zinc, neutral, stone
    // red, orange, amber, yellow, lime, green, emerald, teal, cyan
    // sky, blue, indigo, violet, purple, fuchsia, pink, rose
    // white, black, transparent, current, inherit

    // Special colors
    t.Set("--color", "white", "#ffffff")
    t.Set("--color", "black", "#000000")
    t.Set("--color", "transparent", "transparent")
    t.Set("--color", "current", "currentColor")
    t.Set("--color", "inherit", "inherit")
}

func (t *Theme) loadTypography() {
    // Font sizes
    fontSizes := map[string]string{
        "xs":   "0.75rem",   // 12px
        "sm":   "0.875rem",  // 14px
        "base": "1rem",      // 16px
        "lg":   "1.125rem",  // 18px
        "xl":   "1.25rem",   // 20px
        "2xl":  "1.5rem",    // 24px
        "3xl":  "1.875rem",  // 30px
        "4xl":  "2.25rem",   // 36px
        "5xl":  "3rem",      // 48px
        "6xl":  "3.75rem",   // 60px
        "7xl":  "4.5rem",    // 72px
        "8xl":  "6rem",      // 96px
        "9xl":  "8rem",      // 128px
    }
    for k, v := range fontSizes {
        t.Set("--font-size", k, v)
    }

    // Font weights
    fontWeights := map[string]string{
        "thin":       "100",
        "extralight": "200",
        "light":      "300",
        "normal":     "400",
        "medium":     "500",
        "semibold":   "600",
        "bold":       "700",
        "extrabold":  "800",
        "black":      "900",
    }
    for k, v := range fontWeights {
        t.Set("--font-weight", k, v)
    }

    // Line heights
    lineHeights := map[string]string{
        "none":    "1",
        "tight":   "1.25",
        "snug":    "1.375",
        "normal":  "1.5",
        "relaxed": "1.625",
        "loose":   "2",
    }
    for k, v := range lineHeights {
        t.Set("--leading", k, v)
    }

    // Letter spacing
    letterSpacing := map[string]string{
        "tighter": "-0.05em",
        "tight":   "-0.025em",
        "normal":  "0em",
        "wide":    "0.025em",
        "wider":   "0.05em",
        "widest":  "0.1em",
    }
    for k, v := range letterSpacing {
        t.Set("--tracking", k, v)
    }
}

func (t *Theme) loadShadows() {
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
    for k, v := range shadows {
        t.Set("--shadow", k, v)
    }
}

func (t *Theme) loadBorderRadius() {
    radii := map[string]string{
        "none": "0px",
        "sm":   "0.125rem",  // 2px
        "":     "0.25rem",   // 4px
        "md":   "0.375rem",  // 6px
        "lg":   "0.5rem",    // 8px
        "xl":   "0.75rem",   // 12px
        "2xl":  "1rem",      // 16px
        "3xl":  "1.5rem",    // 24px
        "full": "9999px",
    }
    for k, v := range radii {
        t.Set("--radius", k, v)
    }
}
```

---

## 4. Utility Registry

### 4.1 Registry Structure

```go
// utilities/registry.go

package utilities

import "github.com/vango-dev/vgo-tailwind"

// CompileFunc generates CSS declarations from a candidate.
type CompileFunc func(c *tailwind.Candidate, theme *tailwind.Theme) []tailwind.Declaration

// Utility defines how a utility class generates CSS.
type Utility struct {
    Kind    UtilityKind
    Compile CompileFunc
}

type UtilityKind uint8

const (
    UtilityStatic UtilityKind = iota + 1
    UtilityFunctional
)

// Registry holds all registered utilities.
type Registry struct {
    static     map[string]Utility   // "flex" -> Utility
    functional map[string]Utility   // "p" -> Utility (for p-4, p-[17px], etc.)
}

// NewRegistry creates a registry with all default utilities.
func NewRegistry() *Registry {
    r := &Registry{
        static:     make(map[string]Utility),
        functional: make(map[string]Utility),
    }
    r.registerDefaults()
    return r
}

// RegisterStatic registers a static utility.
func (r *Registry) RegisterStatic(name string, compile CompileFunc) {
    r.static[name] = Utility{
        Kind:    UtilityStatic,
        Compile: compile,
    }
}

// RegisterFunctional registers a functional utility.
func (r *Registry) RegisterFunctional(name string, compile CompileFunc) {
    r.functional[name] = Utility{
        Kind:    UtilityFunctional,
        Compile: compile,
    }
}

// HasStatic checks if a static utility is registered.
func (r *Registry) HasStatic(name string) bool {
    _, ok := r.static[name]
    return ok
}

// HasFunctional checks if a functional utility is registered.
func (r *Registry) HasFunctional(name string) bool {
    _, ok := r.functional[name]
    return ok
}

// Get retrieves a utility by name.
func (r *Registry) Get(name string, kind UtilityKind) (Utility, bool) {
    switch kind {
    case UtilityStatic:
        u, ok := r.static[name]
        return u, ok
    case UtilityFunctional:
        u, ok := r.functional[name]
        return u, ok
    }
    return Utility{}, false
}

func (r *Registry) registerDefaults() {
    // Static utilities are registered in phase 2
    // Functional utilities are registered in phases 3-7
}
```

---

## 5. Engine and Public API

### 5.1 Engine

```go
// tailwind.go

package tailwind

import (
    "strings"
    "sync"

    "github.com/vango-dev/vgo-tailwind/utilities"
)

// Engine is the main vgo-tailwind engine.
type Engine struct {
    theme    *Theme
    registry *utilities.Registry
    cache    *Cache
    mu       sync.RWMutex
}

// New creates a new Engine with default configuration.
func New() *Engine {
    return &Engine{
        theme:    NewDefaultTheme(),
        registry: utilities.NewRegistry(),
        cache:    NewCache(),
    }
}

// NewWithTheme creates an Engine with a custom theme.
func NewWithTheme(theme *Theme) *Engine {
    return &Engine{
        theme:    theme,
        registry: utilities.NewRegistry(),
        cache:    NewCache(),
    }
}

// Generate compiles a list of class strings to CSS.
func (e *Engine) Generate(classes []string) string {
    var rules []*CachedRule

    for _, class := range classes {
        // Split compound classes (e.g., "flex p-4" -> ["flex", "p-4"])
        for _, c := range strings.Fields(class) {
            rule, err := e.compileClass(c)
            if err != nil {
                // Skip invalid classes (could log in debug mode)
                continue
            }
            if rule != nil {
                rules = append(rules, rule)
            }
        }
    }

    // Sort rules by order
    sortRules(rules)

    // Render to CSS string
    return renderRules(rules)
}

// compileClass compiles a single class to a CachedRule.
func (e *Engine) compileClass(class string) (*CachedRule, error) {
    // Check cache first
    if rule, ok := e.cache.Get(class); ok {
        return rule, nil
    }

    // Check negative cache
    if e.cache.IsInvalid(class) {
        return nil, fmt.Errorf("invalid class: %s", class)
    }

    // Parse candidate
    candidate, err := ParseCandidate(class, e.registry)
    if err != nil {
        e.cache.MarkInvalid(class)
        return nil, err
    }

    // Compile to CSS
    rule, err := e.compileCandidateToRule(candidate)
    if err != nil {
        e.cache.MarkInvalid(class)
        return nil, err
    }

    // Cache and return
    e.cache.Set(class, rule)
    return rule, nil
}

// compileCandidateToRule generates CSS for a parsed candidate.
func (e *Engine) compileCandidateToRule(c *Candidate) (*CachedRule, error) {
    // Get the utility
    var util utilities.Utility
    var ok bool

    switch c.Kind {
    case KindStatic:
        util, ok = e.registry.Get(c.Root, utilities.UtilityStatic)
    case KindFunctional:
        util, ok = e.registry.Get(c.Root, utilities.UtilityFunctional)
    case KindArbitrary:
        // Handle arbitrary properties directly
        return e.compileArbitraryProperty(c)
    }

    if !ok {
        return nil, fmt.Errorf("unknown utility: %s", c.Root)
    }

    // Compile declarations
    decls := util.Compile(c, e.theme)
    if len(decls) == 0 {
        return nil, fmt.Errorf("no CSS generated for: %s", c.Raw)
    }

    // Apply important flag
    if c.Important {
        for i := range decls {
            decls[i].Important = true
        }
    }

    // Build selector
    selector := "." + escapeSelector(c.Raw)

    // Create rule
    rule := &CachedRule{
        Selector:     selector,
        Declarations: decls,
    }

    // Apply variants
    for _, v := range c.Variants {
        rule = e.applyVariant(rule, v)
    }

    return rule, nil
}

// compileArbitraryProperty handles [property:value] syntax.
func (e *Engine) compileArbitraryProperty(c *Candidate) (*CachedRule, error) {
    decl := Declaration{
        Property:  c.Root,
        Value:     c.Value.Content,
        Important: c.Important,
    }

    selector := "." + escapeSelector(c.Raw)

    rule := &CachedRule{
        Selector:     selector,
        Declarations: []Declaration{decl},
    }

    // Apply variants
    for _, v := range c.Variants {
        rule = e.applyVariant(rule, v)
    }

    return rule, nil
}
```

---

## 6. CSS Rendering

### 6.1 Rendering Rules to CSS

```go
// render.go

package tailwind

import (
    "fmt"
    "sort"
    "strings"
)

// renderRules converts compiled rules to a CSS string.
func renderRules(rules []*CachedRule) string {
    var sb strings.Builder

    // Group by at-rules
    plain := make([]*CachedRule, 0)
    atRuleGroups := make(map[string][]*CachedRule)

    for _, rule := range rules {
        if len(rule.AtRules) == 0 {
            plain = append(plain, rule)
        } else {
            key := strings.Join(rule.AtRules, " ")
            atRuleGroups[key] = append(atRuleGroups[key], rule)
        }
    }

    // Render plain rules first
    for _, rule := range plain {
        sb.WriteString(rule.String())
        sb.WriteString("\n")
    }

    // Render at-rule groups
    // Sort at-rule keys for consistent output
    var atRuleKeys []string
    for k := range atRuleGroups {
        atRuleKeys = append(atRuleKeys, k)
    }
    sort.Strings(atRuleKeys)

    for _, key := range atRuleKeys {
        rules := atRuleGroups[key]
        atRules := strings.Split(key, " ")

        // Open at-rules
        for _, ar := range atRules {
            sb.WriteString(ar)
            sb.WriteString(" {\n")
        }

        // Render rules inside
        for _, rule := range rules {
            // Temporarily clear at-rules for rendering
            sb.WriteString("  ")
            sb.WriteString(rule.selectorAndDecls())
            sb.WriteString("\n")
        }

        // Close at-rules
        for range atRules {
            sb.WriteString("}\n")
        }
    }

    return sb.String()
}

// String renders a single rule as CSS.
func (r *CachedRule) String() string {
    return r.selectorAndDecls()
}

func (r *CachedRule) selectorAndDecls() string {
    var sb strings.Builder

    sb.WriteString(r.Selector)
    sb.WriteString(" { ")

    for i, decl := range r.Declarations {
        if i > 0 {
            sb.WriteString(" ")
        }
        sb.WriteString(decl.Property)
        sb.WriteString(": ")
        sb.WriteString(decl.Value)
        if decl.Important {
            sb.WriteString(" !important")
        }
        sb.WriteString(";")
    }

    sb.WriteString(" }")
    return sb.String()
}

// escapeSelector escapes special characters in CSS selectors.
func escapeSelector(s string) string {
    var sb strings.Builder
    for _, r := range s {
        switch r {
        case ':', '[', ']', '/', '.', '#', '(', ')', ',', '!', '@', '%', '^', '&', '*', '+', '=', '{', '}', '|', '\\', '<', '>', '?', '`', '~', '"', '\'', ' ':
            sb.WriteRune('\\')
            sb.WriteRune(r)
        default:
            sb.WriteRune(r)
        }
    }
    return sb.String()
}

// sortRules sorts rules by their order value.
func sortRules(rules []*CachedRule) {
    sort.SliceStable(rules, func(i, j int) bool {
        return rules[i].Order < rules[j].Order
    })
}
```

---

## 7. Testing

### 7.1 Parsing Tests

```go
// candidate_test.go

package tailwind

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestParseCandidate(t *testing.T) {
    registry := utilities.NewRegistry()

    tests := []struct {
        input    string
        expected Candidate
    }{
        // Static utility
        {
            input: "flex",
            expected: Candidate{
                Kind: KindStatic,
                Root: "flex",
                Raw:  "flex",
            },
        },
        // Functional utility
        {
            input: "p-4",
            expected: Candidate{
                Kind:  KindFunctional,
                Root:  "p",
                Value: &Value{Kind: ValueNamed, Content: "4"},
                Raw:   "p-4",
            },
        },
        // Arbitrary value
        {
            input: "p-[17px]",
            expected: Candidate{
                Kind:  KindFunctional,
                Root:  "p",
                Value: &Value{Kind: ValueArbitrary, Content: "17px"},
                Raw:   "p-[17px]",
            },
        },
        // With variant
        {
            input: "hover:bg-red-500",
            expected: Candidate{
                Kind:     KindFunctional,
                Root:     "bg",
                Value:    &Value{Kind: ValueNamed, Content: "red-500"},
                Variants: []Variant{{Kind: VariantStatic, Name: "hover"}},
                Raw:      "hover:bg-red-500",
            },
        },
        // With modifier
        {
            input: "bg-red-500/50",
            expected: Candidate{
                Kind:     KindFunctional,
                Root:     "bg",
                Value:    &Value{Kind: ValueNamed, Content: "red-500"},
                Modifier: &Modifier{Kind: ModifierNamed, Value: "50"},
                Raw:      "bg-red-500/50",
            },
        },
        // Important
        {
            input: "p-4!",
            expected: Candidate{
                Kind:      KindFunctional,
                Root:      "p",
                Value:     &Value{Kind: ValueNamed, Content: "4"},
                Important: true,
                Raw:       "p-4!",
            },
        },
        // Negative
        {
            input: "-m-4",
            expected: Candidate{
                Kind:     KindFunctional,
                Root:     "m",
                Value:    &Value{Kind: ValueNamed, Content: "4"},
                Negative: true,
                Raw:      "-m-4",
            },
        },
        // Arbitrary property
        {
            input: "[color:red]",
            expected: Candidate{
                Kind:  KindArbitrary,
                Root:  "color",
                Value: &Value{Kind: ValueArbitrary, Content: "red"},
                Raw:   "[color:red]",
            },
        },
        // Multiple variants
        {
            input: "hover:md:bg-blue-500",
            expected: Candidate{
                Kind:  KindFunctional,
                Root:  "bg",
                Value: &Value{Kind: ValueNamed, Content: "blue-500"},
                Variants: []Variant{
                    {Kind: VariantFunctional, Name: "md"},
                    {Kind: VariantStatic, Name: "hover"},
                },
                Raw: "hover:md:bg-blue-500",
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            got, err := ParseCandidate(tt.input, registry)
            assert.NoError(t, err)
            assert.Equal(t, tt.expected.Kind, got.Kind)
            assert.Equal(t, tt.expected.Root, got.Root)
            assert.Equal(t, tt.expected.Important, got.Important)
            assert.Equal(t, tt.expected.Negative, got.Negative)
            // ... more assertions
        })
    }
}
```

### 7.2 Integration Tests

```go
// tailwind_test.go

package tailwind

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestGenerate(t *testing.T) {
    engine := New()

    tests := []struct {
        classes  []string
        contains []string
    }{
        {
            classes:  []string{"flex"},
            contains: []string{".flex { display: flex; }"},
        },
        {
            classes:  []string{"p-4"},
            contains: []string{".p-4 { padding: 1rem; }"},
        },
        {
            classes:  []string{"hover:bg-blue-500"},
            contains: []string{".hover\\:bg-blue-500:hover", "background-color:"},
        },
    }

    for _, tt := range tests {
        t.Run(strings.Join(tt.classes, " "), func(t *testing.T) {
            css := engine.Generate(tt.classes)
            for _, expected := range tt.contains {
                assert.Contains(t, css, expected)
            }
        })
    }
}

func BenchmarkGenerate(b *testing.B) {
    engine := New()
    classes := []string{
        "flex", "items-center", "justify-between",
        "p-4", "m-2", "bg-white", "rounded-lg",
        "hover:shadow-lg", "md:flex-row",
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        engine.Generate(classes)
    }
}

func BenchmarkGenerateCached(b *testing.B) {
    engine := New()
    classes := []string{"flex", "p-4", "bg-white"}

    // Warm up cache
    engine.Generate(classes)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        engine.Generate(classes)
    }
}
```

---

## 8. Files to Create

| File | Purpose | Lines (est.) |
|------|---------|--------------|
| `tailwind.go` | Engine, public API | 150 |
| `candidate.go` | Parsing logic | 300 |
| `candidate_test.go` | Parsing tests | 200 |
| `theme.go` | Theme system + defaults | 400 |
| `theme_test.go` | Theme tests | 100 |
| `cache.go` | Thread-safe cache | 80 |
| `cache_test.go` | Cache tests | 50 |
| `render.go` | CSS serialization | 150 |
| `render_test.go` | Render tests | 80 |
| `utilities/registry.go` | Utility registration | 100 |

**Total: ~1,600 lines**

---

## 9. Completion Criteria

Phase 1 is complete when:

1. ✅ Candidate parsing works for all class formats
2. ✅ Theme contains all Tailwind default values
3. ✅ Cache provides thread-safe storage
4. ✅ CSS rendering produces valid output
5. ✅ Registry is ready for utility registration
6. ✅ Engine can be instantiated and used
7. ✅ All tests pass
8. ✅ Benchmarks show cached lookups < 100ns

---

## 10. Next Steps

After Phase 1 is complete, proceed to any of Phases 2-7 to add utilities. The recommended order is:

1. **Phase 2: Static Utilities** - Foundation utilities like `flex`, `hidden`
2. **Phase 3: Spacing** - Most commonly used utilities
3. **Phase 6: Colors** - High visual impact

---

*Last Updated: 2024-12-12*
