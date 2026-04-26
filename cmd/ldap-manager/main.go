package main

import (
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"os"

	"github.com/kaedwen/ldap-manager/internal/config"
	"github.com/kaedwen/ldap-manager/internal/handler"
	"github.com/kaedwen/ldap-manager/internal/repository"
	"github.com/kaedwen/ldap-manager/internal/server"
	"github.com/kaedwen/ldap-manager/internal/service"
	"github.com/kaedwen/ldap-manager/pkg/validator"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "configs/config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	defer cfg.Close()

	// Initialize logger
	logLevel := slog.LevelInfo
	switch cfg.Logging.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	var logHandler slog.Handler
	if cfg.Logging.Format == "json" {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	slog.Info("ldap-manager starting",
		"version", "0.1.0",
		"port", cfg.Server.Port,
		"tls_enabled", cfg.Server.TLS.Enabled,
	)

	// Initialize LDAP repository
	ldapRepo, err := repository.NewLDAPRepository(cfg)
	if err != nil {
		slog.Error("failed to initialize LDAP repository", "error", err)
		os.Exit(1)
	}
	defer ldapRepo.Close()

	// Initialize services
	authService := service.NewAuthService(ldapRepo, cfg.LDAP.AdminGroupDN)
	sessionService := service.NewSessionService(cfg.Server.Session.Secret, cfg.Server.Session.MaxAge)
	notificationService := service.NewNotificationService(&cfg.Email)

	// Build base URL for reset links
	baseURL := fmt.Sprintf("https://%s:%d", cfg.Server.Host, cfg.Server.Port)
	if cfg.Server.Host == "0.0.0.0" {
		baseURL = fmt.Sprintf("https://localhost:%d", cfg.Server.Port)
	}

	// Create password requirements from config
	passwordReqs := validator.PasswordRequirements{
		MinLength:      cfg.Password.MinLength,
		RequireUpper:   cfg.Password.RequireUpper,
		RequireLower:   cfg.Password.RequireLower,
		RequireDigit:   cfg.Password.RequireDigit,
		RequireSpecial: cfg.Password.RequireSpecial,
	}

	resetService := service.NewResetService(
		ldapRepo,
		cfg.Token.LengthBytes,
		cfg.Token.ValidityDays,
		baseURL,
		notificationService,
		passwordReqs,
	)

	// Load templates
	templates := template.Must(template.ParseFiles(
		"web/templates/base.html",
		"web/templates/admin/login.html",
		"web/templates/admin/dashboard.html",
		"web/templates/reset/form.html",
		"web/templates/reset/success.html",
		"web/templates/reset/error.html",
	))

	// Initialize handlers
	adminHandler := handler.NewAdminHandler(authService, sessionService, resetService, ldapRepo, templates, baseURL, cfg)
	resetHandler := handler.NewResetHandler(resetService, templates)
	healthHandler := handler.NewHealthHandler()

	// Initialize and start server
	srv := server.NewServer(
		cfg,
		sessionService,
		authService,
		resetService,
		adminHandler,
		resetHandler,
		healthHandler,
	)

	slog.Info("server starting",
		"addr", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		"tls", cfg.Server.TLS.Enabled,
	)

	if err := srv.Start(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
