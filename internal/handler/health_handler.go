package handler

import (
	"net/http"

	"github.com/kaedwen/ldap-manager/internal/repository"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	ldapRepo repository.LDAPRepository
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(ldapRepo repository.LDAPRepository) *HealthHandler {
	return &HealthHandler{
		ldapRepo: ldapRepo,
	}
}

// Check handles GET /health
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check LDAP connection
	if err := h.ldapRepo.HealthCheck(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unhealthy","error":"LDAP connection failed"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
