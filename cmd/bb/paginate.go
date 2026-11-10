package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

// fetchAllPages decodes a Bitbucket paginated list response and follows every
// "next" link, returning the concatenated "values" across all pages. `first` is
// the already-fetched, status-checked first page; the caller retains ownership
// of first.Body (e.g. via its own defer). Subsequent pages fetched here are
// closed as they are consumed, and each page's status is checked so a mid-run
// error surfaces instead of silently truncating the results.
//
// The element type T is provided by the caller (often a function-local named
// type), so each command keeps its own value shape.
func fetchAllPages[T any](ctx context.Context, client *api.Client, first *http.Response) ([]T, error) {
	all, _, err := fetchAllPagesWithStats[T](ctx, client, first)
	return all, err
}

type paginationStats struct {
	PagesFetched int  `json:"pages_fetched"`
	APITotal     *int `json:"api_total,omitempty"`
	Complete     bool `json:"complete"`
	HasMore      bool `json:"has_more"`
}

// fetchAllPagesWithStats provides evidence for list reports. A successful
// return means every advertised next link was followed and any API total was
// satisfied. Errors never return a partial list to the caller.
func fetchAllPagesWithStats[T any](ctx context.Context, client *api.Client, first *http.Response) ([]T, paginationStats, error) {
	var stats paginationStats
	if first.StatusCode != http.StatusOK {
		return nil, stats, fmt.Errorf("API error fetching first page: %s", first.Status)
	}
	type page struct {
		Values []T    `json:"values"`
		Next   string `json:"next"`
		Size   *int   `json:"size"`
	}

	var all []T
	var p page
	if err := json.NewDecoder(first.Body).Decode(&p); err != nil {
		return nil, stats, err
	}
	stats.PagesFetched = 1
	stats.APITotal = p.Size
	all = append(all, p.Values...)
	next := p.Next
	seen := make(map[string]bool)

	for next != "" {
		if seen[next] {
			return nil, stats, fmt.Errorf("API returned a repeated pagination link")
		}
		seen[next] = true
		resp, err := client.GetURL(ctx, next)
		if err != nil {
			return nil, stats, fmt.Errorf("failed to fetch next page: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, stats, fmt.Errorf("API error fetching next page: %s", resp.Status)
		}
		var np page
		err = json.NewDecoder(resp.Body).Decode(&np)
		resp.Body.Close()
		if err != nil {
			return nil, stats, err
		}
		stats.PagesFetched++
		all = append(all, np.Values...)
		next = np.Next
	}
	if p.Size != nil && len(all) < *p.Size {
		return nil, stats, fmt.Errorf("incomplete API results: received %d of %d items but no next link was supplied", len(all), *p.Size)
	}
	stats.Complete = true
	return all, stats, nil
}
