package jobs

import (
	"context"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/certificate/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// TypeWarnExpiringCertificates is the daily-scheduled walk that emits
// expiring_soon events for any stored certs whose not_after falls in
// the next 30 days. Registered in internal/schedule/kernel.go via
// Daily(...).
//
// Rationale for a daily walk (vs per-cert scheduling):
//   - There are few stored certs per team, and the 30-day window
//     means we have 30 retries to surface a cert before it actually
//     expires. Missing a day costs nothing.
//   - A single-process walker is dead-simple to reason about, no
//     side-channel for cancellation when a cert is rotated.
const TypeWarnExpiringCertificates = "certificate:warn_expiring"

// WarnExpiringWindow is the look-ahead horizon: anything expiring
// inside this window from "now" produces an alert.
const WarnExpiringWindow = 30 * 24 * time.Hour

// WarnExpiringPayload is empty — the walker has all the context it
// needs from the DB.
type WarnExpiringPayload struct{}

// WarnExpiringCertificatesJob is the daily tick.
type WarnExpiringCertificatesJob struct {
	Deps    *JobDeps
	Payload WarnExpiringPayload
}

// NewWarnExpiringCertificatesJob is the asynq constructor (see
// register.go).
func NewWarnExpiringCertificatesJob(p WarnExpiringPayload) pkgjobs.Handler {
	return &WarnExpiringCertificatesJob{Deps: deps, Payload: p}
}

// Handle walks alive stored certs across every team and broadcasts a
// certificate.expiring_soon event for each cert whose not_after is
// within the warn window. Future iteration: throttle so the same cert
// doesn't broadcast every day; for now the frontend rate-limits the
// banner via per-cert dismissal state.
func (j *WarnExpiringCertificatesJob) Handle(ctx context.Context) error {
	now := time.Now().UTC()
	deadline := now.Add(WarnExpiringWindow)

	var rows []models.StoredCertificate
	if err := j.Deps.DB.WithContext(ctx).
		Where("not_after >= ? AND not_after <= ?", now, deadline).
		Find(&rows).Error; err != nil {
		j.Deps.Logger.Error().Err(err).Msg("warn-expiring: list query failed")
		return err
	}

	for i := range rows {
		c := &rows[i]
		daysRemaining := int(c.NotAfter.Sub(now).Hours() / 24)
		j.Deps.Logger.Info().
			Str("certificate_id", c.ID).
			Str("team_id", c.TeamID).
			Str("name", c.Name).
			Int("days_remaining", daysRemaining).
			Msg("stored certificate expiring soon")

		j.Deps.Broadcaster.BroadcastToTeam(c.TeamID, "certificate.expiring_soon", map[string]any{
			"team_id":        c.TeamID,
			"certificate_id": c.ID,
			"name":           c.Name,
			"not_after":      c.NotAfter,
			"days_remaining": daysRemaining,
		})
	}
	return nil
}

// NewWarnExpiringCertificatesTask is the constructor used by the
// scheduler. Signature matches what schedule/kernel.go expects.
func NewWarnExpiringCertificatesTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypeWarnExpiringCertificates, WarnExpiringPayload{})
}
