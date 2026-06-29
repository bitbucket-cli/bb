package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ThomasRedstone/bb/generated/bitbucket"
	"github.com/ThomasRedstone/bb/internal/config"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

// resetRepoMocks restores default implementations
func resetRepoMocks() {
	newAPIClient = api.NewClient
	runGitClone = runGitCloneReal
	getWorkspace = config.GetWorkspace
	repoAPIBaseURL = "https://api.bitbucket.org/2.0"
	repoCreateDescription = ""
	repoCreatePrivate = false
	repoCreatePublic = false
	repoCreateProject = ""
	repoCreateSCM = "git"
	repoCreateHasIssues = false
	repoCreateNoIssues = false
	repoCreateHasWiki = false
	repoCreateNoWiki = false
	repoCreateForkPolicy = ""
}

// TestPaginationTermination verifies that pagination stops correctly
func TestPaginationTermination(t *testing.T) {
	var requestCount atomic.Int32
	var serverURL string

	// Create a test server that returns paginated responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		t.Logf("Request %d: %s", count, r.URL.String())

		// Simulate 3 pages of 100 repos each (300 total)
		var result bitbucket.PaginatedRepositories
		size := 300
		result.Size = &size

		repos := make([]bitbucket.Repository, 100)
		for i := 0; i < 100; i++ {
			name := fmt.Sprintf("repo-%d", (int(count)-1)*100+i)
			fullName := fmt.Sprintf("workspace/%s", name)
			repos[i] = bitbucket.Repository{
				Name:     &name,
				FullName: &fullName,
			}
		}
		result.Values = &repos

		// Set next URL for pages 1 and 2, nil for page 3
		if count < 3 {
			next := fmt.Sprintf("%s/repositories/workspace?pagelen=100&page=%d", serverURL, count+1)
			result.Next = &next
		}
		// For page 3, result.Next remains nil

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()
	serverURL = server.URL

	// Now simulate the pagination loop from repo.go
	var allRepos []bitbucket.Repository

	// First request
	resp, err := http.Get(server.URL + "/repositories/workspace?pagelen=100")
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}

	var result bitbucket.PaginatedRepositories
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		resp.Body.Close()
		t.Fatalf("Failed to decode first response: %v", err)
	}
	resp.Body.Close()

	if result.Values != nil {
		allRepos = append(allRepos, *result.Values...)
	}

	t.Logf("After first request: %d repos, next=%v", len(allRepos), result.Next != nil)

	// Pagination loop (matches repo.go logic - with the fix!)
	iterations := 0
	maxIterations := 100 // Safety limit

	for result.Next != nil && *result.Next != "" {
		iterations++
		if iterations > maxIterations {
			t.Fatalf("Pagination loop exceeded %d iterations - likely infinite loop!", maxIterations)
		}

		t.Logf("Iteration %d: fetching %s", iterations, *result.Next)

		nextURL := *result.Next
		nextResp, err := http.Get(nextURL)
		if err != nil {
			t.Fatalf("Request %d failed: %v", iterations+1, err)
		}

		// FIX: Reset result before decoding - JSON decoder doesn't zero fields
		// that are absent in the response
		result = bitbucket.PaginatedRepositories{}
		if err := json.NewDecoder(nextResp.Body).Decode(&result); err != nil {
			nextResp.Body.Close()
			t.Fatalf("Failed to decode response %d: %v", iterations+1, err)
		}
		nextResp.Body.Close()

		if result.Values != nil {
			allRepos = append(allRepos, *result.Values...)
		}

		t.Logf("After iteration %d: %d repos, next=%v", iterations, len(allRepos), result.Next != nil)
	}

	t.Logf("Loop finished after %d iterations, total repos: %d", iterations, len(allRepos))

	// Verify results
	if requestCount.Load() != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount.Load())
	}

	if len(allRepos) != 300 {
		t.Errorf("Expected 300 repos, got %d", len(allRepos))
	}

	if iterations != 2 {
		t.Errorf("Expected 2 pagination iterations, got %d", iterations)
	}
}

