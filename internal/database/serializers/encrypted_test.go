package serializers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

const laravelCBCFixture = "eyJpdiI6Ik1USXpORFUyTnpnNU1HRmlZMlJsWmc9PSIsInZhbHVlIjoidXN4TEgrdWhLUkU4ZjBySG9ManJaMStiZ09sWld5b0k3cFI4UUFUaGlUSEluNlhHL05iU2JwMzBRZVZMOTEybnZHZ3FxamZQRWhxeXdsOHFMN25HOEE9PSIsIm1hYyI6ImM3Y2UxOGRkNWU5MjE2MTc3YjIyYzY1ZjU1OTgwMGYyN2IxYWQyN2IzNDA2OTk2MmZlNDlhMTdhNjFkNWRhNmMiLCJ0YWciOiIifQ=="
const laravelGCMFixture = "eyJpdiI6Ik1USXpORFUyTnpnNU1ERXkiLCJ2YWx1ZSI6IjhuaXJoc1VUVlNweHBTT0xiNGhyNS9xVHkxNGFsMzNwWldjbWxFa093Z0ora3dHeTFZSGE4Z1ZobHdONWI5LzkxRUIva0NRPSIsIm1hYyI6IiIsInRhZyI6Ing0TERnQ1pDMnFtd2lTL2pWSmVhRVE9PSJ9"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	require.NoError(t, SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	encrypted, err := Encrypt("native ciphertext")
	require.NoError(t, err)

	decrypted, err := Decrypt(encrypted)
	require.NoError(t, err)
	require.Equal(t, "native ciphertext", decrypted)
}

