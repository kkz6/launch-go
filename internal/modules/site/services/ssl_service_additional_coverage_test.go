package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/database/serializers"
	certmodels "github.com/kkz6/launch-go/internal/modules/certificate/models"
	certrepos "github.com/kkz6/launch-go/internal/modules/certificate/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

func TestUpdateSSLRejectsInaccessibleOrBusySites(t *testing.T) {
	t.Run("missing site", func(t *testing.T) {
		service, _, site := sslLifecycleFixture(t)
		err := service.UpdateSSL(
			context.Background(),
			"missing",
			site.ServerID,
			site.TeamID,
			"user-1",
			&dto.UpdateSSLRequest{TLSSetting: string(sitetypes.TLSSettingInternal)},
		)
		require.Error(t, err)
	})

	t.Run("wrong team", func(t *testing.T) {
		service, _, site := sslLifecycleFixture(t)
		err := service.UpdateSSL(
			context.Background(),
			site.ID,
			site.ServerID,
			"other-team",
			"user-1",
			&dto.UpdateSSLRequest{TLSSetting: string(sitetypes.TLSSettingInternal)},
		)
		require.Error(t, err)
	})

	t.Run("configuration update in progress", func(t *testing.T) {
		service, db, site := sslLifecycleFixture(t)
		now := time.Now()
		require.NoError(t, db.Model(&models.Site{}).
			Where("id = ?", site.ID).
			Update("pending_caddyfile_update_since", now).Error)

		err := service.UpdateSSL(
			context.Background(),
			site.ID,
			site.ServerID,
			site.TeamID,
			"user-1",
			&dto.UpdateSSLRequest{TLSSetting: string(sitetypes.TLSSettingInternal)},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already in progress")
	})
}

func TestUpdateSSLStoredCertificateLifecycle(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey(
		[]byte("0123456789abcdef0123456789abcdef"),
	))

	t.Run("missing stored certificate", func(t *testing.T) {
		service, db, site := sslLifecycleFixture(t)
		createStoredCertificateTable(t, db)
		service.SetStoredCertificateRepository(certrepos.NewStoredCertificateRepository(db))
		attachSSLTestQueue(t, service)
		storedID := "missing"

		err := service.UpdateSSL(
			context.Background(),
			site.ID,
			site.ServerID,
			site.TeamID,
			"user-1",
			&dto.UpdateSSLRequest{TLSSetting: "stored", StoredCertificateID: &storedID},
		)

		require.Error(t, err)
	})

	t.Run("expired stored certificate", func(t *testing.T) {
		service, db, site := sslLifecycleFixture(t)
		createStoredCertificateTable(t, db)
		stored := createStoredCertificate(t, db, site.TeamID, time.Now().Add(-time.Hour))
		service.SetStoredCertificateRepository(certrepos.NewStoredCertificateRepository(db))
		attachSSLTestQueue(t, service)

		err := service.UpdateSSL(
			context.Background(),
			site.ID,
			site.ServerID,
			site.TeamID,
			"user-1",
			&dto.UpdateSSLRequest{TLSSetting: "stored", StoredCertificateID: &stored.ID},
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("stored certificate reserves replacement and snapshots active certificate", func(t *testing.T) {
		service, db, site := sslLifecycleFixture(t)
		createStoredCertificateTable(t, db)
		stored := createStoredCertificate(t, db, site.TeamID, time.Now().Add(24*time.Hour))
		service.SetStoredCertificateRepository(certrepos.NewStoredCertificateRepository(db))
		attachSSLTestQueue(t, service)

		site.Aliases = dbtype.JSONStringSlice{"www.example.test"}
		require.NoError(t, db.Save(site).Error)
		previousPEM := "previous-certificate"
		previous := &models.Certificate{
			BaseModel:   basemodels.BaseModel{ID: "certificate-previous"},
			SiteScoped:  basemodels.SiteScoped{SiteID: site.ID},
			TeamScoped:  basemodels.TeamScoped{TeamID: site.TeamID},
			Type:        sitetypes.CertificateTypeCustom,
			PrivateKey:  dbtype.EncryptedString("previous-key"),
			Certificate: &previousPEM,
			IsActive:    true,
		}
		require.NoError(t, db.Create(previous).Error)

		err := service.UpdateSSL(
			context.Background(),
			site.ID,
			site.ServerID,
			site.TeamID,
			"user-1",
			&dto.UpdateSSLRequest{TLSSetting: "stored", StoredCertificateID: &stored.ID},
		)

		require.NoError(t, err)
		var persisted models.Site
		require.NoError(t, db.First(&persisted, "id = ?", site.ID).Error)
		require.NotNil(t, persisted.PendingTLSReplacementCertID)
		assert.Equal(t, dbtype.JSONStringSlice{previous.ID}, persisted.PendingTLSPreviousCertIDs)

		var replacement models.Certificate
		require.NoError(t, db.First(
			&replacement,
			"id = ?",
			*persisted.PendingTLSReplacementCertID,
		).Error)
		require.NotNil(t, replacement.StoredCertificateID)
		assert.Equal(t, stored.ID, *replacement.StoredCertificateID)
		assert.Equal(t, dbtype.JSONStringSlice{site.Address, "www.example.test"}, replacement.Domains)
		assert.True(t, replacement.IsActive)
		assert.NotNil(t, replacement.UploadedAt)

		require.NoError(t, db.First(&previous, "id = ?", previous.ID).Error)
		assert.False(t, previous.IsActive)
	})
}

func TestUpdateSSLCustomCertificateValidationChecksEveryField(t *testing.T) {
	privateKey := "private-key"
	certificate := "certificate"
	empty := " "
	tests := []struct {
		name        string
		privateKey  *string
		certificate *string
	}{
		{name: "private key missing", certificate: &certificate},
		{name: "certificate missing", privateKey: &privateKey},
		{name: "private key blank", privateKey: &empty, certificate: &certificate},
		{name: "certificate blank", privateKey: &privateKey, certificate: &empty},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, _, site := sslLifecycleFixture(t)
			attachSSLTestQueue(t, service)
			err := service.UpdateSSL(
				context.Background(),
				site.ID,
				site.ServerID,
				site.TeamID,
				"user-1",
				&dto.UpdateSSLRequest{
					TLSSetting:  string(sitetypes.TLSSettingCustom),
					PrivateKey:  test.privateKey,
					Certificate: test.certificate,
				},
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "requires both")
		})
	}
}

func TestUpdateSSLReturnsCertificateQueryError(t *testing.T) {
	service, db, site := sslLifecycleFixture(t)
	attachSSLTestQueue(t, service)
	require.NoError(t, db.Migrator().DropTable(&models.Certificate{}))

	err := service.UpdateSSL(
		context.Background(),
		site.ID,
		site.ServerID,
		site.TeamID,
		"user-1",
		&dto.UpdateSSLRequest{TLSSetting: string(sitetypes.TLSSettingInternal)},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "load current certificates")

	_, err = service.ListCertificates(
		context.Background(), site.ID, site.ServerID, site.TeamID,
	)
	require.Error(t, err)
}

func TestUpdateSSLTransactionFailurePaths(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*testing.T, *gorm.DB)
		request   func() *dto.UpdateSSLRequest
	}{
		{
			name: "reservation database error",
			configure: func(t *testing.T, db *gorm.DB) {
				registerNthUpdateFailure(t, db, "sites", 1)
			},
			request: internalTLSRequest,
		},
		{
			name: "reservation conflict",
			configure: func(t *testing.T, db *gorm.DB) {
				registerUpdateNoRows(t, db, "sites")
			},
			request: internalTLSRequest,
		},
		{
			name: "deactivate old certificate error",
			configure: func(t *testing.T, db *gorm.DB) {
				registerNthUpdateFailure(t, db, "certificates", 1)
			},
			request: inlineTLSRequest,
		},
		{
			name: "create replacement certificate error",
			configure: func(t *testing.T, db *gorm.DB) {
				registerNthCreateFailure(t, db, "certificates", 1)
			},
			request: inlineTLSRequest,
		},
		{
			name: "save replacement reservation error",
			configure: func(t *testing.T, db *gorm.DB) {
				registerNthUpdateFailure(t, db, "sites", 2)
			},
			request: inlineTLSRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, serializers.SetEncryptionKey(
				[]byte("0123456789abcdef0123456789abcdef"),
			))
			service, db, site := sslLifecycleFixture(t)
			attachSSLTestQueue(t, service)
			test.configure(t, db)

			err := service.UpdateSSL(
				context.Background(),
				site.ID,
				site.ServerID,
				site.TeamID,
				"user-1",
				test.request(),
			)

			require.Error(t, err)
			var persisted models.Site
			require.NoError(t, db.First(&persisted, "id = ?", site.ID).Error)
			assert.Equal(t, sitetypes.TLSSettingAuto, persisted.TLSSetting)
			assert.Nil(t, persisted.PendingTLSUpdateSince)
		})
	}
}

