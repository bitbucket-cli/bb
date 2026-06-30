package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	deployKeyLabel string
	deployKeyFile  string
)

// Mockable dependencies for testing
var deployKeyNewAPIClient = api.NewClient

var deployKeyCmd = &cobra.Command{
	Use:     "deploy-key",
	Aliases: []string{"deploy-keys"},
	Short:   "Manage repository deploy keys (access SSH keys)",
	Long:    `List, add, view, and delete a repository's deploy keys (read-only SSH access keys).`,
}

var deployKeyListCmd = &cobra.Command{
	Use:   "list <workspace/repo>",
	Short: "List deploy keys",
	Long:  `List the deploy keys configured for a repository.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDeployKeyList,
}

var deployKeyAddCmd = &cobra.Command{
	Use:   "add <workspace/repo> [public-key]",
	Short: "Add a deploy key",
	Long: `Add a deploy key to a repository.

Provide the public key inline, or read it from a file with --key-file.

Examples:
  bb deploy-key add myworkspace/myrepo --key-file ~/.ssh/id_ed25519.pub --label ci
  bb deploy-key add myworkspace/myrepo "ssh-ed25519 AAAA... user@host" --label ci`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runDeployKeyAdd,
}

var deployKeyViewCmd = &cobra.Command{
	Use:   "view <workspace/repo> <key-id>",
	Short: "View a deploy key",
	Long:  `Display a deploy key's details.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runDeployKeyView,
}

var deployKeyDeleteCmd = &cobra.Command{
	Use:   "delete <workspace/repo> <key-id>",
	Short: "Delete a deploy key",
	Long:  `Delete a deploy key from a repository.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runDeployKeyDelete,
}

func init() {
	deployKeyAddCmd.Flags().StringVar(&deployKeyLabel, "label", "", "Label for the deploy key")
	deployKeyAddCmd.Flags().StringVar(&deployKeyFile, "key-file", "", "Read the public key from this file")

	deployKeyCmd.AddCommand(deployKeyListCmd)
	deployKeyCmd.AddCommand(deployKeyAddCmd)
	deployKeyCmd.AddCommand(deployKeyViewCmd)
	deployKeyCmd.AddCommand(deployKeyDeleteCmd)
}

func runDeployKeyList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := deployKeyNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugDeployKeys(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	if err != nil {
		return fmt.Errorf("failed to list deploy keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []deployKey `json:"values"`
		Next   string      `json:"next"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	for result.Next != "" {
		next := result.Next
		result.Next = ""
		pageResp, err := client.GetURL(ctx, next)
		if err != nil {
			return fmt.Errorf("failed to fetch next page: %w", err)
		}
		var page struct {
			Values []deployKey `json:"values"`
			Next   string      `json:"next"`
		}
		err = json.NewDecoder(pageResp.Body).Decode(&page)
		pageResp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to parse page: %w", err)
		}
		result.Values = append(result.Values, page.Values...)
		result.Next = page.Next
	}

	if len(result.Values) == 0 {
		output.Info("No deploy keys found")
		return nil
	}

	headers := []string{"ID", "LABEL", "KEY", "CREATED"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, k := range result.Values {
		rows = append(rows, []string{
			fmt.Sprintf("%d", k.ID),
			k.Label,
			output.Truncate(k.Key, 40),
			k.CreatedOn,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"id":         k.ID,
			"label":      k.Label,
			"key":        k.Key,
			"created_on": k.CreatedOn,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runDeployKeyAdd(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := deployKeyNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	key, err := resolveDeployKey(args)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{"key": key}
	if deployKeyLabel != "" {
		payload["label"] = deployKeyLabel
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	withBody := func(ctx context.Context, req *http.Request) error {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/json")
		return nil
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugDeployKeys(ctx, repoCtx.Workspace, repoCtx.RepoSlug, withBody)
	if err != nil {
		return fmt.Errorf("failed to add deploy key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	var result deployKey
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	output.Success("Added deploy key %d", result.ID)
	return nil
}

func runDeployKeyView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := deployKeyNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}
	keyID := args[1]

	resp, err := client.GetRepositoriesWorkspaceRepoSlugDeployKeysKeyId(ctx, repoCtx.Workspace, repoCtx.RepoSlug, keyID)
	if err != nil {
		return fmt.Errorf("failed to get deploy key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var key deployKey
	if err := json.NewDecoder(resp.Body).Decode(&key); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if jsonOutput {
		return output.Print(nil, nil, key)
	}

	fmt.Printf("ID:      %d\n", key.ID)
	fmt.Printf("Label:   %s\n", key.Label)
	fmt.Printf("Created: %s\n", key.CreatedOn)
	fmt.Printf("Key:     %s\n", key.Key)
	return nil
}

func runDeployKeyDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := deployKeyNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}
	keyID := args[1]

	resp, err := client.DeleteRepositoriesWorkspaceRepoSlugDeployKeysKeyId(ctx, repoCtx.Workspace, repoCtx.RepoSlug, keyID)
	if err != nil {
		return fmt.Errorf("failed to delete deploy key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Deleted deploy key %s", keyID)
	return nil
}

// resolveDeployKey returns the public key from the --key-file flag or the
// optional positional argument.
func resolveDeployKey(args []string) (string, error) {
	if deployKeyFile != "" {
		data, err := os.ReadFile(deployKeyFile)
		if err != nil {
			return "", fmt.Errorf("reading key file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	if len(args) >= 2 {
		return strings.TrimSpace(args[1]), nil
	}
	return "", fmt.Errorf("provide a public key argument or --key-file")
}

// deployKey is the subset of a deploy key we render.
type deployKey struct {
	ID        int    `json:"id"`
	Label     string `json:"label"`
	Key       string `json:"key"`
	CreatedOn string `json:"created_on"`
}
