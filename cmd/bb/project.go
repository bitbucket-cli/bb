package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	projectName    string
	projectDesc    string
	projectPrivate bool
)

// Mockable dependency for testing.
var projectNewAPIClient = api.NewClient

type project struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"is_private"`
}

var projectCmd = &cobra.Command{
	Use:     "project",
	Aliases: []string{"projects"},
	Short:   "Manage workspace projects",
	Long:    `List, view, create, and delete projects within a workspace.`,
}

var projectListCmd = &cobra.Command{
	Use:   "list [workspace]",
	Short: "List projects in a workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runProjectList,
}

var projectViewCmd = &cobra.Command{
	Use:   "view [workspace] <project-key>",
	Short: "View a project",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runProjectView,
}

var projectCreateCmd = &cobra.Command{
	Use:   "create [workspace] <project-key>",
	Short: "Create a project",
	Long:  `Create a project in a workspace. The key is the short uppercase identifier (e.g. CLOUD).`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runProjectCreate,
}

var projectDeleteCmd = &cobra.Command{
	Use:   "delete [workspace] <project-key>",
	Short: "Delete a project",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runProjectDelete,
}

func init() {
	projectCreateCmd.Flags().StringVarP(&projectName, "name", "n", "", "Project name (defaults to the key)")
	projectCreateCmd.Flags().StringVarP(&projectDesc, "description", "d", "", "Project description")
	projectCreateCmd.Flags().BoolVar(&projectPrivate, "private", true, "Make the project private")

	projectReviewersCmd.AddCommand(projectReviewersListCmd, projectReviewersAddCmd, projectReviewersRemoveCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectViewCmd)
	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectReviewersCmd)
	projectUpdateCmd.Flags().StringVarP(&projectName, "name", "n", "", "New project name")
	projectUpdateCmd.Flags().StringVarP(&projectDesc, "description", "d", "", "New description")
	projectUpdateCmd.Flags().BoolVar(&projectPrivate, "private", false, "Make the project private")
	projectUpdateCmd.Flags().Bool("public", false, "Make the project public")
	projectCmd.AddCommand(projectUpdateCmd)
	rootCmd.AddCommand(projectCmd)
}

var projectUpdateCmd = &cobra.Command{
	Use:   "update [workspace] <project-key>",
	Short: "Update a project's name/description/visibility",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runProjectUpdate,
}

func runProjectUpdate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := projectNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 1)
	if err != nil {
		return err
	}
	body := map[string]interface{}{}
	if cmd.Flags().Changed("name") {
		body["name"] = projectName
	}
	if cmd.Flags().Changed("description") {
		body["description"] = projectDesc
	}
	if cmd.Flags().Changed("private") {
		body["is_private"] = true
	}
	if cmd.Flags().Changed("public") {
		body["is_private"] = false
	}
	if len(body) == 0 {
		return fmt.Errorf("nothing to update: pass --name, --description, --private, or --public")
	}
	resp, err := client.Request(ctx, http.MethodPut, fmt.Sprintf("/workspaces/%s/projects/%s", ws, rest[0]), body)
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Updated project %s/%s", ws, rest[0])
	return nil
}

var projectReviewersCmd = &cobra.Command{
	Use:   "reviewers",
	Short: "Manage a project's default reviewers",
}

var projectReviewersListCmd = &cobra.Command{
	Use:   "list [workspace] <project-key>",
	Short: "List default reviewers for a project",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runProjectReviewersList,
}

var projectReviewersAddCmd = &cobra.Command{
	Use:   "add [workspace] <project-key> <selected-user>",
	Short: "Add a default reviewer to a project",
	Args:  cobra.RangeArgs(2, 3),
	RunE:  runProjectReviewersAdd,
}

var projectReviewersRemoveCmd = &cobra.Command{
	Use:   "remove [workspace] <project-key> <selected-user>",
	Short: "Remove a default reviewer from a project",
	Args:  cobra.RangeArgs(2, 3),
	RunE:  runProjectReviewersRemove,
}

func runProjectReviewersList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := projectNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 1)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodGet, fmt.Sprintf("/workspaces/%s/projects/%s/default-reviewers", ws, rest[0]), nil)
	if err != nil {
		return fmt.Errorf("failed to list reviewers: %w", err)
	}
	var page struct {
		Values []struct {
			DisplayName string `json:"display_name"`
			UUID        string `json:"uuid"`
			AccountID   string `json:"account_id"`
		} `json:"values"`
	}
	if err := decodeJSON(resp, &page); err != nil {
		return err
	}
	if len(page.Values) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No default reviewers")
		return nil
	}
	headers := []string{"NAME", "UUID"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, u := range page.Values {
		rows = append(rows, []string{u.DisplayName, u.UUID})
		jsonData = append(jsonData, map[string]interface{}{"display_name": u.DisplayName, "uuid": u.UUID, "account_id": u.AccountID})
	}
	return output.Print(headers, rows, jsonData)
}

func runProjectReviewersAdd(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := projectNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 2)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodPut,
		fmt.Sprintf("/workspaces/%s/projects/%s/default-reviewers/%s", ws, rest[0], url.PathEscape(rest[1])), nil)
	if err != nil {
		return fmt.Errorf("failed to add reviewer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Added default reviewer %s to %s/%s", rest[1], ws, rest[0])
	return nil
}

func runProjectReviewersRemove(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := projectNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 2)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodDelete,
		fmt.Sprintf("/workspaces/%s/projects/%s/default-reviewers/%s", ws, rest[0], url.PathEscape(rest[1])), nil)
	if err != nil {
		return fmt.Errorf("failed to remove reviewer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Removed default reviewer %s from %s/%s", rest[1], ws, rest[0])
	return nil
}

func runProjectList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := projectNewAPIClient()
	if err != nil {
		return err
	}
	ws, err := workspaceArg(args)
	if err != nil {
		return err
	}

	var all []project
	url := fmt.Sprintf("/workspaces/%s/projects", ws)
	for url != "" {
		resp, err := client.Request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}
		page, err := decodeProjectPage(resp)
		if err != nil {
			return err
		}
		all = append(all, page.Values...)
		url = page.Next
	}

	if len(all) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No projects found")
		return nil
	}
	headers := []string{"KEY", "NAME", "PRIVATE", "DESCRIPTION"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, p := range all {
		rows = append(rows, []string{p.Key, output.Truncate(p.Name, 30), fmt.Sprintf("%v", p.IsPrivate), output.Truncate(p.Description, 40)})
		jsonData = append(jsonData, map[string]interface{}{"key": p.Key, "name": p.Name, "is_private": p.IsPrivate, "description": p.Description})
	}
	return output.Print(headers, rows, jsonData)
}

func runProjectView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := projectNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 1)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodGet, fmt.Sprintf("/workspaces/%s/projects/%s", ws, rest[0]), nil)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	var p project
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	if output.CurrentFormat == output.FormatJSON {
		return output.JSON(p)
	}
	fmt.Printf("Key:         %s\n", p.Key)
	fmt.Printf("Name:        %s\n", p.Name)
	fmt.Printf("Private:     %v\n", p.IsPrivate)
	fmt.Printf("Description: %s\n", p.Description)
	return nil
}

func runProjectCreate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := projectNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 1)
	if err != nil {
		return err
	}
	key := rest[0]
	name := projectName
	if name == "" {
		name = key
	}
	body := map[string]interface{}{"key": key, "name": name, "is_private": projectPrivate}
	if projectDesc != "" {
		body["description"] = projectDesc
	}

	resp, err := client.Request(ctx, http.MethodPost, fmt.Sprintf("/workspaces/%s/projects", ws), body)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Created project %s/%s", ws, key)
	return nil
}

func runProjectDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := projectNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 1)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodDelete, fmt.Sprintf("/workspaces/%s/projects/%s", ws, rest[0]), nil)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Deleted project %s/%s", ws, rest[0])
	return nil
}

type projectPage struct {
	Values []project `json:"values"`
	Next   string    `json:"next"`
}

func decodeProjectPage(resp *http.Response) (*projectPage, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}
	var p projectPage
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &p, nil
}
