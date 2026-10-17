package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bitbucket-cli/bb/internal/config"
	"github.com/bitbucket-cli/bb/internal/output"
	"github.com/bitbucket-cli/bb/pkg/auth"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Mockable dependencies for testing
var (
	authGetClientID                   = config.GetClientID
	authGetClientSecret               = config.GetClientSecret
	authLogin                         = auth.Login
	authStoreTokens                   = auth.StoreTokens
	authLogout                        = auth.Logout
	authGetStatus                     = auth.GetStatus
	authSetClientCreds                = config.SetClientCredentials
	authHasStoredTokens               = auth.HasStoredTokens
	authRefreshTokens                 = auth.RefreshAccessToken
	authGetScopes                     = config.GetScopes
	authSetScopes                     = config.SetScopes
	authBuildAuthorizeURL             = auth.BuildAuthorizeURL
	authExchangeCallbackURL           = auth.ExchangeCallbackURL
	authStdinReader         io.Reader = os.Stdin
)

var (
	authFlagClientID     string
	authFlagClientSecret string
	authFlagScopes       string
	authFlagManual       bool
	authFlagCallbackURL  string
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
3. Grant the permissions you want this CLI to have (see Scopes below)
4. Save and note down the Key (client ID) and Secret

Scopes (--scopes):
  Choose a level — read | write | admin — or pass an explicit comma-separated
  list. Default is "write". An explicitly chosen level is saved to the config
  and reused on the next login.
    read   account + read-only repository, pull request, pipeline access
    write  read + repository/pull request/pipeline write
    admin  write + repository:admin (required to CREATE repositories)

  IMPORTANT: the requested scopes do NOT narrow the issued token. Bitbucket
  always grants the access token the OAuth consumer's full configured
  permissions; the request is only validated against the consumer and fails if
  it asks for a scope the consumer wasn't granted. So the real least-privilege
  control is which permissions you grant the consumer in step 3 — pick a level
  here that matches it, and login will fail fast if they don't line up.

Over SSH / no browser (--manual):
  The callback normally lands on a local server (http://localhost:34106). Over
  SSH that server is on the remote host, so the redirect can't reach it. Use
  --manual: bb prints the authorize URL, you open it in any browser, and after
  authorizing you paste the URL you were redirected to (the localhost page won't
  load — just copy its address from the browser). bb extracts the code from it.
  Or pass it non-interactively with --callback-url "<pasted-url-or-code>".

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

var authRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh the OAuth access token",
	Long: `Exchange the stored refresh token for a new access token.

Useful to verify silent refresh works, to force a fresh token before a long
operation, or to record an expiry on a token stored by an older version. Not
applicable when authenticating via BITBUCKET_API_TOKEN.`,
	RunE: runAuthRefresh,
}

func init() {
	authLoginCmd.Flags().StringVar(&authFlagClientID, "client-id", "", "OAuth client ID (Key) — avoid shell history: prefer BB_CLIENT_ID env var")
	authLoginCmd.Flags().StringVar(&authFlagClientSecret, "client-secret", "", "OAuth client secret — avoid shell history: prefer BB_CLIENT_SECRET env var")
	authLoginCmd.Flags().StringVar(&authFlagScopes, "scopes", "", fmt.Sprintf("scope level (%s) or explicit comma-separated list; default %q (persisted to config)", strings.Join(auth.ScopePresetNames(), "|"), auth.DefaultScopeLevel))
	authLoginCmd.Flags().BoolVar(&authFlagManual, "manual", false, "No local callback server — print the URL and paste back the redirected URL (useful over SSH)")
	authLoginCmd.Flags().StringVar(&authFlagCallbackURL, "callback-url", "", "Complete login directly from a pasted callback URL (or code), skipping the browser/server")
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authRefreshCmd)
}

