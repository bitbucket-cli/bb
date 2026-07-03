package main

import (
	"fmt"
	"net/http"

	"github.com/ThomasRedstone/bb/internal/output"
	"github.com/spf13/cobra"
)

var deployVarSecured bool

var deployVarCmd = &cobra.Command{
	Use:     "variable",
	Aliases: []string{"var", "variables"},
	Short:   "Manage deployment environment variables",
}

var deployVarListCmd = &cobra.Command{
	Use:   "list [workspace/repo] <env-uuid>",
	Short: "List variables for a deployment environment",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runDeployVarList,
}

var deployVarSetCmd = &cobra.Command{
	Use:   "set [workspace/repo] <env-uuid> <key> <value>",
	Short: "Create a deployment environment variable",
	Args:  cobra.RangeArgs(3, 4),
	RunE:  runDeployVarSet,
}

var deployVarDeleteCmd = &cobra.Command{
	Use:   "delete [workspace/repo] <env-uuid> <var-uuid>",
	Short: "Delete a deployment environment variable",
	Args:  cobra.RangeArgs(2, 3),
	RunE:  runDeployVarDelete,
}

func init() {
	deployVarSetCmd.Flags().BoolVar(&deployVarSecured, "secured", false, "Mark the variable as secured (value hidden)")
	deployVarCmd.AddCommand(deployVarListCmd, deployVarSetCmd, deployVarDeleteCmd)
	deployEnvCmd.AddCommand(deployVarCmd)
}

func deployVarsPath(ws, repo, env string) string {
	return fmt.Sprintf("/repositories/%s/%s/deployments_config/environments/%s/variables", ws, repo, env)
}

func runDeployVarList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := deployNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodGet, deployVarsPath(repoCtx.Workspace, repoCtx.RepoSlug, rest[0]), nil)
	if err != nil {
		return fmt.Errorf("failed to list variables: %w", err)
	}
	var page struct {
		Values []struct {
			UUID    string `json:"uuid"`
			Key     string `json:"key"`
			Value   string `json:"value"`
			Secured bool   `json:"secured"`
		} `json:"values"`
	}
	if err := decodeJSON(resp, &page); err != nil {
		return err
	}
	if len(page.Values) == 0 {
		output.Info("No variables found")
		return nil
	}
	headers := []string{"KEY", "VALUE", "SECURED", "UUID"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, v := range page.Values {
		val := v.Value
		if v.Secured {
			val = "********"
		}
		rows = append(rows, []string{v.Key, val, fmt.Sprintf("%v", v.Secured), v.UUID})
		jsonData = append(jsonData, map[string]interface{}{"key": v.Key, "secured": v.Secured, "uuid": v.UUID})
	}
	return output.Print(headers, rows, jsonData)
}

func runDeployVarSet(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := deployNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndArgs(args, 3)
	if err != nil {
		return err
	}
	body := map[string]interface{}{"key": rest[1], "value": rest[2], "secured": deployVarSecured}
	resp, err := client.Request(ctx, http.MethodPost, deployVarsPath(repoCtx.Workspace, repoCtx.RepoSlug, rest[0]), body)
	if err != nil {
		return fmt.Errorf("failed to set variable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Set deployment variable %s", rest[1])
	return nil
}

func runDeployVarDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := deployNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndArgs(args, 2)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodDelete, deployVarsPath(repoCtx.Workspace, repoCtx.RepoSlug, rest[0])+"/"+rest[1], nil)
	if err != nil {
		return fmt.Errorf("failed to delete variable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Deleted deployment variable %s", rest[1])
	return nil
}
