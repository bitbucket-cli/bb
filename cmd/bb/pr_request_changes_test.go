package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func TestRunPRRequestChanges(t *testing.T) {
	defer func() {
		prNewAPIClient = api.NewClient
		prRequestChangesRemove = false
	}()

	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	// Default: POST request-changes.
	if err := runPRRequestChanges(prRequestChangesCmd, []string{"myws/myrepo", "5"}); err != nil {
		t.Fatalf("runPRRequestChanges error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}

	// --remove: DELETE request-changes.
	prRequestChangesRemove = true
	if err := runPRRequestChanges(prRequestChangesCmd, []string{"myws/myrepo", "5"}); err != nil {
		t.Fatalf("runPRRequestChanges --remove error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
}
