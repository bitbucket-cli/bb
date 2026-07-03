package bitbucket

import (
	"os"
	"testing"

	"github.com/ThomasRedstone/bb/pkg/auth"
)

// TestMain isolates this package's test suite from the developer's real stored
// OAuth token. TestNewClient_NoCredentials calls auth.DeleteTokens(), which
// (absent this redirect) would delete the real "bb-cli" keyring entry and log
// the developer/contributor out on `go test ./...`. Redirect the keyring
// service name to a throwaway so the suite can never touch real credentials.
func TestMain(m *testing.M) {
	auth.SetKeyringServiceForTesting("bb-cli-test")
	os.Exit(m.Run())
}
