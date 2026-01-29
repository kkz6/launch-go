package mail

import (
	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
)

// NewEmailSender creates an EmailSender based on the mail configuration.
// Returns nil if the driver is not configured or the required credentials are missing.
func NewEmailSender(cfg config.MailConfig) channels.EmailSender {
	switch cfg.Driver {
	case "resend":
		if cfg.ResendKey == "" {
			return nil
		}
		return NewResendSender(cfg.ResendKey, cfg.FromAddress, cfg.FromName)
	case "smtp":
		if cfg.SMTPHost == "" {
			return nil
		}
		return NewSMTPSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.FromAddress, cfg.FromName)
	default:
		return nil
	}
}
