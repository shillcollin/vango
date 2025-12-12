package handlers

import (
	"log/slog"

	"github.com/vangoframework/rhone/internal/auth"
	"github.com/vangoframework/rhone/internal/config"
	"github.com/vangoframework/rhone/internal/database"
	"github.com/vangoframework/rhone/internal/database/queries"
)

// Handlers contains all HTTP handler dependencies.
type Handlers struct {
	config    *config.Config
	db        *database.DB
	queries   *queries.Queries
	sessions  *auth.SessionStore
	github    *auth.GitHubOAuth
	githubApp *auth.GitHubApp
	logger    *slog.Logger
}

// New creates a new Handlers instance with all dependencies.
func New(
	cfg *config.Config,
	db *database.DB,
	sessions *auth.SessionStore,
	github *auth.GitHubOAuth,
	githubApp *auth.GitHubApp,
	logger *slog.Logger,
) *Handlers {
	return &Handlers{
		config:    cfg,
		db:        db,
		queries:   queries.New(db.Pool),
		sessions:  sessions,
		github:    github,
		githubApp: githubApp,
		logger:    logger,
	}
}
