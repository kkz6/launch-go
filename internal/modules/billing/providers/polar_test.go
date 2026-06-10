package providers

import (
	"net/http"
	"testing"

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
