package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func resetIssueMocks() {
	issueNewAPIClient = api.NewClient
}

func TestIssueCommandStructure(t *testing.T) {
	if issueCmd.Short == "" {
		t.Error("issue command should have short description")
	}

	subCmds := issueCmd.Commands()
	expectedCmds := []string{"list", "create", "view", "update", "delete", "comment", "comments"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected issue subcommand %q not found", expected)
		}
	}
}

func TestIssueCreateFlags(t *testing.T) {
	titleFlag := issueCreateCmd.Flags().Lookup("title")
	if titleFlag == nil {
		t.Error("--title flag not found")
	}

	kindFlag := issueCreateCmd.Flags().Lookup("kind")
	if kindFlag == nil {
		t.Error("--kind flag not found")
	}

	priorityFlag := issueCreateCmd.Flags().Lookup("priority")
	if priorityFlag == nil {
		t.Error("--priority flag not found")
	}
}

func TestRunIssueList_NoClient(t *testing.T) {
	defer resetIssueMocks()

	issueNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runIssueList(issueListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunIssueList_InvalidRepo(t *testing.T) {
	defer resetIssueMocks()

	issueNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runIssueList(issueListCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunIssueList_APIError(t *testing.T) {
	defer resetIssueMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)

	err := runIssueList(issueListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunIssueList_NoIssues(t *testing.T) {
	defer resetIssueMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)

	err := runIssueList(issueListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunIssueList_WithIssues(t *testing.T) {
	defer resetIssueMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"id":       1,
					"title":    "Test issue",
					"state":    "open",
					"priority": "major",
					"kind":     "bug",
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)

	err := runIssueList(issueListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunIssueCreate_NoClient(t *testing.T) {
	defer resetIssueMocks()

	issueNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runIssueCreate(issueCreateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunIssueCreate_Success(t *testing.T) {
	defer resetIssueMocks()

	origTitle := issueTitle
	issueTitle = "Test issue"
	defer func() { issueTitle = origTitle }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
	}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)

	err := runIssueCreate(issueCreateCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunIssueView_NoClient(t *testing.T) {
	defer resetIssueMocks()

	issueNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runIssueView(issueViewCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunIssueView_Success(t *testing.T) {
	defer resetIssueMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"id":       1,
			"title":    "Test issue",
			"state":    "open",
			"priority": "major",
			"kind":     "bug",
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)

	err := runIssueView(issueViewCmd, []string{"workspace/repo", "1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunIssueUpdate_NoClient(t *testing.T) {
	defer resetIssueMocks()

	issueNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runIssueUpdate(issueUpdateCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunIssueUpdate_NoUpdates(t *testing.T) {
	defer resetIssueMocks()

	// Reset all update flags
	origTitle := issueTitle
	origBody := issueBody
	origKind := issueKind
	origPriority := issuePriority
	origState := issueState
	origAssignee := issueAssignee
	issueTitle = ""
	issueBody = ""
	issueKind = ""
	issuePriority = ""
	issueState = ""
	issueAssignee = ""
	defer func() {
		issueTitle = origTitle
		issueBody = origBody
		issueKind = origKind
		issuePriority = origPriority
		issueState = origState
		issueAssignee = origAssignee
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)

	err := runIssueUpdate(issueUpdateCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error when no updates specified")
	}
}

func TestRunIssueUpdate_Success(t *testing.T) {
	defer resetIssueMocks()

	origState := issueState
	issueState = "resolved"
	defer func() { issueState = origState }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
	}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)

	err := runIssueUpdate(issueUpdateCmd, []string{"workspace/repo", "1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunIssueDelete_NoClient(t *testing.T) {
	defer resetIssueMocks()

	issueNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runIssueDelete(issueDeleteCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunIssueDelete_Success(t *testing.T) {
	defer resetIssueMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)

	err := runIssueDelete(issueDeleteCmd, []string{"workspace/repo", "1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunIssueComment_NoClient(t *testing.T) {
	defer resetIssueMocks()

	issueNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runIssueComment(issueCommentCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunIssueComment_Success(t *testing.T) {
	defer resetIssueMocks()

	origBody := issueCommentBody
	issueCommentBody = "Test comment"
	defer func() { issueCommentBody = origBody }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
	}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)

	err := runIssueComment(issueCommentCmd, []string{"workspace/repo", "1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunIssueComments_NoClient(t *testing.T) {
	defer resetIssueMocks()

	issueNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runIssueComments(issueCommentsCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunIssueComments_NoComments(t *testing.T) {
	defer resetIssueMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)

	err := runIssueComments(issueCommentsCmd, []string{"workspace/repo", "1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunIssueComments_WithComments(t *testing.T) {
	defer resetIssueMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"id": 1,
					"content": map[string]interface{}{
						"raw": "Test comment",
					},
					"user": map[string]interface{}{
						"display_name": "Test User",
					},
					"created_on": "2024-01-01T00:00:00Z",
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	issueNewAPIClient = mockAPIClientFactory(server)

	err := runIssueComments(issueCommentsCmd, []string{"workspace/repo", "1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
