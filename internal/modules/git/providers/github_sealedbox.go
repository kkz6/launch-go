package providers

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

// sealActionsSecret encrypts plaintext for a GitHub Actions secret
// using the repo's libsodium-format public key.
//
// GitHub's Actions Secrets API expects the body field
// `encrypted_value` to be a libsodium "sealed box" — an anonymous
// box.Seal with a fresh ephemeral keypair, the ciphertext base64-
// encoded. The public key the API returns alongside the key_id is
// base64-encoded raw 32-byte curve25519 public key material.
//
// Implementation notes:
//   - box.SealAnonymous is the exact NaCl construction libsodium's
//     sealed boxes are built on, so the output is wire-compatible
//     with what GitHub's runner libsodium will open with the
//     matching private key.
//   - The ephemeral keypair is generated inside box.SealAnonymous;
//     callers don't need to manage one.
//   - We never log the plaintext, even on error — secret leaks
//     through error wrapping are an easy slip-up.
//
// Reference docs:
//
//	https://docs.github.com/en/rest/actions/secrets#create-or-update-a-repository-secret
//	https://docs.github.com/en/rest/actions/secrets#example-encrypting-a-secret-using-go
func sealActionsSecret(repoPublicKey []byte, plaintext string) (string, error) {
	if len(repoPublicKey) != 32 {
		return "", fmt.Errorf("repo public key must be 32 bytes, got %d", len(repoPublicKey))
	}
	var recipient [32]byte
	copy(recipient[:], repoPublicKey)

	// out=nil + a rand.Reader source produces a self-contained
	// sealed-box ciphertext. box.SealAnonymous prepends the ephemeral
	// public key + nonce internally — the result is exactly what
	// libsodium's crypto_box_seal opens.
	ciphertext, err := box.SealAnonymous(nil, []byte(plaintext), &recipient, rand.Reader)
	if err != nil {
		// Wrap without quoting the plaintext.
		return "", errors.New("sealed-box encryption failed")
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decodeRepoPublicKey decodes the base64 public-key blob GitHub
// returns from /actions/secrets/public-key. Factored out so it can
// be shared between PutActionsSecret and its tests.
func decodeRepoPublicKey(b64 string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode repo public key: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("repo public key must be 32 bytes, got %d", len(raw))
	}
	return raw, nil
}
