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

	// existingContentSHA controls the GET /contents/ response used by the
	// PutContents upsert path: non-empty → 200 with that blob sha (file
	// exists), empty → 404 (file absent).
	existingContentSHA string

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
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/"):
		// PutContents resolves the current blob sha here when no SHA was
		// supplied. Empty existingContentSHA == file absent (404).
		if f.existingContentSHA == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"sha": f.existingContentSHA, "type": "file"})
	case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/"):
		// Mirror the real GitHub response shape — BOTH content.sha
		// (file blob) and commit.sha (the commit). The provider must
		// return content.sha; returning commit.sha caused every
		// subsequent sync to 409 with "file modified since last
		// sync" — locked in by TestPutContents_ReturnsBlobSHA.
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": map[string]any{"sha": "fakeBlobSha000000000000000000000000000000"},
			"commit":  map[string]any{"sha": "abc1234567890fakecommitsha"},
		})
	case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/actions/secrets/"):
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/actions/variables/"):
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/jobs"):
		// GET /repos/{o}/{r}/actions/runs/{id}/jobs — used by the live
		// step timeline (ListWorkflowRunJobs).
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_count": 1,
			"jobs": []map[string]any{{
				"id":         99,
				"name":       "build-and-deploy",
				"status":     "in_progress",
				"conclusion": nil,
				"html_url":   "https://github.com/o/r/actions/runs/5/jobs/99",
				"steps": []map[string]any{
					{"name": "Set up job", "status": "completed", "conclusion": "success", "number": 1},
					{"name": "Build (Dockerfile)", "status": "in_progress", "conclusion": nil, "number": 4},
				},
			}},
		})
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/actions/workflows/") && strings.HasSuffix(r.URL.Path, "/runs"):
		// GET /repos/{o}/{r}/actions/workflows/{file}/runs — run discovery
		// (FindLatestWorkflowRun).
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_count": 1,
			"workflow_runs": []map[string]any{{
				"id":          12345,
				"status":      "in_progress",
				"conclusion":  nil,
				"html_url":    "https://github.com/o/r/actions/runs/12345",
				"created_at":  "2026-06-09T00:00:00Z",
				"event":       "workflow_dispatch",
				"head_branch": "main",
			}},
		})
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

func TestPutContents_SendsBase64BodyAndReturnsBlobSHA(t *testing.T) {
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
	// Must return content.sha (the FILE BLOB sha), not commit.sha.
	// PUT contents uses the blob sha for If-Match on subsequent
	// updates — returning the commit sha caused every re-sync to
	// 409 with "file modified since last sync". See the regression
	// test below for the failure mode.
	assert.Equal(t, "fakeBlobSha000000000000000000000000000000", sha)
	assert.NotEqual(t, "abc1234567890fakecommitsha", sha,
		"returning the commit sha breaks subsequent syncs — see PutContents docs")

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

// Regression test for the "every re-sync 409s" bug. The provider
// previously returned commit.sha; round-tripping that as `existingSHA`
// on the next PutContents would 409 because GitHub compared it to the
// stored blob SHA. The fix returns content.sha; this test pins that
// behavior even if the fake's response shape drifts.
func TestPutContents_DoesNotReturnCommitSHA(t *testing.T) {
	fake := newFakeGitHub(t)
	defer fake.Close()
	p := testProvider(t, fake.srv.URL)

	sha, err := p.PutContents(
		context.Background(),
		"100", "kkz6", "test-repo",
		"some-file.txt", "hello", "commit msg", "", "main",
	)
	require.NoError(t, err)
	// content.sha and commit.sha are intentionally different in the
	// fake's response. Returning the commit sha is the bug; we
	// assert NotEqual rather than just Equal-on-blob so the failure
	// mode is named in the assertion message when this regresses.
	assert.NotEqual(t, "abc1234567890fakecommitsha", sha,
		"PutContents must return content.sha (blob), not commit.sha "+
			"— otherwise every subsequent update 409s on If-Match")
	assert.Equal(t, "fakeBlobSha000000000000000000000000000000", sha)
}

// Regression for #80: when no SHA is supplied but the file already exists
// (e.g. several apps share one repo + workflow path), PutContents must resolve
// the current blob SHA and update — not blindly create, which GitHub rejects
// with 422 "sha wasn't supplied".
func TestPutContents_UpsertsWhenFileExistsWithoutSHA(t *testing.T) {
	fake := newFakeGitHub(t)
	defer fake.Close()
	fake.existingContentSHA = "existingBlobSha1111111111111111111111111"
	p := testProvider(t, fake.srv.URL)

	_, err := p.PutContents(
		context.Background(),
		"100", "kkz6", "shared-repo",
		".github/workflows/launch-deploy.yml",
		"name: Launch Deploy\n",
		"Configure Launch deploy workflow",
		"", // no stored SHA, but the file exists on the repo
		"main",
	)
	require.NoError(t, err)

	// A GET must precede the PUT, and the PUT body must carry the resolved sha.
	var sawGet bool
	var put recordedRequest
	for _, r := range fake.requests {
		if strings.Contains(r.Path, "/contents/") {
			if r.Method == http.MethodGet {
				sawGet = true
			}
			if r.Method == http.MethodPut {
				put = r
			}
		}
	}
	assert.True(t, sawGet, "PutContents must GET the existing file SHA when none supplied")
	assert.Equal(t, http.MethodPut, put.Method)
	assert.Contains(t, put.Body, `"sha":"existingBlobSha1111111111111111111111111"`,
		"PUT must update with the resolved blob sha, not create")
}

// When the file does not exist, PutContents creates it — the PUT body carries
// NO sha field.
func TestPutContents_CreatesWhenFileAbsent(t *testing.T) {
	fake := newFakeGitHub(t)
	defer fake.Close()
	// existingContentSHA left empty → GET returns 404 → create path.
	p := testProvider(t, fake.srv.URL)

	_, err := p.PutContents(
		context.Background(),
		"100", "kkz6", "new-repo",
		".github/workflows/launch-deploy.yml",
		"name: Launch Deploy\n", "msg", "", "main",
	)
	require.NoError(t, err)

	put := fake.requests[len(fake.requests)-1]
	assert.Equal(t, http.MethodPut, put.Method)
	assert.NotContains(t, put.Body, `"sha"`, "absent file must be created without a sha")
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
