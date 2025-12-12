package middleware

import (
	"net/http"
)

// RequireAuth redirects unauthenticated users to login.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := GetSession(r.Context())
		if session == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// OptionalAuth allows both authenticated and unauthenticated access.
// Session is already loaded by Session middleware if present.
func OptionalAuth(next http.Handler) http.Handler {
	return next
}
