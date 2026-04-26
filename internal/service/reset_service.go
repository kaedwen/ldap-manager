package service

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/kaedwen/ldap-manager/internal/domain"
	"github.com/kaedwen/ldap-manager/internal/repository"
	"github.com/kaedwen/ldap-manager/pkg/crypto"
	"github.com/kaedwen/ldap-manager/pkg/validator"
)

// ResetService handles password reset operations
type ResetService struct {
	ldapRepo      repository.LDAPRepository
	tokenLength   int
	validityDays  int
	baseURL       string
	notifService  *NotificationService
	passwordReqs  validator.PasswordRequirements
}

// NewResetService creates a new reset service
func NewResetService(
	ldapRepo repository.LDAPRepository,
	tokenLength int,
	validityDays int,
	baseURL string,
	notifService *NotificationService,
	passwordReqs validator.PasswordRequirements,
) *ResetService {
	return &ResetService{
		ldapRepo:     ldapRepo,
		tokenLength:  tokenLength,
		validityDays: validityDays,
		baseURL:      baseURL,
		notifService: notifService,
		passwordReqs: passwordReqs,
	}
}

// InitiateReset generates a reset token for a user
func (s *ResetService) InitiateReset(adminUser *domain.User, targetUser *domain.User, sendEmail bool) (string, error) {
	// Generate token
	token, err := crypto.GenerateToken(s.tokenLength)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Hash token for storage
	tokenHash := crypto.HashToken(token)

	// Calculate expiry
	expiry := time.Now().Add(time.Duration(s.validityDays) * 24 * time.Hour)

	// Store token in LDAP
	if err := s.ldapRepo.StoreResetToken(targetUser.DN, tokenHash, expiry); err != nil {
		return "", fmt.Errorf("failed to store reset token: %w", err)
	}

	// Build reset link
	resetLink := fmt.Sprintf("%s/reset?token=%s", s.baseURL, token)

	slog.Info("password reset initiated",
		"admin", adminUser.UID,
		"target_user", targetUser.UID,
		"expiry", expiry,
	)

	// Send email if requested
	if sendEmail && s.notifService != nil {
		if err := s.notifService.SendResetEmail(targetUser, resetLink); err != nil {
			slog.Error("failed to send reset email", "error", err, "user", targetUser.UID)
			// Don't fail the whole operation if email fails
		}
	}

	return resetLink, nil
}

// ValidateToken validates a reset token and returns the associated user
func (s *ResetService) ValidateToken(token string) (*domain.User, error) {
	if token == "" {
		return nil, domain.ErrTokenInvalid
	}

	// Hash the provided token
	tokenHash := crypto.HashToken(token)

	// Find user by token hash
	user, err := s.ldapRepo.FindUserByTokenHash(tokenHash)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// ResetPassword resets a user's password using a valid token
func (s *ResetService) ResetPassword(token, newPassword string) error {
	// Validate token and get user
	user, err := s.ValidateToken(token)
	if err != nil {
		return err
	}

	// Validate password strength
	if err := validator.ValidatePassword(newPassword, s.passwordReqs); err != nil {
		return domain.ErrPasswordTooWeak
	}

	// Check password doesn't contain username
	if err := validator.ValidatePasswordWithUsername(newPassword, user.UID); err != nil {
		return domain.ErrPasswordTooWeak
	}

	// Set new password in LDAP
	if err := s.ldapRepo.SetPassword(user.DN, newPassword); err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}

	// Clear the reset token (single-use enforcement)
	if err := s.ldapRepo.ClearResetToken(user.DN); err != nil {
		slog.Error("failed to clear reset token", "error", err, "user_dn", user.DN)
		// Don't fail the operation if clearing fails
	}

	slog.Info("password reset completed", "user", user.UID, "dn", user.DN)
	return nil
}
