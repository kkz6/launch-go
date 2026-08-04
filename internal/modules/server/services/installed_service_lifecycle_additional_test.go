package services

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	queuepkg "github.com/kkz6/launch-go/internal/pkg/queue"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

type installedServiceLifecycleRepository struct {
	contracts.ServiceRepository
	service             *models.InstalledService
	findErr             error
	claimResult         bool
	claimErr            error
	restoreResult       bool
	restoreErr          error
	updateFieldsErr     error
	extensionStatusErr  error
	extensionStatus     string
	extensionStatusName string
}

func (r *installedServiceLifecycleRepository) FindByID(
	_ context.Context,
	id string,
) (*models.InstalledService, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.service != nil && r.service.ID == id {
		return r.service, nil
	}
	return nil, errors.New("service not found")
}

func (r *installedServiceLifecycleRepository) FindPhpByServerAndVersion(
	_ context.Context,
	serverID,
	_ string,
) (*models.InstalledService, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.service != nil && r.service.ServerID == serverID {
		return r.service, nil
	}
	return nil, errors.New("service not found")
}

func (r *installedServiceLifecycleRepository) ClaimPhpPatch(
	_ context.Context,
	_ string,
	_ types.ServiceStatus,
) (bool, error) {
	if r.claimErr != nil {
		return false, r.claimErr
	}
	if r.claimResult && r.service != nil {
		r.service.Status = types.ServiceStatusUpdating
	}
	return r.claimResult, nil
}

func (r *installedServiceLifecycleRepository) RestorePhpPatchStatus(
	_ context.Context,
	_ string,
	status types.ServiceStatus,
) (bool, error) {
	if r.restoreErr != nil {
		return false, r.restoreErr
	}
	if r.restoreResult && r.service != nil {
		r.service.Status = status
	}
	return r.restoreResult, nil
}

func (r *installedServiceLifecycleRepository) UpdateFields(
	_ context.Context,
	_ string,
	fields map[string]any,
) error {
	if r.updateFieldsErr != nil {
		return r.updateFieldsErr
	}
	if typeData, ok := fields["type_data"].(dbtype.JSONMap); ok {
		r.service.TypeData = typeData
	}
	return nil
}

func (r *installedServiceLifecycleRepository) SetExtensionStatus(
	_ context.Context,
	_ string,
	extension,
	status string,
) error {
	r.extensionStatusName = extension
	r.extensionStatus = status
	return r.extensionStatusErr
}

func TestReservePHPServiceLifecycleBranches(t *testing.T) {
	t.Run("skips non PHP and inactive PHP services", func(t *testing.T) {
		service, installed, _ := newInstalledServiceLifecycleHarness()

		installed.Type = types.ServiceTypeRedis
		previous, reserved, err := service.reservePHPServiceLifecycle(context.Background(), installed)
		require.NoError(t, err)
		assert.Empty(t, previous)
		assert.False(t, reserved)

		installed.Type = types.ServiceTypePhp
		installed.Status = types.ServiceStatusStopped
		previous, reserved, err = service.reservePHPServiceLifecycle(context.Background(), installed)
		require.NoError(t, err)
		assert.Empty(t, previous)
		assert.False(t, reserved)
	})

	t.Run("requires a queue for active PHP", func(t *testing.T) {
		service, installed, _ := newInstalledServiceLifecycleHarness()
		_, _, err := service.reservePHPServiceLifecycle(context.Background(), installed)
		assert.ErrorIs(t, err, pkgservice.ErrQueueRequired)
	})

	t.Run("wraps claim failure", func(t *testing.T) {
		service, installed, repository := newInstalledServiceLifecycleHarness()
		attachLifecycleTestQueue(t, service)
		repository.claimErr = errors.New("claim failed")

		_, _, err := service.reservePHPServiceLifecycle(context.Background(), installed)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reserve PHP service operation")
		assert.ErrorContains(t, err, "claim failed")
	})

	t.Run("rejects a lost claim", func(t *testing.T) {
		service, installed, _ := newInstalledServiceLifecycleHarness()
		attachLifecycleTestQueue(t, service)

		_, _, err := service.reservePHPServiceLifecycle(context.Background(), installed)
		assert.ErrorIs(t, err, ErrServiceBusy)
	})

	t.Run("returns the exact reserved state", func(t *testing.T) {
		service, installed, repository := newInstalledServiceLifecycleHarness()
		attachLifecycleTestQueue(t, service)
		repository.claimResult = true

		previous, reserved, err := service.reservePHPServiceLifecycle(context.Background(), installed)
		require.NoError(t, err)
		assert.Equal(t, types.ServiceStatusRunning, previous)
		assert.True(t, reserved)
		assert.Equal(t, types.ServiceStatusUpdating, installed.Status)
	})
}

