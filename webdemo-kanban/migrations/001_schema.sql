-- Kanban Board Schema
-- Run this in Supabase SQL Editor

-- Enable UUID extension
create extension if not exists "uuid-ossp";

-- Boards table
create table if not exists boards (
  id uuid primary key default uuid_generate_v4(),
  title text not null,
  owner_id text not null,
  created_at timestamp with time zone default now()
);

-- Columns table
create table if not exists columns (
  id uuid primary key default uuid_generate_v4(),
  board_id uuid references boards(id) on delete cascade,
  title text not null,
  position int not null,
  created_at timestamp with time zone default now()
);

-- Cards table
create table if not exists cards (
  id uuid primary key default uuid_generate_v4(),
  column_id uuid references columns(id) on delete cascade,
  content text not null,
  position int not null,
  created_at timestamp with time zone default now()
);

-- Indexes for performance
create index if not exists idx_boards_owner on boards(owner_id);
create index if not exists idx_columns_board on columns(board_id);
create index if not exists idx_cards_column on cards(column_id);
