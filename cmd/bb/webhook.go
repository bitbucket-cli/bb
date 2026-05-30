package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/ThomasRedstone/bb/internal/api"
	"github.com/ThomasRedstone/bb/internal/output"
)

var (
	webhookURL         string
	webhookDescription string
	webhookEvents      []string
	webhookActive      bool
	webhookSecret      string
)

// Mockable dependencies for testing
var (
	webhookNewAPIClient = api.NewClient
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Manage webhooks",
	Long:  `List, create, view, and delete repository webhooks.`,
}

var webhookListCmd = &cobra.Command{
	Use:   "list <workspace/repo>",
	Short: "List webhooks",
	Long:  `List all webhooks configured for a repository.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runWebhookList,
}

var webhookCreateCmd = &cobra.Command{
	Use:   "create <workspace/repo>",
	Short: "Create a webhook",
	Long: `Create a new webhook for a repository.

Example:
  bb webhook create myworkspace/myrepo --url https://example.com/webhook --events repo:push,pullrequest:created`,
	Args: cobra.ExactArgs(1),
	RunE: runWebhookCreate,
}

var webhookViewCmd = &cobra.Command{
	Use:   "view <workspace/repo> <webhook-id>",
	Short: "View webhook details",
	Long:  `Display detailed information about a specific webhook.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runWebhookView,
}

var webhookDeleteCmd = &cobra.Command{
	Use:   "delete <workspace/repo> <webhook-id>",
	Short: "Delete a webhook",
	Long:  `Delete a webhook from a repository.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runWebhookDelete,
}

func init() {
	webhookCreateCmd.Flags().StringVarP(&webhookURL, "url", "u", "", "Webhook URL (required)")
	webhookCreateCmd.Flags().StringVarP(&webhookDescription, "description", "d", "", "Webhook description")
	webhookCreateCmd.Flags().StringSliceVarP(&webhookEvents, "events", "e", nil, "Events to trigger webhook (e.g., repo:push,pullrequest:created)")
	webhookCreateCmd.Flags().BoolVarP(&webhookActive, "active", "a", true, "Whether webhook is active")
	webhookCreateCmd.Flags().StringVarP(&webhookSecret, "secret", "s", "", "Webhook secret for signature verification")
	webhookCreateCmd.MarkFlagRequired("url")

	webhookCmd.AddCommand(webhookListCmd)
	webhookCmd.AddCommand(webhookCreateCmd)
	webhookCmd.AddCommand(webhookViewCmd)
	webhookCmd.AddCommand(webhookDeleteCmd)
}

func runWebhookList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := webhookNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugHooks(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
	)
	if err != nil {
		return fmt.Errorf("failed to list webhooks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []struct {
			UUID        string   `json:"uuid"`
			URL         string   `json:"url"`
			Description string   `json:"description"`
			Active      bool     `json:"active"`
			Events      []string `json:"events"`
			CreatedAt   string   `json:"created_at"`
		} `json:"values"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Values) == 0 {
		output.Info("No webhooks found")
		return nil
	}

	headers := []string{"ID", "URL", "ACTIVE", "EVENTS"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, webhook := range result.Values {
		active := "No"
		if webhook.Active {
			active = "Yes"
		}

		events := ""
		if len(webhook.Events) > 0 {
			if len(webhook.Events) > 2 {
				events = fmt.Sprintf("%s, %s +%d more", webhook.Events[0], webhook.Events[1], len(webhook.Events)-2)
			} else {
				for i, e := range webhook.Events {
					if i > 0 {
						events += ", "
					}
					events += e
				}
			}
		}

		rows = append(rows, []string{
			webhook.UUID,
			output.Truncate(webhook.URL, 40),
			active,
			events,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"uuid":        webhook.UUID,
			"url":         webhook.URL,
			"description": webhook.Description,
			"active":      webhook.Active,
			"events":      webhook.Events,
			"created_at":  webhook.CreatedAt,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runWebhookCreate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := webhookNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	webhookRequest := map[string]interface{}{
		"url":    webhookURL,
		"active": webhookActive,
	}

	if webhookDescription != "" {
		webhookRequest["description"] = webhookDescription
	}

	if len(webhookEvents) > 0 {
		webhookRequest["events"] = webhookEvents
	}

	if webhookSecret != "" {
		webhookRequest["secret"] = webhookSecret
	}

	body, err := json.Marshal(webhookRequest)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	withBody := func(ctx context.Context, req *http.Request) error {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/json")
		return nil
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugHooks(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		withBody,
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	var result struct {
		UUID string `json:"uuid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	output.Success("Created webhook %s", result.UUID)
	return nil
}

func runWebhookView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := webhookNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	webhookID := args[1]

	resp, err := client.GetRepositoriesWorkspaceRepoSlugHooksUid(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		webhookID,
	)
	if err != nil {
		return fmt.Errorf("failed to get webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var webhook struct {
		UUID        string   `json:"uuid"`
		URL         string   `json:"url"`
		Description string   `json:"description"`
		Active      bool     `json:"active"`
		Events      []string `json:"events"`
		CreatedAt   string   `json:"created_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&webhook); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if jsonOutput {
		return output.Print(nil, nil, webhook)
	}

	fmt.Printf("UUID:        %s\n", webhook.UUID)
	fmt.Printf("URL:         %s\n", webhook.URL)
	fmt.Printf("Description: %s\n", webhook.Description)
	fmt.Printf("Active:      %t\n", webhook.Active)
	fmt.Printf("Created:     %s\n", webhook.CreatedAt)
	fmt.Printf("Events:\n")
	for _, event := range webhook.Events {
		fmt.Printf("  - %s\n", event)
	}

	return nil
}

func runWebhookDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := webhookNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	webhookID := args[1]

	resp, err := client.DeleteRepositoriesWorkspaceRepoSlugHooksUid(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		webhookID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Deleted webhook %s", webhookID)
	return nil
}
