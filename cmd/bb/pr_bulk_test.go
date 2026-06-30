package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func TestRunPRBulkApprove(t *testing.T) {
	defer func() { prNewAPIClient = api.NewClient }()

	var approvals int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&approvals, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	if err := runPRBulk(prBulkApproveCmd, []string{"myws/myrepo", "1", "2", "3"}, "approve"); err != nil {
		t.Fatalf("runPRBulk error: %v", err)
	}
	if approvals != 3 {
		t.Errorf("approvals = %d, want 3", approvals)
	}
}

func TestRunPRBulk_ReportsFailures(t *testing.T) {
	defer func() { prNewAPIClient = api.NewClient }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	err := runPRBulk(prBulkDeclineCmd, []string{"myws/myrepo", "1", "2"}, "decline")
	if err == nil {
		t.Error("expected an error when PRs fail")
	}
}

func TestRunPRBulk_BadID(t *testing.T) {
	defer func() { prNewAPIClient = api.NewClient }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	// "x" is not a number -> counts as a failure.
	if err := runPRBulk(prBulkApproveCmd, []string{"myws/myrepo", "x"}, "approve"); err == nil {
		t.Error("expected an error for a non-numeric PR id")
	}
}
