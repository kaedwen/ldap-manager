package repository

import (
	"time"

	"github.com/kaedwen/ldap-manager/internal/domain"
)

// LDAPRepository defines the interface for LDAP operations
type LDAPRepository interface {
	// Authentication
	Bind(dn, password string) error
	CheckGroupMembership(userDN, groupDN string) (bool, error)

	// User operations
	SearchUser(filter string) (*domain.User, error)
	SearchUsers(filter string) ([]*domain.User, error)
	SearchUserByUID(uid string) (*domain.User, error)
	SearchUserByEmail(email string) (*domain.User, error)

	// Token operations
	StoreResetToken(userDN, tokenHash string, expiry time.Time) error
	FindUserByTokenHash(tokenHash string) (*domain.User, error)
	ClearResetToken(userDN string) error

	// Password operations
	SetPassword(userDN, newPassword string) error

	// Lifecycle
	Close() error
}
