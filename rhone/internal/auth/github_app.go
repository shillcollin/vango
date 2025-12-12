package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GitHubApp handles GitHub App authentication and API calls.
type GitHubApp struct {
	AppID      int64
	PrivateKey *rsa.PrivateKey
	httpClient *http.Client
}

// NewGitHubApp creates a new GitHub App client.
// The privateKeyPEM can be either raw PEM format or base64-encoded PEM.
func NewGitHubApp(appID int64, privateKeyPEM string) (*GitHubApp, error) {
	// Handle base64-encoded key
	decoded, err := base64.StdEncoding.DecodeString(privateKeyPEM)
	if err == nil {
		privateKeyPEM = string(decoded)
	}

	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		pkcs8Key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		key, ok = pkcs8Key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("expected RSA private key")
		}
	}

	return &GitHubApp{
		AppID:      appID,
		PrivateKey: key,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// GenerateJWT creates a JWT for authenticating as the GitHub App.
func (g *GitHubApp) GenerateJWT() (string, error) {
	now := time.Now()

	claims := jwt.MapClaims{
		"iat": now.Add(-60 * time.Second).Unix(), // 60 seconds in the past
		"exp": now.Add(10 * time.Minute).Unix(),  // 10 minutes max
		"iss": strconv.FormatInt(g.AppID, 10),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(g.PrivateKey)
}

// Installation represents a GitHub App installation.
type Installation struct {
	ID                  int64      `json:"id"`
	Account             Account    `json:"account"`
	RepositorySelection string     `json:"repository_selection"` // "all" or "selected"
	AccessTokensURL     string     `json:"access_tokens_url"`
	SuspendedAt         *time.Time `json:"suspended_at"`
}

// Account represents the user or organization that installed the app.
type Account struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Type  string `json:"type"` // "User" or "Organization"
}

// GetInstallation fetches an installation by ID.
func (g *GitHubApp) GetInstallation(ctx context.Context, installationID int64) (*Installation, error) {
	jwtToken, err := g.GenerateJWT()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%d", installationID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api error %d: %s", resp.StatusCode, string(body))
	}

	var installation Installation
	if err := json.NewDecoder(resp.Body).Decode(&installation); err != nil {
		return nil, err
	}

	return &installation, nil
}

// InstallationToken is a temporary access token for an installation.
type InstallationToken struct {
	Token       string            `json:"token"`
	ExpiresAt   time.Time         `json:"expires_at"`
	Permissions map[string]string `json:"permissions"`
}

// GetInstallationToken exchanges installation ID for a temporary access token.
func (g *GitHubApp) GetInstallationToken(ctx context.Context, installationID int64) (*InstallationToken, error) {
	jwtToken, err := g.GenerateJWT()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api error %d: %s", resp.StatusCode, string(body))
	}

	var token InstallationToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

// Repository represents a GitHub repository.
type Repository struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Private       bool      `json:"private"`
	Description   string    `json:"description"`
	DefaultBranch string    `json:"default_branch"`
	CloneURL      string    `json:"clone_url"`
	HTMLURL       string    `json:"html_url"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ListInstallationRepos lists all repositories accessible to an installation.
func (g *GitHubApp) ListInstallationRepos(ctx context.Context, installationID int64) ([]Repository, error) {
	token, err := g.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	var allRepos []Repository
	page := 1

	for {
		url := fmt.Sprintf("https://api.github.com/installation/repositories?per_page=100&page=%d", page)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+token.Token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := g.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		var result struct {
			TotalCount   int          `json:"total_count"`
			Repositories []Repository `json:"repositories"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		allRepos = append(allRepos, result.Repositories...)

		if len(allRepos) >= result.TotalCount {
			break
		}
		page++
	}

	return allRepos, nil
}

// CloneURL returns the authenticated clone URL for a repository.
func (g *GitHubApp) CloneURL(token, repoFullName string) string {
	return fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, repoFullName)
}
