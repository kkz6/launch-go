// internal/modules/certificate/services/parser_test.go
package services_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/certificate/services"
)

func mustRead(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(b)
}

func TestParser_LeafCert_ExtractsAllFields(t *testing.T) {
	parsed, err := services.ParseCertificate(mustRead(t, "leaf.pem"))
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"acme.io", "*.acme.io"}, parsed.Domains)
	assert.Equal(t, "acme.io", parsed.CommonName)
	assert.NotEmpty(t, parsed.Issuer)
	assert.WithinDuration(t, time.Now().Add(365*24*time.Hour), parsed.NotAfter, 1*time.Hour)
	assert.Len(t, parsed.FingerprintSHA256, 64) // hex chars
}

func TestParser_ChainPEM_UsesLeafForMetadata(t *testing.T) {
	parsed, err := services.ParseCertificate(mustRead(t, "chain.pem"))
	require.NoError(t, err)
	assert.Contains(t, parsed.Domains, "acme.io")
}

func TestParser_ExpiredCert_StillParses_WarnsCaller(t *testing.T) {
	parsed, err := services.ParseCertificate(mustRead(t, "expired.pem"))
	require.NoError(t, err)
	assert.True(t, parsed.NotAfter.Before(time.Now()))
}

func TestParser_Malformed_ReturnsError(t *testing.T) {
	_, err := services.ParseCertificate("not a pem block")
	assert.Error(t, err)
}

func TestParser_KeyCertMatch_OK(t *testing.T) {
	err := services.ValidateKeyMatchesCert(
		mustRead(t, "leaf.pem"),
		mustRead(t, "leaf.key"),
	)
	assert.NoError(t, err)
}

func TestParser_KeyCertMismatch_Errors(t *testing.T) {
	err := services.ValidateKeyMatchesCert(
		mustRead(t, "leaf.pem"),
		mustRead(t, "mismatched.key"),
	)
	assert.Error(t, err)
}