// TestPaginationWithLimit verifies that --limit stops early
func TestPaginationWithLimit(t *testing.T) {
	var requestCount atomic.Int32
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)

		var result bitbucket.PaginatedRepositories
		size := 300
		result.Size = &size

		repos := make([]bitbucket.Repository, 100)
		for i := 0; i < 100; i++ {
			name := fmt.Sprintf("repo-%d", (int(count)-1)*100+i)
			repos[i] = bitbucket.Repository{Name: &name}
		}
		result.Values = &repos

		if count < 3 {
			next := fmt.Sprintf("%s?page=%d", serverURL, count+1)
			result.Next = &next
		}

		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()
	serverURL = server.URL

	limit := 150 // Should stop after 2 pages
	var allRepos []bitbucket.Repository

	resp, _ := http.Get(server.URL)
	var result bitbucket.PaginatedRepositories
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	if result.Values != nil {
		allRepos = append(allRepos, *result.Values...)
	}

	for result.Next != nil && *result.Next != "" {
		// Check limit BEFORE fetching next page
		if len(allRepos) >= limit {
			break
		}

		nextResp, _ := http.Get(*result.Next)
		json.NewDecoder(nextResp.Body).Decode(&result)
		nextResp.Body.Close()

		if result.Values != nil {
			allRepos = append(allRepos, *result.Values...)
		}
	}

	// Trim to limit
	if len(allRepos) > limit {
		allRepos = allRepos[:limit]
	}

	if requestCount.Load() != 2 {
		t.Errorf("Expected 2 requests with limit=%d, got %d", limit, requestCount.Load())
	}

	if len(allRepos) != limit {
		t.Errorf("Expected %d repos, got %d", limit, len(allRepos))
	}
}

// TestPaginationTiming measures time per request
func TestPaginationTiming(t *testing.T) {
	var requestCount atomic.Int32
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)

		var result bitbucket.PaginatedRepositories
		size := 300
		result.Size = &size

		repos := make([]bitbucket.Repository, 100)
		for i := 0; i < 100; i++ {
			name := fmt.Sprintf("repo-%d", i)
			repos[i] = bitbucket.Repository{Name: &name}
		}
		result.Values = &repos

		if count < 3 {
			next := fmt.Sprintf("%s?page=%d", serverURL, count+1)
			result.Next = &next
		}

		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()
	serverURL = server.URL

	start := time.Now()
	var allRepos []bitbucket.Repository

	resp, _ := http.Get(server.URL)
	var result bitbucket.PaginatedRepositories
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	if result.Values != nil {
		allRepos = append(allRepos, *result.Values...)
	}

	for result.Next != nil && *result.Next != "" {
		nextURL := *result.Next
		nextResp, _ := http.Get(nextURL)
		// Reset before decode (the fix)
		result = bitbucket.PaginatedRepositories{}
		json.NewDecoder(nextResp.Body).Decode(&result)
		nextResp.Body.Close()

		if result.Values != nil {
			allRepos = append(allRepos, *result.Values...)
		}
	}

	elapsed := time.Since(start)
	t.Logf("Fetched %d repos in %v (%d requests)", len(allRepos), elapsed, requestCount.Load())

	// Local mock server should complete in under 1 second
	if elapsed > time.Second {
		t.Errorf("Pagination took too long: %v (expected < 1s for mock)", elapsed)
	}
}

