package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/ThomasRedstone/bb/generated/bitbucket"
	"github.com/ThomasRedstone/bb/pkg/auth"
)

const baseURL = "https://api.bitbucket.org/2.0"

// For testing - allows mocking
var (
	refreshAccessToken = auth.RefreshAccessToken
	newBitbucketClient = bitbucket.NewClient
)

// Client wraps the generated Bitbucket client with authentication
type Client struct {
	*bitbucket.Client
	httpClient *authClient
}

// authClient is an HTTP client that attaches the resolved credential
// (Bearer or Basic) to each request.
type authClient struct {
	cred       auth.Credential
	tokenMu    sync.RWMutex
	underlying *http.Client
	canRefresh bool // false when using BITBUCKET_API_TOKEN
}

func (c *authClient) Do(req *http.Request) (*http.Response, error) {
	c.tokenMu.RLock()
	cred := c.cred
	c.tokenMu.RUnlock()

	cred.Apply(req)

	// Snapshot body so we can replay it on a 401 retry.
	var bodySnapshot []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		bodySnapshot, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(bodySnapshot))
	}

	resp, err := c.underlying.Do(req)
	if err != nil {
		return nil, err
	}

	// Handle token refresh on 401 (only for OAuth tokens, which are Bearer)
	if resp.StatusCode == http.StatusUnauthorized && cred.Token != "" && c.canRefresh {
		resp.Body.Close()

		newTokens, err := refreshAccessToken()
		if err != nil {
			return nil, fmt.Errorf("authentication expired, please run 'bb auth login': %w", err)
		}

		refreshed := auth.Credential{Scheme: auth.SchemeBearer, Token: newTokens.AccessToken}
		c.tokenMu.Lock()
		c.cred = refreshed
		c.tokenMu.Unlock()

		refreshed.Apply(req)
		if bodySnapshot != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodySnapshot))
		}
		return c.underlying.Do(req)
	}

	return resp, nil
}

// NewClient creates a new authenticated Bitbucket API client
func NewClient() (*Client, error) {
	cred, err := auth.GetCredential()
	if err != nil {
		return nil, err
	}

	// Determine if we can refresh tokens (only with OAuth, not with API token)
	canRefresh := !auth.HasAPIToken()

	httpClient := &authClient{
		cred:       cred,
		underlying: http.DefaultClient,
		canRefresh: canRefresh,
	}

	client, err := newBitbucketClient(baseURL, bitbucket.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return &Client{
		Client:     client,
		httpClient: httpClient,
	}, nil
}

// NewClientOptional returns an authenticated client, or nil when no credentials
// are configured (no BITBUCKET_API_TOKEN and no stored OAuth tokens). Useful for
// callers that treat Bitbucket access as optional.
func NewClientOptional() *Client {
	if !auth.HasAPIToken() && !auth.HasStoredTokens() {
		return nil
	}
	client, err := NewClient()
	if err != nil {
		return nil
	}
	return client
}

// NewUnauthenticatedClient creates a client without authentication (for public endpoints)
func NewUnauthenticatedClient() (*Client, error) {
	client, err := newBitbucketClient(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return &Client{
		Client: client,
	}, nil
}

// Helper types for common operations
// (Repository lives in repositories.go alongside its API methods.)

// PullRequest represents a simplified PR structure
type PullRequest struct {
	ID          int
	Title       string
	Description string
	State       string
	Author      string
	Source      string
	Destination string
	CreatedOn   string
	UpdatedOn   string
}

// Branch represents a simplified branch structure
type Branch struct {
	Name   string
	Target string
}

// Pipeline represents a simplified pipeline structure
type Pipeline struct {
	UUID        string
	BuildNum    int
	State       string
	Result      string
	CreatedOn   string
	CompletedOn string
	Duration    int
	Target      string
}

// Workspace/repo context helper
type RepoContext struct {
	Workspace string
	RepoSlug  string
}

// ParseRepoContext parses "workspace/repo" format
func ParseRepoContext(fullName string) (*RepoContext, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid repository format: expected 'workspace/repo', got '%s'", fullName)
	}
	return &RepoContext{Workspace: parts[0], RepoSlug: parts[1]}, nil
}

// Helper to check response status
func CheckResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("API error: %s", resp.Status)
}

// GetURL makes an authenticated GET request to an arbitrary URL.
// Used to follow Bitbucket pagination "next" links.
func (c *Client) GetURL(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.httpClient != nil {
		return c.httpClient.Do(req)
	}
	// Unauthenticated client (e.g. created directly in tests).
	return http.DefaultClient.Do(req)
}

// Request makes an authenticated request with an optional JSON body and returns
// the raw response. It routes through the same credential/refresh layer as the
// generated client, and is used for Bitbucket endpoints the generated client
// doesn't cover (e.g. pipeline/repository variables). A path beginning with
// "http" is used verbatim; otherwise it is resolved against the API base URL.
func (c *Client) Request(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	url := path
	if !strings.HasPrefix(path, "http") {
		base := baseURL
		// Honour the embedded client's configured server (tests point this at an
		// httptest server; production uses the API base URL).
		if c.Client != nil && c.Server != "" {
			base = strings.TrimSuffix(c.Server, "/")
		}
		url = base + path
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to encode request body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Prefer the auth client (credentials + 401 refresh). Fall back to the
	// embedded client's HTTP doer, which tests set to reach the httptest server.
	if c.httpClient != nil {
		return c.httpClient.Do(req)
	}
	if c.Client != nil && c.Client.Client != nil {
		return c.Client.Client.Do(req)
	}
	return http.DefaultClient.Do(req)
}

// RequestRaw is like Request but sends a pre-encoded body with an explicit
// content type (e.g. multipart/form-data), for endpoints the generated client
// doesn't cover that aren't simple JSON.
func (c *Client) RequestRaw(ctx context.Context, method, path, contentType string, body io.Reader) (*http.Response, error) {
	url := path
	if !strings.HasPrefix(path, "http") {
		base := baseURL
		if c.Client != nil && c.Server != "" {
			base = strings.TrimSuffix(c.Server, "/")
		}
		url = base + path
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	if c.httpClient != nil {
		return c.httpClient.Do(req)
	}
	if c.Client != nil && c.Client.Client != nil {
		return c.Client.Client.Do(req)
	}
	return http.DefaultClient.Do(req)
}

// Context helper
func DefaultContext() context.Context {
	return context.Background()
}
