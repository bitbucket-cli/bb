package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var prRequestChangesRemove bool

var prRequestChangesCmd = &cobra.Command{
	Use:   "request-changes <workspace/repo> [pr-number]",
	Short: "Request changes on a pull request",
	Long: `Mark a pull request as requiring changes.

Use --remove to withdraw a previous request-changes. Use -i to select the PR
interactively when pr-number is omitted.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runPRRequestChanges,
}

func init() {
	prRequestChangesCmd.Flags().BoolVar(&prRequestChangesRemove, "remove", false, "Withdraw a previous request-changes")
	prRequestChangesCmd.Flags().BoolP("interactive", "i", false, "Select PR interactively")
	prCmd.AddCommand(prRequestChangesCmd)
}

func runPRRequestChanges(cmd *cobra.Command, args []string) error {
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

	var resp *http.Response
	if prRequestChangesRemove {
		resp, err = client.DeleteRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdRequestChanges(ctx, repoCtx.Workspace, repoCtx.RepoSlug, prID)
	} else {
		resp, err = client.PostRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdRequestChanges(ctx, repoCtx.Workspace, repoCtx.RepoSlug, prID)
	}
	if err != nil {
		return fmt.Errorf("failed to update request-changes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	if prRequestChangesRemove {
		output.Success("Withdrew request-changes on pull request #%d", prID)
	} else {
		output.Success("Requested changes on pull request #%d", prID)
	}
	return nil
}
