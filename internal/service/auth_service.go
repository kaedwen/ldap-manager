package service

import (
	"fmt"
	"log/slog"

	"github.com/kaedwen/ldap-manager/internal/domain"
	"github.com/kaedwen/ldap-manager/internal/repository"
)

// AuthService handles authentication and authorization
type AuthService struct {
	ldapRepo     repository.LDAPRepository
	adminGroupDN string
}

// NewAuthService creates a new authentication service
func NewAuthService(ldapRepo repository.LDAPRepository, adminGroupDN string) *AuthService {
	return &AuthService{
		ldapRepo:     ldapRepo,
		adminGroupDN: adminGroupDN,
	}
}

// AuthenticateAdmin authenticates an admin user and verifies group membership
func (s *AuthService) AuthenticateAdmin(username, password string) (*domain.User, error) {
	// Search for user by username
	user, err := s.ldapRepo.SearchUserByUID(username)
	if err != nil {
		slog.Warn("admin login failed: user not found", "username", username)
		return nil, domain.ErrInvalidCredentials
	}

	// Try to bind with user credentials
	if err := s.ldapRepo.Bind(user.DN, password); err != nil {
		slog.Warn("admin login failed: invalid password", "username", username, "dn", user.DN)
		return nil, err
	}

	// Check if user is member of admin group
	isMember, err := s.ldapRepo.CheckGroupMembership(user.DN, s.adminGroupDN)
	if err != nil {
		slog.Error("failed to check admin group membership", "error", err, "user_dn", user.DN)
		return nil, fmt.Errorf("failed to verify admin privileges: %w", err)
	}

	if !isMember {
		slog.Warn("admin login failed: not a member of admin group", "username", username, "dn", user.DN)
		return nil, domain.ErrNotAdmin
	}

	slog.Info("admin authenticated successfully", "username", username, "dn", user.DN)
	return user, nil
}

// AuthenticateUser authenticates a regular user (for future use)
func (s *AuthService) AuthenticateUser(username, password string) (*domain.User, error) {
	// Search for user by username
	user, err := s.ldapRepo.SearchUserByUID(username)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	// Try to bind with user credentials
	if err := s.ldapRepo.Bind(user.DN, password); err != nil {
		return nil, err
	}

	slog.Info("user authenticated successfully", "username", username, "dn", user.DN)
	return user, nil
}
