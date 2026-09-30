package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func membersHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/members") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"values": []map[string]interface{}{
				{"user": map[string]interface{}{"display_name": "Dave Riches", "nickname": "driches", "uuid": "{dave-uuid}", "account_id": "712020:dave"}},
				{"user": map[string]interface{}{"display_name": "Dave Smith", "nickname": "dsmith", "uuid": "{dave2-uuid}", "account_id": "712020:dave2"}},
				{"user": map[string]interface{}{"display_name": "Nathan Fletcher", "nickname": "nfletcher", "uuid": "{nathan-uuid}", "account_id": "712020:nathan"}},
			},
		})
	}
}

func TestResolveWorkspaceMember_UUIDPassthrough(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no HTTP call expected for a UUID query, got %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client, err := mockAPIClientFactory(server)()
	if err != nil {
		t.Fatalf("mockAPIClientFactory: %v", err)
	}

	uuid, name, err := resolveWorkspaceMember(t.Context(), client, "myws", "{de3a3453-2e73-4838-8199-e8067ea245ed}")
	if err != nil {
		t.Fatalf("resolveWorkspaceMember: %v", err)
	}
	if uuid != "{de3a3453-2e73-4838-8199-e8067ea245ed}" || name != uuid {
		t.Errorf("got uuid=%q name=%q, want the query passed through unchanged", uuid, name)
	}
}

func TestResolveWorkspaceMember_ExactMatch(t *testing.T) {
	server := httptest.NewServer(membersHandler(t))
	defer server.Close()

	client, err := mockAPIClientFactory(server)()
	if err != nil {
		t.Fatalf("mockAPIClientFactory: %v", err)
	}

	uuid, name, err := resolveWorkspaceMember(t.Context(), client, "myws", "Nathan")
	if err != nil {
		t.Fatalf("resolveWorkspaceMember: %v", err)
	}
	if uuid != "{nathan-uuid}" || name != "Nathan Fletcher" {
		t.Errorf("got uuid=%q name=%q, want {nathan-uuid}/Nathan Fletcher", uuid, name)
	}
}

func TestResolveWorkspaceMember_NoMatch(t *testing.T) {
	server := httptest.NewServer(membersHandler(t))
	defer server.Close()

	client, err := mockAPIClientFactory(server)()
	if err != nil {
		t.Fatalf("mockAPIClientFactory: %v", err)
	}

	if _, _, err := resolveWorkspaceMember(t.Context(), client, "myws", "nobody-like-this"); err == nil {
		t.Fatal("expected an error for a query matching nobody, got nil")
	}
}

func TestResolveWorkspaceMember_Ambiguous(t *testing.T) {
	server := httptest.NewServer(membersHandler(t))
	defer server.Close()

	client, err := mockAPIClientFactory(server)()
	if err != nil {
		t.Fatalf("mockAPIClientFactory: %v", err)
	}

	_, _, err = resolveWorkspaceMember(t.Context(), client, "myws", "Dave")
	if err == nil {
		t.Fatal("expected an error for a query matching more than one member, got nil")
	}
	if !strings.Contains(err.Error(), "Dave Riches") || !strings.Contains(err.Error(), "Dave Smith") {
		t.Errorf("expected error to list both candidates, got: %v", err)
	}
}

func TestMergeReviewers_SkipsExisting(t *testing.T) {
	existing := []reviewerRef{{UUID: "{a}", DisplayName: "A"}}
	merged := mergeReviewers(existing, []reviewerRef{{UUID: "{a}", DisplayName: "A"}, {UUID: "{b}", DisplayName: "B"}})
	if len(merged) != 2 {
		t.Fatalf("got %d reviewers, want 2 (no duplicate of {a}): %+v", len(merged), merged)
	}
}

func TestRemoveReviewers(t *testing.T) {
	reviewers := []reviewerRef{{UUID: "{a}", DisplayName: "Dave Riches"}, {UUID: "{b}", DisplayName: "Nathan Fletcher"}}

	remaining, err := removeReviewers(reviewers, []string{"dave"})
	if err != nil {
		t.Fatalf("removeReviewers: %v", err)
	}
	if len(remaining) != 1 || remaining[0].DisplayName != "Nathan Fletcher" {
		t.Errorf("got %+v, want only Nathan Fletcher remaining", remaining)
	}

	if _, err := removeReviewers(reviewers, []string{"nobody"}); err == nil {
		t.Error("expected an error removing a name that matches no current reviewer")
	}
}

