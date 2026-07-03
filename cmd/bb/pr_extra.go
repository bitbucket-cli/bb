package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/ThomasRedstone/bb/internal/output"
	"github.com/spf13/cobra"
)

var (
	prEditTitle string
	prEditBody  string
	prTaskBody  string
)

// ---- pr edit ----

var prEditCmd = &cobra.Command{
	Use:   "edit [workspace/repo] [pr-number]",
	Short: "Edit a pull request's title/description",
	Args:  cobra.RangeArgs(0, 2),
	RunE:  runPREdit,
}

func runPREdit(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := prNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndOptionalNumber(args)
	if err != nil {
		return err
	}
	interactive, _ := cmd.Flags().GetBool("interactive")
	prID, err := getPRID(ctx, rest, client, repoCtx, interactive)
	if err != nil {
		return err
	}

	// Fetch current PR so unspecified fields are preserved (PUT replaces).
	getResp, err := client.Request(ctx, http.MethodGet,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%d", repoCtx.Workspace, repoCtx.RepoSlug, prID), nil)
	if err != nil {
		return err
	}
	var cur struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := decodeJSON(getResp, &cur); err != nil {
		return err
	}
	title, desc := cur.Title, cur.Description
	if prEditTitle != "" {
		title = prEditTitle
	}
	if cmd.Flags().Changed("body") {
		desc = prEditBody
	}

	resp, err := client.Request(ctx, http.MethodPut,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%d", repoCtx.Workspace, repoCtx.RepoSlug, prID),
		map[string]interface{}{"title": title, "description": desc})
	if err != nil {
		return fmt.Errorf("failed to edit pull request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Updated pull request #%d", prID)
	return nil
}

// ---- pr resolve / unresolve a comment ----

var prResolveCmd = &cobra.Command{
	Use:   "resolve [workspace/repo] <pr-number> <comment-id>",
	Short: "Resolve a pull request comment thread",
	Args:  cobra.RangeArgs(2, 3),
	RunE:  func(c *cobra.Command, a []string) error { return prResolveOp(c, a, http.MethodPost, "Resolved") },
}

var prUnresolveCmd = &cobra.Command{
	Use:   "unresolve [workspace/repo] <pr-number> <comment-id>",
	Short: "Reopen a resolved pull request comment thread",
	Args:  cobra.RangeArgs(2, 3),
	RunE:  func(c *cobra.Command, a []string) error { return prResolveOp(c, a, http.MethodDelete, "Reopened") },
}

func prResolveOp(cmd *cobra.Command, args []string, method, verb string) error {
	ctx := cmdCtx(cmd)
	client, err := prNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndArgs(args, 2)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, method,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%s/comments/%s/resolve", repoCtx.Workspace, repoCtx.RepoSlug, rest[0], rest[1]), nil)
	if err != nil {
		return fmt.Errorf("failed to update comment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("%s comment %s on PR #%s", verb, rest[1], rest[0])
	return nil
}

// ---- pr tasks ----

var prTasksCmd = &cobra.Command{
	Use:     "tasks",
	Aliases: []string{"task"},
	Short:   "Manage pull request tasks",
}

var prTasksListCmd = &cobra.Command{
	Use:   "list [workspace/repo] <pr-number>",
	Short: "List a pull request's tasks",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runPRTasksList,
}

var prTasksAddCmd = &cobra.Command{
	Use:   "add [workspace/repo] <pr-number>",
	Short: "Add a task to a pull request",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runPRTasksAdd,
}

var prTasksDoneCmd = &cobra.Command{
	Use:   "complete [workspace/repo] <pr-number> <task-id>",
	Short: "Mark a pull request task complete",
	Args:  cobra.RangeArgs(2, 3),
	RunE:  runPRTasksDone,
}

func runPRTasksList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := prNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodGet,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%s/tasks", repoCtx.Workspace, repoCtx.RepoSlug, rest[0]), nil)
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}
	var page struct {
		Values []struct {
			ID      int    `json:"id"`
			State   string `json:"state"`
			Content struct {
				Raw string `json:"raw"`
			} `json:"content"`
		} `json:"values"`
	}
	if err := decodeJSON(resp, &page); err != nil {
		return err
	}
	if len(page.Values) == 0 {
		output.Info("No tasks")
		return nil
	}
	headers := []string{"ID", "STATE", "TASK"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, t := range page.Values {
		rows = append(rows, []string{strconv.Itoa(t.ID), t.State, output.Truncate(t.Content.Raw, 50)})
		jsonData = append(jsonData, map[string]interface{}{"id": t.ID, "state": t.State, "task": t.Content.Raw})
	}
	return output.Print(headers, rows, jsonData)
}

func runPRTasksAdd(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := prNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}
	body := map[string]interface{}{"content": map[string]interface{}{"raw": prTaskBody}}
	resp, err := client.Request(ctx, http.MethodPost,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%s/tasks", repoCtx.Workspace, repoCtx.RepoSlug, rest[0]), body)
	if err != nil {
		return fmt.Errorf("failed to add task: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Added task to PR #%s", rest[0])
	return nil
}

func runPRTasksDone(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	client, err := prNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndArgs(args, 2)
	if err != nil {
		return err
	}
	resp, err := client.Request(ctx, http.MethodPut,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%s/tasks/%s", repoCtx.Workspace, repoCtx.RepoSlug, rest[0], rest[1]),
		map[string]interface{}{"state": "RESOLVED"})
	if err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}
	output.Success("Completed task %s on PR #%s", rest[1], rest[0])
	return nil
}

func init() {
	prEditCmd.Flags().StringVarP(&prEditTitle, "title", "t", "", "New title")
	prEditCmd.Flags().StringVarP(&prEditBody, "body", "b", "", "New description")
	prEditCmd.Flags().BoolP("interactive", "i", false, "Select PR interactively")
	prTasksAddCmd.Flags().StringVarP(&prTaskBody, "body", "b", "", "Task text (required)")
	prTasksAddCmd.MarkFlagRequired("body")
	prTasksCmd.AddCommand(prTasksListCmd, prTasksAddCmd, prTasksDoneCmd)
	prCmd.AddCommand(prEditCmd, prResolveCmd, prUnresolveCmd, prTasksCmd)
}
