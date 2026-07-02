package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
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

	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectViewCmd)
	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	rootCmd.AddCommand(projectCmd)
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

	if len(all) == 0 {
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
	if jsonOutput {
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
