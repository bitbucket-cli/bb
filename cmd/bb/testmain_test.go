package main

import (
	"os"
	"testing"

	"github.com/ThomasRedstone/bb/pkg/auth"
)

// TestMain isolates the cmd/bb test suite from the developer's real stored
// token. Several tests call auth.StoreTokens/DeleteTokens against the real
// keyring + config dir; redirect both so the suite can't log you out.
func TestMain(m *testing.M) {
	auth.SetKeyringServiceForTesting("bb-cli-test")
	if tmp, err := os.MkdirTemp("", "bb-cli-test-home"); err == nil {
		os.Setenv("HOME", tmp)
		defer os.RemoveAll(tmp)
	}
	os.Exit(m.Run())
}
