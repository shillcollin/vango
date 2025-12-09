// Package app contains UI components for the Kanban board.
package app

import (
	. "github.com/vango-dev/vango/v2/pkg/vdom"
)

// Layout wraps page content with a consistent shell.
func Layout(title string, children ...*VNode) *VNode {
	return Div(Class("app-shell"),
		// Header
		Header(Class("header"),
			Div(Class("header-brand"),
				Span(Class("logo"), Text("📋")),
				Span(Class("app-name"), Text("Kanban")),
			),
			Nav(Class("header-nav"),
				A(Href("/"), Class("nav-link"), Text("Boards")),
			),
		),

		// Main content
		Main(Class("main-content"),
			children,
		),
	)
}
