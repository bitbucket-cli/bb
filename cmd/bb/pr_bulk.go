package main

import (
	"fmt"
	"net/http"
	"strconv"

	generated "github.com/ThomasRedstone/bb/generated/bitbucket"
	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var prBulkCmd = &cobra.Command{
	Use:   "bulk",
	Short: "Run an action across multiple pull requests",
	Long:  `Approve, decline, or merge several pull requests in one command.`,
}

var prBulkApproveCmd = &cobra.Command{
	Use:   "approve <workspace/repo> <pr-number...>",
	Short: "Approve multiple pull requests",
	Args:  cobra.MinimumNArgs(2),
	RunE:  func(cmd *cobra.Command, args []string) error { return runPRBulk(cmd, args, "approve") },
}

var prBulkDeclineCmd = &cobra.Command{
	Use:   "decline <workspace/repo> <pr-number...>",
	Short: "Decline multiple pull requests",
	Args:  cobra.MinimumNArgs(2),
	RunE:  func(cmd *cobra.Command, args []string) error { return runPRBulk(cmd, args, "decline") },
}

var prBulkMergeCmd = &cobra.Command{
	Use:   "merge <workspace/repo> <pr-number...>",
	Short: "Merge multiple pull requests (default merge strategy)",
	Args:  cobra.MinimumNArgs(2),
	RunE:  func(cmd *cobra.Command, args []string) error { return runPRBulk(cmd, args, "merge") },
}

func init() {
	prBulkCmd.AddCommand(prBulkApproveCmd)
	prBulkCmd.AddCommand(prBulkDeclineCmd)
	prBulkCmd.AddCommand(prBulkMergeCmd)
	prCmd.AddCommand(prBulkCmd)
}

func runPRBulk(cmd *cobra.Command, args []string, action string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	var failures int
	ids := args[1:]
	for _, idStr := range ids {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			output.Error("invalid PR number %q", idStr)
			failures++
			continue
		}

		var resp *http.Response
		switch action {
		case "approve":
			resp, err = client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdApprove(ctx, repoCtx.Workspace, repoCtx.RepoSlug, id)
		case "decline":
			resp, err = client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdDecline(ctx, repoCtx.Workspace, repoCtx.RepoSlug, id)
		case "merge":
			resp, err = client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdMerge(ctx, repoCtx.Workspace, repoCtx.RepoSlug, id, nil, generated.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdMergeJSONRequestBody{})
		default:
			return fmt.Errorf("unknown action %q", action)
		}

		if err != nil {
			output.Error("PR #%d %s failed: %v", id, action, err)
			failures++
			continue
		}
		if resp.StatusCode >= 300 {
			output.Error("PR #%d %s failed: %s", id, action, resp.Status)
			failures++
		} else {
			output.Success("PR #%d %s", id, action)
		}
		resp.Body.Close()
	}

	if failures > 0 {
		return fmt.Errorf("%d of %d pull requests failed", failures, len(ids))
	}
	return nil
}
