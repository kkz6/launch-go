package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

type deleteRoundTripper func(*http.Request) (*http.Response, error)

func (f deleteRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type deleteBroadcast struct {
	broadcast.NopBroadcaster
	events []string
	data   []any
}

func (b *deleteBroadcast) BroadcastToTeam(_ string, event string, data any) {
	b.events = append(b.events, event)
	b.data = append(b.data, data)
}

func newDeleteTestJob(t *testing.T, response int) (*DeleteServerJob, *gorm.DB, *deleteBroadcast, *int) {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "delete.sqlite")), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.Server{}, &models.InstalledService{}, &models.ServerProvider{}, &models.SSHKey{}, &models.ServerSSHKey{}))
	require.NoError(t, db.Exec("CREATE TABLE sites (id TEXT, server_id TEXT)").Error)
	require.NoError(t, db.Exec("CREATE TABLE docker_projects (id TEXT, server_id TEXT, deleted_at DATETIME)").Error)
	team := "team-a"
	account := models.ServerProvider{BaseModel: basemodels.BaseModel{ID: "provider-a"}, UserID: "owner-a", TeamID: &team, Provider: types.ProviderDigitalOcean, Credentials: dbtype.EncryptedString(`{"token":"test-token"}`)}
	require.NoError(t, db.Create(&account).Error)
	server := models.Server{BaseModel: basemodels.BaseModel{ID: "server-a"}, TeamScoped: basemodels.TeamScoped{TeamID: team}, Name: "test server", Provider: types.ProviderDigitalOcean, ProviderData: dbtype.JSONMap{"provider_server_id": "instance-a"}, ServerProviderID: &account.ID, Status: types.ServerStatusDeleting}
	require.NoError(t, db.Create(&server).Error)
	key := models.SSHKey{BaseModel: basemodels.BaseModel{ID: "key-a"}, Name: "server key", PublicKey: "ssh-ed25519 test"}
	require.NoError(t, db.Create(&key).Error)
	require.NoError(t, db.Create(&models.ServerSSHKey{ServerID: server.ID, SSHKeyID: key.ID}).Error)
	logger := zerolog.New(io.Discard)
	broadcaster := &deleteBroadcast{}
	job := &DeleteServerJob{Deps: &JobDeps{Deps: &pkgjobs.Deps{DB: db, Logger: &logger, Broadcaster: broadcaster}, Repos: repositories.NewRegistry(db), ProviderFactory: providers.NewFactory(nil)}, Payload: DeleteServerPayload{ServerID: server.ID, TeamID: team, PreviousStatus: types.ServerStatusRunning}}
	calls := new(int)
	oldTransport := http.DefaultTransport
	http.DefaultTransport = deleteRoundTripper(func(r *http.Request) (*http.Response, error) {
		*calls++
		require.Equal(t, "api.digitalocean.com", r.URL.Host)
		require.Equal(t, "/v2/droplets/instance-a", r.URL.Path)
		return &http.Response{StatusCode: response, Status: fmt.Sprintf("%d %s", response, http.StatusText(response)), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":"secret-provider-response"}`)), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	return job, db, broadcaster, calls
}

func TestDeleteServerJobKeepsRecordAndKeysOnFailure(t *testing.T) {
	cases := map[string]func(*testing.T, *DeleteServerJob, *gorm.DB){
		"provider failure": func(*testing.T, *DeleteServerJob, *gorm.DB) {},
		"AWS unsupported": func(t *testing.T, _ *DeleteServerJob, db *gorm.DB) {
			require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("provider", types.ProviderAWS).Error)
			require.NoError(t, db.Model(&models.ServerProvider{}).Where("id = ?", "provider-a").Update("provider", types.ProviderAWS).Error)
		},
		"missing factory": func(_ *testing.T, j *DeleteServerJob, _ *gorm.DB) { j.Deps.ProviderFactory = nil },
		"unsupported provider": func(t *testing.T, _ *DeleteServerJob, db *gorm.DB) {
			require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("provider", "unsupported").Error)
		},
		"missing credential reference": func(t *testing.T, _ *DeleteServerJob, db *gorm.DB) {
			require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("server_provider_id", nil).Error)
		},
		"missing credential account": func(t *testing.T, _ *DeleteServerJob, db *gorm.DB) {
			require.NoError(t, db.Delete(&models.ServerProvider{}, "id = ?", "provider-a").Error)
		},
		"wrong credential team": func(t *testing.T, _ *DeleteServerJob, db *gorm.DB) {
			require.NoError(t, db.Model(&models.ServerProvider{}).Where("id = ?", "provider-a").Update("team_id", "team-b").Error)
		},
		"malformed credentials": func(t *testing.T, _ *DeleteServerJob, db *gorm.DB) {
			require.NoError(t, db.Model(&models.ServerProvider{}).Where("id = ?", "provider-a").Update("credentials", dbtype.EncryptedString("not-json")).Error)
		},
		"empty credentials": func(t *testing.T, _ *DeleteServerJob, db *gorm.DB) {
			require.NoError(t, db.Model(&models.ServerProvider{}).Where("id = ?", "provider-a").Update("credentials", dbtype.EncryptedString("{}")).Error)
		},
		"missing instance ID": func(t *testing.T, _ *DeleteServerJob, db *gorm.DB) {
			require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("provider_data", dbtype.JSONMap{}).Error)
		},
		"site appeared": func(t *testing.T, _ *DeleteServerJob, db *gorm.DB) {
			require.NoError(t, db.Exec("INSERT INTO sites VALUES (?, ?)", "site-a", "server-a").Error)
		},
		"project appeared": func(t *testing.T, _ *DeleteServerJob, db *gorm.DB) {
			require.NoError(t, db.Exec("INSERT INTO docker_projects (id, server_id) VALUES (?, ?)", "project-a", "server-a").Error)
		},
		"guard query failure": func(t *testing.T, _ *DeleteServerJob, db *gorm.DB) {
			require.NoError(t, db.Exec("DROP TABLE sites").Error)
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			job, db, events, calls := newDeleteTestJob(t, http.StatusForbidden)
			setup(t, job, db)
			err := job.Handle(context.Background())
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret-provider-response")
			if name == "provider failure" {
				require.ErrorContains(t, err, "HTTP 403")
			}
			var server models.Server
			require.NoError(t, db.First(&server, "id = ?", "server-a").Error)
			require.Equal(t, types.ServerStatusDeleting, server.Status)
			var key models.SSHKey
			require.NoError(t, db.First(&key, "id = ?", "key-a").Error)
			if name != "provider failure" {
				require.Zero(t, *calls)
			}
			job.Failed(context.Background(), errors.New("secret-provider-response"))
			require.NoError(t, db.First(&server, "id = ?", "server-a").Error)
			require.Equal(t, types.ServerStatusRunning, server.Status)
			require.Equal(t, []string{"server.deletion_failed", "server.updated"}, events.events)
			payload, err := json.Marshal(events.data)
			require.NoError(t, err)
			require.NotContains(t, string(payload), "secret-provider-response")
			require.Contains(t, string(payload), `"status":"running"`)
		})
	}
}

