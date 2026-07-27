package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"strconv"
	"strings"
	"time"
)

// SignatureAlgorithm represents a hash algorithm for HMAC computation.
type SignatureAlgorithm string

const (
	// SignatureSHA256 uses the SHA-256 hash algorithm.
	SignatureSHA256 SignatureAlgorithm = "sha256"
	// SignatureSHA512 uses the SHA-512 hash algorithm.
	SignatureSHA512 SignatureAlgorithm = "sha512"
)

// SignatureFormat represents the format of the signature header.
type SignatureFormat string

const (
	// SignatureFormatPrefixed expects signatures in the form "sha256=abc123..." or "sha512=abc123..."
	// Used by GitHub and similar providers.
	SignatureFormatPrefixed SignatureFormat = "prefixed"

	// SignatureFormatRaw expects signatures as raw hex strings without any prefix.
	// Used by some providers that send only the signature value.
	SignatureFormatRaw SignatureFormat = "raw"

	// SignatureFormatStripe expects signatures in Stripe's format: "t=timestamp,v1=signature"
	// Includes timestamp validation for replay attack protection.
	SignatureFormatStripe SignatureFormat = "stripe"
)

// SignatureVerifier verifies HMAC signatures for webhook payloads.
type SignatureVerifier struct {
	algorithm SignatureAlgorithm
	format    SignatureFormat
	maxAge    time.Duration
}

// NewSignatureVerifier creates a new signature verifier with the specified algorithm and format.
func NewSignatureVerifier(algo SignatureAlgorithm, format SignatureFormat) *SignatureVerifier {
	return &SignatureVerifier{
		algorithm: algo,
		format:    format,
	}
}

// WithMaxAge sets the maximum age for timestamp-based signatures (e.g., Stripe).
// Signatures older than maxAge will be rejected to prevent replay attacks.
func (v *SignatureVerifier) WithMaxAge(d time.Duration) *SignatureVerifier {
	return &SignatureVerifier{
		algorithm: v.algorithm,
		format:    v.format,
		maxAge:    d,
	}
}

// Verify checks if the provided signature is valid for the given payload and secret.
// Returns true if the signature is valid, false otherwise.
func (v *SignatureVerifier) Verify(payload []byte, signatureHeader, secret string) bool {
	if signatureHeader == "" || secret == "" {
		return false
	}

	switch v.format {
	case SignatureFormatPrefixed:
		return v.verifyPrefixed(payload, signatureHeader, secret)
	case SignatureFormatRaw:
		return v.verifyRaw(payload, signatureHeader, secret)
	case SignatureFormatStripe:
		return v.verifyStripe(payload, signatureHeader, secret)
	default:
		return false
	}
}

// VerifyStandardWebhooks verifies a Standard Webhooks signature.
// Used by providers following the Standard Webhooks spec.
// The signed payload format is: {webhook-id}.{webhook-timestamp}.{payload}
func (v *SignatureVerifier) VerifyStandardWebhooks(payload []byte, webhookID, signature, timestamp, secret string) bool {
	if webhookID == "" || signature == "" || timestamp == "" || secret == "" {
		return false
	}

	if v.maxAge > 0 {
		if !timestampWithinMaxAge(timestamp, v.maxAge) {
			return false
		}
	}

	signedPayload := webhookID + "." + timestamp + "." + string(payload)
	expected := v.computeBase64([]byte(signedPayload), secret)

	signatures := strings.Split(signature, " ")
	for _, sig := range signatures {
		if strings.HasPrefix(sig, "v1,") {
			actualSig := strings.TrimPrefix(sig, "v1,")
			if hmac.Equal([]byte(expected), []byte(actualSig)) {
				return true
			}
		}
	}

	return false
}

// computeBase64 generates the HMAC signature and returns it as base64
func (v *SignatureVerifier) computeBase64(payload []byte, secret string) string {
	var h func() hash.Hash

	switch v.algorithm {
	case SignatureSHA256:
		h = sha256.New
	case SignatureSHA512:
		h = sha512.New
	default:
		h = sha256.New
	}

	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		secretBytes = []byte(secret)
	}

	mac := hmac.New(h, secretBytes)
	_, _ = mac.Write(payload)

	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// Sign computes the HMAC signature for a payload using the given secret.
// Returns the signature as a hex-encoded string.
func (v *SignatureVerifier) Sign(payload []byte, secret string) string {
	return v.compute(payload, secret)
}

// SignWithPrefix computes the HMAC signature and returns it with an algorithm prefix.
// For example: "sha256=abc123..."
func (v *SignatureVerifier) SignWithPrefix(payload []byte, secret string) string {
	sig := v.compute(payload, secret)
	return string(v.algorithm) + "=" + sig
}

// verifyPrefixed handles signatures in the format "algo=signature"
func (v *SignatureVerifier) verifyPrefixed(payload []byte, header, secret string) bool {
	parts := strings.SplitN(header, "=", 2)
	if len(parts) != 2 {
		return false
	}
	if parts[0] != string(v.algorithm) {
		return false
	}

	providedSig := parts[1]
	expected := v.compute(payload, secret)

	return hmac.Equal([]byte(providedSig), []byte(expected))
}

// verifyRaw handles signatures that are raw hex strings
func (v *SignatureVerifier) verifyRaw(payload []byte, header, secret string) bool {
	expected := v.compute(payload, secret)
	return hmac.Equal([]byte(header), []byte(expected))
}

