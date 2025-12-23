package handlers

import (
	"fmt"
	"log/slog"

	"github.com/vangoframework/rhone/internal/auth"
	"github.com/vangoframework/rhone/internal/config"
	"github.com/vangoframework/rhone/internal/database"
	"github.com/vangoframework/rhone/internal/database/queries"
	"github.com/vangoframework/rhone/internal/domain"
)

// Handlers contains all HTTP handler dependencies.
type Handlers struct {
	config    *config.Config
	db        *database.DB
	queries   *queries.Queries
	sessions  *auth.SessionStore
	github    *auth.GitHubOAuth
	githubApp *auth.GitHubApp
	crypto    *domain.EnvVarCrypto
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
) (*Handlers, error) {
	crypto, err := domain.NewEnvVarCrypto(cfg.EnvEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize env var crypto: %w", err)
	}

	return &Handlers{
		config:    cfg,
		db:        db,
		queries:   queries.New(db.Pool),
		sessions:  sessions,
		github:    github,
		githubApp: githubApp,
		crypto:    crypto,
		logger:    logger,
	}, nil
}
