package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/ThomasRedstone/bb/internal/config"
)

// GetAPIToken returns the BITBUCKET_API_TOKEN if set
func GetAPIToken() string {
	return os.Getenv("BITBUCKET_API_TOKEN")
}

// HasAPIToken checks if BITBUCKET_API_TOKEN is set
func HasAPIToken() bool {
	return GetAPIToken() != ""
}

// LikelyAtlassianTokenWithoutEmail reports a common misconfiguration: a
// BITBUCKET_API_TOKEN that looks like an Atlassian API token (ATATT/ATCTT
// prefix) — which requires Basic auth — while BITBUCKET_EMAIL is unset, so bb
// would send it as a Bearer token and Bitbucket would reject every request.
func LikelyAtlassianTokenWithoutEmail() bool {
	t := GetAPIToken()
	if t == "" || GetAPIEmail() != "" {
		return false
	}
	return strings.HasPrefix(t, "ATATT") || strings.HasPrefix(t, "ATCTT")
}

// GetAPIEmail returns the BITBUCKET_EMAIL if set. When paired with
// BITBUCKET_API_TOKEN it selects HTTP Basic auth (the scheme required by
// Atlassian scoped API tokens, the replacement for deprecated app passwords).
func GetAPIEmail() string {
	return os.Getenv("BITBUCKET_EMAIL")
}

// Auth schemes used when attaching credentials to a request.
const (
	SchemeBearer = "bearer"
	SchemeBasic  = "basic"
)

// Credential carries the resolved authentication for a request along with the
// scheme used to attach it. OAuth access tokens and Bitbucket Access Tokens use
// Bearer; Atlassian scoped API tokens use Basic (email:token).
type Credential struct {
	Scheme   string
	Username string // basic-auth username (account email); empty for bearer
	Token    string // bearer token, or basic-auth password
}

// Apply attaches the credential to an outgoing request.
func (c Credential) Apply(req *http.Request) {
	if c.Token == "" {
		return
	}
	if c.Scheme == SchemeBasic {
		req.SetBasicAuth(c.Username, c.Token)
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
}

// GetCredential resolves the credential to use for API requests.
//
// Selection order:
//  1. BITBUCKET_API_TOKEN + BITBUCKET_EMAIL -> Basic (Atlassian API token)
//  2. BITBUCKET_API_TOKEN alone             -> Bearer (Access Token / raw token)
//  3. stored OAuth access token             -> Bearer
func GetCredential() (Credential, error) {
	if token := GetAPIToken(); token != "" {
		if email := GetAPIEmail(); email != "" {
			return Credential{Scheme: SchemeBasic, Username: email, Token: token}, nil
		}
		return Credential{Scheme: SchemeBearer, Token: token}, nil
	}

	// Fall back to OAuth tokens
	tokens, err := LoadTokens()
	if err != nil {
		return Credential{}, fmt.Errorf("not logged in: %w", err)
	}

	if tokens.AccessToken == "" {
		return Credential{}, fmt.Errorf("not logged in")
	}

	// Refresh proactively when the token is expired or about to be. Bitbucket
	// returns 404 (not 401) for expired-token access to private resources, so
	// reactive 401 refresh alone leaves users looking "logged out".
	if tokens.NeedsRefresh() && tokens.RefreshToken != "" {
		if refreshed, rErr := RefreshAccessToken(); rErr == nil {
			tokens = refreshed
		}
		// On refresh failure fall through with the existing token: the request
		// surfaces the auth error, and the client's 401 path may still recover.
	}

	return Credential{Scheme: SchemeBearer, Token: tokens.AccessToken}, nil
}

// GetValidToken returns a valid access token string. Retained for callers that
// only need the raw token; prefer GetCredential, which also carries the scheme.
func GetValidToken() (string, error) {
	cred, err := GetCredential()
	if err != nil {
		return "", err
	}
	return cred.Token, nil
}

// RefreshAccessToken uses the refresh token to get a new access token
func RefreshAccessToken() (*TokenData, error) {
	tokens, err := LoadTokens()
	if err != nil {
		return nil, fmt.Errorf("no tokens to refresh: %w", err)
	}

	if tokens.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	clientID := config.GetClientID()
	clientSecret := config.GetClientSecret()

	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("OAuth client credentials not configured")
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tokens.RefreshToken},
	}

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClientFn().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("token refresh failed: %v", errResp)
	}

	var newTokens TokenData
	if err := json.NewDecoder(resp.Body).Decode(&newTokens); err != nil {
		return nil, err
	}

	// Bitbucket Cloud does not return a refresh_token in the refresh response —
	// the original one stays valid. Without this, StoreTokens would overwrite the
	// stored refresh token with an empty string, so the *next* refresh would fail
	// and force a full re-login. Carry over the existing refresh token (and scopes)
	// when the response omits them.
	if newTokens.RefreshToken == "" {
		newTokens.RefreshToken = tokens.RefreshToken
	}
	if newTokens.Scopes == "" {
		newTokens.Scopes = tokens.Scopes
	}

	// Store the new tokens
	if err := StoreTokens(&newTokens); err != nil {
		return nil, fmt.Errorf("failed to store refreshed tokens: %w", err)
	}

	return &newTokens, nil
}

// Logout clears stored tokens
func Logout() error {
	return DeleteTokens()
}

// AuthMethod indicates how authentication is configured
type AuthMethod string

const (
	AuthMethodNone  AuthMethod = "none"
	AuthMethodToken AuthMethod = "token" // BITBUCKET_API_TOKEN
	AuthMethodOAuth AuthMethod = "oauth" // OAuth 2.0 flow
)

// AuthStatus returns information about the current authentication state
type AuthStatus struct {
	LoggedIn        bool
	Method          AuthMethod
	Scheme          string // auth scheme when Method is token: "basic" or "bearer"
	Scopes          string
	ExpiresAt       int64 // absolute Unix expiry of the OAuth access token (0 = unknown)
	HasRefreshToken bool  // whether a refresh token is stored (enables silent renewal)
}

// GetStatus returns the current authentication status
func GetStatus() (*AuthStatus, error) {
	// Check for API token first
	if HasAPIToken() {
		scheme := SchemeBearer
		if GetAPIEmail() != "" {
			scheme = SchemeBasic
		}
		return &AuthStatus{
			LoggedIn: true,
			Method:   AuthMethodToken,
			Scheme:   scheme,
		}, nil
	}

	// Check for OAuth tokens
	tokens, err := LoadTokens()
	if err != nil {
		return &AuthStatus{LoggedIn: false, Method: AuthMethodNone}, nil
	}

	if tokens.AccessToken == "" {
		return &AuthStatus{LoggedIn: false, Method: AuthMethodNone}, nil
	}

	return &AuthStatus{
		LoggedIn:        true,
		Method:          AuthMethodOAuth,
		Scopes:          tokens.Scopes,
		ExpiresAt:       tokens.ExpiresAt,
		HasRefreshToken: tokens.RefreshToken != "",
	}, nil
}
