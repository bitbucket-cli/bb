package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func resetPipelineScheduleMocks() {
	pipelineNewAPIClient = api.NewClient
}

func TestPipelineScheduleAndCacheStructure(t *testing.T) {
	// schedule subcommands
	for _, name := range []string{"list", "view", "delete"} {
		found := false
		for _, c := range pipelineScheduleCmd.Commands() {
			if c.Name() == name {
				found = true
			}
		}
		if !found {
			t.Errorf("pipeline schedule missing subcommand %q", name)
		}
	}
	// cache subcommands
	for _, name := range []string{"list", "delete"} {
		found := false
		for _, c := range pipelineCacheCmd.Commands() {
			if c.Name() == name {
				found = true
			}
		}
		if !found {
			t.Errorf("pipeline cache missing subcommand %q", name)
		}
	}
}

func TestRunPipelineScheduleList(t *testing.T) {
	defer resetPipelineScheduleMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"uuid":"{s1}","enabled":true,"cron_pattern":"0 0 * * *","target":{"ref_name":"main"}}]}`))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)
	if err := runPipelineScheduleList(pipelineScheduleListCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runPipelineScheduleList error: %v", err)
	}
}

func TestRunPipelineScheduleDelete(t *testing.T) {
	defer resetPipelineScheduleMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)
	if err := runPipelineScheduleDelete(pipelineScheduleDeleteCmd, []string{"myws/myrepo", "{s1}"}); err != nil {
		t.Errorf("runPipelineScheduleDelete error: %v", err)
	}
}

func TestRunPipelineCacheList(t *testing.T) {
	defer resetPipelineScheduleMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[{"uuid":"{c1}","name":"node-modules","path":"node_modules","file_size_bytes":4096}]}`))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)
	if err := runPipelineCacheList(pipelineCacheListCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runPipelineCacheList error: %v", err)
	}
}

func TestRunPipelineCacheDelete(t *testing.T) {
	defer resetPipelineScheduleMocks()

	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		gotQuery = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)
	if err := runPipelineCacheDelete(pipelineCacheDeleteCmd, []string{"myws/myrepo", "node-modules"}); err != nil {
		t.Errorf("runPipelineCacheDelete error: %v", err)
	}
	if gotQuery != "node-modules" {
		t.Errorf("delete query name = %q, want node-modules", gotQuery)
	}
}
