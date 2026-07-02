package auth

import (
	"os"
	"testing"
)

// TestMain isolates the whole auth test suite from the developer's real stored
// token by redirecting the keyring service name. Without this, the tests that
// exercise StoreTokens/DeleteTokens delete the real "bb-cli" keyring entry —
// i.e. `go test ./...` silently logs you out. (Individual tests already isolate
// the file store via setupTestHome, so HOME is left alone here.)
func TestMain(m *testing.M) {
	serviceName = "bb-cli-test"
	os.Exit(m.Run())
}
