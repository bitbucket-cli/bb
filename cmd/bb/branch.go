package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ThomasRedstone/bb/generated/bitbucket"
	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	branchFrom           string
	restrictionKind      string
	restrictionPattern   string
	restrictionValue     int
	restrictionMatchKind string
)

// Mockable dependencies for testing
var (
	branchNewAPIClient = api.NewClient
)

var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Manage branches",
	Long:  `List, create, and delete branches.`,
}

var branchListCmd = &cobra.Command{
	Use:   "list <workspace/repo>",
	Short: "List branches",
	Long:  `List branches in a repository.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runBranchList,
}

var branchCreateCmd = &cobra.Command{
	Use:   "create <workspace/repo> <branch-name>",
	Short: "Create a branch",
	Long:  `Create a new branch in a repository.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runBranchCreate,
}

var branchDeleteCmd = &cobra.Command{
	Use:   "delete <workspace/repo> <branch-name>",
	Short: "Delete a branch",
	Long:  `Delete a branch from a repository.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runBranchDelete,
}

var branchRestrictCmd = &cobra.Command{
	Use:   "restrict",
	Short: "Manage branch restrictions",
	Long:  `List, create, and delete branch restrictions (branch protection rules).`,
}

var branchRestrictListCmd = &cobra.Command{
	Use:   "list <workspace/repo>",
	Short: "List branch restrictions",
	Long:  `List all branch restrictions configured for a repository.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runBranchRestrictList,
}

var branchRestrictCreateCmd = &cobra.Command{
	Use:   "create <workspace/repo>",
	Short: "Create a branch restriction",
	Long: `Create a new branch restriction.

Available kinds:
  push                              - Restrict push access
  force                             - Restrict force push
  delete                            - Restrict branch deletion
  restrict_merges                   - Restrict merges
  require_passing_builds_to_merge   - Require passing builds
  require_approvals_to_merge        - Require minimum approvals
  require_all_comments_resolved     - Require all comments resolved
  require_tasks_to_be_completed     - Require all tasks completed
  reset_pullrequest_approvals_on_change - Reset approvals on change

Example:
  bb branch restrict create myworkspace/myrepo --kind require_approvals_to_merge --pattern "main" --value 2`,
	Args: cobra.ExactArgs(1),
	RunE: runBranchRestrictCreate,
}

var branchRestrictDeleteCmd = &cobra.Command{
	Use:   "delete <workspace/repo> <restriction-id>",
	Short: "Delete a branch restriction",
	Long:  `Delete a branch restriction from a repository.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runBranchRestrictDelete,
}

var branchRestrictViewCmd = &cobra.Command{
	Use:   "view <workspace/repo> <restriction-id>",
	Short: "View a branch restriction",
	Long:  `View details of a specific branch restriction.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runBranchRestrictView,
}

func init() {
	branchCreateCmd.Flags().StringVarP(&branchFrom, "from", "f", "", "Source branch or commit (defaults to main branch)")

	// Restriction create flags
	branchRestrictCreateCmd.Flags().StringVarP(&restrictionKind, "kind", "k", "", "Restriction kind (required)")
	branchRestrictCreateCmd.Flags().StringVarP(&restrictionPattern, "pattern", "p", "", "Branch pattern (e.g., 'main', 'release/*')")
	branchRestrictCreateCmd.Flags().IntVarP(&restrictionValue, "value", "v", 0, "Value (for require_approvals_to_merge, etc.)")
	branchRestrictCreateCmd.Flags().StringVarP(&restrictionMatchKind, "match-kind", "m", "glob", "Branch match kind (glob or branching_model)")
	branchRestrictCreateCmd.MarkFlagRequired("kind")
	branchRestrictCreateCmd.MarkFlagRequired("pattern")

	// Restriction subcommands
	branchRestrictCmd.AddCommand(branchRestrictListCmd)
	branchRestrictCmd.AddCommand(branchRestrictCreateCmd)
	branchRestrictCmd.AddCommand(branchRestrictDeleteCmd)
	branchRestrictCmd.AddCommand(branchRestrictViewCmd)

	branchCmd.AddCommand(branchListCmd)
	branchCmd.AddCommand(branchCreateCmd)
	branchCmd.AddCommand(branchDeleteCmd)
	branchCmd.AddCommand(branchRestrictCmd)
}

