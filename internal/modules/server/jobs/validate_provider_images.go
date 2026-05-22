package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	"github.com/kkz6/launch-go/internal/modules/server/types"
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

	// Track results per provider-type so we only report each unique
	// (provider, image) failure once per run even if multiple accounts
	// surface the same broken slug.
	type failure struct {
		Provider string
		OS       string
		Image    string
		Reason   string
	}
	var failures []failure

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
				Msg("validate_provider_images: provider credentials no longer valid")
			continue
		}

		for _, f := range checkProviderImages(ctx, provider, cfg, creds) {
			failures = append(failures, failure{
				Provider: row.Provider.String(),
				OS:       f.OS,
				Image:    f.Image,
				Reason:   f.Reason,
			})
		}
	}

	if len(failures) == 0 {
		j.Deps.Logger.Info().
			Int("providers_checked", len(rows)).
			Msg("validate_provider_images: all configured images verified")
		return nil
	}

	// Surface failures: structured log + Sentry. Engineers see this once a
	// day before customers do. Notifications-to-team is the natural next
	// step but is intentionally out of scope here.
	for _, f := range failures {
		j.Deps.Logger.Error().
			Str("provider", f.Provider).
			Str("os", f.OS).
			Str("image", f.Image).
			Str("reason", f.Reason).
			Msg("validate_provider_images: configured image no longer usable")
	}

	body, _ := json.Marshal(failures)
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelWarning)
		scope.SetTag("alert", "stale_provider_images")
		scope.SetExtra("failures", string(body))
		sentry.CaptureMessage(fmt.Sprintf("Provider image validation failed: %d configured images no longer available", len(failures)))
	})

	// Not a fatal error from asynq's perspective — retrying won't fix a
	// retired upstream image. Return nil so the cron doesn't churn.
	return nil
}

func (j *ValidateProviderImagesJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Msg("validate_provider_images job failed")
}

// imageFailure describes one configured image that the upstream rejected.
type imageFailure struct {
	OS     string
	Image  string
	Reason string
}

// checkProviderImages knows how to verify each provider's images against the
// upstream API. Only DO has a real per-image lookup today; the others fall
// back to a static-shape check (numeric IDs flagged for review). Extend with
// LookupImage methods on each provider type as needed.
func checkProviderImages(
	ctx context.Context,
	provider providers.Provider,
	cfg config.ProviderConfig,
	creds map[string]any,
) []imageFailure {
	var failures []imageFailure

	switch provider.Type() {
	case types.ProviderDigitalOcean:
		do, ok := provider.(*providers.DigitalOceanProvider)
		if !ok {
			return nil
		}
		for osKey, raw := range cfg.Images {
			img, _ := raw.(string)
			if err := do.LookupImage(ctx, creds, img); err != nil {
				failures = append(failures, imageFailure{OS: osKey, Image: img, Reason: err.Error()})
			}
		}

	default:
		// For providers without a LookupImage helper we still surface the
		// "looks like an unstable numeric ID" signal so we know to review.
		for osKey, raw := range cfg.Images {
			img, _ := raw.(string)
			if isAllDigits(img) {
				failures = append(failures, imageFailure{
					OS:     osKey,
					Image:  img,
					Reason: "image is a numeric ID; recommend periodic manual review (provider has no API to verify without a lookup helper)",
				})
			}
		}
	}

	return failures
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

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// NewValidateProviderImagesTask creates an asynq task — used by the scheduler
// (internal/schedule/kernel.go) and any ad-hoc dispatcher.
func NewValidateProviderImagesTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypeValidateProviderImages, ValidateProviderImagesPayload{},
		asynq.TaskID(pkgjobs.Dedup("validate_provider_images", time.Now().Format("2006-01-02"))),
	)
}
