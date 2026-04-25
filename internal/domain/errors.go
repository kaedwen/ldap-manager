package domain

import "fmt"

// Error types for the application
var (
	ErrUserNotFound        = fmt.Errorf("user not found")
	ErrInvalidCredentials  = fmt.Errorf("invalid credentials")
	ErrNotAdmin            = fmt.Errorf("user is not an administrator")
	ErrTokenNotFound       = fmt.Errorf("reset token not found")
	ErrTokenExpired        = fmt.Errorf("reset token has expired")
	ErrTokenInvalid        = fmt.Errorf("reset token is invalid")
	ErrPasswordTooWeak     = fmt.Errorf("password does not meet strength requirements")
	ErrLDAPConnection      = fmt.Errorf("LDAP connection error")
	ErrLDAPOperation       = fmt.Errorf("LDAP operation error")
	ErrSessionInvalid      = fmt.Errorf("session is invalid or expired")
)
