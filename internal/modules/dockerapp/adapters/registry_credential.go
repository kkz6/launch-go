package adapters

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/contracts"
	dockerregistrymodels "github.com/kkz6/launch-go/internal/modules/dockerregistry/models"
)

// RegistryCredentialFetcher is the upstream surface from the
// dockerregistry module that the dockerapp module needs at deploy time.
// Defined here so the adapter does not import dockerregistry/services
// directly (avoiding tight coupling at the contract layer).
type RegistryCredentialFetcher interface {
	GetForApp(ctx context.Context, id, teamID string) (*dockerregistrymodels.Credential, error)
}

// RegistryCredentialAdapter adapts the dockerregistry service to the
// dockerapp/contracts.RegistryCredentialReader interface.
type RegistryCredentialAdapter struct {
	svc RegistryCredentialFetcher
}

// NewRegistryCredentialAdapter constructs the adapter.
func NewRegistryCredentialAdapter(svc RegistryCredentialFetcher) contracts.RegistryCredentialReader {
	return &RegistryCredentialAdapter{svc: svc}
}

// GetForApp returns the projection used at deploy time, decrypting the
// credentials in the process.
func (a *RegistryCredentialAdapter) GetForApp(ctx context.Context, id, teamID string) (*contracts.RegistryCredential, error) {
	c, err := a.svc.GetForApp(ctx, id, teamID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, nil
	}
	return &contracts.RegistryCredential{
		ID:       c.ID,
		Name:     c.Name,
		URL:      c.URL,
		Username: c.Username.String(),
		Password: c.Password.String(),
	}, nil
}
