package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/ThomasRedstone/bb/generated/bitbucket"
	"github.com/ThomasRedstone/bb/internal/api"
	"github.com/ThomasRedstone/bb/internal/interactive"
	"github.com/ThomasRedstone/bb/internal/output"
	bbpr "github.com/ThomasRedstone/bb/pkg/pr"
)

var (
	prState     string
	prTitle     string
	prBody      string
	prSource    string
	prDest      string
	prReviewers []string
	prComment   string
)

// Mockable dependencies for testing
var (
	prNewAPIClient = api.NewClient
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Manage pull requests",
	Long:  `List, view, create, merge, approve, and decline pull requests.`,
}

var prListCmd = &cobra.Command{
	Use:   "list <workspace/repo>",
	Short: "List pull requests",
	Long:  `List pull requests in a repository.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPRList,
}

var prViewCmd = &cobra.Command{
	Use:   "view <workspace/repo> [pr-number]",
	Short: "View pull request details",
	Long: `Display detailed information about a pull request.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runPRView,
}

var prCreateCmd = &cobra.Command{
	Use:   "create <workspace/repo>",
	Short: "Create a pull request",
	Long:  `Create a new pull request.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPRCreate,
}

var prMergeCmd = &cobra.Command{
	Use:   "merge <workspace/repo> [pr-number]",
	Short: "Merge a pull request",
	Long: `Merge an open pull request.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runPRMerge,
}

var prApproveCmd = &cobra.Command{
	Use:   "approve <workspace/repo> [pr-number]",
	Short: "Approve a pull request",
	Long: `Add your approval to a pull request.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runPRApprove,
}

var prDeclineCmd = &cobra.Command{
	Use:   "decline <workspace/repo> [pr-number]",
	Short: "Decline a pull request",
	Long: `Decline (reject) an open pull request without merging it.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runPRDecline,
}

var prCommentsCmd = &cobra.Command{
	Use:   "comments <workspace/repo> [pr-number]",
	Short: "List comments on a pull request",
	Long: `Display all comments on a pull request.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runPRComments,
}

var prCommentCmd = &cobra.Command{
	Use:   "comment <workspace/repo> [pr-number]",
	Short: "Add a comment to a pull request",
	Long: `Add a new comment to a pull request.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runPRComment,
}

