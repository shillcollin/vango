package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vangoframework/rhone/internal/domain"
)

func TestNewEnvVarCrypto(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid 32 byte key", "01234567890123456789012345678901", false},
		{"too short", "short", true},
		{"too long", "this-key-is-way-too-long-for-aes-256", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crypto, err := domain.NewEnvVarCrypto(tt.key)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, crypto)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, crypto)
			}
		})
	}
}

func TestEnvVarCrypto_EncryptDecrypt(t *testing.T) {
	key := "01234567890123456789012345678901" // 32 bytes
	crypto, err := domain.NewEnvVarCrypto(key)
	require.NoError(t, err)

	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple string", "my-secret-value"},
		{"empty string", ""},
		{"long string", "this is a very long secret value that contains many characters and should still encrypt and decrypt correctly"},
		{"special characters", "p@$$w0rd!@#$%^&*()_+-=[]{}|;':\",./<>?"},
		{"unicode", "秘密の値 🔐 émoji"},
		{"json", `{"key": "value", "nested": {"foo": "bar"}}`},
		{"multiline", "line1\nline2\nline3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			ciphertext, nonce, err := crypto.Encrypt(tt.plaintext)
			require.NoError(t, err)
			assert.NotEmpty(t, nonce)

			// Ciphertext should be different from plaintext (unless empty)
			if tt.plaintext != "" {
				assert.NotEqual(t, []byte(tt.plaintext), ciphertext)
			}

			// Decrypt
			decrypted, err := crypto.Decrypt(ciphertext, nonce)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestEnvVarCrypto_UniqueNonce(t *testing.T) {
	key := "01234567890123456789012345678901"
	crypto, err := domain.NewEnvVarCrypto(key)
	require.NoError(t, err)

	plaintext := "test-value"

	// Encrypt the same value twice
	_, nonce1, err := crypto.Encrypt(plaintext)
	require.NoError(t, err)

	_, nonce2, err := crypto.Encrypt(plaintext)
	require.NoError(t, err)

	// Nonces should be different (random)
	assert.NotEqual(t, nonce1, nonce2, "Nonces should be unique for each encryption")
}

func TestEnvVarCrypto_WrongNonce(t *testing.T) {
	key := "01234567890123456789012345678901"
	crypto, err := domain.NewEnvVarCrypto(key)
	require.NoError(t, err)

	plaintext := "test-value"
	ciphertext, _, err := crypto.Encrypt(plaintext)
	require.NoError(t, err)

	// Try to decrypt with wrong nonce
	wrongNonce := make([]byte, 12) // GCM nonce is 12 bytes
	_, err = crypto.Decrypt(ciphertext, wrongNonce)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrDecryptionFailed)
}

func TestEnvVarCrypto_DifferentKeys(t *testing.T) {
	key1 := "01234567890123456789012345678901"
	key2 := "98765432109876543210987654321098"

	crypto1, err := domain.NewEnvVarCrypto(key1)
	require.NoError(t, err)

	crypto2, err := domain.NewEnvVarCrypto(key2)
	require.NoError(t, err)

	plaintext := "test-value"
	ciphertext, nonce, err := crypto1.Encrypt(plaintext)
	require.NoError(t, err)

	// Try to decrypt with different key
	_, err = crypto2.Decrypt(ciphertext, nonce)
	assert.Error(t, err)
}

func TestValidateEnvKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr error
	}{
		// Valid keys
		{"simple", "DATABASE_URL", nil},
		{"single letter", "A", nil},
		{"with numbers", "API_KEY_123", nil},
		{"all caps", "MYVAR", nil},
		{"underscores", "MY_VAR_NAME", nil},

		// Invalid keys
		{"empty", "", domain.ErrEmptyEnvKey},
		{"lowercase", "database_url", domain.ErrInvalidEnvKey},
		{"mixed case", "Database_Url", domain.ErrInvalidEnvKey},
		{"starts with number", "1_VAR", domain.ErrInvalidEnvKey},
		{"starts with underscore", "_VAR", domain.ErrInvalidEnvKey},
		{"contains hyphen", "MY-VAR", domain.ErrInvalidEnvKey},
		{"contains space", "MY VAR", domain.ErrInvalidEnvKey},
		{"contains dot", "MY.VAR", domain.ErrInvalidEnvKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateEnvKey(tt.key)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsReservedEnvKey(t *testing.T) {
	tests := []struct {
		key      string
		reserved bool
	}{
		// Reserved names
		{"PORT", true},
		{"HOST", true},
		{"PATH", true},
		{"HOME", true},
		{"DATABASE_URL", true},

		// Reserved prefixes
		{"FLY_APP_NAME", true},
		{"FLY_REGION", true},
		{"VANGO_SECRET", true},
		{"INTERNAL_TOKEN", true},

		// Not reserved
		{"API_KEY", false},
		{"SECRET_KEY", false},
		{"MY_DATABASE_URL", false}, // Different from DATABASE_URL
		{"CUSTOM_PORT", false},     // Different from PORT
		{"FLYWAY_CONFIG", false},   // Doesn't start with FLY_
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := domain.IsReservedEnvKey(tt.key)
			assert.Equal(t, tt.reserved, result)
		})
	}
}
