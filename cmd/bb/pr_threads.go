package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var prThreadsCmd = &cobra.Command{
	Use:   "threads <workspace/repo> <pr-number>",
	Short: "Show pull request comments as threads",
	Long:  `Display a pull request's comments grouped into threads (replies nested under their parent, inline comments anchored to their file/line).`,
	Args:  cobra.ExactArgs(2),
	RunE:  runPRThreads,
}

func init() {
	prCmd.AddCommand(prThreadsCmd)
}

type prThreadComment struct {
	ID      int `json:"id"`
	Content struct {
		Raw string `json:"raw"`
	} `json:"content"`
	User struct {
		DisplayName string `json:"display_name"`
	} `json:"user"`
	Parent *struct {
		ID int `json:"id"`
	} `json:"parent"`
	Inline *struct {
		Path string `json:"path"`
		To   int    `json:"to"`
	} `json:"inline"`
	Deleted bool `json:"deleted"`
}

func runPRThreads(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}
	prID, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid pr-number %q", args[1])
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdComments(ctx, repoCtx.Workspace, repoCtx.RepoSlug, prID)
	if err != nil {
		return fmt.Errorf("failed to get comments: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []prThreadComment `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if jsonOutput {
		return output.Print(nil, nil, result.Values)
	}

	// Build parent -> children, ignoring deleted comments.
	children := map[int][]prThreadComment{}
	var roots []prThreadComment
	for _, c := range result.Values {
		if c.Deleted {
			continue
		}
		if c.Parent != nil {
			children[c.Parent.ID] = append(children[c.Parent.ID], c)
		} else {
			roots = append(roots, c)
		}
	}

	if len(roots) == 0 {
		output.Info("No comments found")
		return nil
	}

	for _, root := range roots {
		printThread(root, children, 0)
	}
	return nil
}

func printThread(c prThreadComment, children map[int][]prThreadComment, depth int) {
	indent := strings.Repeat("  ", depth)
	anchor := ""
	if c.Inline != nil && c.Inline.Path != "" {
		if c.Inline.To > 0 {
			anchor = fmt.Sprintf(" [%s:%d]", c.Inline.Path, c.Inline.To)
		} else {
			anchor = fmt.Sprintf(" [%s]", c.Inline.Path)
		}
	}
	fmt.Printf("%s- %s%s: %s\n", indent, c.User.DisplayName, anchor, firstLine(c.Content.Raw))
	for _, child := range children[c.ID] {
		printThread(child, children, depth+1)
	}
}
