package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func TestRunProjectList(t *testing.T) {
	defer func() { projectNewAPIClient = api.NewClient }()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"key":"CLOUD","name":"Cloud","is_private":true,"description":"x"}]}`))
	}))
	defer server.Close()

	projectNewAPIClient = mockAPIClientFactory(server)
	if err := runProjectList(projectListCmd, []string{"myws"}); err != nil {
		t.Fatalf("runProjectList error: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/workspaces/myws/projects") {
		t.Errorf("path = %q, want .../workspaces/myws/projects", gotPath)
	}
}

func TestRunProjectCreate(t *testing.T) {
	defer func() { projectNewAPIClient = api.NewClient; projectName = ""; projectDesc = "" }()

	var gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"key":"CLOUD"}`))
	}))
	defer server.Close()

	projectNewAPIClient = mockAPIClientFactory(server)
	if err := runProjectCreate(projectCreateCmd, []string{"myws", "CLOUD"}); err != nil {
		t.Fatalf("runProjectCreate error: %v", err)
	}
	if gotMethod != http.MethodPost || !strings.HasSuffix(gotPath, "/workspaces/myws/projects") {
		t.Errorf("got %s %q, want POST .../workspaces/myws/projects", gotMethod, gotPath)
	}
}
