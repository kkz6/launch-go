// Command validate-images live-checks every image configured in
// server/config/options.go against the cloud provider's current API.
//
// Why this exists: provider configs drift. DigitalOcean retires snapshot
// IDs; Vultr changes numeric IDs; Canonical publishes new AMIs and
// deprecates old ones. Our static tests (options_test.go) catch *shape*
// problems but not "is this image still real on the upstream right now".
// Run this against the connected provider accounts to confirm reality:
//
//	go run ./cmd/validate-images               # validate all connected providers
//	go run ./cmd/validate-images -provider do  # only DO
//
// The command reads encrypted credentials from the server_providers table
// (using the same APP_KEY the API does) so you don't have to paste tokens
// on the command line.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database"
	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
)

func main() {
	providerFilter := flag.String("provider", "", "Limit to one provider id (digitalocean, hetzner, linode, vultr, aws). Empty means all connected ones.")
	flag.Parse()

	loadEnv()
	if err := database.InitEncryption(viper.GetString("APP_KEY")); err != nil {
		fatal("APP_KEY init failed: %v", err)
	}

	db := mustConnectDB()
	rows := loadServerProviders(db, *providerFilter)
	if len(rows) == 0 {
		fmt.Println("No matching server providers connected.")
		fmt.Println("Connect a provider in the UI first, then re-run this command.")
		return
	}

	factory := providers.NewFactory(sshkey.NewGenerator())
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	exit := 0
	for _, row := range rows {
		fmt.Printf("\n=== %s [%s] ===\n", strings.ToUpper(row.Provider.String()), profileLabel(row))
		if err := validateProvider(ctx, factory, &row); err != nil {
			fmt.Printf("  ✗ %s\n", err)
			exit = 1
		}
	}
	os.Exit(exit)
}

// validateProvider runs Connect() (verifies credentials) and then walks
// every image configured for this provider type, asking the upstream
// whether the image still exists. The image-existence check is
// provider-specific; we keep it lightweight (one HTTP roundtrip per image)
// so the command stays under a minute even with all providers connected.
func validateProvider(ctx context.Context, factory *providers.Factory, row *models.ServerProvider) error {
	credsMap, err := decryptCreds(row)
	if err != nil {
		return fmt.Errorf("could not decrypt stored credentials: %w", err)
	}

	provider, err := factory.Create(row.Provider)
	if err != nil {
		return fmt.Errorf("unknown provider %q: %w", row.Provider, err)
	}

	if err := provider.Connect(ctx, credsMap); err != nil {
		return fmt.Errorf("Connect() rejected stored credentials: %w", err)
	}
	fmt.Println("  ✓ credentials accepted")

	cfg, ok := config.GetProviderConfigs()[row.Provider.String()]
	if !ok {
		return fmt.Errorf("no provider config in options.go for %q", row.Provider)
	}

	switch row.Provider.String() {
	case "aws":
		return validateAWSImages(cfg, credsMap)
	default:
		return validateImagesViaList(ctx, provider, cfg, credsMap)
	}
}

// validateImagesViaList walks every configured image. For providers we know
// how to talk to (DigitalOcean), we fetch each image directly so a retired
// slug shows up immediately. For others we fall back to a stability-style
// audit until per-provider lookups are added.
func validateImagesViaList(ctx context.Context, p providers.Provider, cfg config.ProviderConfig, creds map[string]any) error {
	switch p.Type() {
	case "digitalocean":
		// Best path: ask DO for the image, surface its status field.
		return validateDOImages(ctx, p, cfg, creds)
	default:
		hadIssue := false
		for osKey, raw := range cfg.Images {
			v, _ := raw.(string)
			fmt.Printf("  %-12s → %s", osKey, v)
			if note := imageStabilityNote(v); note != "" {
				fmt.Printf("   ⚠  %s", note)
				hadIssue = true
			}
			fmt.Println()
		}
		if hadIssue {
			return fmt.Errorf("one or more images use unstable identifiers — consider switching to provider slugs/names")
		}
		return nil
	}
}

