package auth

import (
	"encoding/gob"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
)

func init() {
	// Register types for gob encoding
	gob.Register(uuid.UUID{})
	gob.Register(SessionData{})
}

// SessionData holds the user session information stored in the cookie.
type SessionData struct {
	UserID    uuid.UUID
	Email     string
	Username  string
	AvatarURL string
	TeamID    uuid.UUID // Current team context
	TeamSlug  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionStore manages session cookies.
type SessionStore struct {
	cookie *securecookie.SecureCookie
	name   string
	maxAge int
	secure bool
}

// NewSessionStore creates a new session store.
// The secret must be at least 64 bytes: first 32 for hash key, next 32 for block key.
func NewSessionStore(secret string, maxAge time.Duration, secure bool) *SessionStore {
	// Use secret for both hash and encryption keys
	hashKey := []byte(secret)[:32]
	blockKey := []byte(secret)[32:64]

	return &SessionStore{
		cookie: securecookie.New(hashKey, blockKey),
		name:   "rhone_session",
		maxAge: int(maxAge.Seconds()),
		secure: secure,
	}
}

// Get retrieves the session data from the request cookie.
func (s *SessionStore) Get(r *http.Request) (*SessionData, error) {
	cookie, err := r.Cookie(s.name)
	if err != nil {
		return nil, err
	}

	var data SessionData
	if err := s.cookie.Decode(s.name, cookie.Value, &data); err != nil {
		return nil, err
	}

	// Check expiration
	if time.Now().After(data.ExpiresAt) {
		return nil, http.ErrNoCookie
	}

	return &data, nil
}

// Set stores the session data in a cookie.
func (s *SessionStore) Set(w http.ResponseWriter, data *SessionData) error {
	data.CreatedAt = time.Now()
	data.ExpiresAt = time.Now().Add(time.Duration(s.maxAge) * time.Second)

	encoded, err := s.cookie.Encode(s.name, data)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.name,
		Value:    encoded,
		Path:     "/",
		MaxAge:   s.maxAge,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}

// Clear removes the session cookie.
func (s *SessionStore) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}
