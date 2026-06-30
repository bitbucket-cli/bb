package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func resetSourceMocks() {
	sourceNewAPIClient = api.NewClient
	sourceRef = ""
}

func TestSourceCommandStructure(t *testing.T) {
	for _, name := range []string{"list", "view"} {
		found := false
		for _, c := range sourceCmd.Commands() {
			if c.Name() == name {
				found = true
			}
		}
		if !found {
			t.Errorf("source missing subcommand %q", name)
		}
	}
}

func TestRunSourceList(t *testing.T) {
	defer resetSourceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"path":"main.go","type":"commit_file","size":120},{"path":"pkg","type":"commit_directory"}]}`))
	}))
	defer server.Close()

	sourceNewAPIClient = mockAPIClientFactory(server)
	sourceRef = "main" // avoids the default-branch lookup
	if err := runSourceList(sourceListCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runSourceList error: %v", err)
	}
}

func TestRunSourceView(t *testing.T) {
	defer resetSourceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("package main\n"))
	}))
	defer server.Close()

	sourceNewAPIClient = mockAPIClientFactory(server)
	sourceRef = "main"
	if err := runSourceView(sourceViewCmd, []string{"myws/myrepo", "main.go"}); err != nil {
		t.Errorf("runSourceView error: %v", err)
	}
}

func TestResolveSourceRef_DefaultBranch(t *testing.T) {
	defer resetSourceMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"mainbranch":{"name":"develop"}}`))
	}))
	defer server.Close()

	client, _ := mockAPIClientFactory(server)()
	repoCtx := &api.RepoContext{Workspace: "myws", RepoSlug: "myrepo"}
	ref, err := resolveSourceRef(sourceListCmd, client, repoCtx)
	if err != nil || ref != "develop" {
		t.Errorf("resolveSourceRef = %q, %v; want develop", ref, err)
	}
}
