package domain

import "time"

// User represents an LDAP user
type User struct {
	DN          string    // Distinguished Name
	UID         string    // Username
	Email       string    // Email address
	CN          string    // Common Name
	DisplayName string    // Display name
	Attributes  map[string][]string // Additional LDAP attributes
}

// ResetToken represents a password reset token
type ResetToken struct {
	User       *User
	TokenHash  string    // SHA-256 hash of the token
	Expiry     time.Time // When the token expires
	CreatedAt  time.Time // When the token was created
	CreatedBy  string    // DN of admin who created the token
}

// IsExpired checks if the token has expired
func (t *ResetToken) IsExpired() bool {
	return time.Now().After(t.Expiry)
}
