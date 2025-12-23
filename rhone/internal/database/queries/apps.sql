-- name: CreateApp :one
INSERT INTO apps (team_id, name, slug, github_repo, github_branch, github_installation_id, region, auto_deploy)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetApp :one
SELECT * FROM apps WHERE id = $1;

-- name: GetAppBySlug :one
SELECT * FROM apps WHERE team_id = $1 AND slug = $2;

-- name: GetTeamApps :many
SELECT * FROM apps WHERE team_id = $1 ORDER BY created_at DESC;

-- name: UpdateApp :one
UPDATE apps SET
    name = COALESCE(NULLIF(@name::text, ''), name),
    github_branch = COALESCE(NULLIF(@github_branch::text, ''), github_branch),
    region = COALESCE(NULLIF(@region::text, ''), region),
    auto_deploy = @auto_deploy,
    updated_at = NOW()
WHERE id = @id AND team_id = @team_id
RETURNING *;

-- name: DeleteApp :exec
DELETE FROM apps WHERE id = $1 AND team_id = $2;

-- name: SlugExists :one
SELECT EXISTS(SELECT 1 FROM apps WHERE team_id = $1 AND slug = $2);

-- name: GetAppsByGitHubRepo :many
SELECT * FROM apps WHERE github_repo = $1;

-- Environment variable queries

-- name: CreateEnvVar :one
INSERT INTO env_vars (app_id, key, value_encrypted, nonce)
VALUES ($1, $2, $3, $4)
ON CONFLICT (app_id, key) DO UPDATE SET
    value_encrypted = EXCLUDED.value_encrypted,
    nonce = EXCLUDED.nonce,
    updated_at = NOW()
RETURNING *;

-- name: GetEnvVar :one
SELECT * FROM env_vars WHERE app_id = $1 AND key = $2;

-- name: GetAppEnvVars :many
SELECT * FROM env_vars WHERE app_id = $1 ORDER BY key;

-- name: DeleteEnvVar :exec
DELETE FROM env_vars WHERE app_id = $1 AND key = $2;

-- name: DeleteAllAppEnvVars :exec
DELETE FROM env_vars WHERE app_id = $1;
