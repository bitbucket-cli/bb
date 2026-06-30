package main

import (
	"errors"
	"testing"

	"github.com/ThomasRedstone/bb/internal/git"
)

func TestRepoArg(t *testing.T) {
	defer func() {
		repoOverride = ""
		detectRepo = git.DetectRepo
	}()

	t.Run("positional repo is used", func(t *testing.T) {
		repoOverride = ""
		detectRepo = func() (string, string, error) { t.Fatal("git should not be consulted"); return "", "", nil }
		ctx, err := repoArg([]string{"ws/repo"})
		if err != nil || ctx.Workspace != "ws" || ctx.RepoSlug != "repo" {
			t.Fatalf("got %+v err=%v, want ws/repo", ctx, err)
		}
	})

	t.Run("--repo flag overrides positional", func(t *testing.T) {
		repoOverride = "ov/erride"
		defer func() { repoOverride = "" }()
		ctx, err := repoArg([]string{"ws/repo"})
		if err != nil || ctx.Workspace != "ov" || ctx.RepoSlug != "erride" {
			t.Fatalf("got %+v err=%v, want ov/erride", ctx, err)
		}
	})

	t.Run("falls back to git remote when no repo given", func(t *testing.T) {
		repoOverride = ""
		detectRepo = func() (string, string, error) { return "gitws", "gitrepo", nil }
		ctx, err := repoArg(nil)
		if err != nil || ctx.Workspace != "gitws" || ctx.RepoSlug != "gitrepo" {
			t.Fatalf("got %+v err=%v, want gitws/gitrepo", ctx, err)
		}
	})

	t.Run("clear error when no repo and detection fails", func(t *testing.T) {
		repoOverride = ""
		detectRepo = func() (string, string, error) { return "", "", errors.New("not a bitbucket remote") }
		if _, err := repoArg(nil); err == nil {
			t.Fatal("expected an error when no repo is given and detection fails")
		}
	})
}
