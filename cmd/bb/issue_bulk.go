package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var issueBulkState string

var issueBulkCmd = &cobra.Command{
	Use:   "bulk [workspace/repo] <issue-id...>",
	Short: "Update the state of multiple issues",
	Long: `Set the state of several issues at once.

Example:
  bb issue bulk myws/myrepo 12 15 19 --state resolved`,
	Args: cobra.MinimumNArgs(1),
	RunE: runIssueBulk,
}

func init() {
	issueBulkCmd.Flags().Bool("dry-run", false, "Validate and display the batch without authentication or API requests")
	issueBulkCmd.Flags().StringVarP(&issueBulkState, "state", "s", "", "New state for the issues (required)")
	issueBulkCmd.MarkFlagRequired("state")
	issueCmd.AddCommand(issueBulkCmd)
}

func runIssueBulk(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	repoCtx, ids, err := repoAndIDs(args)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return fmt.Errorf("no issue ids given; usage: bb issue bulk [workspace/repo] <issue-id...>")
	}

	numbers, err := parseBulkIDs(ids, "issue")
	if err != nil {
		return err
	}
	if strings.TrimSpace(issueBulkState) == "" {
		return fmt.Errorf("issue state is required")
	}
	results := bulkPlan(repoCtx.Workspace+"/"+repoCtx.RepoSlug, "issue", "update", numbers)
	for i := range results {
		results[i].State = issueBulkState
	}
	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		return reportBulk(results)
	}

	body, err := json.Marshal(map[string]interface{}{"state": issueBulkState})
	if err != nil {
		return err
	}
	client, err := issueNewAPIClient()
	if err != nil {
		return err
	}

	withBody := func(ctx context.Context, req *http.Request) error {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/json")
		return nil
	}
	errs := runConcurrent(numbers, 8, func(id int) error {
		resp, err := client.PutRepositoriesWorkspaceRepoSlugIssuesIssueId(ctx, repoCtx.Workspace, repoCtx.RepoSlug, strconv.Itoa(id), withBody)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("%s", resp.Status)
		}
		return nil
	})

	completeBulk(results, errs)
	return reportBulk(results)
}
