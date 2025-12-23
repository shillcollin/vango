# Phase 9: Framework Integration

## Overview

Phase 9 provides integration examples and helpers for using vgo-tailwind with various Go web frameworks. While vgo-tailwind is framework-agnostic, these examples show best practices for Vango, Templ, and standard `html/template`.

**Prerequisites:** Phases 1-8 (Complete CSS engine)

**Files to create:**
- `examples/vango/` - Vango integration example
- `examples/templ/` - Templ integration example
- `examples/template/` - html/template integration example
- `collector.go` - Helper for collecting classes from HTML

---

## 1. Core Integration Pattern

All integrations follow the same pattern:

1. **Collect classes** from rendered HTML or component tree
2. **Generate CSS** using the engine
3. **Inject CSS** into the page

```go
// Basic usage pattern
engine := tailwind.New()

// Collect classes (framework-specific)
classes := collectClasses(...)

// Generate CSS
css := engine.Generate(classes)

// Inject CSS (framework-specific)
injectCSS(css)
```

---

## 2. Vango Integration

Vango's server-driven architecture makes integration seamless. Classes are collected during VNode tree rendering.

### 2.1 Collector Helper

```go
// collector.go

package tailwind

import "github.com/vango-dev/vango/v2/pkg/vdom"

// VNodeCollector collects Tailwind classes from a VNode tree.
type VNodeCollector struct {
    classes map[string]struct{}
    order   []string
}

// NewVNodeCollector creates a new collector.
func NewVNodeCollector() *VNodeCollector {
    return &VNodeCollector{
        classes: make(map[string]struct{}),
    }
}

// Collect walks the VNode tree and collects all class attributes.
func (c *VNodeCollector) Collect(node *vdom.VNode) {
    c.collectNode(node)
}

func (c *VNodeCollector) collectNode(node *vdom.VNode) {
    if node == nil {
        return
    }

    // Extract class from props
    if node.Props != nil {
        if class, ok := node.Props["class"].(string); ok {
            c.addClass(class)
        }
    }

    // Recurse into children
    for _, child := range node.Children {
        if childNode, ok := child.(*vdom.VNode); ok {
            c.collectNode(childNode)
        }
    }
}

func (c *VNodeCollector) addClass(classStr string) {
    for _, class := range strings.Fields(classStr) {
        if _, exists := c.classes[class]; !exists {
            c.classes[class] = struct{}{}
            c.order = append(c.order, class)
        }
    }
}

// Classes returns all collected classes in order.
func (c *VNodeCollector) Classes() []string {
    return append([]string{}, c.order...)
}
```

### 2.2 Vango Render Integration

```go
// examples/vango/integration.go

package vango_integration

import (
    "io"

    "github.com/vango-dev/vango/v2/pkg/render"
    "github.com/vango-dev/vango/v2/pkg/vdom"
    "github.com/vango-dev/vgo-tailwind"
)

// TailwindRenderer wraps Vango's renderer with Tailwind CSS generation.
type TailwindRenderer struct {
    renderer *render.Renderer
    engine   *tailwind.Engine
}

// NewTailwindRenderer creates a new renderer with Tailwind support.
func NewTailwindRenderer(config render.RendererConfig) *TailwindRenderer {
    return &TailwindRenderer{
        renderer: render.NewRenderer(config),
        engine:   tailwind.New(),
    }
}

// RenderPage renders a page with Tailwind CSS injected.
func (r *TailwindRenderer) RenderPage(w io.Writer, page render.PageData) error {
    // Collect classes from the body
    collector := tailwind.NewVNodeCollector()
    collector.Collect(page.Body)

    // Generate CSS
    css := r.engine.Generate(collector.Classes())

    // Inject CSS into page styles
    page.Styles = append(page.Styles, css)

    // Render the page
    return r.renderer.RenderPage(w, page)
}
```

### 2.3 Vango Usage Example

```go
// examples/vango/main.go

package main

import (
    "net/http"

    . "github.com/vango-dev/vango/v2/pkg/vdom"
    "github.com/vango-dev/vango/v2/pkg/render"
    vango_integration "github.com/vango-dev/vgo-tailwind/examples/vango"
)

var renderer = vango_integration.NewTailwindRenderer(render.RendererConfig{})

func HomePage() *VNode {
    return Div(
        Class("min-h-screen bg-gray-100"),
        Header(
            Class("bg-white shadow"),
            Div(
                Class("max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8"),
                H1(
                    Class("text-3xl font-bold text-gray-900"),
                    Text("Dashboard"),
                ),
            ),
        ),
        Main(
            Class("max-w-7xl mx-auto py-6 sm:px-6 lg:px-8"),
            Div(
                Class("px-4 py-6 sm:px-0"),
                Div(
                    Class("border-4 border-dashed border-gray-200 rounded-lg h-96 flex items-center justify-center"),
                    P(
                        Class("text-gray-500 text-lg"),
                        Text("Your content here"),
                    ),
                ),
            ),
        ),
    )
}

func handleHome(w http.ResponseWriter, r *http.Request) {
    page := render.PageData{
        Title: "Dashboard",
        Body:  HomePage(),
    }
    renderer.RenderPage(w, page)
}

func main() {
    http.HandleFunc("/", handleHome)
    http.ListenAndServe(":8080", nil)
}
```

