// Package signature provides unified HMAC signature verification for webhooks.
//
// This package consolidates signature verification patterns used across different
// webhook providers (GitHub, GitLab, Stripe, Paddle, etc.) into a single,
// well-tested implementation.
//
// Usage:
//
//	// Use pre-configured verifiers for common providers
//	if signature.GitHub.Verify(payload, signatureHeader, secret) {
//	    // Valid GitHub webhook
//	}
//
//	// Or create custom verifiers
//	verifier := signature.NewVerifier(signature.SHA256, signature.FormatPrefixed)
//	if verifier.Verify(payload, signatureHeader, secret) {
//	    // Valid signature
//	}
package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"strconv"
	"strings"
	"time"
)

// Algorithm represents a hash algorithm for HMAC computation.
type Algorithm string

const (
	// SHA256 uses the SHA-256 hash algorithm.
	SHA256 Algorithm = "sha256"
	// SHA512 uses the SHA-512 hash algorithm.
	SHA512 Algorithm = "sha512"
)

// Format represents the format of the signature header.
type Format string

const (
	// FormatPrefixed expects signatures in the form "sha256=abc123..." or "sha512=abc123..."
	// Used by GitHub and similar providers.
	FormatPrefixed Format = "prefixed"

	// FormatRaw expects signatures as raw hex strings without any prefix.
	// Used by some providers that send only the signature value.
	FormatRaw Format = "raw"

	// FormatStripe expects signatures in Stripe's format: "t=timestamp,v1=signature"
	// Includes timestamp validation for replay attack protection.
	FormatStripe Format = "stripe"
)

// Verifier verifies HMAC signatures for webhook payloads.
type Verifier struct {
	algorithm Algorithm
	format    Format
	maxAge    time.Duration
}

// NewVerifier creates a new signature verifier with the specified algorithm and format.
func NewVerifier(algo Algorithm, format Format) *Verifier {
	return &Verifier{
		algorithm: algo,
		format:    format,
	}
}

// WithMaxAge sets the maximum age for timestamp-based signatures (e.g., Stripe).
// Signatures older than maxAge will be rejected to prevent replay attacks.
func (v *Verifier) WithMaxAge(d time.Duration) *Verifier {
	return &Verifier{
		algorithm: v.algorithm,
		format:    v.format,
		maxAge:    d,
	}
}

// Verify checks if the provided signature is valid for the given payload and secret.
// Returns true if the signature is valid, false otherwise.
func (v *Verifier) Verify(payload []byte, signatureHeader, secret string) bool {
	if signatureHeader == "" || secret == "" {
		return false
	}

	switch v.format {
	case FormatPrefixed:
		return v.verifyPrefixed(payload, signatureHeader, secret)
	case FormatRaw:
		return v.verifyRaw(payload, signatureHeader, secret)
	case FormatStripe:
		return v.verifyStripe(payload, signatureHeader, secret)
	default:
		return false
	}
}

// Sign computes the HMAC signature for a payload using the given secret.
// Returns the signature as a hex-encoded string.
func (v *Verifier) Sign(payload []byte, secret string) string {
	return v.compute(payload, secret)
}

// SignWithPrefix computes the HMAC signature and returns it with an algorithm prefix.
// For example: "sha256=abc123..."
func (v *Verifier) SignWithPrefix(payload []byte, secret string) string {
	sig := v.compute(payload, secret)
	return string(v.algorithm) + "=" + sig
}

// verifyPrefixed handles signatures in the format "algo=signature"
func (v *Verifier) verifyPrefixed(payload []byte, header, secret string) bool {
	parts := strings.SplitN(header, "=", 2)
	if len(parts) != 2 {
		return false
	}

	providedSig := parts[1]
	expected := v.compute(payload, secret)

	return hmac.Equal([]byte(providedSig), []byte(expected))
}

// verifyRaw handles signatures that are raw hex strings
func (v *Verifier) verifyRaw(payload []byte, header, secret string) bool {
	expected := v.compute(payload, secret)
	return hmac.Equal([]byte(header), []byte(expected))
}

// verifyStripe handles Stripe's signature format: "t=timestamp,v1=signature,v1=signature2,..."
// Stripe may include multiple signatures for key rotation; any valid signature passes.
func (v *Verifier) verifyStripe(payload []byte, header, secret string) bool {
	parts := strings.Split(header, ",")

	var timestamp string
	var signatures []string

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}

	if timestamp == "" || len(signatures) == 0 {
		return false
	}

	if v.maxAge > 0 {
		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			return false
		}

		signedAt := time.Unix(ts, 0)
		if time.Since(signedAt) > v.maxAge {
			return false
		}
	}

	signedPayload := timestamp + "." + string(payload)
	expected := v.compute([]byte(signedPayload), secret)

	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return true
		}
	}

	return false
}

