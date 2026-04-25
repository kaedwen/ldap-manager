package repository

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/kaedwen/ldap-manager/internal/config"
	"github.com/kaedwen/ldap-manager/internal/domain"
)

// LDAPRepositoryImpl implements the LDAPRepository interface
type LDAPRepositoryImpl struct {
	config *config.Config
	mu     sync.RWMutex
	conn   *ldap.Conn
}

// NewLDAPRepository creates a new LDAP repository
func NewLDAPRepository(cfg *config.Config) (LDAPRepository, error) {
	repo := &LDAPRepositoryImpl{
		config: cfg,
	}

	// Establish initial connection
	if err := repo.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to LDAP: %w", err)
	}

	slog.Info("LDAP repository initialized", "url", cfg.LDAP.URL)
	return repo, nil
}

// connect establishes a connection to the LDAP server
func (r *LDAPRepositoryImpl) connect() error {
	// Use DialURL for modern LDAP client
	conn, err := ldap.DialURL(r.config.LDAP.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to LDAP: %w", err)
	}

	// Bind with service account
	password := r.config.GetLDAPPassword()
	if err := conn.Bind(r.config.LDAP.BindDN, password); err != nil {
		conn.Close()
		return fmt.Errorf("failed to bind with service account: %w", err)
	}

	r.mu.Lock()
	if r.conn != nil {
		r.conn.Close()
	}
	r.conn = conn
	r.mu.Unlock()

	return nil
}

// getConn returns the current connection, reconnecting if necessary
func (r *LDAPRepositoryImpl) getConn() (*ldap.Conn, error) {
	r.mu.RLock()
	conn := r.conn
	r.mu.RUnlock()

	// Test connection
	if conn != nil {
		if !conn.IsClosing() {
			return conn, nil
		}
	}

	// Reconnect
	if err := r.connect(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	conn = r.conn
	r.mu.RUnlock()

	return conn, nil
}

// Bind authenticates a user with their DN and password
func (r *LDAPRepositoryImpl) Bind(dn, password string) error {
	// Create a separate connection for user bind
	conn, err := ldap.DialURL(r.config.LDAP.URL)
	if err != nil {
		return domain.ErrLDAPConnection
	}
	defer conn.Close()

	// Try to bind with user credentials
	if err := conn.Bind(dn, password); err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
			return domain.ErrInvalidCredentials
		}
		return fmt.Errorf("bind failed: %w", err)
	}

	return nil
}

// CheckGroupMembership checks if a user is a member of a group
func (r *LDAPRepositoryImpl) CheckGroupMembership(userDN, groupDN string) (bool, error) {
	conn, err := r.getConn()
	if err != nil {
		return false, err
	}

	// Search for the group and check if the user is a member
	searchRequest := ldap.NewSearchRequest(
		groupDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0, 0, false,
		fmt.Sprintf("(member=%s)", ldap.EscapeFilter(userDN)),
		[]string{"dn"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultNoSuchObject) {
			return false, nil
		}
		return false, fmt.Errorf("group search failed: %w", err)
	}

	return len(result.Entries) > 0, nil
}

// SearchUser searches for a user by a custom filter
func (r *LDAPRepositoryImpl) SearchUser(filter string) (*domain.User, error) {
	conn, err := r.getConn()
	if err != nil {
		return nil, err
	}

	searchRequest := ldap.NewSearchRequest(
		r.config.LDAP.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1,     // SizeLimit: return max 1 entry
		30,    // TimeLimit: 30 seconds timeout
		false, // TypesOnly
		filter,
		[]string{"dn", "uid", "mail", "cn", "displayName"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		// Check if we got "Size Limit Exceeded" but still have results
		if ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded) && result != nil && len(result.Entries) > 0 {
			// Continue with the partial results
			slog.Debug("size limit exceeded but got results", "entries", len(result.Entries))
		} else {
			return nil, fmt.Errorf("user search failed: %w", err)
		}
	}

	if len(result.Entries) == 0 {
		return nil, domain.ErrUserNotFound
	}

	return r.entryToUser(result.Entries[0]), nil
}

// SearchUsers searches for multiple users by a custom filter
func (r *LDAPRepositoryImpl) SearchUsers(filter string) ([]*domain.User, error) {
	conn, err := r.getConn()
	if err != nil {
		return nil, err
	}

	searchRequest := ldap.NewSearchRequest(
		r.config.LDAP.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		100,   // SizeLimit: return max 100 entries
		30,    // TimeLimit: 30 seconds timeout
		false, // TypesOnly
		filter,
		[]string{"dn", "uid", "mail", "cn", "displayName"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		// Check if we got "Size Limit Exceeded" but still have results
		if ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded) && result != nil && len(result.Entries) > 0 {
			slog.Debug("size limit exceeded but got results", "entries", len(result.Entries))
		} else {
			return nil, fmt.Errorf("users search failed: %w", err)
		}
	}

	if len(result.Entries) == 0 {
		return nil, domain.ErrUserNotFound
	}

	users := make([]*domain.User, 0, len(result.Entries))
	for _, entry := range result.Entries {
		users = append(users, r.entryToUser(entry))
	}

	return users, nil
}

