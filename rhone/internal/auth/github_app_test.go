package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vangoframework/rhone/internal/auth"
)

// generateTestKey creates a test RSA key for testing
func generateTestKey(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	return string(privateKeyPEM), privateKey
}

func TestNewGitHubApp_ValidPEM(t *testing.T) {
	pemKey, _ := generateTestKey(t)

	app, err := auth.NewGitHubApp(123456, pemKey)
	require.NoError(t, err)
	assert.NotNil(t, app)
	assert.Equal(t, int64(123456), app.AppID)
	assert.NotNil(t, app.PrivateKey)
}

func TestNewGitHubApp_Base64Encoded(t *testing.T) {
	pemKey, _ := generateTestKey(t)

	// Base64 encode the PEM
	base64Key := base64.StdEncoding.EncodeToString([]byte(pemKey))

	app, err := auth.NewGitHubApp(123456, base64Key)
	require.NoError(t, err)
	assert.NotNil(t, app)
	assert.Equal(t, int64(123456), app.AppID)
}

func TestNewGitHubApp_InvalidKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{
			name: "empty key",
			key:  "",
		},
		{
			name: "invalid PEM",
			key:  "not a valid PEM",
		},
		{
			name: "invalid base64",
			key:  "!!!invalid-base64!!!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := auth.NewGitHubApp(123456, tt.key)
			assert.Error(t, err)
		})
	}
}

func TestGitHubApp_GenerateJWT(t *testing.T) {
	pemKey, _ := generateTestKey(t)

	app, err := auth.NewGitHubApp(123456, pemKey)
	require.NoError(t, err)

	jwt, err := app.GenerateJWT()
	require.NoError(t, err)
	assert.NotEmpty(t, jwt)

	// JWT should have three parts separated by dots
	parts := strings.Split(jwt, ".")
	assert.Len(t, parts, 3, "JWT should have 3 parts (header.payload.signature)")

	// Each part should be non-empty
	for i, part := range parts {
		assert.NotEmpty(t, part, "JWT part %d should not be empty", i)
	}
}

func TestGitHubApp_CloneURL(t *testing.T) {
	pemKey, _ := generateTestKey(t)

	app, err := auth.NewGitHubApp(123456, pemKey)
	require.NoError(t, err)

	url := app.CloneURL("ghp_testtoken123", "owner/repo")
	assert.Equal(t, "https://x-access-token:ghp_testtoken123@github.com/owner/repo.git", url)
}

func TestGitHubApp_CloneURL_SpecialChars(t *testing.T) {
	pemKey, _ := generateTestKey(t)

	app, err := auth.NewGitHubApp(123456, pemKey)
	require.NoError(t, err)

	// Test with a repo that has dashes
	url := app.CloneURL("token", "my-org/my-repo")
	assert.Equal(t, "https://x-access-token:token@github.com/my-org/my-repo.git", url)
}
