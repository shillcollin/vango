package hub

import (
	"context"
	"sync"

	"webdemo-kanban/pkg/db"
)

// Hub is a singleton that manages active BoardModel instances.
// When multiple users view the same board, they share the same BoardModel,
// enabling real-time collaboration through shared Signals.
type Hub struct {
	boards sync.Map // map[string]*BoardModel
	pool   *db.Pool
}

// globalHub is the singleton instance.
var globalHub *Hub
var hubOnce sync.Once

// GetHub returns the global Hub instance.
func GetHub(pool *db.Pool) *Hub {
	hubOnce.Do(func() {
		globalHub = &Hub{
			pool: pool,
		}
	})
	return globalHub
}

// GetBoard returns the BoardModel for the given board ID.
// If the board isn't loaded, it fetches from the database and caches it.
func (h *Hub) GetBoard(ctx context.Context, boardID string) (*BoardModel, error) {
	// Check cache
	if cached, ok := h.boards.Load(boardID); ok {
		return cached.(*BoardModel), nil
	}

	// Demo mode
	if h.pool == nil {
		demoBoard := &db.Board{ID: boardID, Title: "Demo Board"}
		demoCols := []db.Column{
			{ID: "col-1", BoardID: boardID, Title: "To Do", Position: 0},
			{ID: "col-2", BoardID: boardID, Title: "In Progress", Position: 1},
			{ID: "col-3", BoardID: boardID, Title: "Done", Position: 2},
		}
		demoCards := map[string][]db.Card{
			"col-1": {{ID: "card-1", ColumnID: "col-1", Content: "Try dragging me!", Position: 0}},
			"col-2": {},
			"col-3": {},
		}
		model := NewBoardModel(demoBoard, demoCols, demoCards, nil)
		h.boards.Store(boardID, model)
		return model, nil
	}

	// Load from database
	board, columns, cards, err := h.pool.GetBoard(ctx, boardID)
	if err != nil {
		return nil, err
	}

	// Create model
	model := NewBoardModel(board, columns, cards, h.pool)

	// Store (use LoadOrStore for race safety)
	actual, _ := h.boards.LoadOrStore(boardID, model)
	return actual.(*BoardModel), nil
}

// InvalidateBoard removes a board from the cache.
func (h *Hub) InvalidateBoard(boardID string) {
	h.boards.Delete(boardID)
}
