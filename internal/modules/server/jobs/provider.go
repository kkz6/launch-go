package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
)

func resolveServerProvider(ctx context.Context, server *models.Server, repos contracts.RepositoryRegistry, factory *providers.Factory) (providers.Provider, map[string]any, error) {
	if factory == nil {
		return nil, nil, errors.New("cloud provider factory is unavailable")
	}
	provider, err := factory.Create(server.Provider)
	if err != nil {
		return nil, nil, errors.New("cloud provider is unsupported")
	}
	if server.ServerProviderID == nil {
		return nil, nil, errors.New("cloud provider credentials are unavailable")
	}
	account, err := repos.ServerProvider().FindByID(ctx, *server.ServerProviderID)
	if err != nil || account == nil || account.Provider != server.Provider ||
		(account.TeamID != nil && *account.TeamID != server.TeamID) {
		return nil, nil, errors.New("cloud provider credentials are unavailable")
	}
	var credentials map[string]any
	if strings.TrimSpace(account.Credentials.String()) == "" ||
		json.Unmarshal([]byte(account.Credentials.String()), &credentials) != nil || len(credentials) == 0 {
		return nil, nil, errors.New("cloud provider credentials are invalid")
	}
	return provider, credentials, nil
}

func requireServerWithoutWorkloads(ctx context.Context, db *gorm.DB, serverID string) error {
	var count int64
	if err := db.WithContext(ctx).Table("sites").Where("server_id = ?", serverID).Count(&count).Error; err != nil {
		return fmt.Errorf("count server sites: %w", err)
	}
	if count > 0 {
		return errors.New("server still has sites")
	}
	if err := db.WithContext(ctx).Table("docker_projects").Where("server_id = ? AND deleted_at IS NULL", serverID).Count(&count).Error; err != nil {
		return fmt.Errorf("count server Docker projects: %w", err)
	}
	if count > 0 {
		return errors.New("server still has Docker projects")
	}
	return nil
}
