package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

// uuidRe matches a Bitbucket UUID in its canonical braced form, e.g.
// "{de3a3453-2e73-4838-8199-e8067ea245ed}".
var uuidRe = regexp.MustCompile(`^\{[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\}$`)

// workspaceMember is the subset of a workspace-membership entry this file
// needs. Kept local (rather than shared with workspace.go's own row type) so
// each command can decode only the fields it actually reads.
type workspaceMember struct {
	User *struct {
		DisplayName string `json:"display_name"`
		Nickname    string `json:"nickname"`
		AccountID   string `json:"account_id"`
		UUID        string `json:"uuid"`
	} `json:"user"`
}

// resolveWorkspaceMember turns a human-typed reviewer query into the UUID
// Bitbucket's API actually wants. A query that is already a braced UUID is
// passed through unchanged (no network call). Anything else is matched as a
// case-insensitive substring against every member's display name and
// nickname - the only two fields a person is likely to type ("Dave",
// "Dave Riches", their Bitbucket handle).
//
// Exactly one match is required: zero is reported as "nobody matched" and
// more than one lists every candidate so the caller can narrow the query,
// rather than silently picking one.
func resolveWorkspaceMember(ctx context.Context, client *api.Client, workspace, query string) (uuid, displayName string, err error) {
	if uuidRe.MatchString(query) {
		return query, query, nil
	}

	resp, err := client.GetWorkspacesWorkspaceMembers(ctx, workspace)
	if err != nil {
		return "", "", fmt.Errorf("failed to list workspace members: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("failed to list workspace members: API error: %s", resp.Status)
	}

	members, err := fetchAllPages[workspaceMember](ctx, client, resp)
	if err != nil {
		return "", "", fmt.Errorf("failed to list workspace members: %w", err)
	}

	q := strings.ToLower(query)
	var matches []workspaceMember
	for _, m := range members {
		if m.User == nil {
			continue
		}
		if strings.Contains(strings.ToLower(m.User.DisplayName), q) || strings.Contains(strings.ToLower(m.User.Nickname), q) {
			matches = append(matches, m)
		}
	}

	switch len(matches) {
	case 0:
		return "", "", fmt.Errorf("no member of workspace %q matches %q (tried display name and nickname)", workspace, query)
	case 1:
		return matches[0].User.UUID, matches[0].User.DisplayName, nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = fmt.Sprintf("%s (%s)", m.User.DisplayName, m.User.Nickname)
		}
		return "", "", fmt.Errorf("%q matches more than one workspace member, be more specific: %s", query, strings.Join(names, ", "))
	}
}

// resolveWorkspaceMembers resolves each query in turn, short-circuiting on
// the first failure so a typo in the third name doesn't silently drop the
// first two from the caller's perspective.
func resolveWorkspaceMembers(ctx context.Context, client *api.Client, workspace string, queries []string) ([]reviewerRef, error) {
	refs := make([]reviewerRef, 0, len(queries))
	for _, q := range queries {
		uuid, displayName, err := resolveWorkspaceMember(ctx, client, workspace, q)
		if err != nil {
			return nil, err
		}
		refs = append(refs, reviewerRef{UUID: uuid, DisplayName: displayName})
	}
	return refs, nil
}

// reviewerRef is the minimal identity of a reviewer this package threads
// between "resolve a name" and "build the API payload."
type reviewerRef struct {
	UUID        string
	DisplayName string
}
