package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

func TestVerifier_Verify_PrefixedFormat(t *testing.T) {
	secret := "test-secret-key"
	payload := []byte(`{"event": "push", "repository": "test/repo"}`)

	verifier := NewVerifier(SHA256, FormatPrefixed)
	validSig := "sha256=" + verifier.Sign(payload, secret)

	tests := []struct {
		name      string
		signature string
		secret    string
		want      bool
	}{
		{
			name:      "valid signature",
			signature: validSig,
			secret:    secret,
			want:      true,
		},
		{
			name:      "invalid signature",
			signature: "sha256=invalid",
			secret:    secret,
			want:      false,
		},
		{
			name:      "wrong secret",
			signature: validSig,
			secret:    "wrong-secret",
			want:      false,
		},
		{
			name:      "empty signature",
			signature: "",
			secret:    secret,
			want:      false,
		},
		{
			name:      "empty secret",
			signature: validSig,
			secret:    "",
			want:      false,
		},
		{
			name:      "malformed signature - no equals",
			signature: "sha256abc123",
			secret:    secret,
			want:      false,
		},
		{
			name:      "wrong algorithm prefix",
			signature: "sha512=" + verifier.Sign(payload, secret),
			secret:    secret,
			want:      true, // We only compare the signature part, not the prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifier.Verify(payload, tt.signature, tt.secret)
			if got != tt.want {
				t.Errorf("Verify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVerifier_Verify_RawFormat(t *testing.T) {
	secret := "test-secret-key"
	payload := []byte(`{"event": "push"}`)

	verifier := NewVerifier(SHA256, FormatRaw)
	validSig := verifier.Sign(payload, secret)

	tests := []struct {
		name      string
		signature string
		secret    string
		want      bool
	}{
		{
			name:      "valid signature",
			signature: validSig,
			secret:    secret,
			want:      true,
		},
		{
			name:      "invalid signature",
			signature: "invalid",
			secret:    secret,
			want:      false,
		},
		{
			name:      "wrong secret",
			signature: validSig,
			secret:    "wrong-secret",
			want:      false,
		},
		{
			name:      "empty signature",
			signature: "",
			secret:    secret,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifier.Verify(payload, tt.signature, tt.secret)
			if got != tt.want {
				t.Errorf("Verify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVerifier_Verify_StripeFormat(t *testing.T) {
	secret := "whsec_test_secret"
	payload := []byte(`{"type": "checkout.session.completed"}`)

	now := time.Now().Unix()
	signedPayload := strconv.FormatInt(now, 10) + "." + string(payload)

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signedPayload))
	validSig := hex.EncodeToString(mac.Sum(nil))
	validHeader := "t=" + strconv.FormatInt(now, 10) + ",v1=" + validSig

	verifier := NewVerifier(SHA256, FormatStripe).WithMaxAge(5 * time.Minute)

	tests := []struct {
		name      string
		signature string
		secret    string
		want      bool
	}{
		{
			name:      "valid signature",
			signature: validHeader,
			secret:    secret,
			want:      true,
		},
		{
			name:      "invalid signature",
			signature: "t=" + strconv.FormatInt(now, 10) + ",v1=invalid",
			secret:    secret,
			want:      false,
		},
		{
			name:      "wrong secret",
			signature: validHeader,
			secret:    "wrong-secret",
			want:      false,
		},
		{
			name:      "missing timestamp",
			signature: "v1=" + validSig,
			secret:    secret,
			want:      false,
		},
		{
			name:      "missing signature",
			signature: "t=" + strconv.FormatInt(now, 10),
			secret:    secret,
			want:      false,
		},
		{
			name:      "empty header",
			signature: "",
			secret:    secret,
			want:      false,
		},
		{
			name:      "invalid timestamp format",
			signature: "t=invalid,v1=" + validSig,
			secret:    secret,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifier.Verify(payload, tt.signature, tt.secret)
			if got != tt.want {
				t.Errorf("Verify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVerifier_Verify_StripeFormat_ExpiredTimestamp(t *testing.T) {
	secret := "whsec_test_secret"
	payload := []byte(`{"type": "checkout.session.completed"}`)

	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	signedPayload := strconv.FormatInt(oldTimestamp, 10) + "." + string(payload)

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signedPayload))
	validSig := hex.EncodeToString(mac.Sum(nil))
	expiredHeader := "t=" + strconv.FormatInt(oldTimestamp, 10) + ",v1=" + validSig

	verifier := NewVerifier(SHA256, FormatStripe).WithMaxAge(5 * time.Minute)

	if verifier.Verify(payload, expiredHeader, secret) {
		t.Error("expected expired signature to be rejected")
	}

	verifierNoMaxAge := NewVerifier(SHA256, FormatStripe)
	if !verifierNoMaxAge.Verify(payload, expiredHeader, secret) {
		t.Error("expected signature without max age to be accepted")
	}
}

func TestVerifier_Verify_StripeFormat_MultipleSignatures(t *testing.T) {
	secret := "whsec_test_secret"
	payload := []byte(`{"type": "checkout.session.completed"}`)

	now := time.Now().Unix()
	signedPayload := strconv.FormatInt(now, 10) + "." + string(payload)

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signedPayload))
	validSig := hex.EncodeToString(mac.Sum(nil))

	headerWithMultipleSigs := "t=" + strconv.FormatInt(now, 10) + ",v1=invalid_sig,v1=" + validSig

	verifier := NewVerifier(SHA256, FormatStripe)

	if !verifier.Verify(payload, headerWithMultipleSigs, secret) {
		t.Error("expected one valid signature among multiple to pass")
	}
}

func TestVerifier_Sign(t *testing.T) {
	secret := "test-secret"
	payload := []byte("test payload")

	verifier256 := NewVerifier(SHA256, FormatRaw)
	sig256 := verifier256.Sign(payload, secret)

	if len(sig256) != 64 {
		t.Errorf("SHA256 signature length = %d, want 64", len(sig256))
	}

	verifier512 := NewVerifier(SHA512, FormatRaw)
	sig512 := verifier512.Sign(payload, secret)

	if len(sig512) != 128 {
		t.Errorf("SHA512 signature length = %d, want 128", len(sig512))
	}

	if sig256 == sig512 {
		t.Error("SHA256 and SHA512 signatures should be different")
	}
}

func TestVerifier_SignWithPrefix(t *testing.T) {
	secret := "test-secret"
	payload := []byte("test payload")

	verifier := NewVerifier(SHA256, FormatPrefixed)
	sig := verifier.SignWithPrefix(payload, secret)

	if len(sig) < 7 {
		t.Fatal("signature too short")
	}

	if sig[:7] != "sha256=" {
		t.Errorf("signature prefix = %s, want sha256=", sig[:7])
	}

	verifier512 := NewVerifier(SHA512, FormatPrefixed)
	sig512 := verifier512.SignWithPrefix(payload, secret)

	if sig512[:7] != "sha512=" {
		t.Errorf("signature prefix = %s, want sha512=", sig512[:7])
	}
}

func TestVerifier_WithMaxAge(t *testing.T) {
	original := NewVerifier(SHA256, FormatStripe)
	modified := original.WithMaxAge(10 * time.Minute)

	if original.maxAge != 0 {
		t.Error("original verifier should not be modified")
	}

	if modified.maxAge != 10*time.Minute {
		t.Errorf("modified verifier maxAge = %v, want %v", modified.maxAge, 10*time.Minute)
	}

	if original.algorithm != modified.algorithm {
		t.Error("algorithm should be preserved")
	}

	if original.format != modified.format {
		t.Error("format should be preserved")
	}
}

func TestVerifyGitHub(t *testing.T) {
	secret := "github-webhook-secret"
	payload := []byte(`{"action": "opened", "pull_request": {}}`)

	verifier := NewVerifier(SHA256, FormatPrefixed)
	validSig := "sha256=" + verifier.Sign(payload, secret)

	if !VerifyGitHub(payload, validSig, secret) {
		t.Error("expected valid GitHub signature to pass")
	}

	if VerifyGitHub(payload, "sha256=invalid", secret) {
		t.Error("expected invalid GitHub signature to fail")
	}
}

func TestVerifyGitLab(t *testing.T) {
	token := "gitlab-webhook-token"

	if !VerifyGitLab(token, token) {
		t.Error("expected matching tokens to pass")
	}

	if VerifyGitLab("wrong-token", token) {
		t.Error("expected mismatched tokens to fail")
	}

	if VerifyGitLab("", token) {
		t.Error("expected empty provided token to fail")
	}

	if VerifyGitLab(token, "") {
		t.Error("expected empty expected token to fail")
	}
}

func TestVerifyStripe(t *testing.T) {
	secret := "whsec_test_secret"
	payload := []byte(`{"type": "invoice.paid"}`)

	builder := NewStripeSignatureBuilder(secret)
	validHeader := builder.Build(payload)

	if !VerifyStripe(payload, validHeader, secret) {
		t.Error("expected valid Stripe signature to pass")
	}

	if VerifyStripe(payload, "t=123,v1=invalid", secret) {
		t.Error("expected invalid Stripe signature to fail")
	}
}

func TestVerifyPaddle(t *testing.T) {
	secret := "paddle-webhook-secret"
	payload := []byte(`{"event_type": "subscription.created"}`)

	verifier := NewVerifier(SHA256, FormatPrefixed)
	validSig := "sha256=" + verifier.Sign(payload, secret)

	if !VerifyPaddle(payload, validSig, secret) {
		t.Error("expected valid Paddle signature to pass")
	}
}

func TestVerifyLemonSqueezy(t *testing.T) {
	secret := "lemonsqueezy-webhook-secret"
	payload := []byte(`{"meta": {"event_name": "order_created"}}`)

	verifier := NewVerifier(SHA256, FormatPrefixed)
	validSig := "sha256=" + verifier.Sign(payload, secret)

	if !VerifyLemonSqueezy(payload, validSig, secret) {
		t.Error("expected valid LemonSqueezy signature to pass")
	}
}

func TestComputeSignature(t *testing.T) {
	secret := "test-secret"
	payload := []byte("test payload")

	sig := ComputeSignature(payload, secret)

	if len(sig) != 64 {
		t.Errorf("signature length = %d, want 64", len(sig))
	}

	verifier := NewVerifier(SHA256, FormatRaw)
	if !verifier.Verify(payload, sig, secret) {
		t.Error("computed signature should verify")
	}
}

func TestComputeSignatureWithPrefix(t *testing.T) {
	secret := "test-secret"
	payload := []byte("test payload")

	sig := ComputeSignatureWithPrefix(payload, secret)

	if len(sig) < 7 || sig[:7] != "sha256=" {
		t.Errorf("signature should start with sha256=, got %s", sig)
	}

	if !VerifyGitHub(payload, sig, secret) {
		t.Error("computed signature with prefix should verify")
	}
}

func TestStripeSignatureBuilder(t *testing.T) {
	secret := "whsec_test_secret"
	payload := []byte(`{"type": "payment_intent.succeeded"}`)

	builder := NewStripeSignatureBuilder(secret)
	header := builder.Build(payload)

	if !VerifyStripe(payload, header, secret) {
		t.Error("built Stripe signature should verify")
	}

	if len(header) < 4 || header[:2] != "t=" {
		t.Error("header should start with t=")
	}
}

func TestStripeSignatureBuilder_WithTimestamp(t *testing.T) {
	secret := "whsec_test_secret"
	payload := []byte(`{"type": "payment_intent.succeeded"}`)
	customTimestamp := int64(1609459200) // 2021-01-01 00:00:00 UTC

	builder := NewStripeSignatureBuilder(secret).WithTimestamp(customTimestamp)
	header := builder.Build(payload)

	expectedPrefix := "t=1609459200,"
	if len(header) < len(expectedPrefix) || header[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("header should start with %s, got %s", expectedPrefix, header)
	}

	verifierNoMaxAge := NewVerifier(SHA256, FormatStripe)
	if !verifierNoMaxAge.Verify(payload, header, secret) {
		t.Error("signature should verify without max age check")
	}
}

func TestPreConfiguredVerifiers(t *testing.T) {
	secret := "test-secret"
	payload := []byte("test payload")

	t.Run("GitHub", func(t *testing.T) {
		sig := "sha256=" + GitHub.Sign(payload, secret)
		if !GitHub.Verify(payload, sig, secret) {
			t.Error("GitHub verifier should work with prefixed format")
		}
	})

	t.Run("GitLab", func(t *testing.T) {
		sig := GitLab.Sign(payload, secret)
		if !GitLab.Verify(payload, sig, secret) {
			t.Error("GitLab verifier should work with raw format")
		}
	})

	t.Run("Stripe", func(t *testing.T) {
		if Stripe.maxAge != 5*time.Minute {
			t.Errorf("Stripe maxAge = %v, want 5 minutes", Stripe.maxAge)
		}
	})

	t.Run("Paddle", func(t *testing.T) {
		sig := "sha256=" + Paddle.Sign(payload, secret)
		if !Paddle.Verify(payload, sig, secret) {
			t.Error("Paddle verifier should work with prefixed format")
		}
	})

	t.Run("LemonSqueezy", func(t *testing.T) {
		sig := "sha256=" + LemonSqueezy.Sign(payload, secret)
		if !LemonSqueezy.Verify(payload, sig, secret) {
			t.Error("LemonSqueezy verifier should work with prefixed format")
		}
	})
}

func TestVerifier_SHA512(t *testing.T) {
	secret := "test-secret"
	payload := []byte("test payload")

	verifier := NewVerifier(SHA512, FormatPrefixed)
	sig := "sha512=" + verifier.Sign(payload, secret)

	if !verifier.Verify(payload, sig, secret) {
		t.Error("SHA512 verifier should work")
	}

	verifier256 := NewVerifier(SHA256, FormatPrefixed)
	sig256 := "sha256=" + verifier256.Sign(payload, secret)

	if sig == sig256 {
		t.Error("SHA512 and SHA256 signatures should be different")
	}
}

func TestVerifier_InvalidFormat(t *testing.T) {
	secret := "test-secret"
	payload := []byte("test payload")

	verifier := &Verifier{
		algorithm: SHA256,
		format:    "invalid",
	}

	if verifier.Verify(payload, "any-signature", secret) {
		t.Error("invalid format should always fail verification")
	}
}

func TestVerifier_DefaultAlgorithm(t *testing.T) {
	secret := "test-secret"
	payload := []byte("test payload")

	verifier := &Verifier{
		algorithm: "invalid",
		format:    FormatRaw,
	}

	sig := verifier.Sign(payload, secret)

	sha256Verifier := NewVerifier(SHA256, FormatRaw)
	expectedSig := sha256Verifier.Sign(payload, secret)

	if sig != expectedSig {
		t.Error("invalid algorithm should fall back to SHA256")
	}
}

func BenchmarkVerify_Prefixed(b *testing.B) {
	secret := "benchmark-secret"
	payload := []byte(`{"event": "push", "repository": "test/repo", "commits": []}`)

	verifier := NewVerifier(SHA256, FormatPrefixed)
	sig := "sha256=" + verifier.Sign(payload, secret)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier.Verify(payload, sig, secret)
	}
}

func BenchmarkVerify_Stripe(b *testing.B) {
	secret := "whsec_benchmark_secret"
	payload := []byte(`{"type": "payment_intent.succeeded", "data": {"object": {}}}`)

	builder := NewStripeSignatureBuilder(secret)
	header := builder.Build(payload)

	verifier := NewVerifier(SHA256, FormatStripe)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier.Verify(payload, header, secret)
	}
}

func BenchmarkSign(b *testing.B) {
	secret := "benchmark-secret"
	payload := []byte(`{"event": "push", "repository": "test/repo", "commits": []}`)

	verifier := NewVerifier(SHA256, FormatRaw)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier.Sign(payload, secret)
	}
}
