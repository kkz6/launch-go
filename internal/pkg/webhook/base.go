// Package webhook provides common infrastructure for webhook handlers.
package webhook

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/pkg/signature"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// Common webhook header names
const (
	HeaderGitHubSignature       = "X-Hub-Signature-256"
	HeaderGitLabToken           = "X-Gitlab-Token"
	HeaderLemonSqueezySignature = "X-Signature"
	HeaderStripeSignature       = "Stripe-Signature"
	HeaderPaddleSignature       = "Paddle-Signature"
)

// VerifyHMACSHA256 verifies an HMAC-SHA256 signature.
// Deprecated: Use signature.GitLab.Verify or signature.ComputeSignature instead.
func VerifyHMACSHA256(payload []byte, sig string, secret string) bool {
	return signature.GitLab.Verify(payload, sig, secret)
}

// VerifyGitHubSignature verifies a GitHub webhook signature (X-Hub-Signature-256 header).
func VerifyGitHubSignature(payload []byte, signatureHeader, secret string) bool {
	return signature.VerifyGitHub(payload, signatureHeader, secret)
}

// VerifyGitLabToken verifies a GitLab webhook token (X-Gitlab-Token header).
func VerifyGitLabToken(providedToken, expectedToken string) bool {
	return signature.VerifyGitLab(providedToken, expectedToken)
}

// VerifyStripeSignature verifies a Stripe webhook signature (Stripe-Signature header).
func VerifyStripeSignature(payload []byte, signatureHeader, secret string) bool {
	return signature.VerifyStripe(payload, signatureHeader, secret)
}

// VerifyLemonSqueezySignature verifies a LemonSqueezy webhook signature (X-Signature header).
func VerifyLemonSqueezySignature(payload []byte, signatureHeader, secret string) bool {
	return signature.VerifyLemonSqueezy(payload, signatureHeader, secret)
}

// Base provides common dependencies and functionality for webhook handlers.
// Embed this struct in specific handlers to get access to shared resources.
type Base struct {
	Signer *signedurl.Signer
	Logger *zerolog.Logger
}

// NewBase creates a new Base with the given secret key and logger.
func NewBase(secretKey string, logger *zerolog.Logger) Base {
	return Base{
		Signer: signedurl.NewSigner(secretKey),
		Logger: logger,
	}
}

// VerifySignature validates the request signature using the signed URL signer.
// Returns true if the signature is valid, false otherwise.
func (b *Base) VerifySignature(c *fiber.Ctx) bool {
	return signedurl.ValidateSignedURL(c, b.Signer)
}

// LogReceived logs a webhook received event at debug level.
func (b *Base) LogReceived(webhookType string, fields ...any) {
	event := b.Logger.Debug().Str("type", webhookType)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg("Webhook received")
}

// LogInfo logs an info message with optional fields.
func (b *Base) LogInfo(msg string, fields ...any) {
	event := b.Logger.Info()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogWarn logs a warning message with optional fields.
func (b *Base) LogWarn(msg string, fields ...any) {
	event := b.Logger.Warn()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogError logs an error with optional fields.
func (b *Base) LogError(err error, msg string, fields ...any) {
	event := b.Logger.Error().Err(err)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogDebug logs a debug message with optional fields.
func (b *Base) LogDebug(msg string, fields ...any) {
	event := b.Logger.Debug()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}
