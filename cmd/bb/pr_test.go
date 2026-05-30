package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tom/bb/generated/bitbucket"
	"github.com/tom/bb/internal/api"
	bbpr "github.com/tom/bb/pkg/pr"
)

// resetPRMocks restores default implementations
func resetPRMocks() {
	prNewAPIClient = api.NewClient
	bbpr.NewClient = api.NewClient
}

func TestRunPRList_NoClient(t *testing.T) {
	defer resetPRMocks()

	fail := func() (*api.Client, error) { return nil, errors.New("auth required") }
	prNewAPIClient = fail
	bbpr.NewClient = fail

	err := runPRList(prListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPRList_InvalidRepo(t *testing.T) {
	defer resetPRMocks()

	prNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPRList(prListCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPRView_NoClient(t *testing.T) {
	defer resetPRMocks()

	prNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPRView(prViewCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPRView_InvalidRepo(t *testing.T) {
	defer resetPRMocks()

	prNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPRView(prViewCmd, []string{"invalid", "1"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPRView_InvalidPRNumber(t *testing.T) {
	defer resetPRMocks()

	prNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPRView(prViewCmd, []string{"workspace/repo", "not-a-number"})
	if err == nil {
		t.Error("Expected error for invalid PR number")
	}
	if err.Error() != "invalid PR number: not-a-number" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestRunPRCreate_NoClient(t *testing.T) {
	defer resetPRMocks()

	prNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPRCreate(prCreateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPRCreate_InvalidRepo(t *testing.T) {
	defer resetPRMocks()

	prNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPRCreate(prCreateCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPRMerge_NoClient(t *testing.T) {
	defer resetPRMocks()

	prNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPRMerge(prMergeCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPRMerge_InvalidRepo(t *testing.T) {
	defer resetPRMocks()

	prNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPRMerge(prMergeCmd, []string{"invalid", "1"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPRMerge_InvalidPRNumber(t *testing.T) {
	defer resetPRMocks()

	prNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPRMerge(prMergeCmd, []string{"workspace/repo", "not-a-number"})
	if err == nil {
		t.Error("Expected error for invalid PR number")
	}
}

func TestRunPRApprove_NoClient(t *testing.T) {
	defer resetPRMocks()

	prNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPRApprove(prApproveCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPRApprove_InvalidRepo(t *testing.T) {
	defer resetPRMocks()

	prNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPRApprove(prApproveCmd, []string{"invalid", "1"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPRApprove_InvalidPRNumber(t *testing.T) {
	defer resetPRMocks()

	prNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPRApprove(prApproveCmd, []string{"workspace/repo", "not-a-number"})
	if err == nil {
		t.Error("Expected error for invalid PR number")
	}
}

func TestPRCommandStructure(t *testing.T) {
	if prCmd.Short == "" {
		t.Error("pr command should have short description")
	}

	if prListCmd.RunE == nil {
		t.Error("pr list should have RunE")
	}

	if prViewCmd.RunE == nil {
		t.Error("pr view should have RunE")
	}

	if prCreateCmd.RunE == nil {
		t.Error("pr create should have RunE")
	}

	if prMergeCmd.RunE == nil {
		t.Error("pr merge should have RunE")
	}

	if prApproveCmd.RunE == nil {
		t.Error("pr approve should have RunE")
	}
}

func TestPRFlags(t *testing.T) {
	// Test list flags
	stateFlag := prListCmd.Flags().Lookup("state")
	if stateFlag == nil {
		t.Error("--state flag not found on pr list command")
	}

	// Test create flags
	titleFlag := prCreateCmd.Flags().Lookup("title")
	if titleFlag == nil {
		t.Error("--title flag not found on pr create command")
	}

	bodyFlag := prCreateCmd.Flags().Lookup("body")
	if bodyFlag == nil {
		t.Error("--body flag not found on pr create command")
	}

	sourceFlag := prCreateCmd.Flags().Lookup("source")
	if sourceFlag == nil {
		t.Error("--source flag not found on pr create command")
	}

	destFlag := prCreateCmd.Flags().Lookup("dest")
	if destFlag == nil {
		t.Error("--dest flag not found on pr create command")
	}
}

func TestPRSubcommands(t *testing.T) {
	subCmds := prCmd.Commands()
	expectedCmds := []string{"list", "view", "create", "merge", "approve"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected pr subcommand %q not found", expected)
		}
	}
}

func TestBytesReader(t *testing.T) {
	data := []byte("hello world")
	reader := bytesReader(data)

	// Read all data
	buf := make([]byte, 20)
	n, err := reader.Read(buf)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Read %d bytes, expected %d", n, len(data))
	}
	if string(buf[:n]) != "hello world" {
		t.Errorf("Read %q, expected %q", string(buf[:n]), "hello world")
	}

	// Read again should return 0 (EOF)
	n, err = reader.Read(buf)
	if n != 0 {
		t.Errorf("Expected 0 bytes on second read, got %d", n)
	}
}

func TestBytesReader_PartialRead(t *testing.T) {
	data := []byte("hello world")
	reader := bytesReader(data)

	// Read partial data
	buf := make([]byte, 5)
	n, _ := reader.Read(buf)
	if n != 5 {
		t.Errorf("Read %d bytes, expected 5", n)
	}
	if string(buf) != "hello" {
		t.Errorf("Read %q, expected %q", string(buf), "hello")
	}

	// Read remaining data
	n, _ = reader.Read(buf)
	if n != 5 {
		t.Errorf("Read %d bytes, expected 5", n)
	}
	if string(buf) != " worl" {
		t.Errorf("Read %q, expected %q", string(buf), " worl")
	}

	// Read final byte
	n, _ = reader.Read(buf)
	if n != 1 {
		t.Errorf("Read %d bytes, expected 1", n)
	}
}

func TestRunPRList_APIError(t *testing.T) {
	defer resetPRMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	mock := mockAPIClientFactory(server)
	prNewAPIClient = mock
	bbpr.NewClient = mock

	err := runPRList(prListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPRList_NoPRs(t *testing.T) {
	defer resetPRMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := bitbucket.PaginatedPullrequests{}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	mock := mockAPIClientFactory(server)
	prNewAPIClient = mock
	bbpr.NewClient = mock

	err := runPRList(prListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPRList_WithPRs(t *testing.T) {
	defer resetPRMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return simple JSON that matches expected structure
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"values": [{
				"id": 1,
				"title": "Test PR",
				"state": "OPEN",
				"author": {"display_name": "Test User"},
				"source": {"branch": {"name": "feature"}}
			}]
		}`))
	}))
	defer server.Close()

	mock := mockAPIClientFactory(server)
	prNewAPIClient = mock
	bbpr.NewClient = mock

	err := runPRList(prListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPRList_InvalidJSON(t *testing.T) {
	defer resetPRMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	mock := mockAPIClientFactory(server)
	prNewAPIClient = mock
	bbpr.NewClient = mock

	err := runPRList(prListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunPRView_APIError(t *testing.T) {
	defer resetPRMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRView(prViewCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPRView_Success(t *testing.T) {
	defer resetPRMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": 1,
			"title": "Test PR",
			"state": "OPEN",
			"created_on": "2024-01-01T00:00:00Z",
			"updated_on": "2024-01-01T00:00:00Z",
			"author": {"display_name": "Test User"},
			"source": {"branch": {"name": "feature"}},
			"destination": {"branch": {"name": "main"}},
			"summary": {"raw": "Test description"},
			"links": {"html": {"href": "https://bitbucket.org/pr/1"}}
		}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRView(prViewCmd, []string{"workspace/repo", "1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPRView_JSONOutput(t *testing.T) {
	defer resetPRMocks()
	origJSONOutput := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = origJSONOutput }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": 1}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRView(prViewCmd, []string{"workspace/repo", "1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPRView_InvalidJSON(t *testing.T) {
	defer resetPRMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRView(prViewCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunPRCreate_APIError(t *testing.T) {
	defer resetPRMocks()
	origTitle := prTitle
	origSource := prSource
	origDest := prDest
	prTitle = "Test PR"
	prSource = "feature"
	prDest = "main" // Set dest to skip repo lookup
	defer func() {
		prTitle = origTitle
		prSource = origSource
		prDest = origDest
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// PR create fails
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRCreate(prCreateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPRCreate_Success(t *testing.T) {
	defer resetPRMocks()
	origTitle := prTitle
	origSource := prSource
	origDest := prDest
	origBody := prBody
	prTitle = "Test PR"
	prSource = "feature"
	prDest = "main"
	prBody = "Test description"
	defer func() {
		prTitle = origTitle
		prSource = origSource
		prDest = origDest
		prBody = origBody
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": 1, "links": {"html": {"href": "https://bitbucket.org/pr/1"}}}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRCreate(prCreateCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPRCreate_NoDest(t *testing.T) {
	defer resetPRMocks()
	origTitle := prTitle
	origSource := prSource
	origDest := prDest
	prTitle = "Test PR"
	prSource = "feature"
	prDest = ""
	defer func() {
		prTitle = origTitle
		prSource = origSource
		prDest = origDest
	}()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			// Get repo info
			w.Write([]byte(`{"mainbranch": {"name": "main"}}`))
			return
		}
		// Create PR
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 1}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRCreate(prCreateCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPRCreate_NoMainBranch(t *testing.T) {
	defer resetPRMocks()
	origTitle := prTitle
	origSource := prSource
	origDest := prDest
	prTitle = "Test PR"
	prSource = "feature"
	prDest = ""
	defer func() {
		prTitle = origTitle
		prSource = origSource
		prDest = origDest
	}()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			// Get repo info - no mainbranch
			w.Write([]byte(`{}`))
			return
		}
		// Create PR
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 1}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRCreate(prCreateCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPRCreate_InvalidRepoJSON(t *testing.T) {
	defer resetPRMocks()
	origTitle := prTitle
	origSource := prSource
	origDest := prDest
	prTitle = "Test PR"
	prSource = "feature"
	prDest = ""
	defer func() {
		prTitle = origTitle
		prSource = origSource
		prDest = origDest
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRCreate(prCreateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunPRCreate_InvalidResponseJSON(t *testing.T) {
	defer resetPRMocks()
	origTitle := prTitle
	origSource := prSource
	origDest := prDest
	prTitle = "Test PR"
	prSource = "feature"
	prDest = "main"
	defer func() {
		prTitle = origTitle
		prSource = origSource
		prDest = origDest
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRCreate(prCreateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunPRMerge_APIError(t *testing.T) {
	defer resetPRMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "conflicts"})
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRMerge(prMergeCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPRMerge_Success(t *testing.T) {
	defer resetPRMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRMerge(prMergeCmd, []string{"workspace/repo", "1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPRApprove_APIError(t *testing.T) {
	defer resetPRMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "forbidden"})
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRApprove(prApproveCmd, []string{"workspace/repo", "1"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPRApprove_Success(t *testing.T) {
	defer resetPRMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)

	err := runPRApprove(prApproveCmd, []string{"workspace/repo", "1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
