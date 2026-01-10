package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tom/bb/generated/bitbucket"
	"github.com/tom/bb/internal/api"
)

// resetBranchMocks restores default implementations
func resetBranchMocks() {
	branchNewAPIClient = api.NewClient
}

func TestRunBranchList_NoClient(t *testing.T) {
	defer resetBranchMocks()

	branchNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runBranchList(branchListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunBranchList_InvalidRepo(t *testing.T) {
	defer resetBranchMocks()

	branchNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runBranchList(branchListCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunBranchCreate_NoClient(t *testing.T) {
	defer resetBranchMocks()

	branchNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runBranchCreate(branchCreateCmd, []string{"workspace/repo", "new-branch"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunBranchCreate_InvalidRepo(t *testing.T) {
	defer resetBranchMocks()

	branchNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runBranchCreate(branchCreateCmd, []string{"invalid", "branch"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunBranchDelete_NoClient(t *testing.T) {
	defer resetBranchMocks()

	branchNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runBranchDelete(branchDeleteCmd, []string{"workspace/repo", "branch"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunBranchDelete_InvalidRepo(t *testing.T) {
	defer resetBranchMocks()

	branchNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runBranchDelete(branchDeleteCmd, []string{"invalid", "branch"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestBranchCommandStructure(t *testing.T) {
	if branchCmd.Short == "" {
		t.Error("branch command should have short description")
	}

	if branchListCmd.RunE == nil {
		t.Error("branch list should have RunE")
	}

	if branchCreateCmd.RunE == nil {
		t.Error("branch create should have RunE")
	}

	if branchDeleteCmd.RunE == nil {
		t.Error("branch delete should have RunE")
	}
}

func TestBranchFlags(t *testing.T) {
	// Test --from flag exists on create command
	fromFlag := branchCreateCmd.Flags().Lookup("from")
	if fromFlag == nil {
		t.Error("--from flag not found on branch create command")
	}
}

func TestBranchSubcommands(t *testing.T) {
	subCmds := branchCmd.Commands()
	expectedCmds := []string{"list", "create", "delete"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected branch subcommand %q not found", expected)
		}
	}
}

func TestRunBranchList_APIError(t *testing.T) {
	defer resetBranchMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchList(branchListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunBranchList_NoBranches(t *testing.T) {
	defer resetBranchMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := bitbucket.PaginatedBranches{}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchList(branchListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunBranchList_WithBranches(t *testing.T) {
	defer resetBranchMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := "main"
		hash := "abc123def456"
		message := "Initial commit"
		branches := []bitbucket.Branch{
			{
				Name: &name,
				Target: &bitbucket.Commit{
					Hash:    &hash,
					Message: &message,
				},
			},
		}
		result := bitbucket.PaginatedBranches{Values: &branches}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchList(branchListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunBranchList_InvalidJSON(t *testing.T) {
	defer resetBranchMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchList(branchListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunBranchDelete_APIError(t *testing.T) {
	defer resetBranchMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "forbidden"})
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchDelete(branchDeleteCmd, []string{"workspace/repo", "feature"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunBranchDelete_Success(t *testing.T) {
	defer resetBranchMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchDelete(branchDeleteCmd, []string{"workspace/repo", "feature"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunBranchCreate_GetRepoFails(t *testing.T) {
	defer resetBranchMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	// Without --from, it tries to get repo first
	err := runBranchCreate(branchCreateCmd, []string{"workspace/repo", "new-branch"})
	if err == nil {
		t.Error("Expected error when repo lookup fails")
	}
}

func TestRunBranchCreate_SourceBranchNotFound(t *testing.T) {
	defer resetBranchMocks()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			// First request: get repo info
			mainBranch := "main"
			repo := bitbucket.Repository{
				Mainbranch: &bitbucket.Branch{Name: &mainBranch},
			}
			json.NewEncoder(w).Encode(repo)
		} else {
			// Second request: get source branch - not found
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchCreate(branchCreateCmd, []string{"workspace/repo", "new-branch"})
	if err == nil {
		t.Error("Expected error when source branch not found")
	}
}

func TestRunBranchCreate_NoTargetHash(t *testing.T) {
	defer resetBranchMocks()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			mainBranch := "main"
			repo := bitbucket.Repository{
				Mainbranch: &bitbucket.Branch{Name: &mainBranch},
			}
			json.NewEncoder(w).Encode(repo)
		} else if requestCount == 2 {
			// Branch exists but no target hash
			branch := bitbucket.Branch{Target: nil}
			json.NewEncoder(w).Encode(branch)
		}
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchCreate(branchCreateCmd, []string{"workspace/repo", "new-branch"})
	if err == nil {
		t.Error("Expected error when no target hash")
	}
}

func TestRunBranchCreate_Success(t *testing.T) {
	defer resetBranchMocks()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			mainBranch := "main"
			repo := bitbucket.Repository{
				Mainbranch: &bitbucket.Branch{Name: &mainBranch},
			}
			json.NewEncoder(w).Encode(repo)
		} else if requestCount == 2 {
			hash := "abc123"
			branch := bitbucket.Branch{
				Target: &bitbucket.Commit{Hash: &hash},
			}
			json.NewEncoder(w).Encode(branch)
		} else {
			// Create branch
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(bitbucket.Branch{})
		}
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchCreate(branchCreateCmd, []string{"workspace/repo", "new-branch"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunBranchCreate_APIError(t *testing.T) {
	defer resetBranchMocks()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			mainBranch := "main"
			repo := bitbucket.Repository{
				Mainbranch: &bitbucket.Branch{Name: &mainBranch},
			}
			json.NewEncoder(w).Encode(repo)
		} else if requestCount == 2 {
			hash := "abc123"
			branch := bitbucket.Branch{
				Target: &bitbucket.Commit{Hash: &hash},
			}
			json.NewEncoder(w).Encode(branch)
		} else {
			// Create branch fails
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "branch exists"})
		}
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchCreate(branchCreateCmd, []string{"workspace/repo", "new-branch"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunBranchCreate_WithFromFlag(t *testing.T) {
	defer resetBranchMocks()

	// Save and restore branchFrom
	origFrom := branchFrom
	branchFrom = "develop"
	defer func() { branchFrom = origFrom }()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			// Get source branch (not repo since --from is specified)
			hash := "abc123"
			branch := bitbucket.Branch{
				Target: &bitbucket.Commit{Hash: &hash},
			}
			json.NewEncoder(w).Encode(branch)
		} else {
			// Create branch
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(bitbucket.Branch{})
		}
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchCreate(branchCreateCmd, []string{"workspace/repo", "new-branch"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunBranchCreate_NoMainBranch(t *testing.T) {
	defer resetBranchMocks()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			// Repo with no mainbranch - should fall back to "main"
			repo := bitbucket.Repository{}
			json.NewEncoder(w).Encode(repo)
		} else if requestCount == 2 {
			hash := "abc123"
			branch := bitbucket.Branch{
				Target: &bitbucket.Commit{Hash: &hash},
			}
			json.NewEncoder(w).Encode(branch)
		} else {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(bitbucket.Branch{})
		}
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchCreate(branchCreateCmd, []string{"workspace/repo", "new-branch"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunBranchCreate_InvalidRepoJSON(t *testing.T) {
	defer resetBranchMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchCreate(branchCreateCmd, []string{"workspace/repo", "new-branch"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunBranchCreate_InvalidBranchJSON(t *testing.T) {
	defer resetBranchMocks()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			mainBranch := "main"
			repo := bitbucket.Repository{
				Mainbranch: &bitbucket.Branch{Name: &mainBranch},
			}
			json.NewEncoder(w).Encode(repo)
		} else {
			w.Write([]byte("invalid json"))
		}
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchCreate(branchCreateCmd, []string{"workspace/repo", "new-branch"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

// --- Branch Restriction Tests ---

func resetBranchRestrictionMocks() {
	branchNewAPIClient = api.NewClient
	restrictionKind = ""
	restrictionPattern = ""
	restrictionValue = 0
	restrictionMatchKind = "glob"
}

func TestBranchRestrictCommandStructure(t *testing.T) {
	subCmds := branchRestrictCmd.Commands()
	expectedCmds := []string{"list", "create", "delete", "view"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected branch restrict subcommand %q not found", expected)
		}
	}
}

func TestBranchRestrictCreateFlags(t *testing.T) {
	kindFlag := branchRestrictCreateCmd.Flags().Lookup("kind")
	if kindFlag == nil {
		t.Error("--kind flag not found")
	}

	patternFlag := branchRestrictCreateCmd.Flags().Lookup("pattern")
	if patternFlag == nil {
		t.Error("--pattern flag not found")
	}

	valueFlag := branchRestrictCreateCmd.Flags().Lookup("value")
	if valueFlag == nil {
		t.Error("--value flag not found")
	}
}

func TestRunBranchRestrictList_NoClient(t *testing.T) {
	defer resetBranchRestrictionMocks()

	branchNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runBranchRestrictList(branchRestrictListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunBranchRestrictList_InvalidRepo(t *testing.T) {
	defer resetBranchRestrictionMocks()

	branchNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runBranchRestrictList(branchRestrictListCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunBranchRestrictList_APIError(t *testing.T) {
	defer resetBranchRestrictionMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchRestrictList(branchRestrictListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunBranchRestrictList_NoRestrictions(t *testing.T) {
	defer resetBranchRestrictionMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchRestrictList(branchRestrictListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunBranchRestrictList_WithRestrictions(t *testing.T) {
	defer resetBranchRestrictionMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := 2
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"id":               1,
					"kind":             "require_approvals_to_merge",
					"pattern":          "main",
					"branch_match_kind": "glob",
					"value":            value,
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchRestrictList(branchRestrictListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunBranchRestrictCreate_NoClient(t *testing.T) {
	defer resetBranchRestrictionMocks()

	branchNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runBranchRestrictCreate(branchRestrictCreateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunBranchRestrictCreate_Success(t *testing.T) {
	defer resetBranchRestrictionMocks()

	restrictionKind = "require_approvals_to_merge"
	restrictionPattern = "main"
	restrictionValue = 2

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchRestrictCreate(branchRestrictCreateCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunBranchRestrictCreate_APIError(t *testing.T) {
	defer resetBranchRestrictionMocks()

	restrictionKind = "push"
	restrictionPattern = "main"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid"})
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchRestrictCreate(branchRestrictCreateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunBranchRestrictDelete_NoClient(t *testing.T) {
	defer resetBranchRestrictionMocks()

	branchNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runBranchRestrictDelete(branchRestrictDeleteCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunBranchRestrictDelete_Success(t *testing.T) {
	defer resetBranchRestrictionMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchRestrictDelete(branchRestrictDeleteCmd, []string{"workspace/repo", "1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunBranchRestrictDelete_APIError(t *testing.T) {
	defer resetBranchRestrictionMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "not found"})
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchRestrictDelete(branchRestrictDeleteCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunBranchRestrictView_NoClient(t *testing.T) {
	defer resetBranchRestrictionMocks()

	branchNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runBranchRestrictView(branchRestrictViewCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunBranchRestrictView_APIError(t *testing.T) {
	defer resetBranchRestrictionMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchRestrictView(branchRestrictViewCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunBranchRestrictView_Success(t *testing.T) {
	defer resetBranchRestrictionMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := 2
		result := map[string]interface{}{
			"id":               1,
			"kind":             "require_approvals_to_merge",
			"pattern":          "main",
			"branch_match_kind": "glob",
			"value":            value,
			"users":            []interface{}{},
			"groups":           []interface{}{},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	branchNewAPIClient = mockAPIClientFactory(server)

	err := runBranchRestrictView(branchRestrictViewCmd, []string{"workspace/repo", "1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
