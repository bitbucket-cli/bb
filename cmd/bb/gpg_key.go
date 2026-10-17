package main

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var gpgKeyValue string

// Mockable dependency for testing.
var gpgKeyNewAPIClient = api.NewClient

var gpgKeyCmd = &cobra.Command{
	Use:     "gpg-key",
	Aliases: []string{"gpg-keys"},
	Short:   "Manage your account GPG keys",
	Long:    `List, add, and delete GPG keys on your Bitbucket account.`,
}

var gpgKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your account GPG keys",
	Args:  cobra.NoArgs,
	RunE:  runGPGKeyList,
}

var gpgKeyAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a GPG key to your account",
	Args:  cobra.NoArgs,
	RunE:  runGPGKeyAdd,
}

var gpgKeyDeleteCmd = &cobra.Command{
	Use:   "delete <fingerprint>",
	Short: "Delete an account GPG key",
	Args:  cobra.ExactArgs(1),
	RunE:  runGPGKeyDelete,
}

func init() {
	gpgKeyAddCmd.Flags().StringVar(&gpgKeyValue, "key", "", "The ASCII-armored public GPG key (required)")
	gpgKeyAddCmd.MarkFlagRequired("key")

	gpgKeyCmd.AddCommand(gpgKeyListCmd, gpgKeyAddCmd, gpgKeyDeleteCmd)
	rootCmd.AddCommand(gpgKeyCmd)
}

func meGPGPath(cmd *cobra.Command, client *api.Client) (string, error) {
	me, err := currentUserUUID(cmdCtx(cmd), client)
	if err != nil {
		return "", fmt.Errorf("could not determine current user: %w", err)
	}
	return "/users/" + url.PathEscape(me) + "/gpg-keys", nil
}

func runGPGKeyList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := gpgKeyNewAPIClient()
	if err != nil {
		return err
	}
	path, err := meGPGPath(cmd, client)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("failed to list GPG keys: %w", err)
	}
	type listItem struct {
		Fingerprint string `json:"fingerprint"`
		Name        string `json:"name"`
	}
	defer resp.Body.Close()
	items, err := fetchAllPages[listItem](ctx, client, resp)
	if err != nil {
		return err
	}
	if len(items) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No GPG keys found")
		return nil
	}
	headers := []string{"FINGERPRINT", "NAME"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, k := range items {
		rows = append(rows, []string{k.Fingerprint, k.Name})
		jsonData = append(jsonData, map[string]interface{}{"fingerprint": k.Fingerprint, "name": k.Name})
	}
	return output.Print(headers, rows, jsonData)
}

func runGPGKeyAdd(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := gpgKeyNewAPIClient()
	if err != nil {
		return err
	}
	path, err := meGPGPath(cmd, client)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodPost, path, map[string]interface{}{"key": gpgKeyValue})
	if err != nil {
		return fmt.Errorf("failed to add GPG key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Added GPG key")
	return nil
}

func runGPGKeyDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := gpgKeyNewAPIClient()
	if err != nil {
		return err
	}
	path, err := meGPGPath(cmd, client)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodDelete, path+"/"+url.PathEscape(args[0]), nil)
	if err != nil {
		return fmt.Errorf("failed to delete GPG key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Deleted GPG key %s", args[0])
	return nil
}
