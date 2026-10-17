package main

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/bitbucket-cli/bb/internal/output"
	"github.com/spf13/cobra"
)

var (
	projectKeyValue string
	projectKeyLabel string
	projPermSet     string
	projPermLevel   string
	projPermRemove  string
	projPermGroups  bool
)

// ---- project deploy keys ----

var projectDeployKeyCmd = &cobra.Command{
	Use:     "deploy-key",
	Aliases: []string{"deploy-keys"},
	Short:   "Manage a project's deploy keys",
}

var projectDeployKeyListCmd = &cobra.Command{
	Use:   "list [workspace] <project-key>",
	Short: "List project deploy keys",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runProjectDeployKeyList,
}

var projectDeployKeyAddCmd = &cobra.Command{
	Use:   "add [workspace] <project-key>",
	Short: "Add a deploy key to a project",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runProjectDeployKeyAdd,
}

var projectDeployKeyDeleteCmd = &cobra.Command{
	Use:   "delete [workspace] <project-key> <key-id>",
	Short: "Delete a project deploy key",
	Args:  cobra.RangeArgs(2, 3),
	RunE:  runProjectDeployKeyDelete,
}

// ---- project permissions ----

var projectPermissionsCmd = &cobra.Command{
	Use:   "permissions [workspace] <project-key>",
	Short: "List or set a project's user/group permissions",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runProjectPermissions,
}

func init() {
	projectDeployKeyAddCmd.Flags().StringVar(&projectKeyValue, "key", "", "The public key (required)")
	projectDeployKeyAddCmd.Flags().StringVar(&projectKeyLabel, "label", "", "A label for the key")
	projectDeployKeyAddCmd.MarkFlagRequired("key")
	projectDeployKeyCmd.AddCommand(projectDeployKeyListCmd, projectDeployKeyAddCmd, projectDeployKeyDeleteCmd)

	projectPermissionsCmd.Flags().BoolVar(&projPermGroups, "groups", false, "Operate on group permissions instead of user permissions")
	projectPermissionsCmd.Flags().StringVar(&projPermSet, "set", "", "Grant/update permission for this user/group id")
	projectPermissionsCmd.Flags().StringVar(&projPermLevel, "permission", "", "Permission level for --set (read, write, admin, create-repo)")
	projectPermissionsCmd.Flags().StringVar(&projPermRemove, "remove", "", "Remove permission for this user/group id")

	projectCmd.AddCommand(projectDeployKeyCmd, projectPermissionsCmd)
}

func runProjectDeployKeyList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := projectNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 1)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodGet, fmt.Sprintf("/workspaces/%s/projects/%s/deploy-keys", ws, rest[0]), nil)
	if err != nil {
		return fmt.Errorf("failed to list deploy keys: %w", err)
	}
	type listItem struct {
		ID    int    `json:"id"`
		Label string `json:"label"`
		Key   string `json:"key"`
	}
	defer resp.Body.Close()
	items, err := fetchAllPages[listItem](ctx, client, resp)
	if err != nil {
		return err
	}
	if len(items) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No deploy keys")
		return nil
	}
	headers := []string{"ID", "LABEL", "KEY"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, k := range items {
		rows = append(rows, []string{fmt.Sprintf("%d", k.ID), k.Label, output.Truncate(k.Key, 40)})
		jsonData = append(jsonData, map[string]interface{}{"id": k.ID, "label": k.Label, "key": k.Key})
	}
	return output.Print(headers, rows, jsonData)
}

func runProjectDeployKeyAdd(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := projectNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 1)
	if err != nil {
		return err
	}
	body := map[string]interface{}{"key": projectKeyValue}
	if projectKeyLabel != "" {
		body["label"] = projectKeyLabel
	}
	resp, err := client.Request(ctx, http.MethodPost, fmt.Sprintf("/workspaces/%s/projects/%s/deploy-keys", ws, rest[0]), body)
	if err != nil {
		return fmt.Errorf("failed to add deploy key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Added deploy key to project %s/%s", ws, rest[0])
	return nil
}

func runProjectDeployKeyDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := projectNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 2)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodDelete, fmt.Sprintf("/workspaces/%s/projects/%s/deploy-keys/%s", ws, rest[0], rest[1]), nil)
	if err != nil {
		return fmt.Errorf("failed to delete deploy key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Deleted project deploy key %s", rest[1])
	return nil
}

func runProjectPermissions(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := projectNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 1)
	if err != nil {
		return err
	}
	kind := "users"
	if projPermGroups {
		kind = "groups"
	}
	base := fmt.Sprintf("/workspaces/%s/projects/%s/permissions-config/%s", ws, rest[0], kind)

	// Write paths
	if projPermRemove != "" {
		resp, rerr := client.Request(ctx, http.MethodDelete, base+"/"+url.PathEscape(projPermRemove), nil)
		if rerr != nil {
			return fmt.Errorf("failed to remove permission: %w", rerr)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("API error: %s", resp.Status)
		}
		output.Success("Removed %s permission for %s", kind, projPermRemove)
		return nil
	}
	if projPermSet != "" {
		if projPermLevel == "" {
			return fmt.Errorf("--permission is required with --set")
		}
		resp, rerr := client.Request(ctx, http.MethodPut, base+"/"+url.PathEscape(projPermSet),
			map[string]interface{}{"permission": projPermLevel})
		if rerr != nil {
			return fmt.Errorf("failed to set permission: %w", rerr)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("API error: %s", resp.Status)
		}
		output.Success("Set %s permission %q for %s", kind, projPermLevel, projPermSet)
		return nil
	}

	// Read
	resp, err := client.Request(ctx, http.MethodGet, base, nil)
	if err != nil {
		return fmt.Errorf("failed to list permissions: %w", err)
	}
	type listItem struct {
		Permission string `json:"permission"`
		User       *struct {
			DisplayName string `json:"display_name"`
			UUID        string `json:"uuid"`
		} `json:"user"`
		Group *struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"group"`
	}
	defer resp.Body.Close()
	items, err := fetchAllPages[listItem](ctx, client, resp)
	if err != nil {
		return err
	}
	if len(items) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No explicit permissions")
		return nil
	}
	headers := []string{"PERMISSION", "SUBJECT", "ID"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, p := range items {
		subject, id := "", ""
		if p.User != nil {
			subject, id = p.User.DisplayName, p.User.UUID
		} else if p.Group != nil {
			subject, id = p.Group.Name, p.Group.Slug
		}
		rows = append(rows, []string{p.Permission, subject, id})
		jsonData = append(jsonData, map[string]interface{}{"permission": p.Permission, "subject": subject, "id": id})
	}
	return output.Print(headers, rows, jsonData)
}
