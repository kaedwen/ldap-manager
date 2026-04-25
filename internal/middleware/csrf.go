package middleware

import (
	"net/http"

	"github.com/kaedwen/ldap-manager/internal/service"
)

// CSRFMiddleware validates CSRF tokens on state-changing requests
func CSRFMiddleware(sessionService *service.SessionService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only validate CSRF on state-changing methods
			if r.Method == http.MethodPost || r.Method == http.MethodPut ||
				r.Method == http.MethodDelete || r.Method == http.MethodPatch {

				// Get CSRF token from session (stored in context by AuthMiddleware)
				sessionToken := GetCSRFToken(r)
				if sessionToken == "" {
					// No session CSRF token - reject
					http.Error(w, "Forbidden: Invalid CSRF token", http.StatusForbidden)
					return
				}

				// Get CSRF token from form
				if err := r.ParseForm(); err != nil {
					http.Error(w, "Bad Request", http.StatusBadRequest)
					return
				}
				providedToken := r.FormValue("csrf_token")

				// Validate CSRF token
				if !sessionService.ValidateCSRFToken(sessionToken, providedToken) {
					http.Error(w, "Forbidden: Invalid CSRF token", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
