package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	prAll          bool
	prUser         string
	prLinks        bool
	prMeta         bool
	prCreatedSince string
	prUpdatedSince string
)

// wsPR is the subset of a pull request we render from the workspace-wide endpoint.
type wsPR struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	State        string `json:"state"`
	Draft        *bool  `json:"draft"`
	CommentCount *int   `json:"comment_count"`
	Participants *[]struct {
		Approved *bool  `json:"approved"`
		State    string `json:"state"`
	} `json:"participants"`
	Author struct {
		DisplayName string `json:"display_name"`
		UUID        string `json:"uuid"`
		AccountID   string `json:"account_id"`
	} `json:"author"`
	Source struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	} `json:"source"`
	Destination struct {
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	} `json:"destination"`
	Links struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
}

func (p wsPR) url() string {
	if p.Links.HTML.Href != "" {
		return p.Links.HTML.Href
	}
	if repo := p.repository(); repo != "" {
		return fmt.Sprintf("https://bitbucket.org/%s/pull-requests/%d", repo, p.ID)
	}
	return ""
}

func (p wsPR) repository() string {
	if p.Destination.Repository.FullName != "" {
		return p.Destination.Repository.FullName
	}
	return p.Source.Repository.FullName
}

// reviewState derives the current decision from participants, independently of
// assigned reviewers. Missing participant data stays unknown rather than zero.
func (p wsPR) reviewState() string {
	if p.Participants == nil {
		return "unknown"
	}
	approved := false
	for _, participant := range *p.Participants {
		switch strings.ToLower(participant.State) {
		case "changes_requested":
			return "changes_requested"
		case "approved":
			approved = true
		}
		if participant.Approved != nil && *participant.Approved {
			approved = true
		}
	}
	if approved {
		return "approved"
	}
	return "none"
}

type prListQuery struct {
	Scope           string `json:"scope"`
	Workspace       string `json:"workspace"`
	Repository      string `json:"repository,omitempty"`
	AllRepositories bool   `json:"all_repositories"`
	Author          string `json:"author"`
	State           string `json:"state"`
	DraftFilter     string `json:"draft_filter"`
	CreatedSince    string `json:"created_since,omitempty"`
	UpdatedSince    string `json:"updated_since,omitempty"`
}

type prListReport struct {
	Query      prListQuery      `json:"query"`
	Pagination paginationStats  `json:"pagination"`
	Count      int              `json:"count"`
	Summary    prListSummary    `json:"summary"`
	Items      []map[string]any `json:"items"`
}

type prListSummary struct {
	ByState      map[string]int `json:"by_state"`
	ByRepository map[string]int `json:"by_repository"`
	ByReview     map[string]int `json:"by_review"`
	Drafts       int            `json:"drafts"`
	NonDrafts    int            `json:"non_drafts"`
	DraftUnknown int            `json:"draft_unknown"`
}

// renderPRList keeps the default JSON array and CSV contracts intact. The
// opt-in --meta JSON envelope is suitable for scripts that need query evidence.
func renderPRList(prs []wsPR, query prListQuery, stats paginationStats, repoOnly bool) error {
	if prMeta && output.CurrentFormat == output.FormatCSV {
		return fmt.Errorf("--meta is available with --json or table output, not --csv")
	}
	if output.CurrentFormat == output.FormatTable {
		location := query.Workspace
		if query.Repository != "" {
			location += "/" + query.Repository
		}
		output.Info("Query: scope=%s author=%s state=%s drafts=%s", location, query.Author, query.State, query.DraftFilter)
		if query.CreatedSince != "" || query.UpdatedSince != "" {
			output.Info("Time bounds: created_since=%s updated_since=%s", query.CreatedSince, query.UpdatedSince)
		}
		output.Info("Results: count=%d pages=%d complete=%t has_more=%t", len(prs), stats.PagesFetched, stats.Complete, stats.HasMore)
		if len(prs) == 0 {
			output.Info("No pull requests found")
			return nil
		}
	}

	headers := []string{"REPO", "#", "TITLE", "AUTHOR", "STATE", "REVIEW", "COMMENTS"}
	if repoOnly {
		headers = []string{"#", "TITLE", "AUTHOR", "BRANCH", "STATE", "REVIEW", "COMMENTS"}
	}
	if prLinks {
		headers = append(headers, "LINK")
	}
	var rows [][]string
	items := make([]map[string]any, 0, len(prs))
	summary := prListSummary{ByState: map[string]int{}, ByRepository: map[string]int{}, ByReview: map[string]int{}}
	for _, pr := range prs {
		repo := pr.repository()
		if repo == "" && query.Repository != "" {
			repo = query.Workspace + "/" + query.Repository
		}
		url := pr.url()
		if url == "" && repo != "" {
			url = fmt.Sprintf("https://bitbucket.org/%s/pull-requests/%d", repo, pr.ID)
		}
		review := pr.reviewState()
		comments := ""
		if pr.CommentCount != nil {
			comments = strconv.Itoa(*pr.CommentCount)
		}
		row := []string{repo, strconv.Itoa(pr.ID), output.Truncate(pr.Title, 45), pr.Author.DisplayName, pr.State, review, comments}
		if repoOnly {
			row = []string{strconv.Itoa(pr.ID), output.Truncate(pr.Title, 50), pr.Author.DisplayName, pr.Source.Branch.Name, pr.State, review, comments}
		}
		if prLinks {
			row = append(row, url)
		}
		rows = append(rows, row)
		item := map[string]any{"id": pr.ID, "title": pr.Title, "author": pr.Author.DisplayName, "state": pr.State, "review_state": review, "url": url}
		if repoOnly {
			item["branch"] = pr.Source.Branch.Name
		} else {
			item["repo"] = repo
		}
		if pr.Draft != nil {
			item["draft"] = *pr.Draft
		}
		if pr.CommentCount != nil {
			item["comment_count"] = *pr.CommentCount
		}
		summary.ByState[pr.State]++
		summary.ByRepository[repo]++
		summary.ByReview[review]++
		switch {
		case pr.Draft == nil:
			summary.DraftUnknown++
		case *pr.Draft:
			summary.Drafts++
		default:
			summary.NonDrafts++
		}
		items = append(items, item)
	}
	if output.CurrentFormat == output.FormatTable {
		output.Info("Summary: states=%s reviews=%s drafts=%d non_drafts=%d draft_unknown=%d", formatPRCounts(summary.ByState), formatPRCounts(summary.ByReview), summary.Drafts, summary.NonDrafts, summary.DraftUnknown)
		output.Info("Repositories: %s", formatPRCounts(summary.ByRepository))
	}
	if prMeta && output.CurrentFormat == output.FormatJSON {
		return output.JSON(prListReport{Query: query, Pagination: stats, Count: len(items), Summary: summary, Items: items})
	}
	return output.Print(headers, rows, items)
}

