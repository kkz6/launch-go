package models

import (
	"net/url"
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
		require.Contains(t, tt.command, "wget -q -O -")
		link, err := url.Parse(strings.Split(tt.command, "'")[1])
		require.NoError(t, err)
		require.Equal(t, "app.example.test", link.Host)
		require.Equal(t, "/provision/test-server", link.Path)
		require.True(t, signer.Verify(link.Path, link.Query()))
		require.Equal(t, tt.automatic, link.Query().Get("start") == "1")
		require.Equal(t, tt.automatic, link.Query().Has("expires"))
	}
}
