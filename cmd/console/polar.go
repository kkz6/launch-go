package main

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/billing/providers"
	"github.com/kkz6/launch-go/internal/pkg/console"
	"github.com/kkz6/launch-go/internal/pkg/logger"
)

// polarSetupCommand creates the Polar products for each plan and prints their
// IDs so the operator can set the POLAR_PRODUCT_* env vars. Run once after
// configuring POLAR_ACCESS_TOKEN + POLAR_ORGANIZATION_ID.
type polarSetupCommand struct{}

func (polarSetupCommand) Signature() string { return "billing:polar-setup" }

func (polarSetupCommand) Description() string {
	return "Create the Polar products for each plan and print their IDs"
}

func (polarSetupCommand) Extend() console.Extend {
	return console.Extend{Category: "billing"}
}

// planSpec mirrors the in-code plan catalogue (monthly cents). Kept local so
// the command stays self-contained; pricing matches models.PlansFromConfig.
type planSpec struct {
	name       string
	envVar     string
	priceCents int64
}

func (polarSetupCommand) Handle(ctx console.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Billing.Polar.AccessToken == "" {
		ctx.Error("POLAR_ACCESS_TOKEN is not set. Set it (and POLAR_ORGANIZATION_ID) before running this command.")
		return fmt.Errorf("missing POLAR_ACCESS_TOKEN")
	}

	appLogger := logger.New(cfg.App.Environment)
	client := providers.NewPolarClient(&providers.PolarConfig{
		AccessToken:    cfg.Billing.Polar.AccessToken,
		OrganizationID: cfg.Billing.Polar.OrganizationID,
		Sandbox:        cfg.Billing.Polar.Sandbox,
	}, appLogger)

	env := "production"
	if cfg.Billing.Polar.Sandbox {
		env = "sandbox"
	}
	ctx.Info(fmt.Sprintf("Creating Polar products in the %s environment...", env))
	ctx.NewLine()

	plans := []planSpec{
		{name: "Hobby Plan", envVar: "POLAR_PRODUCT_HOBBY", priceCents: 199},
		{name: "Compact Plan", envVar: "POLAR_PRODUCT_COMPACT", priceCents: 699},
		{name: "Turbo Plan", envVar: "POLAR_PRODUCT_TURBO", priceCents: 2000},
	}

	rows := make([][]string, 0, len(plans))
	bg := context.Background()
	for _, p := range plans {
		id, err := client.CreateProduct(bg, p.name, p.priceCents)
		if err != nil {
			ctx.Error(fmt.Sprintf("Failed to create %q: %v", p.name, err))
			return err
		}
		ctx.Success(fmt.Sprintf("Created %s ($%.2f/mo)", p.name, float64(p.priceCents)/100))
		rows = append(rows, []string{p.name, p.envVar, id})
	}

	ctx.NewLine()
	ctx.Comment("Set these env vars to the product IDs below, then restart the app:")
	ctx.Table([]string{"Plan", "Env var", "Product ID"}, rows)

	return nil
}