func TestRunPREdit_AddReviewer(t *testing.T) {
	defer func() {
		prNewAPIClient = api.NewClient
		prEditAddReviewers = nil
		prEditTitle = ""
	}()

	var putBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/members"):
			membersHandler(t)(w, r)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pullrequests/"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"title": "existing title", "description": "existing body", "reviewers": []interface{}{},
			})
		case r.Method == http.MethodPut:
			json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	prEditAddReviewers = []string{"Nathan"}

	if err := runPREdit(prEditCmd, []string{"myws/myrepo", "5"}); err != nil {
		t.Fatalf("runPREdit: %v", err)
	}

	if putBody["title"] != "existing title" {
		t.Errorf("title = %v, want it preserved from the GET", putBody["title"])
	}
	reviewers, _ := putBody["reviewers"].([]interface{})
	if len(reviewers) != 1 {
		t.Fatalf("got %d reviewers in PUT body, want 1: %v", len(reviewers), putBody["reviewers"])
	}
	got := reviewers[0].(map[string]interface{})["uuid"]
	if got != "{nathan-uuid}" {
		t.Errorf("reviewer uuid = %v, want {nathan-uuid}", got)
	}
}

func TestRunPREdit_RemoveReviewer(t *testing.T) {
	defer func() {
		prNewAPIClient = api.NewClient
		prEditRemoveReviewers = nil
	}()

	var putBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pullrequests/"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"title": "t", "description": "d",
				"reviewers": []map[string]interface{}{
					{"uuid": "{dave-uuid}", "display_name": "Dave Riches"},
					{"uuid": "{nathan-uuid}", "display_name": "Nathan Fletcher"},
				},
			})
		case r.Method == http.MethodPut:
			json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	prEditRemoveReviewers = []string{"Dave"}

	if err := runPREdit(prEditCmd, []string{"myws/myrepo", "5"}); err != nil {
		t.Fatalf("runPREdit: %v", err)
	}

	reviewers, _ := putBody["reviewers"].([]interface{})
	if len(reviewers) != 1 {
		t.Fatalf("got %d reviewers in PUT body, want 1: %v", len(reviewers), putBody["reviewers"])
	}
	if reviewers[0].(map[string]interface{})["uuid"] != "{nathan-uuid}" {
		t.Errorf("remaining reviewer = %v, want {nathan-uuid}", reviewers[0])
	}
}

func TestRunPREdit_DestDraftCloseSource(t *testing.T) {
	defer func() {
		prNewAPIClient = api.NewClient
		prEditDest = ""
		prEditReady = false
		prEditCmd.Flags().Set("close-source-branch", "false")
		prEditCmd.Flags().Lookup("close-source-branch").Changed = false
	}()

	var putBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{"title": "t", "description": "d"})
		case http.MethodPut:
			json.NewDecoder(r.Body).Decode(&putBody)
			w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	prEditDest = "develop"
	prEditReady = true
	prEditCmd.Flags().Set("close-source-branch", "true")

	if err := runPREdit(prEditCmd, []string{"myws/myrepo", "5"}); err != nil {
		t.Fatalf("runPREdit: %v", err)
	}

	dest, _ := putBody["destination"].(map[string]interface{})
	branch, _ := dest["branch"].(map[string]interface{})
	if branch["name"] != "develop" {
		t.Errorf("destination = %v, want branch develop", putBody["destination"])
	}
	if putBody["draft"] != false {
		t.Errorf("draft = %v, want false", putBody["draft"])
	}
	if putBody["close_source_branch"] != true {
		t.Errorf("close_source_branch = %v, want true", putBody["close_source_branch"])
	}
	if _, ok := putBody["reviewers"]; ok {
		t.Errorf("reviewers sent although not edited: %v", putBody["reviewers"])
	}
}
