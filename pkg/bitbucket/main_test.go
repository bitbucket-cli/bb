package bitbucket

import (
	"os"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/bitbucket-cli/bb/pkg/auth"
)

// TestMain isolates this package's test suite from the developer's real stored
// OAuth token. TestNewClient_NoCredentials calls auth.DeleteTokens(), which
// (absent this redirect) would delete the real "bb-cli" keyring entry and log
// the developer/contributor out on `go test ./...`. Redirect the keyring
// service name to a throwaway so the suite can never touch real credentials.
func TestMain(m *testing.M) {
	keyring.MockInit()
	auth.SetKeyringServiceForTesting("bb-cli-test")
	tmp, err := os.MkdirTemp("", "bb-bitbucket-test-home")
	if err != nil {
		panic(err)
	}
	for _, key := range []string{"HOME", "USERPROFILE"} {
		if err := os.Setenv(key, tmp); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}
