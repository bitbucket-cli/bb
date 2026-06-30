package git

import "testing"

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
