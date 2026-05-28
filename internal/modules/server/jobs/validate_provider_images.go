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

	// Track results per provider-type so we only report each unique
	// (provider, image) failure once per run even if multiple accounts
	// surface the same broken slug. Carry team_id alongside so we can
	// route notifications to the team owners whose server_providers
	// row was actually affected (different teams may have credentials
	// for the same provider; only the teams hitting failures get paged).
	type failure struct {
		TeamID   string
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

		teamID := ""
		if row.TeamID != nil {
			teamID = *row.TeamID
		}
		for _, f := range checkProviderImages(ctx, provider, cfg, creds) {
			failures = append(failures, failure{
				TeamID:   teamID,
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

	// Surface failures: structured log + Sentry + per-team notification.
	for _, f := range failures {
		j.Deps.Logger.Error().
			Str("team_id", f.TeamID).
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

// imageFailure describes one configured image that the upstream rejected.
type imageFailure struct {
	OS     string
	Image  string
	Reason string
}

// imageLookup is the live-verification subset of the Provider interface.
// Each provider implementation that can validate an image against its
// upstream API satisfies this. checkProviderImages branches on it so we
// don't have to switch on provider.Type() for every new provider that
// gains a LookupImage method.
type imageLookup interface {
	LookupImage(ctx context.Context, credentials map[string]any, image string) error
}

// checkProviderImages verifies each provider's configured images against
// the upstream API. Providers that implement imageLookup get a real
// per-image check; the rest fall through to the "looks like an unstable
// numeric ID" static signal so we still know to manually review.
//
// AWS is a special case: its config.Images map is region->{os->ami_id}
// (and the static AMI IDs drift weekly as Canonical publishes patches),
// so we resolve through SSM Parameter Store instead. The legacy static
// map is still checked as a defence-in-depth so a region we haven't yet
// migrated to SSM gets the same alerting.
func checkProviderImages(
	ctx context.Context,
	provider providers.Provider,
	cfg config.ProviderConfig,
	creds map[string]any,
) []imageFailure {
	var failures []imageFailure

	// AWS has the region->os->ami nested-map shape and SSM-parameter
	// resolution that doesn't fit the simple imageLookup contract.
	if aws, ok := provider.(*providers.AWSProvider); ok {
		return checkAWSImages(ctx, aws, cfg, creds)
	}

	if lookuper, ok := provider.(imageLookup); ok {
		for osKey, raw := range cfg.Images {
			img, _ := raw.(string)
			if err := lookuper.LookupImage(ctx, creds, img); err != nil {
				failures = append(failures, imageFailure{OS: osKey, Image: img, Reason: err.Error()})
			}
		}
		return failures
	}

	// No LookupImage available — flag numeric-shape IDs for review.
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
	return failures
}

// checkAWSImages handles AWS's region->{os->ami} shape. For each
// configured OS, it first tries the SSM parameter (the canonical Ubuntu
// "always-current AMI" pointer) — if Canonical removes a release, that
// fails loudly. As defence-in-depth, it also verifies the static AMI
// IDs in each region; those drift week-over-week, so any deregistered
// AMI in any region surfaces here before a customer hits it during a
// provision in that region.
func checkAWSImages(
	ctx context.Context,
	aws *providers.AWSProvider,
	cfg config.ProviderConfig,
	creds map[string]any,
) []imageFailure {
	var failures []imageFailure

	// Pass 1: resolve each OS through its SSM parameter.
	for osKey := range providers.AWSSSMParameterByOS {
		if _, err := aws.ResolveSSMImage(ctx, creds, osKey); err != nil {
			failures = append(failures, imageFailure{
				OS:     osKey,
				Image:  providers.AWSSSMParameterByOS[osKey],
				Reason: err.Error(),
			})
		}
	}

	// Pass 2: validate the static AMI map's IDs region-by-region.
	// cfg.Images for AWS is map[region]map[osKey]amiID. We're tolerant
	// of mixed shapes (raw could be a string OR a map) since the same
	// validation framework runs against other providers' flat maps.
	for region, raw := range cfg.Images {
		osMap, ok := raw.(map[string]string)
		if !ok {
			continue
		}
		credsForRegion := cloneCreds(creds)
		credsForRegion["region"] = region
		for osKey, amiID := range osMap {
			if err := aws.LookupImage(ctx, credsForRegion, amiID); err != nil {
				failures = append(failures, imageFailure{
					OS:     osKey + " (" + region + ")",
					Image:  amiID,
					Reason: err.Error(),
				})
			}
		}
	}

	return failures
}

// cloneCreds returns a shallow copy of the credentials map with a
// patched region field so the AWS provider hits the right regional
// endpoint without us mutating the caller's map.
func cloneCreds(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
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
