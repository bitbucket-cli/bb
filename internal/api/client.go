package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/tom/bb/generated/bitbucket"
	"github.com/tom/bb/internal/auth"
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

	resp, err := c.underlying.Do(req)
	if err != nil {
		return nil, err
	}

	// Handle token refresh on 401 (only for OAuth tokens)
	if resp.StatusCode == http.StatusUnauthorized && token != "" && c.canRefresh {
		resp.Body.Close()

		// Try to refresh the token
		newTokens, err := refreshAccessToken()
		if err != nil {
			return nil, fmt.Errorf("authentication expired, please run 'bb auth login': %w", err)
		}

		c.tokenMu.Lock()
		c.token = newTokens.AccessToken
		c.tokenMu.Unlock()

		// Retry the request with the new token
		req.Header.Set("Authorization", "Bearer "+newTokens.AccessToken)
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

// Context helper
func DefaultContext() context.Context {
	return context.Background()
}