func TestUpdateSSLReportsEnqueueAndRollbackFailure(t *testing.T) {
	service, db, site := sslLifecycleFixture(t)
	redis := attachSSLTestQueue(t, service)
	redis.Close()
	registerNthUpdateFailure(t, db, "sites", 2)

	err := service.UpdateSSL(
		context.Background(),
		site.ID,
		site.ServerID,
		site.TeamID,
		"user-1",
		internalTLSRequest(),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollback failed")
}

func TestRollbackTLSReservationFailurePaths(t *testing.T) {
	t.Run("reservation database error", func(t *testing.T) {
		service, db, site := reservedTLSFixture(t, false)
		registerNthUpdateFailure(t, db, "sites", 1)

		err := service.rollbackTLSReservation(
			context.Background(), site, sitetypes.TLSSettingInternal, nil, nil,
		)
		require.Error(t, err)
	})

	t.Run("delete replacement error", func(t *testing.T) {
		service, db, site := reservedTLSFixture(t, true)
		registerNthDeleteFailure(t, db, "certificates", 1)
		replacement := &models.Certificate{BaseModel: basemodels.BaseModel{ID: "replacement"}}

		err := service.rollbackTLSReservation(
			context.Background(), site, sitetypes.TLSSettingCustom, replacement, nil,
		)
		require.Error(t, err)
	})

	t.Run("deactivate certificates error", func(t *testing.T) {
		service, db, site := reservedTLSFixture(t, true)
		registerNthUpdateFailure(t, db, "certificates", 1)
		replacement := &models.Certificate{BaseModel: basemodels.BaseModel{ID: "replacement"}}

		err := service.rollbackTLSReservation(
			context.Background(), site, sitetypes.TLSSettingCustom, replacement, nil,
		)
		require.Error(t, err)
	})

	t.Run("replacement with no previous certificates", func(t *testing.T) {
		service, db, site := reservedTLSFixture(t, true)
		replacement := &models.Certificate{BaseModel: basemodels.BaseModel{ID: "replacement"}}

		err := service.rollbackTLSReservation(
			context.Background(), site, sitetypes.TLSSettingCustom, replacement, nil,
		)
		require.NoError(t, err)
		assert.Error(t, db.First(&models.Certificate{}, "id = ?", replacement.ID).Error)
	})
}

func TestListCertificatesReturnsSiteLookupError(t *testing.T) {
	service, _, site := sslLifecycleFixture(t)

	_, err := service.ListCertificates(
		context.Background(), "missing", site.ServerID, site.TeamID,
	)

	require.Error(t, err)
}

func internalTLSRequest() *dto.UpdateSSLRequest {
	return &dto.UpdateSSLRequest{TLSSetting: string(sitetypes.TLSSettingInternal)}
}

func inlineTLSRequest() *dto.UpdateSSLRequest {
	privateKey := "private-key"
	certificate := "certificate"
	return &dto.UpdateSSLRequest{
		TLSSetting:  string(sitetypes.TLSSettingCustom),
		PrivateKey:  &privateKey,
		Certificate: &certificate,
	}
}

func createStoredCertificateTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`
		CREATE TABLE stored_certificates (
			id text PRIMARY KEY,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime,
			team_id text NOT NULL,
			user_id text,
			name text NOT NULL,
			notes text,
			certificate text NOT NULL,
			private_key text NOT NULL,
			domains text NOT NULL DEFAULT '[]',
			common_name text,
			issuer text,
			not_before datetime NOT NULL,
			not_after datetime NOT NULL,
			serial_number text,
			fingerprint_sha256 text
		)
	`).Error)
}

