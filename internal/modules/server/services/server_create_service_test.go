package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

type provisionAccounts struct {
	contracts.RepositoryRegistry
	contracts.ServerProviderRepository
	account *models.ServerProvider
}

func (r provisionAccounts) ServerProvider() contracts.ServerProviderRepository { return r }

func (r provisionAccounts) FindByID(context.Context, string) (*models.ServerProvider, error) {
	if r.account == nil {
		return nil, fiberutil.NotFound()
	}
	return r.account, nil
}

func TestCreateServerRejectsUnavailableConnectionBeforeCreatingResources(t *testing.T) {
	teamID, otherTeam := "team", "other-team"
	for name, account := range map[string]*models.ServerProvider{
		"missing":      nil,
		"other team":   {TeamID: &otherTeam, Connected: true, Provider: types.ProviderVultr},
		"wrong cloud":  {TeamID: &teamID, Connected: true, Provider: types.ProviderLinode},
		"disconnected": {TeamID: &teamID, Provider: types.ProviderVultr},
	} {
		t.Run(name, func(t *testing.T) {
			svc := &Service{repos: provisionAccounts{account: account}}
			_, err := svc.CreateServer(context.Background(), teamID, "user", &dto.CreateServerRequest{
				Provider: "vultr", Type: "php", CredentialID: "account",
			})
			require.ErrorContains(t, err, "Select a connected account")
		})
	}
}
