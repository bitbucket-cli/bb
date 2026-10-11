package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

// Mockable dependency for testing.
var userNewAPIClient = api.NewClient

type bbUser struct {
	DisplayName string `json:"display_name"`
	Nickname    string `json:"nickname"`
	UUID        string `json:"uuid"`
	AccountID   string `json:"account_id"`
}

var userCmd = &cobra.Command{
	Use:   "user [selected-user]",
	Short: "Show the current user, or another user's profile",
	Long: `With no argument, show the authenticated user (whoami). With a UUID or
account_id, show that user's public profile.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUser,
}

func init() {
	rootCmd.AddCommand(userCmd)
}

func runUser(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := userNewAPIClient()
	if err != nil {
		return err
	}

	var resp *http.Response
	if len(args) == 1 {
		resp, err = client.GetUsersSelectedUser(ctx, args[0])
	} else {
		resp, err = client.GetUser(ctx)
	}
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var u bbUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	if output.CurrentFormat == output.FormatJSON {
		return output.JSON(u)
	}
	fmt.Printf("Name:       %s\n", u.DisplayName)
	if u.Nickname != "" {
		fmt.Printf("Nickname:   %s\n", u.Nickname)
	}
	fmt.Printf("UUID:       %s\n", u.UUID)
	if u.AccountID != "" {
		fmt.Printf("Account ID: %s\n", u.AccountID)
	}
	return nil
}
