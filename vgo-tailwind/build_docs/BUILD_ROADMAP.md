# vgo-tailwind Build Roadmap

## Overview

vgo-tailwind is a pure Go implementation of Tailwind CSS. It provides runtime CSS generation with aggressive caching, eliminating the need for Node.js, npm, or a separate build step.

While designed with Vango in mind, vgo-tailwind is a **standalone package** that works with any Go web framework or templating system.

**Repository:** `/Users/collinshill/Documents/vango/vgo-tailwind/`

---

## Architecture Summary

### Core Innovation

Unlike traditional Tailwind (which requires a Node.js build step), vgo-tailwind generates CSS at runtime:

```
Class Strings → Parser → CSS Engine → Cache → CSS Output
```

The cache persists for the process lifetime, so after the first use of any class, subsequent uses have near-zero overhead.

### Key Design Decisions

1. **String-based API**: `Generate([]string{"flex", "items-center", "p-4"})` - matches Tailwind syntax exactly
2. **Runtime generation**: CSS generated on first use, then cached forever
3. **Framework agnostic**: Works with Vango, Templ, html/template, or any Go code
4. **Zero external dependencies**: Pure Go, single binary deployment

---

## Package Structure

```
vgo-tailwind/
├── build_docs/                # Build documentation
│   ├── BUILD_ROADMAP.md       # Overview (this file)
│   ├── PHASE_01_CORE.md       # Core infrastructure
│   ├── PHASE_02_STATIC.md     # Static utilities
│   ├── PHASE_03_SPACING.md    # Spacing utilities
│   ├── PHASE_04_SIZING.md     # Sizing utilities
│   ├── PHASE_05_TYPOGRAPHY.md # Typography utilities
│   ├── PHASE_06_COLORS.md     # Color utilities
│   ├── PHASE_07_BORDERS.md    # Borders & effects
│   ├── PHASE_08_VARIANTS.md   # Variant system
│   └── PHASE_09_INTEGRATION.md # Framework integration examples
├── tailwind.go                # Engine type, public API
├── tailwind_test.go
├── candidate.go               # Class string parsing
├── candidate_test.go
├── variant.go                 # Variant system
├── variant_test.go
├── theme.go                   # Design tokens
├── theme_test.go
├── cache.go                   # Thread-safe CSS cache
├── cache_test.go
├── render.go                  # CSS serialization
├── render_test.go
└── utilities/
    ├── registry.go            # Utility registration
    ├── static.go              # Static utilities
    ├── layout.go              # Display, position, flex, grid
    ├── spacing.go             # Padding, margin, gap
    ├── sizing.go              # Width, height
    ├── typography.go          # Font, text, leading
    ├── colors.go              # Background, text, border colors
    ├── borders.go             # Border width, radius
    ├── effects.go             # Shadow, opacity
    └── utilities_test.go
```

---

## Usage Examples

### Standalone Usage
```go
import "github.com/vango-dev/vgo-tailwind"

func main() {
    engine := tailwind.New()

    css := engine.Generate([]string{
        "flex",
        "items-center",
        "p-4",
        "hover:bg-blue-500",
        "md:flex-row",
    })

    fmt.Println(css)
    // .flex { display: flex; }
    // .items-center { align-items: center; }
    // .p-4 { padding: 1rem; }
    // @media (hover: hover) { .hover\:bg-blue-500:hover { background-color: ... } }
    // @media (min-width: 768px) { .md\:flex-row { flex-direction: row; } }
}
```

### With Vango
```go
// In your Vango app
import (
    "github.com/vango-dev/vango/v2/pkg/vdom"
    "github.com/vango-dev/vgo-tailwind"
)

var tw = tailwind.New()

func Card(title string) *vdom.VNode {
    return Div(
        Class("bg-white rounded-lg shadow-md p-6 hover:shadow-lg"),
        H2(Class("text-xl font-bold text-gray-900"), Text(title)),
    )
}

// In your render setup, collect classes and inject CSS
func renderPage(page PageData) {
    classes := collectClassesFromVNode(page.Body)
    css := tw.Generate(classes)
    page.Styles = append(page.Styles, css)
}
```

