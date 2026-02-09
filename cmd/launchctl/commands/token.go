package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Personal Access Token management",
	}

	createCmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new personal access token",
		Args:  cobra.ExactArgs(1),
		RunE:  runTokenCreate,
	}
	createCmd.Flags().StringSlice("scopes", []string{"*"}, "Token scopes")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List personal access tokens",
			RunE:  runTokenList,
		},
		createCmd,
		&cobra.Command{
			Use:   "revoke [id]",
			Short: "Revoke a personal access token",
			Args:  cobra.ExactArgs(1),
			RunE:  runTokenRevoke,
		},
	)

	return cmd
}

func runTokenList(cmd *cobra.Command, args []string) error {
	api := apiClient()
	resp, err := api.Do(cmd.Context(), "GET", "/user/tokens", nil)
	if err != nil {
		return err
	}

	if flagFormat == "json" {
		fmt.Println(string(resp.Data))
		return nil
	}

	var tokens []struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		LastUsed  *string `json:"last_used_at"`
		ExpiresAt *string `json:"expires_at"`
	}
	if err := json.Unmarshal(resp.Data, &tokens); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(tokens) == 0 {
		fmt.Println("No tokens found.")
		return nil
	}

	fmt.Printf("%-28s %-25s %-20s %s\n", "ID", "NAME", "LAST USED", "EXPIRES")
	for _, t := range tokens {
		lastUsed := "Never"
		if t.LastUsed != nil {
			lastUsed = *t.LastUsed
		}
		expires := "Never"
		if t.ExpiresAt != nil {
			expires = *t.ExpiresAt
		}
		fmt.Printf("%-28s %-25s %-20s %s\n", t.ID, t.Name, lastUsed, expires)
	}

	return nil
}

func runTokenCreate(cmd *cobra.Command, args []string) error {
	scopes, _ := cmd.Flags().GetStringSlice("scopes")

	api := apiClient()
	resp, err := api.Do(cmd.Context(), "POST", "/user/tokens", map[string]any{
		"name":   args[0],
		"scopes": scopes,
	})
	if err != nil {
		return err
	}

	var result struct {
		PlainTextToken string `json:"plain_text_token"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Println("Token created successfully!")
	fmt.Printf("\n  %s\n\n", result.PlainTextToken)
	fmt.Println("Make sure to copy this token - you won't be able to see it again.")

	return nil
}

func runTokenRevoke(cmd *cobra.Command, args []string) error {
	api := apiClient()
	_, err := api.Do(cmd.Context(), "DELETE", "/user/tokens/"+args[0], nil)
	if err != nil {
		return err
	}

	fmt.Println("Token revoked successfully.")
	return nil
}
