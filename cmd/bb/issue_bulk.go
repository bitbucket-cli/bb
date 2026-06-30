package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ThomasRedstone/bb/internal/output"
	"github.com/spf13/cobra"
)

var issueBulkState string

var issueBulkCmd = &cobra.Command{
	Use:   "bulk <workspace/repo> <issue-id...>",
	Short: "Update the state of multiple issues",
	Long: `Set the state of several issues at once.

Example:
  bb issue bulk myws/myrepo 12 15 19 --state resolved`,
	Args: cobra.MinimumNArgs(2),
	RunE: runIssueBulk,
}

func init() {
	issueBulkCmd.Flags().StringVarP(&issueBulkState, "state", "s", "", "New state for the issues (required)")
	issueBulkCmd.MarkFlagRequired("state")
	issueCmd.AddCommand(issueBulkCmd)
}

func runIssueBulk(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := issueNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	body, err := json.Marshal(map[string]interface{}{"state": issueBulkState})
	if err != nil {
		return err
	}
	withBody := func(ctx context.Context, req *http.Request) error {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/json")
		return nil
	}

	ids := args[1:]
	errs := runConcurrent(ids, 8, func(id string) error {
		resp, err := client.PutRepositoriesWorkspaceRepoSlugIssuesIssueId(ctx, repoCtx.Workspace, repoCtx.RepoSlug, id, withBody)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("%s", resp.Status)
		}
		return nil
	})

	var failures int
	for i, e := range errs {
		if e != nil {
			output.Error("issue #%s update failed: %v", ids[i], e)
			failures++
		} else {
			output.Success("issue #%s → %s", ids[i], issueBulkState)
		}
	}

	if failures > 0 {
		return fmt.Errorf("%d of %d issues failed", failures, len(ids))
	}
	return nil
}
