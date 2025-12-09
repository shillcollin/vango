package app

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"webdemo-kanban/pkg/db"
	"webdemo-kanban/pkg/hub"

	"github.com/vango-dev/vango/v2/pkg/features/hooks"
	"github.com/vango-dev/vango/v2/pkg/server"
	"github.com/vango-dev/vango/v2/pkg/vango"
	. "github.com/vango-dev/vango/v2/pkg/vdom"
)

// RootComponent is the top-level app component with simple routing.
type RootComponent struct {
	pool *db.Pool
	hub  *hub.Hub

	// Current route state
	path *vango.Signal[string]

	// Demo user ID (in production, this would come from auth)
	userID string

	// Dashboard state (cached to avoid re-creation)
	boards     *vango.Signal[[]db.Board]
	loading    *vango.Signal[bool]
	newTitle   *vango.Signal[string]
	showCreate *vango.Signal[bool]
	errorMsg   *vango.Signal[string]

	// Ensure we only load once
	loadOnce sync.Once
}

// Root creates the root component with the given initial path.
func Root(pool *db.Pool, h *hub.Hub, initialPath string) server.Component {
	if initialPath == "" {
		initialPath = "/"
	}

	// Demo mode: initialize with data synchronously
	if pool == nil {
		return &RootComponent{
			pool:       nil,
			hub:        h,
			path:       vango.NewSignal(initialPath),
			userID:     "demo-user-001",
			boards:     vango.NewSignal([]db.Board{{ID: "demo-1", Title: "Demo Board", OwnerID: "demo-user-001"}}),
			loading:    vango.NewSignal(false), // Already loaded
			newTitle:   vango.NewSignal(""),
			showCreate: vango.NewSignal(false),
			errorMsg:   vango.NewSignal(""),
		}
	}

	// With database: start loading async
	r := &RootComponent{
		pool:       pool,
		hub:        h,
		path:       vango.NewSignal(initialPath),
		userID:     "demo-user-001",
		boards:     vango.NewSignal([]db.Board{}),
		loading:    vango.NewSignal(true),
		newTitle:   vango.NewSignal(""),
		showCreate: vango.NewSignal(false),
		errorMsg:   vango.NewSignal(""),
	}

	go r.loadBoards()

	return r
}

func (r *RootComponent) loadBoards() {
	// Handle demo mode (no database)
	if r.pool == nil {
		r.boards.Set([]db.Board{
			{ID: "demo-1", Title: "Demo Board", OwnerID: r.userID},
		})
		r.loading.Set(false)
		return
	}

	boards, err := r.pool.GetBoards(context.Background(), r.userID)
	if err != nil {
		r.errorMsg.Set(err.Error())
		r.loading.Set(false)
		return
	}
	r.boards.Set(boards)
	r.loading.Set(false)
}

func (r *RootComponent) handleCreate() {
	title := r.newTitle.Get()
	if title == "" {
		return
	}

	// Demo mode
	if r.pool == nil {
		r.boards.Update(func(b []db.Board) []db.Board {
			return append([]db.Board{{ID: "demo-new", Title: title, OwnerID: r.userID}}, b...)
		})
		r.newTitle.Set("")
		r.showCreate.Set(false)
		return
	}

	board, _, err := r.pool.CreateBoard(context.Background(), title, r.userID)
	if err != nil {
		r.errorMsg.Set(err.Error())
		return
	}

	r.boards.Update(func(b []db.Board) []db.Board {
		return append([]db.Board{*board}, b...)
	})
	r.newTitle.Set("")
	r.showCreate.Set(false)
}

// navigate handles client-side navigation
func (r *RootComponent) navigate(path string) {
	r.path.Set(path)
}

// Render implements server.Component.
func (r *RootComponent) Render() *VNode {
	path := r.path.Get()
	loading := r.loading.Get()
	boardCount := len(r.boards.Get())
	log.Printf("[DEBUG] Render: path=%s, loading=%v, boards=%d", path, loading, boardCount)

	// Simple routing
	switch {
	case path == "/" || path == "":
		return r.renderDashboard()

	case strings.HasPrefix(path, "/board/"):
		boardID := strings.TrimPrefix(path, "/board/")
		model, err := r.hub.GetBoard(context.Background(), boardID)
		if err != nil {
			return r.renderLayout("Error",
				Div(Class("error-state"),
					H1(Text("Board not found")),
					P(Text(err.Error())),
					Button(
						Class("btn btn-primary"),
						OnClick(func() { r.navigate("/") }),
						Text("Back to boards"),
					),
				),
			)
		}
		return r.renderBoard(model)

	default:
		return r.renderLayout("404",
			Div(Class("error-state"),
				H1(Text("Page not found")),
				Button(
					Class("btn btn-primary"),
					OnClick(func() { r.navigate("/") }),
					Text("Go home"),
				),
			),
		)
	}
}

// renderLayout wraps content in the app shell
func (r *RootComponent) renderLayout(title string, children ...*VNode) *VNode {
	return Div(Class("app-shell"),
		// Header
		Header(Class("header"),
			Div(Class("header-brand"),
				Span(Class("logo"), Text("📋")),
				Span(Class("app-name"), Text("Kanban")),
			),
			Nav(Class("header-nav"),
				Button(
					Class("nav-link"),
					OnClick(func() { r.navigate("/") }),
					Text("Boards"),
				),
			),
		),

		// Main content
		Main(Class("main-content"),
			children,
		),
	)
}

