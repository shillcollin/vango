-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByGitHubID :one
SELECT * FROM users WHERE github_id = $1;

-- name: UpsertUser :one
INSERT INTO users (github_id, github_username, email, avatar_url)
VALUES ($1, $2, $3, $4)
ON CONFLICT (github_id) DO UPDATE SET
    github_username = EXCLUDED.github_username,
    email = COALESCE(EXCLUDED.email, users.email),
    avatar_url = COALESCE(EXCLUDED.avatar_url, users.avatar_url),
    updated_at = NOW()
RETURNING *;

-- name: GetTeam :one
SELECT * FROM teams WHERE id = $1;

-- name: GetTeamBySlug :one
SELECT * FROM teams WHERE slug = $1;

-- name: CreateTeam :one
INSERT INTO teams (name, slug, plan)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserTeams :many
SELECT t.* FROM teams t
JOIN team_members tm ON t.id = tm.team_id
WHERE tm.user_id = $1
ORDER BY t.created_at;

-- name: GetTeamMember :one
SELECT * FROM team_members WHERE team_id = $1 AND user_id = $2;

-- name: AddTeamMember :one
INSERT INTO team_members (team_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTeamMembers :many
SELECT u.*, tm.role, tm.created_at as joined_at
FROM users u
JOIN team_members tm ON u.id = tm.user_id
WHERE tm.team_id = $1
ORDER BY tm.created_at;
