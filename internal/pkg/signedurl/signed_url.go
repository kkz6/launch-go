// Package signedurl provides Laravel-style signed URL generation and verification for Fiber.
// It allows creating URLs with HMAC signatures that expire after a specified duration.
package signedurl

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"
)

// Signer handles signed URL generation and verification
type Signer struct {
	secretKey string
	baseURL   string
}

// NewSigner creates a new Signer with the given secret key
func NewSigner(secretKey string) *Signer {
	return &Signer{
		secretKey: secretKey,
	}
}

// WithBaseURL sets the base URL for generating absolute URLs
func (s *Signer) WithBaseURL(baseURL string) *Signer {
	s.baseURL = baseURL
	return s
}

// GetSecretKey returns the secret key used for signing
func (s *Signer) GetSecretKey() string {
	return s.secretKey
}

// SignedURL generates a signed URL with an expiration time
func (s *Signer) SignedURL(path string, params map[string]string, expiresIn time.Duration) string {
	expires := time.Now().Add(expiresIn).Unix()
	return s.SignedURLWithExpiry(path, params, expires)
}

// SignedURLWithExpiry generates a signed URL with a specific expiry timestamp
func (s *Signer) SignedURLWithExpiry(path string, params map[string]string, expiresAt int64) string {
	// Build query parameters
	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}
	query.Set("expires", strconv.FormatInt(expiresAt, 10))

	// Generate signature
	signatureData := path + "?" + query.Encode()
	signature := s.generateSignature(signatureData)
	query.Set("signature", signature)

	// Build final URL
	if s.baseURL != "" {
		return fmt.Sprintf("%s%s?%s", s.baseURL, path, query.Encode())
	}
	return fmt.Sprintf("%s?%s", path, query.Encode())
}

// TemporarySignedURL generates a signed URL that expires after the specified duration
// This is the equivalent of Laravel's URL::temporarySignedRoute()
func (s *Signer) TemporarySignedURL(path string, params map[string]string, expiresIn time.Duration) string {
	return s.SignedURL(path, params, expiresIn)
}

// PermanentSignedURL generates a signed URL without expiration
// Note: The signature still includes all parameters for verification
func (s *Signer) PermanentSignedURL(path string, params map[string]string) string {
	// Build query parameters
	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}

	// Generate signature without expiry
	signatureData := path + "?" + query.Encode()
	signature := s.generateSignature(signatureData)
	query.Set("signature", signature)

	// Build final URL
	if s.baseURL != "" {
		return fmt.Sprintf("%s%s?%s", s.baseURL, path, query.Encode())
	}
	return fmt.Sprintf("%s?%s", path, query.Encode())
}

