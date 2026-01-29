package mail

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPSender sends emails via SMTP.
type SMTPSender struct {
	host        string
	port        int
	username    string
	password    string
	fromAddress string
	fromName    string
}

// NewSMTPSender creates a new SMTP email sender.
func NewSMTPSender(host string, port int, username, password, fromAddress, fromName string) *SMTPSender {
	return &SMTPSender{
		host:        host,
		port:        port,
		username:    username,
		password:    password,
		fromAddress: fromAddress,
		fromName:    fromName,
	}
}

// Send sends an email via SMTP.
func (s *SMTPSender) Send(_ context.Context, to, subject, body string, isHTML bool) error {
	from := s.fromAddress

	var msg strings.Builder
	if s.fromName != "" {
		msg.WriteString(fmt.Sprintf("From: %s <%s>\r\n", s.fromName, from))
	} else {
		msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	}
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	if isHTML {
		msg.WriteString("MIME-Version: 1.0\r\n")
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	} else {
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg.String())); err != nil {
		return fmt.Errorf("SMTP send failed: %w", err)
	}

	return nil
}
