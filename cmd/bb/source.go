package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var sourceRef string

// Mockable dependencies for testing
var sourceNewAPIClient = api.NewClient

var sourceCmd = &cobra.Command{
	Use:     "source",
	Aliases: []string{"src"},
	Short:   "Browse repository source",
	Long: `List directory contents and print file contents from a repository.

Best-effort: the Bitbucket src endpoint returns a JSON listing for a directory
and raw bytes for a file. The ref defaults to the repository's main branch.`,
}

var sourceListCmd = &cobra.Command{
	Use:   "list [workspace/repo] [path]",
	Short: "List a directory's contents",
	Args:  cobra.RangeArgs(0, 2),
	RunE:  runSourceList,
}

var sourceViewCmd = &cobra.Command{
	Use:   "view [workspace/repo] <path>",
	Short: "Print a file's contents",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runSourceView,
}

func init() {
	sourceListCmd.Flags().StringVar(&sourceRef, "ref", "", "Branch, tag, or commit (defaults to the main branch)")
	sourceViewCmd.Flags().StringVar(&sourceRef, "ref", "", "Branch, tag, or commit (defaults to the main branch)")

	sourceCmd.AddCommand(sourceListCmd)
	sourceCmd.AddCommand(sourceViewCmd)
}

// resolveSourceRef returns --ref, or the repository's main branch when unset.
func resolveSourceRef(cmd *cobra.Command, client *api.Client, repoCtx *api.RepoContext) (string, error) {
	if sourceRef != "" {
		return sourceRef, nil
	}
	resp, err := client.GetRepositoriesWorkspaceRepoSlug(cmdCtx(cmd), repoCtx.Workspace, repoCtx.RepoSlug)
	if err != nil {
		return "", fmt.Errorf("failed to resolve default branch: %w", err)
	}
	defer resp.Body.Close()
	var repo struct {
		Mainbranch struct {
			Name string `json:"name"`
		} `json:"mainbranch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return "", err
	}
	if repo.Mainbranch.Name == "" {
		return "main", nil
	}
	return repo.Mainbranch.Name, nil
}

func runSourceList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := sourceNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndArgs(args, 0)
	if err != nil {
		return err
	}
	ref, err := resolveSourceRef(cmd, client, repoCtx)
	if err != nil {
		return err
	}
	path := ""
	if len(rest) > 0 {
		path = rest[0]
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugSrcCommitPath(ctx, repoCtx.Workspace, repoCtx.RepoSlug, ref, path, nil)
	if err != nil {
		return fmt.Errorf("failed to list source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	entries, err := fetchAllPages[sourceEntry](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse listing (is %q a file? use `source view`): %w", path, err)
	}

	if len(entries) == 0 {
		output.Info("No entries found")
		return nil
	}

	headers := []string{"TYPE", "SIZE", "PATH"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, e := range entries {
		kind := "file"
		if e.Type == "commit_directory" {
			kind = "dir"
		}
		rows = append(rows, []string{kind, humanSize(e.Size), e.Path})
		jsonData = append(jsonData, map[string]interface{}{"type": e.Type, "size": e.Size, "path": e.Path})
	}
	return output.Print(headers, rows, jsonData)
}

func runSourceView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := sourceNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}
	ref, err := resolveSourceRef(cmd, client, repoCtx)
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugSrcCommitPath(ctx, repoCtx.Workspace, repoCtx.RepoSlug, ref, rest[0], nil)
	if err != nil {
		return fmt.Errorf("failed to fetch file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	_, err = io.Copy(os.Stdout, resp.Body)
	return err
}

// sourceEntry is a directory entry in a src listing.
type sourceEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}
