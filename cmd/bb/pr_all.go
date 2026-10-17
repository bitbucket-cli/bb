package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/bitbucket-cli/bb/generated/bitbucket"
	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	prAll   bool
	prUser  string
	prLinks bool
)

// wsPR is the subset of a pull request we render from the workspace-wide endpoint.
type wsPR struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Author struct {
		DisplayName string `json:"display_name"`
		UUID        string `json:"uuid"`
		AccountID   string `json:"account_id"`
	} `json:"author"`
	Source struct {
		Branch     struct{ Name string } `json:"branch"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	} `json:"source"`
	Links struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
}

func (p wsPR) url() string {
	if p.Links.HTML.Href != "" {
		return p.Links.HTML.Href
	}
	if p.Source.Repository.FullName != "" {
		return fmt.Sprintf("https://bitbucket.org/%s/pull-requests/%d", p.Source.Repository.FullName, p.ID)
	}
	return ""
}

// runPRListAll lists pull requests for a user across every repository in a
// workspace, via GET /2.0/workspaces/{workspace}/pull-requests/{selected_user}.
func runPRListAll(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)
	if ctx == nil {
		ctx = context.Background()
	}

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	workspace, err := workspaceArg(args)
	if err != nil {
		return err
	}

	user := prUser
	if user == "" {
		user, err = currentUserUUID(ctx, client)
		if err != nil {
			return fmt.Errorf("could not determine current user (pass --user <uuid|account_id>): %w", err)
		}
	}

	params := &bitbucket.GetWorkspacesWorkspacePullrequestsSelectedUserParams{}
	if prState != "" {
		st := bitbucket.GetWorkspacesWorkspacePullrequestsSelectedUserParamsState(prState)
		params.State = &st
	}

	resp, err := client.GetWorkspacesWorkspacePullrequestsSelectedUser(ctx, workspace, user, params)
	if err != nil {
		return fmt.Errorf("failed to list pull requests: %w", err)
	}
	defer resp.Body.Close()
	all, err := fetchAllPages[wsPR](ctx, client, resp)
	if err != nil {
		return err
	}

	if len(all) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No pull requests found")
		return nil
	}

	headers := []string{"REPO", "#", "TITLE", "AUTHOR", "STATE"}
	if prLinks {
		headers = append(headers, "LINK")
	}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, pr := range all {
		row := []string{
			pr.Source.Repository.FullName,
			strconv.Itoa(pr.ID),
			output.Truncate(pr.Title, 45),
			pr.Author.DisplayName,
			pr.State,
		}
		if prLinks {
			row = append(row, pr.url())
		}
		rows = append(rows, row)
		jsonData = append(jsonData, map[string]interface{}{
			"repo":   pr.Source.Repository.FullName,
			"id":     pr.ID,
			"title":  pr.Title,
			"author": pr.Author.DisplayName,
			"state":  pr.State,
			"url":    pr.url(),
		})
	}
	return output.Print(headers, rows, jsonData)
}

// currentUserUUID resolves the authenticated user's UUID via GET /2.0/user.
func currentUserUUID(ctx context.Context, client *api.Client) (string, error) {
	resp, err := client.GetUser(ctx)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %s", resp.Status)
	}
	var u struct {
		UUID      string `json:"uuid"`
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", err
	}
	if u.UUID != "" {
		return u.UUID, nil
	}
	return u.AccountID, nil
}
