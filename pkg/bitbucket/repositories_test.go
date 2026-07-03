package bitbucket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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

func TestRepositoryHTTPSCloneURL(t *testing.T) {
	var r Repository
	r.Links.Clone = []struct {
		Href string `json:"href"`
		Name string `json:"name"`
	}{
		{Href: "git@bitbucket.org:ws/repo.git", Name: "ssh"},
		{Href: "https://bitbucket.org/ws/repo.git", Name: "https"},
	}
	if got, want := r.HTTPSCloneURL(), "https://bitbucket.org/ws/repo.git"; got != want {
		t.Errorf("HTTPSCloneURL() = %q, want %q", got, want)
	}

	// No https link present -> empty string.
	if got := (&Repository{}).HTTPSCloneURL(); got != "" {
		t.Errorf("HTTPSCloneURL() with no links = %q, want empty", got)
	}
}

func TestRepositoryIsStale(t *testing.T) {
	fresh := Repository{UpdatedOn: time.Now().Add(-1 * time.Hour)}
	if fresh.IsStale(24 * time.Hour) {
		t.Error("repo updated 1h ago should not be stale against a 24h threshold")
	}
	old := Repository{UpdatedOn: time.Now().Add(-48 * time.Hour)}
	if !old.IsStale(24 * time.Hour) {
		t.Error("repo updated 48h ago should be stale against a 24h threshold")
	}
}

// withBaseURL points the direct repository helpers at a test server.
func withBaseURL(t *testing.T, server *httptest.Server) {
	t.Helper()
	orig := baseURL
	baseURL = server.URL
	t.Cleanup(func() { baseURL = orig })
}

func TestListRepositories_FollowsPagination(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cursor") == "2" {
			fmt.Fprint(w, `{"values":[{"slug":"repo-b","name":"Repo B"}]}`)
			return
		}
		fmt.Fprintf(w, `{"values":[{"slug":"repo-a","name":"Repo A"}],"next":%q}`, server.URL+"/next?cursor=2")
	}))
	defer server.Close()
	withBaseURL(t, server)

	repos, err := (&Client{}).ListRepositories(context.Background(), "myws")
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(repos) != 2 || repos[0].Slug != "repo-a" || repos[1].Slug != "repo-b" {
		t.Fatalf("expected [repo-a repo-b] across 2 pages, got %+v", repos)
	}
}

func TestGetRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"slug":"myrepo","name":"My Repo","is_private":true}`)
	}))
	defer server.Close()
	withBaseURL(t, server)

	repo, err := (&Client{}).GetRepository(context.Background(), "ws", "myrepo")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if repo.Slug != "myrepo" || !repo.IsPrivate {
		t.Errorf("unexpected repo: %+v", repo)
	}
}

func TestCreateRepository_ReturnsSSHCloneURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		fmt.Fprint(w, `{"slug":"new-svc","full_name":"ws/new-svc","links":{"clone":[{"name":"ssh","href":"git@bitbucket.org:ws/new-svc.git"}]}}`)
	}))
	defer server.Close()
	withBaseURL(t, server)

	url, err := (&Client{}).CreateRepository(context.Background(), "ws", "New Svc", true, "desc")
	if err != nil {
		t.Fatalf("CreateRepository: %v", err)
	}
	if want := "git@bitbucket.org:ws/new-svc.git"; url != want {
		t.Errorf("clone url = %q, want %q", url, want)
	}
}

func TestDoJSON_Non2xxIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":{"message":"nope"}}`)
	}))
	defer server.Close()
	withBaseURL(t, server)

	if _, err := (&Client{}).GetRepository(context.Background(), "ws", "repo"); err == nil {
		t.Fatal("expected an error on 403, got nil")
	}
}