func formatPRCounts(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ", ")
}

// prListParams uses Bitbucket's server-side q filter so time-bounded audits do
// not have to fetch an unbounded history before filtering locally.
func prListParams(now time.Time) (url.Values, string, string, string, error) {
	params := url.Values{}
	state := strings.ToUpper(strings.TrimSpace(prState))
	if state == "" {
		state = "OPEN"
	}
	switch state {
	case "ALL":
		for _, s := range []string{"OPEN", "MERGED", "DECLINED"} {
			params.Add("state", s)
		}
	case "OPEN", "MERGED", "DECLINED":
		params.Set("state", state)
	default:
		return nil, "", "", "", fmt.Errorf("invalid --state %q (use OPEN, MERGED, DECLINED or ALL)", prState)
	}
	created, err := parsePRSince(prCreatedSince, now)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("invalid --created-since: %w", err)
	}
	updated, err := parsePRSince(prUpdatedSince, now)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("invalid --updated-since: %w", err)
	}
	var clauses []string
	if created != "" {
		clauses = append(clauses, "created_on >= "+created)
	}
	if updated != "" {
		clauses = append(clauses, "updated_on >= "+updated)
	}
	if len(clauses) > 0 {
		params.Set("q", strings.Join(clauses, " AND "))
	}
	return params, state, created, updated, nil
}

func parsePRSince(raw string, now time.Time) (string, error) {
	if raw == "" {
		return "", nil
	}
	if d, err := time.ParseDuration(raw); err == nil {
		if d <= 0 {
			return "", fmt.Errorf("duration must be positive")
		}
		return now.Add(-d).UTC().Format(time.RFC3339Nano), nil
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return "", fmt.Errorf("use an RFC3339 timestamp or a positive duration such as 36h")
	}
	return t.UTC().Format(time.RFC3339Nano), nil
}

// runPRListAll lists pull requests for a user across every repository in a
// workspace, via GET /2.0/workspaces/{workspace}/pull-requests/{selected_user}.
func runPRListAll(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	if ctx == nil {
		ctx = context.Background()
	}

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	workspace, err := workspaceArg(args)
	if err != nil {
		return err
	}

	user := prUser
	if user == "" {
		user, err = currentUserUUID(ctx, client)
		if err != nil {
			return fmt.Errorf("could not determine current user (pass --user <uuid|account_id>): %w", err)
		}
	}

	params, state, created, updated, err := prListParams(time.Now())
	if err != nil {
		return err
	}
	path := "/workspaces/" + url.PathEscape(workspace) + "/pullrequests/" + url.PathEscape(user) + "?" + params.Encode()
	resp, err := client.Request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("failed to list pull requests: %w", err)
	}
	defer resp.Body.Close()
	all, stats, err := fetchAllPagesWithStats[wsPR](ctx, client, resp)
	if err != nil {
		return err
	}
	query := prListQuery{Scope: "workspace", Workspace: workspace, AllRepositories: true, Author: user, State: state, DraftFilter: "all", CreatedSince: created, UpdatedSince: updated}
	return renderPRList(all, query, stats, false)
}

// currentUserUUID resolves the authenticated user's UUID via GET /2.0/user.
func currentUserUUID(ctx context.Context, client *api.Client) (string, error) {
	resp, err := client.GetUser(ctx)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %s", resp.Status)
	}
	var u struct {
		UUID      string `json:"uuid"`
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", err
	}
	if u.UUID != "" {
		return u.UUID, nil
	}
	return u.AccountID, nil
}
