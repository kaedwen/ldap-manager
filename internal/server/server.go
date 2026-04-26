package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaedwen/ldap-manager/internal/config"
	"github.com/kaedwen/ldap-manager/internal/handler"
	"github.com/kaedwen/ldap-manager/internal/middleware"
	"github.com/kaedwen/ldap-manager/internal/service"
)

// Server represents the HTTP server
type Server struct {
	config          *config.Config
	httpServer      *http.Server
	healthServer    *http.Server
	sessionService  *service.SessionService
	authService     *service.AuthService
	resetService    *service.ResetService
	adminHandler    *handler.AdminHandler
	resetHandler    *handler.ResetHandler
	healthHandler   *handler.HealthHandler
	loginLimiter    *middleware.RateLimiter
	resetLimiter    *middleware.RateLimiter
}

// NewServer creates a new HTTP server
func NewServer(
	cfg *config.Config,
	sessionService *service.SessionService,
	authService *service.AuthService,
	resetService *service.ResetService,
	adminHandler *handler.AdminHandler,
	resetHandler *handler.ResetHandler,
	healthHandler *handler.HealthHandler,
) *Server {
	return &Server{
		config:         cfg,
		sessionService: sessionService,
		authService:    authService,
		resetService:   resetService,
		adminHandler:   adminHandler,
		resetHandler:   resetHandler,
		healthHandler:  healthHandler,
		loginLimiter:   middleware.NewRateLimiter(cfg.RateLimit.LoginPerIPPerHour),
		resetLimiter:   middleware.NewRateLimiter(cfg.RateLimit.ResetPerIPPerHour),
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Setup routes
	mux := s.setupRoutes()

	// Create main HTTP server
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Create separate health check server (no auth, no middleware)
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("GET /health", s.healthHandler.Check)
	healthMux.HandleFunc("GET /ready", s.healthHandler.Check)
	healthMux.HandleFunc("GET /live", s.healthHandler.Check)

	healthAddr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.HealthPort)
	s.healthServer = &http.Server{
		Addr:         healthAddr,
		Handler:      healthMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Start health server in goroutine
	go func() {
		slog.Info("starting health check server", "addr", healthAddr)
		if err := s.healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("health server error", "error", err)
		}
	}()

	// Start main server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if s.config.Server.TLS.Enabled {
			slog.Info("starting HTTPS server", "addr", addr)
			errChan <- s.httpServer.ListenAndServeTLS(
				s.config.Server.TLS.CertFile,
				s.config.Server.TLS.KeyFile,
			)
		} else {
			slog.Info("starting HTTP server", "addr", addr)
			errChan <- s.httpServer.ListenAndServe()
		}
	}()

	// Wait for interrupt signal or error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case sig := <-sigChan:
		slog.Info("received shutdown signal", "signal", sig)
		return s.Shutdown()
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	slog.Info("shutting down servers gracefully")

	// Shutdown health server
	if s.healthServer != nil {
		if err := s.healthServer.Shutdown(ctx); err != nil {
			slog.Error("health server shutdown failed", "error", err)
		}
	}

	// Shutdown main server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	slog.Info("servers stopped")
	return nil
}
