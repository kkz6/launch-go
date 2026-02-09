package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Server management commands",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List servers",
			RunE:  runServerList,
		},
		&cobra.Command{
			Use:   "show [id]",
			Short: "Show server details",
			Args:  cobra.ExactArgs(1),
			RunE:  runServerShow,
		},
	)

	return cmd
}

func runServerList(cmd *cobra.Command, args []string) error {
	api := apiClient()
	resp, err := api.Do(cmd.Context(), "GET", "/servers", nil)
	if err != nil {
		return err
	}

	if flagFormat == "json" {
		fmt.Println(string(resp.Data))
		return nil
	}

	var servers []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		IP     string `json:"ip_address"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Data, &servers); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(servers) == 0 {
		fmt.Println("No servers found.")
		return nil
	}

	fmt.Printf("%-28s %-20s %-16s %s\n", "ID", "NAME", "IP", "STATUS")
	for _, s := range servers {
		fmt.Printf("%-28s %-20s %-16s %s\n", s.ID, s.Name, s.IP, s.Status)
	}

	return nil
}

func runServerShow(cmd *cobra.Command, args []string) error {
	api := apiClient()
	resp, err := api.Do(cmd.Context(), "GET", "/servers/"+args[0], nil)
	if err != nil {
		return err
	}

	if flagFormat == "json" {
		fmt.Println(string(resp.Data))
		return nil
	}

	var server struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		IP       string `json:"ip_address"`
		Status   string `json:"status"`
		Provider string `json:"provider"`
		Region   string `json:"region"`
		OS       string `json:"os"`
	}
	if err := json.Unmarshal(resp.Data, &server); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("ID:       %s\n", server.ID)
	fmt.Printf("Name:     %s\n", server.Name)
	fmt.Printf("IP:       %s\n", server.IP)
	fmt.Printf("Status:   %s\n", server.Status)
	fmt.Printf("Provider: %s\n", server.Provider)
	fmt.Printf("Region:   %s\n", server.Region)

	return nil
}
