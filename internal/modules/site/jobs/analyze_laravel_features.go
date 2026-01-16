package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
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
	ctx     *JobContext
	Payload AnalyzeLaravelFeaturesPayload
}

// NewAnalyzeLaravelFeaturesJob creates a new AnalyzeLaravelFeaturesJob
func NewAnalyzeLaravelFeaturesJob(ctx *JobContext, payload AnalyzeLaravelFeaturesPayload) *AnalyzeLaravelFeaturesJob {
	return &AnalyzeLaravelFeaturesJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// ComposerJSON represents the structure of composer.json
type ComposerJSON struct {
	Require    map[string]string `json:"require"`
	RequireDev map[string]string `json:"require-dev"`
}

// featurePackageMapping maps Laravel features to their composer packages
var featurePackageMapping = map[string][]string{
	"horizon": {"laravel/horizon"},
	"octane":  {"laravel/octane"},
	"reverb":  {"laravel/reverb"},
	"inertia": {"inertiajs/inertia-laravel"},
	"scout":   {"laravel/scout"},
	"pulse":   {"laravel/pulse"},
	"pennant": {"laravel/pennant"},
	"cashier": {"laravel/cashier", "laravel/cashier-stripe", "laravel/cashier-paddle"},
	"spark":   {"laravel/spark-stripe", "laravel/spark-paddle"},
	"nova":    {"laravel/nova"},
	"vapor":   {"laravel/vapor-core", "laravel/vapor-cli"},
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
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Only analyze Laravel sites
	if site.Type != enums.SiteTypeLaravel {
		j.ctx.LogInfo("Site is not Laravel, skipping feature analysis", "site_id", site.ID, "type", site.Type)
		return nil
	}

	// Get server
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Analyzing Laravel features",
		"site_id", site.ID,
		"server_id", server.ID,
	)

	// Read composer.json from the site
	task := tasks.ReadComposerJSON(site.GetApplicationDirectory())
	result, err := j.ctx.RunTaskOnServer(server, task).AsUser(site.User).Dispatch(ctx)
	if err != nil {
		j.ctx.LogError(err, "Failed to read composer.json", "site_id", site.ID)
		return nil // Don't fail the job, just log and return
	}

	output := strings.TrimSpace(result.GetOutput())
	if output == "" || output == "{}" {
		j.ctx.LogInfo("No composer.json found or empty", "site_id", site.ID)
		return nil
	}

	// Parse composer.json
	var composer ComposerJSON
	if err := json.Unmarshal([]byte(output), &composer); err != nil {
		j.ctx.LogError(err, "Failed to parse composer.json", "site_id", site.ID)
		return nil // Don't fail the job
	}

	// Detect features
	detectedFeatures := j.detectFeatures(composer)

	j.ctx.LogInfo("Detected Laravel features",
		"site_id", site.ID,
		"features", detectedFeatures,
	)

	// Update site's features field
	if len(detectedFeatures) > 0 {
		// JSON marshal for MySQL JSON column (GORM Updates with map bypasses model serializers)
		featuresJSON, _ := json.Marshal(detectedFeatures)
		if err := j.ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
			"features": string(featuresJSON),
		}); err != nil {
			j.ctx.LogError(err, "Failed to update site features", "site_id", site.ID)
			return err
		}
	}

	// Broadcast update
	j.ctx.BroadcastServerEvent(server, "site.features_analyzed", map[string]interface{}{
		"site_id":  site.ID,
		"features": detectedFeatures,
	})

	j.ctx.LogInfo("Laravel feature analysis completed",
		"site_id", site.ID,
		"feature_count", len(detectedFeatures),
	)

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
	j.ctx.LogError(err, "Failed to analyze Laravel features",
		"site_id", j.Payload.SiteID,
	)
}

// NewAnalyzeLaravelFeaturesTask creates an analyze Laravel features task
func NewAnalyzeLaravelFeaturesTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeAnalyzeLaravelFeatures, AnalyzeLaravelFeaturesPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
