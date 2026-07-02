package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func TestRunUser_Current(t *testing.T) {
	defer func() { userNewAPIClient = api.NewClient }()
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"display_name":"Tom","uuid":"{abc}","account_id":"123"}`))
	}))
	defer server.Close()
	userNewAPIClient = mockAPIClientFactory(server)
	if err := runUser(userCmd, nil); err != nil {
		t.Fatalf("runUser error: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/user") {
		t.Errorf("path = %q, want /user", gotPath)
	}
}

func TestRunUser_Selected(t *testing.T) {
	defer func() { userNewAPIClient = api.NewClient }()
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"display_name":"Alice"}`))
	}))
	defer server.Close()
	userNewAPIClient = mockAPIClientFactory(server)
	if err := runUser(userCmd, []string{"{uuid}"}); err != nil {
		t.Fatalf("runUser error: %v", err)
	}
	if !strings.Contains(gotPath, "/users/") {
		t.Errorf("path = %q, want /users/...", gotPath)
	}
}
