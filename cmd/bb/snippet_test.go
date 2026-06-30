package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func resetSnippetMocks() {
	snippetNewAPIClient = api.NewClient
	snippetStdin = os.Stdin
	snippetRole = ""
	snippetTitle = ""
	snippetPrivate = false
	snippetFilename = ""
}

func TestSnippetCommandStructure(t *testing.T) {
	if snippetCmd.Short == "" {
		t.Error("snippet command should have a short description")
	}
	expected := []string{"list", "view", "create", "delete"}
	for _, name := range expected {
		found := false
		for _, c := range snippetCmd.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected snippet subcommand %q not found", name)
		}
	}
}

func TestSnippetCreateFlags(t *testing.T) {
	for _, f := range []string{"title", "private", "filename"} {
		if snippetCreateCmd.Flags().Lookup(f) == nil {
			t.Errorf("--%s flag not found on snippet create", f)
		}
	}
}

func TestRunSnippetList(t *testing.T) {
	defer resetSnippetMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/snippets/myws") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"id":"abc123","title":"hello","is_private":true,"updated_on":"2026-01-01","owner":{"display_name":"Tom"}}]}`))
	}))
	defer server.Close()

	snippetNewAPIClient = mockAPIClientFactory(server)
	if err := runSnippetList(snippetListCmd, []string{"myws"}); err != nil {
		t.Errorf("runSnippetList error: %v", err)
	}
}

func TestRunSnippetView(t *testing.T) {
	defer resetSnippetMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"abc123","title":"hello","is_private":false,"owner":{"display_name":"Tom"},"files":{"main.go":{}}}`))
	}))
	defer server.Close()

	snippetNewAPIClient = mockAPIClientFactory(server)
	if err := runSnippetView(snippetViewCmd, []string{"myws", "abc123"}); err != nil {
		t.Errorf("runSnippetView error: %v", err)
	}
}

func TestRunSnippetCreate_FromFile(t *testing.T) {
	defer resetSnippetMocks()

	var gotMethod, gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"new123"}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "snippet.txt")
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	snippetNewAPIClient = mockAPIClientFactory(server)
	snippetTitle = "test"
	if err := runSnippetCreate(snippetCreateCmd, []string{"myws", path}); err != nil {
		t.Fatalf("runSnippetCreate error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Errorf("content-type = %q, want multipart/form-data", gotContentType)
	}
}

func TestRunSnippetCreate_NoInput(t *testing.T) {
	defer resetSnippetMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called when there is no input")
	}))
	defer server.Close()

	snippetNewAPIClient = mockAPIClientFactory(server)
	err := runSnippetCreate(snippetCreateCmd, []string{"myws"})
	if err == nil {
		t.Error("expected error when no files or --filename given")
	}
}

func TestRunSnippetDelete(t *testing.T) {
	defer resetSnippetMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	snippetNewAPIClient = mockAPIClientFactory(server)
	if err := runSnippetDelete(snippetDeleteCmd, []string{"myws", "abc123"}); err != nil {
		t.Errorf("runSnippetDelete error: %v", err)
	}
}

func TestRunSnippetList_NoClient(t *testing.T) {
	defer resetSnippetMocks()

	snippetNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}
	if err := runSnippetList(snippetListCmd, []string{"myws"}); err == nil {
		t.Error("expected error when client creation fails")
	}
}

func TestResolveWorkspace(t *testing.T) {
	if ws, err := resolveWorkspace("explicit"); err != nil || ws != "explicit" {
		t.Errorf("resolveWorkspace(explicit) = %q, %v", ws, err)
	}

	orig := os.Getenv("BB_WORKSPACE")
	defer os.Setenv("BB_WORKSPACE", orig)

	os.Setenv("BB_WORKSPACE", "fromenv")
	if ws, err := resolveWorkspace(""); err != nil || ws != "fromenv" {
		t.Errorf("resolveWorkspace(\"\") with env = %q, %v; want fromenv", ws, err)
	}
	// Note: the "no workspace anywhere" error path isn't asserted here because
	// config state is a package-global shared across the cmd/bb test binary.
}
