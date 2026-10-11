package bitbucket

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bitbucket-cli/bb/generated/bitbucket"
	"github.com/bitbucket-cli/bb/pkg/auth"
)

// resetMocks restores default function implementations
func resetMocks() {
	refreshAccessToken = auth.RefreshAccessToken
	newBitbucketClient = bitbucket.NewClient
	accessTokenExpiredOrUnknown = auth.AccessTokenExpiredOrUnknown
}

func TestAuthClientDo_404RefreshRetryWhenExpired(t *testing.T) {
	defer resetMocks()

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusNotFound) // expired session masked as 404
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer fresh-token" {
			t.Errorf("retry should carry refreshed token, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	}))
	defer server.Close()

	accessTokenExpiredOrUnknown = func() bool { return true }
	refreshAccessToken = func() (*auth.TokenData, error) {
		return &auth.TokenData{AccessToken: "fresh-token"}, nil
	}

	client := &authClient{
		cred:       auth.Credential{Scheme: auth.SchemeBearer, Token: "stale-token"},
		underlying: http.DefaultClient,
		canRefresh: true,
	}
	resp, err := client.Do(mustReq(t, server.URL))
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 after refresh+retry", resp.StatusCode)
	}
	if calls != 2 {
		t.Errorf("expected one retry (2 calls), got %d", calls)
	}
}

func TestAuthClientDo_404NotRetriedWhenTokenValid(t *testing.T) {
	defer resetMocks()

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	accessTokenExpiredOrUnknown = func() bool { return false } // healthy token
	refreshAccessToken = func() (*auth.TokenData, error) {
		t.Fatal("refresh must not be attempted on a 404 with a valid token")
		return nil, nil
	}

	client := &authClient{
		cred:       auth.Credential{Scheme: auth.SchemeBearer, Token: "good-token"},
		underlying: http.DefaultClient,
		canRefresh: true,
	}
	resp, err := client.Do(mustReq(t, server.URL))
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (genuine not-found)", resp.StatusCode)
	}
	if calls != 1 {
		t.Errorf("expected no retry (1 call), got %d", calls)
	}
}

func mustReq(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	return req
}

func TestParseRepoContext(t *testing.T) {
	tests := []struct {
		input     string
		wantWS    string
		wantSlug  string
		wantError bool
	}{
		{"workspace/repo", "workspace", "repo", false},
		{"my-org/my-project", "my-org", "my-project", false},
		{"atlassian/bitbucket-cli", "atlassian", "bitbucket-cli", false},
		{"invalid", "", "", true},
		{"", "", "", true},
		{"noslash", "", "", true},
		{"/onlyslash", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ctx, err := ParseRepoContext(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("ParseRepoContext(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseRepoContext(%q) unexpected error: %v", tt.input, err)
				return
			}

			if ctx.Workspace != tt.wantWS {
				t.Errorf("ParseRepoContext(%q).Workspace = %q, want %q", tt.input, ctx.Workspace, tt.wantWS)
			}
			if ctx.RepoSlug != tt.wantSlug {
				t.Errorf("ParseRepoContext(%q).RepoSlug = %q, want %q", tt.input, ctx.RepoSlug, tt.wantSlug)
			}
		})
	}
}

func TestCheckResponse(t *testing.T) {
	tests := []struct {
		statusCode int
		wantError  bool
	}{
		{200, false},
		{201, false},
		{204, false},
		{299, false},
		{300, true},
		{400, true},
		{401, true},
		{404, true},
		{500, true},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.statusCode), func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Status:     http.StatusText(tt.statusCode),
			}

			err := CheckResponse(resp)

			if tt.wantError && err == nil {
				t.Errorf("CheckResponse(status=%d) expected error, got nil", tt.statusCode)
			}
			if !tt.wantError && err != nil {
				t.Errorf("CheckResponse(status=%d) unexpected error: %v", tt.statusCode, err)
			}
		})
	}
}

func TestDefaultContext(t *testing.T) {
	ctx := DefaultContext()
	if ctx == nil {
		t.Error("DefaultContext() returned nil")
	}
	if ctx != context.Background() {
		t.Error("DefaultContext() should return context.Background()")
	}
}