func TestDeleteServerJobScopesOwnershipAndCurrentState(t *testing.T) {
	for _, wrongTeam := range []bool{false, true} {
		t.Run(fmt.Sprint(wrongTeam), func(t *testing.T) {
			job, db, events, calls := newDeleteTestJob(t, http.StatusNoContent)
			if wrongTeam {
				job.Payload.TeamID = "team-b"
			} else {
				require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("status", types.ServerStatusArchived).Error)
			}
			err := job.Handle(context.Background())
			require.Error(t, err)
			job.Failed(context.Background(), err)
			var server models.Server
			require.NoError(t, db.First(&server, "id = ?", "server-a").Error)
			if wrongTeam {
				require.Equal(t, types.ServerStatusDeleting, server.Status)
			} else {
				require.Equal(t, types.ServerStatusArchived, server.Status)
			}
			require.Empty(t, events.events)
			require.Zero(t, *calls)
		})
	}
}

func TestDeleteServerRejectedTaskDoesNotRestoreLaterIntent(t *testing.T) {
	job, db, events, calls := newDeleteTestJob(t, http.StatusNoContent)
	require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("status", types.ServerStatusRunning).Error)
	err := job.Handle(context.Background())
	require.Error(t, err)
	// A new request may claim the server after the old task has been rejected.
	require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("status", types.ServerStatusDeleting).Error)
	job.Failed(context.Background(), err)
	var server models.Server
	require.NoError(t, db.First(&server, "id = ?", "server-a").Error)
	require.Equal(t, types.ServerStatusDeleting, server.Status)
	require.Empty(t, events.events)
	require.Zero(t, *calls)
}