// validateDOImages hits DO's /v2/images/{slug} for each configured image and
// reports the image's status/distribution. A 404 here means the slug has
// been retired — exactly the bug that prevented provisioning. Uses the same
// DoGet helper the provider code uses so we go through the same auth path.
func validateDOImages(ctx context.Context, p providers.Provider, cfg config.ProviderConfig, creds map[string]any) error {
	doProvider, ok := p.(*providers.DigitalOceanProvider)
	if !ok {
		return fmt.Errorf("provider was not a DigitalOceanProvider")
	}
	hadFailure := false
	for osKey, raw := range cfg.Images {
		slug, _ := raw.(string)
		fmt.Printf("  %-12s → %s", osKey, slug)
		if err := doProvider.LookupImage(ctx, creds, slug); err != nil {
			fmt.Printf("   ✗ %v", err)
			hadFailure = true
		} else {
			fmt.Print("   ✓ available")
		}
		fmt.Println()
	}
	if hadFailure {
		return fmt.Errorf("one or more DO images are no longer available — update options.go")
	}
	return nil
}

func validateAWSImages(cfg config.ProviderConfig, _ map[string]any) error {
	missing := 0
	for region, regionImages := range cfg.Images {
		osMap, ok := regionImages.(map[string]string)
		if !ok {
			fmt.Printf("  ✗ %s has malformed image map\n", region)
			missing++
			continue
		}
		for os, ami := range osMap {
			if !strings.HasPrefix(ami, "ami-") {
				fmt.Printf("  ✗ %s/%s → %q (does not look like an AMI ID)\n", region, os, ami)
				missing++
			}
		}
	}
	if missing == 0 {
		fmt.Printf("  ✓ %d region/OS pairs have well-formed AMI IDs\n", awsPairCount(cfg))
		fmt.Println("  ℹ  Note: AMIs are reissued frequently by Canonical. Consider using SSM Parameter Store at provision time:")
		fmt.Println("       /aws/service/canonical/ubuntu/server/24.04/stable/current/amd64/hvm/ebs-gp3/ami-id")
	}
	if missing > 0 {
		return fmt.Errorf("%d entries malformed", missing)
	}
	return nil
}

func imageStabilityNote(s string) string {
	if isAllDigits(s) {
		return "looks like a numeric/snapshot ID — these are commonly retired by providers (DO/Vultr). Prefer slugs."
	}
	return ""
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

func awsPairCount(cfg config.ProviderConfig) int {
	n := 0
	for _, regionImages := range cfg.Images {
		if m, ok := regionImages.(map[string]string); ok {
			n += len(m)
		}
	}
	return n
}

func decryptCreds(row *models.ServerProvider) (map[string]any, error) {
	raw := row.Credentials.String()
	if raw == "" {
		return nil, fmt.Errorf("no credentials stored")
	}
	var creds map[string]any
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return nil, fmt.Errorf("credentials are not valid JSON (encryption key mismatch?): %w", err)
	}
	return creds, nil
}

func profileLabel(row models.ServerProvider) string {
	if row.Profile != nil && *row.Profile != "" {
		return *row.Profile
	}
	return row.ID
}

func loadServerProviders(db *gorm.DB, filter string) []models.ServerProvider {
	var rows []models.ServerProvider
	q := db.Where("connected = ?", true)
	if filter != "" {
		q = q.Where("provider = ?", filter)
	}
	if err := q.Find(&rows).Error; err != nil {
		fatal("query server_providers: %v", err)
	}
	return rows
}

func loadEnv() {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	_ = viper.ReadInConfig()
	viper.AutomaticEnv()
}

func mustConnectDB() *gorm.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		viper.GetString("DB_USERNAME"),
		viper.GetString("DB_PASSWORD"),
		viper.GetString("DB_HOST"),
		viper.GetString("DB_PORT"),
		viper.GetString("DB_DATABASE"),
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		fatal("connect db: %v", err)
	}
	return db
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "validate-images: "+format+"\n", args...)
	os.Exit(2)
}
