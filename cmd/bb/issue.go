package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	issueTitle       string
	issueBody        string
	issueKind        string
	issuePriority    string
	issueState       string
	issueAssignee    string
	issueCommentBody string
)

// Mockable dependencies for testing
var (
	issueNewAPIClient = api.NewClient
)

var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Manage issues",
	Long:  `List, create, view, update, and delete repository issues.`,
}

var issueListCmd = &cobra.Command{
	Use:   "list [workspace/repo]",
	Short: "List issues",
	Long:  `List all issues in a repository.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runIssueList,
}

var issueCreateCmd = &cobra.Command{
	Use:   "create [workspace/repo]",
	Short: "Create an issue",
	Long: `Create a new issue in a repository.

Example:
  bb issue create myworkspace/myrepo --title "Bug report" --body "Description" --kind bug --priority major`,
	Args: cobra.MaximumNArgs(1),
	RunE: runIssueCreate,
}

var issueViewCmd = &cobra.Command{
	Use:   "view [workspace/repo] <issue-id>",
	Short: "View issue details",
	Long:  `Display detailed information about a specific issue.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runIssueView,
}

var issueUpdateCmd = &cobra.Command{
	Use:   "update [workspace/repo] <issue-id>",
	Short: "Update an issue",
	Long: `Update an existing issue.

Example:
  bb issue update myworkspace/myrepo 123 --state resolved --priority minor`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runIssueUpdate,
}

var issueDeleteCmd = &cobra.Command{
	Use:   "delete [workspace/repo] <issue-id>",
	Short: "Delete an issue",
	Long:  `Delete an issue from a repository.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runIssueDelete,
}

var issueCommentCmd = &cobra.Command{
	Use:   "comment [workspace/repo] <issue-id>",
	Short: "Add a comment to an issue",
	Long:  `Add a new comment to an issue.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runIssueComment,
}

var issueCommentsCmd = &cobra.Command{
	Use:   "comments [workspace/repo] <issue-id>",
	Short: "List comments on an issue",
	Long:  `Display all comments on an issue.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runIssueComments,
}

func init() {
	// Create flags
	issueCreateCmd.Flags().StringVarP(&issueTitle, "title", "t", "", "Issue title (required)")
	issueCreateCmd.Flags().StringVarP(&issueBody, "body", "b", "", "Issue description")
	issueCreateCmd.Flags().StringVarP(&issueKind, "kind", "k", "bug", "Issue kind (bug, enhancement, proposal, task)")
	issueCreateCmd.Flags().StringVarP(&issuePriority, "priority", "p", "major", "Issue priority (trivial, minor, major, critical, blocker)")
	issueCreateCmd.Flags().StringVarP(&issueAssignee, "assignee", "a", "", "Issue assignee username")
	issueCreateCmd.MarkFlagRequired("title")

	// Update flags
	issueUpdateCmd.Flags().StringVarP(&issueTitle, "title", "t", "", "New issue title")
	issueUpdateCmd.Flags().StringVarP(&issueBody, "body", "b", "", "New issue description")
	issueUpdateCmd.Flags().StringVarP(&issueKind, "kind", "k", "", "New issue kind")
	issueUpdateCmd.Flags().StringVarP(&issuePriority, "priority", "p", "", "New issue priority")
	issueUpdateCmd.Flags().StringVarP(&issueState, "state", "s", "", "New issue state (new, open, resolved, closed, on hold, invalid, duplicate, wontfix)")
	issueUpdateCmd.Flags().StringVarP(&issueAssignee, "assignee", "a", "", "New assignee username")

	// Comment flags
	issueCommentCmd.Flags().StringVarP(&issueCommentBody, "body", "b", "", "Comment text (required)")
	issueCommentCmd.MarkFlagRequired("body")

	issueCmd.AddCommand(issueListCmd)
	issueCmd.AddCommand(issueCreateCmd)
	issueCmd.AddCommand(issueViewCmd)
	issueCmd.AddCommand(issueUpdateCmd)
	issueCmd.AddCommand(issueDeleteCmd)
	issueCmd.AddCommand(issueCommentCmd)
	issueCmd.AddCommand(issueCommentsCmd)
}

func runIssueList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := issueNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugIssues(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
	)
	if err != nil {
		return fmt.Errorf("failed to list issues: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	type issueRow struct {
		ID       int    `json:"id"`
		Title    string `json:"title"`
		State    string `json:"state"`
		Priority string `json:"priority"`
		Kind     string `json:"kind"`
		Reporter *struct {
			DisplayName string `json:"display_name"`
		} `json:"reporter"`
		Assignee *struct {
			DisplayName string `json:"display_name"`
		} `json:"assignee"`
		CreatedOn string `json:"created_on"`
	}

	issues, err := fetchAllPages[issueRow](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(issues) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No issues found")
		return nil
	}

	headers := []string{"ID", "TITLE", "STATE", "PRIORITY", "KIND", "ASSIGNEE"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, issue := range issues {
		assignee := ""
		if issue.Assignee != nil {
			assignee = issue.Assignee.DisplayName
		}

		rows = append(rows, []string{
			fmt.Sprintf("#%d", issue.ID),
			output.Truncate(issue.Title, 40),
			issue.State,
			issue.Priority,
			issue.Kind,
			assignee,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"id":         issue.ID,
			"title":      issue.Title,
			"state":      issue.State,
			"priority":   issue.Priority,
			"kind":       issue.Kind,
			"assignee":   assignee,
			"created_on": issue.CreatedOn,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runIssueCreate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := issueNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	issueRequest := map[string]interface{}{
		"title":    issueTitle,
		"kind":     issueKind,
		"priority": issuePriority,
	}

	if issueBody != "" {
		issueRequest["content"] = map[string]interface{}{
			"raw": issueBody,
		}
	}

	if issueAssignee != "" {
		issueRequest["assignee"] = map[string]interface{}{
			"username": issueAssignee,
		}
	}

	body, err := json.Marshal(issueRequest)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	withBody := func(ctx context.Context, req *http.Request) error {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/json")
		return nil
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugIssuesWithBody(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		"application/json",
		bytes.NewReader(body),
		withBody,
	)
	if err != nil {
		return fmt.Errorf("failed to create issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	var result struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	output.Success("Created issue #%d", result.ID)
	return nil
}

func runIssueView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := issueNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	issueID := rest[0]

	resp, err := client.GetRepositoriesWorkspaceRepoSlugIssuesIssueId(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		issueID,
	)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var issue struct {
		ID       int    `json:"id"`
		Title    string `json:"title"`
		State    string `json:"state"`
		Priority string `json:"priority"`
		Kind     string `json:"kind"`
		Content  *struct {
			Raw string `json:"raw"`
		} `json:"content"`
		Reporter *struct {
			DisplayName string `json:"display_name"`
		} `json:"reporter"`
		Assignee *struct {
			DisplayName string `json:"display_name"`
		} `json:"assignee"`
		CreatedOn string `json:"created_on"`
		UpdatedOn string `json:"updated_on"`
		Votes     int    `json:"votes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if output.CurrentFormat == output.FormatJSON {
		return output.Print(nil, nil, issue)
	}

	fmt.Printf("Issue #%d: %s\n", issue.ID, issue.Title)
	fmt.Printf("State:    %s\n", issue.State)
	fmt.Printf("Priority: %s\n", issue.Priority)
	fmt.Printf("Kind:     %s\n", issue.Kind)
	if issue.Reporter != nil {
		fmt.Printf("Reporter: %s\n", issue.Reporter.DisplayName)
	}
	if issue.Assignee != nil {
		fmt.Printf("Assignee: %s\n", issue.Assignee.DisplayName)
	}
	fmt.Printf("Votes:    %d\n", issue.Votes)
	fmt.Printf("Created:  %s\n", issue.CreatedOn)
	fmt.Printf("Updated:  %s\n", issue.UpdatedOn)
	if issue.Content != nil && issue.Content.Raw != "" {
		fmt.Printf("\nDescription:\n%s\n", issue.Content.Raw)
	}

	return nil
}