// compute generates the HMAC signature for the payload
func (v *Verifier) compute(payload []byte, secret string) string {
	var h func() hash.Hash

	switch v.algorithm {
	case SHA256:
		h = sha256.New
	case SHA512:
		h = sha512.New
	default:
		h = sha256.New
	}

	mac := hmac.New(h, []byte(secret))
	_, _ = mac.Write(payload)

	return hex.EncodeToString(mac.Sum(nil))
}

// Pre-configured verifiers for common webhook providers.
var (
	// GitHub verifies GitHub webhook signatures.
	// GitHub sends: X-Hub-Signature-256: sha256=abc123...
	GitHub = NewVerifier(SHA256, FormatPrefixed)

	// GitLab verifies GitLab webhook signatures.
	// GitLab uses a simple token match, not HMAC, but this can verify raw signatures.
	GitLab = NewVerifier(SHA256, FormatRaw)

	// Stripe verifies Stripe webhook signatures with timestamp validation.
	// Stripe sends: Stripe-Signature: t=timestamp,v1=signature
	// Default max age is 5 minutes to prevent replay attacks.
	Stripe = NewVerifier(SHA256, FormatStripe).WithMaxAge(5 * time.Minute)

	// Paddle verifies Paddle webhook signatures.
	// Paddle sends signatures in prefixed format.
	Paddle = NewVerifier(SHA256, FormatPrefixed)

	// LemonSqueezy verifies LemonSqueezy webhook signatures.
	// LemonSqueezy sends: X-Signature: sha256=abc123...
	LemonSqueezy = NewVerifier(SHA256, FormatPrefixed)
)

// VerifyGitHub is a convenience function for verifying GitHub webhook signatures.
// signature should be the value of the X-Hub-Signature-256 header.
func VerifyGitHub(payload []byte, signature, secret string) bool {
	return GitHub.Verify(payload, signature, secret)
}

// VerifyGitLab verifies GitLab webhook tokens.
// GitLab uses simple token comparison, not HMAC signatures.
// providedToken is the value of the X-Gitlab-Token header.
func VerifyGitLab(providedToken, expectedToken string) bool {
	if providedToken == "" || expectedToken == "" {
		return false
	}
	return hmac.Equal([]byte(providedToken), []byte(expectedToken))
}

// VerifyStripe is a convenience function for verifying Stripe webhook signatures.
// signatureHeader should be the value of the Stripe-Signature header.
func VerifyStripe(payload []byte, signatureHeader, secret string) bool {
	return Stripe.Verify(payload, signatureHeader, secret)
}

// VerifyPaddle is a convenience function for verifying Paddle webhook signatures.
func VerifyPaddle(payload []byte, signature, secret string) bool {
	return Paddle.Verify(payload, signature, secret)
}

// VerifyLemonSqueezy is a convenience function for verifying LemonSqueezy webhook signatures.
// signature should be the value of the X-Signature header.
func VerifyLemonSqueezy(payload []byte, signature, secret string) bool {
	return LemonSqueezy.Verify(payload, signature, secret)
}

// ComputeSignature computes an HMAC-SHA256 signature for the given payload.
// This is useful for generating signatures when sending webhooks.
func ComputeSignature(payload []byte, secret string) string {
	return GitHub.Sign(payload, secret)
}

// ComputeSignatureWithPrefix computes an HMAC-SHA256 signature with algorithm prefix.
// Returns a string in the format "sha256=abc123..."
func ComputeSignatureWithPrefix(payload []byte, secret string) string {
	return GitHub.SignWithPrefix(payload, secret)
}

// StripeSignatureBuilder helps construct Stripe-format signature headers.
type StripeSignatureBuilder struct {
	timestamp int64
	secret    string
}

// NewStripeSignatureBuilder creates a builder for Stripe signatures.
func NewStripeSignatureBuilder(secret string) *StripeSignatureBuilder {
	return &StripeSignatureBuilder{
		timestamp: time.Now().Unix(),
		secret:    secret,
	}
}

// WithTimestamp sets a specific timestamp for the signature.
func (b *StripeSignatureBuilder) WithTimestamp(ts int64) *StripeSignatureBuilder {
	b.timestamp = ts
	return b
}

// Build generates a Stripe-format signature header for the payload.
// Returns a string in the format "t=timestamp,v1=signature"
func (b *StripeSignatureBuilder) Build(payload []byte) string {
	signedPayload := strconv.FormatInt(b.timestamp, 10) + "." + string(payload)
	verifier := NewVerifier(SHA256, FormatRaw)
	sig := verifier.Sign([]byte(signedPayload), b.secret)

	return "t=" + strconv.FormatInt(b.timestamp, 10) + ",v1=" + sig
}
