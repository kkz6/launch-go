package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kkz6/launch-go/cmd/launchctl/client"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "CLI configuration",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "set [key] [value]",
			Short: "Set a configuration value",
			Long:  "Available keys: api_url, default_team_id",
			Args:  cobra.ExactArgs(2),
			RunE:  runConfigSet,
		},
		&cobra.Command{
			Use:   "get [key]",
			Short: "Get a configuration value",
			Long:  "Available keys: api_url, default_team_id",
			Args:  cobra.ExactArgs(1),
			RunE:  runConfigGet,
		},
	)

	return cmd
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	cfg, err := client.LoadConfig()
	if err != nil {
		return err
	}

	switch args[0] {
	case "api_url":
		cfg.APIURL = args[1]
	case "default_team_id":
		cfg.DefaultTeamID = args[1]
	default:
		return fmt.Errorf("unknown config key: %s", args[0])
	}

	if err := client.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Set %s = %s\n", args[0], args[1])
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	cfg, err := client.LoadConfig()
	if err != nil {
		return err
	}

	switch args[0] {
	case "api_url":
		fmt.Println(cfg.APIURL)
	case "default_team_id":
		if cfg.DefaultTeamID == "" {
			fmt.Println("(not set)")
		} else {
			fmt.Println(cfg.DefaultTeamID)
		}
	default:
		return fmt.Errorf("unknown config key: %s", args[0])
	}

	return nil
}
