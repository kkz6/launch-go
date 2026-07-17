package jobs

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

func githubJobSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestProcessGitWebhookHandleRejectsInvalidInput(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name      string
		payload   ProcessGitWebhookPayload
		configure bool
		contains  string
	}{
		{name: "invalid provider", payload: ProcessGitWebhookPayload{Provider: "unknown"}, contains: "invalid provider"},
		{name: "provider not configured", payload: ProcessGitWebhookPayload{Provider: "github"}, contains: "failed to get provider"},
		{name: "invalid signature", configure: true, payload: ProcessGitWebhookPayload{Provider: "github", Payload: `{}`, Signature: "invalid"}, contains: "invalid webhook signature"},
		{name: "invalid JSON", configure: true, payload: ProcessGitWebhookPayload{Provider: "github", Payload: `{`, Signature: githubJobSignature([]byte(`{`), "secret")}, contains: "failed to parse payload"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := providers.NewProviderFactory()
			if tt.configure {
				factory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{WebhookSecret: "secret"})
			}
			job := &ProcessGitWebhookJob{
				Deps:    &JobDeps{Deps: &pkgjobs.Deps{Logger: &logger}, ProviderFactory: factory},
				Payload: tt.payload,
			}

			err := job.Handle(context.Background())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.contains)
		})
	}
}

func TestProcessGitWebhookHandleAcceptsValidNoopPayload(t *testing.T) {
	const payload = `{}`
	logger := zerolog.Nop()
	factory := providers.NewProviderFactory()
	factory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{WebhookSecret: "secret"})
	job := &ProcessGitWebhookJob{
		Deps: &JobDeps{Deps: &pkgjobs.Deps{Logger: &logger}, ProviderFactory: factory},
		Payload: ProcessGitWebhookPayload{
			Provider:  "github",
			Payload:   payload,
			Signature: githubJobSignature([]byte(payload), "secret"),
		},
	}

	require.NoError(t, job.Handle(context.Background()))
}