func TestDeleteServerJobDeletesOnlyAfterProviderSuccess(t *testing.T) {
	for _, kind := range []string{"success", "already absent", "custom"} {
		t.Run(kind, func(t *testing.T) {
			status := http.StatusNoContent
			if kind == "already absent" {
				status = http.StatusNotFound
			}
			job, db, events, calls := newDeleteTestJob(t, status)
			if kind == "custom" {
				job.Deps.ProviderFactory = nil
				require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("provider", types.ProviderCustom).Error)
			}
			require.NoError(t, db.Exec("INSERT INTO docker_projects VALUES (?, ?, CURRENT_TIMESTAMP)", "removed-project", "server-a").Error)
			require.NoError(t, job.Handle(context.Background()))
			var count int64
			require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Count(&count).Error)
			require.Zero(t, count)
			require.Equal(t, []string{"server.deleted"}, events.events)
			if kind == "custom" {
				require.Zero(t, *calls)
			} else {
				require.Equal(t, 1, *calls)
			}
		})
	}
}

func TestDeleteServerFailureRestoresLegacyFallbackAfterCancellation(t *testing.T) {
	for _, previous := range []types.ServerStatus{"", types.ServerStatusDeleting, types.ServerStatusProvisioning, types.ServerStatusStarting, "invalid"} {
		t.Run(string(previous), func(t *testing.T) {
			job, db, events, _ := newDeleteTestJob(t, http.StatusForbidden)
			job.Payload.PreviousStatus = previous
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			job.Failed(ctx, errors.New("failure before server lookup"))
			var server models.Server
			require.NoError(t, db.First(&server, "id = ?", "server-a").Error)
			require.Equal(t, types.ServerStatusFailed, server.Status)
			require.Equal(t, []string{"server.deletion_failed", "server.updated"}, events.events)
		})
	}
}

func TestDeleteServerTasksCarryActorAndAllowNewIntentAfterFailure(t *testing.T) {
	actor := "staff-a"
	first, err := NewDeleteServerTask("server-a", "team-a", &actor, types.ServerStatusRunning)
	require.NoError(t, err)
	second, err := NewDeleteServerTask("server-a", "team-a", &actor, types.ServerStatusRunning)
	require.NoError(t, err)
	redis := miniredis.RunT(t)
	options := asynq.RedisClientOpt{Addr: redis.Addr()}
	client := asynq.NewClient(options)
	defer client.Close()
	inspector := asynq.NewInspector(options)
	defer inspector.Close()
	firstInfo, err := client.Enqueue(first)
	require.NoError(t, err)
	require.NoError(t, inspector.ArchiveTask(firstInfo.Queue, firstInfo.ID))
	secondInfo, err := client.Enqueue(second)
	require.NoError(t, err, "an archived failure must not prevent a fresh deletion intent")
	require.NotEqual(t, firstInfo.ID, secondInfo.ID)
	payload, err := pkgjobs.UnmarshalPayload[DeleteServerPayload](first)
	require.NoError(t, err)
	require.Equal(t, types.ServerStatusRunning, payload.PreviousStatus)
	require.Equal(t, &actor, payload.UserID)
	legacy, err := NewDeleteServerTask("server-a", "team-a", nil)
	require.NoError(t, err)
	legacyPayload, err := pkgjobs.UnmarshalPayload[DeleteServerPayload](legacy)
	require.NoError(t, err)
	require.Equal(t, types.ServerStatusFailed, legacyPayload.PreviousStatus)
}

