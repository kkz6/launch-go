package providers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

func deleteTestProviders(baseURL string) map[string]Provider {
	do := NewDigitalOceanProvider(nil)
	do.baseURL = baseURL
	het := NewHetznerProvider(nil)
	het.baseURL = baseURL
	lin := NewLinodeProvider(nil)
	lin.baseURL = baseURL
	vul := NewVultrProvider(nil)
	vul.baseURL = baseURL
	return map[string]Provider{"digitalocean": do, "hetzner": het, "linode": lin, "vultr": vul}
}

func TestCloudDeleteLegacyNumericIDs(t *testing.T) {
	for _, provider := range []struct{ name, key, path string }{
		{"digitalocean", "droplet_id", "/droplets/"},
		{"hetzner", "hetzner_id", "/servers/"},
		{"linode", "linode_id", "/linode/instances/"},
	} {
		for _, tc := range []struct{ name, raw string }{
			{"legacy numeric ID", `{"%s":56516756}`},
			{"current string ID", `{"%s":"56516756"}`},
			{"primary ID wins", `{"%s":56516756,"provider_server_id":"999"}`},
			{"fallback string ID", `{"provider_server_id":"56516756","unused":"%s"}`},
			{"fallback numeric ID", `{"provider_server_id":56516756,"unused":"%s"}`},
		} {
			t.Run(provider.name+"/"+tc.name, func(t *testing.T) {
				// Laravel stored cloud API IDs as JSON numbers; exercise the same
				// database scan as a migrated server record, without rewriting it.
				var data dbtype.JSONMap
				require.NoError(t, data.Scan(fmt.Sprintf(tc.raw, provider.key)))
				var paths []string
				api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, http.MethodDelete, r.Method)
					paths = append(paths, r.URL.Path)
					w.WriteHeader(http.StatusNoContent)
				}))
				defer api.Close()
				err := deleteTestProviders(api.URL)[provider.name].Delete(context.Background(), &models.Server{ProviderData: data}, map[string]any{"token": "test-token"})
				require.NoError(t, err)
				require.Equal(t, []string{provider.path + "56516756"}, paths)
			})
		}
	}
}

func TestCloudDeleteRejectsInvalidPrimaryIDWithoutFallback(t *testing.T) {
	for _, provider := range []struct{ name, key string }{
		{"digitalocean", "droplet_id"},
		{"hetzner", "hetzner_id"},
		{"linode", "linode_id"},
	} {
		t.Run(provider.name, func(t *testing.T) {
			requests := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests++
				w.WriteHeader(http.StatusNoContent)
			}))
			defer api.Close()
			for _, invalid := range []any{nil, "", float64(0), float64(-1), 123.5, float64(1 << 53), math.NaN(), math.Inf(1), true, map[string]any{"id": 123}} {
				data := dbtype.JSONMap{provider.key: invalid, "provider_server_id": "999"}
				err := deleteTestProviders(api.URL)[provider.name].Delete(context.Background(), &models.Server{ProviderData: data}, map[string]any{"token": "test-token"})
				require.ErrorContains(t, err, "instance ID is missing")
			}
			require.Zero(t, requests, "an invalid primary ID must not target the fallback resource")
		})
	}
}

func TestCloudDeleteRequiresProviderSuccess(t *testing.T) {
	for _, name := range []string{"digitalocean", "hetzner", "linode", "vultr"} {
		for _, status := range []int{http.StatusNoContent, http.StatusNotFound, http.StatusUnauthorized, http.StatusForbidden} {
			t.Run(name+http.StatusText(status), func(t *testing.T) {
				var paths []string
				api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, http.MethodDelete, r.Method)
					paths = append(paths, r.URL.Path)
					w.WriteHeader(status)
					if status != http.StatusNoContent {
						_, _ = w.Write([]byte(`{"error":"secret-provider-token"}`))
					}
				}))
				defer api.Close()
				server := &models.Server{ProviderData: dbtype.JSONMap{"provider_server_id": "instance-1", "ssh_key_id": "key-1"}}
				err := deleteTestProviders(api.URL)[name].Delete(context.Background(), server, map[string]any{"token": "test-token"})
				if status == http.StatusUnauthorized || status == http.StatusForbidden {
					require.Error(t, err)
					var failure *DeletionHTTPError
					require.ErrorAs(t, err, &failure)
					require.Equal(t, status, failure.StatusCode)
					require.Nil(t, errors.Unwrap(failure), "the raw provider body must not survive in the error chain")
					require.NotContains(t, err.Error(), "secret-provider-token")
					require.Len(t, paths, 1, "key cleanup must not run before VM deletion succeeds")
				} else {
					require.NoError(t, err)
					require.NotEmpty(t, paths)
				}
			})
		}
	}
}

func TestCloudDeleteMissingIDAndAWSFailClosed(t *testing.T) {
	for name, provider := range deleteTestProviders("http://127.0.0.1:1") {
		t.Run(name, func(t *testing.T) {
			for _, data := range []dbtype.JSONMap{nil, {"ssh_key_id": "key-only"}} {
				err := provider.Delete(context.Background(), &models.Server{ProviderData: data}, map[string]any{"token": "test-token"})
				require.ErrorContains(t, err, "instance ID is missing")
			}
		})
	}
	require.ErrorContains(t, NewAWSProvider(nil).Delete(context.Background(), &models.Server{}, nil), "not supported")
}

func TestCloudDeleteTransportFailureAndSecondaryCleanup(t *testing.T) {
	for _, name := range []string{"digitalocean", "hetzner", "linode", "vultr"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := deleteTestProviders("http://127.0.0.1:1")[name].Delete(ctx, &models.Server{ProviderData: dbtype.JSONMap{"provider_server_id": "instance-1"}}, map[string]any{"token": "test-token"})
			require.Error(t, err)
		})
	}
	for _, name := range []string{"hetzner", "vultr"} {
		t.Run(name+"key cleanup", func(t *testing.T) {
			var paths []string
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				if len(paths) == 1 {
					w.WriteHeader(http.StatusNoContent)
				} else {
					w.WriteHeader(http.StatusForbidden)
				}
			}))
			defer api.Close()
			err := deleteTestProviders(api.URL)[name].Delete(context.Background(), &models.Server{ProviderData: dbtype.JSONMap{"provider_server_id": "instance-1", "ssh_key_id": "key-1"}}, map[string]any{"token": "test-token"})
			require.NoError(t, err)
			require.Len(t, paths, 2)
		})
	}
}
