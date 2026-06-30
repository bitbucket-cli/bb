package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"
)

// refreshBufferSeconds refreshes the access token slightly before its actual
// expiry, to avoid racing a request against the boundary.
const refreshBufferSeconds = 120

const (
	serviceName     = "bb-cli"
	accessTokenKey  = "access_token"
	refreshTokenKey = "refresh_token"
)

// ForceFileStorage can be set to true in tests to skip keyring
var ForceFileStorage = false

// Mockable functions for testing
var userHomeDir = os.UserHomeDir

// TokenData holds the stored token information
type TokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"` // absolute Unix seconds; computed at store time
	TokenType    string `json:"token_type"`
	Scopes       string `json:"scopes"`
}

// NeedsRefresh reports whether the access token has expired or is within the
// refresh buffer of expiring. Returns false when expiry is unknown
// (ExpiresAt == 0, e.g. tokens stored by an older version), so callers fall
// back to optimistic use plus reactive 401 refresh.
func (t *TokenData) NeedsRefresh() bool {
	if t.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() >= t.ExpiresAt-refreshBufferSeconds
}

// AccessTokenExpiredOrUnknown reports whether the stored OAuth access token has
// expired (or is within the refresh buffer of expiring), or whether its expiry
// is unknown (ExpiresAt == 0, e.g. stored by an older version). Used to decide
// whether a 404 — which Bitbucket returns for expired access to private
// resources — is worth a refresh-and-retry rather than a genuine not-found.
func AccessTokenExpiredOrUnknown() bool {
	tokens, err := LoadTokens()
	if err != nil || tokens.AccessToken == "" {
		return false
	}
	if tokens.ExpiresAt == 0 {
		return true
	}
	return tokens.NeedsRefresh()
}

// StoreTokens saves tokens to the system keyring
func StoreTokens(data *TokenData) error {
	// Record an absolute expiry so callers can refresh proactively rather than
	// relying on a 401 (which Bitbucket masks as 404 for private resources).
	if data.ExpiresAt == 0 && data.ExpiresIn > 0 {
		data.ExpiresAt = time.Now().Unix() + data.ExpiresIn
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	// Skip keyring in test mode
	if ForceFileStorage {
		return storeTokensToFile(data)
	}

	// Try system keyring first
	err = keyring.Set(serviceName, "tokens", string(jsonData))
	if err != nil {
		// Fallback to file-based storage
		return storeTokensToFile(data)
	}
	return nil
}

// LoadTokens retrieves tokens from the system keyring
func LoadTokens() (*TokenData, error) {
	// Skip keyring in test mode
	if ForceFileStorage {
		return loadTokensFromFile()
	}

	// Try system keyring first
	jsonData, err := keyring.Get(serviceName, "tokens")
	if err != nil {
		// Fallback to file-based storage
		return loadTokensFromFile()
	}

	var data TokenData
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token data: %w", err)
	}
	return &data, nil
}

// DeleteTokens removes stored tokens from all storage backends.
func DeleteTokens() error {
	_ = keyring.Delete(serviceName, "tokens")
	_ = deleteTokenFile()
	return nil
}

// HasStoredTokens checks if tokens are stored
func HasStoredTokens() bool {
	tokens, err := LoadTokens()
	return err == nil && tokens != nil && tokens.AccessToken != ""
}

// File-based fallback storage

func tokenFilePath() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "bb", "tokens.json"), nil
}

func storeTokensToFile(data *TokenData) error {
	path, err := tokenFilePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return os.WriteFile(path, jsonData, 0600)
}

func loadTokensFromFile() (*TokenData, error) {
	path, err := tokenFilePath()
	if err != nil {
		return nil, err
	}

	jsonData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data TokenData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func deleteTokenFile() error {
	path, err := tokenFilePath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}