func TestNewUnauthenticatedClient(t *testing.T) {
	client, err := NewUnauthenticatedClient()
	if err != nil {
		t.Fatalf("NewUnauthenticatedClient() error: %v", err)
	}
	if client == nil {
		t.Error("NewUnauthenticatedClient() returned nil client")
	}
	if client.Client == nil {
		t.Error("NewUnauthenticatedClient() client.Client is nil")
	}
}

func TestCheckResponse_404AuthHint(t *testing.T) {
	notFound := &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found"}

	t.Run("OAuth mode hints re-login", func(t *testing.T) {
		os.Unsetenv("BITBUCKET_API_TOKEN")
		err := CheckResponse(notFound)
		if err == nil || !strings.Contains(err.Error(), "bb auth login") {
			t.Errorf("expected re-login hint on 404 in OAuth mode, got: %v", err)
		}
	})

	t.Run("API-token mode does not hint", func(t *testing.T) {
		os.Setenv("BITBUCKET_API_TOKEN", "x")
		defer os.Unsetenv("BITBUCKET_API_TOKEN")
		err := CheckResponse(notFound)
		if err == nil || strings.Contains(err.Error(), "bb auth login") {
			t.Errorf("expected no hint with API token, got: %v", err)
		}
	})

	t.Run("non-404 does not hint", func(t *testing.T) {
		os.Unsetenv("BITBUCKET_API_TOKEN")
		err := CheckResponse(&http.Response{StatusCode: http.StatusInternalServerError, Status: "500 Internal Server Error"})
		if err == nil || strings.Contains(err.Error(), "bb auth login") {
			t.Errorf("expected no hint on 500, got: %v", err)
		}
	})
}

func TestAuthClientDo_WithToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Expected Bearer test-token, got %q", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := &authClient{
		cred:       auth.Credential{Scheme: auth.SchemeBearer, Token: "test-token"},
		underlying: http.DefaultClient,
		canRefresh: false,
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Do() status = %d, want 200", resp.StatusCode)
	}
}

func TestAuthClientDo_WithBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Errorf("Expected Basic auth, got Authorization %q", r.Header.Get("Authorization"))
		}
		if user != "me@example.com" || pass != "api-token" {
			t.Errorf("BasicAuth = %q:%q, want me@example.com:api-token", user, pass)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := &authClient{
		cred:       auth.Credential{Scheme: auth.SchemeBasic, Username: "me@example.com", Token: "api-token"},
		underlying: http.DefaultClient,
		canRefresh: false,
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Do() status = %d, want 200", resp.StatusCode)
	}
}

func TestAuthClientDo_WithoutToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "" {
			t.Errorf("Expected no Authorization header, got %q", auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &authClient{
		cred:       auth.Credential{},
		underlying: http.DefaultClient,
		canRefresh: false,
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Do() status = %d, want 200", resp.StatusCode)
	}
}

func TestAuthClientDo_NetworkError(t *testing.T) {
	client := &authClient{
		cred:       auth.Credential{Scheme: auth.SchemeBearer, Token: "test-token"},
		underlying: http.DefaultClient,
		canRefresh: false,
	}

	req, _ := http.NewRequest("GET", "http://localhost:99999", nil)
	_, err := client.Do(req)
	if err == nil {
		t.Error("Do() expected network error, got nil")
	}
}

func TestAuthClientDo_401NoRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := &authClient{
		cred:       auth.Credential{Scheme: auth.SchemeBearer, Token: "test-token"},
		underlying: http.DefaultClient,
		canRefresh: false, // Cannot refresh
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	// Should return 401 without attempting refresh
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Do() status = %d, want 401", resp.StatusCode)
	}
}

func TestNewClient_WithAPIToken(t *testing.T) {
	// Set API token environment variable
	os.Setenv("BITBUCKET_API_TOKEN", "test-api-token")
	defer os.Unsetenv("BITBUCKET_API_TOKEN")

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	if client == nil {
		t.Error("NewClient() returned nil")
	}
}

