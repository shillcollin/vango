package ui

import (
	"strings"

	"github.com/vango-dev/vango/v2/pkg/features/hooks/standard"
	"github.com/vango-dev/vango/v2/pkg/vdom"
)

// --- Kanban Board ---

type KanbanBoardConfig struct {
	BaseConfig
}

func (c *KanbanBoardConfig) GetBase() *BaseConfig { return &c.BaseConfig }

type KanbanBoardOption = Option[*KanbanBoardConfig]

func KanbanBoard(opts ...KanbanBoardOption) *vdom.VNode {
	c := &KanbanBoardConfig{}
	for _, opt := range opts {
		opt(c)
	}

	finalClass := CN(
		"flex h-full w-full gap-4 overflow-x-auto p-4 bg-muted/20",
		strings.Join(c.BaseConfig.Classes, " "),
	)

	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+1)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	renderOpts = append(renderOpts, c.BaseConfig.Options...)

	return vdom.Div(renderOpts...)
}

// --- Kanban Column ---

type KanbanColumnConfig struct {
	BaseConfig
	ID    string
	Title string
}

func (c *KanbanColumnConfig) GetBase() *BaseConfig { return &c.BaseConfig }

type KanbanColumnOption = Option[*KanbanColumnConfig]

func KanbanColumnID(id string) KanbanColumnOption {
	return func(c *KanbanColumnConfig) { c.ID = id }
}

func KanbanColumnTitle(title string) KanbanColumnOption {
	return func(c *KanbanColumnConfig) { c.Title = title }
}

func KanbanColumn(opts ...KanbanColumnOption) *vdom.VNode {
	c := &KanbanColumnConfig{}
	for _, opt := range opts {
		opt(c)
	}

	finalClass := CN(
		"flex w-80 shrink-0 flex-col rounded-lg bg-secondary",
		strings.Join(c.BaseConfig.Classes, " "),
	)

	// Sortable configuration for the column
	sortable := standard.Sortable(standard.SortableConfig{
		Group:      "kanban", // Shared group allows moving between columns
		Animation:  150,
		GhostClass: "opacity-50",
	})

	// Create data attributes for identification
	dataID := vdom.Attr{Key: "data-column-id", Value: c.ID}

	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+5)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	renderOpts = append(renderOpts, dataID)

	// Render Header
	if c.Title != "" {
		renderOpts = append(renderOpts, vdom.H3(
			vdom.Class("p-4 font-semibold text-secondary-foreground"),
			vdom.Text(c.Title),
		))
	}

	// Content container needs the sortable hook
	contentOpts := []any{
		vdom.Class("flex flex-col gap-2 p-4 pt-0 min-h-[50px]"), // min-h ensures drop target exists when empty
		sortable,
	}
	contentOpts = append(contentOpts, c.BaseConfig.Options...)

	renderOpts = append(renderOpts, vdom.Div(contentOpts...))

	return vdom.Div(renderOpts...)
}

// --- Kanban Card ---

type KanbanCardConfig struct {
	BaseConfig
	ID string
}

func (c *KanbanCardConfig) GetBase() *BaseConfig { return &c.BaseConfig }

type KanbanCardOption = Option[*KanbanCardConfig]

func KanbanCardID(id string) KanbanCardOption {
	return func(c *KanbanCardConfig) { c.ID = id }
}

func KanbanCard(opts ...KanbanCardOption) *vdom.VNode {
	c := &KanbanCardConfig{}
	for _, opt := range opts {
		opt(c)
	}

	finalClass := CN(
		"cursor-grab rounded border bg-card p-3 text-card-foreground shadow-sm hover:ring-2 hover:ring-primary/50",
		strings.Join(c.BaseConfig.Classes, " "),
	)

	dataID := vdom.Attr{Key: "data-id", Value: c.ID}

	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+2)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	if c.ID != "" {
		renderOpts = append(renderOpts, dataID)
	}
	renderOpts = append(renderOpts, c.BaseConfig.Options...)

	return vdom.Div(renderOpts...)
}
