
# Collaborative Kanban Board - Vango V2 Spec

This document outlines the architecture and implementation plan for a real-time, collaborative Kanban Board built with Vango V2 and Supabase.

## 1. Overview

**Goal**: Demonstrate the power of Vango's server-driven architecture by building a Trello-like application where multiple users can drag-and-drop cards simultaneously and see updates instantly without any client-side complexity.

**Key Features**:
*   **Real-time Collaboration**: Changes by one user are instantly reflected for all other users on the same board.
*   **Drag and Drop**: Smooth card movement using Vango's `Sortable` hook.
*   **Persistent Storage**: All data is persisted to Supabase (Postgres).
*   **Authentication**: Users sign in via Supabase Auth.

## 2. Architecture

### 2.1 State Management (The "WOW" Factor)
To demonstrate Vango's capabilities, we will use a **Shared In-Memory State** pattern backed by the database.

1.  **Ref**: `Hub` (Singleton)
    *   Manages active `BoardModel` instances.
    *   Ensures all users viewing Board X share the exact same `*vango.Signal` pointers.
2.  **Model**: `BoardModel`
    *   Contains the state of the board: `Columns` (Signal), `Cards` (Signal).
    *   When a user moves a card, they update the shared Signal.
    *   Vango's reactivity automatically triggers re-renders for *all* connected users.
    *   A background goroutine (or the setter itself) persists the change to Supabase asynchronously.

### 2.2 Database Schema (Supabase)

```sql
create table users (
  id uuid primary key default uuid_generate_v4(),
  email text not null,
  created_at timestamp with time zone default now()
);

create table boards (
  id uuid primary key default uuid_generate_v4(),
  title text not null,
  owner_id uuid references users(id),
  created_at timestamp with time zone default now()
);

create table columns (
  id uuid primary key default uuid_generate_v4(),
  board_id uuid references boards(id) on delete cascade,
  title text not null,
  position int not null,
  created_at timestamp with time zone default now()
);

create table cards (
  id uuid primary key default uuid_generate_v4(),
  column_id uuid references columns(id) on delete cascade,
  content text not null,
  position int not null,
  created_at timestamp with time zone default now()
);
```

## 3. Package Structure

```
webdemo-kanban/
├── main.go                 # Entry point
├── go.mod
├── pkg/
│   ├── app/                # UI Components
│   │   ├── app.go          # Root component & Routing
│   │   ├── board.go        # Board View (Kanban)
│   │   ├── dashboard.go    # List of boards
│   │   ├── auth.go         # Login/Signup forms
│   │   ├── layout.go       # Shell
│   │   └── types.go        # Common types
│   ├── hub/                # Shared State Manager
│   │   ├── hub.go          # Board Hub
│   │   └── model.go        # Board Logic (MoveCard, etc.)
│   └── supabase/           # DB Client
│       └── client.go       # GoTwo/Supabase wrapper
```

## 4. Implementation Details

### 4.1 The Hub (State Manager)

```go
type Hub struct {
    boards sync.Map // map[string]*BoardModel
}

func (h *Hub) GetBoard(id string) *BoardModel {
    // Return existing or load from DB
}
```

### 4.2 The Board Model (Reactivity)

```go
type BoardModel struct {
    ID      string
    // Signals specific to this board
    Columns *vango.Signal[[]Column]
    Cards   *vango.Signal[map[string][]Card] // Keyed by ColumnID
}

func (m *BoardModel) MoveCard(cardID, fromCol, toCol string, newIndex int) {
    // 1. Update In-Memory Signal (Triggers UI updates for EVERYONE)
    // 2. Fire-and-forget DB update
}
```

### 4.3 UI Components

**Board Component**:
Uses `vango.Hooks.Sortable` implementation.

```go
func ColumnView(col Column, cards []Card, onDrop func(evt SortableEvent)) *VNode {
    return Div(
        Class("column"),
        Sortable(SortableConfig{
            Group: "board",
            OnEnd: onDrop, // Handler provided by Vango
        }),
        // Render cards...
    )
}
```

## 5. Deployment Plan

1.  **Setup Supabase**: Create project and tables.
2.  **Scaffold App**: `webdemo-kanban` directory.
3.  **Implement Auth**: Basic Email/Password flow.
4.  **Implement Dashboard**: Create/List boards.
5.  **Implement Kanban**: The core drag-and-drop logic.
6.  **Polish**: Styling and "presence" indicators (e.g., "User X is viewing").
