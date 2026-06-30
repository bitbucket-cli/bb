package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

// captureOut captures stdout while f runs.
func captureOut(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// TestIntegration_SnippetListThroughCobra exercises the full path: cobra arg
// parsing -> subcommand dispatch -> RunE -> API call (mock server) -> rendered
// output. This is the mock-API-server integration test.
func TestIntegration_SnippetListThroughCobra(t *testing.T) {
	defer func() { snippetNewAPIClient = api.NewClient }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/snippets/myws") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"id":"xyz","title":"hello","is_private":false,"owner":{"display_name":"Tom"}}]}`))
	}))
	defer server.Close()

	snippetNewAPIClient = mockAPIClientFactory(server)
	defer rootCmd.SetArgs(nil)

	out := captureOut(func() {
		rootCmd.SetArgs([]string{"snippet", "list", "myws"})
		if err := rootCmd.Execute(); err != nil {
			t.Errorf("Execute error: %v", err)
		}
	})

	if !strings.Contains(out, "xyz") || !strings.Contains(out, "hello") {
		t.Errorf("integration output missing snippet data, got: %q", out)
	}
}

// TestIntegration_DownloadDeleteThroughCobra runs a mutating command end to end.
func TestIntegration_DownloadDeleteThroughCobra(t *testing.T) {
	defer func() { downloadNewAPIClient = api.NewClient }()

	var sawDelete bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			sawDelete = true
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	downloadNewAPIClient = mockAPIClientFactory(server)
	defer rootCmd.SetArgs(nil)

	captureOut(func() {
		rootCmd.SetArgs([]string{"download", "delete", "myws/myrepo", "app.zip"})
		if err := rootCmd.Execute(); err != nil {
			t.Errorf("Execute error: %v", err)
		}
	})

	if !sawDelete {
		t.Error("expected a DELETE through the full command path")
	}
}
