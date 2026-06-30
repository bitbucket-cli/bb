package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Prefixes in a repo name/slug that indicate an archived or deprecated repo.
var archivedPrefixes = []string{
	"[archived]",
	"[ARCHIVED]",
	"archived-",
	"archived_",
	"_archived_",
	"deprecated-",
	"deprecated_",
	"[deprecated]",
	"[DEPRECATED]",
	"old-",
	"old_",
}

// Repository is a Bitbucket repository as returned by the 2.0 REST API.
type Repository struct {
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	FullName    string    `json:"full_name"`
	Description string    `json:"description"`
	IsPrivate   bool      `json:"is_private"`
	UpdatedOn   time.Time `json:"updated_on"`
	CreatedOn   time.Time `json:"created_on"`
	SCM         string    `json:"scm"`
	Links       struct {
		Clone []struct {
			Href string `json:"href"`
			Name string `json:"name"`
		} `json:"clone"`
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
	Mainbranch struct {
		Name string `json:"name"`
	} `json:"mainbranch"`
	Workspace struct {
		Slug string `json:"slug"`
	} `json:"workspace"`
}

// doJSON performs an authenticated request (credentials applied by the wrapped
// http client) and returns the response body, erroring on non-2xx.
func (c *Client) doJSON(ctx context.Context, method, url string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	var resp *http.Response
	if c.httpClient != nil {
		resp, err = c.httpClient.Do(req)
	} else {
		resp, err = http.DefaultClient.Do(req)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error: %s - %s%s", resp.Status, string(data), authHintFor404(resp.StatusCode))
	}
	return data, nil
}

// ListRepositories returns all repositories in a workspace, following pagination.
func (c *Client) ListRepositories(ctx context.Context, workspace string) ([]Repository, error) {
	var all []Repository
	url := fmt.Sprintf("%s/repositories/%s?pagelen=100", baseURL, workspace)

	for url != "" {
		data, err := c.doJSON(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		var page struct {
			Values []Repository `json:"values"`
			Next   string       `json:"next"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		all = append(all, page.Values...)
		url = page.Next // already a full URL, or "" when exhausted
	}

	return all, nil
}

// GetRepository returns a single repository by workspace and slug.
func (c *Client) GetRepository(ctx context.Context, workspace, slug string) (*Repository, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s", baseURL, workspace, slug)

	data, err := c.doJSON(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var repo Repository
	if err := json.Unmarshal(data, &repo); err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}
	return &repo, nil
}

// CreateRepository creates a repository and returns its SSH clone URL.
func (c *Client) CreateRepository(ctx context.Context, workspace, name string, isPrivate bool, description string) (string, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s", baseURL, workspace, repoSlug(name))

	payload := map[string]interface{}{
		"scm":        "git",
		"is_private": isPrivate,
		"name":       name,
	}
	if description != "" {
		payload["description"] = description
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	data, err := c.doJSON(ctx, http.MethodPost, url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return "", fmt.Errorf("failed to create repository: %w", err)
	}

	var repo Repository
	if err := json.Unmarshal(data, &repo); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	return repo.SSHCloneURL(), nil
}

// repoSlug converts a human repository name to a Bitbucket repo_slug: lowercase,
// with runs of non-alphanumeric characters collapsed to single hyphens. The API
// path expects the slug, not the display name (which may contain spaces/caps).
func repoSlug(name string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen {
				b.WriteRune('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// IsArchived reports whether the repo name or slug carries an archived/deprecated prefix.
func (r *Repository) IsArchived() bool {
	nameLower := strings.ToLower(r.Name)
	slugLower := strings.ToLower(r.Slug)
	for _, prefix := range archivedPrefixes {
		prefixLower := strings.ToLower(prefix)
		if strings.HasPrefix(nameLower, prefixLower) || strings.HasPrefix(slugLower, prefixLower) {
			return true
		}
	}
	return false
}

// SSHCloneURL returns the SSH clone URL, falling back to one constructed from full_name.
func (r *Repository) SSHCloneURL() string {
	for _, link := range r.Links.Clone {
		if link.Name == "ssh" {
			return link.Href
		}
	}
	if r.FullName != "" {
		return fmt.Sprintf("git@bitbucket.org:%s.git", r.FullName)
	}
	return ""
}

// HTTPSCloneURL returns the HTTPS clone URL, or "" if none is present.
func (r *Repository) HTTPSCloneURL() string {
	for _, link := range r.Links.Clone {
		if link.Name == "https" {
			return link.Href
		}
	}
	return ""
}

// IsStale reports whether the repo hasn't been updated within the given duration.
func (r *Repository) IsStale(threshold time.Duration) bool {
	return time.Since(r.UpdatedOn) > threshold
}
