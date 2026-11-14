package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
	"github.com/kkz6/launch-go/tests/testutil"
)

type contaboStub struct {
	instance providers.ContaboInstance
	calls    int
	listErr  error
}

func (c *contaboStub) ListInstances(_ context.Context, _ providers.ContaboCredentials, _, _ int) (providers.ContaboInstancePage, error) {
	c.calls++
	return providers.ContaboInstancePage{Data: []providers.ContaboInstance{c.instance}}, c.listErr
}

func (c *contaboStub) GetInstance(_ context.Context, _ providers.ContaboCredentials, id int64) (providers.ContaboInstance, error) {
	c.calls++
	if id != c.instance.ID {
		return providers.ContaboInstance{}, fiberutil.NotFound()
	}
	return c.instance, nil
}

func TestContaboConnectListAndImport(t *testing.T) {
	signer := signedurl.GetDefaultSigner()
	signedurl.SetDefaultSigner(signedurl.NewSigner("test-signing-secret"))
	t.Cleanup(func() { signedurl.SetDefaultSigner(signer) })
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db := testutil.SQLite(t, &models.ServerProvider{}, &models.Server{}, &models.InstalledService{}, &models.FirewallRule{})
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_servers_contabo_import ON servers (team_id, contabo_instance_key)").Error)
	log := zerolog.Nop()
	svc := NewService(ServiceDeps{
		Dependencies: service.Dependencies{DB: db, Logger: &log},
		Repos:        repositories.NewRegistry(db),
	})
	upstream := &contaboStub{}
	require.NoError(t, json.Unmarshal([]byte(`{"instanceId":123,"tenantId":"INT","displayName":"Example VPS","status":"running","osType":"Linux","ipConfig":{"v4":{"ip":"8.8.4.4"}}}`), &upstream.instance))
	svc.contabo = upstream
	ctx := context.Background()
	req := &dto.CreateServerProviderRequest{
		Provider: "contabo", Profile: "Test account", ClientID: "client-id", ClientSecret: "client-secret",
		APIUser: "test@example.com", APIPassword: "api-password",
	}
	upstream.listErr = fiberutil.BadRequest("Contabo denied access")
	_, err := svc.CreateServerProvider(ctx, "team-a", "user", req)
	require.ErrorContains(t, err, "Contabo denied access")
	var count int64
	require.NoError(t, db.Model(&models.ServerProvider{}).Count(&count).Error)
	require.Zero(t, count)
	upstream.listErr = nil
	account, err := svc.CreateServerProvider(ctx, "team-a", "user", req)
	require.NoError(t, err)
	require.Equal(t, "contabo", account.Provider)
	var encrypted string
	require.NoError(t, db.Raw("SELECT credentials FROM server_providers WHERE id = ?", account.ID).Scan(&encrypted).Error)
	require.NotContains(t, encrypted, req.ClientSecret)
	require.NotContains(t, encrypted, req.APIPassword)

	before := upstream.calls
	_, err = svc.ListContaboVPSs(ctx, account.ID, "team-b", 1)
	require.True(t, fiberutil.IsNotFound(err))
	input := &dto.ImportContaboVPSRequest{InstanceID: "123", Type: "docker", OperatingSystem: "ubuntu_24", SSHPort: 2222}
	_, err = svc.ImportContaboVPS(ctx, account.ID, "team-b", "user", input)
	require.True(t, fiberutil.IsNotFound(err))
	require.Equal(t, before, upstream.calls)

	page, err := svc.ListContaboVPSs(ctx, account.ID, "team-a", 1)
	require.NoError(t, err)
	require.Len(t, page.Instances, 1)
	require.True(t, page.Instances[0].CanImport)
	upstream.instance.Status = "stopped"
	_, err = svc.ImportContaboVPS(ctx, account.ID, "team-a", "user", input)
	require.ErrorContains(t, err, "Select a running Linux VPS")
	upstream.instance.Status = "running"
	server, err := svc.ImportContaboVPS(ctx, account.ID, "team-a", "user", input)
	require.NoError(t, err)
	require.Equal(t, "Example VPS", server.Name)
	require.Equal(t, "custom_server", server.Provider)
	require.Equal(t, "awaiting_connection", server.Status)
	require.Equal(t, 2222, server.SSHPort)
	require.NotEmpty(t, server.ProvisionCommand)
	var rule models.FirewallRule
	require.NoError(t, db.First(&rule, "server_id = ? AND name = ?", server.ID, "ssh").Error)
	require.Equal(t, "2222", rule.Port)

	secondAccount, err := svc.CreateServerProvider(ctx, "team-a", "user", req)
	require.NoError(t, err)
	_, err = svc.ImportContaboVPS(ctx, secondAccount.ID, "team-a", "user", input)
	require.True(t, fiberutil.IsConflict(err))
	page, err = svc.ListContaboVPSs(ctx, secondAccount.ID, "team-a", 1)
	require.NoError(t, err)
	require.Len(t, page.Instances, 1)
	require.Equal(t, server.ID, page.Instances[0].ImportedServerID)
	require.False(t, page.Instances[0].CanImport)
}
