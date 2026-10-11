package main

import (
	"fmt"
	"net/http"

	"github.com/bitbucket-cli/bb/internal/output"
	"github.com/spf13/cobra"
)

var prCommentEditBody string

var prCommentEditCmd = &cobra.Command{
	Use:   "comment-edit [workspace/repo] <pr-number> <comment-id>",
	Short: "Edit a pull request comment",
	Long: `Replace the text of one of your pull request comments.

Comment IDs are shown by 'bb pr comments' and 'bb pr threads'.`,
	Args: cobra.RangeArgs(2, 3),
	RunE: runPRCommentEdit,
}

var prCommentDeleteCmd = &cobra.Command{
	Use:   "comment-delete [workspace/repo] <pr-number> <comment-id>",
	Short: "Delete a pull request comment",
	Long: `Delete one of your pull request comments.

Comment IDs are shown by 'bb pr comments' and 'bb pr threads'.`,
	Args: cobra.RangeArgs(2, 3),
	RunE: runPRCommentDelete,
}

func init() {
	prCommentEditCmd.Flags().StringVarP(&prCommentEditBody, "body", "b", "", "New comment text (required)")
	prCommentEditCmd.MarkFlagRequired("body")
	prCmd.AddCommand(prCommentEditCmd, prCommentDeleteCmd)
}

func runPRCommentEdit(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := prNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndArgs(args, 2)
	if err != nil {
		return err
	}
	body := map[string]interface{}{"content": map[string]interface{}{"raw": prCommentEditBody}}
	resp, err := client.Request(ctx, http.MethodPut,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%s/comments/%s", repoCtx.Workspace, repoCtx.RepoSlug, rest[0], rest[1]), body)
	if err != nil {
		return fmt.Errorf("failed to edit comment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Edited comment %s on PR #%s", rest[1], rest[0])
	return nil
}

func runPRCommentDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := prNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndArgs(args, 2)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodDelete,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%s/comments/%s", repoCtx.Workspace, repoCtx.RepoSlug, rest[0], rest[1]), nil)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Deleted comment %s on PR #%s", rest[1], rest[0])
	return nil
}
