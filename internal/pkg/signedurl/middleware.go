package signedurl

import (
	"net/url"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

// Config holds the configuration for the signed URL middleware
type Config struct {
	// Signer is the signer instance to use for verification
	// If nil, the default signer will be used
	Signer *Signer

	// ErrorHandler is called when signature verification fails
	// Default: returns 403 Forbidden
	ErrorHandler fiber.ErrorHandler

	// ExpiredHandler is called when the URL has expired
	// Default: returns 403 Forbidden with "URL has expired" message
	ExpiredHandler fiber.ErrorHandler

	// SkipExpiredCheck skips the expiration check (only verify signature)
	SkipExpiredCheck bool

	// ContextKey is the key used to store verification result in context
	// Default: "signed_url_valid"
	ContextKey string
}

// DefaultConfig returns the default middleware configuration
func DefaultConfig() Config {
	return Config{
		Signer:           nil,
		ErrorHandler:     nil,
		ExpiredHandler:   nil,
		SkipExpiredCheck: false,
		ContextKey:       "signed_url_valid",
	}
}

// New creates a new signed URL verification middleware
func New(config ...Config) fiber.Handler {
	cfg := DefaultConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	if cfg.ContextKey == "" {
		cfg.ContextKey = "signed_url_valid"
	}

	return func(c *fiber.Ctx) error {
		signer := cfg.Signer
		if signer == nil {
			signer = defaultSigner
		}

		if signer == nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Signed URL signer not configured")
		}

		// Parse query parameters
		params := make(url.Values)
		c.Request().URI().QueryArgs().VisitAll(func(key, value []byte) {
			params.Add(string(key), string(value))
		})

		path := string(c.Request().URI().Path())

		// Check expiration first
		if !cfg.SkipExpiredCheck && signer.IsExpired(params) {
			if cfg.ExpiredHandler != nil {
				return cfg.ExpiredHandler(c, fiber.NewError(fiber.StatusForbidden, "URL has expired"))
			}
			return response.Forbidden(c, "URL has expired")
		}

		// Verify signature
		var valid bool
		if cfg.SkipExpiredCheck {
			valid = signer.VerifyWithoutExpiry(path, params)
		} else {
			valid = signer.Verify(path, params)
		}

		if !valid {
			if cfg.ErrorHandler != nil {
				return cfg.ErrorHandler(c, fiber.NewError(fiber.StatusForbidden, "Invalid signature"))
			}
			return response.Forbidden(c, "Invalid signature")
		}

		// Store verification result in context
		c.Locals(cfg.ContextKey, true)

		return c.Next()
	}
}

// RequireSignedURL is a simpler middleware that just checks for a valid signed URL
func RequireSignedURL(signer *Signer) fiber.Handler {
	return New(Config{
		Signer: signer,
	})
}

// RequireSignedURLWithoutExpiry checks signature but ignores expiration
func RequireSignedURLWithoutExpiry(signer *Signer) fiber.Handler {
	return New(Config{
		Signer:           signer,
		SkipExpiredCheck: true,
	})
}

// OptionalSignedURL verifies signature if present, but doesn't require it
func OptionalSignedURL(signer *Signer) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if signer == nil {
			signer = defaultSigner
		}

		if signer == nil {
			return c.Next()
		}

		// Parse query parameters
		params := make(url.Values)
		c.Request().URI().QueryArgs().VisitAll(func(key, value []byte) {
			params.Add(string(key), string(value))
		})

		// If no signature present, continue without verification
		if params.Get("signature") == "" {
			c.Locals("signed_url_valid", false)
			return c.Next()
		}

		path := string(c.Request().URI().Path())

		// Verify signature
		valid := signer.Verify(path, params)
		c.Locals("signed_url_valid", valid)

		return c.Next()
	}
}

// IsSignedURLValid returns whether the current request has a valid signed URL
func IsSignedURLValid(c *fiber.Ctx) bool {
	valid, ok := c.Locals("signed_url_valid").(bool)
	return ok && valid
}

// ValidateSignedURL is a helper function to manually validate a signed URL in a handler
func ValidateSignedURL(c *fiber.Ctx, signer *Signer) bool {
	if signer == nil {
		signer = defaultSigner
	}

	if signer == nil {
		return false
	}

	params := make(url.Values)
	c.Request().URI().QueryArgs().VisitAll(func(key, value []byte) {
		params.Add(string(key), string(value))
	})

	path := string(c.Request().URI().Path())

	return signer.Verify(path, params)
}
