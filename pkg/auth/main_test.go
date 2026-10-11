package auth

import (
	"os"
	"testing"

	"github.com/zalando/go-keyring"
)

// TestMain isolates the whole auth test suite from the developer's real stored
// token. It redirects BOTH:
//   - the keyring service name (so StoreTokens/DeleteTokens can't touch the real
//     "bb-cli" keyring entry), and
//   - HOME (so the default "auto" store mode's 0600 file mirror, written under
//     ~/.config/bb, lands in a throwaway dir — not the developer's real login).
//
// Without the HOME redirect, any test that calls StoreTokens in auto mode would
// pollute ~/.config/bb/tokens.json (and, via keyring re-seed, the real keyring).
// Individual tests may still layer their own temp HOME on top of this.
func TestMain(m *testing.M) {
	keyring.MockInit()
	serviceName = "bb-cli-test"
	tmp, err := os.MkdirTemp("", "bb-auth-test-home")
	if err != nil {
		panic(err)
	}
	for _, key := range []string{"HOME", "USERPROFILE"} {
		if err := os.Setenv(key, tmp); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	if tmp != "" {
		os.RemoveAll(tmp)
	}
	os.Exit(code)
}
