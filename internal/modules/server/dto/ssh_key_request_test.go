package dto

import (
	"testing"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/stretchr/testify/require"
)

func TestAttachSSHKeyRequest(t *testing.T) {
	for _, tc := range []struct {
		name  string
		req   AttachSSHKeyRequest
		valid bool
	}{
		{"saved key", AttachSSHKeyRequest{SSHKeyID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}, true},
		{"custom key", AttachSSHKeyRequest{Name: "Laptop", PublicKey: "ssh-ed25519 AAAA"}, true},
		{"empty", AttachSSHKeyRequest{}, false},
		{"custom sentinel is not an ID", AttachSSHKeyRequest{SSHKeyID: "custom"}, false},
		{"missing name", AttachSSHKeyRequest{PublicKey: "ssh-ed25519 AAAA"}, false},
		{"missing public key", AttachSSHKeyRequest{Name: "Laptop"}, false},
		{"mixed modes", AttachSSHKeyRequest{SSHKeyID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Name: "Laptop", PublicKey: "ssh-ed25519 AAAA"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := fiberutil.ValidateStruct(&tc.req)
			if tc.valid {
				require.Nil(t, errs)
			} else {
				require.NotEmpty(t, errs)
			}
		})
	}
}
