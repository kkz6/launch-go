package commands

import (
	"github.com/spf13/cobra"

	"github.com/kkz6/launch-go/cmd/launchctl/client"
)

var (
	flagAPIURL string
	flagTeam   string
	flagFormat string
)

// NewRootCmd creates the root command
func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:     "launchctl",
		Short:   "CLI for the Launch deployment platform",
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if flagAPIURL == "" {
				cfg, err := client.LoadConfig()
				if err == nil && cfg.APIURL != "" {
					flagAPIURL = cfg.APIURL
				}
			}
			if flagTeam == "" {
				cfg, err := client.LoadConfig()
				if err == nil && cfg.DefaultTeamID != "" {
					flagTeam = cfg.DefaultTeamID
				}
			}
		},
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "API base URL (default from config)")
	root.PersistentFlags().StringVar(&flagTeam, "team", "", "Team ID to use (default from config)")
	root.PersistentFlags().StringVar(&flagFormat, "format", "text", "Output format: text or json")

	root.AddCommand(
		newAuthCmd(),
		newServerCmd(),
		newSiteCmd(),
		newTeamCmd(),
		newTokenCmd(),
		newConfigCmd(),
	)

	return root
}

func apiClient() *client.APIClient {
	c := client.NewAPIClient(flagAPIURL)
	if flagTeam != "" {
		c.SetTeam(flagTeam)
	}
	return c
}
