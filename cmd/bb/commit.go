package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
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
	Use:   "list [workspace/repo]",
	Short: "List commits",
	Long: `List a repository's commits, most recent first.

Use --branch to list commits reachable from a specific branch, tag, or hash.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCommitList,
}

var commitViewCmd = &cobra.Command{
	Use:   "view [workspace/repo] <hash>",
	Short: "View a commit",
	Long:  `Display a single commit's details.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runCommitView,
}

func init() {
	commitListCmd.Flags().StringVar(&commitBranch, "branch", "", "List commits from this branch, tag, or hash")
	commitListCmd.Flags().IntVar(&commitLimit, "limit", 25, "Maximum number of commits to show")
	commitActivityCmd.Flags().StringVar(&commitBranch, "branch", "", "Count commits from this branch, tag, or hash")
	commitActivityCmd.Flags().IntVar(&commitLimit, "limit", 100, "Maximum number of commits to consider")

	commitCmd.AddCommand(commitListCmd)
	commitCmd.AddCommand(commitViewCmd)
	commitCmd.AddCommand(commitActivityCmd)
}

func runCommitList(cmd *cobra.Command, args []string) error {
	client, err := commitNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	commits, err := fetchCommits(cmd, client, repoCtx, commitBranch, commitLimit)
	if err != nil {
		return err
	}

	if len(commits) == 0 {
		output.Info("No commits found")
		return nil
	}

	headers := []string{"HASH", "AUTHOR", "DATE", "MESSAGE"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, c := range commits {
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
	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugCommitCommit(ctx, repoCtx.Workspace, repoCtx.RepoSlug, rest[0])
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

var commitActivityCmd = &cobra.Command{
	Use:   "activity [workspace/repo]",
	Short: "Show commit counts per author",
	Long:  `Summarise commit activity by author over the most recent commits (--limit, --branch).`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCommitActivity,
}

func runCommitActivity(cmd *cobra.Command, args []string) error {
	client, err := commitNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	commits, err := fetchCommits(cmd, client, repoCtx, commitBranch, commitLimit)
	if err != nil {
		return err
	}
	if len(commits) == 0 {
		output.Info("No commits found")
		return nil
	}

	// Tally per author, preserving first-seen order for stable output.
	counts := map[string]int{}
	var order []string
	for _, c := range commits {
		name := c.authorName()
		if _, seen := counts[name]; !seen {
			order = append(order, name)
		}
		counts[name]++
	}
	// Sort by descending count, then name.
	sort.SliceStable(order, func(i, j int) bool {
		if counts[order[i]] != counts[order[j]] {
			return counts[order[i]] > counts[order[j]]
		}
		return order[i] < order[j]
	})

	headers := []string{"AUTHOR", "COMMITS"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, name := range order {
		rows = append(rows, []string{name, fmt.Sprintf("%d", counts[name])})
		jsonData = append(jsonData, map[string]interface{}{"author": name, "commits": counts[name]})
	}
	return output.Print(headers, rows, jsonData)
}

// fetchCommits returns commits for a repo (optionally a branch/ref), following
// pagination up to limit.
func fetchCommits(cmd *cobra.Command, client *api.Client, repoCtx *api.RepoContext, branch string, limit int) ([]commitSummary, error) {
	ctx := cmdCtx(cmd)

	var resp *http.Response
	var err error
	if branch != "" {
		resp, err = client.GetRepositoriesWorkspaceRepoSlugCommitsRevision(ctx, repoCtx.Workspace, repoCtx.RepoSlug, branch)
	} else {
		resp, err = client.GetRepositoriesWorkspaceRepoSlugCommits(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list commits: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []commitSummary `json:"values"`
		Next   string          `json:"next"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	for result.Next != "" && len(result.Values) < limit {
		next := result.Next
		result.Next = ""
		pageResp, err := client.GetURL(ctx, next)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch next page: %w", err)
		}
		var page struct {
			Values []commitSummary `json:"values"`
			Next   string          `json:"next"`
		}
		err = json.NewDecoder(pageResp.Body).Decode(&page)
		pageResp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to parse page: %w", err)
		}
		result.Values = append(result.Values, page.Values...)
		result.Next = page.Next
	}

	if len(result.Values) > limit {
		result.Values = result.Values[:limit]
	}
	return result.Values, nil
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