### With Templ
```go
import "github.com/vango-dev/vgo-tailwind"

var tw = tailwind.New()

templ Card(title string) {
    <div class="bg-white rounded-lg shadow-md p-6">
        <h2 class="text-xl font-bold">{ title }</h2>
    </div>
}

// Extract classes from rendered HTML and generate CSS
```

---

## Phase Dependencies

```
Phase 1: Core Infrastructure
    │
    ├── Phase 2: Static Utilities ─────────────────────┐
    │                                                   │
    ├── Phase 3: Spacing Utilities ────────────────────┤
    │                                                   │
    ├── Phase 4: Sizing Utilities ─────────────────────┤
    │                                                   │
    ├── Phase 5: Typography ───────────────────────────┤
    │                                                   │
    ├── Phase 6: Colors ───────────────────────────────┤
    │                                                   │
    └── Phase 7: Borders & Effects ────────────────────┤
                                                        │
                        Phase 8: Variants ◄─────────────┘
                            │
                            ▼
                    Phase 9: Framework Integration Examples
```

**Notes:**
- Phase 1 must be completed first (provides parsing, caching, theme)
- Phases 2-7 can be implemented in any order (all depend only on Phase 1)
- Phase 8 requires all utilities to be defined (wraps them with selectors/media queries)
- Phase 9 provides integration examples for Vango, Templ, and others

---

## Phase Summaries

### Phase 1: Core Infrastructure
**Files:** `tailwind.go`, `candidate.go`, `theme.go`, `cache.go`, `render.go`

- Candidate parsing (class string → structured data)
- Theme system with Tailwind defaults
- Thread-safe CSS cache
- CSS serialization
- Engine type with public API

### Phase 2: Static Utilities (~100 classes)
**Files:** `utilities/registry.go`, `utilities/static.go`, `utilities/layout.go`

- Display: `flex`, `grid`, `block`, `hidden`, `inline-flex`
- Position: `static`, `relative`, `absolute`, `fixed`, `sticky`
- Flex: `flex-row`, `flex-col`, `flex-wrap`, `flex-nowrap`
- Justify: `justify-start`, `justify-center`, `justify-between`, etc.
- Align: `items-start`, `items-center`, `items-stretch`, etc.

### Phase 3: Spacing Utilities
**Files:** `utilities/spacing.go`

- Padding: `p-*`, `px-*`, `py-*`, `pt-*`, `pr-*`, `pb-*`, `pl-*`
- Margin: `m-*`, `mx-*`, `my-*`, `mt-*`, `mr-*`, `mb-*`, `ml-*`
- Gap: `gap-*`, `gap-x-*`, `gap-y-*`
- Space between: `space-x-*`, `space-y-*`
- Arbitrary values: `p-[17px]`, `m-[2rem]`
- Negative values: `-m-4`, `-mt-2`

### Phase 4: Sizing Utilities
**Files:** `utilities/sizing.go`

- Width: `w-*`, `min-w-*`, `max-w-*`
- Height: `h-*`, `min-h-*`, `max-h-*`
- Size: `size-*` (sets both width and height)
- Fractions: `w-1/2`, `w-2/3`, `h-1/4`
- Special: `w-full`, `w-screen`, `w-auto`, `w-fit`

### Phase 5: Typography
**Files:** `utilities/typography.go`

- Font size: `text-xs`, `text-sm`, `text-base`, `text-lg`, `text-xl`, etc.
- Font weight: `font-thin`, `font-light`, `font-normal`, `font-bold`, etc.
- Font family: `font-sans`, `font-serif`, `font-mono`
- Text alignment: `text-left`, `text-center`, `text-right`, `text-justify`
- Line height: `leading-none`, `leading-tight`, `leading-normal`, `leading-relaxed`
- Letter spacing: `tracking-tighter`, `tracking-normal`, `tracking-wide`
- Text decoration: `underline`, `line-through`, `no-underline`
- Text transform: `uppercase`, `lowercase`, `capitalize`, `normal-case`

### Phase 6: Colors
**Files:** `utilities/colors.go`

