package dbtype

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/database/serializers"
)

const laravelEncryptedMapFixture = "eyJpdiI6Ik1USXpORFUyTnpnNU1HRmlZMlJsWmc9PSIsInZhbHVlIjoidXN4TEgrdWhLUkU4ZjBySG9ManJaMStiZ09sWld5b0k3cFI4UUFUaGlUSEluNlhHL05iU2JwMzBRZVZMOTEybnZHZ3FxamZQRWhxeXdsOHFMN25HOEE9PSIsIm1hYyI6ImM3Y2UxOGRkNWU5MjE2MTc3YjIyYzY1ZjU1OTgwMGYyN2IxYWQyN2IzNDA2OTk2MmZlNDlhMTdhNjFkNWRhNmMiLCJ0YWciOiIifQ=="

func TestEncryptedJSONMapScansLaravelPayload(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	var credentials EncryptedJSONMap
	require.NoError(t, credentials.Scan(laravelEncryptedMapFixture))

	require.Equal(t, "nightly", credentials.GetString("bucket"))
	require.Equal(t, "access", credentials.GetString("key"))
	require.Equal(t, "hidden", credentials.GetString("secret"))
}

func TestEncryptedJSONMapRejectsUnknownCiphertext(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	var credentials EncryptedJSONMap
	err := credentials.Scan("not-json-or-supported-ciphertext")

	require.ErrorContains(t, err, "invalid JSON")
}

func TestEncryptedJSONStringMapRejectsUnknownCiphertext(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	var credentials EncryptedJSONStringMap
	err := credentials.Scan([]byte("not-json-or-supported-ciphertext"))

	require.ErrorContains(t, err, "decrypt EncryptedJSONStringMap: invalid JSON")
}
