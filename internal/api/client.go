package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/ThomasRedstone/bb/generated/bitbucket"
	"github.com/ThomasRedstone/bb/internal/auth"
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

// authClient is an HTTP client that adds Bearer token authentication
type authClient struct {
	token        string
	tokenMu      sync.RWMutex
	underlying   *http.Client
	canRefresh   bool // false when using BITBUCKET_API_TOKEN
}

func (c *authClient) Do(req *http.Request) (*http.Response, error) {
	c.tokenMu.RLock()
	token := c.token
	c.tokenMu.RUnlock()

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

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

	// Handle token refresh on 401 (only for OAuth tokens)
	if resp.StatusCode == http.StatusUnauthorized && token != "" && c.canRefresh {
		resp.Body.Close()

		newTokens, err := refreshAccessToken()
		if err != nil {
			return nil, fmt.Errorf("authentication expired, please run 'bb auth login': %w", err)
		}

		c.tokenMu.Lock()
		c.token = newTokens.AccessToken
		c.tokenMu.Unlock()

		req.Header.Set("Authorization", "Bearer "+newTokens.AccessToken)
		if bodySnapshot != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodySnapshot))
		}
		return c.underlying.Do(req)
	}

	return resp, nil
}

// NewClient creates a new authenticated Bitbucket API client
func NewClient() (*Client, error) {
	token, err := auth.GetValidToken()
	if err != nil {
		return nil, err
	}

	// Determine if we can refresh tokens (only with OAuth, not with API token)
	canRefresh := !auth.HasAPIToken()

	httpClient := &authClient{
		token:      token,
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

// Repository represents a simplified repository structure
type Repository struct {
	UUID        string
	Slug        string
	Name        string
	FullName    string
	Description string
	IsPrivate   bool
	Language    string
	UpdatedOn   string
	MainBranch  string
	CloneSSH    string
	CloneHTTPS  string
}

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
	UUID       string
	BuildNum   int
	State      string
	Result     string
	CreatedOn  string
	CompletedOn string
	Duration   int
	Target     string
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

// Context helper
func DefaultContext() context.Context {
	return context.Background()
}
