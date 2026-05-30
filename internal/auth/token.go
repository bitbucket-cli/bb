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

// GetValidToken returns a valid access token, refreshing if necessary
func GetValidToken() (string, error) {
	// Check for BITBUCKET_API_TOKEN first (app password or access token)
	if token := GetAPIToken(); token != "" {
		return token, nil
	}

	// Fall back to OAuth tokens
	tokens, err := LoadTokens()
	if err != nil {
		return "", fmt.Errorf("not logged in: %w", err)
	}

	if tokens.AccessToken == "" {
		return "", fmt.Errorf("not logged in")
	}

	// Access tokens expire in 2 hours (7200 seconds)
	// We don't store expiry time, so always try to use the token first
	// The API client wrapper will handle refresh on 401
	return tokens.AccessToken, nil
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
	AuthMethodNone   AuthMethod = "none"
	AuthMethodToken  AuthMethod = "token"  // BITBUCKET_API_TOKEN
	AuthMethodOAuth  AuthMethod = "oauth"  // OAuth 2.0 flow
)

// AuthStatus returns information about the current authentication state
type AuthStatus struct {
	LoggedIn bool
	Method   AuthMethod
	Scopes   string
}

// GetStatus returns the current authentication status
func GetStatus() (*AuthStatus, error) {
	// Check for API token first
	if HasAPIToken() {
		return &AuthStatus{
			LoggedIn: true,
			Method:   AuthMethodToken,
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
		LoggedIn: true,
		Method:   AuthMethodOAuth,
		Scopes:   tokens.Scopes,
	}, nil
}
