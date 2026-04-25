package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kaedwen/ldap-manager/internal/domain"
)

// SessionData represents the data stored in a session
type SessionData struct {
	UserDN    string `json:"user_dn"`
	CSRFToken string `json:"csrf_token"`
	ExpiresAt int64  `json:"expires_at"`
}

// SessionService handles session management with HMAC-signed cookies
type SessionService struct {
	secret []byte
	maxAge int
}

// NewSessionService creates a new session service
func NewSessionService(secret string, maxAge int) *SessionService {
	return &SessionService{
		secret: []byte(secret),
		maxAge: maxAge,
	}
}

// CreateSession creates a new session with CSRF token
func (s *SessionService) CreateSession(userDN string) (string, string, error) {
	// Generate CSRF token
	csrfToken, err := s.generateCSRFToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}

	// Create session data
	data := SessionData{
		UserDN:    userDN,
		CSRFToken: csrfToken,
		ExpiresAt: time.Now().Add(time.Duration(s.maxAge) * time.Second).Unix(),
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal session data: %w", err)
	}

	// Encode data
	encodedData := base64.RawURLEncoding.EncodeToString(jsonData)

	// Create HMAC signature
	signature := s.sign(encodedData)
	encodedSignature := base64.RawURLEncoding.EncodeToString(signature)

	// Combine: data|signature
	signedCookie := encodedData + "|" + encodedSignature

	return signedCookie, csrfToken, nil
}

// ValidateSession validates a session cookie and returns the user DN and CSRF token
func (s *SessionService) ValidateSession(signedCookie string) (string, string, error) {
	if signedCookie == "" {
		return "", "", domain.ErrSessionInvalid
	}

	// Split cookie into data and signature
	parts := strings.Split(signedCookie, "|")
	if len(parts) != 2 {
		return "", "", domain.ErrSessionInvalid
	}

	encodedData := parts[0]
	encodedSignature := parts[1]

	// Decode signature
	providedSignature, err := base64.RawURLEncoding.DecodeString(encodedSignature)
	if err != nil {
		return "", "", domain.ErrSessionInvalid
	}

	// Verify signature using constant-time comparison
	expectedSignature := s.sign(encodedData)
	if !hmac.Equal(providedSignature, expectedSignature) {
		return "", "", domain.ErrSessionInvalid
	}

	// Decode data
	jsonData, err := base64.RawURLEncoding.DecodeString(encodedData)
	if err != nil {
		return "", "", domain.ErrSessionInvalid
	}

	// Unmarshal session data
	var data SessionData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return "", "", domain.ErrSessionInvalid
	}

	// Check if session has expired
	if time.Now().Unix() > data.ExpiresAt {
		return "", "", domain.ErrSessionInvalid
	}

	return data.UserDN, data.CSRFToken, nil
}

// ValidateCSRFToken validates a CSRF token using constant-time comparison
func (s *SessionService) ValidateCSRFToken(sessionToken, providedToken string) bool {
	if sessionToken == "" || providedToken == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(sessionToken), []byte(providedToken)) == 1
}

// sign creates an HMAC-SHA256 signature
func (s *SessionService) sign(data string) []byte {
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// generateCSRFToken generates a cryptographically secure CSRF token
func (s *SessionService) generateCSRFToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(tokenBytes), nil
}