func TestDecryptLaravelAESCBCPayload(t *testing.T) {
	require.NoError(t, SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	decrypted, err := Decrypt(laravelCBCFixture)
	require.NoError(t, err)
	require.JSONEq(t, `{"bucket":"nightly","key":"access","secret":"hidden"}`, decrypted)
}

func TestDecryptLaravelAESGCMPayload(t *testing.T) {
	require.NoError(t, SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	decrypted, err := Decrypt(laravelGCMFixture)
	require.NoError(t, err)
	require.JSONEq(t, `{"bucket":"nightly","key":"access","secret":"hidden"}`, decrypted)
}

func TestDecryptRejectsTamperedLaravelPayload(t *testing.T) {
	require.NoError(t, SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	raw, err := base64.StdEncoding.DecodeString(laravelCBCFixture)
	require.NoError(t, err)
	var payload map[string]string
	require.NoError(t, json.Unmarshal(raw, &payload))
	payload["mac"] = "07ce18dd5e9216177b22c65f559800f27b1ad27b34069962fe49a17a61d5da6c"
	tampered, err := json.Marshal(payload)
	require.NoError(t, err)

	_, err = Decrypt(base64.StdEncoding.EncodeToString(tampered))
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrDecryptFailed))
}

func TestDecryptPreservesPlaintextCompatibility(t *testing.T) {
	require.NoError(t, SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	decrypted, err := Decrypt(`{"unencrypted":true}`)
	require.NoError(t, err)
	require.JSONEq(t, `{"unencrypted":true}`, decrypted)
}

func TestDecryptPreservesUnsupportedBase64Payloads(t *testing.T) {
	require.NoError(t, SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	tests := []string{
		base64.StdEncoding.EncodeToString([]byte("short")),
		base64.StdEncoding.EncodeToString([]byte(`{"value":"not-a-laravel-envelope"}`)),
	}
	for _, encrypted := range tests {
		decrypted, err := Decrypt(encrypted)
		require.NoError(t, err)
		require.Equal(t, encrypted, decrypted)
	}
}

func TestDecryptLaravelPayloadValidation(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	validIV := []byte("1234567890abcdef")
	validCiphertext := make([]byte, aes.BlockSize)

	tests := []struct {
		name      string
		raw       []byte
		key       []byte
		detected  bool
		errorType error
	}{
		{
			name:     "invalid JSON",
			raw:      []byte("not-json"),
			key:      key,
			detected: false,
		},
		{
			name:     "missing IV",
			raw:      marshalLaravelPayload(t, laravelEncryptedPayload{Value: "value", MAC: "mac"}),
			key:      key,
			detected: false,
		},
		{
			name:     "missing ciphertext",
			raw:      marshalLaravelPayload(t, laravelEncryptedPayload{IV: "iv", MAC: "mac"}),
			key:      key,
			detected: false,
		},
		{
			name:     "missing authenticator",
			raw:      marshalLaravelPayload(t, laravelEncryptedPayload{IV: "iv", Value: "value"}),
			key:      key,
			detected: false,
		},
		{
			name: "invalid IV encoding",
			raw: marshalLaravelPayload(t, laravelEncryptedPayload{
				IV: "!", Value: base64.StdEncoding.EncodeToString(validCiphertext), MAC: "00",
			}),
			key:       key,
			detected:  true,
			errorType: ErrInvalidData,
		},
		{
			name: "invalid ciphertext encoding",
			raw: marshalLaravelPayload(t, laravelEncryptedPayload{
				IV: base64.StdEncoding.EncodeToString(validIV), Value: "!", MAC: "00",
			}),
			key:       key,
			detected:  true,
			errorType: ErrInvalidData,
		},
		{
			name: "invalid AES key",
			raw: marshalLaravelPayload(t, laravelEncryptedPayload{
				IV:    base64.StdEncoding.EncodeToString(validIV),
				Value: base64.StdEncoding.EncodeToString(validCiphertext),
				MAC:   "00",
			}),
			key:       []byte("short"),
			detected:  true,
			errorType: ErrDecryptFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, detected, err := decryptLaravelPayload(tt.raw, tt.key)
			require.Equal(t, tt.detected, detected)
			if tt.errorType == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.errorType)
		})
	}
}

func TestDecryptLaravelGCMPayloadValidation(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	validIV := []byte("123456789012")
	validValue := base64.StdEncoding.EncodeToString([]byte("ciphertext"))

	tests := []struct {
		name      string
		payload   laravelEncryptedPayload
		errorType error
	}{
		{
			name: "invalid tag encoding",
			payload: laravelEncryptedPayload{
				IV: base64.StdEncoding.EncodeToString(validIV), Value: validValue, Tag: "!",
			},
			errorType: ErrInvalidData,
		},
		{
			name: "invalid nonce size",
			payload: laravelEncryptedPayload{
				IV: base64.StdEncoding.EncodeToString([]byte("short")), Value: validValue, Tag: base64.StdEncoding.EncodeToString(make([]byte, 16)),
			},
			errorType: ErrInvalidData,
		},
		{
			name: "invalid authentication tag",
			payload: laravelEncryptedPayload{
				IV: base64.StdEncoding.EncodeToString(validIV), Value: validValue, Tag: base64.StdEncoding.EncodeToString(make([]byte, 16)),
			},
			errorType: ErrDecryptFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, detected, err := decryptLaravelPayload(marshalLaravelPayload(t, tt.payload), key)
			require.True(t, detected)
			require.ErrorIs(t, err, tt.errorType)
		})
	}
}

func TestDecryptLaravelGCMPayloadReportsConstructorFailure(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	raw, err := base64.StdEncoding.DecodeString(laravelGCMFixture)
	require.NoError(t, err)

	_, detected, err := decryptLaravelPayloadWithGCM(raw, key, func(cipher.Block) (cipher.AEAD, error) {
		return nil, errors.New("GCM unavailable")
	})

	require.True(t, detected)
	require.ErrorIs(t, err, ErrDecryptFailed)
	require.ErrorContains(t, err, "GCM unavailable")
}

