package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

// commitStatus is the subset of a Bitbucket commit/build status we render.
type commitStatus struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	State       string `json:"state"` // SUCCESSFUL | FAILED | INPROGRESS | STOPPED
	URL         string `json:"url"`
}

type statusPage struct {
	Values []commitStatus `json:"values"`
	Next   string         `json:"next"`
}

func decodeStatusPage(resp *http.Response) (*statusPage, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}
	var p statusPage
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &p, nil
}

// renderStatuses decodes a paginated commit/PR statuses response (following
// pagination) and prints STATE/KEY/NAME/URL. Shared by `commit status` and
// `pr checks`.
func renderStatuses(ctx context.Context, client *api.Client, resp *http.Response) error {
	page, err := decodeStatusPage(resp)
	if err != nil {
		return err
	}
	all := page.Values
	for page.Next != "" {
		next, err := client.GetURL(ctx, page.Next)
		if err != nil {
			return fmt.Errorf("failed to fetch next page: %w", err)
		}
		if page, err = decodeStatusPage(next); err != nil {
			return err
		}
		all = append(all, page.Values...)
	}

	if len(all) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No statuses/checks reported")
		return nil
	}

	headers := []string{"STATE", "KEY", "NAME", "URL"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, s := range all {
		rows = append(rows, []string{s.State, s.Key, output.Truncate(s.Name, 40), s.URL})
		jsonData = append(jsonData, map[string]interface{}{
			"state":       s.State,
			"key":         s.Key,
			"name":        s.Name,
			"description": s.Description,
			"url":         s.URL,
		})
	}
	return output.Print(headers, rows, jsonData)
}
