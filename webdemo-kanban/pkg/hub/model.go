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
	Labels  *vango.Signal[[]db.Label]

	pool *db.Pool
	mu   sync.Mutex
}

// NewBoardModel creates a BoardModel from loaded data.
func NewBoardModel(board *db.Board, columns []db.Column, cards map[string][]db.Card, pool *db.Pool) *BoardModel {
	m := &BoardModel{
		ID:      board.ID,
		Title:   board.Title,
		Columns: vango.NewSignal(columns),
		Cards:   vango.NewSignal(cards),
		Labels:  vango.NewSignal([]db.Label{}),
		pool:    pool,
	}

	// Load labels asynchronously
	if pool != nil {
		go func() {
			labels, err := pool.GetBoardLabels(context.Background(), board.ID)
			if err != nil {
				log.Printf("[WARN] Failed to load labels: %v", err)
				return
			}
			m.Labels.Set(labels)
		}()
	}

	return m
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
func (m *BoardModel) AddCard(columnID, title string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cards := m.Cards.Get()
	position := len(cards[columnID])

	// Demo mode: create fake card
	if m.pool == nil {
		fakeCard := db.Card{
			ID:       fmt.Sprintf("card-%d", time.Now().UnixNano()),
			ColumnID: columnID,
			Title:    title,
			Content:  title,
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
	card, err := m.pool.CreateCard(context.Background(), columnID, title, position)
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

// UpdateCard updates a card's properties (title, description, due date, etc.)
// and persists to the database asynchronously.
func (m *BoardModel) UpdateCard(cardID string, update func(c *db.Card)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cards := m.Cards.Get()

	// Deep copy and find card to update
	newCards := make(map[string][]db.Card)
	var updatedCard *db.Card
	for colID, colCards := range cards {
		newCards[colID] = make([]db.Card, len(colCards))
		for i, c := range colCards {
			if c.ID == cardID {
				// Apply update
				update(&c)
				updatedCard = &c
			}
			newCards[colID][i] = c
		}
	}

	if updatedCard == nil {
		log.Printf("[WARN] UpdateCard: card %s not found", cardID)
		return
	}

	m.Cards.Set(newCards)

	// Persist to DB asynchronously
	if m.pool != nil {
		card := *updatedCard
		go func() {
			if card.Title != "" {
				if err := m.pool.UpdateCardTitle(context.Background(), card.ID, card.Title); err != nil {
					log.Printf("[ERROR] Failed to update card title: %v", err)
				}
			}
			if card.Description != "" {
				if err := m.pool.UpdateCardDescription(context.Background(), card.ID, card.Description); err != nil {
					log.Printf("[ERROR] Failed to update card description: %v", err)
				}
			}
			if err := m.pool.UpdateCardDueDate(context.Background(), card.ID, card.DueDate); err != nil {
				log.Printf("[ERROR] Failed to update card due date: %v", err)
			}
		}()
	}
}

// AddColumn creates a new column at the end of the board.
func (m *BoardModel) AddColumn(title string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	columns := m.Columns.Get()
	position := len(columns)

	// Demo mode
	if m.pool == nil {
		fakeCol := db.Column{
			ID:       fmt.Sprintf("col-%d", time.Now().UnixNano()),
			BoardID:  m.ID,
			Title:    title,
			Position: position,
		}
		newColumns := make([]db.Column, len(columns))
		copy(newColumns, columns)
		newColumns = append(newColumns, fakeCol)
		m.Columns.Set(newColumns)

		// Initialize empty cards for new column
		cards := m.Cards.Get()
		newCards := make(map[string][]db.Card)
		for colID, colCards := range cards {
			newCards[colID] = colCards
		}
		newCards[fakeCol.ID] = []db.Card{}
		m.Cards.Set(newCards)
		return
	}

	col, err := m.pool.CreateColumn(context.Background(), m.ID, title, position)
	if err != nil {
		log.Printf("[ERROR] Failed to create column: %v", err)
		return
	}

	newColumns := make([]db.Column, len(columns))
	copy(newColumns, columns)
	newColumns = append(newColumns, *col)
	m.Columns.Set(newColumns)

	// Initialize empty cards for new column
	cards := m.Cards.Get()
	newCards := make(map[string][]db.Card)
	for colID, colCards := range cards {
		newCards[colID] = colCards
	}
	newCards[col.ID] = []db.Card{}
	m.Cards.Set(newCards)
}

// RenameColumn updates a column's title.
func (m *BoardModel) RenameColumn(columnID, title string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	columns := m.Columns.Get()
	newColumns := make([]db.Column, len(columns))
	copy(newColumns, columns)

	for i := range newColumns {
		if newColumns[i].ID == columnID {
			newColumns[i].Title = title
			break
		}
	}

	m.Columns.Set(newColumns)

	if m.pool != nil {
		go func() {
			if err := m.pool.UpdateColumnTitle(context.Background(), columnID, title); err != nil {
				log.Printf("[ERROR] Failed to rename column: %v", err)
			}
		}()
	}
}

// DeleteColumn removes a column and all its cards.
func (m *BoardModel) DeleteColumn(columnID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	columns := m.Columns.Get()
	newColumns := make([]db.Column, 0, len(columns)-1)
	for _, c := range columns {
		if c.ID != columnID {
			newColumns = append(newColumns, c)
		}
	}
	m.Columns.Set(newColumns)

	// Remove cards for this column
	cards := m.Cards.Get()
	newCards := make(map[string][]db.Card)
	for colID, colCards := range cards {
		if colID != columnID {
			newCards[colID] = colCards
		}
	}
	m.Cards.Set(newCards)

	if m.pool != nil {
		go func() {
			if err := m.pool.DeleteColumn(context.Background(), columnID); err != nil {
				log.Printf("[ERROR] Failed to delete column: %v", err)
			}
		}()
	}
}
