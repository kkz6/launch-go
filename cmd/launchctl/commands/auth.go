package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kkz6/launch-go/cmd/launchctl/client"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a Personal Access Token",
		Long:  "Authenticate with Launch using a Personal Access Token (PAT).\nCreate a token at your account settings page under Security > Personal Access Tokens.",
		RunE:  runLogin,
	}
	loginCmd.Flags().String("token", "", "Personal access token (reads from stdin if not provided)")

	cmd.AddCommand(
		loginCmd,
		&cobra.Command{
			Use:   "logout",
			Short: "Sign out and clear credentials",
			RunE:  runLogout,
		},
		&cobra.Command{
			Use:   "status",
			Short: "Show current authentication status",
			RunE:  runStatus,
		},
	)

	return cmd
}

func runLogin(cmd *cobra.Command, args []string) error {
	token, _ := cmd.Flags().GetString("token")

	if token == "" {
		fmt.Print("Enter your Personal Access Token: ")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read token: %w", err)
		}
		token = strings.TrimSpace(line)
	}

	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	if err := client.SaveCredentials(&client.Credentials{Token: token}); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	// Verify the token works
	api := apiClient()
	_, err := api.Do(cmd.Context(), "GET", "/auth/user", nil)
	if err != nil {
		// Clear invalid credentials
		_ = client.ClearCredentials()
		return fmt.Errorf("token validation failed: %w", err)
	}

	fmt.Println("Authenticated successfully!")
	return nil
}

func runLogout(cmd *cobra.Command, args []string) error {
	if err := client.ClearCredentials(); err != nil {
		return fmt.Errorf("failed to clear credentials: %w", err)
	}

	fmt.Println("Logged out successfully.")
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	creds, err := client.LoadCredentials()
	if err != nil {
		return fmt.Errorf("failed to read credentials: %w", err)
	}
	if creds == nil || creds.Token == "" {
		fmt.Println("Not authenticated. Run 'launchctl auth login' to sign in.")
		return nil
	}

	// Verify the token is still valid
	api := apiClient()
	_, err = api.Do(cmd.Context(), "GET", "/auth/user", nil)
	if err != nil {
		fmt.Println("Status: Token invalid or expired")
		return nil
	}

	fmt.Println("Status: Authenticated")
	return nil
}
