package main

import (
	"fmt"
	"net/http"

	"github.com/ThomasRedstone/bb/internal/output"
	"github.com/spf13/cobra"
)

var issueMilestonesCmd = &cobra.Command{
	Use:   "milestones <workspace/repo>",
	Short: "List issue milestones",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return runIssueNamedList(cmd, args, "milestones") },
}

var issueVersionsCmd = &cobra.Command{
	Use:   "versions <workspace/repo>",
	Short: "List issue versions",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return runIssueNamedList(cmd, args, "versions") },
}

var issueComponentsCmd = &cobra.Command{
	Use:   "components <workspace/repo>",
	Short: "List issue components",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return runIssueNamedList(cmd, args, "components") },
}

func init() {
	issueCmd.AddCommand(issueMilestonesCmd)
	issueCmd.AddCommand(issueVersionsCmd)
	issueCmd.AddCommand(issueComponentsCmd)
}

// runIssueNamedList lists the issue-tracker metadata (milestones, versions, or
// components), which share an {id, name} shape.
func runIssueNamedList(cmd *cobra.Command, args []string, kind string) error {
	ctx := cmdCtx(cmd)

	client, err := issueNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	var resp *http.Response
	switch kind {
	case "milestones":
		resp, err = client.GetRepositoriesWorkspaceRepoSlugMilestones(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	case "versions":
		resp, err = client.GetRepositoriesWorkspaceRepoSlugVersions(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	case "components":
		resp, err = client.GetRepositoriesWorkspaceRepoSlugComponents(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	default:
		return fmt.Errorf("unknown kind %q", kind)
	}
	if err != nil {
		return fmt.Errorf("failed to list %s: %w", kind, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	type namedItem struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	items, err := fetchAllPages[namedItem](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(items) == 0 {
		output.Info("No %s found", kind)
		return nil
	}

	headers := []string{"ID", "NAME"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, v := range items {
		rows = append(rows, []string{fmt.Sprintf("%d", v.ID), v.Name})
		jsonData = append(jsonData, map[string]interface{}{"id": v.ID, "name": v.Name})
	}
	return output.Print(headers, rows, jsonData)
}