func runBranchList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := branchNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	params := &bitbucket.GetRepositoriesWorkspaceRepoSlugRefsBranchesParams{}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugRefsBranches(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		params,
	)
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result bitbucket.PaginatedBranches
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Collect all branches across pages.
	allBranches := make([]bitbucket.Branch, 0)
	if result.Values != nil {
		allBranches = append(allBranches, *result.Values...)
	}
	for result.Next != nil && *result.Next != "" {
		nextResp, err := client.GetURL(ctx, *result.Next)
		if err != nil {
			return fmt.Errorf("failed to fetch next page: %w", err)
		}
		result = bitbucket.PaginatedBranches{}
		if err := json.NewDecoder(nextResp.Body).Decode(&result); err != nil {
			nextResp.Body.Close()
			return fmt.Errorf("failed to parse response: %w", err)
		}
		nextResp.Body.Close()
		if result.Values != nil {
			allBranches = append(allBranches, *result.Values...)
		}
	}

	if len(allBranches) == 0 {
		output.Info("No branches found")
		return nil
	}

	headers := []string{"NAME", "COMMIT", "MESSAGE"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, branch := range allBranches {
		name := ""
		if branch.Name != nil {
			name = *branch.Name
		}

		commit := ""
		message := ""
		if branch.Target != nil {
			if branch.Target.Hash != nil {
				h := *branch.Target.Hash
				commit = h[:min(12, len(h))]
			}
			if branch.Target.Message != nil {
				message = output.Truncate(*branch.Target.Message, 50)
			}
		}

		rows = append(rows, []string{name, commit, message})
		jsonData = append(jsonData, map[string]interface{}{
			"name":   name,
			"commit": commit,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runBranchCreate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := branchNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	branchName := args[1]

	// Get source commit/branch
	sourceBranch := branchFrom
	if sourceBranch == "" {
		// Get repo to find main branch
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
			sourceBranch = *repo.Mainbranch.Name
		} else {
			sourceBranch = "main"
		}
	}

	// Get the commit hash for the source branch
	resp, err := client.GetRepositoriesWorkspaceRepoSlugRefsBranchesName(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		sourceBranch,
	)
	if err != nil {
		return fmt.Errorf("failed to get source branch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("source branch '%s' not found", sourceBranch)
	}

	var sourceBranchData bitbucket.Branch
	if err := json.NewDecoder(resp.Body).Decode(&sourceBranchData); err != nil {
		return fmt.Errorf("failed to parse source branch: %w", err)
	}

	if sourceBranchData.Target == nil || sourceBranchData.Target.Hash == nil {
		return fmt.Errorf("could not get commit hash for source branch")
	}

	// Create the new branch using RequestEditorFn to inject body
	branchRequest := map[string]interface{}{
		"name": branchName,
		"target": map[string]interface{}{
			"hash": *sourceBranchData.Target.Hash,
		},
	}

	body, err := json.Marshal(branchRequest)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	// Use RequestEditorFn to inject the body
	withBody := func(ctx context.Context, req *http.Request) error {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/json")
		return nil
	}

	createResp, err := client.PostRepositoriesWorkspaceRepoSlugRefsBranches(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		withBody,
	)
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != 201 {
		var errResp map[string]interface{}
		json.NewDecoder(createResp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", createResp.Status, errResp)
	}

	output.Success("Created branch '%s' from '%s'", branchName, sourceBranch)
	return nil
}

func runBranchDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := branchNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	branchName := args[1]

	resp, err := client.DeleteRepositoriesWorkspaceRepoSlugRefsBranchesName(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		branchName,
	)
	if err != nil {
		return fmt.Errorf("failed to delete branch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Deleted branch '%s'", branchName)
	return nil
}

func runBranchRestrictList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := branchNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugBranchRestrictions(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to list branch restrictions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []struct {
			ID              int    `json:"id"`
			Kind            string `json:"kind"`
			Pattern         string `json:"pattern"`
			BranchMatchKind string `json:"branch_match_kind"`
			Value           *int   `json:"value"`
		} `json:"values"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Values) == 0 {
		output.Info("No branch restrictions found")
		return nil
	}

	headers := []string{"ID", "KIND", "PATTERN", "VALUE"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, restriction := range result.Values {
		value := ""
		if restriction.Value != nil {
			value = fmt.Sprintf("%d", *restriction.Value)
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", restriction.ID),
			restriction.Kind,
			restriction.Pattern,
			value,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"id":                restriction.ID,
			"kind":              restriction.Kind,
			"pattern":           restriction.Pattern,
			"branch_match_kind": restriction.BranchMatchKind,
			"value":             restriction.Value,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runBranchRestrictCreate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := branchNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	restrictionReq := map[string]interface{}{
		"kind":              restrictionKind,
		"pattern":           restrictionPattern,
		"branch_match_kind": restrictionMatchKind,
	}

	if restrictionValue > 0 {
		restrictionReq["value"] = restrictionValue
	}

	body, err := json.Marshal(restrictionReq)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugBranchRestrictionsWithBody(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create branch restriction: %w", err)
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

	output.Success("Created branch restriction #%d", result.ID)
	return nil
}

func runBranchRestrictDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := branchNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	restrictionID := args[1]

	resp, err := client.DeleteRepositoriesWorkspaceRepoSlugBranchRestrictionsId(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		restrictionID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete branch restriction: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Deleted branch restriction #%s", restrictionID)
	return nil
}

func runBranchRestrictView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := branchNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	restrictionID := args[1]

	resp, err := client.GetRepositoriesWorkspaceRepoSlugBranchRestrictionsId(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		restrictionID,
	)
	if err != nil {
		return fmt.Errorf("failed to get branch restriction: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var restriction struct {
		ID              int    `json:"id"`
		Kind            string `json:"kind"`
		Pattern         string `json:"pattern"`
		BranchMatchKind string `json:"branch_match_kind"`
		Value           *int   `json:"value"`
		Users           []struct {
			DisplayName string `json:"display_name"`
		} `json:"users"`
		Groups []struct {
			Name string `json:"name"`
		} `json:"groups"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&restriction); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if jsonOutput {
		return output.Print(nil, nil, restriction)
	}

	fmt.Printf("ID:          %d\n", restriction.ID)
	fmt.Printf("Kind:        %s\n", restriction.Kind)
	fmt.Printf("Pattern:     %s\n", restriction.Pattern)
	fmt.Printf("Match Kind:  %s\n", restriction.BranchMatchKind)
	if restriction.Value != nil {
		fmt.Printf("Value:       %d\n", *restriction.Value)
	}
	if len(restriction.Users) > 0 {
		fmt.Printf("Users:\n")
		for _, user := range restriction.Users {
			fmt.Printf("  - %s\n", user.DisplayName)
		}
	}
	if len(restriction.Groups) > 0 {
		fmt.Printf("Groups:\n")
		for _, group := range restriction.Groups {
			fmt.Printf("  - %s\n", group.Name)
		}
	}

	return nil
}
