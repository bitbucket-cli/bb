package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunCommitStatus(t *testing.T) {
	defer resetCommitMocks()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"key":"PIPELINE","name":"Build","state":"SUCCESSFUL","url":"https://ci/1"}]}`))
	}))
	defer server.Close()

	commitNewAPIClient = mockAPIClientFactory(server)
	if err := runCommitStatus(commitStatusCmd, []string{"myws/myrepo", "abc123"}); err != nil {
		t.Fatalf("runCommitStatus error: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/commit/abc123/statuses") {
		t.Errorf("path = %q, want suffix /commit/abc123/statuses", gotPath)
	}
}

func TestRunPRChecks(t *testing.T) {
	defer resetPRMocks()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"key":"PIPELINE","name":"Build","state":"INPROGRESS","url":"https://ci/2"}]}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	if err := runPRChecks(prChecksCmd, []string{"myws/myrepo", "42"}); err != nil {
		t.Fatalf("runPRChecks error: %v", err)
	}
	if !strings.Contains(gotPath, "/pullrequests/42/statuses") {
		t.Errorf("path = %q, want to contain /pullrequests/42/statuses", gotPath)
	}
}