func createStoredCertificate(
	t *testing.T,
	db *gorm.DB,
	teamID string,
	notAfter time.Time,
) *certmodels.StoredCertificate {
	t.Helper()
	stored := &certmodels.StoredCertificate{
		BaseModel:   basemodels.BaseModel{ID: "stored-certificate"},
		TeamID:      teamID,
		Name:        "Stored certificate",
		Certificate: "stored-certificate-pem",
		PrivateKey:  dbtype.EncryptedString("stored-private-key"),
		Domains:     dbtype.JSONStringSlice{"example.test"},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    notAfter,
	}
	require.NoError(t, db.Create(stored).Error)
	return stored
}

func reservedTLSFixture(
	t *testing.T,
	withReplacement bool,
) (*SSLService, *gorm.DB, *models.Site) {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey(
		[]byte("0123456789abcdef0123456789abcdef"),
	))
	service, db, site := sslLifecycleFixture(t)
	target := sitetypes.TLSSettingInternal
	updates := map[string]any{
		"tls_setting":                  target,
		"pending_tls_update_since":     time.Now(),
		"pending_tls_previous_setting": site.TLSSetting,
	}
	if withReplacement {
		target = sitetypes.TLSSettingCustom
		updates["tls_setting"] = target
		certificate := "replacement-pem"
		replacement := &models.Certificate{
			BaseModel:   basemodels.BaseModel{ID: "replacement"},
			SiteScoped:  basemodels.SiteScoped{SiteID: site.ID},
			TeamScoped:  basemodels.TeamScoped{TeamID: site.TeamID},
			Type:        sitetypes.CertificateTypeCustom,
			PrivateKey:  dbtype.EncryptedString("replacement-key"),
			Certificate: &certificate,
			IsActive:    true,
		}
		require.NoError(t, db.Create(replacement).Error)
	}
	require.NoError(t, db.Model(&models.Site{}).
		Where("id = ?", site.ID).
		Updates(updates).Error)
	return service, db, site
}

