package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	LDAP      LDAPConfig      `yaml:"ldap"`
	Token     TokenConfig     `yaml:"token"`
	Email     EmailConfig     `yaml:"email"`
	RateLimit RateLimitConfig `yaml:"ratelimit"`
	Logging   LoggingConfig   `yaml:"logging"`

	mu              sync.RWMutex
	passwordWatcher *fsnotify.Watcher
}

type ServerConfig struct {
	Port    int           `yaml:"port"`
	Host    string        `yaml:"host"`
	TLS     TLSConfig     `yaml:"tls"`
	Session SessionConfig `yaml:"session"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type SessionConfig struct {
	Secret string `yaml:"secret"`
	MaxAge int    `yaml:"max_age"`
}

type LDAPConfig struct {
	URL              string `yaml:"url"`
	BindDN           string `yaml:"bind_dn"`
	BindPassword     string `yaml:"bind_password"`
	BindPasswordFile string `yaml:"bind_password_file"`
	BaseDN           string `yaml:"base_dn"`
	AdminGroupDN     string `yaml:"admin_group_dn"`
	UserFilter       string `yaml:"user_filter"`
}

type TokenConfig struct {
	ValidityDays int `yaml:"validity_days"`
	LengthBytes  int `yaml:"length_bytes"`
}

type EmailConfig struct {
	Enabled      bool   `yaml:"enabled"`
	SMTPHost     string `yaml:"smtp_host"`
	SMTPPort     int    `yaml:"smtp_port"`
	SMTPUsername string `yaml:"smtp_username"`
	SMTPPassword string `yaml:"smtp_password"`
	FromAddress  string `yaml:"from_address"`
	FromName     string `yaml:"from_name"`
}

type RateLimitConfig struct {
	ResetPerIPPerHour int `yaml:"reset_per_ip_per_hour"`
	LoginPerIPPerHour int `yaml:"login_per_ip_per_hour"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Load reads configuration from file and applies environment variable overrides
func Load(configPath string) (*Config, error) {
	var cfg Config

	// Set defaults
	cfg.setDefaults()

	// Read config file if it exists
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	// Read password from file if specified
	if err := cfg.loadPasswordFromFile(); err != nil {
		return nil, err
	}

	// Start watching password file if configured
	if cfg.LDAP.BindPasswordFile != "" {
		if err := cfg.startPasswordFileWatcher(); err != nil {
			return nil, fmt.Errorf("failed to start password file watcher: %w", err)
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values for optional configuration fields
func (c *Config) setDefaults() {
	c.Server.Port = 8443
	c.Server.Host = "0.0.0.0"
	c.Server.Session.MaxAge = 3600
	c.Token.ValidityDays = 3
	c.Token.LengthBytes = 32
	c.RateLimit.ResetPerIPPerHour = 5
	c.RateLimit.LoginPerIPPerHour = 10
	c.Logging.Level = "info"
	c.Logging.Format = "json"
	c.LDAP.UserFilter = "(uid=%s)"
}

// applyEnvOverrides applies environment variable overrides with LDAP_MANAGER_ prefix
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("LDAP_MANAGER_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.Port = port
		}
	}
	if v := os.Getenv("LDAP_MANAGER_SERVER_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("LDAP_MANAGER_SERVER_TLS_ENABLED"); v != "" {
		c.Server.TLS.Enabled = v == "true"
	}
	if v := os.Getenv("LDAP_MANAGER_SERVER_TLS_CERT_FILE"); v != "" {
		c.Server.TLS.CertFile = v
	}
	if v := os.Getenv("LDAP_MANAGER_SERVER_TLS_KEY_FILE"); v != "" {
		c.Server.TLS.KeyFile = v
	}
	if v := os.Getenv("LDAP_MANAGER_SERVER_SESSION_SECRET"); v != "" {
		c.Server.Session.Secret = v
	}
	if v := os.Getenv("LDAP_MANAGER_SERVER_SESSION_MAX_AGE"); v != "" {
		if age, err := strconv.Atoi(v); err == nil {
			c.Server.Session.MaxAge = age
		}
	}

	if v := os.Getenv("LDAP_MANAGER_LDAP_URL"); v != "" {
		c.LDAP.URL = v
	}
	if v := os.Getenv("LDAP_MANAGER_LDAP_BIND_DN"); v != "" {
		c.LDAP.BindDN = v
	}
	if v := os.Getenv("LDAP_MANAGER_LDAP_BIND_PASSWORD"); v != "" {
		c.LDAP.BindPassword = v
	}
	if v := os.Getenv("LDAP_MANAGER_LDAP_BIND_PASSWORD_FILE"); v != "" {
		c.LDAP.BindPasswordFile = v
	}
	if v := os.Getenv("LDAP_MANAGER_LDAP_BASE_DN"); v != "" {
		c.LDAP.BaseDN = v
	}
	if v := os.Getenv("LDAP_MANAGER_LDAP_ADMIN_GROUP_DN"); v != "" {
		c.LDAP.AdminGroupDN = v
	}
	if v := os.Getenv("LDAP_MANAGER_LDAP_USER_FILTER"); v != "" {
		c.LDAP.UserFilter = v
	}

	if v := os.Getenv("LDAP_MANAGER_TOKEN_VALIDITY_DAYS"); v != "" {
		if days, err := strconv.Atoi(v); err == nil {
			c.Token.ValidityDays = days
		}
	}
	if v := os.Getenv("LDAP_MANAGER_TOKEN_LENGTH_BYTES"); v != "" {
		if length, err := strconv.Atoi(v); err == nil {
			c.Token.LengthBytes = length
		}
	}

	if v := os.Getenv("LDAP_MANAGER_EMAIL_ENABLED"); v != "" {
		c.Email.Enabled = v == "true"
	}
	if v := os.Getenv("LDAP_MANAGER_EMAIL_SMTP_HOST"); v != "" {
		c.Email.SMTPHost = v
	}
	if v := os.Getenv("LDAP_MANAGER_EMAIL_SMTP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Email.SMTPPort = port
		}
	}
	if v := os.Getenv("LDAP_MANAGER_EMAIL_SMTP_USERNAME"); v != "" {
		c.Email.SMTPUsername = v
	}
	if v := os.Getenv("LDAP_MANAGER_EMAIL_SMTP_PASSWORD"); v != "" {
		c.Email.SMTPPassword = v
	}
	if v := os.Getenv("LDAP_MANAGER_EMAIL_FROM_ADDRESS"); v != "" {
		c.Email.FromAddress = v
	}
	if v := os.Getenv("LDAP_MANAGER_EMAIL_FROM_NAME"); v != "" {
		c.Email.FromName = v
	}

	if v := os.Getenv("LDAP_MANAGER_RATELIMIT_RESET_PER_IP_PER_HOUR"); v != "" {
		if limit, err := strconv.Atoi(v); err == nil {
			c.RateLimit.ResetPerIPPerHour = limit
		}
	}
	if v := os.Getenv("LDAP_MANAGER_RATELIMIT_LOGIN_PER_IP_PER_HOUR"); v != "" {
		if limit, err := strconv.Atoi(v); err == nil {
			c.RateLimit.LoginPerIPPerHour = limit
		}
	}

	if v := os.Getenv("LDAP_MANAGER_LOGGING_LEVEL"); v != "" {
		c.Logging.Level = v
	}
	if v := os.Getenv("LDAP_MANAGER_LOGGING_FORMAT"); v != "" {
		c.Logging.Format = v
	}
}

// loadPasswordFromFile reads LDAP bind password from file if configured
func (c *Config) loadPasswordFromFile() error {
	if c.LDAP.BindPasswordFile == "" {
		return nil
	}

	data, err := os.ReadFile(c.LDAP.BindPasswordFile)
	if err != nil {
		return fmt.Errorf("failed to read password file: %w", err)
	}

	// Trim whitespace and newlines
	password := strings.TrimSpace(string(data))
	if password == "" {
		return fmt.Errorf("password file is empty")
	}

	c.mu.Lock()
	c.LDAP.BindPassword = password
	c.mu.Unlock()

	slog.Info("loaded LDAP bind password from file", "file", c.LDAP.BindPasswordFile)
	return nil
}

// startPasswordFileWatcher starts watching the password file for changes
func (c *Config) startPasswordFileWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Watch the directory, not the file directly (for atomic file replacement)
	dir := filepath.Dir(c.LDAP.BindPasswordFile)
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	c.passwordWatcher = watcher

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Check if the event is for our password file
				if filepath.Clean(event.Name) == filepath.Clean(c.LDAP.BindPasswordFile) {
					if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
						if err := c.loadPasswordFromFile(); err != nil {
							slog.Error("failed to reload password from file", "error", err)
						} else {
							slog.Info("reloaded LDAP bind password from file")
						}
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error("password file watcher error", "error", err)
			}
		}
	}()

	slog.Info("started watching password file for changes", "file", c.LDAP.BindPasswordFile)
	return nil
}

