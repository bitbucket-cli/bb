package pr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bitbucket-cli/bb/generated/bitbucket"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

// State constants matching the Bitbucket API.
const (
	StateOpen     = "OPEN"
	StateMerged   = "MERGED"
	StateDeclined = "DECLINED"
)

// PullRequest is a Bitbucket pull request.
type PullRequest struct {
	ID     int
	Title  string
	Author string
	Branch string // source branch name
	URL    string
	State  string
}

// NewClient is the API client constructor. Override in tests.
var NewClient = api.NewClient

// List returns all pull requests for workspace/repo filtered by state.
// Use the State* constants for state.
func List(ctx context.Context, workspace, repoSlug, state string) ([]PullRequest, error) {
	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("bitbucket auth: %w", err)
	}

	s := bitbucket.GetRepositoriesWorkspaceRepoSlugPullrequestsParamsState(state)
	params := &bitbucket.GetRepositoriesWorkspaceRepoSlugPullrequestsParams{
		State: &s,
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugPullrequests(ctx, workspace, repoSlug, params)
	if err != nil {
		return nil, fmt.Errorf("list PRs %s/%s: %w", workspace, repoSlug, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %s: %s", resp.Status, body)
	}

	var result bitbucket.PaginatedPullrequests
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	prs := extractPRs(workspace, repoSlug, &result)

	// Follow pagination links until exhausted.
	for result.Next != nil && *result.Next != "" {
		nextResp, err := client.GetURL(ctx, *result.Next)
		if err != nil {
			return prs, fmt.Errorf("fetch next page: %w", err)
		}
		if nextResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(nextResp.Body)
			nextResp.Body.Close()
			return prs, fmt.Errorf("API error %s: %s", nextResp.Status, body)
		}
		result = bitbucket.PaginatedPullrequests{}
		if err := json.NewDecoder(nextResp.Body).Decode(&result); err != nil {
			nextResp.Body.Close()
			return prs, fmt.Errorf("decode response: %w", err)
		}
		nextResp.Body.Close()
		prs = append(prs, extractPRs(workspace, repoSlug, &result)...)
	}

	return prs, nil
}

func extractPRs(workspace, repoSlug string, result *bitbucket.PaginatedPullrequests) []PullRequest {
	if result.Values == nil {
		return nil
	}
	prs := make([]PullRequest, 0, len(*result.Values))
	for _, p := range *result.Values {
		pr := PullRequest{State: StateOpen}
		if p.Id != nil {
			pr.ID = *p.Id
		}
		if p.Title != nil {
			pr.Title = *p.Title
		}
		if p.Author != nil && p.Author.DisplayName != nil {
			pr.Author = *p.Author.DisplayName
		}
		if p.Source != nil && p.Source.Branch != nil && p.Source.Branch.Name != nil {
			pr.Branch = *p.Source.Branch.Name
		}
		if p.State != nil {
			pr.State = string(*p.State)
		}
		if pr.ID > 0 {
			pr.URL = fmt.Sprintf("https://bitbucket.org/%s/%s/pull-requests/%d", workspace, repoSlug, pr.ID)
		}
		prs = append(prs, pr)
	}
	return prs
}
