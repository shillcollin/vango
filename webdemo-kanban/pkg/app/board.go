package app

import (
	"fmt"

	"webdemo-kanban/pkg/db"
	"webdemo-kanban/pkg/hub"

	"github.com/vango-dev/vango/v2/pkg/features/hooks"
	"github.com/vango-dev/vango/v2/pkg/vango"
	. "github.com/vango-dev/vango/v2/pkg/vdom"
)

// BoardComponent displays the Kanban board with drag-and-drop.
type BoardComponent struct {
	model *hub.BoardModel
	pool  *db.Pool

	newCardText *vango.Signal[string]
	addingToCol *vango.Signal[string] // Column ID we're adding to, or empty
	errorMsg    *vango.Signal[string]
}

// NewBoard creates the board component.
func NewBoard(model *hub.BoardModel, pool *db.Pool) *BoardComponent {
	return &BoardComponent{
		model:       model,
		pool:        pool,
		newCardText: vango.NewSignal(""),
		addingToCol: vango.NewSignal(""),
		errorMsg:    vango.NewSignal(""),
	}
}

func (b *BoardComponent) handleAddCard(columnID string) {
	text := b.newCardText.Get()
	if text == "" {
		return
	}

	b.model.AddCard(columnID, text)
	b.newCardText.Set("")
	b.addingToCol.Set("")
}

// Render implements server.Component.
func (b *BoardComponent) Render() *VNode {
	columns := b.model.Columns.Get()
	cards := b.model.Cards.Get()
	addingToCol := b.addingToCol.Get()

	return Layout(b.model.Title,
		Div(Class("board"),
			// Board header
			Div(Class("board-header"),
				H1(Text(b.model.Title)),
			),

			// Error message
			If(b.errorMsg.Get() != "",
				Div(Class("error-banner"),
					Text(b.errorMsg.Get()),
					Button(
						Class("btn-icon"),
						OnClick(func() { b.errorMsg.Set("") }),
						Text("×"),
					),
				),
			),

			// Columns container
			Div(Class("board-columns"),
				Range(columns, func(col db.Column, _ int) *VNode {
					colCards := cards[col.ID]

					return Div(
						Class("column"),
						Key(col.ID),
						Data("column-id", col.ID),

						// Column header
						Div(Class("column-header"),
							H3(Text(col.Title)),
							Span(Class("card-count"), Textf("%d", len(colCards))),
						),

						// Cards container with sortable
						Div(
							Class("cards-container"),
							Data("column-id", col.ID),
							hooks.Hook("Sortable", map[string]any{
								"group":      "board",
								"animation":  150,
								"ghostClass": "card-ghost",
								"onEnd":      fmt.Sprintf("moveCard:%s", col.ID),
							}),

							Range(colCards, func(card db.Card, idx int) *VNode {
								return b.renderCard(card, col.ID, idx)
							}),
						),

						// Add card button/form
						If(addingToCol == col.ID,
							Div(Class("add-card-form"),
								Textarea(
									Class("input"),
									Placeholder("Enter card content..."),
									Value(b.newCardText.Get()),
									OnInput(func(v string) { b.newCardText.Set(v) }),
								),
								Div(Class("form-actions"),
									Button(
										Class("btn btn-primary btn-sm"),
										OnClick(func() { b.handleAddCard(col.ID) }),
										Text("Add"),
									),
									Button(
										Class("btn btn-ghost btn-sm"),
										OnClick(func() { b.addingToCol.Set("") }),
										Text("Cancel"),
									),
								),
							),
						),
						If(addingToCol != col.ID,
							Button(
								Class("add-card-btn"),
								OnClick(func() { b.addingToCol.Set(col.ID) }),
								Text("+ Add card"),
							),
						),
					)
				}),
			),
		),
	)
}

func (b *BoardComponent) renderCard(card db.Card, columnID string, _ int) *VNode {
	return Div(
		Class("card"),
		Key(card.ID),
		Data("card-id", card.ID),
		Data("column-id", columnID),

		Div(Class("card-content"),
			P(Text(card.Content)),
		),

		Div(Class("card-actions"),
			Button(
				Class("btn-icon btn-danger"),
				OnClick(func() { b.model.DeleteCard(card.ID, columnID) }),
				Text("🗑"),
			),
		),
	)
}
