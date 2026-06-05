// images:validate — live-check every image configured in server/config/options.go
// against the cloud provider's current API. Faithful port of the former
// cmd/validate-images entrypoint.
//
// Provider configs drift (DigitalOcean retires snapshots, Vultr changes IDs,
// Canonical reissues AMIs). Static tests catch shape problems; this confirms
// the images still exist upstream right now. Credentials are read (encrypted)
// from the server_providers table using the same APP_KEY as the API.
package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/kkz6/launch-go/internal/pkg/console"
	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
)

type imagesValidateCommand struct{}

func (imagesValidateCommand) Signature() string { return "images:validate" }
func (imagesValidateCommand) Description() string {
	return "Validate configured provider images against the cloud APIs"
}
func (imagesValidateCommand) Extend() console.Extend {
	return console.Extend{
		Category: "images",
		Flags: []console.Flag{
			console.StringFlag{
				Name:  "provider",
				Usage: "Limit to one provider id (digitalocean, hetzner, linode, vultr, aws). Empty means all connected ones.",
			},
		},
	}
}

func (imagesValidateCommand) Handle(ctx console.Context) error {
	providerFilter := ctx.Option("provider")

	imagesLoadEnv()
	if err := database.InitEncryption(viper.GetString("APP_KEY")); err != nil {
		ctx.Error(fmt.Sprintf("APP_KEY init failed: %v", err))
		return err
	}

	db, err := imagesConnectDB()
	if err != nil {
		ctx.Error(fmt.Sprintf("connect db: %v", err))
		return err
	}

	rows, err := imagesLoadProviders(db, providerFilter)
	if err != nil {
		ctx.Error(fmt.Sprintf("query server_providers: %v", err))
		return err
	}
	if len(rows) == 0 {
		ctx.Warning("No matching server providers connected.")
		ctx.Comment("Connect a provider in the UI first, then re-run this command.")
		return nil
	}

	factory := providers.NewFactory(sshkey.NewGenerator())
	runCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var failed bool
	for i := range rows {
		row := rows[i]
		ctx.NewLine()
		ctx.Info(fmt.Sprintf("=== %s [%s] ===", strings.ToUpper(row.Provider.String()), imagesProfileLabel(row)))
		if err := imagesValidateProvider(runCtx, ctx, factory, &row); err != nil {
			ctx.Error(fmt.Sprintf("  ✗ %v", err))
			failed = true
		}
	}

	if failed {
		return fmt.Errorf("one or more providers reported image issues")
	}
	return nil
}

func imagesValidateProvider(runCtx context.Context, ctx console.Context, factory *providers.Factory, row *models.ServerProvider) error {
	credsMap, err := imagesDecryptCreds(row)
	if err != nil {
		return fmt.Errorf("could not decrypt stored credentials: %w", err)
	}

	provider, err := factory.Create(row.Provider)
	if err != nil {
		return fmt.Errorf("unknown provider %q: %w", row.Provider, err)
	}

	if err := provider.Connect(runCtx, credsMap); err != nil {
		return fmt.Errorf("Connect() rejected stored credentials: %w", err)
	}
	ctx.Line("  ✓ credentials accepted")

	cfg, ok := config.GetProviderConfigs()[row.Provider.String()]
	if !ok {
		return fmt.Errorf("no provider config in options.go for %q", row.Provider)
	}

	if row.Provider.String() == "aws" {
		return imagesValidateAWS(ctx, cfg)
	}
	return imagesValidateViaList(runCtx, ctx, provider, cfg, credsMap)
}

func imagesValidateViaList(runCtx context.Context, ctx console.Context, p providers.Provider, cfg config.ProviderConfig, creds map[string]any) error {
	if p.Type() == "digitalocean" {
		return imagesValidateDO(runCtx, ctx, p, cfg, creds)
	}

	hadIssue := false
	for osKey, raw := range cfg.Images {
		v, _ := raw.(string)
		line := fmt.Sprintf("  %-12s → %s", osKey, v)
		if note := imagesStabilityNote(v); note != "" {
			line += "   ⚠  " + note
			hadIssue = true
		}
		ctx.Line(line)
	}
	if hadIssue {
		return fmt.Errorf("one or more images use unstable identifiers — consider switching to provider slugs/names")
	}
	return nil
}

func imagesValidateDO(runCtx context.Context, ctx console.Context, p providers.Provider, cfg config.ProviderConfig, creds map[string]any) error {
	doProvider, ok := p.(*providers.DigitalOceanProvider)
	if !ok {
		return fmt.Errorf("provider was not a DigitalOceanProvider")
	}
	hadFailure := false
	for osKey, raw := range cfg.Images {
		slug, _ := raw.(string)
		if err := doProvider.LookupImage(runCtx, creds, slug); err != nil {
			ctx.Line(fmt.Sprintf("  %-12s → %s   ✗ %v", osKey, slug, err))
			hadFailure = true
		} else {
			ctx.Line(fmt.Sprintf("  %-12s → %s   ✓ available", osKey, slug))
		}
	}
	if hadFailure {
		return fmt.Errorf("one or more DO images are no longer available — update options.go")
	}
	return nil
}

func imagesValidateAWS(ctx console.Context, cfg config.ProviderConfig) error {
	missing := 0
	for region, regionImages := range cfg.Images {
		osMap, ok := regionImages.(map[string]string)
		if !ok {
			ctx.Line(fmt.Sprintf("  ✗ %s has malformed image map", region))
			missing++
			continue
		}
		for osName, ami := range osMap {
			if !strings.HasPrefix(ami, "ami-") {
				ctx.Line(fmt.Sprintf("  ✗ %s/%s → %q (does not look like an AMI ID)", region, osName, ami))
				missing++
			}
		}
	}
	if missing == 0 {
		ctx.Line(fmt.Sprintf("  ✓ %d region/OS pairs have well-formed AMI IDs", imagesAWSPairCount(cfg)))
		ctx.Comment("  ℹ  AMIs are reissued frequently by Canonical. Consider using SSM Parameter Store at provision time:")
		ctx.Comment("       /aws/service/canonical/ubuntu/server/24.04/stable/current/amd64/hvm/ebs-gp3/ami-id")
		return nil
	}
	return fmt.Errorf("%d entries malformed", missing)
}

func imagesStabilityNote(s string) string {
	if imagesIsAllDigits(s) {
		return "looks like a numeric/snapshot ID — these are commonly retired by providers (DO/Vultr). Prefer slugs."
	}
	return ""
}

func imagesIsAllDigits(s string) bool {
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

func imagesAWSPairCount(cfg config.ProviderConfig) int {
	n := 0
	for _, regionImages := range cfg.Images {
		if m, ok := regionImages.(map[string]string); ok {
			n += len(m)
		}
	}
	return n
}

func imagesDecryptCreds(row *models.ServerProvider) (map[string]any, error) {
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

func imagesProfileLabel(row models.ServerProvider) string {
	if row.Profile != nil && *row.Profile != "" {
		return *row.Profile
	}
	return row.ID
}

func imagesLoadProviders(db *gorm.DB, filter string) ([]models.ServerProvider, error) {
	var rows []models.ServerProvider
	q := db.Where("connected = ?", true)
	if filter != "" {
		q = q.Where("provider = ?", filter)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func imagesLoadEnv() {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	_ = viper.ReadInConfig()
	viper.AutomaticEnv()
}

func imagesConnectDB() (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		viper.GetString("DB_USERNAME"),
		viper.GetString("DB_PASSWORD"),
		viper.GetString("DB_HOST"),
		viper.GetString("DB_PORT"),
		viper.GetString("DB_DATABASE"),
	)
	return gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}
