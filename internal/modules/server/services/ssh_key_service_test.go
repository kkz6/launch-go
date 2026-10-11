package services

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type sshKeyTestRegistry struct {
	contracts.RepositoryRegistry
	db     *gorm.DB
	server *mockServerRepo
}

func (r *sshKeyTestRegistry) DB() *gorm.DB                       { return r.db }
func (r *sshKeyTestRegistry) Server() contracts.ServerRepository { return r.server }
func (r *sshKeyTestRegistry) SSHKey() contracts.SSHKeyRepository {
	return repositories.NewSSHKeyRepository(r.db)
}

func sshKeyFixture(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&models.SSHKey{}))
	require.NoError(t, db.Migrator().DropTable(&models.ServerSSHKey{}))
	// Match the production migration, which uses a composite key and has no id column.
	require.NoError(t, db.Exec(`CREATE TABLE server_ssh_keys (
		server_id char(26) NOT NULL, ssh_key_id char(26) NOT NULL,
		created_at timestamp NULL, updated_at timestamp NULL,
		PRIMARY KEY (server_id, ssh_key_id)
	)`).Error)
	server := &models.Server{BaseModel: basemodels.BaseModel{ID: "server"}, TeamScoped: basemodels.TeamScoped{TeamID: "team"}}
	return &Service{repos: &sshKeyTestRegistry{db: db, server: &mockServerRepo{server: server}}}, db
}

func validSSHKey(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
}

func TestCreateServerSSHKey(t *testing.T) {
	s, db := sshKeyFixture(t)
	attachTestQueue(t, s)
	publicKey := validSSHKey(t)
	result, err := s.CreateServerSSHKey(context.Background(), "server", "team", "user", &dto.AttachSSHKeyRequest{Name: " Laptop ", PublicKey: "  " + publicKey + "\n"})
	require.NoError(t, err)
	require.Len(t, result.ID, 26)
	var key models.SSHKey
	require.NoError(t, db.First(&key, "id = ?", result.ID).Error)
	require.Equal(t, "Laptop", key.Name)
	require.Equal(t, publicKey, key.PublicKey)
	require.Equal(t, "team", key.TeamID)
	require.Equal(t, "user", key.UserID)
	require.False(t, key.IsGlobal)
	attached, err := s.repos.SSHKey().IsAttachedToServer(context.Background(), "server", key.ID)
	require.NoError(t, err)
	require.True(t, attached)
	pending, err := s.Queue.Inspector().ListPendingTasks("default")
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.Equal(t, jobs.TypeAddSSHKey, pending[0].Type)
	var payload jobs.AddSSHKeyPayload
	require.NoError(t, json.Unmarshal(pending[0].Payload, &payload))
	require.Equal(t, "server", payload.ServerID)
	require.Equal(t, key.ID, payload.KeyID)
}

func TestCreateServerSSHKeyRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct{ name, publicKey string }{
		{"Laptop", "ssh-ed25519 invalid"},
		{"Laptop", validSSHKey(t) + "\n" + validSSHKey(t)},
		{"  ", validSSHKey(t)},
	} {
		s, db := sshKeyFixture(t)
		_, err := s.CreateServerSSHKey(context.Background(), "server", "team", "user", &dto.AttachSSHKeyRequest{Name: tc.name, PublicKey: tc.publicKey})
		require.Error(t, err)
		var count int64
		require.NoError(t, db.Model(&models.SSHKey{}).Count(&count).Error)
		require.Zero(t, count)
	}
}

func TestCreateServerSSHKeyRollsBackIfAttachmentFails(t *testing.T) {
	s, db := sshKeyFixture(t)
	require.NoError(t, db.Migrator().DropTable(&models.ServerSSHKey{}))
	_, err := s.CreateServerSSHKey(context.Background(), "server", "team", "user", &dto.AttachSSHKeyRequest{Name: "Laptop", PublicKey: validSSHKey(t)})
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&models.SSHKey{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestSSHKeyAttachmentTeamScopeAndIdempotency(t *testing.T) {
	s, db := sshKeyFixture(t)
	attachTestQueue(t, s)
	key := &models.SSHKey{Name: "Saved", PublicKey: validSSHKey(t), IsGlobal: true}
	key.TeamID = "team"
	require.NoError(t, db.Create(key).Error)
	ctx := context.Background()
	require.NoError(t, s.AttachSSHKey(ctx, "server", "team", key.ID))
	require.NoError(t, s.AttachSSHKey(ctx, "server", "team", key.ID))
	// The background add-key job repeats this insert after authorizing the key.
	require.NoError(t, s.repos.SSHKey().AttachToServer(ctx, "server", key.ID))
	listed, err := s.ListServerSSHKeys(ctx, "server", "team")
	require.NoError(t, err)
	require.Len(t, listed, 1)
	var count int64
	require.NoError(t, db.Model(&models.ServerSSHKey{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	key.TeamID = "other-team"
	require.NoError(t, db.Save(key).Error)
	require.Error(t, s.AttachSSHKey(ctx, "server", "team", key.ID))
	require.Error(t, s.DetachSSHKey(ctx, key.ID, "server", "team", "user"))
	require.NoError(t, db.Model(&models.ServerSSHKey{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestListGlobalSSHKeysUsesTeamScope(t *testing.T) {
	s, db := sshKeyFixture(t)
	for _, tc := range []struct {
		team   string
		global bool
	}{
		{"team", true}, {"team", false}, {"other-team", true},
	} {
		key := &models.SSHKey{Name: "Saved", PublicKey: validSSHKey(t), IsGlobal: tc.global}
		key.TeamID = tc.team
		require.NoError(t, db.Create(key).Error)
	}
	keys, err := s.ListSSHKeys(context.Background(), "team", true)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.True(t, keys[0].IsGlobal)
}

func TestCreateServerSSHKeyRequiresAccessibleServer(t *testing.T) {
	s, db := sshKeyFixture(t)
	s.repos.(*sshKeyTestRegistry).server.err = fiberutil.NotFound()
	_, err := s.CreateServerSSHKey(context.Background(), "server", "team", "user", &dto.AttachSSHKeyRequest{Name: "Laptop", PublicKey: validSSHKey(t)})
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&models.SSHKey{}).Count(&count).Error)
	require.Zero(t, count)
}
