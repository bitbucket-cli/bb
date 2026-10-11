package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

func TestRunPRThreads(t *testing.T) {
	defer func() { prNewAPIClient = api.NewClient }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[
			{"id":1,"content":{"raw":"top level"},"user":{"display_name":"Tom"}},
			{"id":2,"content":{"raw":"a reply"},"user":{"display_name":"Ann"},"parent":{"id":1}},
			{"id":3,"content":{"raw":"inline note"},"user":{"display_name":"Tom"},"inline":{"path":"main.go","to":10}},
			{"id":4,"content":{"raw":"deleted"},"user":{"display_name":"X"},"deleted":true}
		]}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	if err := runPRThreads(prThreadsCmd, []string{"myws/myrepo", "5"}); err != nil {
		t.Errorf("runPRThreads error: %v", err)
	}
}

func TestRunPRThreads_BadNumber(t *testing.T) {
	defer func() { prNewAPIClient = api.NewClient }()
	prNewAPIClient = mockAPIClientFactory(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	if err := runPRThreads(prThreadsCmd, []string{"myws/myrepo", "abc"}); err == nil {
		t.Error("expected error for non-numeric pr-number")
	}
}
