package models

import (
	"net/url"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

func TestCustomServerProvisionCommandURLs(t *testing.T) {
	previous := signedurl.GetDefaultSigner()
	signer := signedurl.NewSigner("model-signing-test").WithBaseURL("https://app.example.test")
	signedurl.SetDefaultSigner(signer)
	t.Cleanup(func() { signedurl.SetDefaultSigner(previous) })
	server := &Server{}
	server.ID = "test-server"
	for _, tt := range []struct {
		command   string
		automatic bool
	}{
		{server.GetProvisionCommand(), false},
		{server.GetAutomaticProvisionCommand(), true},
	} {
		require.Contains(t, tt.command, "wget -qO-")
		require.NotContains(t, tt.command, "\n")
		link, err := url.Parse(strings.Split(tt.command, "'")[1])
		require.NoError(t, err)
		require.Equal(t, "app.example.test", link.Host)
		require.Equal(t, "/provision/test-server", link.Path)
		require.True(t, signer.Verify(link.Path, link.Query()))
		require.Equal(t, tt.automatic, link.Query().Get("start") == "1")
		require.Equal(t, tt.automatic, link.Query().Has("expires"))
	}
}

func TestProvisionCommandRunsDownloadedScript(t *testing.T) {
	const stubs = `
wget() {
    printf '%s' "$TEST_PROVISION_SCRIPT"
}
sudo() { [ "$#" -eq 1 ] && "$@"; }
`
	for _, tt := range []struct {
		name, script, output string
		wantError            bool
	}{
		{name: "setup succeeded", script: "echo setup started", output: "setup started\n"},
		{name: "setup failed", script: "echo setup failed >&2; exit 1", output: "setup failed\n", wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("bash", "-c", stubs+provisionCommand("https://app.example.test/provision/server"))
			cmd.Env = append(cmd.Environ(), "TEST_PROVISION_SCRIPT="+tt.script)
			output, err := cmd.CombinedOutput()
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.output, string(output))
		})
	}
}
