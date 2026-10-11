package main

import (
	"encoding/json"
	"net/http"
	"testing"

	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

func TestRunPRComment_ReplyTo(t *testing.T) {
	defer func() {
		prNewAPIClient = api.NewClient
		prComment = ""
		prCommentReply = 0
	}()

	var got map[string]interface{}
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repositories/myws/myrepo/pullrequests/5/comments" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":2}`))
	})
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	prComment = "agreed"
	prCommentReply = 99
	if err := runPRComment(prCommentCmd, []string{"myws/myrepo", "5"}); err != nil {
		t.Fatalf("runPRComment: %v", err)
	}
	parent, _ := got["parent"].(map[string]interface{})
	if parent["id"] != float64(99) {
		t.Errorf("parent = %v, want id 99", got["parent"])
	}
	if _, ok := got["inline"]; ok {
		t.Errorf("reply sent an inline anchor: %v", got["inline"])
	}
}

func TestRunPRCommentEdit(t *testing.T) {
	defer func() {
		prNewAPIClient = api.NewClient
		prCommentEditBody = ""
	}()

	var got map[string]interface{}
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/repositories/myws/myrepo/pullrequests/5/comments/42" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"id":42}`))
	})
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	prCommentEditBody = "fixed typo"
	if err := runPRCommentEdit(prCommentEditCmd, []string{"myws/myrepo", "5", "42"}); err != nil {
		t.Fatalf("runPRCommentEdit: %v", err)
	}
	content, _ := got["content"].(map[string]interface{})
	if content["raw"] != "fixed typo" {
		t.Errorf("content = %v, want raw %q", got["content"], "fixed typo")
	}
}

func TestRunPRCommentDelete(t *testing.T) {
	defer func() { prNewAPIClient = api.NewClient }()

	var gotMethod, gotPath string
	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	if err := runPRCommentDelete(prCommentDeleteCmd, []string{"myws/myrepo", "5", "42"}); err != nil {
		t.Fatalf("runPRCommentDelete: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/repositories/myws/myrepo/pullrequests/5/comments/42" {
		t.Errorf("got %s %s, want DELETE .../pullrequests/5/comments/42", gotMethod, gotPath)
	}
}

func TestRunPRCommentDelete_APIError(t *testing.T) {
	defer func() { prNewAPIClient = api.NewClient }()

	server := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	if err := runPRCommentDelete(prCommentDeleteCmd, []string{"myws/myrepo", "5", "42"}); err == nil {
		t.Fatal("expected an error for a 403")
	}
}
