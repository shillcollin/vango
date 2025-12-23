package domain

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"regexp"
	"strings"
)

// Crypto and environment variable errors
var (
	ErrInvalidKeyLength = errors.New("encryption key must be exactly 32 bytes")
	ErrDecryptionFailed = errors.New("decryption failed")
	ErrInvalidEnvKey    = errors.New("environment variable key must start with a letter and contain only uppercase letters, numbers, and underscores")
	ErrReservedEnvKey   = errors.New("environment variable key is reserved")
	ErrEmptyEnvKey      = errors.New("environment variable key cannot be empty")
)

// envKeyRegex validates environment variable keys:
// - Must start with an uppercase letter
// - Can contain uppercase letters, digits, and underscores
var envKeyRegex = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// Reserved environment variable prefixes that are managed by the system
var reservedPrefixes = []string{
	"FLY_",
	"VANGO_",
	"INTERNAL_",
}

// Reserved environment variable names that cannot be set by users
var reservedNames = map[string]bool{
	"PORT":         true,
	"HOST":         true,
	"PATH":         true,
	"HOME":         true,
	"USER":         true,
	"SHELL":        true,
	"DATABASE_URL": true, // Will be set by Rhone
}

// EnvVarCrypto handles AES-256-GCM encryption for environment variables.
type EnvVarCrypto struct {
	aead cipher.AEAD
}

// NewEnvVarCrypto creates a new EnvVarCrypto with the given 32-byte key.
func NewEnvVarCrypto(key string) (*EnvVarCrypto, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeyLength
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &EnvVarCrypto{aead: aead}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns ciphertext and nonce.
func (e *EnvVarCrypto) Encrypt(plaintext string) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = e.aead.Seal(nil, nonce, []byte(plaintext), nil)
	return ciphertext, nonce, nil
}

// Decrypt decrypts ciphertext using the given nonce.
func (e *EnvVarCrypto) Decrypt(ciphertext, nonce []byte) (string, error) {
	plaintext, err := e.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}
	return string(plaintext), nil
}

// ValidateEnvKey validates an environment variable key.
// Valid keys:
// - Must start with an uppercase letter
// - Can contain only uppercase letters, digits, and underscores
// - Cannot be empty
func ValidateEnvKey(key string) error {
	if key == "" {
		return ErrEmptyEnvKey
	}
	if !envKeyRegex.MatchString(key) {
		return ErrInvalidEnvKey
	}
	return nil
}

// IsReservedEnvKey checks if a key is reserved by the system.
func IsReservedEnvKey(key string) bool {
	upperKey := strings.ToUpper(key)

	// Check reserved names
	if reservedNames[upperKey] {
		return true
	}

	// Check reserved prefixes
	for _, prefix := range reservedPrefixes {
		if strings.HasPrefix(upperKey, prefix) {
			return true
		}
	}

	return false
}
