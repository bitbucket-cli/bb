package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func TestRunPRRequestChanges_WithComment(t *testing.T) {
	defer func() {
		prNewAPIClient = api.NewClient
		prRequestChangesRemove = false
		prRequestChangesComment = ""
	}()

	var sawComment, sawRequestChanges bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/comments") {
			sawComment = true
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":1}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/request-changes") {
			sawRequestChanges = true
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	prRequestChangesComment = "please fix the tests"
	if err := runPRRequestChanges(prRequestChangesCmd, []string{"myws/myrepo", "5"}); err != nil {
		t.Fatalf("runPRRequestChanges error: %v", err)
	}
	if !sawComment {
		t.Error("expected a comment to be posted with --comment")
	}
	if !sawRequestChanges {
		t.Error("expected request-changes to be posted")
	}
}

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
