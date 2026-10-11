package auth

import (
	"context"
	"errors"
	"github.com/zalando/go-keyring"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogoutClearsInactiveStores(t *testing.T) {
	keyring.MockInit()
	t.Setenv("HOME", t.TempDir())
	oldForce, oldMode := ForceFileStorage, tokenStoreMode
	ForceFileStorage = false
	tokenStoreMode = "auto"
	defer func() { ForceFileStorage = oldForce; tokenStoreMode = oldMode; keyring.MockInit() }()
	if err := StoreTokens(&TokenData{AccessToken: "dummy"}); err != nil {
		t.Fatal(err)
	}
	tokenStoreMode = "keyring"
	if err := DeleteTokens(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".config", "bb", "tokens.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mirror retained: %v", err)
	}
	if _, err := keyring.Get(serviceName, "tokens"); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("keyring retained: %v", err)
	}
}

func TestLogoutReportsBackendFailureButStillClearsFile(t *testing.T) {
	keyring.MockInitWithError(errors.New("locked"))
	defer keyring.MockInit()
	t.Setenv("HOME", t.TempDir())
	old := ForceFileStorage
	ForceFileStorage = true
	defer func() { ForceFileStorage = old }()
	if err := StoreTokens(&TokenData{AccessToken: "dummy"}); err != nil {
		t.Fatal(err)
	}
	ForceFileStorage = false
	if err := DeleteTokens(); err == nil {
		t.Fatal("backend failure hidden")
	}
	if _, err := loadTokensFromFile(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file not cleared: %v", err)
	}
}

func TestCallbackFloodCannotBlockShutdown(t *testing.T) {
	codes := make(chan string, 1)
	errs := make(chan error, 1)
	server, err := startCallbackServer("expected", codes, errs)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		resp, err := http.Get("http://localhost:34106/callback?state=expected&code=dummy")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		server.Close()
		t.Fatal(err)
	}
}

func TestTokenWriteDoesNotFollowLegacyTempSymlink(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := ForceFileStorage
	ForceFileStorage = true
	defer func() { ForceFileStorage = old }()
	path, err := tokenFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "untouched")
	if err := os.WriteFile(target, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path+".tmp"); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := StoreTokens(&TokenData{AccessToken: "dummy"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "unchanged" {
		t.Fatal("token write followed predictable temp symlink")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("token permissions=%o", info.Mode().Perm())
	}
}

func TestCallbackURLsFailBeforeTokenExchange(t *testing.T) {
	defer resetMocks()
	calls := 0
	httpClientFn = func() *http.Client { calls++; return &http.Client{} }
	cfg := OAuthConfig{ClientID: "dummy", ClientSecret: "dummy"}
	cases := []struct{ input, want string }{
		{"http://localhost:34106/callback?error=access_denied", "state mismatch"},
		{"http://localhost:34106/callback?state=expected&error=access_denied", "authorization failed"},
		{"http://localhost:34106/callback?state=expected", "no authorization code"},
		{"http://localhost:34106/callback", "state mismatch"},
		{"http://%zz/callback?code=dummy", "invalid callback URL"},
	}
	for _, tc := range cases {
		_, err := ExchangeCallbackURL(cfg, "expected", tc.input)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("input %q: err=%v want %q", tc.input, err, tc.want)
		}
	}
	if calls != 0 {
		t.Fatalf("invalid callbacks attempted %d token exchanges", calls)
	}
}
