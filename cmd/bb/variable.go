package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var variableSecured bool

// Mockable dependencies for testing
var variableNewAPIClient = api.NewClient

var variableCmd = &cobra.Command{
	Use:     "variable",
	Aliases: []string{"var", "variables"},
	Short:   "Manage repository pipeline variables",
	Long:    `List, set, and delete a repository's pipeline variables.`,
}

var variableListCmd = &cobra.Command{
	Use:   "list <workspace/repo>",
	Short: "List pipeline variables",
	Long:  `List the pipeline variables configured for a repository. Secured values are not returned by the API.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runVariableList,
}

var variableSetCmd = &cobra.Command{
	Use:   "set <workspace/repo> <key> <value>",
	Short: "Set a pipeline variable",
	Long: `Create or update a pipeline variable.

If a variable with the given key already exists it is updated, otherwise it is
created. Use --secured to mark the value as secured (write-only).`,
	Args: cobra.ExactArgs(3),
	RunE: runVariableSet,
}

var variableDeleteCmd = &cobra.Command{
	Use:   "delete <workspace/repo> <key>",
	Short: "Delete a pipeline variable",
	Long:  `Delete a pipeline variable by key.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runVariableDelete,
}

func init() {
	variableSetCmd.Flags().BoolVar(&variableSecured, "secured", false, "Mark the value as secured (write-only)")

	variableCmd.AddCommand(variableListCmd)
	variableCmd.AddCommand(variableSetCmd)
	variableCmd.AddCommand(variableDeleteCmd)
}

// pipelineVariable is a repository pipeline variable.
type pipelineVariable struct {
	UUID    string `json:"uuid,omitempty"`
	Key     string `json:"key"`
	Value   string `json:"value"`
	Secured bool   `json:"secured"`
}

func variablesPath(repo *api.RepoContext) string {
	return fmt.Sprintf("/repositories/%s/%s/pipelines_config/variables/", repo.Workspace, repo.RepoSlug)
}

func runVariableList(cmd *cobra.Command, args []string) error {
	client, err := variableNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	vars, err := fetchVariables(cmd, client, repoCtx)
	if err != nil {
		return err
	}

	if len(vars) == 0 {
		output.Info("No variables found")
		return nil
	}

	headers := []string{"KEY", "SECURED", "VALUE"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, v := range vars {
		value := v.Value
		secured := "No"
		if v.Secured {
			secured = "Yes"
			value = "********"
		}
		rows = append(rows, []string{v.Key, secured, output.Truncate(value, 40)})
		jsonData = append(jsonData, map[string]interface{}{
			"uuid":    v.UUID,
			"key":     v.Key,
			"secured": v.Secured,
			"value":   v.Value,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runVariableSet(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := variableNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}
	key, value := args[1], args[2]

	// Upsert: find an existing variable with this key to decide POST vs PUT.
	existing, err := fetchVariables(cmd, client, repoCtx)
	if err != nil {
		return err
	}
	var existingUUID string
	for _, v := range existing {
		if v.Key == key {
			existingUUID = v.UUID
			break
		}
	}

	payload := pipelineVariable{Key: key, Value: value, Secured: variableSecured}

	var resp *http.Response
	if existingUUID != "" {
		resp, err = client.Request(ctx, http.MethodPut, variablesPath(repoCtx)+existingUUID, payload)
	} else {
		resp, err = client.Request(ctx, http.MethodPost, variablesPath(repoCtx), payload)
	}
	if err != nil {
		return fmt.Errorf("failed to set variable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Set variable %s", key)
	return nil
}

func runVariableDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := variableNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}
	key := args[1]

	existing, err := fetchVariables(cmd, client, repoCtx)
	if err != nil {
		return err
	}
	var uuid string
	for _, v := range existing {
		if v.Key == key {
			uuid = v.UUID
			break
		}
	}
	if uuid == "" {
		return fmt.Errorf("no variable with key %q", key)
	}

	resp, err := client.Request(ctx, http.MethodDelete, variablesPath(repoCtx)+uuid, nil)
	if err != nil {
		return fmt.Errorf("failed to delete variable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Deleted variable %s", key)
	return nil
}

// fetchVariables returns all pipeline variables for a repo, following pagination.
func fetchVariables(cmd *cobra.Command, client *api.Client, repo *api.RepoContext) ([]pipelineVariable, error) {
	ctx := cmdCtx(cmd)

	resp, err := client.Request(ctx, http.MethodGet, variablesPath(repo), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list variables: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []pipelineVariable `json:"values"`
		Next   string             `json:"next"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	all := result.Values

	for result.Next != "" {
		next := result.Next
		result.Next = ""
		pageResp, err := client.GetURL(ctx, next)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch next page: %w", err)
		}
		var page struct {
			Values []pipelineVariable `json:"values"`
			Next   string             `json:"next"`
		}
		err = json.NewDecoder(pageResp.Body).Decode(&page)
		pageResp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to parse page: %w", err)
		}
		all = append(all, page.Values...)
		result.Next = page.Next
	}

	return all, nil
}
