package main

import (
	"fmt"
	"net/http"

	generated "github.com/bitbucket-cli/bb/generated/bitbucket"
	"github.com/spf13/cobra"
)

var prBulkCmd = &cobra.Command{
	Use:   "bulk",
	Short: "Run an action across multiple pull requests",
	Long:  `Approve, decline, or merge several pull requests in one command.`,
}

var prBulkApproveCmd = &cobra.Command{
	Use:   "approve [workspace/repo] <pr-number...>",
	Short: "Approve multiple pull requests",
	Args:  cobra.MinimumNArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return runPRBulk(cmd, args, "approve") },
}

var prBulkDeclineCmd = &cobra.Command{
	Use:   "decline [workspace/repo] <pr-number...>",
	Short: "Decline multiple pull requests",
	Args:  cobra.MinimumNArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return runPRBulk(cmd, args, "decline") },
}

var prBulkMergeCmd = &cobra.Command{
	Use:   "merge [workspace/repo] <pr-number...>",
	Short: "Merge multiple pull requests",
	Long: `Merge several pull requests, one API call each, run concurrently.

Without --strategy each PR is merged with the repository's default strategy.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error { return runPRBulk(cmd, args, "merge") },
}

var (
	prBulkMergeStrategy    string
	prBulkMergeCloseSource bool
)

func init() {
	for _, cmd := range []*cobra.Command{prBulkApproveCmd, prBulkDeclineCmd, prBulkMergeCmd} {
		cmd.Flags().Bool("dry-run", false, "Validate and display the batch without authentication or API requests")
	}
	prBulkMergeCmd.Flags().StringVar(&prBulkMergeStrategy, "strategy", "", "Merge strategy: merge_commit, squash, fast_forward, squash_fast_forward, rebase_fast_forward, rebase_merge (default: the repo's default)")
	prBulkMergeCmd.Flags().BoolVar(&prBulkMergeCloseSource, "close-source-branch", false, "Delete each source branch after merging (use =false to keep them; default: each PR's setting)")
	prBulkCmd.AddCommand(prBulkApproveCmd)
	prBulkCmd.AddCommand(prBulkDeclineCmd)
	prBulkCmd.AddCommand(prBulkMergeCmd)
	prCmd.AddCommand(prBulkCmd)
}

func runPRBulk(cmd *cobra.Command, args []string, action string) error {
	ctx := cmdCtx(cmd)

	repoCtx, ids, err := repoAndIDs(args)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return fmt.Errorf("no pull request numbers given; usage: bb pr bulk %s [workspace/repo] <pr-number...>", action)
	}

	numbers, err := parseBulkIDs(ids, "PR")
	if err != nil {
		return err
	}
	switch action {
	case "approve", "decline", "merge":
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	results := bulkPlan(repoCtx.Workspace+"/"+repoCtx.RepoSlug, "pr", action, numbers)

	// Built once and shared: every PR is merged with the same parameters.
	mergeBody := generated.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdMergeJSONRequestBody{}
	if action == "merge" {
		if prBulkMergeStrategy != "" {
			strategy, err := parseMergeStrategy(prBulkMergeStrategy)
			if err != nil {
				return err
			}
			mergeBody.MergeStrategy = &strategy
		}
		if cmd.Flags().Changed("close-source-branch") {
			mergeBody.CloseSourceBranch = &prBulkMergeCloseSource
		}
	}

	for i := range results {
		if mergeBody.MergeStrategy != nil {
			results[i].Strategy = string(*mergeBody.MergeStrategy)
		}
		results[i].CloseSourceBranch = mergeBody.CloseSourceBranch
	}
	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		return reportBulk(results)
	}
	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	// Independent calls — run them concurrently.
	errs := runConcurrent(numbers, 8, func(id int) error {
		var resp *http.Response
		var err error
		switch action {
		case "approve":
			resp, err = client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdApprove(ctx, repoCtx.Workspace, repoCtx.RepoSlug, id)
		case "decline":
			resp, err = client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdDecline(ctx, repoCtx.Workspace, repoCtx.RepoSlug, id)
		case "merge":
			resp, err = client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdMerge(ctx, repoCtx.Workspace, repoCtx.RepoSlug, id, nil, mergeBody)
		default:
			return fmt.Errorf("unknown action %q", action)
		}
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("%s", resp.Status)
		}
		return nil
	})

	completeBulk(results, errs)
	return reportBulk(results)
}
