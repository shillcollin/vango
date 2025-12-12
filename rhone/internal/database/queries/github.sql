-- name: CreateGitHubInstallation :one
INSERT INTO github_installations (
    team_id, installation_id, account_type, account_login, account_id
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (installation_id) DO UPDATE SET
    team_id = EXCLUDED.team_id,
    account_type = EXCLUDED.account_type,
    account_login = EXCLUDED.account_login,
    account_id = EXCLUDED.account_id,
    suspended_at = NULL,
    updated_at = NOW()
RETURNING *;

-- name: GetGitHubInstallation :one
SELECT * FROM github_installations WHERE id = $1;

-- name: GetGitHubInstallationByInstallationID :one
SELECT * FROM github_installations WHERE installation_id = $1;

-- name: GetTeamGitHubInstallations :many
SELECT * FROM github_installations
WHERE team_id = $1
ORDER BY created_at DESC;

-- name: DeleteGitHubInstallation :exec
DELETE FROM github_installations WHERE installation_id = $1;

-- name: SuspendGitHubInstallation :exec
UPDATE github_installations
SET suspended_at = NOW(), updated_at = NOW()
WHERE installation_id = $1;

-- name: UnsuspendGitHubInstallation :exec
UPDATE github_installations
SET suspended_at = NULL, updated_at = NOW()
WHERE installation_id = $1;
