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

// Token storage modes.
const (
	// storeAuto keeps the system keyring as the primary store but ALSO mirrors
	// tokens to a 0600 file, and on read falls back to (and re-seeds from) that
	// file. This is the default: it means a keyring that silently drops the entry
	// — locking, or storing into a session collection that later gets evicted —
	// can no longer log the user out.
	storeAuto = "auto"
	// storeKeyring uses only the system keyring (no on-disk token file).
	storeKeyring = "keyring"
	// storeFile uses only a 0600 file (no keyring). Reliable/portable, works
	// headless; tokens sit at-rest on disk.
	storeFile = "file"
)

// tokenStoreMode, when non-empty, overrides the BB_TOKEN_STORE env var. The CLI
// sets it from config (config key `token_store`) via SetTokenStoreMode.
var tokenStoreMode string

// SetTokenStoreMode sets the token storage mode ("auto", "keyring", or "file").
// Empty or unrecognised values resolve to auto.
func SetTokenStoreMode(m string) { tokenStoreMode = m }

// storeMode resolves the effective token storage mode. ForceFileStorage (tests)
// wins so a test run can never touch the real keyring. Otherwise config
// (tokenStoreMode) then BB_TOKEN_STORE, defaulting to auto.
func storeMode() string {
	if ForceFileStorage {
		return storeFile
	}
	m := tokenStoreMode
	if m == "" {
		m = os.Getenv("BB_TOKEN_STORE")
	}
	switch m {
	case storeFile, storeKeyring:
		return m
	default:
		return storeAuto
	}
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

	mode := storeMode()

	if mode == storeFile {
		debugLog("StoreTokens: mode=file -> FILE only (access_len=%d refresh_present=%v expires_at=%d)", len(data.AccessToken), data.RefreshToken != "", data.ExpiresAt)
		return storeTokensToFile(data)
	}

	// keyring / auto: write the keyring first (retry + read-back verify).
	keyErr := keyringSetVerified(string(jsonData))

	if mode == storeKeyring {
		if keyErr != nil {
			debugLog("StoreTokens: mode=keyring, keyring.Set FAILED after retries: %v", keyErr)
			return fmt.Errorf("failed to store tokens in system keyring: %w", keyErr)
		}
		debugLog("StoreTokens: mode=keyring -> stored to KEYRING (access_len=%d refresh_present=%v expires_at=%d)", len(data.AccessToken), data.RefreshToken != "", data.ExpiresAt)
		return nil
	}

	// auto: ALSO mirror to a 0600 file so a later keyring drop can't log the user
	// out. Succeed as long as at least one backend persisted.
	fileErr := storeTokensToFile(data)
	switch {
	case keyErr == nil && fileErr == nil:
		debugLog("StoreTokens: mode=auto -> stored to KEYRING + file mirror (access_len=%d refresh_present=%v expires_at=%d)", len(data.AccessToken), data.RefreshToken != "", data.ExpiresAt)
		return nil
	case keyErr == nil:
		debugLog("StoreTokens: mode=auto -> KEYRING ok, file mirror failed: %v", fileErr)
		return nil
	case fileErr == nil:
		debugLog("StoreTokens: mode=auto -> keyring failed (%v), FILE mirror ok", keyErr)
		return nil
	default:
		debugLog("StoreTokens: mode=auto -> BOTH backends failed (keyring=%v file=%v)", keyErr, fileErr)
		return fmt.Errorf("failed to store tokens (keyring: %v; file: %v)", keyErr, fileErr)
	}
}

// LoadTokens retrieves tokens, preferring the system keyring and (in auto mode)
// falling back to the 0600 file mirror.
func LoadTokens() (*TokenData, error) {
	mode := storeMode()

	if mode == storeFile {
		data, err := loadTokensFromFile()
		if err != nil {
			debugLog("LoadTokens: mode=file, load failed: %v", err)
			return nil, err
		}
		debugLog("LoadTokens: loaded from FILE (mode=file)")
		return data, nil
	}

	// keyring / auto: try the keyring first (retry on transient errors).
	jsonData, err := keyringGetRetry()
	if err == nil {
		debugLog("LoadTokens: loaded from KEYRING (len=%d)", len(jsonData))
		var data TokenData
		if uerr := json.Unmarshal([]byte(jsonData), &data); uerr != nil {
			return nil, fmt.Errorf("failed to unmarshal token data: %w", uerr)
		}
		return &data, nil
	}
	notFound := errors.Is(err, keyring.ErrNotFound)

	if mode == storeKeyring {
		// keyring-only: no file fallback. Distinguish "not logged in" from
		// "keyring unavailable" so we don't loop the user through re-login.
		debugLog("LoadTokens: mode=keyring, keyring.Get error notFound=%v err=%v", notFound, err)
		if notFound {
			return nil, err
		}
		return nil, fmt.Errorf("system keyring unavailable (%v); no fallback token file — this is a keyring problem, not a login one", err)
	}

	// auto: fall back to the file mirror.
	debugLog("LoadTokens: keyring miss notFound=%v err=%v -> file mirror", notFound, err)
	data, ferr := loadTokensFromFile()
	if ferr != nil {
		if notFound {
			debugLog("LoadTokens: keyring not-found and no file mirror (not logged in): %v", ferr)
			return nil, ferr
		}
		debugLog("LoadTokens: keyring UNAVAILABLE and no file mirror: %v", err)
		return nil, fmt.Errorf("system keyring unavailable (%v); no fallback token file — this is a keyring problem, not a login one", err)
	}

	// The keyring dropped the entry but the file mirror still has it. Re-seed the
	// keyring (best-effort) so the primary store is repopulated for next time.
	debugLog("LoadTokens: loaded from FILE mirror; re-seeding keyring")
	if reseed, merr := json.Marshal(data); merr == nil {
		if serr := keyring.Set(serviceName, "tokens", string(reseed)); serr != nil {
			debugLog("LoadTokens: keyring re-seed failed: %v", serr)
		}
	}
	return data, nil
}

// DeleteTokens removes stored tokens. In file mode (tests via ForceFileStorage,
// or BB_TOKEN_STORE=file) the keyring is left untouched so a test run can never
// delete the developer's real OAuth token. keyring mode clears only the keyring;
// auto clears both the keyring and the 0600 file mirror.
func DeleteTokens() error {
	mode := storeMode()
	if mode == storeFile {
		debugLog("DeleteTokens: mode=file -> clearing file token store only")
		_ = deleteTokenFile()
		return nil
	}
	debugLog("DeleteTokens: mode=%s -> clearing keyring%s", mode, ternary(mode == storeKeyring, "", " + file mirror"))
	_ = keyring.Delete(serviceName, "tokens")
	if mode != storeKeyring {
		_ = deleteTokenFile()
	}
	return nil
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
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
