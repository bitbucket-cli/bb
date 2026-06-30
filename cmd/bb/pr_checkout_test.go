package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func resetPrCheckoutMocks() {
	prNewAPIClient = api.NewClient
	prCheckoutDetect = nil
	prCheckoutFetch = nil
	prCheckoutCheckout = nil
}

func TestRunPrCheckout_ExplicitRepo(t *testing.T) {
	defer resetPrCheckoutMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"source":{"branch":{"name":"feature/x"}}}`))
	}))
	defer server.Close()

	var fetchedRemote, fetchedRef, checkedOut string
	prNewAPIClient = mockAPIClientFactory(server)
	prCheckoutFetch = func(remote, ref string) error { fetchedRemote, fetchedRef = remote, ref; return nil }
	prCheckoutCheckout = func(branch string) error { checkedOut = branch; return nil }

	if err := runPrCheckout(prCheckoutCmd, []string{"42", "myws/myrepo"}); err != nil {
		t.Fatalf("runPrCheckout error: %v", err)
	}
	if fetchedRemote != "origin" || fetchedRef != "feature/x" {
		t.Errorf("fetch = %q %q, want origin feature/x", fetchedRemote, fetchedRef)
	}
	if checkedOut != "feature/x" {
		t.Errorf("checkout = %q, want feature/x", checkedOut)
	}
}

func TestRunPrCheckout_AutoDetect(t *testing.T) {
	defer resetPrCheckoutMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"source":{"branch":{"name":"bugfix"}}}`))
	}))
	defer server.Close()

	var detected bool
	prNewAPIClient = mockAPIClientFactory(server)
	prCheckoutDetect = func() (string, string, error) { detected = true; return "autows", "autorepo", nil }
	prCheckoutFetch = func(remote, ref string) error { return nil }
	prCheckoutCheckout = func(branch string) error { return nil }

	if err := runPrCheckout(prCheckoutCmd, []string{"7"}); err != nil {
		t.Fatalf("runPrCheckout error: %v", err)
	}
	if !detected {
		t.Error("expected auto-detect to be used when repo arg omitted")
	}
}

func TestRunPrCheckout_BadNumber(t *testing.T) {
	defer resetPrCheckoutMocks()
	if err := runPrCheckout(prCheckoutCmd, []string{"notanumber", "myws/myrepo"}); err == nil {
		t.Error("expected error for non-numeric pr-number")
	}
}

func TestRunPrCreate_DefaultsToCurrentBranch(t *testing.T) {
	defer resetPrCheckoutMocks()
	defer func() {
		prTitle, prSource, prDest, prBody, prReviewers = "", "", "", "", nil
		prCurrentBranch = nil
	}()

	var postedSource string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"mainbranch":{"name":"main"}}`))
		case http.MethodPost:
			var body struct {
				Source struct {
					Branch struct {
						Name string `json:"name"`
					} `json:"branch"`
				} `json:"source"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			postedSource = body.Source.Branch.Name
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":1}`))
		}
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	prCurrentBranch = func() (string, error) { return "feature/auto", nil }
	prTitle = "Test PR"
	prSource = "" // should default to the current branch

	if err := runPRCreate(prCreateCmd, []string{"myws/myrepo"}); err != nil {
		t.Fatalf("runPRCreate error: %v", err)
	}
	if postedSource != "feature/auto" {
		t.Errorf("posted source branch = %q, want feature/auto", postedSource)
	}
}

func TestRunPRComment_Inline(t *testing.T) {
	defer resetPrCheckoutMocks()
	defer func() { prComment, prCommentPath, prCommentLine = "", "", 0 }()

	var gotPath string
	var gotLine float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Inline map[string]interface{} `json:"inline"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Inline != nil {
			gotPath, _ = body.Inline["path"].(string)
			gotLine, _ = body.Inline["to"].(float64)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	prComment = "needs a fix"
	prCommentPath = "main.go"
	prCommentLine = 42

	if err := runPRComment(prCommentCmd, []string{"myws/myrepo", "5"}); err != nil {
		t.Fatalf("runPRComment error: %v", err)
	}
	if gotPath != "main.go" {
		t.Errorf("inline.path = %q, want main.go", gotPath)
	}
	if gotLine != 42 {
		t.Errorf("inline.to = %v, want 42", gotLine)
	}
}

func TestRunPRCreate_RequiresTitle(t *testing.T) {
	defer resetPrCheckoutMocks()
	defer func() {
		prTitle, prSource, prBody, prReviewers = "", "", "", nil
		prCurrentBranch = nil
	}()

	prNewAPIClient = mockAPIClientFactory(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	prCurrentBranch = func() (string, error) { return "feature", nil }
	prTitle = "" // no title, not interactive -> error

	err := runPRCreate(prCreateCmd, []string{"myws/myrepo"})
	if err == nil {
		t.Error("expected an error when no title is given without -i")
	}
}

func TestLoadPRTemplate(t *testing.T) {
	dir := t.TempDir()

	// No template yet.
	if got := loadPRTemplate(dir); got != "" {
		t.Errorf("loadPRTemplate(empty) = %q, want \"\"", got)
	}

	// .github template is found.
	if err := os.MkdirAll(filepath.Join(dir, ".github"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".github", "pull_request_template.md"), []byte("## Summary"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := loadPRTemplate(dir); got != "## Summary" {
		t.Errorf("loadPRTemplate = %q, want '## Summary'", got)
	}

	// A higher-precedence template wins.
	if err := os.WriteFile(filepath.Join(dir, "pull_request_template.md"), []byte("ROOT"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := loadPRTemplate(dir); got != "ROOT" {
		t.Errorf("loadPRTemplate precedence = %q, want ROOT", got)
	}
}

func TestResolvePrRepo(t *testing.T) {
	defer resetPrCheckoutMocks()

	ctx, err := resolvePrRepo([]string{"42", "myws/myrepo"})
	if err != nil || ctx.Workspace != "myws" || ctx.RepoSlug != "myrepo" {
		t.Errorf("resolvePrRepo explicit = %+v, %v", ctx, err)
	}

	prCheckoutDetect = func() (string, string, error) { return "dws", "drepo", nil }
	ctx, err = resolvePrRepo([]string{"42"})
	if err != nil || ctx.Workspace != "dws" || ctx.RepoSlug != "drepo" {
		t.Errorf("resolvePrRepo auto = %+v, %v", ctx, err)
	}
}
