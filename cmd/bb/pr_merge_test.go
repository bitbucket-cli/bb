package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func TestParseMergeStrategy(t *testing.T) {
	if s, err := parseMergeStrategy("squash"); err != nil || s != "squash" {
		t.Errorf("parseMergeStrategy(squash) = %q, %v", s, err)
	}
	if _, err := parseMergeStrategy("sqaush"); err == nil {
		t.Error("expected an error for an unknown strategy")
	}
}

func TestRunPRMerge_StrategyAndMessage(t *testing.T) {
	defer func() {
		prNewAPIClient = api.NewClient
		prMergeStrategy = ""
		prMergeMessage = ""
		prMergeCmd.Flags().Set("close-source-branch", "false")
		prMergeCmd.Flags().Lookup("close-source-branch").Changed = false
	}()

	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repositories/myws/myrepo/pullrequests/7/merge" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&body)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	prMergeStrategy = "squash"
	prMergeMessage = "Squashed"
	prMergeCmd.Flags().Set("close-source-branch", "false")

	if err := runPRMerge(prMergeCmd, []string{"myws/myrepo", "7"}); err != nil {
		t.Fatalf("runPRMerge: %v", err)
	}
	if body["merge_strategy"] != "squash" || body["message"] != "Squashed" || body["close_source_branch"] != false {
		t.Errorf("merge body = %v", body)
	}
}

func TestRunPRMerge_DefaultsOmitted(t *testing.T) {
	defer func() { prNewAPIClient = api.NewClient }()

	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	if err := runPRMerge(prMergeCmd, []string{"myws/myrepo", "7"}); err != nil {
		t.Fatalf("runPRMerge: %v", err)
	}
	for _, k := range []string{"merge_strategy", "message", "close_source_branch"} {
		if _, ok := body[k]; ok {
			t.Errorf("%s sent without its flag: %v", k, body)
		}
	}
}

func TestRunPRApprove_Remove(t *testing.T) {
	defer func() {
		prNewAPIClient = api.NewClient
		prApproveRemove = false
	}()

	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	prApproveRemove = true
	if err := runPRApprove(prApproveCmd, []string{"myws/myrepo", "7"}); err != nil {
		t.Fatalf("runPRApprove: %v", err)
	}
	if method != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", method)
	}
}

func TestRunPRCreate_DraftCloseSource(t *testing.T) {
	defer resetPRMocks()
	prTitle, prSource, prDest, prDraft, prCloseSource = "T", "feature", "main", true, true
	defer func() { prTitle, prSource, prDest, prDraft, prCloseSource = "", "", "", false, false }()

	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			json.NewDecoder(r.Body).Decode(&body)
			w.WriteHeader(http.StatusCreated)
		}
		w.Write([]byte(`{"id": 1, "links": {"html": {"href": "x"}}}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	if err := runPRCreate(prCreateCmd, []string{"myws/myrepo"}); err != nil {
		t.Fatalf("runPRCreate: %v", err)
	}
	if body["draft"] != true || body["close_source_branch"] != true {
		t.Errorf("create body = %v", body)
	}
}
