package main

import (
	"fmt"

	"github.com/ThomasRedstone/bb/internal/git"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

// repoOverride holds the value of the global --repo/-R flag.
var repoOverride string

// detectRepo is overridable in tests.
var detectRepo = git.DetectRepo

// repoArg resolves the target repository for a repo-scoped command. It is a
// drop-in replacement for api.ParseRepoContext(args[0]): when a repo is supplied
// (positionally or via --repo) behaviour is identical, so it is safe to use even
// in commands whose repo positional is still required. When no repo is given it
// falls back to the current git repository's Bitbucket remote.
//
// Resolution order: --repo/-R flag > leading positional > git remote.
func repoArg(args []string) (*api.RepoContext, error) {
	if repoOverride != "" {
		return api.ParseRepoContext(repoOverride)
	}
	if len(args) > 0 {
		return api.ParseRepoContext(args[0])
	}
	ws, slug, err := detectRepo()
	if err != nil {
		return nil, fmt.Errorf("no repository given and none detected from the git remote; pass <workspace/repo> or use --repo: %w", err)
	}
	return &api.RepoContext{Workspace: ws, RepoSlug: slug}, nil
}
