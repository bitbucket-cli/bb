package git

import (
	"fmt"
	"testing"
)

func TestParseRemoteURL(t *testing.T) {
	cases := []struct {
		in        string
		workspace string
		repo      string
		wantErr   bool
	}{
		{"git@bitbucket.org:myws/myrepo.git", "myws", "myrepo", false},
		{"git@bitbucket.org:myws/myrepo", "myws", "myrepo", false},
		{"https://bitbucket.org/myws/myrepo.git", "myws", "myrepo", false},
		{"https://user@bitbucket.org/myws/myrepo.git", "myws", "myrepo", false},
		{"https://x-token-auth@bitbucket.org/myws/myrepo", "myws", "myrepo", false},
		{"ssh://git@bitbucket.org/myws/myrepo.git", "myws", "myrepo", false},
		{"https://bitbucket.org/myws/myrepo/", "myws", "myrepo", false},
		{"git@github.com:myws/myrepo.git", "", "", true},
		{"not a url", "", "", true},
	}
	for _, c := range cases {
		ws, repo, err := ParseRemoteURL(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseRemoteURL(%q) expected error, got %q/%q", c.in, ws, repo)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRemoteURL(%q) error: %v", c.in, err)
			continue
		}
		if ws != c.workspace || repo != c.repo {
			t.Errorf("ParseRemoteURL(%q) = %q/%q, want %q/%q", c.in, ws, repo, c.workspace, c.repo)
		}
	}
}

func TestGitOpsInvokeCorrectArgs(t *testing.T) {
	orig := runGit
	defer func() { runGit = orig }()

	var gotArgs []string
	runGit = func(args ...string) (string, error) {
		gotArgs = args
		return "the-branch", nil
	}

	if err := Fetch("origin", "feature/x"); err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(gotArgs) != 3 || gotArgs[0] != "fetch" || gotArgs[1] != "origin" || gotArgs[2] != "feature/x" {
		t.Errorf("Fetch args = %v, want [fetch origin feature/x]", gotArgs)
	}

	if err := Checkout("feature/x"); err != nil {
		t.Fatalf("Checkout error: %v", err)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "checkout" || gotArgs[1] != "feature/x" {
		t.Errorf("Checkout args = %v, want [checkout feature/x]", gotArgs)
	}

	branch, err := CurrentBranch()
	if err != nil || branch != "the-branch" {
		t.Errorf("CurrentBranch = %q, %v; want the-branch", branch, err)
	}
	if len(gotArgs) != 3 || gotArgs[0] != "rev-parse" {
		t.Errorf("CurrentBranch args = %v, want rev-parse ...", gotArgs)
	}
}

func TestDetectRepo_Error(t *testing.T) {
	orig := runGit
	defer func() { runGit = orig }()

	runGit = func(args ...string) (string, error) {
		return "", errInjected
	}
	if _, _, err := DetectRepo(); err == nil {
		t.Error("expected error when origin remote can't be read")
	}
}

var errInjected = fmt.Errorf("no remote")

func TestDetectRepo(t *testing.T) {
	orig := runGit
	defer func() { runGit = orig }()

	runGit = func(args ...string) (string, error) {
		return "git@bitbucket.org:detected/repo.git", nil
	}
	ws, repo, err := DetectRepo()
	if err != nil || ws != "detected" || repo != "repo" {
		t.Errorf("DetectRepo() = %q/%q, %v; want detected/repo", ws, repo, err)
	}
}

func TestValidateCloneURL(t *testing.T) {
	for _, raw := range []string{
		"git@bitbucket.org:workspace/repo.git",
		"ssh://git@bitbucket.org/workspace/repo.git",
		"ssh://git@bitbucket.org:22/workspace/repo.git",
		"https://bitbucket.org/workspace/repo.git",
		"https://user@bitbucket.org/workspace/repo.git",
	} {
		if err := ValidateCloneURL(raw); err != nil {
			t.Errorf("valid clone rejected %q: %v", raw, err)
		}
	}
	for _, raw := range []string{
		"--upload-pack=evil", "ext::evil", "/tmp/local", "file:///tmp/local",
		"http://bitbucket.org/workspace/repo", "https://bitbucket.org.evil/workspace/repo",
		"git@evil:workspace/repo", "ssh://user@bitbucket.org/workspace/repo",
		"https://bitbucket.org/workspace/repo?option=evil", "https://bitbucket.org/workspace/repo#evil",
		"https://user:secret@bitbucket.org/workspace/repo", "https://bitbucket.org/workspace/..",
		"https://bitbucket.org/workspace/repo\n", "https://bitbucket.org/workspace",
	} {
		if err := ValidateCloneURL(raw); err == nil {
			t.Errorf("unsafe clone accepted %q", raw)
		}
	}
}