---

## 3. Templ Integration

Templ uses code generation for templates. We can extract classes from the generated HTML.

### 3.1 HTML Class Extractor

```go
// collector.go

// HTMLCollector extracts Tailwind classes from HTML strings.
type HTMLCollector struct {
    classes map[string]struct{}
    order   []string
}

// NewHTMLCollector creates a new HTML collector.
func NewHTMLCollector() *HTMLCollector {
    return &HTMLCollector{
        classes: make(map[string]struct{}),
    }
}

// CollectFromHTML extracts classes from an HTML string.
func (c *HTMLCollector) CollectFromHTML(html string) {
    // Simple regex to extract class attributes
    // class="..." or class='...'
    re := regexp.MustCompile(`class=["']([^"']+)["']`)
    matches := re.FindAllStringSubmatch(html, -1)

    for _, match := range matches {
        if len(match) > 1 {
            c.addClass(match[1])
        }
    }
}

func (c *HTMLCollector) addClass(classStr string) {
    for _, class := range strings.Fields(classStr) {
        if _, exists := c.classes[class]; !exists {
            c.classes[class] = struct{}{}
            c.order = append(c.order, class)
        }
    }
}

// Classes returns all collected classes.
func (c *HTMLCollector) Classes() []string {
    return append([]string{}, c.order...)
}
```

### 3.2 Templ Usage Example

```go
// examples/templ/main.go

package main

import (
    "bytes"
    "context"
    "net/http"

    "github.com/vango-dev/vgo-tailwind"
)

var engine = tailwind.New()

// Templ component (in .templ file)
// templ HomePage() {
//     <div class="min-h-screen bg-gray-100">
//         <header class="bg-white shadow">
//             <div class="max-w-7xl mx-auto py-6 px-4">
//                 <h1 class="text-3xl font-bold text-gray-900">Dashboard</h1>
//             </div>
//         </header>
//     </div>
// }

func handleHome(w http.ResponseWriter, r *http.Request) {
    // Render Templ component to buffer first
    var buf bytes.Buffer
    HomePage().Render(context.Background(), &buf)
    html := buf.String()

    // Collect classes
    collector := tailwind.NewHTMLCollector()
    collector.CollectFromHTML(html)

    // Generate CSS
    css := engine.Generate(collector.Classes())

    // Write full page with CSS
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <style>` + css + `</style>
</head>
<body>
` + html + `
</body>
</html>`))
}
```

### 3.3 Templ with Middleware

```go
// examples/templ/middleware.go

package main

import (
    "bytes"
    "net/http"

    "github.com/vango-dev/vgo-tailwind"
)

// TailwindMiddleware injects Tailwind CSS into responses.
func TailwindMiddleware(engine *tailwind.Engine) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Capture response
            rec := &responseRecorder{
                ResponseWriter: w,
                body:           &bytes.Buffer{},
            }

            next.ServeHTTP(rec, r)

            // Extract classes from HTML
            html := rec.body.String()
            collector := tailwind.NewHTMLCollector()
            collector.CollectFromHTML(html)

            // Generate and inject CSS
            css := engine.Generate(collector.Classes())

            // Inject before </head>
            modified := strings.Replace(html,
                "</head>",
                "<style>"+css+"</style></head>",
                1,
            )

            w.Header().Set("Content-Length", strconv.Itoa(len(modified)))
            w.Write([]byte(modified))
        })
    }
}

type responseRecorder struct {
    http.ResponseWriter
    body *bytes.Buffer
}

func (r *responseRecorder) Write(b []byte) (int, error) {
    return r.body.Write(b)
}
```

---

## 4. html/template Integration

For standard Go templates, we extract classes from the rendered output.

### 4.1 Template Usage Example

```go
// examples/template/main.go

package main

import (
    "bytes"
    "html/template"
    "net/http"

    "github.com/vango-dev/vgo-tailwind"
)

var engine = tailwind.New()