// GetLDAPPassword returns the current LDAP bind password (thread-safe)
func (c *Config) GetLDAPPassword() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LDAP.BindPassword
}

// Close cleans up resources
func (c *Config) Close() error {
	if c.passwordWatcher != nil {
		return c.passwordWatcher.Close()
	}
	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Server.TLS.Enabled {
		if c.Server.TLS.CertFile == "" {
			return fmt.Errorf("TLS cert file is required when TLS is enabled")
		}
		if c.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS key file is required when TLS is enabled")
		}
	}

	if c.Server.Session.Secret == "" {
		return fmt.Errorf("session secret is required")
	}

	if len(c.Server.Session.Secret) < 32 {
		return fmt.Errorf("session secret must be at least 32 bytes")
	}

	if c.LDAP.URL == "" {
		return fmt.Errorf("LDAP URL is required")
	}

	if !strings.HasPrefix(c.LDAP.URL, "ldap://") && !strings.HasPrefix(c.LDAP.URL, "ldaps://") {
		return fmt.Errorf("LDAP URL must start with ldap:// or ldaps://")
	}

	if c.LDAP.BindDN == "" {
		return fmt.Errorf("LDAP bind DN is required")
	}

	if c.LDAP.BindPassword == "" && c.LDAP.BindPasswordFile == "" {
		return fmt.Errorf("LDAP bind password or password file is required")
	}

	if c.LDAP.BaseDN == "" {
		return fmt.Errorf("LDAP base DN is required")
	}

	if c.LDAP.AdminGroupDN == "" {
		return fmt.Errorf("LDAP admin group DN is required")
	}

	if c.Token.ValidityDays < 1 {
		return fmt.Errorf("token validity days must be at least 1")
	}

	if c.Token.LengthBytes < 16 {
		return fmt.Errorf("token length must be at least 16 bytes")
	}

	if c.Email.Enabled {
		if c.Email.SMTPHost == "" {
			return fmt.Errorf("SMTP host is required when email is enabled")
		}
		if c.Email.SMTPPort < 1 || c.Email.SMTPPort > 65535 {
			return fmt.Errorf("invalid SMTP port: %d", c.Email.SMTPPort)
		}
		if c.Email.FromAddress == "" {
			return fmt.Errorf("from address is required when email is enabled")
		}
	}

	return nil
}
