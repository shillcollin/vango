-- GitHub App installations
CREATE TABLE github_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    installation_id BIGINT UNIQUE NOT NULL,
    account_type VARCHAR(50) NOT NULL,  -- 'User' or 'Organization'
    account_login VARCHAR(255) NOT NULL,
    account_id BIGINT NOT NULL,
    suspended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for looking up installations by team
CREATE INDEX idx_github_installations_team_id ON github_installations(team_id);

-- Index for looking up by account (to find which team owns an account)
CREATE INDEX idx_github_installations_account ON github_installations(account_login);

-- Trigger for updated_at
CREATE TRIGGER update_github_installations_updated_at
    BEFORE UPDATE ON github_installations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
