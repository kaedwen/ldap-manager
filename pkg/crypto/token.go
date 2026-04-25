package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// GenerateToken generates a cryptographically secure random token
// of the specified length (in bytes) and returns it as a base64url-encoded string
func GenerateToken(lengthBytes int) (string, error) {
	if lengthBytes < 16 {
		return "", fmt.Errorf("token length must be at least 16 bytes")
	}

	// Generate random bytes
	tokenBytes := make([]byte, lengthBytes)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	// Encode as base64url (URL-safe)
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	return token, nil
}

// HashToken creates a SHA-256 hash of the token for storage in LDAP
// This ensures that even if the LDAP directory is compromised,
// the actual reset tokens cannot be recovered
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
