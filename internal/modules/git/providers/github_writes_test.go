package providers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/nacl/box"
)

// fakeGitHub stands in for api.github.com so the write helpers can be
// exercised without a real installation. The struct tracks the requests
// it received so assertions can be specific (right URL, right method,
// right body shape) without relying on the test server printing logs.
type fakeGitHub struct {
	t   *testing.T
	srv *httptest.Server

	// repoPub / repoPriv form the libsodium keypair the fake's
	// /actions/secrets/public-key endpoint hands out. Tests use
	// repoPriv to decrypt the sealed box they receive on the
	// PutActionsSecret round-trip and assert the original plaintext.
	repoPub  *[32]byte
	repoPriv *[32]byte

	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct {
	Method string
	Path   string
	Auth   string
	Body   string
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	pub, priv, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)

	f := &fakeGitHub{t: t, repoPub: pub, repoPriv: priv}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	return f
}

func (f *fakeGitHub) Close() { f.srv.Close() }

func (f *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()
	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Auth:   r.Header.Get("Authorization"),
		Body:   string(body),
	})
	f.mu.Unlock()

	// WriteHeader MUST be called before Write/Encode — calling Encode
	// first commits an implicit 200 and any subsequent WriteHeader is
	// a no-op with a "superfluous WriteHeader" log line. The token
	// endpoint specifically needs to return 201 so GetInstallationToken
	// is happy; the wrong-order bug meant every test got 200 and
	// bailed out at the JWT step.
	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/access_tokens"):
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "ghs_fake_installation_token"})
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/actions/secrets/public-key"):
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"key_id": "fake-key-id-12345",
			"key":    base64.StdEncoding.EncodeToString(f.repoPub[:]),
		})
	case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/actions/secrets/"):
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/actions/variables"):
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/actions/variables/"):
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/"):
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit": map[string]any{"sha": "abc1234567890fakecommitsha"},
		})
	case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/actions/secrets/"):
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/actions/variables/"):
		w.WriteHeader(http.StatusNoContent)
	default:
		f.t.Logf("fakeGitHub: unexpected %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

// testProvider wires a GitHubProvider against the fake server so
// PutContents/PutActionsSecret/etc. can be exercised end-to-end
// without touching api.github.com.
func testProvider(t *testing.T, baseURL string) *GitHubProvider {
	t.Helper()
	// Generate an ephemeral RSA private key so generateJWT works.
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	})
	cfg := &ProviderConfig{
		AppID:      "12345",
		PrivateKey: string(pemBytes),
	}
	// Build the base provider, then point its httpclient at the fake.
	base := NewBaseGitProvider(
		cfg,
		WithProviderType(GitProviderGitHub),
		WithBaseURL(baseURL),
		WithAPIURL(baseURL),
	)
	return &GitHubProvider{BaseGitProvider: base}
}

func TestSealActionsSecret_RoundTrip(t *testing.T) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)

	plaintext := "deadbeef-deploy-token-1234"
	enc, err := sealActionsSecret(pub[:], plaintext)
	require.NoError(t, err)

	// Decode + open with the corresponding private key — proves the
	// output is wire-compatible with libsodium's sealed_box_open,
	// which is what GitHub Actions does internally.
	raw, err := base64.StdEncoding.DecodeString(enc)
	require.NoError(t, err)
	opened, ok := box.OpenAnonymous(nil, raw, pub, priv)
	require.True(t, ok, "sealed box must open with the matching private key")
	assert.Equal(t, plaintext, string(opened))
}

