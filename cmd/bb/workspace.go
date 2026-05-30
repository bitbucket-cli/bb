package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ThomasRedstone/bb/generated/bitbucket"
	"github.com/ThomasRedstone/bb/internal/api"
	"github.com/ThomasRedstone/bb/internal/output"
)

// Mockable dependencies for testing
var (
	workspaceNewAPIClient = api.NewClient
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Aliases: []string{"workspaces", "ws"},
	Short:   "Manage workspaces",
	Long:    `List workspaces and view workspace details and members.`,
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspaces",
	Long:  `List all workspaces you have access to.`,
	Args:  cobra.NoArgs,
	RunE:  runWorkspaceList,
}

var workspaceViewCmd = &cobra.Command{
	Use:   "view <workspace>",
	Short: "View workspace details",
	Long:  `Display detailed information about a workspace.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkspaceView,
}

var workspaceMembersCmd = &cobra.Command{
	Use:   "members <workspace>",
	Short: "List workspace members",
	Long:  `List all members of a workspace.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkspaceMembers,
}

var workspaceProjectsCmd = &cobra.Command{
	Use:   "projects <workspace>",
	Short: "List workspace projects",
	Long:  `List all projects in a workspace.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkspaceProjects,
}

func init() {
	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceViewCmd)
	workspaceCmd.AddCommand(workspaceMembersCmd)
	workspaceCmd.AddCommand(workspaceProjectsCmd)
}

func runWorkspaceList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := workspaceNewAPIClient()
	if err != nil {
		return err
	}

	params := &bitbucket.GetWorkspacesParams{}

	resp, err := client.GetWorkspaces(
		ctx,
		params,
	)
	if err != nil {
		return fmt.Errorf("failed to list workspaces: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
			Slug string `json:"slug"`
			Type string `json:"type"`
		} `json:"values"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Values) == 0 {
		output.Info("No workspaces found")
		return nil
	}

	headers := []string{"SLUG", "NAME", "TYPE"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, ws := range result.Values {
		rows = append(rows, []string{
			ws.Slug,
			ws.Name,
			ws.Type,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"uuid": ws.UUID,
			"slug": ws.Slug,
			"name": ws.Name,
			"type": ws.Type,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runWorkspaceView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := workspaceNewAPIClient()
	if err != nil {
		return err
	}

	workspace := args[0]

	resp, err := client.GetWorkspacesWorkspace(
		ctx,
		workspace,
	)
	if err != nil {
		return fmt.Errorf("failed to get workspace: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var ws struct {
		UUID      string `json:"uuid"`
		Name      string `json:"name"`
		Slug      string `json:"slug"`
		Type      string `json:"type"`
		IsPrivate bool   `json:"is_private"`
		CreatedOn string `json:"created_on"`
		Links     *struct {
			Html *struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ws); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if jsonOutput {
		return output.Print(nil, nil, ws)
	}

	fmt.Printf("Name:       %s\n", ws.Name)
	fmt.Printf("Slug:       %s\n", ws.Slug)
	fmt.Printf("UUID:       %s\n", ws.UUID)
	fmt.Printf("Type:       %s\n", ws.Type)
	fmt.Printf("Private:    %t\n", ws.IsPrivate)
	fmt.Printf("Created:    %s\n", ws.CreatedOn)
	if ws.Links != nil && ws.Links.Html != nil {
		fmt.Printf("URL:        %s\n", ws.Links.Html.Href)
	}

	return nil
}

func runWorkspaceMembers(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := workspaceNewAPIClient()
	if err != nil {
		return err
	}

	workspace := args[0]

	resp, err := client.GetWorkspacesWorkspaceMembers(
		ctx,
		workspace,
	)
	if err != nil {
		return fmt.Errorf("failed to list workspace members: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []struct {
			User *struct {
				DisplayName string `json:"display_name"`
				Nickname    string `json:"nickname"`
				AccountID   string `json:"account_id"`
				UUID        string `json:"uuid"`
			} `json:"user"`
		} `json:"values"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Values) == 0 {
		output.Info("No members found")
		return nil
	}

	headers := []string{"NAME", "NICKNAME", "ACCOUNT ID"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, member := range result.Values {
		if member.User == nil {
			continue
		}

		rows = append(rows, []string{
			member.User.DisplayName,
			member.User.Nickname,
			member.User.AccountID,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"uuid":         member.User.UUID,
			"display_name": member.User.DisplayName,
			"nickname":     member.User.Nickname,
			"account_id":   member.User.AccountID,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runWorkspaceProjects(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := workspaceNewAPIClient()
	if err != nil {
		return err
	}

	workspace := args[0]

	resp, err := client.GetWorkspacesWorkspaceProjects(
		ctx,
		workspace,
	)
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []struct {
			Key         string `json:"key"`
			Name        string `json:"name"`
			Description string `json:"description"`
			IsPrivate   bool   `json:"is_private"`
			UUID        string `json:"uuid"`
		} `json:"values"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Values) == 0 {
		output.Info("No projects found")
		return nil
	}

	headers := []string{"KEY", "NAME", "PRIVATE", "DESCRIPTION"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, project := range result.Values {
		private := "No"
		if project.IsPrivate {
			private = "Yes"
		}

		rows = append(rows, []string{
			project.Key,
			project.Name,
			private,
			output.Truncate(project.Description, 40),
		})
		jsonData = append(jsonData, map[string]interface{}{
			"uuid":        project.UUID,
			"key":         project.Key,
			"name":        project.Name,
			"is_private":  project.IsPrivate,
			"description": project.Description,
		})
	}

	return output.Print(headers, rows, jsonData)
}
