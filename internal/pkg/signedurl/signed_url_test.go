package signedurl

import (
	"net/url"
	"testing"
	"time"
)

func TestSigner_SignedURL(t *testing.T) {
	signer := NewSigner("test-secret-key")

	signedURL := signer.SignedURL("/api/test", map[string]string{
		"id": "123",
	}, 1*time.Hour)

	// Parse the URL
	parsedURL, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("Failed to parse signed URL: %v", err)
	}

	// Check that signature is present
	params := parsedURL.Query()
	if params.Get("signature") == "" {
		t.Error("Signature should be present in URL")
	}

	// Check that expires is present
	if params.Get("expires") == "" {
		t.Error("Expires should be present in URL")
	}

	// Check that id is present
	if params.Get("id") != "123" {
		t.Errorf("Expected id=123, got id=%s", params.Get("id"))
	}
}

func TestSigner_Verify(t *testing.T) {
	signer := NewSigner("test-secret-key")

	// Generate a signed URL
	signedURL := signer.SignedURL("/api/test", map[string]string{
		"id": "123",
	}, 1*time.Hour)

	// Parse the URL
	parsedURL, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("Failed to parse signed URL: %v", err)
	}

	// Verify should pass
	if !signer.Verify(parsedURL.Path, parsedURL.Query()) {
		t.Error("Verification should pass for valid signature")
	}
}

func TestSigner_Verify_InvalidSignature(t *testing.T) {
	signer := NewSigner("test-secret-key")

	params := url.Values{}
	params.Set("id", "123")
	params.Set("expires", "9999999999")
	params.Set("signature", "invalid-signature")

	if signer.Verify("/api/test", params) {
		t.Error("Verification should fail for invalid signature")
	}
}

func TestSigner_Verify_ExpiredURL(t *testing.T) {
	signer := NewSigner("test-secret-key")

	// Generate a signed URL that's already expired
	signedURL := signer.SignedURL("/api/test", map[string]string{
		"id": "123",
	}, -1*time.Hour) // Negative duration = already expired

	// Parse the URL
	parsedURL, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("Failed to parse signed URL: %v", err)
	}

	// Verify should fail due to expiration
	if signer.Verify(parsedURL.Path, parsedURL.Query()) {
		t.Error("Verification should fail for expired URL")
	}
}

func TestSigner_VerifyWithoutExpiry(t *testing.T) {
	signer := NewSigner("test-secret-key")

	// Generate a signed URL that's already expired
	signedURL := signer.SignedURL("/api/test", map[string]string{
		"id": "123",
	}, -1*time.Hour) // Negative duration = already expired

	// Parse the URL
	parsedURL, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("Failed to parse signed URL: %v", err)
	}

	// VerifyWithoutExpiry should pass despite expiration
	if !signer.VerifyWithoutExpiry(parsedURL.Path, parsedURL.Query()) {
		t.Error("VerifyWithoutExpiry should pass even for expired URL")
	}
}

func TestSigner_PermanentSignedURL(t *testing.T) {
	signer := NewSigner("test-secret-key")

	signedURL := signer.PermanentSignedURL("/api/test", map[string]string{
		"id": "123",
	})

	// Parse the URL
	parsedURL, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("Failed to parse signed URL: %v", err)
	}

	params := parsedURL.Query()

	// Check that signature is present
	if params.Get("signature") == "" {
		t.Error("Signature should be present in URL")
	}

	// Check that expires is NOT present
	if params.Get("expires") != "" {
		t.Error("Expires should NOT be present in permanent URL")
	}

	// Verify should pass
	if !signer.Verify(parsedURL.Path, params) {
		t.Error("Verification should pass for valid permanent signature")
	}
}

func TestSigner_WithBaseURL(t *testing.T) {
	signer := NewSigner("test-secret-key").WithBaseURL("https://example.com")

	signedURL := signer.SignedURL("/api/test", map[string]string{
		"id": "123",
	}, 1*time.Hour)

	// Check that URL starts with base URL
	if signedURL[:19] != "https://example.com" {
		t.Errorf("URL should start with base URL, got: %s", signedURL)
	}
}

func TestSigner_DifferentSecrets(t *testing.T) {
	signer1 := NewSigner("secret-1")
	signer2 := NewSigner("secret-2")

	// Generate URL with signer1
	signedURL := signer1.SignedURL("/api/test", map[string]string{
		"id": "123",
	}, 1*time.Hour)

	// Parse the URL
	parsedURL, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("Failed to parse signed URL: %v", err)
	}

	// Verify with signer2 should fail
	if signer2.Verify(parsedURL.Path, parsedURL.Query()) {
		t.Error("Verification with different secret should fail")
	}
}

func TestSignedURLParams(t *testing.T) {
	params := NewParams().
		Add("name", "test").
		AddInt("count", 42).
		AddInt64("id", 1234567890123).
		Build()

	if params["name"] != "test" {
		t.Errorf("Expected name=test, got name=%s", params["name"])
	}

	if params["count"] != "42" {
		t.Errorf("Expected count=42, got count=%s", params["count"])
	}

	if params["id"] != "1234567890123" {
		t.Errorf("Expected id=1234567890123, got id=%s", params["id"])
	}
}

func TestIsExpired(t *testing.T) {
	signer := NewSigner("test-secret-key")

	// Test expired URL
	expiredParams := url.Values{}
	expiredParams.Set("expires", "1000000000") // Way in the past
	if !signer.IsExpired(expiredParams) {
		t.Error("Should detect expired URL")
	}

	// Test valid URL
	validParams := url.Values{}
	validParams.Set("expires", "9999999999") // Way in the future
	if signer.IsExpired(validParams) {
		t.Error("Should not detect valid URL as expired")
	}

	// Test URL without expiration
	noExpiryParams := url.Values{}
	if signer.IsExpired(noExpiryParams) {
		t.Error("URL without expiration should not be considered expired")
	}
}