func TestRollbackPHPServiceDispatchReportsRecoveryFailures(t *testing.T) {
	dispatchErr := errors.New("queue unavailable")

	t.Run("restores the previous status", func(t *testing.T) {
		service, installed, repository := newInstalledServiceLifecycleHarness()
		installed.Status = types.ServiceStatusUpdating
		repository.restoreResult = true

		err := service.rollbackPHPServiceDispatch(
			context.Background(), installed, types.ServiceStatusRunning, dispatchErr,
		)
		assert.ErrorIs(t, err, dispatchErr)
		assert.Equal(t, types.ServiceStatusRunning, installed.Status)
	})

	t.Run("includes a restore error", func(t *testing.T) {
		service, installed, repository := newInstalledServiceLifecycleHarness()
		repository.restoreErr = errors.New("database unavailable")

		err := service.rollbackPHPServiceDispatch(
			context.Background(), installed, types.ServiceStatusRunning, dispatchErr,
		)
		require.Error(t, err)
		assert.ErrorIs(t, err, dispatchErr)
		assert.Contains(t, err.Error(), "restore status: database unavailable")
	})

	t.Run("reports a lost reservation", func(t *testing.T) {
		service, installed, _ := newInstalledServiceLifecycleHarness()

		err := service.rollbackPHPServiceDispatch(
			context.Background(), installed, types.ServiceStatusRunning, dispatchErr,
		)
		require.Error(t, err)
		assert.ErrorIs(t, err, dispatchErr)
		assert.Contains(t, err.Error(), "reservation could not be released")
	})
}

func TestPHPServiceOperationsReserveAndRollbackOnQueueFailure(t *testing.T) {
	for _, operation := range []types.ServiceOption{
		types.ServiceOptionStart,
		types.ServiceOptionRestart,
		types.ServiceOptionStop,
		types.ServiceOptionUpdate,
		types.ServiceOptionRemove,
	} {
		t.Run(string(operation), func(t *testing.T) {
			service, installed, repository := newInstalledServiceLifecycleHarness()
			repository.claimResult = true
			repository.restoreResult = true
			attachClosedLifecycleTestQueue(t, service)

			err := service.HandleServiceOperation(
				context.Background(), "server-a", "team-a", installed.ID, operation,
			)
			require.Error(t, err)
			assert.Equal(t, types.ServiceStatusRunning, installed.Status)
		})
	}
}

func TestPHPServiceOperationsPropagateReservationFailure(t *testing.T) {
	for _, operation := range []types.ServiceOption{
		types.ServiceOptionStart,
		types.ServiceOptionRemove,
	} {
		t.Run(string(operation), func(t *testing.T) {
			service, installed, repository := newInstalledServiceLifecycleHarness()
			attachLifecycleTestQueue(t, service)
			repository.claimErr = errors.New("reservation failed")

			err := service.HandleServiceOperation(
				context.Background(), "server-a", "team-a", installed.ID, operation,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "reservation failed")
		})
	}
}

