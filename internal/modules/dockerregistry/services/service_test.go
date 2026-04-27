package services

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/database"
	"github.com/kkz6/launch-go/internal/modules/dockerregistry/dto"
	"github.com/kkz6/launch-go/internal/modules/dockerregistry/models"
	"github.com/kkz6/launch-go/internal/modules/dockerregistry/repositories"
	"github.com/kkz6/launch-go/internal/modules/dockerregistry/types"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

func init() {
	// Encryption is initialised once for the package; the test key is a
	// fixed base64-encoded 32-byte value not used outside this binary.
	_ = database.InitEncryption("QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUE=")
}

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Credential{}))

	logger := zerolog.Nop()
	repos := repositories.NewRegistry(db)
	svc := NewService(ServiceDeps{
		Dependencies: service.Dependencies{
			DB:     db,
			Logger: &logger,
		},
		Repos: repos,
	})
	return svc, db
}

func TestCreate_DefaultsURLForKnownType(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	resp, err := svc.Create(ctx, "team-1", "user-1", &dto.CreateCredentialRequest{
		Name:     "Acme GHCR",
		Type:     types.TypeGHCR,
		Username: "acme-bot",
		Password: "ghp_super_secret",
	})
	require.NoError(t, err)
	require.Equal(t, "ghcr.io", resp.URL)
	require.Equal(t, "acme-bot", resp.Username)
}

func TestCreate_RequiresURLForGeneric(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, "team-1", "user-1", &dto.CreateCredentialRequest{
		Name:     "Self-hosted",
		Type:     types.TypeGeneric,
		Username: "ops",
		Password: "secret",
	})
	require.Error(t, err)
}

func TestCreate_RejectsDuplicateNames(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, "team-1", "user-1", &dto.CreateCredentialRequest{
		Name: "Acme GHCR", Type: types.TypeGHCR, Username: "u", Password: "p",
	})
	require.NoError(t, err)

	_, err = svc.Create(ctx, "team-1", "user-1", &dto.CreateCredentialRequest{
		Name: "Acme GHCR", Type: types.TypeGHCR, Username: "u2", Password: "p2",
	})
	require.ErrorIs(t, err, ErrNameTaken)
}

func TestUpdate_KeepsPasswordWhenBlank(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, "team-1", "user-1", &dto.CreateCredentialRequest{
		Name: "Acme", Type: types.TypeGHCR, Username: "u", Password: "original",
	})
	require.NoError(t, err)

	_, err = svc.Update(ctx, created.ID, "team-1", "user-1", &dto.UpdateCredentialRequest{
		Username: "renamed-user",
		// Password omitted intentionally — must NOT clear the stored value.
	})
	require.NoError(t, err)

	var stored models.Credential
	require.NoError(t, db.First(&stored, "id = ?", created.ID).Error)
	require.Equal(t, "original", stored.Password.String())
	require.Equal(t, "renamed-user", stored.Username.String())
}

func TestList_TeamScoped(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, "team-A", "u", &dto.CreateCredentialRequest{
		Name: "A", Type: types.TypeGHCR, Username: "u", Password: "p",
	})
	require.NoError(t, err)
	_, err = svc.Create(ctx, "team-B", "u", &dto.CreateCredentialRequest{
		Name: "B", Type: types.TypeGHCR, Username: "u", Password: "p",
	})
	require.NoError(t, err)

	creds, err := svc.List(ctx, "team-A")
	require.NoError(t, err)
	require.Len(t, creds, 1)
	require.Equal(t, "A", creds[0].Name)
}

func TestDelete_RemovesCredential(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, "team-1", "u", &dto.CreateCredentialRequest{
		Name: "Trash me", Type: types.TypeGHCR, Username: "u", Password: "p",
	})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, created.ID, "team-1", "u"))

	var count int64
	require.NoError(t, db.Model(&models.Credential{}).Where("id = ?", created.ID).Count(&count).Error)
	require.EqualValues(t, 0, count)
}
