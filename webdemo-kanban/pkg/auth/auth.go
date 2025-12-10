package auth

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

// VerifyToken verifies a Supabase JWT and returns the claims.
// It handles Base64 decoding of the SUPABASE_JWT_SECRET if necessary.
func VerifyToken(tokenString string) (jwt.MapClaims, error) {
	secretRaw := os.Getenv("SUPABASE_JWT_SECRET")
	if secretRaw == "" {
		return nil, fmt.Errorf("SUPABASE_JWT_SECRET not set")
	}

	// Supabase JWT secret is often base64 encoded check based on characters
	// Try to decode it
	secret, err := base64.StdEncoding.DecodeString(secretRaw)
	if err != nil {
		// If fails, assume it's a raw string
		secret = []byte(secretRaw)
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("verify token failed: %w", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}