// Verify checks if a signed URL is valid and not expired
func (s *Signer) Verify(path string, params url.Values) bool {
	signature := params.Get("signature")
	if signature == "" {
		return false
	}

	// Check expiration if present
	expiresStr := params.Get("expires")
	if expiresStr != "" {
		expires, err := strconv.ParseInt(expiresStr, 10, 64)
		if err != nil {
			return false
		}
		if time.Now().Unix() > expires {
			return false
		}
	}

	// Rebuild signature data without the signature parameter
	paramsCopy := url.Values{}
	for k, v := range params {
		if k != "signature" {
			paramsCopy[k] = v
		}
	}

	signatureData := path + "?" + paramsCopy.Encode()
	expectedSignature := s.generateSignature(signatureData)

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// VerifyWithoutExpiry checks if a signed URL is valid without checking expiration
func (s *Signer) VerifyWithoutExpiry(path string, params url.Values) bool {
	signature := params.Get("signature")
	if signature == "" {
		return false
	}

	// Rebuild signature data without the signature parameter
	paramsCopy := url.Values{}
	for k, v := range params {
		if k != "signature" {
			paramsCopy[k] = v
		}
	}

	signatureData := path + "?" + paramsCopy.Encode()
	expectedSignature := s.generateSignature(signatureData)

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// HasValidSignature checks if the signature is valid (alias for Verify)
func (s *Signer) HasValidSignature(path string, params url.Values) bool {
	return s.Verify(path, params)
}

// IsExpired checks if a signed URL has expired
func (s *Signer) IsExpired(params url.Values) bool {
	expiresStr := params.Get("expires")
	if expiresStr == "" {
		return false // No expiration set
	}

	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return true // Invalid expiration is treated as expired
	}

	return time.Now().Unix() > expires
}

// generateSignature creates an HMAC-SHA256 signature
func (s *Signer) generateSignature(data string) string {
	mac := hmac.New(sha256.New, []byte(s.secretKey))
	_, _ = mac.Write([]byte(data)) // hash.Hash.Write never returns an error
	return hex.EncodeToString(mac.Sum(nil))
}

// SignedURLParams is a helper struct for building signed URL parameters
type SignedURLParams struct {
	params map[string]string
}

// NewParams creates a new SignedURLParams builder
func NewParams() *SignedURLParams {
	return &SignedURLParams{
		params: make(map[string]string),
	}
}

// Add adds a parameter to the signed URL
func (p *SignedURLParams) Add(key, value string) *SignedURLParams {
	p.params[key] = value
	return p
}

// AddInt adds an integer parameter to the signed URL
func (p *SignedURLParams) AddInt(key string, value int) *SignedURLParams {
	p.params[key] = strconv.Itoa(value)
	return p
}

// AddInt64 adds an int64 parameter to the signed URL
func (p *SignedURLParams) AddInt64(key string, value int64) *SignedURLParams {
	p.params[key] = strconv.FormatInt(value, 10)
	return p
}

// Build returns the map of parameters
func (p *SignedURLParams) Build() map[string]string {
	return p.params
}

// Global signer instance (set during app initialization)
var defaultSigner atomic.Pointer[Signer]

// SetDefaultSigner sets the global signer instance
func SetDefaultSigner(signer *Signer) {
	defaultSigner.Store(signer)
}

// GetDefaultSigner returns the global signer instance
func GetDefaultSigner() *Signer {
	return defaultSigner.Load()
}

// Sign generates a signed URL using the default signer
func Sign(path string, params map[string]string, expiresIn time.Duration) string {
	signer := GetDefaultSigner()
	if signer == nil {
		panic("signedurl: default signer not configured")
	}
	return signer.SignedURL(path, params, expiresIn)
}

// TemporarySign generates a temporary signed URL using the default signer
func TemporarySign(path string, params map[string]string, expiresIn time.Duration) string {
	signer := GetDefaultSigner()
	if signer == nil {
		panic("signedurl: default signer not configured")
	}
	return signer.TemporarySignedURL(path, params, expiresIn)
}

// PermanentSign generates a permanent signed URL using the default signer
func PermanentSign(path string, params map[string]string) string {
	signer := GetDefaultSigner()
	if signer == nil {
		panic("signedurl: default signer not configured")
	}
	return signer.PermanentSignedURL(path, params)
}

// publicSigner is the signer for user-facing URLs (rendered on the frontend
// domain, then proxied to the API). Verification still happens on the API
// with the default signer — both share the same secret key, so the
// signature matches regardless of which host the user hits.
var publicSigner atomic.Pointer[Signer]

// SetPublicSigner sets the global public-facing signer instance.
func SetPublicSigner(signer *Signer) {
	publicSigner.Store(signer)
}

// PublicPermanentSign generates a permanent signed URL on the public-facing
// (frontend) host. Falls back to the default signer when the public signer
// isn't configured.
func PublicPermanentSign(path string, params map[string]string) string {
	s := publicSigner.Load()
	if s == nil {
		s = GetDefaultSigner()
	}
	if s == nil {
		panic("signedurl: no signer configured")
	}
	return s.PermanentSignedURL(path, params)
}