// This test uses a disposable PostgreSQL database and an isolated schema.
// The real FK locking behavior is not available in the SQLite fixtures above.
func TestDeleteServerJobSerializesConcurrentWorkloadInsertion(t *testing.T) {
	dsn := os.Getenv("SERVER_DELETE_TEST_DSN")
	if dsn == "" {
		dsn = os.Getenv("MIGRATION_TEST_DSN")
	}
	if dsn == "" {
		t.Skip("set SERVER_DELETE_TEST_DSN to a disposable PostgreSQL database")
	}
	for _, table := range []string{"sites", "docker_projects"} {
		for _, succeed := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/success=%t", table, succeed), func(t *testing.T) {
				schema := "delete_job_" + strings.ToLower(util.NewULID())
				scopedDSN := dsn
				if strings.Contains(dsn, "://") {
					parsed, err := url.Parse(dsn)
					require.NoError(t, err)
					query := parsed.Query()
					query.Set("search_path", schema)
					parsed.RawQuery = query.Encode()
					scopedDSN = parsed.String()
				} else {
					scopedDSN += " search_path=" + schema
				}
				db, err := gorm.Open(postgres.Open(scopedDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
				require.NoError(t, err)
				sqlDB, err := db.DB()
				require.NoError(t, err)
				sqlDB.SetMaxOpenConns(8)
				t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
				require.NoError(t, db.Exec("CREATE SCHEMA "+schema).Error)
				t.Cleanup(func() { require.NoError(t, db.Exec("DROP SCHEMA "+schema+" CASCADE").Error) })
				for _, statement := range []string{
					"CREATE TABLE servers (id TEXT PRIMARY KEY, team_id TEXT, name TEXT, provider TEXT, provider_data JSONB, server_provider_id TEXT, status TEXT, updated_at TIMESTAMP)",
					"CREATE TABLE server_providers (id TEXT PRIMARY KEY, team_id TEXT, provider TEXT, credentials TEXT)",
					"CREATE TABLE services (id TEXT, server_id TEXT)",
					"CREATE TABLE sites (id TEXT PRIMARY KEY, server_id TEXT REFERENCES servers(id) ON DELETE CASCADE)",
					"CREATE TABLE docker_projects (id TEXT PRIMARY KEY, server_id TEXT REFERENCES servers(id) ON DELETE CASCADE, deleted_at TIMESTAMP)",
					"CREATE TABLE ssh_keys (id TEXT PRIMARY KEY, is_global BOOLEAN)",
					"CREATE TABLE server_ssh_keys (server_id TEXT, ssh_key_id TEXT)",
				} {
					require.NoError(t, db.Exec(statement).Error)
				}
				require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
				require.NoError(t, db.Exec("INSERT INTO server_providers VALUES (?, ?, ?, ?)", "provider-a", "team-a", "digitalocean", dbtype.EncryptedString(`{"token":"test-token"}`)).Error)
				require.NoError(t, db.Exec("INSERT INTO servers (id, team_id, name, provider, provider_data, server_provider_id, status) VALUES (?, ?, ?, ?, ?, ?, ?)", "server-a", "team-a", "test server", "digitalocean", `{"provider_server_id":"instance-a"}`, "provider-a", "deleting").Error)
				log := zerolog.New(io.Discard)
				job := &DeleteServerJob{Deps: &JobDeps{Deps: &pkgjobs.Deps{DB: db, Logger: &log}, Repos: repositories.NewRegistry(db), ProviderFactory: providers.NewFactory(nil)}, Payload: DeleteServerPayload{ServerID: "server-a", TeamID: "team-a", PreviousStatus: types.ServerStatusRunning}}
				enteredProvider, releaseProvider := make(chan struct{}), make(chan struct{})
				var releaseOnce sync.Once
				release := func() { releaseOnce.Do(func() { close(releaseProvider) }) }
				previousTransport := http.DefaultTransport
				http.DefaultTransport = deleteRoundTripper(func(r *http.Request) (*http.Response, error) {
					close(enteredProvider)
					select {
					case <-releaseProvider:
					case <-r.Context().Done():
						return nil, r.Context().Err()
					}
					status := http.StatusNoContent
					if !succeed {
						status = http.StatusForbidden
					}
					return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
				})
				t.Cleanup(func() { http.DefaultTransport = previousTransport })
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				jobError, jobDone := make(chan error, 1), make(chan struct{})
				go func() { jobError <- job.Handle(ctx); close(jobDone) }()
				defer func() { release(); <-jobDone }()
				select {
				case <-enteredProvider:
				case <-ctx.Done():
					t.Fatal("job did not reach provider call")
				}
				connection, err := sqlDB.Conn(ctx)
				require.NoError(t, err)
				defer connection.Close()
				var pid int
				require.NoError(t, connection.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&pid))
				insertError := make(chan error, 1)
				go func() {
					_, err := connection.ExecContext(ctx, "INSERT INTO "+table+" (id, server_id) VALUES ($1, $2)", "new-workload", "server-a")
					insertError <- err
				}()
				require.Eventually(t, func() bool {
					var wait string
					err := db.Raw("SELECT COALESCE(wait_event_type, '') FROM pg_stat_activity WHERE pid = ?", pid).Scan(&wait).Error
					return err == nil && wait == "Lock"
				}, 2*time.Second, 10*time.Millisecond, "FK insertion must wait while the deletion job holds its parent lock")
				release()
				if succeed {
					require.NoError(t, <-jobError)
					err := <-insertError
					require.Error(t, err)
					var state interface{ SQLState() string }
					require.ErrorAs(t, err, &state)
					require.Equal(t, "23503", state.SQLState(), "insertion must fail after server deletion commits")
				} else {
					err := <-jobError
					require.Error(t, err)
					require.NoError(t, <-insertError, "failed provider deletion must retain the parent and allow insertion after rollback")
					job.Failed(context.Background(), err)
					var status types.ServerStatus
					require.NoError(t, db.Model(&models.Server{}).Select("status").Where("id = ?", "server-a").Scan(&status).Error)
					require.Equal(t, types.ServerStatusRunning, status)
				}
			})
		}
	}
}
