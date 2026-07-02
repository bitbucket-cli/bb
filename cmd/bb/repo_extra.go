package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ThomasRedstone/bb/internal/output"
	"github.com/spf13/cobra"
)

var (
	repoPermissionsGroups bool
	branchingSetDev       string
	branchingSetProd      string
)

var repoBranchingModelCmd = &cobra.Command{
	Use:   "branching-model [workspace/repo]",
	Short: "Show or set the repository branching model",
	Long: `Display the repository's branching model (development/production branches and
branch types), or set the development/production branches with --set-development
and --set-production.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRepoBranchingModel,
}

var repoPermissionsCmd = &cobra.Command{
	Use:   "permissions [workspace/repo]",
	Short: "List repository user/group permissions",
	Long:  `List the explicit user (or, with --groups, group) permissions on a repository.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRepoPermissions,
}

var repoForksCmd = &cobra.Command{
	Use:   "forks [workspace/repo]",
	Short: "List repository forks",
	Long:  `List the forks of a repository.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRepoForks,
}

func init() {
	repoPermissionsCmd.Flags().BoolVar(&repoPermissionsGroups, "groups", false, "List group permissions instead of user permissions")
	repoBranchingModelCmd.Flags().StringVar(&branchingSetDev, "set-development", "", "Set the development branch name")
	repoBranchingModelCmd.Flags().StringVar(&branchingSetProd, "set-production", "", "Set the production branch name (enables production)")

	repoCmd.AddCommand(repoBranchingModelCmd)
	repoCmd.AddCommand(repoPermissionsCmd)
	repoCmd.AddCommand(repoForksCmd)
}

func runRepoBranchingModel(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := newAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	// Write path: --set-development / --set-production update the branching-model
	// settings via PUT; otherwise fall through to the read below.
	if branchingSetDev != "" || branchingSetProd != "" {
		body := map[string]interface{}{}
		if branchingSetDev != "" {
			body["development"] = map[string]interface{}{"name": branchingSetDev, "use_mainbranch": false}
		}
		if branchingSetProd != "" {
			body["production"] = map[string]interface{}{"name": branchingSetProd, "use_mainbranch": false, "enabled": true}
		}
		resp, err := client.Request(ctx, http.MethodPut,
			fmt.Sprintf("/repositories/%s/%s/branching-model/settings", repoCtx.Workspace, repoCtx.RepoSlug), body)
		if err != nil {
			return fmt.Errorf("failed to update branching model: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API error: %s", resp.Status)
		}
		output.Success("Updated branching model for %s/%s", repoCtx.Workspace, repoCtx.RepoSlug)
		return nil
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugBranchingModel(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	if err != nil {
		return fmt.Errorf("failed to get branching model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var model struct {
		Development struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
		} `json:"development"`
		Production struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
		} `json:"production"`
		BranchTypes []struct {
			Kind   string `json:"kind"`
			Prefix string `json:"prefix"`
		} `json:"branch_types"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&model); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if jsonOutput {
		return output.Print(nil, nil, model)
	}
	fmt.Printf("Development: %s\n", model.Development.Branch.Name)
	fmt.Printf("Production:  %s\n", model.Production.Branch.Name)
	fmt.Printf("Branch types:\n")
	for _, bt := range model.BranchTypes {
		fmt.Printf("  - %s (%s)\n", bt.Kind, bt.Prefix)
	}
	return nil
}

func runRepoPermissions(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := newAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	var resp *http.Response
	if repoPermissionsGroups {
		resp, err = client.GetRepositoriesWorkspaceRepoSlugPermissionsConfigGroups(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	} else {
		resp, err = client.GetRepositoriesWorkspaceRepoSlugPermissionsConfigUsers(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	}
	if err != nil {
		return fmt.Errorf("failed to list permissions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []struct {
			Permission string `json:"permission"`
			User       *struct {
				DisplayName string `json:"display_name"`
			} `json:"user"`
			Group *struct {
				Name string `json:"name"`
			} `json:"group"`
		} `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Values) == 0 {
		output.Info("No permissions found")
		return nil
	}

	headers := []string{"NAME", "PERMISSION"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, p := range result.Values {
		name := ""
		if p.User != nil {
			name = p.User.DisplayName
		} else if p.Group != nil {
			name = p.Group.Name
		}
		rows = append(rows, []string{name, p.Permission})
		jsonData = append(jsonData, map[string]interface{}{"name": name, "permission": p.Permission})
	}
	return output.Print(headers, rows, jsonData)
}

func runRepoForks(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := newAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugForks(ctx, repoCtx.Workspace, repoCtx.RepoSlug, nil)
	if err != nil {
		return fmt.Errorf("failed to list forks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []struct {
			FullName  string `json:"full_name"`
			IsPrivate bool   `json:"is_private"`
			Owner     struct {
				DisplayName string `json:"display_name"`
			} `json:"owner"`
		} `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Values) == 0 {
		output.Info("No forks found")
		return nil
	}

	headers := []string{"FULL NAME", "PRIVATE", "OWNER"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, f := range result.Values {
		priv := "No"
		if f.IsPrivate {
			priv = "Yes"
		}
		rows = append(rows, []string{f.FullName, priv, f.Owner.DisplayName})
		jsonData = append(jsonData, map[string]interface{}{"full_name": f.FullName, "is_private": f.IsPrivate, "owner": f.Owner.DisplayName})
	}
	return output.Print(headers, rows, jsonData)
}
