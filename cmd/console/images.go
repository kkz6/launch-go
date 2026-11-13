// images:validate — live-check every image configured in server/config/options.go
// against the cloud provider's current API.
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

	"gorm.io/gorm"

	appconfig "github.com/kkz6/launch-go/internal/config"
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

	cfg, err := appconfig.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if err := database.InitEncryption(cfg.App.Key); err != nil {
		ctx.Error(fmt.Sprintf("APP_KEY init failed: %v", err))
		return err
	}

	db, err := database.Connect(cfg.Database)
	if err != nil {
		ctx.Error(fmt.Sprintf("connect db: %v", err))
		return err
	}

	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}

	rows, err := imagesLoadProviders(db, providerFilter)
	if err != nil {
		ctx.Error(fmt.Sprintf("query server_providers: %v", err))
		return err
	}
	if len(rows) == 0 {
		ctx.Warning("No matching server providers connected.")
		ctx.Comment("Connect a provider in the UI first, then re-run this command.")
		return fmt.Errorf("no matching provider credentials; no images were verified")
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

	findings := providers.ValidateImages(runCtx, provider, cfg, credsMap)
	for _, finding := range findings {
		ctx.Line(fmt.Sprintf("  %s → %s: %s", finding.OS, finding.Image, finding.Reason))
	}
	if len(findings) != 0 {
		return fmt.Errorf("%d image checks did not pass", len(findings))
	}
	ctx.Line("  ✓ all configured images verified against provider API")
	return nil
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