var pageTemplate = template.Must(template.New("page").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <style>{{.CSS}}</style>
</head>
<body>
    <div class="min-h-screen bg-gray-100">
        <header class="bg-white shadow">
            <div class="max-w-7xl mx-auto py-6 px-4">
                <h1 class="text-3xl font-bold text-gray-900">{{.Title}}</h1>
            </div>
        </header>
        <main class="max-w-7xl mx-auto py-6 px-4">
            {{.Content}}
        </main>
    </div>
</body>
</html>
`))

var contentTemplate = template.Must(template.New("content").Parse(`
<div class="bg-white rounded-lg shadow p-6">
    <h2 class="text-xl font-semibold mb-4">Welcome</h2>
    <p class="text-gray-600">This is your dashboard.</p>
    <button class="mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
        Get Started
    </button>
</div>
`))

func handleHome(w http.ResponseWriter, r *http.Request) {
    // Render content first
    var contentBuf bytes.Buffer
    contentTemplate.Execute(&contentBuf, nil)
    content := contentBuf.String()

    // Collect classes from content
    collector := tailwind.NewHTMLCollector()
    collector.CollectFromHTML(content)

    // Also need to collect from page template structure
    // (hardcoded classes in the layout)
    collector.CollectFromHTML(`
        class="min-h-screen bg-gray-100"
        class="bg-white shadow"
        class="max-w-7xl mx-auto py-6 px-4"
        class="text-3xl font-bold text-gray-900"
    `)

    // Generate CSS
    css := engine.Generate(collector.Classes())

    // Render full page
    data := struct {
        Title   string
        CSS     template.CSS
        Content template.HTML
    }{
        Title:   "Dashboard",
        CSS:     template.CSS(css),
        Content: template.HTML(content),
    }

    pageTemplate.Execute(w, data)
}
```

---

## 5. Pre-warming the Cache

For production, you can pre-warm the cache at startup:

```go
// prewarm.go

package tailwind

import (
    "bufio"
    "os"
    "path/filepath"
    "regexp"
    "sync"
)

// PrewarmFromFiles scans Go files for Tailwind classes and pre-compiles them.
func (e *Engine) PrewarmFromFiles(patterns ...string) error {
    var allClasses []string
    classSet := make(map[string]struct{})

    for _, pattern := range patterns {
        files, err := filepath.Glob(pattern)
        if err != nil {
            return err
        }

        for _, file := range files {
            classes, err := extractClassesFromFile(file)
            if err != nil {
                continue
            }

            for _, class := range classes {
                if _, exists := classSet[class]; !exists {
                    classSet[class] = struct{}{}
                    allClasses = append(allClasses, class)
                }
            }
        }
    }

    // Pre-compile all classes in parallel
    var wg sync.WaitGroup
    for _, class := range allClasses {
        wg.Add(1)
        go func(c string) {
            defer wg.Done()
            e.compileClass(c) // Populates cache
        }(class)
    }
    wg.Wait()

    return nil
}

// extractClassesFromFile extracts Tailwind classes from a Go file.
func extractClassesFromFile(path string) ([]string, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var classes []string
    classSet := make(map[string]struct{})

    // Match Class("...") or class="..." patterns
    re := regexp.MustCompile(`(?:Class\(|class=)["']([^"']+)["']`)

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Text()
        matches := re.FindAllStringSubmatch(line, -1)
        for _, match := range matches {
            if len(match) > 1 {
                for _, class := range strings.Fields(match[1]) {
                    if _, exists := classSet[class]; !exists {
                        classSet[class] = struct{}{}
                        classes = append(classes, class)
                    }
                }
            }
        }
    }

    return classes, scanner.Err()
}
```

### 5.1 Usage

```go
func main() {
    engine := tailwind.New()

    // Pre-warm cache from source files
    err := engine.PrewarmFromFiles(
        "**/*.go",
        "**/*.templ",
    )
    if err != nil {
        log.Printf("Prewarm warning: %v", err)
    }

    // Start server...
}
```

---

## 6. Development Mode Features

```go
// dev.go

package tailwind

import (
    "fmt"
    "log"
)

// DevEngine wraps Engine with development features.
type DevEngine struct {
    *Engine
    warnUnknown bool
    logClasses  bool
}

// NewDevEngine creates an engine with development features enabled.
func NewDevEngine() *DevEngine {
    return &DevEngine{
        Engine:      New(),
        warnUnknown: true,
        logClasses:  true,
    }
}

// Generate with development logging.
func (e *DevEngine) Generate(classes []string) string {
    if e.logClasses {
        log.Printf("[tailwind] Generating CSS for %d classes", len(classes))
    }

    var unknownClasses []string
    for _, class := range classes {
        if e.cache.IsInvalid(class) {
            unknownClasses = append(unknownClasses, class)
        }
    }

    css := e.Engine.Generate(classes)

    if e.warnUnknown && len(unknownClasses) > 0 {
        for _, class := range unknownClasses {
            log.Printf("[tailwind] Warning: unknown class '%s'", class)
        }
    }

    return css
}

// Stats returns cache statistics.
func (e *DevEngine) Stats() string {
    stats := e.cache.Stats()
    return fmt.Sprintf("Cache: %d valid rules, %d invalid classes",
        stats.ValidRules, stats.InvalidClasses)
}
```

---

## 7. Testing Integration

```go
// integration_test.go

func TestVangoIntegration(t *testing.T) {
    renderer := vango_integration.NewTailwindRenderer(render.RendererConfig{})

    body := Div(
        Class("flex items-center p-4 bg-white"),
        H1(Class("text-xl font-bold"), Text("Hello")),
    )

    page := render.PageData{
        Title: "Test",
        Body:  body,
    }

    var buf bytes.Buffer
    err := renderer.RenderPage(&buf, page)
    assert.NoError(t, err)

    html := buf.String()
    assert.Contains(t, html, "<style>")
    assert.Contains(t, html, ".flex")
    assert.Contains(t, html, "display: flex")
}

func TestHTMLCollector(t *testing.T) {
    html := `
        <div class="flex items-center">
            <span class="text-lg font-bold">Hello</span>
        </div>
    `

    collector := tailwind.NewHTMLCollector()
    collector.CollectFromHTML(html)

    classes := collector.Classes()
    assert.Contains(t, classes, "flex")
    assert.Contains(t, classes, "items-center")
    assert.Contains(t, classes, "text-lg")
    assert.Contains(t, classes, "font-bold")
}

func TestPrewarm(t *testing.T) {
    engine := tailwind.New()

    // Create test file
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.go")
    os.WriteFile(testFile, []byte(`
        package test
        func Component() {
            Class("flex items-center p-4")
        }
    `), 0644)

    err := engine.PrewarmFromFiles(filepath.Join(tmpDir, "*.go"))
    assert.NoError(t, err)

    // Check cache is populated
    stats := engine.cache.Stats()
    assert.Greater(t, stats.ValidRules, 0)
}
```

---

## 8. Complete Example Project Structure

```
my-vango-app/
├── main.go
├── go.mod
├── components/
│   ├── layout.go      # Layout components with Tailwind classes
│   ├── header.go
│   └── card.go
├── pages/
│   ├── home.go
│   └── about.go
└── tailwind/
    └── setup.go       # Tailwind engine setup
```

### 8.1 setup.go

```go
// tailwind/setup.go

package tailwind

import (
    tw "github.com/vango-dev/vgo-tailwind"
    "log"
    "os"
)

var Engine *tw.Engine

func init() {
    Engine = tw.New()

    // In production, pre-warm the cache
    if os.Getenv("GO_ENV") == "production" {
        if err := Engine.PrewarmFromFiles("**/*.go"); err != nil {
            log.Printf("Tailwind prewarm warning: %v", err)
        }
    }
}
```

---

## 9. Files to Create

| File | Purpose | Lines (est.) |
|------|---------|--------------|
| `collector.go` | VNode and HTML collectors | 150 |
| `collector_test.go` | Collector tests | 100 |
| `prewarm.go` | Cache pre-warming | 80 |
| `dev.go` | Development mode helpers | 60 |
| `examples/vango/integration.go` | Vango integration | 100 |
| `examples/vango/main.go` | Vango example | 80 |
| `examples/templ/main.go` | Templ example | 100 |
| `examples/template/main.go` | html/template example | 100 |

---

## 10. Completion Criteria

Phase 9 is complete when:

1. ✅ VNode collector works with Vango components
2. ✅ HTML collector extracts classes from HTML strings
3. ✅ Vango integration example works end-to-end
4. ✅ Templ integration example works
5. ✅ html/template integration example works
6. ✅ Cache pre-warming works
7. ✅ Development mode logging works
8. ✅ All integration tests pass
9. ✅ Example projects can be run and produce correct CSS

---

## 11. Final Project Verification

After completing Phase 9, verify the entire project:

```bash
# Run all tests
go test ./...

# Run benchmarks
go test -bench=. ./...

# Try the example projects
cd examples/vango && go run .
cd examples/templ && go run .
cd examples/template && go run .

# Check for race conditions
go test -race ./...
```

---

*Last Updated: 2024-12-12*