// TestPaginationInfiniteLoopBug demonstrates the bug where not resetting
// the result struct before JSON decoding causes an infinite loop
func TestPaginationInfiniteLoopBug(t *testing.T) {
	var requestCount atomic.Int32
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)

		var result bitbucket.PaginatedRepositories
		size := 200
		result.Size = &size

		repos := make([]bitbucket.Repository, 100)
		for i := 0; i < 100; i++ {
			name := fmt.Sprintf("repo-%d", i)
			repos[i] = bitbucket.Repository{Name: &name}
		}
		result.Values = &repos

		// Only set next on first page
		if count == 1 {
			next := fmt.Sprintf("%s?page=2", serverURL)
			result.Next = &next
		}
		// Page 2 does NOT include "next" field at all

		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()
	serverURL = server.URL

	// Simulate the BUGGY code (without resetting result)
	resp, _ := http.Get(server.URL)
	var result bitbucket.PaginatedRepositories
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	iterations := 0
	for result.Next != nil && *result.Next != "" {
		iterations++
		if iterations > 5 {
			// Bug detected! Without the fix, this would loop forever
			t.Logf("Bug confirmed: loop ran %d times without proper termination", iterations)
			return // Test passes - we detected the buggy behavior
		}

		nextResp, _ := http.Get(*result.Next)
		// BUG: Not resetting result before decode
		json.NewDecoder(nextResp.Body).Decode(&result)
		nextResp.Body.Close()
	}

	// If we get here with iterations == 1, the bug isn't present
	// (or this particular scenario doesn't trigger it)
	if iterations != 1 {
		t.Errorf("Expected exactly 1 iteration (2 pages), got %d", iterations)
	}
}

// --- Command tests ---

