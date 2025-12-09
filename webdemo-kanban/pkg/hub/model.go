// Package hub manages shared board state across sessions.
package hub

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"webdemo-kanban/pkg/db"

	"github.com/vango-dev/vango/v2/pkg/vango"
)

// BoardModel holds the reactive state for a single board.
// All users viewing the same board share the same BoardModel instance,
// meaning Signal updates propagate to everyone automatically.
type BoardModel struct {
	ID      string
	Title   string
	Columns *vango.Signal[[]db.Column]
	Cards   *vango.Signal[map[string][]db.Card]

	pool *db.Pool
	mu   sync.Mutex
}

// NewBoardModel creates a BoardModel from loaded data.
func NewBoardModel(board *db.Board, columns []db.Column, cards map[string][]db.Card, pool *db.Pool) *BoardModel {
	return &BoardModel{
		ID:      board.ID,
		Title:   board.Title,
		Columns: vango.NewSignal(columns),
		Cards:   vango.NewSignal(cards),
		pool:    pool,
	}
}

// MoveCard moves a card from one column/position to another.
// This updates the in-memory signals (triggering UI updates for all viewers)
// and persists to the database asynchronously.
func (m *BoardModel) MoveCard(cardID, fromColID, toColID string, newIndex int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cards := m.Cards.Get()

	// Deep copy the map to avoid mutation issues
	newCards := make(map[string][]db.Card)
	for colID, colCards := range cards {
		newCards[colID] = make([]db.Card, len(colCards))
		copy(newCards[colID], colCards)
	}

	// Find and remove the card from the source column
	var movedCard db.Card
	fromCards := newCards[fromColID]
	for i, c := range fromCards {
		if c.ID == cardID {
			movedCard = c
			newCards[fromColID] = append(fromCards[:i], fromCards[i+1:]...)
			break
		}
	}

	if movedCard.ID == "" {
		log.Printf("[WARN] MoveCard: card %s not found in column %s", cardID, fromColID)
		return
	}

	// Update card's column reference
	movedCard.ColumnID = toColID
	movedCard.Position = newIndex

	// Insert into target column at the new index
	toCards := newCards[toColID]
	if newIndex >= len(toCards) {
		newCards[toColID] = append(toCards, movedCard)
	} else {
		// Insert at index
		newCards[toColID] = append(toCards[:newIndex], append([]db.Card{movedCard}, toCards[newIndex:]...)...)
	}

	// Update positions for all cards in affected columns
	for i := range newCards[fromColID] {
		newCards[fromColID][i].Position = i
	}
	for i := range newCards[toColID] {
		newCards[toColID][i].Position = i
	}

	// Update the signal - this triggers re-renders for ALL connected users
	m.Cards.Set(newCards)

	// Persist to DB asynchronously
	if m.pool != nil {
		go func() {
			if err := m.pool.UpdateCard(context.Background(), cardID, toColID, newIndex); err != nil {
				log.Printf("[ERROR] Failed to persist card move: %v", err)
			}
		}()
	}
}

// AddCard creates a new card in a column.
func (m *BoardModel) AddCard(columnID, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cards := m.Cards.Get()
	position := len(cards[columnID])

	// Demo mode: create fake card
	if m.pool == nil {
		fakeCard := db.Card{
			ID:       fmt.Sprintf("card-%d", time.Now().UnixNano()),
			ColumnID: columnID,
			Content:  content,
			Position: position,
		}
		newCards := make(map[string][]db.Card)
		for colID, colCards := range cards {
			newCards[colID] = make([]db.Card, len(colCards))
			copy(newCards[colID], colCards)
		}
		newCards[columnID] = append(newCards[columnID], fakeCard)
		m.Cards.Set(newCards)
		return
	}

	// Create card in DB
	card, err := m.pool.CreateCard(context.Background(), columnID, content, position)
	if err != nil {
		log.Printf("[ERROR] Failed to create card: %v", err)
		return
	}

	// Deep copy and add
	newCards := make(map[string][]db.Card)
	for colID, colCards := range cards {
		newCards[colID] = make([]db.Card, len(colCards))
		copy(newCards[colID], colCards)
	}
	newCards[columnID] = append(newCards[columnID], *card)

	m.Cards.Set(newCards)
}

// DeleteCard removes a card.
func (m *BoardModel) DeleteCard(cardID, columnID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cards := m.Cards.Get()

	// Deep copy
	newCards := make(map[string][]db.Card)
	for colID, colCards := range cards {
		newCards[colID] = make([]db.Card, len(colCards))
		copy(newCards[colID], colCards)
	}

	// Remove card
	colCards := newCards[columnID]
	for i, c := range colCards {
		if c.ID == cardID {
			newCards[columnID] = append(colCards[:i], colCards[i+1:]...)
			break
		}
	}

	m.Cards.Set(newCards)

	// Persist to DB
	if m.pool != nil {
		go func() {
			if err := m.pool.DeleteCard(context.Background(), cardID); err != nil {
				log.Printf("[ERROR] Failed to delete card: %v", err)
			}
		}()
	}
}
