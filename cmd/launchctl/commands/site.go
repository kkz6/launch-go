package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newSiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "site",
		Short: "Site management commands",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List sites",
			RunE:  runSiteList,
		},
		&cobra.Command{
			Use:   "show [id]",
			Short: "Show site details",
			Args:  cobra.ExactArgs(1),
			RunE:  runSiteShow,
		},
		&cobra.Command{
			Use:   "deploy [id]",
			Short: "Trigger a deployment",
			Args:  cobra.ExactArgs(1),
			RunE:  runSiteDeploy,
		},
	)

	return cmd
}

func runSiteList(cmd *cobra.Command, args []string) error {
	api := apiClient()
	resp, err := api.Do(cmd.Context(), "GET", "/sites", nil)
	if err != nil {
		return err
	}

	if flagFormat == "json" {
		fmt.Println(string(resp.Data))
		return nil
	}

	var sites []struct {
		ID     string `json:"id"`
		Domain string `json:"domain"`
		Type   string `json:"type"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Data, &sites); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(sites) == 0 {
		fmt.Println("No sites found.")
		return nil
	}

	fmt.Printf("%-28s %-30s %-12s %s\n", "ID", "DOMAIN", "TYPE", "STATUS")
	for _, s := range sites {
		fmt.Printf("%-28s %-30s %-12s %s\n", s.ID, s.Domain, s.Type, s.Status)
	}

	return nil
}

func runSiteShow(cmd *cobra.Command, args []string) error {
	api := apiClient()
	resp, err := api.Do(cmd.Context(), "GET", "/sites/"+args[0], nil)
	if err != nil {
		return err
	}

	if flagFormat == "json" {
		fmt.Println(string(resp.Data))
		return nil
	}

	var site struct {
		ID         string `json:"id"`
		Domain     string `json:"domain"`
		Type       string `json:"type"`
		Status     string `json:"status"`
		Repository string `json:"repository"`
		Branch     string `json:"branch"`
	}
	if err := json.Unmarshal(resp.Data, &site); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("ID:         %s\n", site.ID)
	fmt.Printf("Domain:     %s\n", site.Domain)
	fmt.Printf("Type:       %s\n", site.Type)
	fmt.Printf("Status:     %s\n", site.Status)
	fmt.Printf("Repository: %s\n", site.Repository)
	fmt.Printf("Branch:     %s\n", site.Branch)

	return nil
}

func runSiteDeploy(cmd *cobra.Command, args []string) error {
	api := apiClient()
	resp, err := api.Do(cmd.Context(), "POST", "/sites/"+args[0]+"/deploy", nil)
	if err != nil {
		return err
	}

	fmt.Println(resp.Message)
	return nil
}
