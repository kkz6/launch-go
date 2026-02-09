package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kkz6/launch-go/cmd/launchctl/client"
)

func newTeamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Team management commands",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List teams",
			RunE:  runTeamList,
		},
		&cobra.Command{
			Use:   "switch [id]",
			Short: "Switch active team",
			Args:  cobra.ExactArgs(1),
			RunE:  runTeamSwitch,
		},
		&cobra.Command{
			Use:   "current",
			Short: "Show current team",
			RunE:  runTeamCurrent,
		},
	)

	return cmd
}

func runTeamList(cmd *cobra.Command, args []string) error {
	api := apiClient()
	resp, err := api.Do(cmd.Context(), "GET", "/auth/teams", nil)
	if err != nil {
		return err
	}

	if flagFormat == "json" {
		fmt.Println(string(resp.Data))
		return nil
	}

	var teams []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(resp.Data, &teams); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(teams) == 0 {
		fmt.Println("No teams found.")
		return nil
	}

	cfg, _ := client.LoadConfig()
	currentTeam := ""
	if cfg != nil {
		currentTeam = cfg.DefaultTeamID
	}

	fmt.Printf("%-28s %s\n", "ID", "NAME")
	for _, t := range teams {
		marker := "  "
		if t.ID == currentTeam {
			marker = "* "
		}
		fmt.Printf("%s%-28s %s\n", marker, t.ID, t.Name)
	}

	return nil
}

func runTeamSwitch(cmd *cobra.Command, args []string) error {
	cfg, err := client.LoadConfig()
	if err != nil {
		return err
	}

	cfg.DefaultTeamID = args[0]
	if err := client.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Switched to team %s\n", args[0])
	return nil
}

func runTeamCurrent(cmd *cobra.Command, args []string) error {
	cfg, err := client.LoadConfig()
	if err != nil {
		return err
	}

	if cfg.DefaultTeamID == "" {
		fmt.Println("No team selected. Run 'launchctl team switch <id>' to set one.")
		return nil
	}

	fmt.Printf("Current team: %s\n", cfg.DefaultTeamID)
	return nil
}