var prDiffCmd = &cobra.Command{
	Use:   "diff <workspace/repo> [pr-number]",
	Short: "View pull request diff",
	Long: `Display the diff of changes in a pull request.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runPRDiff,
}

func init() {
	prListCmd.Flags().StringVarP(&prState, "state", "s", "OPEN", "Filter by state (OPEN, MERGED, DECLINED)")

	prCreateCmd.Flags().StringVarP(&prTitle, "title", "t", "", "Pull request title (required)")
	prCreateCmd.Flags().StringVarP(&prBody, "body", "b", "", "Pull request description")
	prCreateCmd.Flags().StringVarP(&prSource, "source", "s", "", "Source branch (required)")
	prCreateCmd.Flags().StringVarP(&prDest, "dest", "d", "", "Destination branch (defaults to main branch)")
	prCreateCmd.Flags().StringSliceVarP(&prReviewers, "reviewer", "r", nil, "Reviewers (can be specified multiple times)")
	prCreateCmd.MarkFlagRequired("title")
	prCreateCmd.MarkFlagRequired("source")

	prCommentCmd.Flags().StringVarP(&prComment, "body", "b", "", "Comment text (required)")
	prCommentCmd.MarkFlagRequired("body")

	// Add interactive flag to commands that support it
	prViewCmd.Flags().BoolP("interactive", "i", false, "Select PR interactively")
	prMergeCmd.Flags().BoolP("interactive", "i", false, "Select PR interactively")
	prApproveCmd.Flags().BoolP("interactive", "i", false, "Select PR interactively")
	prDeclineCmd.Flags().BoolP("interactive", "i", false, "Select PR interactively")
	prCommentsCmd.Flags().BoolP("interactive", "i", false, "Select PR interactively")
	prCommentCmd.Flags().BoolP("interactive", "i", false, "Select PR interactively")
	prDiffCmd.Flags().BoolP("interactive", "i", false, "Select PR interactively")

	prCmd.AddCommand(prListCmd)
	prCmd.AddCommand(prViewCmd)
	prCmd.AddCommand(prCreateCmd)
	prCmd.AddCommand(prMergeCmd)
	prCmd.AddCommand(prApproveCmd)
	prCmd.AddCommand(prDeclineCmd)
	prCmd.AddCommand(prCommentsCmd)
	prCmd.AddCommand(prCommentCmd)
	prCmd.AddCommand(prDiffCmd)
}

func runPRList(cmd *cobra.Command, args []string) error {
	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	ctx := cmdCtx(cmd)
	if ctx == nil {
		ctx = context.Background()
	}

	prs, err := bbpr.List(ctx, repoCtx.Workspace, repoCtx.RepoSlug, prState)
	if err != nil {
		return err
	}

	if len(prs) == 0 {
		output.Info("No pull requests found")
		return nil
	}

	headers := []string{"#", "TITLE", "AUTHOR", "BRANCH", "STATE"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, pr := range prs {
		rows = append(rows, []string{
			strconv.Itoa(pr.ID),
			output.Truncate(pr.Title, 50),
			pr.Author,
			pr.Branch,
			pr.State,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"id":     pr.ID,
			"title":  pr.Title,
			"author": pr.Author,
			"branch": pr.Branch,
			"state":  pr.State,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runPRView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, args, client, repoCtx, interactive)
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugPullrequestsPullRequestId(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		prID,
	)
	if err != nil {
		return fmt.Errorf("failed to get pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var pr bitbucket.Pullrequest
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if jsonOutput {
		return output.JSON(pr)
	}

	// Display PR details
	id := ""
	if pr.Id != nil {
		id = strconv.Itoa(*pr.Id)
	}

	title := ""
	if pr.Title != nil {
		title = *pr.Title
	}

	state := ""
	if pr.State != nil {
		state = string(*pr.State)
	}

	author := ""
	if pr.Author != nil && pr.Author.DisplayName != nil {
		author = *pr.Author.DisplayName
	}

	source := ""
	if pr.Source != nil && pr.Source.Branch != nil && pr.Source.Branch.Name != nil {
		source = *pr.Source.Branch.Name
	}

	dest := ""
	if pr.Destination != nil && pr.Destination.Branch != nil && pr.Destination.Branch.Name != nil {
		dest = *pr.Destination.Branch.Name
	}

	fmt.Printf("PR #%s: %s\n", id, title)
	fmt.Printf("State:       %s\n", state)
	fmt.Printf("Author:      %s\n", author)
	fmt.Printf("Branches:    %s → %s\n", source, dest)

	if pr.Summary != nil && pr.Summary.Raw != nil && *pr.Summary.Raw != "" {
		fmt.Printf("\nDescription:\n%s\n", *pr.Summary.Raw)
	}

	if pr.CreatedOn != nil {
		fmt.Printf("\nCreated:     %s\n", pr.CreatedOn.Format("2006-01-02 15:04:05"))
	}
	if pr.UpdatedOn != nil {
		fmt.Printf("Updated:     %s\n", pr.UpdatedOn.Format("2006-01-02 15:04:05"))
	}

	// Show link
	if pr.Links != nil && pr.Links.Html != nil && pr.Links.Html.Href != nil {
		fmt.Printf("\nURL:         %s\n", *pr.Links.Html.Href)
	}

	return nil
}

func runPRCreate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	// If destination not specified, we need to get the repo to find the main branch
	destBranch := prDest
	if destBranch == "" {
		resp, err := client.GetRepositoriesWorkspaceRepoSlug(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
		if err != nil {
			return fmt.Errorf("failed to get repository info: %w", err)
		}
		defer resp.Body.Close()

		var repo bitbucket.Repository
		if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
			return fmt.Errorf("failed to parse repository info: %w", err)
		}

		if repo.Mainbranch != nil && repo.Mainbranch.Name != nil {
			destBranch = *repo.Mainbranch.Name
		} else {
			destBranch = "main" // fallback
		}
	}

	// Build PR request body
	prRequest := map[string]interface{}{
		"title": prTitle,
		"source": map[string]interface{}{
			"branch": map[string]interface{}{
				"name": prSource,
			},
		},
		"destination": map[string]interface{}{
			"branch": map[string]interface{}{
				"name": destBranch,
			},
		},
	}

	if prBody != "" {
		prRequest["description"] = prBody
	}

	// Add reviewers if specified
	if len(prReviewers) > 0 {
		reviewers := make([]map[string]interface{}, len(prReviewers))
		for i, r := range prReviewers {
			reviewers[i] = map[string]interface{}{
				"username": r,
			}
		}
		prRequest["reviewers"] = reviewers
	}

	body, err := json.Marshal(prRequest)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugPullrequestsWithBody(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		"application/json",
		bytesReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	var pr bitbucket.Pullrequest
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	id := 0
	if pr.Id != nil {
		id = *pr.Id
	}

	url := ""
	if pr.Links != nil && pr.Links.Html != nil && pr.Links.Html.Href != nil {
		url = *pr.Links.Html.Href
	}

	output.Success("Created pull request #%d", id)
	if url != "" {
		output.Info("URL: %s", url)
	}

	return nil
}

func runPRMerge(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, args, client, repoCtx, interactive)
	if err != nil {
		return err
	}

	// Use nil params and empty body for default merge behavior
	resp, err := client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdMerge(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		prID,
		nil,
		bitbucket.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdMergeJSONRequestBody{},
	)
	if err != nil {
		return fmt.Errorf("failed to merge pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Merged pull request #%d", prID)
	return nil
}

func runPRApprove(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, args, client, repoCtx, interactive)
	if err != nil {
		return err
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdApprove(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		prID,
	)
	if err != nil {
		return fmt.Errorf("failed to approve pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Approved pull request #%d", prID)
	return nil
}

func runPRDecline(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, args, client, repoCtx, interactive)
	if err != nil {
		return err
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdDecline(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		prID,
	)
	if err != nil {
		return fmt.Errorf("failed to decline pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Declined pull request #%d", prID)
	return nil
}

// Helper to create a bytes reader
func bytesReader(b []byte) *bytesReaderImpl {
	return &bytesReaderImpl{data: b, pos: 0}
}

type bytesReaderImpl struct {
	data []byte
	pos  int
}

func (r *bytesReaderImpl) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func runPRComments(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, args, client, repoCtx, interactive)
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdComments(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		prID,
	)
	if err != nil {
		return fmt.Errorf("failed to get comments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []bitbucket.PullrequestComment `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Values) == 0 {
		output.Info("No comments found")
		return nil
	}

	if jsonOutput {
		return output.JSON(result.Values)
	}

	for _, comment := range result.Values {
		if comment.Deleted != nil && *comment.Deleted {
			continue
		}

		author := "Unknown"
		if comment.User != nil && comment.User.DisplayName != nil {
			author = *comment.User.DisplayName
		}

		createdOn := ""
		if comment.CreatedOn != nil {
			createdOn = comment.CreatedOn.Format("2006-01-02 15:04")
		}

		content := ""
		if comment.Content != nil && comment.Content.Raw != nil {
			content = *comment.Content.Raw
		}

		id := int64(0)
		if comment.Id != nil {
			id = *comment.Id
		}

		// Check if it's an inline comment
		location := ""
		if comment.Inline != nil {
			location = fmt.Sprintf(" [%s", comment.Inline.Path)
			if comment.Inline.To != nil {
				location += fmt.Sprintf(":%d", *comment.Inline.To)
			}
			location += "]"
		}

		fmt.Printf("--- Comment #%d by %s on %s%s ---\n", id, author, createdOn, location)
		fmt.Printf("%s\n\n", content)
	}

	return nil
}

