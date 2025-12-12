package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vangoframework/rhone/internal/auth"
)

// testSecret is a 64-byte secret for testing
const testSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestSessionStore_SetAndGet(t *testing.T) {
	store := auth.NewSessionStore(testSecret, time.Hour, false)

	session := &auth.SessionData{
		UserID:    uuid.New(),
		Username:  "testuser",
		Email:     "test@example.com",
		AvatarURL: "https://example.com/avatar.png",
		TeamID:    uuid.New(),
		TeamSlug:  "testteam",
	}

	// Set session
	w := httptest.NewRecorder()
	err := store.Set(w, session)
	require.NoError(t, err)

	// Get cookies from response
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "rhone_session", cookies[0].Name)

	// Create request with cookie
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookies[0])

	// Get session
	got, err := store.Get(req)
	require.NoError(t, err)
	assert.Equal(t, session.UserID, got.UserID)
	assert.Equal(t, session.Username, got.Username)
	assert.Equal(t, session.Email, got.Email)
	assert.Equal(t, session.AvatarURL, got.AvatarURL)
	assert.Equal(t, session.TeamID, got.TeamID)
	assert.Equal(t, session.TeamSlug, got.TeamSlug)
}

func TestSessionStore_NoCookie(t *testing.T) {
	store := auth.NewSessionStore(testSecret, time.Hour, false)

	req := httptest.NewRequest("GET", "/", nil)

	_, err := store.Get(req)
	assert.Error(t, err)
}

func TestSessionStore_ExpiredSession(t *testing.T) {
	// Create store with negative max age (already expired)
	store := auth.NewSessionStore(testSecret, -time.Hour, false)

	session := &auth.SessionData{
		UserID:   uuid.New(),
		Username: "testuser",
	}

	// Set session
	w := httptest.NewRecorder()
	err := store.Set(w, session)
	require.NoError(t, err)

	// Get cookies
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)

	// Create request with cookie
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookies[0])

	// Get session should fail due to expiration
	_, err = store.Get(req)
	assert.Error(t, err)
}

func TestSessionStore_Clear(t *testing.T) {
	store := auth.NewSessionStore(testSecret, time.Hour, false)

	w := httptest.NewRecorder()
	store.Clear(w)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "rhone_session", cookies[0].Name)
	assert.Equal(t, "", cookies[0].Value)
	assert.Equal(t, -1, cookies[0].MaxAge)
}

func TestSessionStore_SecureCookie(t *testing.T) {
	// Test with secure=true (production)
	store := auth.NewSessionStore(testSecret, time.Hour, true)

	session := &auth.SessionData{
		UserID:   uuid.New(),
		Username: "testuser",
	}

	w := httptest.NewRecorder()
	err := store.Set(w, session)
	require.NoError(t, err)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.True(t, cookies[0].Secure)
	assert.True(t, cookies[0].HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
}

func TestSessionStore_InvalidCookie(t *testing.T) {
	store := auth.NewSessionStore(testSecret, time.Hour, false)

	// Create request with invalid cookie value
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "rhone_session",
		Value: "invalid-cookie-value",
	})

	_, err := store.Get(req)
	assert.Error(t, err)
}
