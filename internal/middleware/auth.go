package middleware

import (
	"context"
	"net/http"

	"github.com/kaedwen/ldap-manager/internal/service"
)

type contextKey string

const (
	UserDNKey    contextKey = "user_dn"
	CSRFTokenKey contextKey = "csrf_token"
)

// AuthMiddleware validates session and ensures user is authenticated
func AuthMiddleware(sessionService *service.SessionService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get session cookie
			cookie, err := r.Cookie("session")
			if err != nil {
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			// Validate session
			userDN, csrfToken, err := sessionService.ValidateSession(cookie.Value)
			if err != nil {
				// Clear invalid cookie
				http.SetCookie(w, &http.Cookie{
					Name:     "session",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
					Secure:   true,
					SameSite: http.SameSiteStrictMode,
				})
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			// Store user DN and CSRF token in context
			ctx := context.WithValue(r.Context(), UserDNKey, userDN)
			ctx = context.WithValue(ctx, CSRFTokenKey, csrfToken)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserDN extracts the user DN from the request context
func GetUserDN(r *http.Request) string {
	userDN, _ := r.Context().Value(UserDNKey).(string)
	return userDN
}

// GetCSRFToken extracts the CSRF token from the request context
func GetCSRFToken(r *http.Request) string {
	csrfToken, _ := r.Context().Value(CSRFTokenKey).(string)
	return csrfToken
}