func runPRComment(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, args, client, repoCtx, interactive)
	if err != nil {
		return err
	}

	commentBody := map[string]interface{}{
		"content": map[string]interface{}{
			"raw": prComment,
		},
	}

	body, err := json.Marshal(commentBody)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdCommentsWithBody(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		prID,
		"application/json",
		bytesReader(body),
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

	output.Success("Added comment to pull request #%d", prID)
	return nil
}

func runPRDiff(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, args, client, repoCtx, interactive)
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdDiff(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		prID,
	)
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	// Diff is returned as plain text
	diffBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read diff: %w", err)
	}

	fmt.Print(string(diffBytes))
	return nil
}

// selectPRInteractively fetches PRs and prompts the user to select one
func selectPRInteractively(ctx context.Context, client *api.Client, workspace, repoSlug string) (int, error) {
	state := bitbucket.GetRepositoriesWorkspaceRepoSlugPullrequestsParamsState("OPEN")
	params := &bitbucket.GetRepositoriesWorkspaceRepoSlugPullrequestsParams{
		State: &state,
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugPullrequests(
		ctx,
		workspace,
		repoSlug,
		params,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to list pull requests: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("API error: %s", resp.Status)
	}

	var result bitbucket.PaginatedPullrequests
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	// Collect all PRs across pages.
	allPRs := make([]bitbucket.Pullrequest, 0)
	if result.Values != nil {
		allPRs = append(allPRs, *result.Values...)
	}
	for result.Next != nil && *result.Next != "" {
		nextResp, err := client.GetURL(ctx, *result.Next)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch next page: %w", err)
		}
		if nextResp.StatusCode != 200 {
			nextResp.Body.Close()
			return 0, fmt.Errorf("API error: %s", nextResp.Status)
		}
		result = bitbucket.PaginatedPullrequests{}
		if err := json.NewDecoder(nextResp.Body).Decode(&result); err != nil {
			nextResp.Body.Close()
			return 0, fmt.Errorf("failed to parse response: %w", err)
		}
		nextResp.Body.Close()
		if result.Values != nil {
			allPRs = append(allPRs, *result.Values...)
		}
	}

	if len(allPRs) == 0 {
		return 0, fmt.Errorf("no open pull requests found")
	}

	// Build options for interactive selection
	var options []interactive.Option
	for _, pr := range allPRs {
		id := 0
		if pr.Id != nil {
			id = *pr.Id
		}

		title := ""
		if pr.Title != nil {
			title = *pr.Title
		}

		author := ""
		if pr.Author != nil && pr.Author.DisplayName != nil {
			author = *pr.Author.DisplayName
		}

		options = append(options, interactive.Option{
			Label:       fmt.Sprintf("#%d: %s", id, output.Truncate(title, 50)),
			Value:       strconv.Itoa(id),
			Description: fmt.Sprintf("Author: %s", author),
		})
	}

	selected, err := interactive.Select("Select a pull request", options)
	if err != nil {
		return 0, err
	}

	prID, _ := strconv.Atoi(selected.Value)
	return prID, nil
}

// getPRID gets PR ID from args or interactive selection
func getPRID(ctx context.Context, args []string, client *api.Client, repoCtx *api.RepoContext, interactive bool) (int, error) {
	if len(args) >= 2 {
		prID, err := strconv.Atoi(args[1])
		if err != nil {
			return 0, fmt.Errorf("invalid PR number: %s", args[1])
		}
		return prID, nil
	}

	// Interactive mode
	if !interactive {
		return 0, fmt.Errorf("PR number required (use -i for interactive selection)")
	}

	return selectPRInteractively(ctx, client, repoCtx.Workspace, repoCtx.RepoSlug)
}