func (r *RootComponent) renderDashboard() *VNode {
	if r.loading.Get() {
		return r.renderLayout("My Boards",
			Div(Class("loading-state"),
				Div(Class("spinner")),
				P(Text("Loading boards...")),
			),
		)
	}

	boards := r.boards.Get()
	showCreate := r.showCreate.Get()

	return r.renderLayout("My Boards",
		Div(Class("dashboard"),
			// Header
			Div(Class("dashboard-header"),
				H1(Text("My Boards")),
				Button(
					Class("btn btn-primary"),
					OnClick(func() { r.showCreate.Set(!showCreate) }),
					Text("+ New Board"),
				),
			),

			// Create form
			If(showCreate,
				Div(Class("create-board-form"),
					Input(
						Type("text"),
						Class("input"),
						Placeholder("Board title..."),
						Value(r.newTitle.Get()),
						OnInput(func(v string) { r.newTitle.Set(v) }),
					),
					Div(Class("form-actions"),
						Button(
							Class("btn btn-primary"),
							OnClick(r.handleCreate),
							Text("Create"),
						),
						Button(
							Class("btn btn-ghost"),
							OnClick(func() { r.showCreate.Set(false) }),
							Text("Cancel"),
						),
					),
				),
			),

			// Error message
			If(r.errorMsg.Get() != "",
				Div(Class("error-banner"),
					Text(r.errorMsg.Get()),
					Button(
						Class("btn-icon"),
						OnClick(func() { r.errorMsg.Set("") }),
						Text("×"),
					),
				),
			),

			// Boards grid
			If(len(boards) == 0,
				Div(Class("empty-state"),
					H2(Text("No boards yet")),
					P(Text("Create your first board to get started.")),
				),
			),

			Div(Class("boards-grid"),
				Range(boards, func(board db.Board, _ int) *VNode {
					return Div(
						Class("board-card"),
						Key(board.ID),
						OnClick(func() { r.navigate("/board/" + board.ID) }),
						Div(Class("board-card-title"), Text(board.Title)),
						Div(Class("board-card-meta"),
							Text(board.CreatedAt.Format("Jan 2, 2006")),
						),
					)
				}),
			),
		),
	)
}

func (r *RootComponent) renderBoard(model *hub.BoardModel) *VNode {
	columns := model.Columns.Get()
	cards := model.Cards.Get()

	return r.renderLayout(model.Title,
		Div(Class("board"),
			// Board header
			Div(Class("board-header"),
				Button(
					Class("btn btn-ghost"),
					OnClick(func() { r.navigate("/") }),
					Text("← Back"),
				),
				H1(Text(model.Title)),
			),

			// Columns container
			Div(Class("board-columns"),
				Range(columns, func(col db.Column, _ int) *VNode {
					colCards := cards[col.ID]

					return Div(
						Class("column"),
						Key(col.ID),

						// Column header
						Div(Class("column-header"),
							H3(Text(col.Title)),
							Span(Class("card-count"), Textf("%d", len(colCards))),
						),

						// Cards container with Sortable for drag and drop
						Div(
							Class("cards-container"),
							DataAttr("id", col.ID), // Needed for cross-column drag to identify container
							hooks.Hook("Sortable", map[string]any{
								"group":      "cards",
								"animation":  150,
								"ghostClass": "card-ghost",
							}),
							hooks.OnEvent("onreorder", func(e hooks.HookEvent) {
								cardID := e.String("id")
								fromCol := e.String("fromContainer")
								toCol := e.String("toContainer")
								toIndex := e.Int("toIndex")

								log.Printf("[MoveCard] %s: %s -> %s [%d]", cardID, fromCol, toCol, toIndex)

								if cardID != "" && fromCol != "" && toCol != "" {
									model.MoveCard(cardID, fromCol, toCol, toIndex)
								}
							}),
							Range(colCards, func(card db.Card, _ int) *VNode {
								return Div(
									Class("card"),
									Key(card.ID),
									DataAttr("id", card.ID), // For identification
									Div(Class("card-content"), Text(card.Content)),
									Div(Class("card-actions"),
										Button(
											Class("btn-icon btn-danger"),
											OnClick(func() { model.DeleteCard(card.ID, col.ID) }),
											Text("🗑"),
										),
									),
								)
							}),
							// Add card button
							Div(
								Class("add-card"),
								Button(
									Class("btn-ghost btn-sm"),
									Text("+ Add Card"),
									OnClick(func() {
										// Add a new card to this column
										model.AddCard(col.ID, fmt.Sprintf("New Card in %s", col.Title))
									}),
								),
							),
						),
					)
				}),
			),
		),
	)
}

func (r *RootComponent) renderAddCardButton(model *hub.BoardModel, columnID string) *VNode {
	// Simple add card - in production you'd have inline form
	return Button(
		Class("add-card-btn"),
		OnClick(func() {
			model.AddCard(columnID, "New card")
		}),
		Text("+ Add card"),
	)
}
