package handler

import (
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/kaedwen/ldap-manager/internal/config"
	"github.com/kaedwen/ldap-manager/internal/domain"
	"github.com/kaedwen/ldap-manager/internal/middleware"
	"github.com/kaedwen/ldap-manager/internal/repository"
	"github.com/kaedwen/ldap-manager/internal/service"
)

// AdminHandler handles admin-related requests
type AdminHandler struct {
	authService    *service.AuthService
	sessionService *service.SessionService
	resetService   *service.ResetService
	ldapRepo       repository.LDAPRepository
	templates      *template.Template
	baseURL        string
	config         *config.Config
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(
	authService *service.AuthService,
	sessionService *service.SessionService,
	resetService *service.ResetService,
	ldapRepo repository.LDAPRepository,
	templates *template.Template,
	baseURL string,
	cfg *config.Config,
) *AdminHandler {
	return &AdminHandler{
		authService:    authService,
		sessionService: sessionService,
		resetService:   resetService,
		ldapRepo:       ldapRepo,
		templates:      templates,
		baseURL:        baseURL,
		config:         cfg,
	}
}

// TemplateData holds data for template rendering
type TemplateData struct {
	CSRFToken    string
	Error        string
	Success      string
	User         interface{}
	Data         interface{}
	SearchQuery  string
	SearchResult interface{}
	ResetLink    string
}

// LoginPage handles GET /admin/login
func (h *AdminHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	data := TemplateData{
		CSRFToken: "",
	}

	if err := h.templates.ExecuteTemplate(w, "admin_login.html", data); err != nil {
		slog.Error("failed to render login template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Login handles POST /admin/login
func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// Authenticate admin
	user, err := h.authService.AuthenticateAdmin(username, password)
	if err != nil {
		data := TemplateData{
			Error: "Invalid username or password, or insufficient privileges",
		}
		w.WriteHeader(http.StatusUnauthorized)
		if err := h.templates.ExecuteTemplate(w, "admin_login.html", data); err != nil {
			slog.Error("failed to render login template", "error", err)
		}
		return
	}

	// Create session
	sessionCookie, csrfToken, err := h.sessionService.CreateSession(user.DN)
	if err != nil {
		slog.Error("failed to create session", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionCookie,
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	slog.Info("admin logged in", "username", username, "csrf_token_length", len(csrfToken))

	// Redirect to dashboard
	http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
}

// Dashboard handles GET /admin/dashboard
func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.GetCSRFToken(r)

	data := TemplateData{
		CSRFToken: csrfToken,
	}

	if err := h.templates.ExecuteTemplate(w, "admin_dashboard.html", data); err != nil {
		slog.Error("failed to render dashboard template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// SearchUsers handles POST /admin/search
func (h *AdminHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	query := r.FormValue("query")
	csrfToken := middleware.GetCSRFToken(r)

	if query == "" {
		// If query is empty, search for all users
		filter := "(objectClass=inetOrgPerson)"
		users, err := h.ldapRepo.SearchUsers(filter)
		if err != nil {
			slog.Error("failed to search users", "error", err, "filter", filter)
			data := TemplateData{
				CSRFToken:   csrfToken,
				Error:       "No users found in LDAP",
				SearchQuery: query,
			}
			if err := h.templates.ExecuteTemplate(w, "admin_dashboard.html", data); err != nil {
				slog.Error("failed to render dashboard template", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}

		slog.Info("users found", "count", len(users))

		// Return all users
		data := TemplateData{
			CSRFToken:    csrfToken,
			SearchQuery:  query,
			SearchResult: users,
		}

		if err := h.templates.ExecuteTemplate(w, "admin_dashboard.html", data); err != nil {
			slog.Error("failed to render dashboard template", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Search for specific user
	user, err := h.ldapRepo.SearchUserByUID(query)
	if err != nil {
		// Try by email if UID search failed
		user, err = h.ldapRepo.SearchUserByEmail(query)
		if err != nil {
			slog.Warn("user not found", "query", query)
			data := TemplateData{
				CSRFToken:   csrfToken,
				Error:       "User not found: " + query,
				SearchQuery: query,
			}
			if err := h.templates.ExecuteTemplate(w, "admin_dashboard.html", data); err != nil {
				slog.Error("failed to render dashboard template", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}
	}

	slog.Info("user found", "uid", user.UID, "dn", user.DN, "email", user.Email)

	// Return single user (wrap in array for template consistency)
	data := TemplateData{
		CSRFToken:    csrfToken,
		SearchQuery:  query,
		SearchResult: []*domain.User{user},
	}

	if err := h.templates.ExecuteTemplate(w, "admin_dashboard.html", data); err != nil {
		slog.Error("failed to render dashboard template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// GenerateResetToken handles POST /admin/reset/{userdn}
func (h *AdminHandler) GenerateResetToken(w http.ResponseWriter, r *http.Request) {
	userDN := r.PathValue("userdn")
	if userDN == "" {
		http.Error(w, "Bad Request: missing user DN", http.StatusBadRequest)
		return
	}

	// URL-decode the user DN (it may be encoded in the path)
	decodedDN, err := url.PathUnescape(userDN)
	if err != nil {
		http.Error(w, "Bad Request: invalid user DN", http.StatusBadRequest)
		return
	}

	csrfToken := middleware.GetCSRFToken(r)
	adminDN := middleware.GetUserDN(r)

	// Get admin user info
	adminUser := &domain.User{DN: adminDN}

	// Get target user by DN - we need to search for them
	// Extract UID from DN (e.g., uid=pascal,ou=users,dc=heinrich,dc=blue -> pascal)
	var targetUser *domain.User

	// Try to extract uid from DN
	if strings.Contains(decodedDN, "uid=") {
		parts := strings.Split(decodedDN, ",")
		if len(parts) > 0 {
			uidPart := parts[0]
			uid := strings.TrimPrefix(uidPart, "uid=")
			targetUser, err = h.ldapRepo.SearchUserByUID(uid)
			if err != nil {
				slog.Error("failed to find target user", "error", err, "dn", decodedDN)
				data := TemplateData{
					CSRFToken: csrfToken,
					Error:     "Failed to find user: " + decodedDN,
				}
				if err := h.templates.ExecuteTemplate(w, "admin_dashboard.html", data); err != nil {
					slog.Error("failed to render dashboard template", "error", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
				return
			}
		}
	}

	if targetUser == nil {
		http.Error(w, "Bad Request: could not parse user DN", http.StatusBadRequest)
		return
	}

	// Check if email should be sent
	sendEmail := r.FormValue("send_email") == "true"

	// Determine scheme and host for reset link
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	isHTTPS := scheme == "https"

	// Generate reset token with correct host
	resetLink, err := h.resetService.InitiateReset(adminUser, targetUser, sendEmail, r.Host, isHTTPS)
	if err != nil {
		slog.Error("failed to generate reset token", "error", err, "target_user", targetUser.UID)
		data := TemplateData{
			CSRFToken: csrfToken,
			Error:     "Failed to generate reset token: " + err.Error(),
		}
		if err := h.templates.ExecuteTemplate(w, "admin_dashboard.html", data); err != nil {
			slog.Error("failed to render dashboard template", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	slog.Info("reset token generated", "admin", adminDN, "target_user", targetUser.UID, "reset_link", resetLink)

	// Show success with reset link
	successMsg := "Reset token generated successfully for " + targetUser.CN
	if sendEmail && targetUser.Email != "" {
		successMsg += " (email sent to " + targetUser.Email + ")"
	}

	data := TemplateData{
		CSRFToken: csrfToken,
		Success:   successMsg,
		ResetLink: resetLink,
	}

	if err := h.templates.ExecuteTemplate(w, "admin_dashboard.html", data); err != nil {
		slog.Error("failed to render dashboard template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Logout handles GET /admin/logout
func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	// Redirect based on auth mode
	logoutURL := "/admin/login"
	if h.config.Server.Auth.Mode == "proxy" && h.config.Server.Auth.LogoutURL != "" {
		// In proxy mode, redirect to SSO logout with return URL
		// Build return URL from current request
		scheme := "https"
		if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
			scheme = "http"
		}
		returnURL := scheme + "://" + r.Host + "/admin/dashboard"
		logoutURL = h.config.Server.Auth.LogoutURL + "?rd=" + url.QueryEscape(returnURL)
	}

	http.Redirect(w, r, logoutURL, http.StatusSeeOther)
}
