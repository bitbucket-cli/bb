package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func TestRunIssueBulk(t *testing.T) {
	defer func() { issueNewAPIClient = api.NewClient; issueBulkState = "" }()

	var updates int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		atomic.AddInt32(&updates, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)
	issueBulkState = "resolved"
	if err := runIssueBulk(issueBulkCmd, []string{"myws/myrepo", "1", "2"}); err != nil {
		t.Fatalf("runIssueBulk error: %v", err)
	}
	if updates != 2 {
		t.Errorf("updates = %d, want 2", updates)
	}
}

func TestRunIssueBulk_Failure(t *testing.T) {
	defer func() { issueNewAPIClient = api.NewClient; issueBulkState = "" }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)
	issueBulkState = "resolved"
	if err := runIssueBulk(issueBulkCmd, []string{"myws/myrepo", "1"}); err == nil {
		t.Error("expected an error when an update fails")
	}
}
