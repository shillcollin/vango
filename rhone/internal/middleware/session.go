package middleware

import (
	"context"
	"net/http"

	"github.com/vangoframework/rhone/internal/auth"
)

type contextKey string

// SessionContextKey is the context key for the session.
const SessionContextKey contextKey = "session"

// Session returns a middleware that loads the session into the request context.
func Session(store *auth.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := store.Get(r)
			if err == nil && session != nil {
				ctx := context.WithValue(r.Context(), SessionContextKey, session)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetSession retrieves the session from context.
func GetSession(ctx context.Context) *auth.SessionData {
	session, ok := ctx.Value(SessionContextKey).(*auth.SessionData)
	if !ok {
		return nil
	}
	return session
}
