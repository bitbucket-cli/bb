package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ThomasRedstone/bb/generated/bitbucket"
	"github.com/ThomasRedstone/bb/internal/api"
)

// mockAPIClientFactory creates a mock API client pointing to the test server
func mockAPIClientFactory(server *httptest.Server) func() (*api.Client, error) {
	return func() (*api.Client, error) {
		// Create a bitbucket client pointing to the test server
		bbClient, err := bitbucket.NewClient(server.URL)
		if err != nil {
			return nil, err
		}

		return &api.Client{Client: bbClient}, nil
	}
}

// setupMockServer creates an httptest server with the given handler
func setupMockServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// setupTestEnv sets up a clean test environment
func setupTestEnv(t *testing.T) func() {
	t.Helper()

	// Save original values
	origHome := os.Getenv("HOME")
	origToken := os.Getenv("BITBUCKET_API_TOKEN")

	// Set up temp directory
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	os.Setenv("BITBUCKET_API_TOKEN", "test-token")

	// Return cleanup function
	return func() {
		os.Setenv("HOME", origHome)
		if origToken != "" {
			os.Setenv("BITBUCKET_API_TOKEN", origToken)
		} else {
			os.Unsetenv("BITBUCKET_API_TOKEN")
		}
	}
}
