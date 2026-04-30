package validator

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// PasswordRequirements defines password strength requirements
type PasswordRequirements struct {
	MinLength      int
	RequireUpper   bool
	RequireLower   bool
	RequireDigit   bool
	RequireSpecial bool
}

// DefaultRequirements returns the default password requirements
func DefaultRequirements() PasswordRequirements {
	return PasswordRequirements{
		MinLength:      12,
		RequireUpper:   true,
		RequireLower:   true,
		RequireDigit:   true,
		RequireSpecial: true,
	}
}

// ValidatePassword checks if a password meets the strength requirements
func ValidatePassword(password string, requirements PasswordRequirements) error {
	if len(password) < requirements.MinLength {
		return fmt.Errorf("password must be at least %d characters long", requirements.MinLength)
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasDigit   bool
		hasSpecial bool
	)

	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}

	if requirements.RequireUpper && !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}

	if requirements.RequireLower && !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}

	if requirements.RequireDigit && !hasDigit {
		return fmt.Errorf("password must contain at least one digit")
	}

	if requirements.RequireSpecial && !hasSpecial {
		return fmt.Errorf("password must contain at least one special character")
	}

	// Check against common weak passwords
	if isCommonPassword(password) {
		return fmt.Errorf("password is too common")
	}

	return nil
}

// isCommonPassword checks if a password is in the list of common passwords
func isCommonPassword(password string) bool {
	// List of common passwords (partial list for demonstration)
	commonPasswords := []string{
		"password", "password123", "123456", "12345678", "qwerty",
		"abc123", "monkey", "1234567", "letmein", "trustno1",
		"dragon", "baseball", "iloveyou", "master", "sunshine",
		"ashley", "bailey", "passw0rd", "shadow", "123123",
		"654321", "superman", "qazwsx", "michael", "football",
	}

	lowerPassword := strings.ToLower(password)
	return slices.Contains(commonPasswords, lowerPassword)
}

// ValidatePasswordWithUsername checks if password contains username
func ValidatePasswordWithUsername(password, username string) error {
	if username == "" {
		return nil
	}

	lowerPassword := strings.ToLower(password)
	lowerUsername := strings.ToLower(username)

	if strings.Contains(lowerPassword, lowerUsername) {
		return fmt.Errorf("password must not contain your username")
	}

	return nil
}

// GetRequirementsDescription returns a human-readable description of the password requirements
func (r PasswordRequirements) GetRequirementsDescription() string {
	requirements := []string{}

	// Minimum length
	requirements = append(requirements, fmt.Sprintf("at least %d characters", r.MinLength))

	// Character requirements
	charReqs := []string{}
	if r.RequireUpper {
		charReqs = append(charReqs, "uppercase")
	}
	if r.RequireLower {
		charReqs = append(charReqs, "lowercase")
	}
	if r.RequireDigit {
		charReqs = append(charReqs, "digits")
	}
	if r.RequireSpecial {
		charReqs = append(charReqs, "special characters")
	}

	if len(charReqs) > 0 {
		requirements = append(requirements, strings.Join(charReqs, ", "))
	}

	return "Must be " + strings.Join(requirements, " with ")
}
