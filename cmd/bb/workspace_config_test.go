package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

func TestRunWorkspaceHookList(t *testing.T) {
	defer func() { workspaceNewAPIClient = api.NewClient }()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"values":[{"uuid":"{h}","url":"https://x","active":true,"events":["repo:push"]}]}`))
	}))
	defer srv.Close()
	workspaceNewAPIClient = mockAPIClientFactory(srv)
	if err := runWorkspaceHookList(workspaceHookListCmd, []string{"myws"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/workspaces/myws/hooks") {
		t.Errorf("path=%q", gotPath)
	}
}

func TestRunWorkspaceVarSet(t *testing.T) {
	defer func() { workspaceNewAPIClient = api.NewClient }()
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"uuid":"{v}"}`))
	}))
	defer srv.Close()
	workspaceNewAPIClient = mockAPIClientFactory(srv)
	if err := runWorkspaceVarSet(workspaceVarSetCmd, []string{"myws", "KEY", "VAL"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if gotMethod != http.MethodPost || !strings.Contains(gotPath, "/workspaces/myws/pipelines-config/variables") {
		t.Errorf("got %s %q", gotMethod, gotPath)
	}
}
