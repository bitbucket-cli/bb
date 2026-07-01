package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func resetCommitMocks() {
	commitNewAPIClient = api.NewClient
	commitBranch = ""
	commitLimit = 25
}

func TestCommitCommandStructure(t *testing.T) {
	for _, name := range []string{"list", "view"} {
		found := false
		for _, c := range commitCmd.Commands() {
			if c.Name() == name {
				found = true
			}
		}
		if !found {
			t.Errorf("commit missing subcommand %q", name)
		}
	}
}

func TestRunCommitList(t *testing.T) {
	defer resetCommitMocks()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"hash":"abcdef1234567890","message":"fix bug\n\ndetail","date":"2026-01-01","author":{"user":{"display_name":"Tom"}}}]}`))
	}))
	defer server.Close()

	commitNewAPIClient = mockAPIClientFactory(server)
	if err := runCommitList(commitListCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runCommitList error: %v", err)
	}
	if gotPath == "" {
		t.Error("expected the commits endpoint to be called")
	}
}

func TestRunCommitList_Branch(t *testing.T) {
	defer resetCommitMocks()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[]}`))
	}))
	defer server.Close()

	commitNewAPIClient = mockAPIClientFactory(server)
	commitBranch = "main"
	if err := runCommitList(commitListCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runCommitList error: %v", err)
	}
	if wantSuffix := "/commits/main"; gotPath[len(gotPath)-len(wantSuffix):] != wantSuffix {
		t.Errorf("path = %q, want suffix %q", gotPath, wantSuffix)
	}
}

func TestRunCommitView(t *testing.T) {
	defer resetCommitMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"hash":"abcdef1234567890","message":"a commit","date":"2026-01-01","author":{"raw":"Tom <tom@example.com>"}}`))
	}))
	defer server.Close()

	commitNewAPIClient = mockAPIClientFactory(server)
	if err := runCommitView(commitViewCmd, []string{"myws/myrepo", "abcdef1234567890"}); err != nil {
		t.Errorf("runCommitView error: %v", err)
	}
}

func TestRunCommitActivity(t *testing.T) {
	defer resetCommitMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[
			{"hash":"a1","author":{"user":{"display_name":"Tom"}}},
			{"hash":"a2","author":{"user":{"display_name":"Tom"}}},
			{"hash":"a3","author":{"raw":"Other <o@x>"}}
		]}`))
	}))
	defer server.Close()

	commitNewAPIClient = mockAPIClientFactory(server)
	if err := runCommitActivity(commitActivityCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runCommitActivity error: %v", err)
	}
}

func TestCommitHelpers(t *testing.T) {
	if got := shortHash("abcdef1234567890"); got != "abcdef123456" {
		t.Errorf("shortHash = %q, want abcdef123456", got)
	}
	if got := shortHash("abc"); got != "abc" {
		t.Errorf("shortHash short = %q, want abc", got)
	}
	if got := firstLine("line1\nline2"); got != "line1" {
		t.Errorf("firstLine = %q, want line1", got)
	}

	c := commitSummary{}
	c.Author.Raw = "Raw Name"
	if got := c.authorName(); got != "Raw Name" {
		t.Errorf("authorName raw = %q", got)
	}
	c.Author.User = &struct {
		DisplayName string `json:"display_name"`
	}{DisplayName: "Display"}
	if got := c.authorName(); got != "Display" {
		t.Errorf("authorName user = %q, want Display", got)
	}
}
