-- Enhanced Kanban Schema for Trello Clone
-- Run this in Supabase SQL Editor

-- Users table (links to Supabase Auth)
create table if not exists users (
  id uuid primary key default uuid_generate_v4(),
  auth_id uuid unique,  -- Links to auth.users
  email text not null,
  display_name text,
  avatar_url text,
  created_at timestamp with time zone default now()
);

-- Board members (sharing)
create table if not exists board_members (
  board_id uuid references boards(id) on delete cascade,
  user_id uuid references users(id) on delete cascade,
  role text not null default 'member',  -- 'owner', 'member', 'viewer'
  invited_at timestamp with time zone default now(),
  primary key (board_id, user_id)
);

-- Labels (per board, predefined colors)
create table if not exists labels (
  id uuid primary key default uuid_generate_v4(),
  board_id uuid references boards(id) on delete cascade,
  name text,
  color text not null  -- 'red', 'orange', 'yellow', 'green', 'blue', 'purple'
);

-- Card labels (many-to-many)
create table if not exists card_labels (
  card_id uuid references cards(id) on delete cascade,
  label_id uuid references labels(id) on delete cascade,
  primary key (card_id, label_id)
);

-- Checklists
create table if not exists checklists (
  id uuid primary key default uuid_generate_v4(),
  card_id uuid references cards(id) on delete cascade,
  title text not null,
  position int not null
);

-- Checklist items
create table if not exists checklist_items (
  id uuid primary key default uuid_generate_v4(),
  checklist_id uuid references checklists(id) on delete cascade,
  content text not null,
  completed boolean default false,
  position int not null
);

-- Comments (with @mentions support via text parsing)
create table if not exists comments (
  id uuid primary key default uuid_generate_v4(),
  card_id uuid references cards(id) on delete cascade,
  user_id uuid references users(id),
  content text not null,
  created_at timestamp with time zone default now()
);

-- Enhance cards table with new properties
alter table cards add column if not exists title text;
alter table cards add column if not exists description text;
alter table cards add column if not exists due_date timestamp with time zone;
alter table cards add column if not exists assignee_id uuid references users(id);
alter table cards add column if not exists cover_color text;

-- Enhance boards table
alter table boards add column if not exists description text;
alter table boards add column if not exists background text default '#1e3a5f';

-- Indexes for performance
create index if not exists idx_users_auth on users(auth_id);
create index if not exists idx_board_members_user on board_members(user_id);
create index if not exists idx_labels_board on labels(board_id);
create index if not exists idx_card_labels_card on card_labels(card_id);
create index if not exists idx_checklists_card on checklists(card_id);
create index if not exists idx_checklist_items_checklist on checklist_items(checklist_id);
create index if not exists idx_comments_card on comments(card_id);
create index if not exists idx_cards_due_date on cards(due_date) where due_date is not null;
