package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	serviceName    = "bb-cli"
	accessTokenKey = "access_token"
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
	TokenType    string `json:"token_type"`
	Scopes       string `json:"scopes"`
}

// StoreTokens saves tokens to the system keyring
func StoreTokens(data *TokenData) error {
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
