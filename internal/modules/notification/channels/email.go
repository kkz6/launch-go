package channels

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
)

// EmailSender interface for sending emails
type EmailSender interface {
	Send(ctx context.Context, to, subject, body string, isHTML bool) error
}

// EmailChannel handles email notifications
type EmailChannel struct {
	channel *NotificationChannel
	sender  EmailSender
}

// NewEmailChannel creates a new email channel
func NewEmailChannel(channel *NotificationChannel, sender EmailSender) *EmailChannel {
	return &EmailChannel{
		channel: channel,
		sender:  sender,
	}
}

// Send sends a notification via email
func (e *EmailChannel) Send(ctx context.Context, notif Notification) error {
	email := e.channel.GetEmail()
	if email == "" {
		return ErrInvalidConfiguration
	}

	if e.sender == nil {
		return ErrSendFailed
	}

	msg := notif.ToEmail()
	err := e.sender.Send(ctx, email, msg.Subject, msg.Body, msg.IsHTML)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	return nil
}

// Connect tests the email connection by sending a test email
func (e *EmailChannel) Connect(ctx context.Context) error {
	email := e.channel.GetEmail()
	if email == "" {
		return ErrInvalidConfiguration
	}

	if e.sender == nil {
		return ErrConnectionFailed
	}

	html, _, err := templates.ConnectionTestEmail()
	if err != nil {
		// Fallback to plain text
		err = e.sender.Send(ctx, email, "Connected to launchctl", "This email confirms that you have connected your email to launchctl.", false)
	} else {
		err = e.sender.Send(ctx, email, "Connected to launchctl", html, true)
	}

	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	return nil
}

// GetCreateRules returns the validation rules for creating an email channel
func (e *EmailChannel) GetCreateRules() map[string]string {
	return MergeValidationRules(map[string]string{
		"email": "required,email",
	})
}

// GetData returns the email channel data
func (e *EmailChannel) GetData() ChannelData {
	return e.channel.Data
}

// MockEmailSender is a mock email sender for testing
type MockEmailSender struct {
	ShouldFail bool
	SentEmails []SentEmail
	SendFunc   func(ctx context.Context, to, subject, body string, isHTML bool) error
}

// SentEmail represents an email that was sent
type SentEmail struct {
	To      string
	Subject string
	Body    string
	IsHTML  bool
}

// Send records the sent email or returns an error if configured to fail
func (m *MockEmailSender) Send(ctx context.Context, to, subject, body string, isHTML bool) error {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, to, subject, body, isHTML)
	}

	if m.ShouldFail {
		return fmt.Errorf("mock email send failed")
	}

	m.SentEmails = append(m.SentEmails, SentEmail{
		To:      to,
		Subject: subject,
		Body:    body,
		IsHTML:  isHTML,
	})

	return nil
}

// GetLastSentEmail returns the last sent email
func (m *MockEmailSender) GetLastSentEmail() *SentEmail {
	if len(m.SentEmails) == 0 {
		return nil
	}
	return &m.SentEmails[len(m.SentEmails)-1]
}

// Reset clears all sent emails
func (m *MockEmailSender) Reset() {
	m.SentEmails = nil
	m.ShouldFail = false
}
