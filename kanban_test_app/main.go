package main

import (
	"log"
	"net/http"

	"github.com/vango-dev/vango-ui/app/components/ui"
	"github.com/vango-dev/vango/v2/pkg/render"
	"github.com/vango-dev/vango/v2/pkg/server"
	"github.com/vango-dev/vango/v2/pkg/vdom"
)

func createRootComponent() func() server.Component {
	return func() server.Component {
		return server.FuncComponent(func() *vdom.VNode {
			return ui.KanbanBoard(
				ui.Child[*ui.KanbanBoardConfig](
					ui.KanbanColumn(
						ui.KanbanColumnTitle("To Do"),
						ui.KanbanColumnID("todo"),
						ui.Child[*ui.KanbanColumnConfig](
							ui.KanbanCard(
								ui.KanbanCardID("card-1"),
								ui.Child[*ui.KanbanCardConfig](vdom.Text("Task 1")),
							),
							ui.KanbanCard(
								ui.KanbanCardID("card-2"),
								ui.Child[*ui.KanbanCardConfig](vdom.Text("Task 2")),
							),
						),
					),
					ui.KanbanColumn(
						ui.KanbanColumnTitle("In Progress"),
						ui.KanbanColumnID("in-progress"),
						ui.Child[*ui.KanbanColumnConfig](
							ui.KanbanCard(
								ui.KanbanCardID("card-3"),
								ui.Child[*ui.KanbanCardConfig](vdom.Text("Task 3")),
							),
						),
					),
					ui.KanbanColumn(
						ui.KanbanColumnTitle("Done"),
						ui.KanbanColumnID("done"),
						ui.Child[*ui.KanbanColumnConfig](
							ui.KanbanCard(
								ui.KanbanCardID("card-4"),
								ui.Child[*ui.KanbanCardConfig](vdom.Text("Task 4")),
							),
						),
					),
				),
			)
		})
	}
}

func main() {
	// Configure server
	cfg := server.DefaultServerConfig()
	srv := server.New(cfg)

	// Define the root component
	srv.SetRootComponent(createRootComponent())

	// Create a mux to handle routes
	mux := http.NewServeMux()

	// Serve client JS
	mux.HandleFunc("/_vango/client.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../vango_v2/client/dist/vango.js")
	})

	// Serve WebSocket
	mux.HandleFunc("/_vango/ws", srv.HandleWebSocket)

	// Serve main page
	mux.HandleFunc("/", handleIndex)

	srv.SetHandler(mux)

	log.Println("Listening on :8080")
	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	comp := createRootComponent()()
	node := comp.Render()

	renderer := render.NewRenderer(render.RendererConfig{Pretty: true})
	htmlContent, err := renderer.RenderToString(node)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <title>Kanban Test</title>
    <script src="/_vango/client.js"></script>
    <link rel="stylesheet" href="https://cdn.tailwindcss.com">
</head>
<body class="bg-background text-foreground">
    <div id="app" class="h-screen w-screen p-4">
` + htmlContent + `
    </div>
</body>
</html>
`))
}
