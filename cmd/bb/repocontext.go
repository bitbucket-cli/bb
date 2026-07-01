package main

import (
	"fmt"
	"strings"

	"github.com/ThomasRedstone/bb/internal/config"
	"github.com/ThomasRedstone/bb/internal/git"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

// workspaceArg resolves just a workspace (for cross-repo commands). Resolution:
// --repo/-R workspace > leading positional's workspace > BB_WORKSPACE/config >
// current git remote's workspace.
func workspaceArg(args []string) (string, error) {
	if repoOverride != "" {
		return firstSegment(repoOverride), nil
	}
	if len(args) > 0 {
		return firstSegment(args[0]), nil
	}
	if ws := config.GetWorkspace(); ws != "" {
		return ws, nil
	}
	ws, _, err := detectRepo()
	if err != nil {
		return "", fmt.Errorf("no workspace given and none detected; pass a workspace, set BB_WORKSPACE, or use --repo: %w", err)
	}
	return ws, nil
}

// firstSegment returns the part of s before the first '/', or s if none.
func firstSegment(s string) string {
	if i := strings.IndexByte(s, '/'); i >= 0 {
		return s[:i]
	}
	return s
}

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
	return gitRepoCtx()
}

// gitRepoCtx resolves the repo from the current git remote.
func gitRepoCtx() (*api.RepoContext, error) {
	ws, slug, err := detectRepo()
	if err != nil {
		return nil, fmt.Errorf("no repository given and none detected from the git remote; pass <workspace/repo> or use --repo: %w", err)
	}
	return &api.RepoContext{Workspace: ws, RepoSlug: slug}, nil
}

// repoAndArgs resolves the repo for a command that takes requiredTrailing
// positional args after an optional leading <workspace/repo>, returning the repo
// and the trailing (non-repo) args. Resolution: --repo flag > leading positional
// (present only when there are more args than requiredTrailing) > git remote.
//
// Count-based, so it's unambiguous when the trailing args are required:
//
//	commit view <repo> <hash>  -> repoAndArgs(args, 1)
//	  ["ws/r","abc"] -> repo=ws/r, rest=["abc"]
//	  ["abc"]        -> repo=git,  rest=["abc"]
func repoAndArgs(args []string, requiredTrailing int) (*api.RepoContext, []string, error) {
	if repoOverride != "" {
		ctx, err := api.ParseRepoContext(repoOverride)
		return ctx, args, err
	}
	if len(args) > requiredTrailing {
		ctx, err := api.ParseRepoContext(args[0])
		return ctx, args[1:], err
	}
	ctx, err := gitRepoCtx()
	return ctx, args, err
}

// repoAndOptionalNumber resolves a repo plus an optional trailing numeric id
// (e.g. a PR number). A lone positional is the repo if it looks like
// workspace/repo, otherwise the id (with the repo auto-detected). Used where the
// trailing arg is optional and numeric, so format disambiguation is safe.
func repoAndOptionalNumber(args []string) (*api.RepoContext, []string, error) {
	if repoOverride != "" {
		ctx, err := api.ParseRepoContext(repoOverride)
		return ctx, args, err
	}
	if len(args) >= 2 {
		ctx, err := api.ParseRepoContext(args[0])
		return ctx, args[1:], err
	}
	if len(args) == 1 && looksLikeRepo(args[0]) {
		ctx, err := api.ParseRepoContext(args[0])
		return ctx, nil, err
	}
	ctx, err := gitRepoCtx()
	return ctx, args, err
}

// looksLikeRepo reports whether s has the workspace/repo shape (a single
// interior slash with non-empty sides).
func looksLikeRepo(s string) bool {
	i := strings.IndexByte(s, '/')
	return i > 0 && i < len(s)-1
}