func TestSetDefaultPHPAdditionalValidationAndQueueRollback(t *testing.T) {
	t.Run("rejects an invalid PHP software identity", func(t *testing.T) {
		service, installed, _ := newInstalledServiceLifecycleHarness()
		installed.Software = types.SoftwareRedis.String()

		err := service.SetDefaultPhpVersion(
			context.Background(), installed.ID, "server-a", "team-a", "user-a",
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PHP software identity")
	})

	t.Run("does nothing when already default", func(t *testing.T) {
		service, installed, _ := newInstalledServiceLifecycleHarness()
		installed.IsDefault = true

		err := service.SetDefaultPhpVersion(
			context.Background(), installed.ID, "server-a", "team-a", "user-a",
		)
		require.NoError(t, err)
	})

	t.Run("requires a configured database after queue validation", func(t *testing.T) {
		service, installed, _ := newInstalledServiceLifecycleHarness()
		attachLifecycleTestQueue(t, service)

		err := service.SetDefaultPhpVersion(
			context.Background(), installed.ID, "server-a", "team-a", "user-a",
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database not configured")
	})

	t.Run("releases reservation when enqueue fails", func(t *testing.T) {
		service, db, server, installed := phpLifecycleDatabaseFixture(t)
		attachClosedLifecycleTestQueue(t, service)

		err := service.SetDefaultPhpVersion(
			context.Background(), installed[1].ID, server.ID, server.TeamID, "user-a",
		)
		require.Error(t, err)

		var persistedServer models.Server
		require.NoError(t, db.First(&persistedServer, "id = ?", server.ID).Error)
		assert.Nil(t, persistedServer.PendingDefaultPHPServiceID)
		var persistedService models.InstalledService
		require.NoError(t, db.First(&persistedService, "id = ?", installed[1].ID).Error)
		assert.Equal(t, types.ServiceStatusInstalled, persistedService.Status)
	})

	t.Run("propagates a reservation conflict", func(t *testing.T) {
		service, db, server, installed := phpLifecycleDatabaseFixture(t)
		attachLifecycleTestQueue(t, service)
		require.NoError(t, db.Model(&models.Server{}).
			Where("id = ?", server.ID).
			Update("pending_default_php_service_id", installed[0].ID).Error)

		err := service.SetDefaultPhpVersion(
			context.Background(), installed[1].ID, server.ID, server.TeamID, "user-a",
		)
		assert.ErrorIs(t, err, ErrServiceBusy)
	})

	t.Run("reports release database failure after enqueue error", func(t *testing.T) {
		service, db, server, installed := phpLifecycleDatabaseFixture(t)
		attachClosedLifecycleTestQueue(t, service)
		require.NoError(t, db.Exec(`
			CREATE TRIGGER fail_pending_default_release
			BEFORE UPDATE OF pending_default_php_service_id ON servers
			WHEN NEW.pending_default_php_service_id IS NULL
			BEGIN SELECT RAISE(FAIL, 'release failed'); END
		`).Error)

		err := service.SetDefaultPhpVersion(
			context.Background(), installed[1].ID, server.ID, server.TeamID, "user-a",
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "release reservation: release failed")
	})

	t.Run("reports a lost target reservation after enqueue error", func(t *testing.T) {
		service, db, server, installed := phpLifecycleDatabaseFixture(t)
		attachClosedLifecycleTestQueue(t, service)
		require.NoError(t, db.Exec(`
			CREATE TRIGGER ignore_default_status_restore
			BEFORE UPDATE OF status ON services
			WHEN OLD.status = 'updating' AND NEW.status = 'installed'
			BEGIN SELECT RAISE(IGNORE); END
		`).Error)

		err := service.SetDefaultPhpVersion(
			context.Background(), installed[1].ID, server.ID, server.TeamID, "user-a",
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "target service reservation was lost")
	})
}

func TestDefaultPHPReservationReportsDatabaseFailures(t *testing.T) {
	t.Run("lost target claim", func(t *testing.T) {
		service, _, server, installed := phpLifecycleDatabaseFixture(t)
		err := service.reserveDefaultPHPChange(
			context.Background(), server.ID, "missing", installed[0].Status,
		)
		assert.ErrorIs(t, err, ErrServiceBusy)
	})

	t.Run("database failure", func(t *testing.T) {
		service, db, server, installed := phpLifecycleDatabaseFixture(t)
		require.NoError(t, db.Migrator().DropTable(&models.InstalledService{}))

		err := service.reserveDefaultPHPChange(
			context.Background(), server.ID, installed[0].ID, installed[0].Status,
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reserve default PHP change")
	})

	t.Run("server reservation update failure", func(t *testing.T) {
		service, db, server, installed := phpLifecycleDatabaseFixture(t)
		require.NoError(t, db.Exec(`
			CREATE TRIGGER fail_pending_default_reservation
			BEFORE UPDATE OF pending_default_php_service_id ON servers
			WHEN NEW.pending_default_php_service_id IS NOT NULL
			BEGIN SELECT RAISE(FAIL, 'reservation failed'); END
		`).Error)

		err := service.reserveDefaultPHPChange(
			context.Background(), server.ID, installed[1].ID, installed[1].Status,
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reservation failed")
	})

	t.Run("target reservation disappears during release", func(t *testing.T) {
		service, db, server, installed := phpLifecycleDatabaseFixture(t)
		target := installed[1]
		require.NoError(t, service.reserveDefaultPHPChange(
			context.Background(), server.ID, target.ID, target.Status,
		))
		require.NoError(t, db.Model(&models.InstalledService{}).
			Where("id = ?", target.ID).
			Update("status", types.ServiceStatusFailed).Error)

		restored, err := service.releaseDefaultPHPReservation(
			context.Background(), server.ID, target.ID, target.Status,
		)
		require.NoError(t, err)
		assert.False(t, restored)
	})

	t.Run("target status restore fails", func(t *testing.T) {
		service, db, server, installed := phpLifecycleDatabaseFixture(t)
		target := installed[1]
		require.NoError(t, service.reserveDefaultPHPChange(
			context.Background(), server.ID, target.ID, target.Status,
		))
		require.NoError(t, db.Exec(`
			CREATE TRIGGER fail_default_status_restore
			BEFORE UPDATE OF status ON services
			WHEN OLD.status = 'updating' AND NEW.status = 'installed'
			BEGIN SELECT RAISE(FAIL, 'status restore failed'); END
		`).Error)

		restored, err := service.releaseDefaultPHPReservation(
			context.Background(), server.ID, target.ID, target.Status,
		)
		assert.False(t, restored)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status restore failed")
	})
}

func TestPatchPHPVersionFailureRecovery(t *testing.T) {
	t.Run("rejects inactive PHP", func(t *testing.T) {
		service, installed, _ := newInstalledServiceLifecycleHarness()
		installed.Status = types.ServiceStatusStopped

		err := service.PatchPhpVersion(
			context.Background(), installed.ID, "server-a", "team-a", "user-a",
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be active")
	})

	t.Run("wraps claim failure", func(t *testing.T) {
		service, installed, repository := newInstalledServiceLifecycleHarness()
		attachLifecycleTestQueue(t, service)
		repository.claimErr = errors.New("claim failed")

		err := service.PatchPhpVersion(
			context.Background(), installed.ID, "server-a", "team-a", "user-a",
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reserve PHP patch: claim failed")
	})

	t.Run("rejects a lost claim", func(t *testing.T) {
		service, installed, _ := newInstalledServiceLifecycleHarness()
		attachLifecycleTestQueue(t, service)

		err := service.PatchPhpVersion(
			context.Background(), installed.ID, "server-a", "team-a", "user-a",
		)
		assert.ErrorIs(t, err, ErrServiceBusy)
	})

	for _, tc := range []struct {
		name          string
		restoreResult bool
		restoreErr    error
		message       string
	}{
		{name: "restores after state initialization failure", restoreResult: true, message: "initialize PHP patch state"},
		{name: "reports rollback failure", restoreErr: errors.New("restore failed"), message: "rollback status: restore failed"},
		{name: "reports lost rollback reservation", message: "patch reservation could not be released"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, installed, repository := newInstalledServiceLifecycleHarness()
			attachLifecycleTestQueue(t, service)
			repository.claimResult = true
			repository.updateFieldsErr = errors.New("update failed")
			repository.restoreResult = tc.restoreResult
			repository.restoreErr = tc.restoreErr

			err := service.PatchPhpVersion(
				context.Background(), installed.ID, "server-a", "team-a", "user-a",
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.message)
		})
	}

	for _, tc := range []struct {
		name          string
		restoreResult bool
		restoreErr    error
		message       string
	}{
		{name: "restores after enqueue failure", restoreResult: true, message: "client is closed"},
		{name: "reports enqueue rollback failure", restoreErr: errors.New("restore failed"), message: "rollback status: restore failed"},
		{name: "reports lost enqueue reservation", message: "patch reservation could not be released"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, installed, repository := newInstalledServiceLifecycleHarness()
			attachClosedLifecycleTestQueue(t, service)
			repository.claimResult = true
			repository.restoreResult = tc.restoreResult
			repository.restoreErr = tc.restoreErr
			installed.TypeData = dbtype.JSONMap{
				"patch_error":       "old failure",
				"patch_finished_at": "yesterday",
				"preserved":         true,
			}

			err := service.PatchPhpVersion(
				context.Background(), installed.ID, "server-a", "team-a", "user-a",
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.message)
			assert.Equal(t, "queued", installed.TypeData["patch_status"])
			assert.Equal(t, true, installed.TypeData["preserved"])
			assert.NotContains(t, installed.TypeData, "patch_error")
			assert.NotContains(t, installed.TypeData, "patch_finished_at")
		})
	}
}

func TestPHPExtensionLifecycleValidationAndDispatch(t *testing.T) {
	for _, tc := range []struct {
		name     string
		install  bool
		expected string
	}{
		{name: "install", install: true, expected: "installing"},
		{name: "uninstall", expected: "removing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := func(service *Service) error {
				if tc.install {
					return service.InstallPhpExtension(
						context.Background(), "server-a", "team-a", "8.3", "redis", nil,
					)
				}
				return service.UninstallPhpExtension(
					context.Background(), "server-a", "team-a", "8.3", "redis", nil,
				)
			}

			t.Run("inactive", func(t *testing.T) {
				service, installed, _ := newInstalledServiceLifecycleHarness()
				installed.Status = types.ServiceStatusStopped
				err := call(service)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "must be active")
			})

			t.Run("queue required", func(t *testing.T) {
				service, _, _ := newInstalledServiceLifecycleHarness()
				assert.ErrorIs(t, call(service), ErrQueueNotConfigured)
			})

			t.Run("dispatch", func(t *testing.T) {
				service, _, repository := newInstalledServiceLifecycleHarness()
				attachLifecycleTestQueue(t, service)
				require.NoError(t, call(service))
				assert.Equal(t, "redis", repository.extensionStatusName)
				assert.Equal(t, tc.expected, repository.extensionStatus)
			})

			t.Run("lookup failure", func(t *testing.T) {
				service, _, repository := newInstalledServiceLifecycleHarness()
				repository.findErr = errors.New("lookup failed")
				assert.ErrorContains(t, call(service), "lookup failed")
			})

			t.Run("server lookup failure", func(t *testing.T) {
				service, _, _ := newInstalledServiceLifecycleHarness()
				var err error
				if tc.install {
					err = service.InstallPhpExtension(
						context.Background(), "server-a", "wrong-team", "8.3", "redis", nil,
					)
				} else {
					err = service.UninstallPhpExtension(
						context.Background(), "server-a", "wrong-team", "8.3", "redis", nil,
					)
				}
				require.Error(t, err)
			})
		})
	}
}

func newInstalledServiceLifecycleHarness() (
	*Service,
	*models.InstalledService,
	*installedServiceLifecycleRepository,
) {
	server := &models.Server{}
	server.ID = "server-a"
	server.TeamID = "team-a"
	installed := &models.InstalledService{
		Type:     types.ServiceTypePhp,
		Name:     "PHP 8.3",
		Version:  "8.3.6",
		Status:   types.ServiceStatusRunning,
		Software: types.SoftwarePhp83.String(),
	}
	installed.ID = "service-php83"
	installed.ServerID = server.ID
	repository := &installedServiceLifecycleRepository{service: installed}
	registry := &phpLifecycleRegistry{
		serverRepository:  &phpLifecycleServerRepository{server: server},
		serviceRepository: repository,
	}
	logger := zerolog.New(io.Discard)
	service := NewService(ServiceDeps{
		Dependencies: pkgservice.Dependencies{Logger: &logger},
		Repos:        registry,
	})
	return service, installed, repository
}

func attachLifecycleTestQueue(t *testing.T, service *Service) {
	t.Helper()
	redis := miniredis.RunT(t)
	client := queuepkg.NewClient(config.RedisConfig{Address: redis.Addr()})
	service.Queue = client
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})
}

func attachClosedLifecycleTestQueue(t *testing.T, service *Service) {
	t.Helper()
	redis := miniredis.RunT(t)
	client := queuepkg.NewClient(config.RedisConfig{Address: redis.Addr()})
	require.NoError(t, client.Close())
	service.Queue = client
}
