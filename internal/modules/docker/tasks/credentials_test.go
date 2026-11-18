package tasks

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentDeploymentCredentialConfigsStayIsolated(t *testing.T) {
	root := t.TempDir()
	ambient := filepath.Join(root, "ambient")
	require.NoError(t, os.MkdirAll(filepath.Join(ambient, ".docker"), 0700))
	ambientPath := filepath.Join(ambient, ".docker", "config.json")
	require.NoError(t, os.WriteFile(ambientPath, []byte("ambient unchanged"), 0600))
	dirs := []string{filepath.Join(root, "first"), filepath.Join(root, "second")}
	tokens := []string{"first-private-bearer", "second-private-bearer"}
	for i, dir := range dirs {
		require.NoError(t, os.MkdirAll(dir, 0700))
		files := deploymentSecretFiles("https://x-access-token:git-secret@github.com/acme/repo.git\n", []ComposeRegistryLogin{{RegistryURL: "ghcr.io", Username: "oauth2", Password: tokens[i]}})
		for name, value := range files {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(value), 0600))
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, dir := range dirs {
		wg.Add(1)
		go func(dir string) {
			defer wg.Done()
			cmd := exec.Command("bash", "-c", isolatedCredentialsScript+"\ntest \"$GIT_CONFIG_VALUE_1\" = true\n")
			cmd.Env = append(os.Environ(), "HOME="+ambient, "LAUNCH_SECRET_DIR="+dir)
			errs <- cmd.Run()
		}(dir)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	for i, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(dir, "docker", "config.json"))
		require.NoError(t, err)
		require.Contains(t, string(data), tokens[i])
		require.NotContains(t, string(data), tokens[1-i])
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(data, &parsed))
		info, err := os.Stat(filepath.Join(dir, "docker", "config.json"))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
	data, err := os.ReadFile(ambientPath)
	require.NoError(t, err)
	require.Equal(t, "ambient unchanged", string(data))
}

func TestGitCredentialsNeverEnterStoredScript(t *testing.T) {
	cfg := DeployConfig{GitCredentials: "https://x-access-token:private-git-token@github.com/acme/repo.git\n", GitRepo: "https://github.com/acme/repo.git"}
	task := DeployApplication(cfg)
	require.NotContains(t, task.Script(), "private-git-token")
	require.Contains(t, task.(interface{ SecretFiles() map[string]string }).SecretFiles()["git-credentials"], "private-git-token")
}

func TestGitCredentialHelperMatchesOnlyExplicitRepositoryPaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for credential matching integration")
	}
	dir := t.TempDir()
	// An existing checkout may retain a helper from an earlier deployment.
	// It must never override this execution's explicit repository grants.
	require.NoError(t, exec.Command("git", "init", "--quiet", dir).Run())
	ambient := dir + "/ambient-credentials"
	require.NoError(t, os.WriteFile(ambient, []byte("https://x-access-token:another-account-token@github.com\n"), 0600))
	require.NoError(t, exec.Command("git", "-C", dir, "config", "credential.helper", "store --file="+ambient).Run())
	files := deploymentSecretFiles("https://x-access-token:selected-token@github.com/acme/selected.git\nhttps://x-access-token:dependency-token@github.com/acme/dependency.git\n", nil)
	for key, value := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, key), []byte(value), 0600))
	}
	for _, test := range []struct {
		path, token string
		allowed     bool
	}{{"acme/selected.git", "selected-token", true}, {"acme/dependency.git", "dependency-token", true}, {"acme/unselected.git", "", false}, {"acme/selected.git/other", "", false}} {
		t.Run(test.path, func(t *testing.T) {
			cmd := exec.Command("bash", "-c", isolatedCredentialsScript+"\ngit credential fill\n")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "LAUNCH_SECRET_DIR="+dir)
			cmd.Stdin = strings.NewReader("protocol=https\nhost=github.com\npath=" + test.path + "\n\n")
			output, err := cmd.CombinedOutput()
			require.NotContains(t, string(output), "another-account-token")
			if test.allowed {
				require.NoError(t, err)
				require.Contains(t, string(output), "password="+test.token)
			} else {
				require.Error(t, err)
				require.NotContains(t, string(output), "selected-token")
				require.NotContains(t, string(output), "dependency-token")
			}
		})
	}
}

func TestGitTransportsCannotUseAmbientSSHOrLocalRepositories(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for transport isolation integration")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docker-config"), []byte(`{"auths":{}}`), 0600))
	for _, repository := range []string{"ssh://github-alias/org/private.git", "file:///srv/other-repository", "ext::sh -c true"} {
		cmd := exec.Command("bash", "-c", isolatedCredentialsScript+"\ngit ls-remote \"$1\"\n", "bash", repository)
		cmd.Env = append(os.Environ(), "LAUNCH_SECRET_DIR="+dir, "GIT_SSH_COMMAND=echo ambient-ssh-must-not-run")
		output, err := cmd.CombinedOutput()
		require.Error(t, err)
		require.Contains(t, string(output), "not allowed")
		require.NotContains(t, string(output), "ambient-ssh-must-not-run")
	}
}

func TestGitHTTPUsesOnlySelectedCredentialsAndIgnoresNetrc(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for HTTP credential isolation integration")
	}
	var mu sync.Mutex
	seen := map[string]bool{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, password, _ := r.BasicAuth()
		if password != "" {
			mu.Lock()
			seen[password] = true
			mu.Unlock()
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="repositories"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".netrc"), []byte("machine "+u.Hostname()+" login ambient password ambient-token\n"), 0600))
	credential := "https://x-access-token:selected-token@" + u.Host + "/selected.git\n"
	for key, value := range deploymentSecretFiles(credential, nil) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, key), []byte(value), 0600))
	}
	run := func(path string, options ...string) {
		args := ""
		if len(options) > 0 {
			args = "-c http.emptyAuth=false "
		}
		cmd := exec.Command("bash", "-c", isolatedCredentialsScript+"\ngit "+args+"ls-remote \"$1\"\n", "bash", server.URL+path)
		cmd.Env = append(os.Environ(), "HOME="+dir, "LAUNCH_SECRET_DIR="+dir, "GIT_SSL_NO_VERIFY=true")
		_, err := cmd.CombinedOutput()
		require.Error(t, err, "the fixture returns no repository content")
	}
	// Establish that this Git/libcurl actually reads the ambient netrc.
	run("/unselected.git", "baseline")
	mu.Lock()
	require.True(t, seen["ambient-token"])
	clear(seen)
	mu.Unlock()
	run("/unselected.git")
	run("/selected.git")
	mu.Lock()
	defer mu.Unlock()
	require.False(t, seen["ambient-token"], "ambient netrc must not authorize an unselected repository")
	require.True(t, seen["selected-token"], "selected repository helper must still authenticate")
}
