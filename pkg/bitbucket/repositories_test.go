package bitbucket

import "testing"

func TestRepoSlug(t *testing.T) {
	cases := map[string]string{
		"My Service":      "my-service",
		"already-a-slug":  "already-a-slug",
		"Mixed_Case Repo": "mixed-case-repo",
		"trailing!!":      "trailing",
		"  spaced  out  ": "spaced-out",
		"under_score":     "under-score",
		"keep123numbers":  "keep123numbers",
		"weird@#$chars":   "weird-chars",
	}
	for in, want := range cases {
		if got := repoSlug(in); got != want {
			t.Errorf("repoSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRepositoryIsArchived(t *testing.T) {
	cases := map[string]bool{
		"my-service":         false,
		"[archived] old-svc": true,
		"deprecated-thing":   true,
		"old_payments":       true,
		"normalish":          false,
	}
	for name, want := range cases {
		r := Repository{Name: name}
		if got := r.IsArchived(); got != want {
			t.Errorf("Repository{Name:%q}.IsArchived() = %v, want %v", name, got, want)
		}
	}
}

func TestRepositorySSHCloneURLFallback(t *testing.T) {
	r := Repository{FullName: "ws/repo"}
	if got, want := r.SSHCloneURL(), "git@bitbucket.org:ws/repo.git"; got != want {
		t.Errorf("SSHCloneURL() = %q, want %q", got, want)
	}
}
