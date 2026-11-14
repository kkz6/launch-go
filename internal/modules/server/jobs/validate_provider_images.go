package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/notification/notifications"
	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
)

// TypeValidateProviderImages is the asynq task type for the daily image-
// validation cron. It walks every connected cloud-provider account and asks
// each provider's API whether the OS images we have configured still exist.
// Catches the class of bug that brought provisioning down before:
// DigitalOcean retired snapshot ID 168977420 (ubuntu_24) and our config
// still pointed at it, so every new server failed with a 422.
const TypeValidateProviderImages = "server:validate_provider_images"

type ValidateProviderImagesPayload struct{}

type ValidateProviderImagesJob struct {
	Deps    *JobDeps
	Payload ValidateProviderImagesPayload
}

func NewValidateProviderImagesJob(p ValidateProviderImagesPayload) pkgjobs.Handler {
	return &ValidateProviderImagesJob{Deps: deps, Payload: p}
}

func (j *ValidateProviderImagesJob) Handle(ctx context.Context) error {
	var rows []models.ServerProvider
	if err := j.Deps.DB.WithContext(ctx).
		Where("connected = ?", true).
		Find(&rows).Error; err != nil {
		return fmt.Errorf("query server_providers: %w", err)
	}

	if len(rows) == 0 {
		j.Deps.Logger.Info().Msg("validate_provider_images: no connected providers, skipping")
		return nil
	}

	factory := providers.NewFactory(sshkey.NewGenerator())
	configs := config.GetProviderConfigs()

	// Keep findings scoped to the team whose connection was checked.
	type failure struct {
		TeamID   string
		Provider string
		OS       string
		Image    string
		Reason   string
	}
	var failures []failure
	checked := 0

	for i := range rows {
		row := &rows[i]
		cfg, ok := configs[row.Provider.String()]
		if !ok {
			continue
		}

		creds, err := decryptCredentials(row)
		if err != nil {
			j.Deps.Logger.Warn().
				Err(err).
				Str("provider", row.Provider.String()).
				Str("server_provider_id", row.ID).
				Msg("validate_provider_images: skipping (credentials unreadable)")
			continue
		}

		provider, err := factory.Create(row.Provider)
		if err != nil {
			continue
		}

		// Verify the credentials still authenticate before spending more API
		// calls on image lookups. If Connect fails, the bigger story is the
		// stored token is no longer valid — we log but don't claim it's an
		// image issue.
		if err := provider.Connect(ctx, creds); err != nil {
			j.Deps.Logger.Warn().
				Err(err).
				Str("provider", row.Provider.String()).
				Str("server_provider_id", row.ID).
				Msg("validate_provider_images: provider connection could not be verified")
			continue
		}

		teamID := ""
		if row.TeamID != nil {
			teamID = *row.TeamID
		}
		checked++
		for _, f := range providers.ValidateImages(ctx, provider, cfg, creds) {
			failures = append(failures, failure{
				TeamID:   teamID,
				Provider: row.Provider.String(),
				OS:       f.OS,
				Image:    f.Image,
				Reason:   f.Reason,
			})
		}
	}

	if checked != len(rows) {
		j.Deps.Logger.Warn().
			Int("providers_checked", checked).
			Int("providers_skipped", len(rows)-checked).
			Msg("validate_provider_images: provider coverage incomplete")
	}
	if len(failures) == 0 {
		if checked == len(rows) {
			j.Deps.Logger.Info().
				Int("providers_checked", checked).
				Msg("validate_provider_images: all configured images verified")
		}
		return nil
	}

	// Surface failures: structured log + Sentry + per-team notification.
	for _, f := range failures {
		j.Deps.Logger.Error().
			Str("team_id", f.TeamID).
			Str("provider", f.Provider).
			Str("os", f.OS).
			Str("image", f.Image).
			Str("reason", f.Reason).
			Msg("validate_provider_images: configured image could not be verified")
	}

	body, _ := json.Marshal(failures)
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelWarning)
		scope.SetTag("alert", "stale_provider_images")
		scope.SetExtra("failures", string(body))
		sentry.CaptureMessage(fmt.Sprintf("Provider image validation failed: %d configured image checks did not pass", len(failures)))
	})

	// Group findings by team so each owner gets one notification
	// summarising the cumulative state of their providers, rather
	// than N separate pages per image. Skip silently when the
	// notifier isn't wired (dev mode, tests).
	if j.Deps.TaskRunnerDeps != nil && j.Deps.TaskRunnerDeps.Notifier != nil {
		byTeam := make(map[string][]notifications.StaleProviderImageFinding, 4)
		for _, f := range failures {
			byTeam[f.TeamID] = append(byTeam[f.TeamID], notifications.StaleProviderImageFinding{
				Provider: f.Provider,
				OS:       f.OS,
				Image:    f.Image,
				Reason:   f.Reason,
			})
		}
		for teamID, findings := range byTeam {
			if teamID == "" {
				// Unscoped server_providers row (shouldn't happen in
				// practice — every row has a team). Skip rather than
				// trying to fan out to "all teams".
				continue
			}
			notif := notifications.NewStaleProviderImagesNotification(findings)
			if err := j.Deps.TaskRunnerDeps.Notifier.SendToTeam(ctx, teamID, notif); err != nil {
				// Per-team failure: log and continue with the rest.
				// One unreachable channel for one team shouldn't block
				// the cron from notifying the others.
				j.Deps.Logger.Warn().
					Err(err).
					Str("team_id", teamID).
					Msg("validate_provider_images: failed to send stale-images notification")
			}
		}
	}

	// Not a fatal error from asynq's perspective — retrying won't fix a
	// retired upstream image. Return nil so the cron doesn't churn.
	return nil
}

func (j *ValidateProviderImagesJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Msg("validate_provider_images job failed")
}

func decryptCredentials(row *models.ServerProvider) (map[string]any, error) {
	raw := row.Credentials.String()
	if raw == "" {
		return nil, fmt.Errorf("empty credentials")
	}
	var creds map[string]any
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return nil, fmt.Errorf("credentials not valid JSON (encryption key mismatch?): %w", err)
	}
	return creds, nil
}

// NewValidateProviderImagesTask creates an asynq task — used by the scheduler
// (internal/schedule/kernel.go) and any ad-hoc dispatcher.
func NewValidateProviderImagesTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypeValidateProviderImages, ValidateProviderImagesPayload{},
		asynq.TaskID(pkgjobs.Dedup("validate_provider_images", time.Now().Format("2006-01-02"))),
	)
}