// SearchUserByUID searches for a user by their UID
func (r *LDAPRepositoryImpl) SearchUserByUID(uid string) (*domain.User, error) {
	filter := fmt.Sprintf(r.config.LDAP.UserFilter, ldap.EscapeFilter(uid))
	return r.SearchUser(filter)
}

// SearchUserByEmail searches for a user by their email address
func (r *LDAPRepositoryImpl) SearchUserByEmail(email string) (*domain.User, error) {
	filter := fmt.Sprintf("(mail=%s)", ldap.EscapeFilter(email))
	return r.SearchUser(filter)
}

// StoreResetToken stores a password reset token hash and expiry in LDAP
func (r *LDAPRepositoryImpl) StoreResetToken(userDN, tokenHash string, expiry time.Time) error {
	conn, err := r.getConn()
	if err != nil {
		return err
	}

	expiryUnix := strconv.FormatInt(expiry.Unix(), 10)

	modifyRequest := ldap.NewModifyRequest(userDN, nil)
	modifyRequest.Replace("pwdResetToken", []string{tokenHash})
	modifyRequest.Replace("pwdResetExpiry", []string{expiryUnix})

	// Add the pwdResetAccount object class if not already present
	modifyRequest.Add("objectClass", []string{"pwdResetAccount"})

	if err := conn.Modify(modifyRequest); err != nil {
		// Ignore error if objectClass already exists
		if !ldap.IsErrorWithCode(err, ldap.LDAPResultAttributeOrValueExists) {
			return fmt.Errorf("failed to store reset token: %w", err)
		}

		// Retry without adding objectClass
		modifyRequest = ldap.NewModifyRequest(userDN, nil)
		modifyRequest.Replace("pwdResetToken", []string{tokenHash})
		modifyRequest.Replace("pwdResetExpiry", []string{expiryUnix})

		if err := conn.Modify(modifyRequest); err != nil {
			return fmt.Errorf("failed to store reset token: %w", err)
		}
	}

	slog.Info("stored reset token", "user_dn", userDN, "expiry", expiry)
	return nil
}

// FindUserByTokenHash finds a user by their reset token hash
func (r *LDAPRepositoryImpl) FindUserByTokenHash(tokenHash string) (*domain.User, error) {
	conn, err := r.getConn()
	if err != nil {
		return nil, err
	}

	filter := fmt.Sprintf("(&(pwdResetToken=%s)(objectClass=pwdResetAccount))", ldap.EscapeFilter(tokenHash))

	searchRequest := ldap.NewSearchRequest(
		r.config.LDAP.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, 0, false,
		filter,
		[]string{"dn", "uid", "mail", "cn", "displayName", "pwdResetExpiry"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("token search failed: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, domain.ErrTokenNotFound
	}

	entry := result.Entries[0]

	// Check if token has expired
	expiryStr := entry.GetAttributeValue("pwdResetExpiry")
	if expiryStr != "" {
		expiryUnix, err := strconv.ParseInt(expiryStr, 10, 64)
		if err == nil {
			expiry := time.Unix(expiryUnix, 0)
			if time.Now().After(expiry) {
				return nil, domain.ErrTokenExpired
			}
		}
	}

	return r.entryToUser(entry), nil
}

// ClearResetToken removes the reset token from a user's LDAP entry
func (r *LDAPRepositoryImpl) ClearResetToken(userDN string) error {
	conn, err := r.getConn()
	if err != nil {
		return err
	}

	modifyRequest := ldap.NewModifyRequest(userDN, nil)
	modifyRequest.Delete("pwdResetToken", []string{})
	modifyRequest.Delete("pwdResetExpiry", []string{})

	if err := conn.Modify(modifyRequest); err != nil {
		// Ignore error if attributes don't exist
		if !ldap.IsErrorWithCode(err, ldap.LDAPResultNoSuchAttribute) {
			return fmt.Errorf("failed to clear reset token: %w", err)
		}
	}

	slog.Info("cleared reset token", "user_dn", userDN)
	return nil
}

// SetPassword sets a new password for a user
func (r *LDAPRepositoryImpl) SetPassword(userDN, newPassword string) error {
	conn, err := r.getConn()
	if err != nil {
		return err
	}

	modifyRequest := ldap.NewModifyRequest(userDN, nil)
	modifyRequest.Replace("userPassword", []string{newPassword})

	if err := conn.Modify(modifyRequest); err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}

	slog.Info("password changed successfully", "user_dn", userDN)
	return nil
}

// entryToUser converts an LDAP entry to a domain.User
func (r *LDAPRepositoryImpl) entryToUser(entry *ldap.Entry) *domain.User {
	user := &domain.User{
		DN:          entry.DN,
		UID:         entry.GetAttributeValue("uid"),
		Email:       entry.GetAttributeValue("mail"),
		CN:          entry.GetAttributeValue("cn"),
		DisplayName: entry.GetAttributeValue("displayName"),
		Attributes:  make(map[string][]string),
	}

	// Store all attributes
	for _, attr := range entry.Attributes {
		user.Attributes[attr.Name] = attr.Values
	}

	return user
}

// Close closes the LDAP connection
func (r *LDAPRepositoryImpl) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conn != nil {
		r.conn.Close()
		r.conn = nil
	}

	return nil
}
