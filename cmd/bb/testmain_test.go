package main

import (
	"os"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/bitbucket-cli/bb/pkg/auth"
)

// TestMain isolates the cmd/bb test suite from the developer's real stored
// token. Several tests call auth.StoreTokens/DeleteTokens against the real
// keyring + config dir; redirect both so the suite can't log you out.
func TestMain(m *testing.M) {
	keyring.MockInit()
	auth.SetKeyringServiceForTesting("bb-cli-test")
	tmp, err := os.MkdirTemp("", "bb-cli-test-home")
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
