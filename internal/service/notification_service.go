package service

import (
	"fmt"
	"log/slog"
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

	// SMTP authentication
	var auth smtp.Auth
	if s.config.SMTPUsername != "" && s.config.SMTPPassword != "" {
		auth = smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)
	}

	// Server address
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)

	// Send email
	if err := smtp.SendMail(addr, auth, s.config.FromAddress, []string{to}, []byte(message)); err != nil {
		return err
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
