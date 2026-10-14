package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/bitbucket-cli/bb/internal/output"
	"github.com/spf13/cobra"
)

var (
	repoPermissionsGroups bool
	branchingSetDev       string
	branchingSetProd      string
	permSetTarget         string
	permSetLevel          string
	permRemoveTarget      string
)

var repoBranchingModelCmd = &cobra.Command{
	Use:   "branching-model [workspace/repo]",
	Short: "Show or set the repository branching model",
	Long: `Display the repository's effective branching model (development/production
branches and branch types) and whether it is inherited from the project or set on
the repository. A production branch that is absent or disabled shows as "(not set)".
Set the development/production branches with --set-development and --set-production.`,
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
	repoPermissionsCmd.Flags().BoolVar(&repoPermissionsGroups, "groups", false, "Operate on group permissions instead of user permissions")
	repoPermissionsCmd.Flags().StringVar(&permSetTarget, "set", "", "Grant/update permission for this user UUID/account_id (or group slug with --groups)")
	repoPermissionsCmd.Flags().StringVar(&permSetLevel, "permission", "", "Permission level for --set (read, write, admin)")
	repoPermissionsCmd.Flags().StringVar(&permRemoveTarget, "remove", "", "Remove permission for this user/group")
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

	// The effective model is what Bitbucket actually applies: the repo's own
	// settings when it overrides the project, otherwise the project's.
	resp, err := client.GetRepositoriesWorkspaceRepoSlugEffectiveBranchingModel(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	if err != nil {
		return fmt.Errorf("failed to get branching model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var model branchingModel
	if err := json.NewDecoder(resp.Body).Decode(&model); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// override-settings says whether the repo inherits the project's
	// branching model or uses its own. Fall back to "unknown" rather than
	// failing the read if it isn't available.
	model.Source = "unknown"
	if oresp, err := client.GetRepositoriesWorkspaceRepoSlugOverrideSettings(ctx, repoCtx.Workspace, repoCtx.RepoSlug); err == nil {
		defer oresp.Body.Close()
		if oresp.StatusCode == 200 {
			var ov map[string]interface{}
			if json.NewDecoder(oresp.Body).Decode(&ov) == nil {
				model.Source = branchingModelSource(ov)
			}
		}
	}

	if output.CurrentFormat == output.FormatJSON {
		return output.Print(nil, nil, model)
	}
	fmt.Printf("Source:      %s\n", model.Source)
	fmt.Printf("Development: %s\n", branchOrUnset(model.Development))
	fmt.Printf("Production:  %s\n", branchOrUnset(model.Production))
	fmt.Printf("Branch types:\n")
	for _, bt := range model.BranchTypes {
		fmt.Printf("  - %s (%s)\n", bt.Kind, bt.Prefix)
	}
	return nil
}

type branchingModelBranch struct {
	Name   string `json:"name,omitempty"`
	Branch *struct {
		Name string `json:"name"`
	} `json:"branch,omitempty"`
	UseMainbranch bool `json:"use_mainbranch,omitempty"`
}

type branchingModel struct {
	Source      string                `json:"source"`
	Development *branchingModelBranch `json:"development,omitempty"`
	Production  *branchingModelBranch `json:"production,omitempty"`
	BranchTypes []struct {
		Kind   string `json:"kind"`
		Prefix string `json:"prefix"`
	} `json:"branch_types"`
}

// branchOrUnset renders a development/production entry, distinguishing a
// configured branch from one that is not set (absent or disabled) or one
// whose configured name does not exist as a branch.
func branchOrUnset(b *branchingModelBranch) string {
	switch {
	case b == nil:
		return "(not set)"
	case b.Branch != nil && b.Branch.Name != "":
		return b.Branch.Name
	case b.UseMainbranch:
		return "(main branch, not found)"
	case b.Name != "":
		return b.Name + " (branch not found)"
	default:
		return "(not set)"
	}
}

// branchingModelSource maps the override-settings response to where the
// effective branching model comes from. Despite the endpoint's name,
// branching_model=true means the repo inherits the project's model and
// false means the repo uses its own settings (checked against the
// effective vs repo-level models on live repos).
func branchingModelSource(ov map[string]interface{}) string {
	v, ok := ov["branching_model"].(bool)
	if !ok {
		return "unknown"
	}
	if v {
		return "project (inherited)"
	}
	return "repository (overrides project)"
}

func runRepoPermissions(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := newAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := repoArg(args)
	if err == nil && (permSetTarget != "" || permRemoveTarget != "") {
		kind := "users"
		if repoPermissionsGroups {
			kind = "groups"
		}
		if permRemoveTarget != "" {
			resp, rerr := client.Request(ctx, http.MethodDelete,
				fmt.Sprintf("/repositories/%s/%s/permissions-config/%s/%s", repoCtx.Workspace, repoCtx.RepoSlug, kind, url.PathEscape(permRemoveTarget)), nil)
			if rerr != nil {
				return fmt.Errorf("failed to remove permission: %w", rerr)
			}
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return fmt.Errorf("API error: %s", resp.Status)
			}
			output.Success("Removed %s permission for %s", kind, permRemoveTarget)
			return nil
		}
		if permSetLevel == "" {
			return fmt.Errorf("--permission is required with --set (read, write, admin)")
		}
		resp, rerr := client.Request(ctx, http.MethodPut,
			fmt.Sprintf("/repositories/%s/%s/permissions-config/%s/%s", repoCtx.Workspace, repoCtx.RepoSlug, kind, url.PathEscape(permSetTarget)),
			map[string]interface{}{"permission": permSetLevel})
		if rerr != nil {
			return fmt.Errorf("failed to set permission: %w", rerr)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("API error: %s", resp.Status)
		}
		output.Success("Set %s permission %q for %s", kind, permSetLevel, permSetTarget)
		return nil
	}
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

	type repoPermission struct {
		Permission string `json:"permission"`
		User       *struct {
			DisplayName string `json:"display_name"`
		} `json:"user"`
		Group *struct {
			Name string `json:"name"`
		} `json:"group"`
	}
	permissions, err := fetchAllPages[repoPermission](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(permissions) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No permissions found")
		return nil
	}

	headers := []string{"NAME", "PERMISSION"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, p := range permissions {
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

	type repoFork struct {
		FullName  string `json:"full_name"`
		IsPrivate bool   `json:"is_private"`
		Owner     struct {
			DisplayName string `json:"display_name"`
		} `json:"owner"`
	}
	forks, err := fetchAllPages[repoFork](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(forks) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No forks found")
		return nil
	}

	headers := []string{"FULL NAME", "PRIVATE", "OWNER"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, f := range forks {
		priv := "No"
		if f.IsPrivate {
			priv = "Yes"
		}
		rows = append(rows, []string{f.FullName, priv, f.Owner.DisplayName})
		jsonData = append(jsonData, map[string]interface{}{"full_name": f.FullName, "is_private": f.IsPrivate, "owner": f.Owner.DisplayName})
	}
	return output.Print(headers, rows, jsonData)
}
