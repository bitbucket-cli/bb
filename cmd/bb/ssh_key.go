package main

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	sshKeyValue string
	sshKeyLabel string
)

// Mockable dependency for testing.
var sshKeyNewAPIClient = api.NewClient

var sshKeyCmd = &cobra.Command{
	Use:     "ssh-key",
	Aliases: []string{"ssh-keys"},
	Short:   "Manage your account SSH keys",
	Long:    `List, add, and delete SSH keys on your Bitbucket account (distinct from repository deploy keys).`,
}

var sshKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your account SSH keys",
	Args:  cobra.NoArgs,
	RunE:  runSSHKeyList,
}

var sshKeyAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add an SSH key to your account",
	Args:  cobra.NoArgs,
	RunE:  runSSHKeyAdd,
}

var sshKeyDeleteCmd = &cobra.Command{
	Use:   "delete <key-uuid>",
	Short: "Delete an account SSH key",
	Args:  cobra.ExactArgs(1),
	RunE:  runSSHKeyDelete,
}

func init() {
	sshKeyAddCmd.Flags().StringVar(&sshKeyValue, "key", "", "The public key (e.g. 'ssh-ed25519 AAAA...') (required)")
	sshKeyAddCmd.Flags().StringVar(&sshKeyLabel, "label", "", "A label for the key")
	sshKeyAddCmd.MarkFlagRequired("key")

	sshKeyCmd.AddCommand(sshKeyListCmd, sshKeyAddCmd, sshKeyDeleteCmd)
	rootCmd.AddCommand(sshKeyCmd)
}

// meKeysPath returns /users/{current-user}/ssh-keys with the user id path-escaped.
func meKeysPath(cmd *cobra.Command, client *api.Client) (string, error) {
	me, err := currentUserUUID(cmdCtx(cmd), client)
	if err != nil {
		return "", fmt.Errorf("could not determine current user: %w", err)
	}
	return "/users/" + url.PathEscape(me) + "/ssh-keys", nil
}

func runSSHKeyList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := sshKeyNewAPIClient()
	if err != nil {
		return err
	}
	path, err := meKeysPath(cmd, client)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("failed to list SSH keys: %w", err)
	}
	type listItem struct {
		UUID    string `json:"uuid"`
		Key     string `json:"key"`
		Label   string `json:"label"`
		Comment string `json:"comment"`
	}
	defer resp.Body.Close()
	items, err := fetchAllPages[listItem](ctx, client, resp)
	if err != nil {
		return err
	}
	if len(items) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No SSH keys found")
		return nil
	}
	headers := []string{"LABEL", "KEY", "UUID"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, k := range items {
		rows = append(rows, []string{k.Label, output.Truncate(k.Key, 40), k.UUID})
		jsonData = append(jsonData, map[string]interface{}{"label": k.Label, "key": k.Key, "comment": k.Comment, "uuid": k.UUID})
	}
	return output.Print(headers, rows, jsonData)
}

func runSSHKeyAdd(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := sshKeyNewAPIClient()
	if err != nil {
		return err
	}
	path, err := meKeysPath(cmd, client)
	if err != nil {
		return err
	}
	body := map[string]interface{}{"key": sshKeyValue}
	if sshKeyLabel != "" {
		body["label"] = sshKeyLabel
	}
	resp, err := client.Request(ctx, http.MethodPost, path, body)
	if err != nil {
		return fmt.Errorf("failed to add SSH key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Added SSH key")
	return nil
}

func runSSHKeyDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := sshKeyNewAPIClient()
	if err != nil {
		return err
	}
	path, err := meKeysPath(cmd, client)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodDelete, path+"/"+url.PathEscape(args[0]), nil)
	if err != nil {
		return fmt.Errorf("failed to delete SSH key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Deleted SSH key %s", args[0])
	return nil
}
