-- Apps table
CREATE TABLE apps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(63) NOT NULL,
    github_repo VARCHAR(500),
    github_branch VARCHAR(255) DEFAULT 'main',
    github_installation_id BIGINT REFERENCES github_installations(installation_id) ON DELETE SET NULL,
    fly_app_id VARCHAR(255),
    region VARCHAR(10) DEFAULT 'iad',
    auto_deploy BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, slug)
);

-- Environment variables table (encrypted)
CREATE TABLE env_vars (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    key VARCHAR(255) NOT NULL,
    value_encrypted BYTEA NOT NULL,
    nonce BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (app_id, key)
);

-- Indexes
CREATE INDEX idx_apps_team_id ON apps(team_id);
CREATE INDEX idx_apps_slug ON apps(team_id, slug);
CREATE INDEX idx_apps_github_repo ON apps(github_repo);
CREATE INDEX idx_env_vars_app_id ON env_vars(app_id);

-- Triggers
CREATE TRIGGER update_apps_updated_at
    BEFORE UPDATE ON apps
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_env_vars_updated_at
    BEFORE UPDATE ON env_vars
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
