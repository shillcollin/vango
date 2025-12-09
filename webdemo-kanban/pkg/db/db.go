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

// Board represents a Kanban board.
type Board struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
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
	ID        string    `json:"id"`
	ColumnID  string    `json:"column_id"`
	Content   string    `json:"content"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// GetBoards returns all boards for a user.
func (p *Pool) GetBoards(ctx context.Context, userID string) ([]Board, error) {
	rows, err := p.Query(ctx, `
		SELECT id, title, owner_id, created_at 
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
		if err := rows.Scan(&b.ID, &b.Title, &b.OwnerID, &b.CreatedAt); err != nil {
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
		SELECT id, title, owner_id, created_at 
		FROM boards WHERE id = $1
	`, boardID).Scan(&board.ID, &board.Title, &board.OwnerID, &board.CreatedAt)
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

	// Get cards
	cardRows, err := p.Query(ctx, `
		SELECT id, column_id, content, position, created_at 
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
		if err := cardRows.Scan(&c.ID, &c.ColumnID, &c.Content, &c.Position, &c.CreatedAt); err != nil {
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

// CreateCard creates a new card.
func (p *Pool) CreateCard(ctx context.Context, columnID, content string, position int) (*Card, error) {
	var card Card
	err := p.QueryRow(ctx, `
		INSERT INTO cards (column_id, content, position) 
		VALUES ($1, $2, $3) 
		RETURNING id, column_id, content, position, created_at
	`, columnID, content, position).Scan(&card.ID, &card.ColumnID, &card.Content, &card.Position, &card.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &card, nil
}

// UpdateCard updates a card's column and position.
func (p *Pool) UpdateCard(ctx context.Context, cardID, columnID string, position int) error {
	_, err := p.Exec(ctx, `
		UPDATE cards SET column_id = $1, position = $2 WHERE id = $3
	`, columnID, position, cardID)
	return err
}

// DeleteCard deletes a card.
func (p *Pool) DeleteCard(ctx context.Context, cardID string) error {
	_, err := p.Exec(ctx, `DELETE FROM cards WHERE id = $1`, cardID)
	return err
}
