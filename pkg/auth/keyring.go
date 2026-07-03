package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"
)

// refreshBufferSeconds refreshes the access token slightly before its actual
// expiry, to avoid racing a request against the boundary.
const refreshBufferSeconds = 120

// serviceName is the Secret Service item under which tokens are stored. It is a
// var (not const) so tests can redirect it away from the real user's keyring
// entry via SetKeyringServiceForTesting — otherwise running the test suite,
// which exercises StoreTokens/DeleteTokens, would delete the developer's real
// login token from the keyring.
var serviceName = "bb-cli"

// SetKeyringServiceForTesting overrides the keyring service name and returns a
// restore func. Tests (including in other packages) MUST call this so the suite
// never reads or deletes the real user's stored OAuth token.
func SetKeyringServiceForTesting(name string) func() {
	old := serviceName
	serviceName = name
	return func() { serviceName = old }
}

// ForceFileStorage can be set to true in tests to skip keyring
var ForceFileStorage = false

// useFileStore reports whether to bypass the system keyring and persist tokens
// in a 0600 file instead. Enabled in tests (ForceFileStorage) or by the user via
// BB_TOKEN_STORE=file — a reliable fallback when the system keyring is flaky
// (locking, or storing into a session collection that later gets evicted, which
// makes tokens silently disappear between commands).
func useFileStore() bool {
	return ForceFileStorage || os.Getenv("BB_TOKEN_STORE") == "file"
}

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

// keyringAttempts is how many times keyring reads/writes are retried on
// transient errors.
const keyringAttempts = 3

// keyringSetVerified stores value under the tokens key, then reads it back to
// confirm it persisted, retrying on transient failure. This guards against a
// Set that reports success but doesn't persist (e.g. a store interrupted by a
// killed process, or a transient D-Bus/keyring error) — the class of failure
// that made tokens silently vanish.
func keyringSetVerified(value string) error {
	var lastErr error
	for i := 0; i < keyringAttempts; i++ {
		if i > 0 {
			time.Sleep(time.Duration(i) * 50 * time.Millisecond)
		}
		if err := keyring.Set(serviceName, "tokens", value); err != nil {
			lastErr = err
			debugLog("keyring.Set attempt %d failed: %v", i+1, err)
			continue
		}
		got, gerr := keyring.Get(serviceName, "tokens")
		if gerr == nil && got == value {
			if i > 0 {
				debugLog("keyring.Set succeeded on attempt %d (after read-back verify)", i+1)
			}
			return nil
		}
		lastErr = fmt.Errorf("keyring write did not persist (read-back err=%v)", gerr)
		debugLog("keyring.Set attempt %d: read-back verify FAILED (err=%v)", i+1, gerr)
	}
	return lastErr
}

// keyringGetRetry reads the tokens value, retrying on transient (non-not-found)
// errors. A genuine "not found" returns immediately (no retry).
func keyringGetRetry() (string, error) {
	var lastErr error
	for i := 0; i < keyringAttempts; i++ {
		if i > 0 {
			time.Sleep(time.Duration(i) * 50 * time.Millisecond)
		}
		v, err := keyring.Get(serviceName, "tokens")
		if err == nil {
			return v, nil
		}
		if errors.Is(err, keyring.ErrNotFound) {
			return "", err
		}
		lastErr = err
		debugLog("keyring.Get attempt %d transient error: %v", i+1, err)
	}
	return "", lastErr
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

	// Use the file store when the keyring is bypassed.
	if useFileStore() {
		debugLog("StoreTokens: BB_TOKEN_STORE=file -> storing to FILE (access_len=%d refresh_present=%v expires_at=%d)", len(data.AccessToken), data.RefreshToken != "", data.ExpiresAt)
		return storeTokensToFile(data)
	}

	// Try system keyring first (with retry + read-back verify).
	err = keyringSetVerified(string(jsonData))
	if err != nil {
		debugLog("StoreTokens: keyring.Set FAILED after retries (%v) -> file fallback", err)
		if ferr := storeTokensToFile(data); ferr != nil {
			debugLog("StoreTokens: file fallback FAILED: %v", ferr)
			return ferr
		}
		debugLog("StoreTokens: stored to FILE (access_len=%d refresh_present=%v expires_at=%d)", len(data.AccessToken), data.RefreshToken != "", data.ExpiresAt)
		return nil
	}
	debugLog("StoreTokens: stored to KEYRING (access_len=%d refresh_present=%v expires_at=%d)", len(data.AccessToken), data.RefreshToken != "", data.ExpiresAt)
	return nil
}

// LoadTokens retrieves tokens from the system keyring
func LoadTokens() (*TokenData, error) {
	// Use the file store when the keyring is bypassed.
	if useFileStore() {
		data, err := loadTokensFromFile()
		if err != nil {
			debugLog("LoadTokens: BB_TOKEN_STORE=file, load failed: %v", err)
			return nil, err
		}
		debugLog("LoadTokens: loaded from FILE (BB_TOKEN_STORE=file)")
		return data, nil
	}

	// Try system keyring first (with retry on transient errors).
	jsonData, err := keyringGetRetry()
	if err != nil {
		notFound := errors.Is(err, keyring.ErrNotFound)
		debugLog("LoadTokens: keyring.Get error notFound=%v err=%v -> file fallback", notFound, err)
		data, ferr := loadTokensFromFile()
		if ferr != nil {
			// Distinguish "keyring says no such secret" (genuinely not logged in)
			// from "keyring unavailable/locked" (a real problem we must not hide as
			// a login prompt — otherwise the user re-logs-in in a loop).
			if notFound {
				debugLog("LoadTokens: keyring not-found and no file (not logged in): %v", ferr)
				return nil, ferr
			}
			debugLog("LoadTokens: keyring UNAVAILABLE and no file: %v", err)
			return nil, fmt.Errorf("system keyring unavailable (%v); no fallback token file — this is a keyring problem, not a login one", err)
		}
		debugLog("LoadTokens: loaded from FILE fallback")
		return data, nil
	}

	debugLog("LoadTokens: loaded from KEYRING (len=%d)", len(jsonData))
	var data TokenData
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token data: %w", err)
	}
	return &data, nil
}

// DeleteTokens removes stored tokens from all storage backends. When the file
// store is forced (tests via ForceFileStorage, or BB_TOKEN_STORE=file), the
// keyring is left untouched so a test run can never delete the developer's real
// OAuth token from the system keyring.
func DeleteTokens() error {
	if useFileStore() {
		debugLog("DeleteTokens: file store forced -> clearing file token store only")
		_ = deleteTokenFile()
		return nil
	}
	debugLog("DeleteTokens: clearing keyring + file token stores")
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

	// Write atomically: write a temp file then rename over the target, so a
	// process killed mid-write can never leave a truncated/empty tokens file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, jsonData, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
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
