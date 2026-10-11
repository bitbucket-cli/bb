package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

func TestRunPipelineAnalytics(t *testing.T) {
	defer func() { pipelineNewAPIClient = api.NewClient; pipelineAnalyticsLimit = 50 }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[
			{"state":{"name":"COMPLETED","result":{"name":"SUCCESSFUL"}}},
			{"state":{"name":"COMPLETED","result":{"name":"FAILED"}}},
			{"state":{"name":"COMPLETED","result":{"name":"SUCCESSFUL"}}},
			{"state":{"name":"IN_PROGRESS"}}
		]}`))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)
	if err := runPipelineAnalytics(pipelineAnalyticsCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runPipelineAnalytics error: %v", err)
	}
}

func TestRunPipelineAnalytics_Empty(t *testing.T) {
	defer func() { pipelineNewAPIClient = api.NewClient }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[]}`))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)
	if err := runPipelineAnalytics(pipelineAnalyticsCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runPipelineAnalytics error: %v", err)
	}
}
