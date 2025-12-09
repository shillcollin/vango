package app

import (
	"context"

	"webdemo-kanban/pkg/db"

	"github.com/vango-dev/vango/v2/pkg/vango"
	. "github.com/vango-dev/vango/v2/pkg/vdom"
)

// DashboardComponent displays a list of user's boards.
type DashboardComponent struct {
	pool   *db.Pool
	userID string

	boards     *vango.Signal[[]db.Board]
	loading    *vango.Signal[bool]
	newTitle   *vango.Signal[string]
	showCreate *vango.Signal[bool]
	errorMsg   *vango.Signal[string]
}

// NewDashboard creates the dashboard component.
func NewDashboard(pool *db.Pool, userID string) *DashboardComponent {
	d := &DashboardComponent{
		pool:       pool,
		userID:     userID,
		boards:     vango.NewSignal([]db.Board{}),
		loading:    vango.NewSignal(true),
		newTitle:   vango.NewSignal(""),
		showCreate: vango.NewSignal(false),
		errorMsg:   vango.NewSignal(""),
	}

	// Load boards
	go d.loadBoards()

	return d
}

func (d *DashboardComponent) loadBoards() {
	d.loading.Set(true)
	defer d.loading.Set(false)

	// Handle demo mode (no database)
	if d.pool == nil {
		d.boards.Set([]db.Board{
			{ID: "demo-1", Title: "Demo Board", OwnerID: d.userID},
		})
		return
	}

	boards, err := d.pool.GetBoards(context.Background(), d.userID)
	if err != nil {
		d.errorMsg.Set(err.Error())
		return
	}
	d.boards.Set(boards)
}

func (d *DashboardComponent) handleCreate() {
	title := d.newTitle.Get()
	if title == "" {
		return
	}

	// Demo mode
	if d.pool == nil {
		d.boards.Update(func(b []db.Board) []db.Board {
			return append([]db.Board{{ID: "demo-new", Title: title, OwnerID: d.userID}}, b...)
		})
		d.newTitle.Set("")
		d.showCreate.Set(false)
		return
	}

	board, _, err := d.pool.CreateBoard(context.Background(), title, d.userID)
	if err != nil {
		d.errorMsg.Set(err.Error())
		return
	}

	d.boards.Update(func(b []db.Board) []db.Board {
		return append([]db.Board{*board}, b...)
	})
	d.newTitle.Set("")
	d.showCreate.Set(false)
}

// Render implements server.Component.
func (d *DashboardComponent) Render() *VNode {
	if d.loading.Get() {
		return Layout("My Boards",
			Div(Class("loading-state"),
				Div(Class("spinner")),
				P(Text("Loading boards...")),
			),
		)
	}

	boards := d.boards.Get()
	showCreate := d.showCreate.Get()

	return Layout("My Boards",
		Div(Class("dashboard"),
			// Header
			Div(Class("dashboard-header"),
				H1(Text("My Boards")),
				Button(
					Class("btn btn-primary"),
					OnClick(func() { d.showCreate.Set(!showCreate) }),
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
						Value(d.newTitle.Get()),
						OnInput(func(v string) { d.newTitle.Set(v) }),
					),
					Div(Class("form-actions"),
						Button(
							Class("btn btn-primary"),
							OnClick(d.handleCreate),
							Text("Create"),
						),
						Button(
							Class("btn btn-ghost"),
							OnClick(func() { d.showCreate.Set(false) }),
							Text("Cancel"),
						),
					),
				),
			),

			// Error message
			If(d.errorMsg.Get() != "",
				Div(Class("error-banner"),
					Text(d.errorMsg.Get()),
					Button(
						Class("btn-icon"),
						OnClick(func() { d.errorMsg.Set("") }),
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
					return A(
						Href("/board/"+board.ID),
						Class("board-card"),
						Key(board.ID),
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
