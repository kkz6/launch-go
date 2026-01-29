package channels

import (
	"context"
	"errors"
	"testing"
)

func TestEmailChannel_Send(t *testing.T) {
	tests := []struct {
		name       string
		email      string
		shouldFail bool
		expectErr  bool
	}{
		{"successful send", "test@example.com", false, false},
		{"send fails", "test@example.com", true, true},
		{"empty email", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSender := &MockEmailSender{ShouldFail: tt.shouldFail}
			nc := &NotificationChannel{
				Provider: ChannelTypeEmail,
				Data:     ChannelData{Email: tt.email},
			}
			channel := NewEmailChannel(nc, mockSender)

			notif := NewMockNotification("Test notification")

			err := channel.Send(context.Background(), notif)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(mockSender.SentEmails) != 1 {
					t.Errorf("expected 1 sent email, got %d", len(mockSender.SentEmails))
				}
			}
		})
	}
}

func TestEmailChannel_Send_NilSender(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeEmail,
		Data:     ChannelData{Email: "test@example.com"},
	}
	channel := NewEmailChannel(nc, nil)

	notif := NewMockNotification("Test notification")

	err := channel.Send(context.Background(), notif)
	if err == nil {
		t.Error("expected error for nil sender")
	}
}

func TestEmailChannel_Connect(t *testing.T) {
	tests := []struct {
		name       string
		email      string
		shouldFail bool
		expectErr  bool
	}{
		{"successful connect", "test@example.com", false, false},
		{"connect fails", "test@example.com", true, true},
		{"empty email", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSender := &MockEmailSender{ShouldFail: tt.shouldFail}
			nc := &NotificationChannel{
				Provider: ChannelTypeEmail,
				Data:     ChannelData{Email: tt.email},
			}
			channel := NewEmailChannel(nc, mockSender)

			err := channel.Connect(context.Background())

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestEmailChannel_Connect_NilSender(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeEmail,
		Data:     ChannelData{Email: "test@example.com"},
	}
	channel := NewEmailChannel(nc, nil)

	err := channel.Connect(context.Background())
	if err == nil {
		t.Error("expected error for nil sender")
	}
}

func TestEmailChannel_GetCreateRules(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeEmail,
	}
	channel := NewEmailChannel(nc, nil)

	rules := channel.GetCreateRules()

	if rules["email"] != "required,email" {
		t.Errorf("email rule = %s, want 'required,email'", rules["email"])
	}
}

func TestEmailChannel_GetData(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeEmail,
		Data: ChannelData{
			Email:     "test@example.com",
			AppDeploy: true,
		},
	}
	channel := NewEmailChannel(nc, nil)

	data := channel.GetData()

	if data.Email != "test@example.com" {
		t.Errorf("Email = %s, want 'test@example.com'", data.Email)
	}
	if !data.AppDeploy {
		t.Error("AppDeploy should be true")
	}
}

func TestMockEmailSender_Send(t *testing.T) {
	sender := &MockEmailSender{}
	ctx := context.Background()

	err := sender.Send(ctx, "test@example.com", "Subject", "Body", false)
	if err != nil {
		t.Errorf("Send() error = %v", err)
	}

	if len(sender.SentEmails) != 1 {
		t.Errorf("len(SentEmails) = %d, want 1", len(sender.SentEmails))
	}

	email := sender.SentEmails[0]
	if email.To != "test@example.com" {
		t.Errorf("To = %s, want 'test@example.com'", email.To)
	}
	if email.Subject != "Subject" {
		t.Errorf("Subject = %s, want 'Subject'", email.Subject)
	}
	if email.Body != "Body" {
		t.Errorf("Body = %s, want 'Body'", email.Body)
	}
	if email.IsHTML {
		t.Error("IsHTML should be false")
	}
}

func TestMockEmailSender_Send_WithFunc(t *testing.T) {
	customErr := errors.New("custom error")
	sender := &MockEmailSender{
		SendFunc: func(ctx context.Context, to, subject, body string, isHTML bool) error {
			return customErr
		},
	}

	err := sender.Send(context.Background(), "test@example.com", "Subject", "Body", false)
	if err != customErr {
		t.Errorf("expected custom error, got %v", err)
	}
}

func TestMockEmailSender_Send_ShouldFail(t *testing.T) {
	sender := &MockEmailSender{ShouldFail: true}

	err := sender.Send(context.Background(), "test@example.com", "Subject", "Body", false)
	if err == nil {
		t.Error("expected error when ShouldFail is true")
	}
}

func TestMockEmailSender_GetLastSentEmail(t *testing.T) {
	sender := &MockEmailSender{}

	// No emails sent yet
	if sender.GetLastSentEmail() != nil {
		t.Error("GetLastSentEmail() should return nil when no emails sent")
	}

	// Send an email
	_ = sender.Send(context.Background(), "first@example.com", "First", "Body", false)
	_ = sender.Send(context.Background(), "second@example.com", "Second", "Body", true)

	last := sender.GetLastSentEmail()
	if last == nil {
		t.Error("GetLastSentEmail() should not be nil")
		return
	}
	if last.To != "second@example.com" {
		t.Errorf("last.To = %s, want 'second@example.com'", last.To)
	}
}

func TestMockEmailSender_Reset(t *testing.T) {
	sender := &MockEmailSender{ShouldFail: true}
	_ = sender.Send(context.Background(), "test@example.com", "Subject", "Body", false)

	sender.Reset()

	if sender.ShouldFail {
		t.Error("ShouldFail should be false after Reset()")
	}
	if len(sender.SentEmails) != 0 {
		t.Errorf("len(SentEmails) = %d, want 0 after Reset()", len(sender.SentEmails))
	}
}

func TestNewEmailChannel(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeEmail,
	}
	sender := &MockEmailSender{}

	channel := NewEmailChannel(nc, sender)

	if channel == nil {
		t.Fatal("NewEmailChannel() returned nil")
	}
	if channel.channel != nc {
		t.Error("channel not set correctly")
	}
	if channel.sender != sender {
		t.Error("sender not set correctly")
	}
}