func registerNthUpdateFailure(t *testing.T, db *gorm.DB, table string, nth int) {
	t.Helper()
	name := callbackName(t, "update", table)
	count := 0
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(
		name,
		func(tx *gorm.DB) {
			if tx.Statement.Table != table {
				return
			}
			count++
			if count == nth {
				tx.AddError(errors.New("forced update failure"))
			}
		},
	))
}

func registerNthCreateFailure(t *testing.T, db *gorm.DB, table string, nth int) {
	t.Helper()
	name := callbackName(t, "create", table)
	count := 0
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(
		name,
		func(tx *gorm.DB) {
			if tx.Statement.Table != table {
				return
			}
			count++
			if count == nth {
				tx.AddError(errors.New("forced create failure"))
			}
		},
	))
}

func registerNthDeleteFailure(t *testing.T, db *gorm.DB, table string, nth int) {
	t.Helper()
	name := callbackName(t, "delete", table)
	count := 0
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register(
		name,
		func(tx *gorm.DB) {
			if tx.Statement.Table != table {
				return
			}
			count++
			if count == nth {
				tx.AddError(errors.New("forced delete failure"))
			}
		},
	))
}

func registerUpdateNoRows(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	name := callbackName(t, "no-rows", table)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(
		name,
		func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				tx.Statement.AddClause(clause.Where{Exprs: []clause.Expression{
					clause.Expr{SQL: "1 = 0"},
				}})
			}
		},
	))
}

func callbackName(t *testing.T, operation, table string) string {
	return fmt.Sprintf(
		"test:%s:%s:%s",
		operation,
		table,
		strings.NewReplacer("/", "-", " ", "-").Replace(t.Name()),
	)
}
