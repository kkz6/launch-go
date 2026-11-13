package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/config"
)

func TestVultrProvisioningSendsNumericImageID(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/instances", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		var payload struct {
			OSID int `json:"os_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); !assert.NoError(t, err) {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		assert.Equal(t, 2284, payload.OSID)
		fmt.Fprint(w, `{"instance":{"id":"test-instance"}}`)
	}))
	defer api.Close()
	provider := NewVultrProvider(nil)
	provider.baseURL = api.URL
	result, err := provider.createInstance(context.Background(), provider.NewClient("test-token"), "test", "ewr", "vc2-1c-1gb", 2284, "test-key")
	require.NoError(t, err)
	require.Equal(t, "test-instance", result.ID)
}

func TestVultrImageValidationUsesOSCatalog(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/os", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("cursor") {
		case "":
			fmt.Fprint(w, `{"os":[{"id":1743}],"meta":{"links":{"next":"next+/="}}}`)
		case "next+/=":
			fmt.Fprint(w, `{"os":[{"id":2284}],"meta":{"links":{"next":""}}}`)
		default:
			t.Errorf("unexpected cursor: %q", r.URL.Query().Get("cursor"))
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	provider := NewVultrProvider(nil)
	provider.baseURL = api.URL
	ctx := context.Background()
	creds := map[string]any{"token": "test-token"}
	require.Empty(t, ValidateImages(ctx, provider, config.GetProviderConfigs()["vultr"], creds))
	require.ErrorContains(t, provider.LookupImage(ctx, creds, "12345"), "not found in Vultr catalog")
}

func TestLinodeImageValidationRejectsDeprecatedImage(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/images/linode/ubuntu22.04", r.URL.Path)
		fmt.Fprint(w, `{"id":"linode/ubuntu22.04","status":"available","deprecated":true}`)
	}))
	defer api.Close()
	provider := NewLinodeProvider(nil)
	provider.baseURL = api.URL
	require.ErrorContains(t, provider.LookupImage(context.Background(), map[string]any{"token": "test-token"}, "linode/ubuntu22.04"), "deprecated")
}

func TestHetznerImageValidationChecksBothArchitectures(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/images", r.URL.Path)
		assert.Equal(t, "system", r.URL.Query().Get("type"))
		name := r.URL.Query().Get("name")
		deprecation := "null"
		if name == "ubuntu-22.04" {
			deprecation = `{"announced":"2026-01-01T00:00:00Z","unavailable_after":"2027-01-01T00:00:00Z"}`
		}
		fmt.Fprintf(w, `{"images":[{"name":%q,"architecture":"x86","status":"available","deprecation":null},{"name":%q,"architecture":"arm","status":"available","deprecation":%s}]}`, name, name, deprecation)
	}))
	defer api.Close()
	provider := NewHetznerProvider(nil)
	provider.baseURL = api.URL
	ctx := context.Background()
	creds := map[string]any{"token": "test-token"}
	require.NoError(t, provider.LookupImage(ctx, creds, "ubuntu-24.04"))
	require.ErrorContains(t, provider.LookupImage(ctx, creds, "ubuntu-22.04"), "for arm")
}

type recordingImageProvider struct {
	Provider
	checks []string
}

func (p *recordingImageProvider) LookupImage(_ context.Context, credentials map[string]any, image string) error {
	p.checks = append(p.checks, GetStringField(credentials, "region", "")+":"+image)
	return nil
}

func TestAWSImageValidationUsesCanonicalParameters(t *testing.T) {
	p := &recordingImageProvider{Provider: NewAWSProvider(nil)}
	creds := map[string]any{"region": "eu-west-1"}
	require.Empty(t, ValidateImages(context.Background(), p, config.GetProviderConfigs()["aws"], creds))
	require.ElementsMatch(t, []string{
		"eu-west-1:/aws/service/canonical/ubuntu/server/22.04/stable/current/amd64/hvm/ebs-gp2/ami-id",
		"eu-west-1:/aws/service/canonical/ubuntu/server/24.04/stable/current/amd64/hvm/ebs-gp3/ami-id",
	}, p.checks)
	result, err := p.Create(context.Background(), nil, nil)
	require.Nil(t, result)
	require.ErrorContains(t, err, "provisioning is not supported")
}