- Text color: `text-red-500`, `text-blue-600`, `text-[#ff0000]`
- Background: `bg-white`, `bg-gray-100`, `bg-[rgb(0,0,255)]`
- Opacity modifier: `text-red-500/50`, `bg-blue-500/75`
- Special values: `text-inherit`, `text-current`, `text-transparent`
- Full Tailwind palette: gray, red, orange, amber, yellow, lime, green, emerald, teal, cyan, sky, blue, indigo, violet, purple, fuchsia, pink, rose (each with 50-950 shades)

### Phase 7: Borders & Effects
**Files:** `utilities/borders.go`, `utilities/effects.go`

- Border width: `border`, `border-0`, `border-2`, `border-t`, `border-r-4`
- Border style: `border-solid`, `border-dashed`, `border-dotted`, `border-none`
- Border radius: `rounded`, `rounded-sm`, `rounded-lg`, `rounded-full`, `rounded-t-lg`
- Border color: `border-gray-300`, `border-red-500`
- Box shadow: `shadow-sm`, `shadow`, `shadow-md`, `shadow-lg`, `shadow-xl`
- Opacity: `opacity-0`, `opacity-25`, `opacity-50`, `opacity-75`, `opacity-100`
- Ring: `ring`, `ring-2`, `ring-blue-500`

### Phase 8: Variants
**Files:** `variant.go`

- Pseudo-class: `hover:`, `focus:`, `active:`, `disabled:`, `visited:`
- Pseudo-element: `before:`, `after:`, `placeholder:`
- Responsive: `sm:`, `md:`, `lg:`, `xl:`, `2xl:`
- Dark mode: `dark:`
- State: `first:`, `last:`, `odd:`, `even:`, `empty:`
- Group: `group-hover:`, `group-focus:`
- Peer: `peer-checked:`, `peer-focus:`
- Arbitrary: `[&:nth-child(3)]:`

### Phase 9: Framework Integration Examples
**Files:** `examples/` directory

- Vango integration example
- Templ integration example
- html/template integration example
- Documentation for custom integrations

---

## Reference Materials

### Tailwind Source Code (cloned to /Users/collinshill/Documents/vango/tailwind)

| Purpose | File |
|---------|------|
| All utilities | `packages/tailwindcss/src/utilities.ts` (6,430 lines) |
| Parsing logic | `packages/tailwindcss/src/candidate.ts` |
| Variant system | `packages/tailwindcss/src/variants.ts` |
| Theme values | `packages/tailwindcss/theme.css` |
| Design system | `packages/tailwindcss/src/design-system.ts` |

---

## Testing Strategy

Each phase should include comprehensive tests:

1. **Unit tests** for individual functions
2. **Integration tests** for the full parsing → compilation → output cycle
3. **Snapshot tests** comparing output to Tailwind's output
4. **Benchmark tests** for cache performance

Example test pattern:
```go
func TestPaddingUtility(t *testing.T) {
    engine := tailwind.New()

    tests := []struct {
        input    string
        expected string
    }{
        {"p-4", ".p-4 { padding: 1rem; }"},
        {"px-2", ".px-2 { padding-left: 0.5rem; padding-right: 0.5rem; }"},
        {"p-[17px]", ".p-\\[17px\\] { padding: 17px; }"},
    }

    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            css := engine.Generate([]string{tt.input})
            assert.Contains(t, css, tt.expected)
        })
    }
}
```

---

## Success Criteria

The project is complete when:

1. All Tailwind core utilities work with string-based API
2. All variants work (hover:, sm:, dark:, etc.)
3. Arbitrary values work (`w-[137px]`, `text-[#ff0000]`)
4. Opacity modifiers work (`bg-red-500/50`)
5. CSS output matches Tailwind's output
6. Cache provides near-zero overhead after first use
7. No Node.js or external dependencies required
8. Works seamlessly with Vango, Templ, and other Go frameworks

---

## Getting Started

To work on a specific phase:

1. Read the phase document thoroughly (e.g., `PHASE_01_CORE.md`)
2. Reference the corresponding Tailwind source files
3. Implement with tests
4. Verify against Tailwind's output
5. Update the phase document with any learnings

---

*Last Updated: 2024-12-12*
