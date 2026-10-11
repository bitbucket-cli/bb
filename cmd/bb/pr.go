package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bitbucket-cli/bb/generated/bitbucket"
	"github.com/bitbucket-cli/bb/internal/interactive"
	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	bbpr "github.com/bitbucket-cli/bb/pkg/pr"
	"github.com/spf13/cobra"
)

var (
	prState        string
	prTitle        string
	prBody         string
	prSource       string
	prDest         string
	prReviewers    []string
	prComment      string
	prCommentPath  string
	prCommentLine  int
	prCommentReply int

	prDraft            bool
	prCloseSource      bool
	prMergeStrategy    string
	prMergeMessage     string
	prMergeCloseSource bool
	prApproveRemove    bool
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
	Use:   "list [workspace/repo]",
	Short: "List pull requests",
	Long:  `List pull requests in a repository.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPRList,
}

var prViewCmd = &cobra.Command{
	Use:   "view [workspace/repo] [pr-number]",
	Short: "View pull request details",
	Long: `Display detailed information about a pull request.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runPRView,
}

var prCreateCmd = &cobra.Command{
	Use:   "create [workspace/repo]",
	Short: "Create a pull request",
	Long:  `Create a new pull request.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPRCreate,
}

var prMergeCmd = &cobra.Command{
	Use:   "merge [workspace/repo] [pr-number]",
	Short: "Merge a pull request",
	Long: `Merge an open pull request.

Without --strategy the repository's default merge strategy is used.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runPRMerge,
}

var prApproveCmd = &cobra.Command{
	Use:   "approve [workspace/repo] [pr-number]",
	Short: "Approve a pull request",
	Long: `Add your approval to a pull request, or withdraw it with --remove.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runPRApprove,
}

var prDeclineCmd = &cobra.Command{
	Use:   "decline [workspace/repo] [pr-number]",
	Short: "Decline a pull request",
	Long: `Decline (reject) an open pull request without merging it.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runPRDecline,
}

var prCommentsCmd = &cobra.Command{
	Use:   "comments [workspace/repo] [pr-number]",
	Short: "List comments on a pull request",
	Long: `Display all comments on a pull request.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runPRComments,
}

var prCommentCmd = &cobra.Command{
	Use:   "comment [workspace/repo] [pr-number]",
	Short: "Add a comment to a pull request",
	Long: `Add a new comment to a pull request.

Use --reply-to <comment-id> to reply under an existing comment; the reply
inherits the parent's inline anchor. Comment IDs are shown by 'bb pr comments'
and 'bb pr threads'.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runPRComment,
}

