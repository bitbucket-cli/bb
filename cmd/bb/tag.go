package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bitbucket-cli/bb/generated/bitbucket"
	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	tagTarget  string
	tagMessage string
)

// Mockable dependencies for testing
var (
	tagNewAPIClient = api.NewClient
)

var tagCmd = &cobra.Command{
	Use:     "tag",
	Aliases: []string{"tags"},
	Short:   "Manage tags",
	Long:    `List, create, view, and delete repository tags.`,
}

var tagListCmd = &cobra.Command{
	Use:   "list [workspace/repo]",
	Short: "List tags",
	Long:  `List all tags in a repository.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runTagList,
}

var tagCreateCmd = &cobra.Command{
	Use:   "create [workspace/repo] <tag-name>",
	Short: "Create a tag",
	Long: `Create a new tag in a repository.

Example:
  bb tag create myworkspace/myrepo v1.0.0 --target main
  bb tag create myworkspace/myrepo v1.0.0 --target abc123 --message "Release 1.0.0"`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTagCreate,
}

var tagViewCmd = &cobra.Command{
	Use:   "view [workspace/repo] <tag-name>",
	Short: "View tag details",
	Long:  `Display detailed information about a specific tag.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runTagView,
}

var tagDeleteCmd = &cobra.Command{
	Use:   "delete [workspace/repo] <tag-name>",
	Short: "Delete a tag",
	Long:  `Delete a tag from a repository.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runTagDelete,
}

func init() {
	tagCreateCmd.Flags().StringVarP(&tagTarget, "target", "t", "", "Target commit or branch (required)")
	tagCreateCmd.Flags().StringVarP(&tagMessage, "message", "m", "", "Tag message (for annotated tags)")
	tagCreateCmd.MarkFlagRequired("target")

	tagCmd.AddCommand(tagListCmd)
	tagCmd.AddCommand(tagCreateCmd)
	tagCmd.AddCommand(tagViewCmd)
	tagCmd.AddCommand(tagDeleteCmd)
}

func runTagList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := tagNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	params := &bitbucket.GetRepositoriesWorkspaceRepoSlugRefsTagsParams{}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugRefsTags(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		params,
	)
	if err != nil {
		return fmt.Errorf("failed to list tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	type tagRow struct {
		Name   string `json:"name"`
		Target *struct {
			Hash    string `json:"hash"`
			Date    string `json:"date"`
			Message string `json:"message"`
		} `json:"target"`
	}

	tags, err := fetchAllPages[tagRow](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(tags) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No tags found")
		return nil
	}

	headers := []string{"NAME", "COMMIT", "DATE", "MESSAGE"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, tag := range tags {
		hash := ""
		date := ""
		message := ""
		if tag.Target != nil {
			if len(tag.Target.Hash) >= 12 {
				hash = tag.Target.Hash[:12]
			} else {
				hash = tag.Target.Hash
			}
			if len(tag.Target.Date) >= 10 {
				date = tag.Target.Date[:10]
			}
			message = output.Truncate(tag.Target.Message, 40)
		}

		rows = append(rows, []string{
			tag.Name,
			hash,
			date,
			message,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"name":    tag.Name,
			"commit":  hash,
			"date":    date,
			"message": message,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runTagCreate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := tagNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	tagName := rest[0]

	tagReq := map[string]interface{}{
		"name": tagName,
		"target": map[string]interface{}{
			"hash": tagTarget,
		},
	}

	if tagMessage != "" {
		tagReq["message"] = tagMessage
	}

	body, err := json.Marshal(tagReq)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	withBody := func(ctx context.Context, req *http.Request) error {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/json")
		return nil
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugRefsTagsWithBody(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		"application/json",
		bytes.NewReader(body),
		withBody,
	)
	if err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Created tag '%s' at %s", tagName, tagTarget)
	return nil
}

func runTagView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := tagNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	tagName := rest[0]

	resp, err := client.GetRepositoriesWorkspaceRepoSlugRefsTagsName(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		tagName,
	)
	if err != nil {
		return fmt.Errorf("failed to get tag: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var tag struct {
		Name   string `json:"name"`
		Target *struct {
			Hash    string `json:"hash"`
			Date    string `json:"date"`
			Message string `json:"message"`
			Author  *struct {
				Raw string `json:"raw"`
			} `json:"author"`
		} `json:"target"`
		Message string `json:"message"`
		Tagger  *struct {
			Raw string `json:"raw"`
		} `json:"tagger"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tag); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if output.CurrentFormat == output.FormatJSON {
		return output.Print(nil, nil, tag)
	}

	fmt.Printf("Tag:     %s\n", tag.Name)
	if tag.Target != nil {
		fmt.Printf("Commit:  %s\n", tag.Target.Hash)
		fmt.Printf("Date:    %s\n", tag.Target.Date)
		if tag.Target.Author != nil {
			fmt.Printf("Author:  %s\n", tag.Target.Author.Raw)
		}
		if tag.Target.Message != "" {
			fmt.Printf("Message: %s\n", tag.Target.Message)
		}
	}
	if tag.Message != "" {
		fmt.Printf("\nTag Message:\n%s\n", tag.Message)
	}
	if tag.Tagger != nil {
		fmt.Printf("Tagger:  %s\n", tag.Tagger.Raw)
	}

	return nil
}

func runTagDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := tagNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	tagName := rest[0]

	// Use DeleteRepositoriesWorkspaceRepoSlugRefsTagsName
	resp, err := client.DeleteRepositoriesWorkspaceRepoSlugRefsTagsName(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		tagName,
	)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Deleted tag '%s'", tagName)
	return nil
}