func runAuthRefresh(cmd *cobra.Command, args []string) error {
	if auth.HasAPIToken() {
		return fmt.Errorf("authenticated via BITBUCKET_API_TOKEN — nothing to refresh (refresh applies to OAuth logins only)")
	}
	if !authHasStoredTokens() {
		return fmt.Errorf("not logged in — run 'bb auth login' first")
	}

	tokens, err := authRefreshTokens()
	if err != nil {
		return fmt.Errorf("refresh failed (run 'bb auth login' to re-authenticate): %w", err)
	}

	output.Success("Access token refreshed")
	if tokens.ExpiresAt > 0 {
		output.Info("Valid for %s", time.Until(time.Unix(tokens.ExpiresAt, 0)).Round(time.Second))
	}
	return nil
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	// Flags take precedence, then env/config, then interactive prompt.
	clientID, _ := cmd.Flags().GetString("client-id")
	if clientID == "" {
		clientID = authGetClientID()
	}
	clientSecret, _ := cmd.Flags().GetString("client-secret")
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
		if f, ok := authStdinReader.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			// Read exactly this line so bufio cannot prefetch a pasted secret before
			// ReadPassword switches off terminal echo.
			var line strings.Builder
			one := make([]byte, 1)
			for {
				n, err := f.Read(one)
				if n > 0 {
					if one[0] == '\n' {
						break
					}
					line.WriteByte(one[0])
				}
				if err != nil {
					return fmt.Errorf("read client ID: %w", err)
				}
			}
			clientID = line.String()
		} else {
			clientID, _ = reader.ReadString('\n')
		}
		clientID = strings.TrimSpace(clientID)

		fmt.Print("Enter Client Secret: ")
		if f, ok := authStdinReader.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			secret, err := term.ReadPassword(int(f.Fd()))
			fmt.Println()
			if err != nil {
				return fmt.Errorf("read client secret: %w", err)
			}
			clientSecret = string(secret)
		} else {
			clientSecret, _ = reader.ReadString('\n')
		}
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

	// Resolve the requested scope level: --scopes flag > config > default level.
	// This is validated against the OAuth consumer at login (it fails fast if it
	// asks for scopes the consumer wasn't granted) but does NOT narrow the issued
	// token — Bitbucket always grants the consumer's full configured scopes.
	scopeSpec := authFlagScopes
	if scopeSpec == "" {
		scopeSpec = authGetScopes()
	}
	scopes, err := auth.ResolveScopes(scopeSpec)
	if err != nil {
		return err
	}

	// Perform OAuth login
	oauthCfg := auth.OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
	}

	var tokens *auth.TokenData
	switch {
	case authFlagCallbackURL != "":
		// Exchange directly from a pasted callback URL / code (no state to check).
		tokens, err = authExchangeCallbackURL(oauthCfg, "", authFlagCallbackURL)
	case authFlagManual:
		// No local callback server — print the URL and read the redirect back.
		authURL, state, berr := authBuildAuthorizeURL(oauthCfg)
		if berr != nil {
			return fmt.Errorf("login failed: %w", berr)
		}
		output.Info("Open this URL, authorize, then paste the URL you were redirected to")
		output.Info("(the localhost page won't load over SSH — just copy its address):")
		fmt.Println(authURL)
		fmt.Print("\nPaste callback URL (or code): ")
		reader := bufio.NewReader(authStdinReader)
		pasted, _ := reader.ReadString('\n')
		tokens, err = authExchangeCallbackURL(oauthCfg, state, pasted)
	default:
		tokens, err = authLogin(oauthCfg)
	}
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Store tokens
	if err := authStoreTokens(tokens); err != nil {
		return fmt.Errorf("failed to store tokens: %w", err)
	}

	// Remember an explicitly chosen level so the next login reuses it.
	if authFlagScopes != "" {
		if err := authSetScopes(authFlagScopes); err != nil {
			output.Error("Failed to save scope preference: %v", err)
		}
	}

	output.Success("Logged in to Bitbucket")
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	hadTokens := authHasStoredTokens()
	if err := authLogout(); err != nil {
		return fmt.Errorf("failed to log out: %w", err)
	}
	if hadTokens {
		output.Success("Logged out of Bitbucket")
	} else {
		output.Info("Already logged out")
	}

	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	status, err := authGetStatus()
	if err != nil {
		return err
	}

	if !status.LoggedIn {
		output.Info("Not logged in")
		if status.LegacyTokenFile {
			output.Info("An existing token file was left unchanged. Select 'bb config set token_store file' to use it, or log in to the system keyring.")
		}
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
		if authHasStoredTokens() {
			output.Info("Note: an OAuth login is also stored, but BITBUCKET_API_TOKEN takes precedence.")
			output.Info("      Unset BITBUCKET_API_TOKEN to use the OAuth session (which auto-refreshes).")
		}
	case auth.AuthMethodOAuth:
		output.Info("Method: OAuth 2.0")
		if status.Scopes != "" {
			output.Info("Scopes: %s", status.Scopes)
		}
		printTokenExpiry(status)
	}

	return nil
}

// printTokenExpiry reports access-token expiry and whether silent refresh is
// available — the things that determine whether you should ever need to re-login.
func printTokenExpiry(status *auth.AuthStatus) {
	switch status.ExpiresAt {
	case 0:
		output.Info("Access token: expiry unknown (stored by an older version — run 'bb auth login' once to enable proactive refresh)")
	default:
		remaining := time.Until(time.Unix(status.ExpiresAt, 0))
		if remaining > 0 {
			output.Info("Access token: valid for %s", remaining.Round(time.Second))
		} else {
			output.Info("Access token: expired %s ago (will refresh on next use)", (-remaining).Round(time.Second))
		}
	}

	if status.HasRefreshToken {
		output.Info("Refresh token: present — sessions renew automatically, no re-login needed")
	} else {
		output.Info("Refresh token: MISSING — you'll be forced to re-login when the access token expires; run 'bb auth login'")
	}
}
