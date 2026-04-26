package service

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"

	"github.com/kaedwen/ldap-manager/internal/config"
	"github.com/kaedwen/ldap-manager/internal/domain"
)

// NotificationService handles email notifications
type NotificationService struct {
	config *config.EmailConfig
}

// NewNotificationService creates a new notification service
func NewNotificationService(emailConfig *config.EmailConfig) *NotificationService {
	return &NotificationService{
		config: emailConfig,
	}
}

// SendResetEmail sends a password reset email to a user
func (s *NotificationService) SendResetEmail(user *domain.User, resetLink string) error {
	if !s.config.Enabled {
		slog.Debug("email notifications disabled, skipping")
		return nil
	}

	if user.Email == "" {
		return fmt.Errorf("user has no email address")
	}

	// Build email content
	subject := "Password Reset Request"
	body := s.buildResetEmailBody(user, resetLink)

	// Send email
	if err := s.sendEmail(user.Email, subject, body); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	slog.Info("reset email sent", "recipient", user.Email, "user", user.UID)
	return nil
}

// buildResetEmailBody creates the email body for password reset
func (s *NotificationService) buildResetEmailBody(user *domain.User, resetLink string) string {
	displayName := user.DisplayName
	if displayName == "" {
		displayName = user.CN
	}
	if displayName == "" {
		displayName = user.UID
	}

	return fmt.Sprintf(`Hello %s,

You have requested to reset your password.

Please click the link below to set a new password:

%s

This link will expire in a few days. If you did not request this reset, please ignore this email.

Best regards,
%s`, displayName, resetLink, s.config.FromName)
}

// sendEmail sends an email using SMTP
func (s *NotificationService) sendEmail(to, subject, body string) error {
	// Build MIME message
	message := s.buildMIMEMessage(to, subject, body)

	// Server address
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)

	// If InsecureSkipVerify is enabled, use custom dialer with TLS config
	if s.config.InsecureSkipVerify {
		return s.sendEmailInsecure(addr, message, to)
	}

	// Default: use standard smtp.SendMail with proper TLS verification
	var auth smtp.Auth
	if s.config.SMTPUsername != "" && s.config.SMTPPassword != "" {
		auth = smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)
	}

	if err := smtp.SendMail(addr, auth, s.config.FromAddress, []string{to}, []byte(message)); err != nil {
		return err
	}

	return nil
}

// sendEmailInsecure sends email with TLS certificate verification disabled
// This is useful for internal mail relays with self-signed certificates
func (s *NotificationService) sendEmailInsecure(addr, message, to string) error {
	// Connect to SMTP server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, s.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// Try STARTTLS with insecure config
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         s.config.SMTPHost,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			// If STARTTLS fails, log but continue without TLS
			slog.Debug("STARTTLS failed, continuing without TLS", "error", err)
		}
	}

	// Authenticate if credentials provided
	if s.config.SMTPUsername != "" && s.config.SMTPPassword != "" {
		auth := smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(s.config.FromAddress); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipient
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	// Send message body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start data: %w", err)
	}

	if _, err := w.Write([]byte(message)); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data: %w", err)
	}

	// Send QUIT
	if err := client.Quit(); err != nil {
		return fmt.Errorf("failed to quit: %w", err)
	}

	return nil
}

// buildMIMEMessage builds a MIME-formatted email message
func (s *NotificationService) buildMIMEMessage(to, subject, body string) string {
	var sb strings.Builder

	sb.WriteString("From: ")
	if s.config.FromName != "" {
		sb.WriteString(fmt.Sprintf("%s <%s>", s.config.FromName, s.config.FromAddress))
	} else {
		sb.WriteString(s.config.FromAddress)
	}
	sb.WriteString("\r\n")

	sb.WriteString("To: ")
	sb.WriteString(to)
	sb.WriteString("\r\n")

	sb.WriteString("Subject: ")
	sb.WriteString(subject)
	sb.WriteString("\r\n")

	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	sb.WriteString("\r\n")

	sb.WriteString(body)

	return sb.String()
}