func TestSealActionsSecret_RejectsWrongLengthKey(t *testing.T) {
	_, err := sealActionsSecret([]byte("too short"), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestDecodeRepoPublicKey_RoundTrip(t *testing.T) {
	pub, _, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)
	b64 := base64.StdEncoding.EncodeToString(pub[:])

	got, err := decodeRepoPublicKey(b64)
	require.NoError(t, err)
	assert.Equal(t, pub[:], got)
}

func TestPutContents_SendsBase64BodyAndReturnsCommitSHA(t *testing.T) {
	fake := newFakeGitHub(t)
	defer fake.Close()
	p := testProvider(t, fake.srv.URL)

	sha, err := p.PutContents(
		context.Background(),
		"100", // installationID
		"kkz6", "test-repo",
		".github/workflows/launch-deploy.yml",
		"name: Launch Deploy\n",
		"Configure Launch deploy workflow",
		"", // no existingSHA — create path
		"main",
	)
	require.NoError(t, err)
	assert.Equal(t, "abc1234567890fakecommitsha", sha)

	// The contents PUT body must carry base64-encoded content + the
	// commit message + the branch. The token mint hits first so the
	// PUT is requests[1].
	require.GreaterOrEqual(t, len(fake.requests), 2)
	contents := fake.requests[len(fake.requests)-1]
	assert.Equal(t, http.MethodPut, contents.Method)
	assert.Equal(t, "/repos/kkz6/test-repo/contents/.github/workflows/launch-deploy.yml", contents.Path)
	assert.Equal(t, "Bearer ghs_fake_installation_token", contents.Auth)
	assert.Contains(t, contents.Body, base64.StdEncoding.EncodeToString([]byte("name: Launch Deploy\n")))
	assert.Contains(t, contents.Body, `"branch":"main"`)
}

func TestPutActionsSecret_EncryptsValueAndPosts(t *testing.T) {
	fake := newFakeGitHub(t)
	defer fake.Close()
	p := testProvider(t, fake.srv.URL)

	require.NoError(t, p.PutActionsSecret(
		context.Background(),
		"100", "kkz6", "test-repo",
		"LAUNCH_DEPLOY_TOKEN",
		"super-secret-token-value",
	))

	// Find the PUT request to the secret endpoint.
	var put *recordedRequest
	for i := range fake.requests {
		if fake.requests[i].Method == http.MethodPut && strings.Contains(fake.requests[i].Path, "/actions/secrets/LAUNCH_DEPLOY_TOKEN") {
			put = &fake.requests[i]
			break
		}
	}
	require.NotNil(t, put, "expected a PUT /actions/secrets/LAUNCH_DEPLOY_TOKEN")

	// Decrypt the encrypted_value the helper sent and confirm it
	// round-trips back to the plaintext we passed in. This is the
	// invariant that matters: the helper produced something Actions
	// can decrypt to the original value.
	var body struct {
		EncryptedValue string `json:"encrypted_value"`
		KeyID          string `json:"key_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(put.Body), &body))
	assert.Equal(t, "fake-key-id-12345", body.KeyID)

	raw, err := base64.StdEncoding.DecodeString(body.EncryptedValue)
	require.NoError(t, err)
	opened, ok := box.OpenAnonymous(nil, raw, fake.repoPub, fake.repoPriv)
	require.True(t, ok, "encrypted value must decrypt with the fake's private key")
	assert.Equal(t, "super-secret-token-value", string(opened))
}

func TestPutActionsVariable_PostsCreate(t *testing.T) {
	fake := newFakeGitHub(t)
	defer fake.Close()
	p := testProvider(t, fake.srv.URL)

	require.NoError(t, p.PutActionsVariable(
		context.Background(),
		"100", "kkz6", "test-repo",
		"LAUNCH_APP_ID", "01HJXVHGRGTQRX4P0G3Y8R6CK7",
	))

	// First non-token request should be the POST create.
	var seen bool
	for _, r := range fake.requests {
		if r.Method == http.MethodPost && strings.HasSuffix(r.Path, "/actions/variables") {
			assert.Contains(t, r.Body, `"name":"LAUNCH_APP_ID"`)
			assert.Contains(t, r.Body, `"value":"01HJXVHGRGTQRX4P0G3Y8R6CK7"`)
			seen = true
			break
		}
	}
	assert.True(t, seen, "expected a POST /actions/variables for create")
}

// putActionsVariable_FallsBackToPatchOnConflict — exercise the
// already-exists -> PATCH fallback path. We swap the fake's create
// handler to return 422 so the helper has to fall through to PATCH.
func TestPutActionsVariable_FallsBackToPatchOnConflict(t *testing.T) {
	fake := newFakeGitHub(t)
	defer fake.Close()

	// Override the fake's handler to force a 422 on POST.
	orig := fake.srv.Config.Handler
	fake.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/actions/variables") {
			fake.mu.Lock()
			body, _ := io.ReadAll(r.Body)
			fake.requests = append(fake.requests, recordedRequest{
				Method: r.Method, Path: r.URL.Path,
				Auth: r.Header.Get("Authorization"),
				Body: string(body),
			})
			fake.mu.Unlock()
			http.Error(w, `{"message":"already exists"}`, http.StatusUnprocessableEntity)
			return
		}
		orig.ServeHTTP(w, r)
	})

	p := testProvider(t, fake.srv.URL)
	require.NoError(t, p.PutActionsVariable(
		context.Background(),
		"100", "kkz6", "test-repo",
		"LAUNCH_APP_ID", "updated-value",
	))

	// Both POST (422) and PATCH (204) should have happened.
	var sawPost, sawPatch bool
	for _, r := range fake.requests {
		if r.Method == http.MethodPost && strings.HasSuffix(r.Path, "/actions/variables") {
			sawPost = true
		}
		if r.Method == http.MethodPatch && strings.Contains(r.Path, "/actions/variables/LAUNCH_APP_ID") {
			sawPatch = true
		}
	}
	assert.True(t, sawPost, "POST create should have been attempted")
	assert.True(t, sawPatch, "PATCH update should have run after the 422")
}

func TestDeleteActionsSecret_TreatsNotFoundAsSuccess(t *testing.T) {
	fake := newFakeGitHub(t)
	defer fake.Close()

	// Override DELETE handler to 404.
	orig := fake.srv.Config.Handler
	fake.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/actions/secrets/") {
			fake.mu.Lock()
			fake.requests = append(fake.requests, recordedRequest{Method: r.Method, Path: r.URL.Path})
			fake.mu.Unlock()
			w.WriteHeader(http.StatusNotFound)
			return
		}
		orig.ServeHTTP(w, r)
	})

	p := testProvider(t, fake.srv.URL)
	err := p.DeleteActionsSecret(context.Background(), "100", "kkz6", "test-repo", "LAUNCH_DEPLOY_TOKEN")
	assert.NoError(t, err, "404 on DELETE must be treated as idempotent success")
}

// guard: this file uses fmt only via tests calling Sprintf in the
// fake — keep importing it so a future refactor doesn't drop it.
var _ = fmt.Sprintf
