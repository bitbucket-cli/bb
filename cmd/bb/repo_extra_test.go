package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func resetRepoExtraMocks() {
	newAPIClient = api.NewClient
	repoPermissionsGroups = false
}

func TestRunRepoBranchingModel(t *testing.T) {
	defer resetRepoExtraMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"development":{"branch":{"name":"main"}},"production":{"branch":{"name":"release"}},"branch_types":[{"kind":"feature","prefix":"feature/"}]}`))
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)
	if err := runRepoBranchingModel(repoBranchingModelCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runRepoBranchingModel error: %v", err)
	}
}

func TestRunRepoPermissions(t *testing.T) {
	defer resetRepoExtraMocks()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"permission":"admin","user":{"display_name":"Tom"}}]}`))
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)
	if err := runRepoPermissions(repoPermissionsCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runRepoPermissions error: %v", err)
	}
	if wantSuffix := "/permissions-config/users"; gotPath[len(gotPath)-len(wantSuffix):] != wantSuffix {
		t.Errorf("path = %q, want users endpoint", gotPath)
	}

	// --groups switches to the groups endpoint.
	repoPermissionsGroups = true
	if err := runRepoPermissions(repoPermissionsCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runRepoPermissions groups error: %v", err)
	}
	if wantSuffix := "/permissions-config/groups"; gotPath[len(gotPath)-len(wantSuffix):] != wantSuffix {
		t.Errorf("path = %q, want groups endpoint", gotPath)
	}
}

func TestRunRepoForks(t *testing.T) {
	defer resetRepoExtraMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"full_name":"other/myrepo","is_private":true,"owner":{"display_name":"Other"}}]}`))
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)
	if err := runRepoForks(repoForksCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runRepoForks error: %v", err)
	}
}