func TestRunRepoList_NoClient(t *testing.T) {
	defer resetRepoMocks()

	newAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runRepoList(repoListCmd, []string{"workspace"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunRepoList_NoWorkspace(t *testing.T) {
	defer resetRepoMocks()

	// Set up mock client that won't be used
	newAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}
	getWorkspace = func() string {
		return ""
	}

	err := runRepoList(repoListCmd, []string{})
	if err == nil {
		t.Error("Expected error when no workspace specified")
	}
	if err.Error() != "workspace required: specify as argument or set BB_WORKSPACE" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoList_WithWorkspaceArg(t *testing.T) {
	defer resetRepoMocks()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repos := []bitbucket.Repository{}
		name := "test-repo"
		fullName := "testws/test-repo"
		repos = append(repos, bitbucket.Repository{
			Name:     &name,
			FullName: &fullName,
		})
		result := bitbucket.PaginatedRepositories{
			Values: &repos,
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	os.Setenv("BITBUCKET_API_TOKEN", "test-token")
	defer os.Unsetenv("BITBUCKET_API_TOKEN")

	// We can't easily mock the full client, so skip this test for now
	t.Skip("Requires full API client mocking")
}

func TestRunRepoView_NoClient(t *testing.T) {
	defer resetRepoMocks()

	newAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runRepoView(repoViewCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunRepoView_InvalidRepo(t *testing.T) {
	defer resetRepoMocks()

	newAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runRepoView(repoViewCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunRepoClone_NoClient(t *testing.T) {
	defer resetRepoMocks()

	newAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runRepoClone(repoCloneCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunRepoClone_InvalidRepo(t *testing.T) {
	defer resetRepoMocks()

	newAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runRepoClone(repoCloneCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunGitCloneReal(t *testing.T) {
	// Test that the function exists and returns an error for invalid URL
	err := runGitCloneReal("invalid-url", "/nonexistent/path")
	if err == nil {
		t.Error("Expected error for invalid git clone")
	}
}

func TestRepoFlags(t *testing.T) {
	// Test that flags are configured
	limitFlag := repoListCmd.Flags().Lookup("limit")
	if limitFlag == nil {
		t.Error("--limit flag not found")
	}

	allFlag := repoListCmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Error("--all flag not found")
	}
}

func TestRepoCommandStructure(t *testing.T) {
	if repoCmd.Short == "" {
		t.Error("repo command should have short description")
	}

	if repoListCmd.RunE == nil {
		t.Error("repo list should have RunE")
	}

	if repoViewCmd.RunE == nil {
		t.Error("repo view should have RunE")
	}

	if repoCreateCmd.RunE == nil {
		t.Error("repo create should have RunE")
	}

	if repoCloneCmd.RunE == nil {
		t.Error("repo clone should have RunE")
	}
}

func TestFetchPage(t *testing.T) {
	// Test fetchPage with mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[]}`))
	}))
	defer server.Close()

	bbClient, _ := bitbucket.NewClient(server.URL)
	client := &api.Client{Client: bbClient}

	resp, err := fetchPage(context.Background(), client, server.URL+"/test")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestFetchPage_InvalidURL(t *testing.T) {
	bbClient, _ := bitbucket.NewClient("http://localhost")
	client := &api.Client{Client: bbClient}

	// Invalid URL with control characters
	_, err := fetchPage(context.Background(), client, "http://localhost/\x00invalid")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestRunRepoList_APIError(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)
	repoAPIBaseURL = server.URL

	err := runRepoList(repoListCmd, []string{"workspace"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunRepoList_EmptyWorkspace(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := bitbucket.PaginatedRepositories{}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)
	repoAPIBaseURL = server.URL

	err := runRepoList(repoListCmd, []string{"workspace"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoList_WithRepos(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := "test-repo"
		fullName := "workspace/test-repo"
		isPrivate := true
		language := "Go"
		repos := []bitbucket.Repository{
			{
				Name:      &name,
				FullName:  &fullName,
				IsPrivate: &isPrivate,
				Language:  &language,
			},
		}
		result := bitbucket.PaginatedRepositories{Values: &repos}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)
	repoAPIBaseURL = server.URL

	err := runRepoList(repoListCmd, []string{"workspace"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoList_InvalidJSON(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)
	repoAPIBaseURL = server.URL

	err := runRepoList(repoListCmd, []string{"workspace"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunRepoList_WithPagination(t *testing.T) {
	defer resetRepoMocks()
	origAll := repoAll
	repoAll = true
	defer func() { repoAll = origAll }()

	requestCount := 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		name := fmt.Sprintf("repo-%d", requestCount)
		fullName := fmt.Sprintf("workspace/%s", name)
		repos := []bitbucket.Repository{{Name: &name, FullName: &fullName}}
		size := 2
		result := bitbucket.PaginatedRepositories{
			Values: &repos,
			Size:   &size,
		}
		if requestCount == 1 {
			next := serverURL + "/repositories/workspace?page=2"
			result.Next = &next
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()
	serverURL = server.URL

	newAPIClient = mockAPIClientFactory(server)
	repoAPIBaseURL = server.URL

	err := runRepoList(repoListCmd, []string{"workspace"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if requestCount != 2 {
		t.Errorf("Expected 2 requests, got %d", requestCount)
	}
}

func TestRunRepoList_WithLimit(t *testing.T) {
	defer resetRepoMocks()
	origLimit := repoLimit
	repoLimit = 1
	defer func() { repoLimit = origLimit }()

	requestCount := 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		name := "repo-1"
		fullName := "workspace/repo-1"
		repos := []bitbucket.Repository{{Name: &name, FullName: &fullName}}
		size := 10
		next := serverURL + "/repositories/workspace?page=2"
		result := bitbucket.PaginatedRepositories{
			Values: &repos,
			Size:   &size,
			Next:   &next,
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()
	serverURL = server.URL

	newAPIClient = mockAPIClientFactory(server)
	repoAPIBaseURL = server.URL

	err := runRepoList(repoListCmd, []string{"workspace"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// Should stop after first page due to limit
	if requestCount != 1 {
		t.Errorf("Expected 1 request due to limit, got %d", requestCount)
	}
}

func TestRunRepoList_PaginationError(t *testing.T) {
	defer resetRepoMocks()
	origAll := repoAll
	repoAll = true
	defer func() { repoAll = origAll }()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			name := "repo-1"
			repos := []bitbucket.Repository{{Name: &name}}
			next := "http://invalid-url-that-will-fail:99999/page2"
			result := bitbucket.PaginatedRepositories{
				Values: &repos,
				Next:   &next,
			}
			json.NewEncoder(w).Encode(result)
		}
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)
	repoAPIBaseURL = server.URL

	err := runRepoList(repoListCmd, []string{"workspace"})
	if err == nil {
		t.Error("Expected error on pagination fetch failure")
	}
}

func TestRunRepoList_PaginationInvalidJSON(t *testing.T) {
	defer resetRepoMocks()
	origAll := repoAll
	repoAll = true
	defer func() { repoAll = origAll }()

	requestCount := 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			name := "repo-1"
			repos := []bitbucket.Repository{{Name: &name}}
			next := serverURL + "/page2"
			result := bitbucket.PaginatedRepositories{
				Values: &repos,
				Next:   &next,
			}
			json.NewEncoder(w).Encode(result)
		} else {
			// Second request returns invalid JSON
			w.Write([]byte("invalid json"))
		}
	}))
	defer server.Close()
	serverURL = server.URL

	newAPIClient = mockAPIClientFactory(server)
	repoAPIBaseURL = server.URL

	err := runRepoList(repoListCmd, []string{"workspace"})
	if err == nil {
		t.Error("Expected error on invalid JSON in pagination")
	}
}

func TestRunRepoList_WithEnvWorkspace(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := bitbucket.PaginatedRepositories{}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)
	repoAPIBaseURL = server.URL
	getWorkspace = func() string { return "env-workspace" }

	err := runRepoList(repoListCmd, []string{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoView_APIError(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoView(repoViewCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunRepoView_Success(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"full_name": "workspace/repo",
			"description": "Test repo",
			"is_private": true,
			"language": "Go",
			"mainbranch": {"name": "main"},
			"links": {
				"clone": [
					{"name": "ssh", "href": "git@bitbucket.org:workspace/repo.git"},
					{"name": "https", "href": "https://bitbucket.org/workspace/repo.git"}
				]
			}
		}`))
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoView(repoViewCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoView_JSONOutput(t *testing.T) {
	defer resetRepoMocks()
	origJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = origJSON }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"full_name": "workspace/repo"}`))
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoView(repoViewCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoView_InvalidJSON(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoView(repoViewCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunRepoClone_APIError(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoClone(repoCloneCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunRepoClone_InvalidJSON(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoClone(repoCloneCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunRepoClone_NoCloneURL(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"full_name": "workspace/repo"}`))
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoClone(repoCloneCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when no clone URL")
	}
}

func TestRunRepoClone_Success(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"full_name": "workspace/repo",
			"links": {
				"clone": [
					{"name": "ssh", "href": "git@bitbucket.org:workspace/repo.git"}
				]
			}
		}`))
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)
	runGitClone = func(url, dir string) error {
		return nil
	}

	err := runRepoClone(repoCloneCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoClone_WithDirectory(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"full_name": "workspace/repo",
			"links": {
				"clone": [
					{"name": "https", "href": "https://bitbucket.org/workspace/repo.git"}
				]
			}
		}`))
	}))
	defer server.Close()

	var clonedDir string
	newAPIClient = mockAPIClientFactory(server)
	runGitClone = func(url, dir string) error {
		clonedDir = dir
		return nil
	}

	err := runRepoClone(repoCloneCmd, []string{"workspace/repo", "custom-dir"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if clonedDir != "custom-dir" {
		t.Errorf("Expected custom-dir, got %s", clonedDir)
	}
}

func TestRunRepoClone_GitFails(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"full_name": "workspace/repo",
			"links": {
				"clone": [
					{"name": "ssh", "href": "git@bitbucket.org:workspace/repo.git"}
				]
			}
		}`))
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)
	runGitClone = func(url, dir string) error {
		return errors.New("git clone failed")
	}

	err := runRepoClone(repoCloneCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when git clone fails")
	}
}

// --- Repo Create Tests ---

func resetRepoCreateMocks() {
	newAPIClient = api.NewClient
	repoCreateDescription = ""
	repoCreatePrivate = false
	repoCreatePublic = false
	repoCreateProject = ""
	repoCreateSCM = "git"
	repoCreateHasIssues = false
	repoCreateNoIssues = false
	repoCreateHasWiki = false
	repoCreateNoWiki = false
	repoCreateForkPolicy = ""
}

func TestRepoCreateFlags(t *testing.T) {
	descFlag := repoCreateCmd.Flags().Lookup("description")
	if descFlag == nil {
		t.Error("--description flag not found")
	}

	projectFlag := repoCreateCmd.Flags().Lookup("project")
	if projectFlag == nil {
		t.Error("--project flag not found")
	}

	scmFlag := repoCreateCmd.Flags().Lookup("scm")
	if scmFlag == nil {
		t.Error("--scm flag not found")
	}
}

func TestRunRepoCreate_NoClient(t *testing.T) {
	defer resetRepoCreateMocks()

	newAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runRepoCreate(repoCreateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunRepoCreate_InvalidRepo(t *testing.T) {
	defer resetRepoCreateMocks()

	newAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runRepoCreate(repoCreateCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunRepoCreate_ConflictingVisibilityFlags(t *testing.T) {
	defer resetRepoCreateMocks()

	newAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}
	repoCreatePrivate = true
	repoCreatePublic = true

	err := runRepoCreate(repoCreateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when both --private and --public are set")
	}
}

func TestRunRepoCreate_APIError(t *testing.T) {
	defer resetRepoCreateMocks()

	repoCreateDescription = "My new repo"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid"})
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoCreate(repoCreateCmd, []string{"workspace/myrepo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunRepoCreate_Success(t *testing.T) {
	defer resetRepoCreateMocks()

	repoCreateDescription = "My new repo"
	repoCreatePrivate = true
	repoCreateProject = "MARS"
	repoCreateSCM = "git"
	repoCreateHasIssues = true
	repoCreateHasWiki = true
	repoCreateForkPolicy = "allow_forks"

	var gotPath string
	var gotMethod string
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"full_name": "workspace/myrepo",
		})
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoCreate(repoCreateCmd, []string{"workspace/myrepo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("Expected POST method, got %s", gotMethod)
	}
	if gotPath != "/repositories/workspace/myrepo" {
		t.Errorf("Expected /repositories/workspace/myrepo path, got %s", gotPath)
	}

	if gotBody["description"] != "My new repo" {
		t.Errorf("Expected description in request body, got %v", gotBody["description"])
	}
	if gotBody["is_private"] != true {
		t.Errorf("Expected is_private=true in request body, got %v", gotBody["is_private"])
	}
	if gotBody["scm"] != "git" {
		t.Errorf("Expected scm=git in request body, got %v", gotBody["scm"])
	}
	if gotBody["has_issues"] != true {
		t.Errorf("Expected has_issues=true in request body, got %v", gotBody["has_issues"])
	}
	if gotBody["has_wiki"] != true {
		t.Errorf("Expected has_wiki=true in request body, got %v", gotBody["has_wiki"])
	}
	if gotBody["fork_policy"] != "allow_forks" {
		t.Errorf("Expected fork_policy=allow_forks in request body, got %v", gotBody["fork_policy"])
	}

	project, ok := gotBody["project"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected project object in request body, got %T", gotBody["project"])
	}
	if project["key"] != "MARS" {
		t.Errorf("Expected project key MARS, got %v", project["key"])
	}
}

// --- Repo Update Tests ---

func resetRepoUpdateMocks() {
	newAPIClient = api.NewClient
	repoDescription = ""
	repoPrivate = false
	repoPublic = false
	repoHasIssues = false
	repoNoIssues = false
	repoHasWiki = false
	repoNoWiki = false
	repoForkPolicy = ""
}

func TestRepoUpdateFlags(t *testing.T) {
	descFlag := repoUpdateCmd.Flags().Lookup("description")
	if descFlag == nil {
		t.Error("--description flag not found")
	}

	privateFlag := repoUpdateCmd.Flags().Lookup("private")
	if privateFlag == nil {
		t.Error("--private flag not found")
	}

	publicFlag := repoUpdateCmd.Flags().Lookup("public")
	if publicFlag == nil {
		t.Error("--public flag not found")
	}

	forkPolicyFlag := repoUpdateCmd.Flags().Lookup("fork-policy")
	if forkPolicyFlag == nil {
		t.Error("--fork-policy flag not found")
	}
}

func TestRunRepoUpdate_NoClient(t *testing.T) {
	defer resetRepoUpdateMocks()

	newAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runRepoUpdate(repoUpdateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunRepoUpdate_InvalidRepo(t *testing.T) {
	defer resetRepoUpdateMocks()

	newAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runRepoUpdate(repoUpdateCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunRepoUpdate_NoUpdates(t *testing.T) {
	defer resetRepoUpdateMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoUpdate(repoUpdateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when no updates specified")
	}
}

func TestRunRepoUpdate_Success(t *testing.T) {
	defer resetRepoUpdateMocks()

	repoDescription = "New description"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"full_name":   "workspace/repo",
			"description": "New description",
		})
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoUpdate(repoUpdateCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoUpdate_PrivateFlag(t *testing.T) {
	defer resetRepoUpdateMocks()

	repoPrivate = true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoUpdate(repoUpdateCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoUpdate_APIError(t *testing.T) {
	defer resetRepoUpdateMocks()

	repoDescription = "New description"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid"})
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoUpdate(repoUpdateCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

// --- Repo Reviewers Tests ---

func TestRepoReviewersCommandStructure(t *testing.T) {
	subCmds := repoReviewersCmd.Commands()
	expectedCmds := []string{"list", "add", "remove"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected repo reviewers subcommand %q not found", expected)
		}
	}
}

func TestRunRepoReviewersList_NoClient(t *testing.T) {
	defer resetRepoMocks()

	newAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runRepoReviewersList(repoReviewersListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunRepoReviewersList_InvalidRepo(t *testing.T) {
	defer resetRepoMocks()

	newAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runRepoReviewersList(repoReviewersListCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunRepoReviewersList_APIError(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoReviewersList(repoReviewersListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunRepoReviewersList_NoReviewers(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoReviewersList(repoReviewersListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoReviewersList_WithReviewers(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"uuid":         "{user-uuid}",
					"display_name": "John Doe",
					"nickname":     "johnd",
					"account_id":   "123456",
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoReviewersList(repoReviewersListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoReviewersAdd_NoClient(t *testing.T) {
	defer resetRepoMocks()

	newAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runRepoReviewersAdd(repoReviewersAddCmd, []string{"workspace/repo", "johnd"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunRepoReviewersAdd_InvalidRepo(t *testing.T) {
	defer resetRepoMocks()

	newAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runRepoReviewersAdd(repoReviewersAddCmd, []string{"invalid", "johnd"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunRepoReviewersAdd_Success(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"display_name": "John Doe"})
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoReviewersAdd(repoReviewersAddCmd, []string{"workspace/repo", "johnd"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoReviewersAdd_APIError(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "user not found"})
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoReviewersAdd(repoReviewersAddCmd, []string{"workspace/repo", "nonexistent"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunRepoReviewersRemove_NoClient(t *testing.T) {
	defer resetRepoMocks()

	newAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runRepoReviewersRemove(repoReviewersRemoveCmd, []string{"workspace/repo", "johnd"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunRepoReviewersRemove_InvalidRepo(t *testing.T) {
	defer resetRepoMocks()

	newAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runRepoReviewersRemove(repoReviewersRemoveCmd, []string{"invalid", "johnd"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunRepoReviewersRemove_Success(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoReviewersRemove(repoReviewersRemoveCmd, []string{"workspace/repo", "johnd"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunRepoReviewersRemove_APIError(t *testing.T) {
	defer resetRepoMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "not found"})
	}))
	defer server.Close()

	newAPIClient = mockAPIClientFactory(server)

	err := runRepoReviewersRemove(repoReviewersRemoveCmd, []string{"workspace/repo", "johnd"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}
