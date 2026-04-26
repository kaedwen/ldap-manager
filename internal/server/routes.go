package server

import (
	"net/http"

	"github.com/kaedwen/ldap-manager/internal/middleware"
)

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Static files
	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// Note: /health endpoint is on separate health_port (no auth, no middleware)

	// Public reset routes (with rate limiting and CSRF)
	mux.Handle("GET /reset", middleware.RateLimitMiddleware(s.resetLimiter)(http.HandlerFunc(s.resetHandler.ShowForm)))
	mux.Handle("POST /reset", middleware.RateLimitMiddleware(s.resetLimiter)(http.HandlerFunc(s.resetHandler.Submit)))

	// Choose authentication middleware based on config
	var authMW func(http.Handler) http.Handler
	if s.config.Server.Auth.Mode == "proxy" {
		// Proxy mode: validate headers from OIDC proxy (Authelia, Authentik, Keycloak, etc.)
		authMW = middleware.ProxyAuth(s.config, s.sessionService)
	} else {
		// Internal mode: validate session cookie
		authMW = middleware.AuthMiddleware(s.sessionService)
	}

	// Admin login (only needed in internal mode)
	if s.config.Server.Auth.Mode == "internal" {
		mux.Handle("GET /admin/login", http.HandlerFunc(s.adminHandler.LoginPage))
		mux.Handle("POST /admin/login", middleware.RateLimitMiddleware(s.loginLimiter)(http.HandlerFunc(s.adminHandler.Login)))
	}

	// CSRF middleware
	csrfMW := middleware.CSRFMiddleware(s.sessionService)

	// Admin protected routes (require authentication)
	// Root redirect to dashboard
	mux.Handle("GET /", authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
			return
		}
		http.NotFound(w, r)
	})))
	mux.Handle("GET /admin/dashboard", authMW(http.HandlerFunc(s.adminHandler.Dashboard)))
	mux.Handle("GET /admin/logout", authMW(http.HandlerFunc(s.adminHandler.Logout)))

	// Search and reset (with CSRF protection)
	mux.Handle("POST /admin/search", authMW(csrfMW(http.HandlerFunc(s.adminHandler.SearchUsers))))
	mux.Handle("POST /admin/reset/{userdn}",
		authMW(csrfMW(middleware.RateLimitMiddleware(s.resetLimiter)(http.HandlerFunc(s.adminHandler.GenerateResetToken)))))

	// Wrap entire mux with global middleware
	handler := middleware.LoggingMiddleware(
		middleware.SecurityHeadersMiddleware(mux),
	)

	return handler
}
