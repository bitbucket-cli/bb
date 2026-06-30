package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	commitBranch string
	commitLimit  int
)

// Mockable dependencies for testing
var commitNewAPIClient = api.NewClient

var commitCmd = &cobra.Command{
	Use:     "commit",
	Aliases: []string{"commits"},
	Short:   "View repository commits",
	Long:    `List and view a repository's commits.`,
}

var commitListCmd = &cobra.Command{
	Use:   "list <workspace/repo>",
	Short: "List commits",
	Long: `List a repository's commits, most recent first.

Use --branch to list commits reachable from a specific branch, tag, or hash.`,
	Args: cobra.ExactArgs(1),
	RunE: runCommitList,
}

var commitViewCmd = &cobra.Command{
	Use:   "view <workspace/repo> <hash>",
	Short: "View a commit",
	Long:  `Display a single commit's details.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runCommitView,
}

func init() {
	commitListCmd.Flags().StringVar(&commitBranch, "branch", "", "List commits from this branch, tag, or hash")
	commitListCmd.Flags().IntVar(&commitLimit, "limit", 25, "Maximum number of commits to show")

	commitCmd.AddCommand(commitListCmd)
	commitCmd.AddCommand(commitViewCmd)
}

func runCommitList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := commitNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	var resp *http.Response
	if commitBranch != "" {
		resp, err = client.GetRepositoriesWorkspaceRepoSlugCommitsRevision(ctx, repoCtx.Workspace, repoCtx.RepoSlug, commitBranch)
	} else {
		resp, err = client.GetRepositoriesWorkspaceRepoSlugCommits(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	}
	if err != nil {
		return fmt.Errorf("failed to list commits: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []commitSummary `json:"values"`
		Next   string          `json:"next"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Follow pagination only until we have enough for the limit.
	for result.Next != "" && len(result.Values) < commitLimit {
		next := result.Next
		result.Next = ""
		pageResp, err := client.GetURL(ctx, next)
		if err != nil {
			return fmt.Errorf("failed to fetch next page: %w", err)
		}
		var page struct {
			Values []commitSummary `json:"values"`
			Next   string          `json:"next"`
		}
		err = json.NewDecoder(pageResp.Body).Decode(&page)
		pageResp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to parse page: %w", err)
		}
		result.Values = append(result.Values, page.Values...)
		result.Next = page.Next
	}

	if len(result.Values) > commitLimit {
		result.Values = result.Values[:commitLimit]
	}

	if len(result.Values) == 0 {
		output.Info("No commits found")
		return nil
	}

	headers := []string{"HASH", "AUTHOR", "DATE", "MESSAGE"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, c := range result.Values {
		rows = append(rows, []string{
			shortHash(c.Hash),
			c.authorName(),
			c.Date,
			output.Truncate(firstLine(c.Message), 50),
		})
		jsonData = append(jsonData, map[string]interface{}{
			"hash":    c.Hash,
			"author":  c.authorName(),
			"date":    c.Date,
			"message": c.Message,
		})
	}
	return output.Print(headers, rows, jsonData)
}

func runCommitView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := commitNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugCommitCommit(ctx, repoCtx.Workspace, repoCtx.RepoSlug, args[1])
	if err != nil {
		return fmt.Errorf("failed to get commit: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var c commitSummary
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if jsonOutput {
		return output.Print(nil, nil, c)
	}
	fmt.Printf("Hash:    %s\n", c.Hash)
	fmt.Printf("Author:  %s\n", c.authorName())
	fmt.Printf("Date:    %s\n", c.Date)
	fmt.Printf("Message:\n%s\n", c.Message)
	return nil
}

// commitSummary is the subset of a commit we render.
type commitSummary struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Date    string `json:"date"`
	Author  struct {
		Raw  string `json:"raw"`
		User *struct {
			DisplayName string `json:"display_name"`
		} `json:"user"`
	} `json:"author"`
}

func (c commitSummary) authorName() string {
	if c.Author.User != nil && c.Author.User.DisplayName != "" {
		return c.Author.User.DisplayName
	}
	return c.Author.Raw
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
