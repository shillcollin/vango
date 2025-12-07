package ui

import (
	"strings"
	"testing"

	"github.com/vango-dev/vango/v2/pkg/vango"
	"github.com/vango-dev/vango/v2/pkg/vdom"
)

func TestButton(t *testing.T) {
	btn := Button(Variant(ButtonVariantPrimary), Class[*ButtonConfig]("my-class"))
	if btn.Tag != "button" {
		t.Errorf("expected tag button, got %s", btn.Tag)
	}

	classVal, ok := btn.Props["class"].(string)
	if !ok {
		t.Error("expected class prop")
	}

	if !strings.Contains(classVal, "bg-primary") {
		t.Error("missing primary variant class")
	}
	if !strings.Contains(classVal, "my-class") {
		t.Error("missing custom class")
	}
}

func TestDialog(t *testing.T) {
	s := vango.NewSignal(false)
	dlg := Dialog(DialogOpen(s), DialogCloseOnEscape(true))

	if dlg.Tag != "div" {
		t.Errorf("expected tag div, got %s", dlg.Tag)
	}

	foundHook := false
	for k := range dlg.Props {
		if k == "v-hook" {
			foundHook = true
			break
		}
	}

	if !foundHook {
		t.Error("missing v-hook attribute")
	}
}

func TestInput(t *testing.T) {
	inp := Input(InputType("email"), InputPlaceholder("test@example.com"))
	if inp.Tag != "input" {
		t.Errorf("expected tag input, got %s", inp.Tag)
	}

	if inp.Props["type"] != "email" {
		t.Errorf("expected type email, got %v", inp.Props["type"])
	}

	classVal, _ := inp.Props["class"].(string)
	if !strings.Contains(classVal, "bg-background") {
		t.Error("missing default styles")
	}
}

func TestLabel(t *testing.T) {
	lbl := Label(LabelFor("my-id"), Class[*LabelConfig]("text-red-500"))
	if lbl.Tag != "label" {
		t.Errorf("expected tag label, got %s", lbl.Tag)
	}

	if lbl.Props["for"] != "my-id" {
		t.Errorf("expected for my-id, got %v", lbl.Props["for"])
	}
}

func TestCard(t *testing.T) {
	card := Card(
		Class[*CardConfig]("w-[350px]"),
		Child[*CardConfig](
			CardHeader(Child[*CardHeaderConfig](
				CardTitle(Child[*CardTitleConfig](vdom.Text("Title"))),
			)),
			CardContent(Child[*CardContentConfig](vdom.Text("Content"))),
		),
	)

	if card.Tag != "div" {
		t.Errorf("expected tag div, got %s", card.Tag)
	}

	classVal, _ := card.Props["class"].(string)
	if !strings.Contains(classVal, "rounded-lg") {
		t.Error("missing card styles")
	}

	// Basic hierarchy check
	if len(card.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(card.Children))
	}
}

func TestKanban(t *testing.T) {
	board := KanbanBoard(
		Class[*KanbanBoardConfig]("bg-gray-100"),
		Child[*KanbanBoardConfig](
			KanbanColumn(
				KanbanColumnID("col-1"),
				KanbanColumnTitle("Todo"),
				Child[*KanbanColumnConfig](
					KanbanCard(
						KanbanCardID("card-1"),
						Child[*KanbanCardConfig](vdom.Text("Task 1")),
					),
				),
			),
		),
	)

	if board.Tag != "div" {
		t.Errorf("expected tag div, got %s", board.Tag)
	}

	col := board.Children[0]
	// Column structure: Div(Header, Content(Sortable))
	// Actually implementation is: Div(Header?, Content(Sortable))
	// Let's inspect properties
	if col.Props["data-column-id"] != "col-1" {
		t.Errorf("expected column id col-1, got %v", col.Props["data-column-id"])
	}

	// Check for sortable hook in column's content div
	// Column children: H3 (title), Div (content + hook)
	contentDiv := col.Children[1] // Index 1 because Header is present
	foundHook := false
	for k := range contentDiv.Props {
		if k == "v-hook" {
			foundHook = true
			break
		}
	}

	if !foundHook {
		t.Error("missing Sortable hook in column content")
	}
}
