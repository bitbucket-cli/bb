package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ThomasRedstone/bb/internal/output"
	"github.com/spf13/cobra"
)

var (
	wsHookURL    string
	wsHookEvents []string
	wsHookDesc   string
	wsVarSecured bool
)

// ---- workspace webhooks ----

var workspaceHookCmd = &cobra.Command{
	Use:     "webhook",
	Aliases: []string{"hook", "webhooks"},
	Short:   "Manage workspace-level webhooks",
}

var workspaceHookListCmd = &cobra.Command{
	Use:   "list [workspace]",
	Short: "List workspace webhooks",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWorkspaceHookList,
}

var workspaceHookCreateCmd = &cobra.Command{
	Use:   "create [workspace]",
	Short: "Create a workspace webhook",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWorkspaceHookCreate,
}

var workspaceHookDeleteCmd = &cobra.Command{
	Use:   "delete [workspace] <uuid>",
	Short: "Delete a workspace webhook",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runWorkspaceHookDelete,
}

// ---- workspace pipeline variables ----

var workspaceVarCmd = &cobra.Command{
	Use:     "variable",
	Aliases: []string{"var", "variables"},
	Short:   "Manage workspace-level pipeline variables",
}

var workspaceVarListCmd = &cobra.Command{
	Use:   "list [workspace]",
	Short: "List workspace pipeline variables",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWorkspaceVarList,
}

var workspaceVarSetCmd = &cobra.Command{
	Use:   "set [workspace] <key> <value>",
	Short: "Create a workspace pipeline variable",
	Args:  cobra.RangeArgs(2, 3),
	RunE:  runWorkspaceVarSet,
}

var workspaceVarDeleteCmd = &cobra.Command{
	Use:   "delete [workspace] <uuid>",
	Short: "Delete a workspace pipeline variable",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runWorkspaceVarDelete,
}

func init() {
	workspaceHookCreateCmd.Flags().StringVar(&wsHookURL, "url", "", "Webhook payload URL (required)")
	workspaceHookCreateCmd.Flags().StringSliceVar(&wsHookEvents, "events", []string{"repo:push"}, "Event types (comma-separated)")
	workspaceHookCreateCmd.Flags().StringVar(&wsHookDesc, "description", "", "Webhook description")
	workspaceHookCreateCmd.MarkFlagRequired("url")
	workspaceHookCmd.AddCommand(workspaceHookListCmd, workspaceHookCreateCmd, workspaceHookDeleteCmd)

	workspaceVarSetCmd.Flags().BoolVar(&wsVarSecured, "secured", false, "Mark the variable as secured (value hidden)")
	workspaceVarCmd.AddCommand(workspaceVarListCmd, workspaceVarSetCmd, workspaceVarDeleteCmd)

	workspaceCmd.AddCommand(workspaceHookCmd, workspaceVarCmd)
}

func runWorkspaceHookList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := workspaceNewAPIClient()
	if err != nil {
		return err
	}
	ws, err := workspaceArg(args)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodGet, fmt.Sprintf("/workspaces/%s/hooks", ws), nil)
	if err != nil {
		return fmt.Errorf("failed to list webhooks: %w", err)
	}
	var page struct {
		Values []struct {
			UUID        string   `json:"uuid"`
			URL         string   `json:"url"`
			Description string   `json:"description"`
			Active      bool     `json:"active"`
			Events      []string `json:"events"`
		} `json:"values"`
	}
	if err := decodeJSON(resp, &page); err != nil {
		return err
	}
	if len(page.Values) == 0 {
		output.Info("No webhooks found")
		return nil
	}
	headers := []string{"UUID", "URL", "ACTIVE", "EVENTS"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, h := range page.Values {
		rows = append(rows, []string{h.UUID, output.Truncate(h.URL, 40), fmt.Sprintf("%v", h.Active), fmt.Sprintf("%d", len(h.Events))})
		jsonData = append(jsonData, map[string]interface{}{"uuid": h.UUID, "url": h.URL, "active": h.Active, "events": h.Events, "description": h.Description})
	}
	return output.Print(headers, rows, jsonData)
}

func runWorkspaceHookCreate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := workspaceNewAPIClient()
	if err != nil {
		return err
	}
	ws, err := workspaceArg(args)
	if err != nil {
		return err
	}
	body := map[string]interface{}{"url": wsHookURL, "active": true, "events": wsHookEvents}
	if wsHookDesc != "" {
		body["description"] = wsHookDesc
	}
	resp, err := client.Request(ctx, http.MethodPost, fmt.Sprintf("/workspaces/%s/hooks", ws), body)
	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Created workspace webhook")
	return nil
}

func runWorkspaceHookDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := workspaceNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 1)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodDelete, fmt.Sprintf("/workspaces/%s/hooks/%s", ws, rest[0]), nil)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Deleted workspace webhook %s", rest[0])
	return nil
}

func runWorkspaceVarList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := workspaceNewAPIClient()
	if err != nil {
		return err
	}
	ws, err := workspaceArg(args)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodGet, fmt.Sprintf("/workspaces/%s/pipelines-config/variables/", ws), nil)
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

func runWorkspaceVarSet(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := workspaceNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 2)
	if err != nil {
		return err
	}
	body := map[string]interface{}{"key": rest[0], "value": rest[1], "secured": wsVarSecured}
	resp, err := client.Request(ctx, http.MethodPost, fmt.Sprintf("/workspaces/%s/pipelines-config/variables/", ws), body)
	if err != nil {
		return fmt.Errorf("failed to set variable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Set workspace variable %s", rest[0])
	return nil
}

func runWorkspaceVarDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := workspaceNewAPIClient()
	if err != nil {
		return err
	}
	ws, rest, err := workspaceAndArgs(args, 1)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodDelete, fmt.Sprintf("/workspaces/%s/pipelines-config/variables/%s", ws, rest[0]), nil)
	if err != nil {
		return fmt.Errorf("failed to delete variable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Deleted workspace variable %s", rest[0])
	return nil
}

// decodeJSON reads a 2xx JSON body into v, or returns an API error.
func decodeJSON(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	return nil
}