func runIssueUpdate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := issueNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	issueID := rest[0]

	issueRequest := make(map[string]interface{})

	if issueTitle != "" {
		issueRequest["title"] = issueTitle
	}
	if issueBody != "" {
		issueRequest["content"] = map[string]interface{}{
			"raw": issueBody,
		}
	}
	if issueKind != "" {
		issueRequest["kind"] = issueKind
	}
	if issuePriority != "" {
		issueRequest["priority"] = issuePriority
	}
	if issueState != "" {
		issueRequest["state"] = issueState
	}
	if issueAssignee != "" {
		issueRequest["assignee"] = map[string]interface{}{
			"username": issueAssignee,
		}
	}

	if len(issueRequest) == 0 {
		return fmt.Errorf("no updates specified")
	}

	body, err := json.Marshal(issueRequest)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	withBody := func(ctx context.Context, req *http.Request) error {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/json")
		return nil
	}

	resp, err := client.PutRepositoriesWorkspaceRepoSlugIssuesIssueId(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		issueID,
		withBody,
	)
	if err != nil {
		return fmt.Errorf("failed to update issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Updated issue #%s", issueID)
	return nil
}

func runIssueDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := issueNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	issueID := rest[0]

	resp, err := client.DeleteRepositoriesWorkspaceRepoSlugIssuesIssueId(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		issueID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Deleted issue #%s", issueID)
	return nil
}

func runIssueComment(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := issueNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	issueID := rest[0]

	commentRequest := map[string]interface{}{
		"content": map[string]interface{}{
			"raw": issueCommentBody,
		},
	}

	body, err := json.Marshal(commentRequest)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	withBody := func(ctx context.Context, req *http.Request) error {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/json")
		return nil
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugIssuesIssueIdCommentsWithBody(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		issueID,
		"application/json",
		bytes.NewReader(body),
		withBody,
	)
	if err != nil {
		return fmt.Errorf("failed to add comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Added comment to issue #%s", issueID)
	return nil
}

func runIssueComments(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := issueNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	issueID := rest[0]

	resp, err := client.GetRepositoriesWorkspaceRepoSlugIssuesIssueIdComments(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		issueID,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to get comments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	type issueComment struct {
		ID      int `json:"id"`
		Content *struct {
			Raw string `json:"raw"`
		} `json:"content"`
		User *struct {
			DisplayName string `json:"display_name"`
		} `json:"user"`
		CreatedOn string `json:"created_on"`
	}

	comments, err := fetchAllPages[issueComment](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(comments) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No comments found")
		return nil
	}

	if output.CurrentFormat == output.FormatJSON {
		return output.Print(nil, nil, comments)
	}

	var commentRows [][]string
	for _, comment := range comments {
		author := "Unknown"
		if comment.User != nil {
			author = comment.User.DisplayName
		}
		content := ""
		if comment.Content != nil {
			content = comment.Content.Raw
		}
		if output.CurrentFormat == output.FormatCSV {
			commentRows = append(commentRows, []string{strconv.Itoa(comment.ID), author, comment.CreatedOn, content})
			continue
		}
		fmt.Printf("Comment #%d by %s (%s):\n%s\n\n", comment.ID, author, comment.CreatedOn, content)
	}

	if output.CurrentFormat == output.FormatCSV {
		return output.Print([]string{"ID", "AUTHOR", "CREATED", "CONTENT"}, commentRows, comments)
	}
	return nil
}
