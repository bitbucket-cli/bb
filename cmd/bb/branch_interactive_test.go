package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

func TestSelectBranchInteractively_FetchesAndErrorsOffTTY(t *testing.T) {
	defer func() { branchNewAPIClient = api.NewClient }()

	var listed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		listed = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"name":"main"},{"name":"dev"}]}`))
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)
	client, _ := branchNewAPIClient()
	repoCtx := &api.RepoContext{Workspace: "myws", RepoSlug: "myrepo"}

	// Off a TTY the selector returns an error, but the branch list must have been
	// fetched first.
	_, err := selectBranchInteractively(branchDeleteCmd, client, repoCtx)
	if !listed {
		t.Error("expected branches to be listed before prompting")
	}
	if err == nil {
		t.Error("expected an error when not running in a terminal")
	}
}

func TestSelectBranchInteractively_NoBranches(t *testing.T) {
	defer func() { branchNewAPIClient = api.NewClient }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[]}`))
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)
	client, _ := branchNewAPIClient()
	repoCtx := &api.RepoContext{Workspace: "myws", RepoSlug: "myrepo"}

	if _, err := selectBranchInteractively(branchDeleteCmd, client, repoCtx); err == nil {
		t.Error("expected an error when there are no branches")
	}
}
