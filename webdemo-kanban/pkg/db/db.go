// Package db provides a PostgreSQL client using pgx.
package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is a connection pool to the database.
type Pool struct {
	*pgxpool.Pool
}

// NewPool creates a new connection pool from DATABASE_URL.
func NewPool(ctx context.Context) (*Pool, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL not set")
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Connection pool settings
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	return &Pool{pool}, nil
}

// User represents a user in the system.
type User struct {
	ID          string    `json:"id"`
	AuthID      *string   `json:"auth_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
}

// Board represents a Kanban board.
type Board struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Background  string    `json:"background"`
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// BoardMember represents a user's membership on a board.
type BoardMember struct {
	BoardID   string    `json:"board_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"` // "owner", "member", "viewer"
	InvitedAt time.Time `json:"invited_at"`
	User      *User     `json:"user,omitempty"` // Populated when joining
}

// Column represents a column in a board.
type Column struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	Title     string    `json:"title"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// Card represents a card in a column.
type Card struct {
	ID          string     `json:"id"`
	ColumnID    string     `json:"column_id"`
	Title       string     `json:"title"`
	Content     string     `json:"content"` // Legacy: used as description fallback
	Description string     `json:"description"`
	Position    int        `json:"position"`
	DueDate     *time.Time `json:"due_date"`
	AssigneeID  *string    `json:"assignee_id"`
	CoverColor  *string    `json:"cover_color"`
	CreatedAt   time.Time  `json:"created_at"`
	// Populated on fetch
	Labels   []Label `json:"labels,omitempty"`
	Assignee *User   `json:"assignee,omitempty"`
}

// Label represents a colored label on a board.
type Label struct {
	ID      string `json:"id"`
	BoardID string `json:"board_id"`
	Name    string `json:"name"`
	Color   string `json:"color"` // red, orange, yellow, green, blue, purple
}

// Checklist represents a checklist on a card.
type Checklist struct {
	ID       string          `json:"id"`
	CardID   string          `json:"card_id"`
	Title    string          `json:"title"`
	Position int             `json:"position"`
	Items    []ChecklistItem `json:"items,omitempty"`
}

// ChecklistItem represents an item in a checklist.
type ChecklistItem struct {
	ID          string `json:"id"`
	ChecklistID string `json:"checklist_id"`
	Content     string `json:"content"`
	Completed   bool   `json:"completed"`
	Position    int    `json:"position"`
}

// Comment represents a comment on a card.
type Comment struct {
	ID        string    `json:"id"`
	CardID    string    `json:"card_id"`
	UserID    *string   `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	User      *User     `json:"user,omitempty"` // Populated when joining
}

// GetBoards returns all boards for a user.
func (p *Pool) GetBoards(ctx context.Context, userID string) ([]Board, error) {
	rows, err := p.Query(ctx, `
		SELECT id, title, COALESCE(description, ''), COALESCE(background, '#1e3a5f'), owner_id, created_at 
		FROM boards 
		WHERE owner_id = $1 
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []Board
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.Title, &b.Description, &b.Background, &b.OwnerID, &b.CreatedAt); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

// GetBoard returns a board with its columns and cards.
func (p *Pool) GetBoard(ctx context.Context, boardID string) (*Board, []Column, map[string][]Card, error) {
	// Get board
	var board Board
	err := p.QueryRow(ctx, `
		SELECT id, title, COALESCE(description, ''), COALESCE(background, '#1e3a5f'), owner_id, created_at 
		FROM boards WHERE id = $1
	`, boardID).Scan(&board.ID, &board.Title, &board.Description, &board.Background, &board.OwnerID, &board.CreatedAt)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("board not found: %w", err)
	}

	// Get columns
	colRows, err := p.Query(ctx, `
		SELECT id, board_id, title, position, created_at 
		FROM columns 
		WHERE board_id = $1 
		ORDER BY position ASC
	`, boardID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer colRows.Close()

	var columns []Column
	columnIDs := []string{}
	for colRows.Next() {
		var c Column
		if err := colRows.Scan(&c.ID, &c.BoardID, &c.Title, &c.Position, &c.CreatedAt); err != nil {
			return nil, nil, nil, err
		}
		columns = append(columns, c)
		columnIDs = append(columnIDs, c.ID)
	}
	if err := colRows.Err(); err != nil {
		return nil, nil, nil, err
	}

	// Initialize cards map
	cardsByColumn := make(map[string][]Card)
	for _, c := range columns {
		cardsByColumn[c.ID] = []Card{}
	}

	if len(columnIDs) == 0 {
		return &board, columns, cardsByColumn, nil
	}

	// Get cards with enhanced fields
	cardRows, err := p.Query(ctx, `
		SELECT id, column_id, COALESCE(title, ''), COALESCE(content, ''), COALESCE(description, ''),
		       position, due_date, assignee_id, cover_color, created_at 
		FROM cards 
		WHERE column_id = ANY($1) 
		ORDER BY position ASC
	`, columnIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	defer cardRows.Close()

	for cardRows.Next() {
		var c Card
		if err := cardRows.Scan(&c.ID, &c.ColumnID, &c.Title, &c.Content, &c.Description,
			&c.Position, &c.DueDate, &c.AssigneeID, &c.CoverColor, &c.CreatedAt); err != nil {
			return nil, nil, nil, err
		}
		cardsByColumn[c.ColumnID] = append(cardsByColumn[c.ColumnID], c)
	}

	return &board, columns, cardsByColumn, cardRows.Err()
}

// CreateBoard creates a new board with default columns.
func (p *Pool) CreateBoard(ctx context.Context, title, ownerID string) (*Board, []Column, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	// Create board
	var board Board
	err = tx.QueryRow(ctx, `
		INSERT INTO boards (title, owner_id) 
		VALUES ($1, $2) 
		RETURNING id, title, owner_id, created_at
	`, title, ownerID).Scan(&board.ID, &board.Title, &board.OwnerID, &board.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	// Create default columns
	defaultCols := []string{"To Do", "In Progress", "Done"}
	var columns []Column
	for i, colTitle := range defaultCols {
		var col Column
		err = tx.QueryRow(ctx, `
			INSERT INTO columns (board_id, title, position) 
			VALUES ($1, $2, $3) 
			RETURNING id, board_id, title, position, created_at
		`, board.ID, colTitle, i).Scan(&col.ID, &col.BoardID, &col.Title, &col.Position, &col.CreatedAt)
		if err != nil {
			return nil, nil, err
		}
		columns = append(columns, col)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return &board, columns, nil
}

// CreateCard creates a new card with a title.
func (p *Pool) CreateCard(ctx context.Context, columnID, title string, position int) (*Card, error) {
	var card Card
	err := p.QueryRow(ctx, `
		INSERT INTO cards (column_id, title, content, position) 
		VALUES ($1, $2, $2, $3) 
		RETURNING id, column_id, COALESCE(title, ''), COALESCE(content, ''), COALESCE(description, ''),
		          position, due_date, assignee_id, cover_color, created_at
	`, columnID, title, position).Scan(&card.ID, &card.ColumnID, &card.Title, &card.Content, &card.Description,
		&card.Position, &card.DueDate, &card.AssigneeID, &card.CoverColor, &card.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &card, nil
}

// UpdateCardPosition updates a card's column and position.
func (p *Pool) UpdateCardPosition(ctx context.Context, cardID, columnID string, position int) error {
	_, err := p.Exec(ctx, `
		UPDATE cards SET column_id = $1, position = $2 WHERE id = $3
	`, columnID, position, cardID)
	return err
}

// UpdateCard is an alias for UpdateCardPosition for backward compatibility.
func (p *Pool) UpdateCard(ctx context.Context, cardID, columnID string, position int) error {
	return p.UpdateCardPosition(ctx, cardID, columnID, position)
}

// UpdateCardTitle updates a card's title.
func (p *Pool) UpdateCardTitle(ctx context.Context, cardID, title string) error {
	_, err := p.Exec(ctx, `UPDATE cards SET title = $1 WHERE id = $2`, title, cardID)
	return err
}

// UpdateCardDescription updates a card's description.
func (p *Pool) UpdateCardDescription(ctx context.Context, cardID, description string) error {
	_, err := p.Exec(ctx, `UPDATE cards SET description = $1 WHERE id = $2`, description, cardID)
	return err
}

// UpdateCardDueDate updates a card's due date.
func (p *Pool) UpdateCardDueDate(ctx context.Context, cardID string, dueDate *time.Time) error {
	_, err := p.Exec(ctx, `UPDATE cards SET due_date = $1 WHERE id = $2`, dueDate, cardID)
	return err
}

// UpdateCardAssignee updates a card's assignee.
func (p *Pool) UpdateCardAssignee(ctx context.Context, cardID string, assigneeID *string) error {
	_, err := p.Exec(ctx, `UPDATE cards SET assignee_id = $1 WHERE id = $2`, assigneeID, cardID)
	return err
}

// UpdateCardCoverColor updates a card's cover color.
func (p *Pool) UpdateCardCoverColor(ctx context.Context, cardID string, coverColor *string) error {
	_, err := p.Exec(ctx, `UPDATE cards SET cover_color = $1 WHERE id = $2`, coverColor, cardID)
	return err
}

// DeleteCard deletes a card.
func (p *Pool) DeleteCard(ctx context.Context, cardID string) error {
	_, err := p.Exec(ctx, `DELETE FROM cards WHERE id = $1`, cardID)
	return err
}

// GetCard returns a single card by ID with labels.
func (p *Pool) GetCard(ctx context.Context, cardID string) (*Card, error) {
	var c Card
	err := p.QueryRow(ctx, `
		SELECT id, column_id, COALESCE(title, ''), COALESCE(content, ''), COALESCE(description, ''),
		       position, due_date, assignee_id, cover_color, created_at 
		FROM cards WHERE id = $1
	`, cardID).Scan(&c.ID, &c.ColumnID, &c.Title, &c.Content, &c.Description,
		&c.Position, &c.DueDate, &c.AssigneeID, &c.CoverColor, &c.CreatedAt)
	if err != nil {
		return nil, err
	}

	// Get labels for this card
	labels, err := p.GetCardLabels(ctx, cardID)
	if err == nil {
		c.Labels = labels
	}

	return &c, nil
}

// --- Labels ---

// GetBoardLabels returns all labels for a board.
func (p *Pool) GetBoardLabels(ctx context.Context, boardID string) ([]Label, error) {
	rows, err := p.Query(ctx, `
		SELECT id, board_id, COALESCE(name, ''), color 
		FROM labels WHERE board_id = $1 ORDER BY color
	`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.BoardID, &l.Name, &l.Color); err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

// CreateLabel creates a new label.
func (p *Pool) CreateLabel(ctx context.Context, boardID, name, color string) (*Label, error) {
	var l Label
	err := p.QueryRow(ctx, `
		INSERT INTO labels (board_id, name, color) VALUES ($1, $2, $3)
		RETURNING id, board_id, name, color
	`, boardID, name, color).Scan(&l.ID, &l.BoardID, &l.Name, &l.Color)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// GetCardLabels returns labels attached to a card.
func (p *Pool) GetCardLabels(ctx context.Context, cardID string) ([]Label, error) {
	rows, err := p.Query(ctx, `
		SELECT l.id, l.board_id, COALESCE(l.name, ''), l.color
		FROM labels l
		JOIN card_labels cl ON cl.label_id = l.id
		WHERE cl.card_id = $1
		ORDER BY l.color
	`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.BoardID, &l.Name, &l.Color); err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

// AddCardLabel adds a label to a card.
func (p *Pool) AddCardLabel(ctx context.Context, cardID, labelID string) error {
	_, err := p.Exec(ctx, `
		INSERT INTO card_labels (card_id, label_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, cardID, labelID)
	return err
}

// RemoveCardLabel removes a label from a card.
func (p *Pool) RemoveCardLabel(ctx context.Context, cardID, labelID string) error {
	_, err := p.Exec(ctx, `DELETE FROM card_labels WHERE card_id = $1 AND label_id = $2`, cardID, labelID)
	return err
}

// --- Checklists ---

// GetCardChecklists returns all checklists for a card with their items.
func (p *Pool) GetCardChecklists(ctx context.Context, cardID string) ([]Checklist, error) {
	rows, err := p.Query(ctx, `
		SELECT id, card_id, title, position FROM checklists 
		WHERE card_id = $1 ORDER BY position
	`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checklists []Checklist
	checklistIDs := []string{}
	for rows.Next() {
		var c Checklist
		if err := rows.Scan(&c.ID, &c.CardID, &c.Title, &c.Position); err != nil {
			return nil, err
		}
		checklists = append(checklists, c)
		checklistIDs = append(checklistIDs, c.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(checklistIDs) == 0 {
		return checklists, nil
	}

	// Get items
	itemRows, err := p.Query(ctx, `
		SELECT id, checklist_id, content, completed, position
		FROM checklist_items WHERE checklist_id = ANY($1) ORDER BY position
	`, checklistIDs)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	itemsByChecklist := make(map[string][]ChecklistItem)
	for itemRows.Next() {
		var i ChecklistItem
		if err := itemRows.Scan(&i.ID, &i.ChecklistID, &i.Content, &i.Completed, &i.Position); err != nil {
			return nil, err
		}
		itemsByChecklist[i.ChecklistID] = append(itemsByChecklist[i.ChecklistID], i)
	}

	// Attach items to checklists
	for i := range checklists {
		checklists[i].Items = itemsByChecklist[checklists[i].ID]
	}

	return checklists, nil
}

// CreateChecklist creates a new checklist.
func (p *Pool) CreateChecklist(ctx context.Context, cardID, title string, position int) (*Checklist, error) {
	var c Checklist
	err := p.QueryRow(ctx, `
		INSERT INTO checklists (card_id, title, position) VALUES ($1, $2, $3)
		RETURNING id, card_id, title, position
	`, cardID, title, position).Scan(&c.ID, &c.CardID, &c.Title, &c.Position)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// AddChecklistItem adds an item to a checklist.
func (p *Pool) AddChecklistItem(ctx context.Context, checklistID, content string, position int) (*ChecklistItem, error) {
	var i ChecklistItem
	err := p.QueryRow(ctx, `
		INSERT INTO checklist_items (checklist_id, content, position) VALUES ($1, $2, $3)
		RETURNING id, checklist_id, content, completed, position
	`, checklistID, content, position).Scan(&i.ID, &i.ChecklistID, &i.Content, &i.Completed, &i.Position)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

// ToggleChecklistItem toggles the completion status of a checklist item.
func (p *Pool) ToggleChecklistItem(ctx context.Context, itemID string) error {
	_, err := p.Exec(ctx, `UPDATE checklist_items SET completed = NOT completed WHERE id = $1`, itemID)
	return err
}

// DeleteChecklist deletes a checklist and its items.
func (p *Pool) DeleteChecklist(ctx context.Context, checklistID string) error {
	_, err := p.Exec(ctx, `DELETE FROM checklists WHERE id = $1`, checklistID)
	return err
}

// --- Comments ---

// GetCardComments returns all comments for a card.
func (p *Pool) GetCardComments(ctx context.Context, cardID string) ([]Comment, error) {
	rows, err := p.Query(ctx, `
		SELECT c.id, c.card_id, c.user_id, c.content, c.created_at,
		       u.id, u.email, COALESCE(u.display_name, ''), COALESCE(u.avatar_url, '')
		FROM comments c
		LEFT JOIN users u ON u.id = c.user_id
		WHERE c.card_id = $1
		ORDER BY c.created_at DESC
	`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		var userID, userEmail, displayName, avatarURL *string
		if err := rows.Scan(&c.ID, &c.CardID, &c.UserID, &c.Content, &c.CreatedAt,
			&userID, &userEmail, &displayName, &avatarURL); err != nil {
			return nil, err
		}
		if userID != nil && userEmail != nil {
			c.User = &User{
				ID:          *userID,
				Email:       *userEmail,
				DisplayName: *displayName,
				AvatarURL:   *avatarURL,
			}
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// CreateComment creates a comment on a card.
func (p *Pool) CreateComment(ctx context.Context, cardID string, userID *string, content string) (*Comment, error) {
	var c Comment
	err := p.QueryRow(ctx, `
		INSERT INTO comments (card_id, user_id, content) VALUES ($1, $2, $3)
		RETURNING id, card_id, user_id, content, created_at
	`, cardID, userID, content).Scan(&c.ID, &c.CardID, &c.UserID, &c.Content, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// --- Columns ---

// CreateColumn creates a new column.
func (p *Pool) CreateColumn(ctx context.Context, boardID, title string, position int) (*Column, error) {
	var col Column
	err := p.QueryRow(ctx, `
		INSERT INTO columns (board_id, title, position) VALUES ($1, $2, $3)
		RETURNING id, board_id, title, position, created_at
	`, boardID, title, position).Scan(&col.ID, &col.BoardID, &col.Title, &col.Position, &col.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &col, nil
}

// UpdateColumnTitle updates a column's title.
func (p *Pool) UpdateColumnTitle(ctx context.Context, columnID, title string) error {
	_, err := p.Exec(ctx, `UPDATE columns SET title = $1 WHERE id = $2`, title, columnID)
	return err
}

// DeleteColumn deletes a column.
func (p *Pool) DeleteColumn(ctx context.Context, columnID string) error {
	_, err := p.Exec(ctx, `DELETE FROM columns WHERE id = $1`, columnID)
	return err
}