func TestNewClient_NoCredentials(t *testing.T) {
	// Clear all credential sources
	os.Unsetenv("BITBUCKET_API_TOKEN")
	os.Unsetenv("BB_CLIENT_ID")
	os.Unsetenv("BB_CLIENT_SECRET")

	// Use temp home with file storage only
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	auth.ForceFileStorage = true
	defer func() {
		os.Setenv("HOME", origHome)
		auth.ForceFileStorage = false
	}()

	// Delete tokens
	auth.DeleteTokens()

	_, err := NewClient()
	if err == nil {
		t.Error("NewClient() expected error when no credentials, got nil")
	}
}

func TestAuthClientDo_401WithRefreshFailure(t *testing.T) {
	defer resetMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	// Mock refresh to fail
	refreshAccessToken = func() (*auth.TokenData, error) {
		return nil, errors.New("refresh failed")
	}

	client := &authClient{
		cred:       auth.Credential{Scheme: auth.SchemeBearer, Token: "expired-token"},
		underlying: http.DefaultClient,
		canRefresh: true,
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err := client.Do(req)

	if err == nil {
		t.Error("Do() expected error when refresh fails, got nil")
	}
	if !strings.Contains(err.Error(), "authentication expired") {
		t.Errorf("Do() error = %v, expected authentication expired message", err)
	}
}

// TestRepoContext tests the struct fields
func TestRepoContext(t *testing.T) {
	ctx := &RepoContext{
		Workspace: "test-ws",
		RepoSlug:  "test-repo",
	}

	if ctx.Workspace != "test-ws" {
		t.Errorf("Workspace = %q, want %q", ctx.Workspace, "test-ws")
	}
	if ctx.RepoSlug != "test-repo" {
		t.Errorf("RepoSlug = %q, want %q", ctx.RepoSlug, "test-repo")
	}
}

// TestTypesExist verifies the helper types exist and can be instantiated
func TestTypesExist(t *testing.T) {
	_ = Repository{}
	_ = PullRequest{}
	_ = Branch{}
	_ = Pipeline{}
	_ = RepoContext{}
}

func TestAuthClientDo_401RetrySuccess(t *testing.T) {
	defer resetMocks()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First request: return 401
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			// Retry: return success
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer new-token" {
				t.Errorf("Retry request should have new token, got %q", authHeader)
			}
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "success")
		}
	}))
	defer server.Close()

	// Mock successful refresh
	refreshAccessToken = func() (*auth.TokenData, error) {
		return &auth.TokenData{AccessToken: "new-token"}, nil
	}

	client := &authClient{
		cred:       auth.Credential{Scheme: auth.SchemeBearer, Token: "expired-token"},
		underlying: http.DefaultClient,
		canRefresh: true,
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Do() status = %d, want 200", resp.StatusCode)
	}

	// Verify token was updated
	if client.cred.Token != "new-token" {
		t.Errorf("client.cred.Token = %q, want %q", client.cred.Token, "new-token")
	}
}

func TestNewClient_ClientCreationError(t *testing.T) {
	defer resetMocks()

	os.Setenv("BITBUCKET_API_TOKEN", "test-token")
	defer os.Unsetenv("BITBUCKET_API_TOKEN")

	// Mock client creation to fail
	newBitbucketClient = func(server string, opts ...bitbucket.ClientOption) (*bitbucket.Client, error) {
		return nil, errors.New("client creation failed")
	}

	_, err := NewClient()
	if err == nil {
		t.Error("NewClient() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create API client") {
		t.Errorf("NewClient() error = %v, expected client creation error", err)
	}
}

func TestNewUnauthenticatedClient_Error(t *testing.T) {
	defer resetMocks()

	// Mock client creation to fail
	newBitbucketClient = func(server string, opts ...bitbucket.ClientOption) (*bitbucket.Client, error) {
		return nil, errors.New("client creation failed")
	}

	_, err := NewUnauthenticatedClient()
	if err == nil {
		t.Error("NewUnauthenticatedClient() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create API client") {
		t.Errorf("NewUnauthenticatedClient() error = %v, expected client creation error", err)
	}
}