// verifyStripe handles Stripe's signature format: "t=timestamp,v1=signature,v1=signature2,..."
// Stripe may include multiple signatures for key rotation; any valid signature passes.
func (v *SignatureVerifier) verifyStripe(payload []byte, header, secret string) bool {
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
		if !timestampWithinMaxAge(timestamp, v.maxAge) {
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

func timestampWithinMaxAge(timestamp string, maxAge time.Duration) bool {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	return time.Since(time.Unix(ts, 0)).Abs() <= maxAge
}

// compute generates the HMAC signature for the payload
func (v *SignatureVerifier) compute(payload []byte, secret string) string {
	var h func() hash.Hash

	switch v.algorithm {
	case SignatureSHA256:
		h = sha256.New
	case SignatureSHA512:
		h = sha512.New
	default:
		h = sha256.New
	}

	mac := hmac.New(h, []byte(secret))
	_, _ = mac.Write(payload)

	return hex.EncodeToString(mac.Sum(nil))
}

// SignatureFormatStandardWebhooks expects signatures in Standard Webhooks format.
// Headers: webhook-id, webhook-signature (v1,signature), webhook-timestamp
// Used by providers following the Standard Webhooks spec.
const SignatureFormatStandardWebhooks SignatureFormat = "standard_webhooks"

// Pre-configured verifiers for common webhook providers.
var (
	// GitHubSignature verifies GitHub webhook signatures.
	// GitHub sends: X-Hub-Signature-256: sha256=abc123...
	GitHubSignature = NewSignatureVerifier(SignatureSHA256, SignatureFormatPrefixed)

	// GitLabSignature verifies GitLab webhook signatures.
	// GitLab uses a simple token match, not HMAC, but this can verify raw signatures.
	GitLabSignature = NewSignatureVerifier(SignatureSHA256, SignatureFormatRaw)

	// StripeSignature verifies Stripe webhook signatures with timestamp validation.
	// Stripe sends: Stripe-Signature: t=timestamp,v1=signature
	// Default max age is 5 minutes to prevent replay attacks.
	StripeSignature = NewSignatureVerifier(SignatureSHA256, SignatureFormatStripe).WithMaxAge(5 * time.Minute)

	// PaddleSignature verifies Paddle webhook signatures.
	// Paddle sends signatures in prefixed format.
	PaddleSignature = NewSignatureVerifier(SignatureSHA256, SignatureFormatPrefixed)

	// StandardWebhooksSignature verifies Standard Webhooks signatures.
	// The spec uses headers: webhook-id, webhook-signature
	// (v1,base64signature), webhook-timestamp.
	// Default max age is 5 minutes to prevent replay attacks.
	StandardWebhooksSignature = NewSignatureVerifier(SignatureSHA256, SignatureFormatStandardWebhooks).WithMaxAge(5 * time.Minute)
)

// VerifyGitHubSignature is a convenience function for verifying GitHub webhook signatures.
// signature should be the value of the X-Hub-Signature-256 header.
func VerifyGitHubSignature(payload []byte, signature, secret string) bool {
	return GitHubSignature.Verify(payload, signature, secret)
}

// VerifyGitLabToken verifies GitLab webhook tokens.
// GitLab uses simple token comparison, not HMAC signatures.
// providedToken is the value of the X-Gitlab-Token header.
func VerifyGitLabToken(providedToken, expectedToken string) bool {
	if providedToken == "" || expectedToken == "" {
		return false
	}
	return hmac.Equal([]byte(providedToken), []byte(expectedToken))
}

// VerifyStripeSignature is a convenience function for verifying Stripe webhook signatures.
// signatureHeader should be the value of the Stripe-Signature header.
func VerifyStripeSignature(payload []byte, signatureHeader, secret string) bool {
	return StripeSignature.Verify(payload, signatureHeader, secret)
}

// VerifyPaddleSignature is a convenience function for verifying Paddle webhook signatures.
func VerifyPaddleSignature(payload []byte, signature, secret string) bool {
	return PaddleSignature.Verify(payload, signature, secret)
}

// VerifyStandardWebhooksSignature is a convenience function for verifying Standard Webhooks signatures.
// Standard Webhooks spec uses with three headers:
// - webhook-id: Unique identifier for the webhook
// - webhook-signature: The signature in format "v1,base64signature"
// - webhook-timestamp: Unix timestamp when the webhook was sent
func VerifyStandardWebhooksSignature(payload []byte, webhookID, signature, timestamp, secret string) bool {
	return StandardWebhooksSignature.VerifyStandardWebhooks(payload, webhookID, signature, timestamp, secret)
}

// ComputeHMACSignature computes an HMAC-SHA256 signature for the given payload.
// This is useful for generating signatures when sending webhooks.
func ComputeHMACSignature(payload []byte, secret string) string {
	return GitHubSignature.Sign(payload, secret)
}

// ComputeHMACSignatureWithPrefix computes an HMAC-SHA256 signature with algorithm prefix.
// Returns a string in the format "sha256=abc123..."
func ComputeHMACSignatureWithPrefix(payload []byte, secret string) string {
	return GitHubSignature.SignWithPrefix(payload, secret)
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
	verifier := NewSignatureVerifier(SignatureSHA256, SignatureFormatRaw)
	sig := verifier.Sign([]byte(signedPayload), b.secret)

	return "t=" + strconv.FormatInt(b.timestamp, 10) + ",v1=" + sig
}
