package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bitbucket-cli/bb/internal/output"
	"github.com/spf13/cobra"
)

var prThreadsCmd = &cobra.Command{
	Use:   "threads [workspace/repo] <pr-number>",
	Short: "Show pull request comments as threads",
	Long:  `Display a pull request's comments grouped into threads (replies nested under their parent, inline comments anchored to their file/line).`,
	Args:  cobra.RangeArgs(1, 2),
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
	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}
	prID, err := strconv.Atoi(rest[0])
	if err != nil {
		return fmt.Errorf("invalid pr-number %q", rest[0])
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdComments(ctx, repoCtx.Workspace, repoCtx.RepoSlug, prID)
	if err != nil {
		return fmt.Errorf("failed to get comments: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	comments, err := fetchAllPages[prThreadComment](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if output.CurrentFormat == output.FormatJSON {
		return output.Print(nil, nil, comments)
	}

	if output.CurrentFormat == output.FormatCSV {
		var rows [][]string
		for _, c := range comments {
			parent := ""
			if c.Parent != nil {
				parent = strconv.Itoa(c.Parent.ID)
			}
			content := c.Content.Raw
			if c.Deleted {
				content = "[deleted]"
			}
			rows = append(rows, []string{strconv.Itoa(c.ID), parent, c.User.DisplayName, content})
		}
		return output.Print([]string{"ID", "PARENT", "AUTHOR", "CONTENT"}, rows, comments)
	}

	// Retain deleted ancestors as placeholders so their live replies remain visible.
	byID := map[int]prThreadComment{}
	for _, c := range comments {
		byID[c.ID] = c
	}
	visible := map[int]bool{}
	for _, c := range comments {
		if c.Deleted {
			continue
		}
		for !visible[c.ID] {
			visible[c.ID] = true
			if c.Parent == nil {
				break
			}
			parent, ok := byID[c.Parent.ID]
			if !ok {
				break
			}
			c = parent
		}
	}

	// Build parent -> children from the retained comments.
	children := map[int][]prThreadComment{}
	var roots []prThreadComment
	for _, c := range comments {
		if !visible[c.ID] {
			continue
		}
		if c.Parent != nil && visible[c.Parent.ID] {
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
	content := firstLine(c.Content.Raw)
	if c.Deleted {
		content = "[deleted]"
	}
	fmt.Printf("%s- #%d %s%s: %s\n", indent, c.ID, c.User.DisplayName, anchor, content)
	for _, child := range children[c.ID] {
		printThread(child, children, depth+1)
	}
}
