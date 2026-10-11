package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

func resetDeployKeyMocks() {
	deployKeyNewAPIClient = api.NewClient
	deployKeyLabel = ""
	deployKeyFile = ""
}

func TestDeployKeyCommandStructure(t *testing.T) {
	if deployKeyCmd.Short == "" {
		t.Error("deploy-key command should have a short description")
	}
	expected := []string{"list", "add", "view", "delete"}
	for _, name := range expected {
		found := false
		for _, c := range deployKeyCmd.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected deploy-key subcommand %q not found", name)
		}
	}
}

func TestRunDeployKeyList(t *testing.T) {
	defer resetDeployKeyMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"id":7,"label":"ci","key":"ssh-ed25519 AAAA","created_on":"2026-01-01"}]}`))
	}))
	defer server.Close()

	deployKeyNewAPIClient = mockAPIClientFactory(server)
	if err := runDeployKeyList(deployKeyListCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runDeployKeyList error: %v", err)
	}
}

func TestRunDeployKeyAdd_Inline(t *testing.T) {
	defer resetDeployKeyMocks()

	var gotMethod, gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":7}`))
	}))
	defer server.Close()

	deployKeyNewAPIClient = mockAPIClientFactory(server)
	deployKeyLabel = "ci"
	if err := runDeployKeyAdd(deployKeyAddCmd, []string{"myws/myrepo", "ssh-ed25519 AAAA user@host"}); err != nil {
		t.Fatalf("runDeployKeyAdd error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotContentType)
	}
}

func TestRunDeployKeyAdd_FromFile(t *testing.T) {
	defer resetDeployKeyMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":8}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id.pub")
	if err := os.WriteFile(keyPath, []byte("ssh-ed25519 AAAA fromfile\n"), 0644); err != nil {
		t.Fatal(err)
	}

	deployKeyNewAPIClient = mockAPIClientFactory(server)
	deployKeyFile = keyPath
	if err := runDeployKeyAdd(deployKeyAddCmd, []string{"myws/myrepo"}); err != nil {
		t.Fatalf("runDeployKeyAdd from file error: %v", err)
	}
}

func TestRunDeployKeyAdd_NoKey(t *testing.T) {
	defer resetDeployKeyMocks()

	deployKeyNewAPIClient = mockAPIClientFactory(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called without a key")
	})))
	err := runDeployKeyAdd(deployKeyAddCmd, []string{"myws/myrepo"})
	if err == nil {
		t.Error("expected error when no key argument or --key-file given")
	}
}

func TestRunDeployKeyDelete(t *testing.T) {
	defer resetDeployKeyMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	deployKeyNewAPIClient = mockAPIClientFactory(server)
	if err := runDeployKeyDelete(deployKeyDeleteCmd, []string{"myws/myrepo", "7"}); err != nil {
		t.Errorf("runDeployKeyDelete error: %v", err)
	}
}

func TestRunDeployKeyList_NoClient(t *testing.T) {
	defer resetDeployKeyMocks()

	deployKeyNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}
	if err := runDeployKeyList(deployKeyListCmd, []string{"myws/myrepo"}); err == nil {
		t.Error("expected error when client creation fails")
	}
}
