package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeAnalyzeLaravelFeatures = "site:analyze_laravel_features"

// AnalyzeLaravelFeaturesPayload holds data for Laravel feature analysis
type AnalyzeLaravelFeaturesPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// AnalyzeLaravelFeaturesJob analyzes a Laravel site to detect available features
type AnalyzeLaravelFeaturesJob struct {
	Deps    *JobDeps
	Payload AnalyzeLaravelFeaturesPayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewAnalyzeLaravelFeaturesJob creates a new AnalyzeLaravelFeaturesJob
func NewAnalyzeLaravelFeaturesJob(p AnalyzeLaravelFeaturesPayload) pkgjobs.Handler {
	return &AnalyzeLaravelFeaturesJob{Deps: deps, Payload: p}
}

// ComposerJSON represents the structure of composer.json
type ComposerJSON struct {
	Require    map[string]string `json:"require"`
	RequireDev map[string]string `json:"require-dev"`
}

// featurePackageMapping maps Laravel features to their composer packages
var featurePackageMapping = map[string][]string{
	"horizon":   {"laravel/horizon"},
	"octane":    {"laravel/octane"},
	"reverb":    {"laravel/reverb"},
	"inertia":   {"inertiajs/inertia-laravel"},
	"scout":     {"laravel/scout"},
	"pulse":     {"laravel/pulse"},
	"pennant":   {"laravel/pennant"},
	"cashier":   {"laravel/cashier", "laravel/cashier-stripe", "laravel/cashier-paddle"},
	"spark":     {"laravel/spark-stripe", "laravel/spark-paddle"},
	"nova":      {"laravel/nova"},
	"vapor":     {"laravel/vapor-core", "laravel/vapor-cli"},
	"jetstream": {"laravel/jetstream"},
	"breeze":    {"laravel/breeze"},
	"sanctum":   {"laravel/sanctum"},
	"passport":  {"laravel/passport"},
	"socialite": {"laravel/socialite"},
	"telescope": {"laravel/telescope"},
	"dusk":      {"laravel/dusk"},
}

// Handle executes the analyze features job
func (j *AnalyzeLaravelFeaturesJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	// Only analyze Laravel sites
	if site.Type != sitetypes.SiteTypeLaravel {
		j.Deps.Logger.Info().Str("site_id", site.ID).Str("type", string(site.Type)).Msg("Site is not Laravel, skipping feature analysis")
		return nil
	}

	// Get server
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	j.Deps.Logger.Info().Str("site_id", site.ID).Str("server_id", server.ID).Msg("Analyzing Laravel features")

	// Read composer.json from the site
	task := tasks.ReadComposerJSON(site.GetApplicationDirectory())
	result, err := j.Deps.RunTask(server, task).AsUser(site.User).Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Str("site_id", site.ID).Msg("Failed to read composer.json")
		return nil // Don't fail the job, just log and return
	}

	output := strings.TrimSpace(result.GetOutput())
	if output == "" || output == "{}" {
		j.Deps.Logger.Info().Str("site_id", site.ID).Msg("No composer.json found or empty")
		return nil
	}

	// Parse composer.json
	var composer ComposerJSON
	if err := json.Unmarshal([]byte(output), &composer); err != nil {
		j.Deps.Logger.Error().Err(err).Str("site_id", site.ID).Msg("Failed to parse composer.json")
		return nil // Don't fail the job
	}

	// Detect features
	detectedFeatures := j.detectFeatures(composer)

	j.Deps.Logger.Info().Str("site_id", site.ID).Strs("features", detectedFeatures).Msg("Detected Laravel features")

	// Update site's features field
	if len(detectedFeatures) > 0 {
		// JSON marshal for MySQL JSON column (GORM Updates with map bypasses model serializers)
		featuresJSON, _ := json.Marshal(detectedFeatures)
		if err := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
			"features": string(featuresJSON),
		}); err != nil {
			j.Deps.Logger.Error().Err(err).Str("site_id", site.ID).Msg("Failed to update site features")
			return err
		}
	}

	// Broadcast update
	j.Deps.BroadcastServerEvent(server, "site.features_analyzed", map[string]interface{}{
		"site_id":  site.ID,
		"features": detectedFeatures,
	})

	j.Deps.Logger.Info().Str("site_id", site.ID).Int("feature_count", len(detectedFeatures)).Msg("Laravel feature analysis completed")

	return nil
}

// detectFeatures checks composer packages against known feature packages
func (j *AnalyzeLaravelFeaturesJob) detectFeatures(composer ComposerJSON) []string {
	detected := make([]string, 0)

	// Merge require and require-dev
	allPackages := make(map[string]bool)
	for pkg := range composer.Require {
		allPackages[pkg] = true
	}
	for pkg := range composer.RequireDev {
		allPackages[pkg] = true
	}

	// Check each feature
	for feature, packages := range featurePackageMapping {
		for _, pkg := range packages {
			if allPackages[pkg] {
				detected = append(detected, feature)
				break // Found one package for this feature, move to next feature
			}
		}
	}

	return detected
}

// Failed handles job failure
func (j *AnalyzeLaravelFeaturesJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Failed to analyze Laravel features")
}

// NewAnalyzeLaravelFeaturesTask creates an analyze Laravel features task
// Uses TaskID for deduplication to prevent the same site from being analyzed multiple times
func NewAnalyzeLaravelFeaturesTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeAnalyzeLaravelFeatures, AnalyzeLaravelFeaturesPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	}, asynq.TaskID(pkgjobs.Dedup("analyze_features", siteID)))
}
