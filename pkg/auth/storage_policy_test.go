package auth

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func isolatedKeyringPolicy(t *testing.T) {
	t.Helper()
	oldForce, oldMode := ForceFileStorage, tokenStoreMode
	ForceFileStorage, tokenStoreMode = false, ""
	keyring.MockInit()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("BB_TOKEN_STORE", "")
	t.Setenv("BITBUCKET_API_TOKEN", "")
	t.Cleanup(func() { ForceFileStorage, tokenStoreMode = oldForce, oldMode; keyring.MockInit() })
}

func TestExplicitKeyringStorageNeverCreatesPlaintextMirror(t *testing.T) {
	isolatedKeyringPolicy(t)
	SetTokenStoreMode("keyring")
	if storeMode() != storeKeyring {
		t.Fatalf("default mode %s", storeMode())
	}
	if err := StoreTokens(&TokenData{AccessToken: "dummy-access", RefreshToken: "dummy-refresh"}); err != nil {
		t.Fatal(err)
	}
	path, _ := tokenFilePath()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("default created a file: %v", err)
	}
	if tokens, err := LoadTokens(); err != nil || tokens.AccessToken != "dummy-access" {
		t.Fatalf("keyring read failed: %v", err)
	}
	SetTokenStoreMode("misspelled")
	if storeMode() != storeKeyring {
		t.Fatal("unknown mode enabled plaintext storage")
	}
}

func TestLegacyFileRequiresExplicitOptInAndIsPreserved(t *testing.T) {
	isolatedKeyringPolicy(t)
	SetTokenStoreMode("keyring")
	if err := storeTokensToFile(&TokenData{AccessToken: "legacy-dummy"}); err != nil {
		t.Fatal(err)
	}
	path, _ := tokenFilePath()
	before, _ := os.ReadFile(path)
	if _, err := LoadTokens(); !errors.Is(err, keyring.ErrNotFound) || !strings.Contains(err.Error(), "left unchanged") {
		t.Fatalf("missing migration guidance: %v", err)
	}
	status, err := GetStatus()
	if err != nil || status.LoggedIn || !status.LegacyTokenFile {
		t.Fatalf("legacy status: %+v %v", status, err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("legacy credentials were modified")
	}
	if _, err := keyring.Get(serviceName, "tokens"); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatal("legacy credentials were implicitly imported")
	}
	SetTokenStoreMode("file")
	if tokens, err := LoadTokens(); err != nil || tokens.AccessToken != "legacy-dummy" {
		t.Fatalf("explicit file opt-in failed: %v", err)
	}
}

func TestVerifiedEntryLaterMissingIsVisibleWithoutFallback(t *testing.T) {
	isolatedKeyringPolicy(t)
	SetTokenStoreMode("keyring")
	t.Setenv("BB_AUTH_DEBUG", "1")
	if err := StoreTokens(&TokenData{AccessToken: "dummy-disappearing-token"}); err != nil {
		t.Fatal(err)
	}
	// Simulate loss in the in-memory backend, never the user's OS keyring.
	if err := keyring.Delete(serviceName, "tokens"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTokens(); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("missing entry was hidden: %v", err)
	}
	log, err := os.ReadFile(DebugLogPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "keyring.Set verified") || !strings.Contains(string(log), "notFound=true") || strings.Contains(string(log), "dummy-disappearing-token") {
		t.Fatalf("missing or unsafe disappearance diagnostics: %q", log)
	}
}

func TestKeyringFailuresAreVisibleAndDebugOmitsBackendSecrets(t *testing.T) {
	isolatedKeyringPolicy(t)
	SetTokenStoreMode("keyring")
	t.Setenv("BB_AUTH_DEBUG", "1")
	secret := "dummy-backend-credential-MUST-NOT-BE-LOGGED"
	locked := errors.New("keyring unavailable: " + secret)
	keyring.MockInitWithError(locked)
	if _, err := GetStatus(); !errors.Is(err, locked) {
		t.Fatalf("status hid keyring failure: %v", err)
	}
	if _, err := GetCredential(); !errors.Is(err, locked) || strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("credential lookup misdiagnosed failure: %v", err)
	}
	if err := StoreTokens(&TokenData{AccessToken: secret}); !errors.Is(err, locked) {
		t.Fatalf("storage hid backend failure: %v", err)
	}
	path, _ := tokenFilePath()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unavailable keyring silently fell back to plaintext")
	}
	log, err := os.ReadFile(DebugLogPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(log), secret) || !strings.Contains(string(log), "attempt 3") || !strings.Contains(string(log), "backend-error") {
		t.Fatalf("unsafe or missing debug classification: %q", log)
	}
}

func TestDefaultMirrorRecoversMissingKeyringEntry(t *testing.T) {
	isolatedKeyringPolicy(t)
	if storeMode() != storeAuto {
		t.Fatalf("default mode %s", storeMode())
	}
	if err := StoreTokens(&TokenData{AccessToken: "dummy-default-mirror", RefreshToken: "dummy-refresh"}); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTokensFromFile(); err != nil {
		t.Fatalf("default mirror missing: %v", err)
	}
	if err := keyring.Delete(serviceName, "tokens"); err != nil {
		t.Fatal(err)
	}
	got, err := LoadTokens()
	if err != nil || got.AccessToken != "dummy-default-mirror" {
		t.Fatalf("fallback failed: %v", err)
	}
	if _, err := keyring.Get(serviceName, "tokens"); err != nil {
		t.Fatalf("keyring not re-seeded: %v", err)
	}
}
