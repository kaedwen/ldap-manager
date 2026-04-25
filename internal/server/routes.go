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

	// Health check endpoint
	mux.HandleFunc("GET /health", s.healthHandler.Check)

	// Public reset routes (with rate limiting and CSRF)
	mux.Handle("GET /reset", middleware.RateLimitMiddleware(s.resetLimiter)(http.HandlerFunc(s.resetHandler.ShowForm)))
	mux.Handle("POST /reset", middleware.RateLimitMiddleware(s.resetLimiter)(http.HandlerFunc(s.resetHandler.Submit)))

	// Admin login (public, with rate limiting)
	mux.Handle("GET /admin/login", http.HandlerFunc(s.adminHandler.LoginPage))
	mux.Handle("POST /admin/login", middleware.RateLimitMiddleware(s.loginLimiter)(http.HandlerFunc(s.adminHandler.Login)))

	// Admin protected routes (require authentication)
	authMW := middleware.AuthMiddleware(s.sessionService)
	csrfMW := middleware.CSRFMiddleware(s.sessionService)

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
