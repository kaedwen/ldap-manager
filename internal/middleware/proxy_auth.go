package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/kaedwen/ldap-manager/internal/config"
	"github.com/kaedwen/ldap-manager/internal/domain"
	"github.com/kaedwen/ldap-manager/internal/service"
)

// ProxyAuth validates authentication via reverse proxy headers (Authelia, Authentik, Keycloak, etc.)
func ProxyAuth(cfg *config.Config, sessionService *service.SessionService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user header from proxy (e.g., Remote-User, X-Forwarded-User, X-Auth-User)
			remoteUser := r.Header.Get(cfg.Server.Auth.HeaderUser)
			remoteGroups := r.Header.Get(cfg.Server.Auth.HeaderGroups)
			remoteEmail := r.Header.Get(cfg.Server.Auth.HeaderEmail)
			remoteName := r.Header.Get(cfg.Server.Auth.HeaderName)

			if remoteUser == "" {
				slog.Warn("proxy auth: no user header found",
					"path", r.URL.Path,
					"header", cfg.Server.Auth.HeaderUser)
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			// Verify user is in required group
			groups := parseGroups(remoteGroups)
			isAdmin := false
			requiredGroup := cfg.Server.Auth.RequireGroup
			for _, group := range groups {
				// Case-insensitive group matching
				if strings.EqualFold(group, requiredGroup) {
					isAdmin = true
					break
				}
			}

			if !isAdmin {
				slog.Warn("proxy auth: user not in required group",
					"user", remoteUser,
					"groups", remoteGroups,
					"required_group", requiredGroup)
				http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
				return
			}

			// Construct user DN from username
			// Format: uid=<username>,ou=users,<base_dn>
			userDN := "uid=" + remoteUser + ",ou=users," + cfg.LDAP.BaseDN

			slog.Info("proxy auth: user authenticated",
				"user", remoteUser,
				"dn", userDN,
				"groups", remoteGroups,
				"email", remoteEmail)

			// Check if valid session already exists
			existingCookie, err := r.Cookie("session")
			var sessionCookie, csrfToken string

			if err == nil && existingCookie != nil {
				// Try to validate existing session
				existingUserDN, existingCSRF, err := sessionService.ValidateSession(existingCookie.Value)
				if err == nil && existingUserDN == userDN {
					// Valid session exists for this user, reuse it
					sessionCookie = existingCookie.Value
					csrfToken = existingCSRF
					slog.Debug("proxy auth: reusing existing session", "user", remoteUser)
				}
			}

			// Create new session if no valid session exists
			if sessionCookie == "" {
				sessionCookie, csrfToken, err = sessionService.CreateSession(userDN)
				if err != nil {
					slog.Error("proxy auth: failed to create session", "error", err, "user", remoteUser)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				// Set session cookie
				http.SetCookie(w, &http.Cookie{
					Name:     "session",
					Value:    sessionCookie,
					Path:     "/",
					MaxAge:   cfg.Server.Session.MaxAge,
					HttpOnly: true,
					Secure:   cfg.Server.TLS.Enabled,
					SameSite: http.SameSiteStrictMode,
				})
				slog.Debug("proxy auth: created new session", "user", remoteUser)
			}

			// Store user info in context for handlers
			user := &domain.User{
				DN:          userDN,
				UID:         remoteUser,
				Email:       remoteEmail,
				CN:          remoteName,
				DisplayName: remoteName,
			}
			ctx := SetUserContext(r.Context(), user)
			ctx = SetCSRFToken(ctx, csrfToken)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// parseGroups parses the groups header (comma-separated or space-separated)
func parseGroups(groupsHeader string) []string {
	if groupsHeader == "" {
		return nil
	}

	// Try comma-separated first (Authelia format)
	groups := strings.Split(groupsHeader, ",")

	// If no commas found, try space-separated (some proxies use this)
	if len(groups) == 1 {
		groups = strings.Split(groupsHeader, " ")
	}

	result := make([]string, 0, len(groups))
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g != "" {
			result = append(result, g)
		}
	}
	return result
}
