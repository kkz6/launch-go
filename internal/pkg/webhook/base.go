// Package webhook provides common infrastructure for webhook handlers.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// Common webhook header names
const (
	HeaderGitHubSignature       = "X-Hub-Signature-256"
	HeaderGitLabToken           = "X-Gitlab-Token"
	HeaderLemonSqueezySignature = "X-Signature"
)

// VerifyHMACSHA256 verifies an HMAC-SHA256 signature
func VerifyHMACSHA256(payload []byte, signature string, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload) // hash.Hash.Write never returns an error
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(signature))
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
