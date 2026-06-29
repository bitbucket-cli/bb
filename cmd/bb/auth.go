package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ThomasRedstone/bb/internal/config"
	"github.com/ThomasRedstone/bb/internal/output"
	"github.com/ThomasRedstone/bb/pkg/auth"
	"github.com/spf13/cobra"
)

// Mockable dependencies for testing
var (
	authGetClientID               = config.GetClientID
	authGetClientSecret           = config.GetClientSecret
	authLogin                     = auth.Login
	authStoreTokens               = auth.StoreTokens
	authLogout                    = auth.Logout
	authGetStatus                 = auth.GetStatus
	authSetClientCreds            = config.SetClientCredentials
	authHasStoredTokens           = auth.HasStoredTokens
	authStdinReader     io.Reader = os.Stdin
)

var (
	authFlagClientID     string
	authFlagClientSecret string
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  `Authenticate with Bitbucket Cloud using OAuth 2.0.`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Bitbucket",
	Long: `Authenticate with Bitbucket Cloud using OAuth 2.0.

You need to create an OAuth consumer in your Bitbucket workspace settings first:
1. Go to Workspace settings > OAuth consumers > Add consumer
2. Set callback URL to: http://localhost:34106/callback
3. Select permissions: Account, Repositories, Pull requests, Pipelines
4. Save and note down the Key (client ID) and Secret

Alternative: Set the BITBUCKET_API_TOKEN environment variable to skip the
OAuth flow entirely:
  • Atlassian API token (the app-password replacement): also set
    BITBUCKET_EMAIL — these are sent as HTTP Basic auth (email:token).
  • Repository/Workspace Access Token: leave BITBUCKET_EMAIL unset — sent
    as a Bearer token.
Note: Bitbucket app passwords are deprecated and being removed; create an
Atlassian API token with scopes instead.`,
	RunE: runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of Bitbucket",
	Long:  `Remove stored authentication tokens.`,
	RunE:  runAuthLogout,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	Long:  `Display the current authentication status.`,
	RunE:  runAuthStatus,
}

func init() {
	authLoginCmd.Flags().StringVar(&authFlagClientID, "client-id", "", "OAuth client ID (Key) — avoid shell history: prefer BB_CLIENT_ID env var")
	authLoginCmd.Flags().StringVar(&authFlagClientSecret, "client-secret", "", "OAuth client secret — avoid shell history: prefer BB_CLIENT_SECRET env var")
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	// Flags take precedence, then env/config, then interactive prompt.
	clientID := authFlagClientID
	if clientID == "" {
		clientID = authGetClientID()
	}
	clientSecret := authFlagClientSecret
	if clientSecret == "" {
		clientSecret = authGetClientSecret()
	}

	// If credentials not configured, prompt for them
	if clientID == "" || clientSecret == "" {
		output.Info("OAuth credentials not found in environment or config file.")
		output.Info("")
		output.Info("To set up OAuth:")
		output.Info("1. Go to Bitbucket > Workspace settings > OAuth consumers")
		output.Info("2. Add consumer with callback URL: http://localhost:34106/callback")
		output.Info("3. Grant permissions: Account, Repositories, Pull requests, Pipelines")
		output.Info("")

		reader := bufio.NewReader(authStdinReader)

		fmt.Print("Enter Client ID (Key): ")
		clientID, _ = reader.ReadString('\n')
		clientID = strings.TrimSpace(clientID)

		fmt.Print("Enter Client Secret: ")
		clientSecret, _ = reader.ReadString('\n')
		clientSecret = strings.TrimSpace(clientSecret)

		if clientID == "" || clientSecret == "" {
			return fmt.Errorf("client ID and secret are required")
		}

		// Save credentials to config
		if err := authSetClientCreds(clientID, clientSecret); err != nil {
			output.Error("Failed to save credentials: %v", err)
			// Continue anyway, they can use env vars
		} else {
			output.Success("Credentials saved to config file")
		}
	}

	// Perform OAuth login
	oauthCfg := auth.OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       auth.DefaultScopes(),
	}

	tokens, err := authLogin(oauthCfg)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Store tokens
	if err := authStoreTokens(tokens); err != nil {
		return fmt.Errorf("failed to store tokens: %w", err)
	}

	output.Success("Logged in to Bitbucket")
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	if !authHasStoredTokens() {
		output.Info("Already logged out")
		return nil
	}
	if err := authLogout(); err != nil {
		output.Info("Already logged out")
		return nil
	}
	output.Success("Logged out of Bitbucket")
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	status, err := authGetStatus()
	if err != nil {
		return err
	}

	if !status.LoggedIn {
		output.Info("Not logged in")
		output.Info("")
		output.Info("Options:")
		output.Info("  • Run 'bb auth login' for OAuth authentication")
		output.Info("  • Set BITBUCKET_API_TOKEN (+ BITBUCKET_EMAIL for an Atlassian API token)")
		return nil
	}

	output.Success("Logged in to Bitbucket")

	switch status.Method {
	case auth.AuthMethodToken:
		if status.Scheme == auth.SchemeBasic {
			output.Info("Method: BITBUCKET_API_TOKEN (Basic auth, BITBUCKET_EMAIL set)")
		} else {
			output.Info("Method: BITBUCKET_API_TOKEN (Bearer token)")
		}
	case auth.AuthMethodOAuth:
		output.Info("Method: OAuth 2.0")
		if status.Scopes != "" {
			output.Info("Scopes: %s", status.Scopes)
		}
	}

	return nil
}
