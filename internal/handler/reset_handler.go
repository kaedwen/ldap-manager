package handler

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/kaedwen/ldap-manager/internal/domain"
	"github.com/kaedwen/ldap-manager/internal/service"
)

// ResetHandler handles password reset requests
type ResetHandler struct {
	resetService *service.ResetService
	templates    *template.Template
}

// NewResetHandler creates a new reset handler
func NewResetHandler(resetService *service.ResetService, templates *template.Template) *ResetHandler {
	return &ResetHandler{
		resetService: resetService,
		templates:    templates,
	}
}

// ShowForm handles GET /reset
func (h *ResetHandler) ShowForm(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Bad Request: missing token", http.StatusBadRequest)
		return
	}

	// Validate token
	user, err := h.resetService.ValidateToken(token)
	if err != nil {
		var errorMsg string
		switch err {
		case domain.ErrTokenNotFound:
			errorMsg = "Invalid or expired reset token"
		case domain.ErrTokenExpired:
			errorMsg = "Reset token has expired"
		default:
			errorMsg = "Invalid reset token"
		}

		data := TemplateData{
			Error: errorMsg,
		}
		w.WriteHeader(http.StatusBadRequest)
		if err := h.templates.ExecuteTemplate(w, "reset_error.html", data); err != nil {
			slog.Error("failed to render error template", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Show password reset form
	data := TemplateData{
		User: user,
		Data: map[string]string{"token": token},
	}

	if err := h.templates.ExecuteTemplate(w, "reset_form.html", data); err != nil {
		slog.Error("failed to render reset form template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Submit handles POST /reset
func (h *ResetHandler) Submit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	// Validate passwords match
	if newPassword != confirmPassword {
		data := TemplateData{
			Error: "Passwords do not match",
			Data:  map[string]string{"token": token},
		}
		w.WriteHeader(http.StatusBadRequest)
		if err := h.templates.ExecuteTemplate(w, "reset_form.html", data); err != nil {
			slog.Error("failed to render reset form template", "error", err)
		}
		return
	}

	// Reset password
	if err := h.resetService.ResetPassword(token, newPassword); err != nil {
		var errorMsg string
		switch err {
		case domain.ErrPasswordTooWeak:
			errorMsg = "Password does not meet strength requirements. Must be at least 12 characters with uppercase, lowercase, digits, and special characters."
		case domain.ErrTokenNotFound, domain.ErrTokenExpired:
			errorMsg = "Invalid or expired reset token"
		default:
			errorMsg = "Failed to reset password: " + err.Error()
		}

		data := TemplateData{
			Error: errorMsg,
			Data:  map[string]string{"token": token},
		}
		w.WriteHeader(http.StatusBadRequest)
		if err := h.templates.ExecuteTemplate(w, "reset_form.html", data); err != nil {
			slog.Error("failed to render reset form template", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Show success page
	data := TemplateData{
		Success: "Password has been reset successfully",
	}

	if err := h.templates.ExecuteTemplate(w, "reset_success.html", data); err != nil {
		slog.Error("failed to render success template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
