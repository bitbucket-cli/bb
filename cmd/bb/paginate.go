package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
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
	type page struct {
		Values []T    `json:"values"`
		Next   string `json:"next"`
	}

	var all []T
	var p page
	if err := json.NewDecoder(first.Body).Decode(&p); err != nil {
		return nil, err
	}
	all = append(all, p.Values...)
	next := p.Next

	for next != "" {
		resp, err := client.GetURL(ctx, next)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch next page: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("API error fetching next page: %s", resp.Status)
		}
		var np page
		err = json.NewDecoder(resp.Body).Decode(&np)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		all = append(all, np.Values...)
		next = np.Next
	}
	return all, nil
}
