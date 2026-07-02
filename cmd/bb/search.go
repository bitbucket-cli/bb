package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var searchLimit int

// Mockable dependency for testing.
var searchNewAPIClient = api.NewClient

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search code across a workspace",
	Long: `Search code across every repository in a workspace using Bitbucket's code
search. The workspace comes from --repo, BB_WORKSPACE/config, or the current git
remote.

Example:
  bb search "TODO fixme"
  bb search "func NewClient" -R myworkspace/anything`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 25, "Maximum number of results to show")
	rootCmd.AddCommand(searchCmd)
}

type codeSearchResult struct {
	ContentMatchCount int `json:"content_match_count"`
	File              struct {
		Path   string `json:"path"`
		Commit struct {
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		} `json:"commit"`
	} `json:"file"`
}

type codeSearchPage struct {
	Values []codeSearchResult `json:"values"`
	Next   string             `json:"next"`
}

func runSearch(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := searchNewAPIClient()
	if err != nil {
		return err
	}
	ws, err := workspaceArg(nil)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/workspaces/%s/search/code?search_query=%s", ws, url.QueryEscape(args[0]))
	var all []codeSearchResult
	for path != "" && len(all) < searchLimit {
		resp, err := client.Request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		page, err := decodeCodeSearchPage(resp)
		if err != nil {
			return err
		}
		all = append(all, page.Values...)
		path = page.Next
	}
	if len(all) > searchLimit {
		all = all[:searchLimit]
	}

	if len(all) == 0 {
		output.Info("No matches found")
		return nil
	}

	headers := []string{"REPO", "FILE", "MATCHES"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, r := range all {
		repo := r.File.Commit.Repository.FullName
		rows = append(rows, []string{repo, r.File.Path, fmt.Sprintf("%d", r.ContentMatchCount)})
		jsonData = append(jsonData, map[string]interface{}{
			"repo":    repo,
			"file":    r.File.Path,
			"matches": r.ContentMatchCount,
		})
	}
	return output.Print(headers, rows, jsonData)
}

func decodeCodeSearchPage(resp *http.Response) (*codeSearchPage, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}
	var p codeSearchPage
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &p, nil
}
