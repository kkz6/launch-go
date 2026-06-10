package providers

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"

	"github.com/rs/zerolog"
)

func TestPolarClientImplementsProvider(t *testing.T) {
	// Compile-time guarantee, asserted here for documentation.
	var _ BillingProvider = (*PolarClient)(nil)
}

func TestNewPolarClient_SelectsEnvironment(t *testing.T) {
	logger := zerolog.Nop()

	for _, tc := range []struct {
		name    string
		sandbox bool
	}{
		{"sandbox", true},
		{"production", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewPolarClient(&PolarConfig{AccessToken: "tok", Sandbox: tc.sandbox}, &logger)
			if c == nil || c.Client() == nil {
				t.Fatal("expected a non-nil Polar client")
			}
		})
	}
}

func TestValidateWebhook_NoSecret(t *testing.T) {
	logger := zerolog.Nop()
	c := NewPolarClient(&PolarConfig{AccessToken: "tok"}, &logger)

	if err := c.ValidateWebhook([]byte("{}"), http.Header{}); err == nil {
		t.Fatal("expected an error when no webhook secret is configured")
	}
}

// TestValidateWebhook_PolarPlainSecret guards the regression where Polar's
// plain "polar_whs_…" secret (not base64) must verify. Polar's effective HMAC
// key is the raw secret-string bytes, so we sign the way Polar does (raw key)
// and confirm ValidateWebhook accepts it. The old NewWebhook path would have
// base64-decoded the secret and rejected everything.
func TestValidateWebhook_PolarPlainSecret(t *testing.T) {
	logger := zerolog.Nop()
	secret := "polar_whs_SGx7H4fJSDjPy80y7oZFyJBoJK2uKn24SQjU10UdMJK"
	c := NewPolarClient(&PolarConfig{AccessToken: "tok", WebhookSecret: secret}, &logger)

	payload := []byte(`{"type":"subscription.active","data":{}}`)
	msgID := "msg_test"
	ts := time.Now()

	// Sign exactly how Polar does: raw secret-string bytes as the HMAC key.
	signer, err := standardwebhooks.NewWebhookRaw([]byte(secret))
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	sig, err := signer.Sign(msgID, ts, payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	headers := http.Header{}
	headers.Set("webhook-id", msgID)
	headers.Set("webhook-timestamp", strconv.FormatInt(ts.Unix(), 10))
	headers.Set("webhook-signature", sig)

	if err := c.ValidateWebhook(payload, headers); err != nil {
		t.Fatalf("expected a Polar-signed webhook to verify, got: %v", err)
	}

	// Document the bug we fixed: the base64 path can't even construct a verifier
	// from Polar's secret (the "_" isn't valid base64).
	if _, err := standardwebhooks.NewWebhook(secret); err == nil {
		t.Fatal("expected NewWebhook to reject the plain polar_whs_ secret")
	}
}
