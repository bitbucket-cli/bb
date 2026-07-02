package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var prChecksCmd = &cobra.Command{
	Use:   "checks [workspace/repo] [pr-number]",
	Short: "List build/CI statuses (checks) on a pull request",
	Long: `Show the build/CI statuses reported on a pull request.

Use -i for interactive selection when pr-number is omitted.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runPRChecks,
}

func runPRChecks(cmd *cobra.Command, args []string) error {
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

	resp, err := client.GetRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdStatuses(ctx, repoCtx.Workspace, repoCtx.RepoSlug, prID, nil)
	if err != nil {
		return fmt.Errorf("failed to get pull request statuses: %w", err)
	}
	return renderStatuses(ctx, client, resp)
}