var prDiffCmd = &cobra.Command{
	Use:   "diff [workspace/repo] [pr-number]",
	Short: "View pull request diff",
	Long: `Display the diff of changes in a pull request.

Use -i flag for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runPRDiff,
}

func init() {
	prListCmd.Flags().StringVarP(&prState, "state", "s", "OPEN", "Filter by state (OPEN, MERGED, DECLINED)")
	prListCmd.Flags().BoolVar(&prAll, "all", false, "List across all repos in the workspace (uses the workspace endpoint); defaults to your PRs")
	prListCmd.Flags().StringVar(&prUser, "user", "", "With --all, list PRs for this user (UUID or account_id) instead of yourself")
	prListCmd.Flags().BoolVar(&prLinks, "links", false, "Include a LINK column with each PR's URL (always present in --json)")

	prCreateCmd.Flags().StringVarP(&prTitle, "title", "t", "", "Pull request title (required)")
	prCreateCmd.Flags().StringVarP(&prBody, "body", "b", "", "Pull request description")
	prCreateCmd.Flags().StringVarP(&prSource, "source", "s", "", "Source branch (defaults to the current git branch)")
	prCreateCmd.Flags().StringVarP(&prDest, "dest", "d", "", "Destination branch (defaults to main branch)")
	prCreateCmd.Flags().StringSliceVarP(&prReviewers, "reviewer", "r", nil, "Reviewers, by display name/nickname substring or {uuid} (can be specified multiple times)")
	prCreateCmd.Flags().BoolP("interactive", "i", false, "Prompt for the title (and other fields) interactively")
	prCreateCmd.Flags().BoolVar(&prDraft, "draft", false, "Create the pull request as a draft")
	prCreateCmd.Flags().BoolVar(&prCloseSource, "close-source-branch", false, "Delete the source branch when the pull request is merged")
	prMergeCmd.Flags().StringVar(&prMergeStrategy, "strategy", "", "Merge strategy: merge_commit, squash, fast_forward, squash_fast_forward, rebase_fast_forward, rebase_merge (default: the repo's default)")
	prMergeCmd.Flags().StringVarP(&prMergeMessage, "message", "m", "", "Commit message for the merge commit")
	prMergeCmd.Flags().BoolVar(&prMergeCloseSource, "close-source-branch", false, "Delete the source branch after merging (use =false to keep it; default: the PR's setting)")
	prApproveCmd.Flags().BoolVar(&prApproveRemove, "remove", false, "Withdraw your approval")

	prCommentCmd.Flags().StringVarP(&prComment, "body", "b", "", "Comment text (required)")
	prCommentCmd.Flags().StringVar(&prCommentPath, "path", "", "File path for an inline comment")
	prCommentCmd.Flags().IntVar(&prCommentLine, "line", 0, "Line number (in the new version) for an inline comment; requires --path")
	prCommentCmd.Flags().IntVar(&prCommentReply, "reply-to", 0, "ID of the comment to reply to")
	prCommentCmd.MarkFlagsMutuallyExclusive("reply-to", "path")
	prCommentCmd.MarkFlagsMutuallyExclusive("reply-to", "line")
	prCommentCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("reply-to") && prCommentReply <= 0 {
			return fmt.Errorf("--reply-to must be a positive comment ID")
		}
		if cmd.Flags().Changed("line") && (prCommentPath == "" || prCommentLine <= 0) {
			return fmt.Errorf("--line must be positive and requires --path")
		}
		return nil
	}
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
	prChecksCmd.Flags().BoolP("interactive", "i", false, "Select PR interactively")
	prCmd.AddCommand(prChecksCmd)
}

func runPRList(cmd *cobra.Command, args []string) error {
	if prAll || prUser != "" {
		return runPRListAll(cmd, args)
	}

	repoCtx, err := repoArg(args)
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

	if len(prs) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No pull requests found")
		return nil
	}

	headers := []string{"#", "TITLE", "AUTHOR", "BRANCH", "STATE"}
	if prLinks {
		headers = append(headers, "LINK")
	}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, pr := range prs {
		row := []string{
			strconv.Itoa(pr.ID),
			output.Truncate(pr.Title, 50),
			pr.Author,
			pr.Branch,
			pr.State,
		}
		if prLinks {
			row = append(row, pr.URL)
		}
		rows = append(rows, row)
		jsonData = append(jsonData, map[string]interface{}{
			"id":     pr.ID,
			"title":  pr.Title,
			"author": pr.Author,
			"branch": pr.Branch,
			"state":  pr.State,
			"url":    pr.URL,
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

	repoCtx, rest, err := repoAndOptionalNumber(args)
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, rest, client, repoCtx, interactive)
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

	if output.CurrentFormat == output.FormatJSON {
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

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	// Default the source branch to the current git branch.
	if prSource == "" {
		branch, err := prCurrentBranch()
		if err != nil {
			return fmt.Errorf("no --source given and could not detect the current git branch: %w", err)
		}
		prSource = branch
	}

	// Title is required; prompt for it in interactive mode, else error.
	if prTitle == "" {
		interactiveMode, _ := cmd.Flags().GetBool("interactive")
		if interactiveMode {
			prTitle, err = interactive.Input("Pull request title", "")
			if err != nil {
				return err
			}
		}
		if prTitle == "" {
			return fmt.Errorf("a title is required (pass --title, or -i to enter it interactively)")
		}
	}

	// Default the description from a PR template when --body is not given.
	if prBody == "" {
		prBody = loadPRTemplate(".")
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
	if prDraft {
		prRequest["draft"] = true
	}
	if prCloseSource {
		prRequest["close_source_branch"] = true
	}

	// Bitbucket's create-pull-request API does not auto-apply the repo's
	// default reviewers (that only happens via the web UI), so fetch and
	// include them explicitly, alongside any manually specified reviewers.
	var reviewers []map[string]interface{}

	// The API rejects a PR whose author is listed as a reviewer, which
	// happens if the author is themselves a default reviewer, so resolve
	// the current user and exclude them below.
	selfUUID, err := currentUserUUID(ctx, client)
	if err != nil {
		output.Info("Warning: failed to resolve current user: %v", err)
	}

	defaultResp, err := client.GetRepositoriesWorkspaceRepoSlugDefaultReviewers(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
	)
	if err != nil {
		output.Info("Warning: failed to fetch default reviewers: %v", err)
	} else {
		defer defaultResp.Body.Close()
		if defaultResp.StatusCode == 200 {
			var result struct {
				Values []struct {
					UUID string `json:"uuid"`
				} `json:"values"`
			}
			if err := json.NewDecoder(defaultResp.Body).Decode(&result); err != nil {
				output.Info("Warning: failed to parse default reviewers: %v", err)
			} else {
				for _, dr := range result.Values {
					if selfUUID != "" && dr.UUID == selfUUID {
						continue
					}
					reviewers = append(reviewers, map[string]interface{}{
						"uuid": dr.UUID,
					})
				}
			}
		} else {
			output.Info("Warning: failed to fetch default reviewers: %s", defaultResp.Status)
		}
	}

	if len(prReviewers) > 0 {
		refs, err := resolveWorkspaceMembers(ctx, client, repoCtx.Workspace, prReviewers)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			reviewers = append(reviewers, map[string]interface{}{
				"uuid": ref.UUID,
			})
		}
	}

	if len(reviewers) > 0 {
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

	repoCtx, rest, err := repoAndOptionalNumber(args)
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, rest, client, repoCtx, interactive)
	if err != nil {
		return err
	}

	// Unset fields fall back to the repo's default strategy and the
	// PR's own close_source_branch setting.
	mergeBody := bitbucket.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdMergeJSONRequestBody{}
	if prMergeStrategy != "" {
		strategy, err := parseMergeStrategy(prMergeStrategy)
		if err != nil {
			return err
		}
		mergeBody.MergeStrategy = &strategy
	}
	if prMergeMessage != "" {
		mergeBody.Message = &prMergeMessage
	}
	if cmd.Flags().Changed("close-source-branch") {
		mergeBody.CloseSourceBranch = &prMergeCloseSource
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdMerge(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		prID,
		nil,
		mergeBody,
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

var mergeStrategies = []bitbucket.PullrequestMergeParametersMergeStrategy{
	bitbucket.MergeCommit,
	bitbucket.Squash,
	bitbucket.FastForward,
	bitbucket.SquashFastForward,
	bitbucket.RebaseFastForward,
	bitbucket.RebaseMerge,
}

// parseMergeStrategy validates s against the strategies the API accepts,
// so a typo fails locally rather than as an opaque 400.
func parseMergeStrategy(s string) (bitbucket.PullrequestMergeParametersMergeStrategy, error) {
	names := make([]string, len(mergeStrategies))
	for i, m := range mergeStrategies {
		if string(m) == s {
			return m, nil
		}
		names[i] = string(m)
	}
	return "", fmt.Errorf("unknown merge strategy %q (valid: %s)", s, strings.Join(names, ", "))
}

func runPRApprove(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndOptionalNumber(args)
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, rest, client, repoCtx, interactive)
	if err != nil {
		return err
	}

	var resp *http.Response
	if prApproveRemove {
		resp, err = client.DeleteRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdApprove(ctx, repoCtx.Workspace, repoCtx.RepoSlug, prID)
	} else {
		resp, err = client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdApprove(ctx, repoCtx.Workspace, repoCtx.RepoSlug, prID)
	}
	if err != nil {
		return fmt.Errorf("failed to update approval: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	if prApproveRemove {
		output.Success("Withdrew approval of pull request #%d", prID)
	} else {
		output.Success("Approved pull request #%d", prID)
	}
	return nil
}

func runPRDecline(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndOptionalNumber(args)
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, rest, client, repoCtx, interactive)
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

	repoCtx, rest, err := repoAndOptionalNumber(args)
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, rest, client, repoCtx, interactive)
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

	comments, err := fetchAllPages[bitbucket.PullrequestComment](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(comments) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No comments found")
		return nil
	}

	if output.CurrentFormat == output.FormatJSON {
		return output.JSON(comments)
	}

	var commentRows [][]string
	for _, comment := range comments {
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

		if output.CurrentFormat == output.FormatCSV {
			commentRows = append(commentRows, []string{strconv.FormatInt(id, 10), author, createdOn, location, content})
			continue
		}
		fmt.Printf("--- Comment #%d by %s on %s%s ---\n", id, author, createdOn, location)
		fmt.Printf("%s\n\n", content)
	}

	if output.CurrentFormat == output.FormatCSV {
		return output.Print([]string{"ID", "AUTHOR", "CREATED", "LOCATION", "CONTENT"}, commentRows, comments)
	}
	return nil
}

func runPRComment(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndOptionalNumber(args)
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, rest, client, repoCtx, interactive)
	if err != nil {
		return err
	}

	if err := postPRComment(ctx, client, repoCtx, prID, prComment, prCommentPath, prCommentLine, prCommentReply); err != nil {
		return err
	}

	if prCommentReply > 0 {
		output.Success("Replied to comment %d on pull request #%d", prCommentReply, prID)
		return nil
	}
	output.Success("Added comment to pull request #%d", prID)
	return nil
}

// postPRComment adds a comment to a pull request, optionally anchored inline to
// a file path and line (in the new version of the file), or as a reply to the
// comment with ID parent (when parent > 0).
func postPRComment(ctx context.Context, client *api.Client, repoCtx *api.RepoContext, prID int, text, path string, line, parent int) error {
	commentBody := map[string]interface{}{
		"content": map[string]interface{}{"raw": text},
	}
	if parent > 0 {
		commentBody["parent"] = map[string]interface{}{"id": parent}
	}
	if path != "" {
		inline := map[string]interface{}{"path": path}
		if line > 0 {
			inline["to"] = line
		}
		commentBody["inline"] = inline
	}

	body, err := json.Marshal(commentBody)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdCommentsWithBody(
		ctx, repoCtx.Workspace, repoCtx.RepoSlug, prID, "application/json", bytesReader(body))
	if err != nil {
		return fmt.Errorf("failed to add comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}
	return nil
}

func runPRDiff(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndOptionalNumber(args)
	if err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, rest, client, repoCtx, interactive)
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
func getPRID(ctx context.Context, idArgs []string, client *api.Client, repoCtx *api.RepoContext, interactive bool) (int, error) {
	if len(idArgs) >= 1 {
		prID, err := strconv.Atoi(idArgs[0])
		if err != nil {
			return 0, fmt.Errorf("invalid PR number: %s", idArgs[0])
		}
		return prID, nil
	}

	// Interactive mode
	if !interactive {
		return 0, fmt.Errorf("PR number required (use -i for interactive selection)")
	}

	return selectPRInteractively(ctx, client, repoCtx.Workspace, repoCtx.RepoSlug)
}
