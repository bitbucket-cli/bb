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

func resetDownloadMocks() {
	downloadNewAPIClient = api.NewClient
	downloadOutput = ""
}

func TestDownloadCommandStructure(t *testing.T) {
	if downloadCmd.Short == "" {
		t.Error("download command should have a short description")
	}
	expected := []string{"list", "upload", "get", "delete"}
	for _, name := range expected {
		found := false
		for _, c := range downloadCmd.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected download subcommand %q not found", name)
		}
	}
}

func TestRunDownloadList(t *testing.T) {
	defer resetDownloadMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"name":"app.zip","size":2048,"downloads":3,"created_on":"2026-01-01"}]}`))
	}))
	defer server.Close()

	downloadNewAPIClient = mockAPIClientFactory(server)
	if err := runDownloadList(downloadListCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runDownloadList error: %v", err)
	}
}

func TestRunDownloadUpload(t *testing.T) {
	defer resetDownloadMocks()

	var gotMethod, gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "artifact.bin")
	if err := os.WriteFile(path, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}

	downloadNewAPIClient = mockAPIClientFactory(server)
	if err := runDownloadUpload(downloadUploadCmd, []string{"myws/myrepo", path}); err != nil {
		t.Fatalf("runDownloadUpload error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Errorf("content-type = %q, want multipart/form-data", gotContentType)
	}
}

func TestRunDownloadGet(t *testing.T) {
	defer resetDownloadMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("file-content"))
	}))
	defer server.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")

	downloadNewAPIClient = mockAPIClientFactory(server)
	downloadOutput = dest
	if err := runDownloadGet(downloadGetCmd, []string{"myws/myrepo", "app.zip"}); err != nil {
		t.Fatalf("runDownloadGet error: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(got) != "file-content" {
		t.Errorf("downloaded content = %q, want %q", got, "file-content")
	}
}

func TestRunDownloadDelete(t *testing.T) {
	defer resetDownloadMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	downloadNewAPIClient = mockAPIClientFactory(server)
	if err := runDownloadDelete(downloadDeleteCmd, []string{"myws/myrepo", "app.zip"}); err != nil {
		t.Errorf("runDownloadDelete error: %v", err)
	}
}

func TestRunDownloadList_NoClient(t *testing.T) {
	defer resetDownloadMocks()

	downloadNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}
	if err := runDownloadList(downloadListCmd, []string{"myws/myrepo"}); err == nil {
		t.Error("expected error when client creation fails")
	}
}

func TestHumanSize(t *testing.T) {
	cases := map[int64]string{
		512:             "512B",
		2048:            "2.0KB",
		5 * 1024 * 1024: "5.0MB",
	}
	for in, want := range cases {
		if got := humanSize(in); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", in, got, want)
		}
	}
}