func TestDecryptLaravelCBCPayloadValidation(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	validIV := []byte("1234567890abcdef")
	validCiphertext := make([]byte, aes.BlockSize)
	emptyCiphertext := laravelEncryptedPayload{
		IV:    base64.StdEncoding.EncodeToString(validIV),
		Value: "\n",
	}
	signLaravelCBCPayload(&emptyCiphertext, key)

	tests := []struct {
		name      string
		payload   laravelEncryptedPayload
		errorType error
	}{
		{
			name: "invalid MAC encoding",
			payload: laravelEncryptedPayload{
				IV: base64.StdEncoding.EncodeToString(validIV), Value: base64.StdEncoding.EncodeToString(validCiphertext), MAC: "!",
			},
			errorType: ErrInvalidData,
		},
		{
			name: "invalid MAC value",
			payload: laravelEncryptedPayload{
				IV: base64.StdEncoding.EncodeToString(validIV), Value: base64.StdEncoding.EncodeToString(validCiphertext), MAC: hex.EncodeToString(make([]byte, sha256.Size)),
			},
			errorType: ErrDecryptFailed,
		},
		{
			name:      "invalid IV size",
			payload:   signedLaravelCBCPayload([]byte("short"), validCiphertext, key),
			errorType: ErrInvalidData,
		},
		{
			name:      "empty ciphertext",
			payload:   emptyCiphertext,
			errorType: ErrInvalidData,
		},
		{
			name:      "unaligned ciphertext",
			payload:   signedLaravelCBCPayload(validIV, []byte("short"), key),
			errorType: ErrInvalidData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, detected, err := decryptLaravelPayload(marshalLaravelPayload(t, tt.payload), key)
			require.True(t, detected)
			require.ErrorIs(t, err, tt.errorType)
		})
	}
}

func TestDecryptLaravelCBCRejectsInvalidPadding(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	iv := []byte("1234567890abcdef")

	tests := []struct {
		name      string
		plaintext []byte
	}{
		{name: "zero padding", plaintext: append(make([]byte, aes.BlockSize-1), 0)},
		{name: "oversized padding", plaintext: append(make([]byte, aes.BlockSize-1), aes.BlockSize+1)},
		{name: "inconsistent padding", plaintext: append(make([]byte, aes.BlockSize-2), 1, 2)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := aes.NewCipher(key)
			require.NoError(t, err)
			ciphertext := make([]byte, len(tt.plaintext))
			cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, tt.plaintext)

			payload := signedLaravelCBCPayload(iv, ciphertext, key)
			_, detected, err := decryptLaravelPayload(marshalLaravelPayload(t, payload), key)
			require.True(t, detected)
			require.ErrorIs(t, err, ErrDecryptFailed)
		})
	}
}

func TestUnpadPKCS7(t *testing.T) {
	tests := []struct {
		name     string
		value    []byte
		expected []byte
		wantErr  bool
	}{
		{name: "empty", value: nil, wantErr: true},
		{name: "unaligned", value: []byte{1, 2, 3}, wantErr: true},
		{name: "zero padding", value: []byte{1, 2, 3, 0}, wantErr: true},
		{name: "oversized padding", value: []byte{1, 2, 3, 5}, wantErr: true},
		{name: "inconsistent padding", value: []byte{1, 2, 1, 2}, wantErr: true},
		{name: "valid", value: []byte{1, 2, 2, 2}, expected: []byte{1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := unpadPKCS7(tt.value, 4)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrDecryptFailed)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func marshalLaravelPayload(t *testing.T, payload laravelEncryptedPayload) []byte {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	return raw
}

func signedLaravelCBCPayload(iv, ciphertext, key []byte) laravelEncryptedPayload {
	payload := laravelEncryptedPayload{
		IV:    base64.StdEncoding.EncodeToString(iv),
		Value: base64.StdEncoding.EncodeToString(ciphertext),
	}
	signLaravelCBCPayload(&payload, key)
	return payload
}

func signLaravelCBCPayload(payload *laravelEncryptedPayload, key []byte) {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(payload.IV + payload.Value))
	payload.MAC = hex.EncodeToString(mac.Sum(nil))
}
