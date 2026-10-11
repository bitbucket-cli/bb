package pr

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bitbucket "github.com/bitbucket-cli/bb/generated/bitbucket"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

// withServer points pr.NewClient at an httptest server for the duration of a
// test and restores the original constructor afterwards.
func withServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	orig := NewClient
	NewClient = func() (*api.Client, error) {
		c, err := bitbucket.NewClient(server.URL)
		if err != nil {
			return nil, err
		}
		return &api.Client{Client: c}, nil
	}
	t.Cleanup(func() {
		NewClient = orig
		server.Close()
	})
	return server
}

func TestList_FollowsPaginationAndMapsFields(t *testing.T) {
	var server *httptest.Server
	server = withServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "page=2") || strings.HasSuffix(r.URL.Path, "/page2") {
			fmt.Fprint(w, `{"values":[
				{"id":2,"title":"Second","author":{"display_name":"Bob"},"source":{"branch":{"name":"feat-2"}},"state":"OPEN"}
			]}`)
			return
		}
		// first page carries a next link back to this server
		fmt.Fprintf(w, `{"values":[
			{"id":1,"title":"First","author":{"display_name":"Alice"},"source":{"branch":{"name":"feat-1"}},"state":"OPEN"}
		],"next":%q}`, server.URL+"/page2")
	})

	prs, err := List(context.Background(), "myws", "myrepo", StateOpen)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs across 2 pages, got %d: %+v", len(prs), prs)
	}

	first := prs[0]
	if first.ID != 1 || first.Title != "First" || first.Author != "Alice" || first.Branch != "feat-1" || first.State != "OPEN" {
		t.Errorf("field mapping wrong for first PR: %+v", first)
	}
	wantURL := "https://bitbucket.org/myws/myrepo/pull-requests/1"
	if first.URL != wantURL {
		t.Errorf("URL = %q, want %q", first.URL, wantURL)
	}
	if prs[1].ID != 2 || prs[1].Branch != "feat-2" {
		t.Errorf("second PR wrong: %+v", prs[1])
	}
}

func TestList_APIError(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"message":"boom"}}`)
	})

	if _, err := List(context.Background(), "ws", "repo", StateOpen); err == nil {
		t.Fatal("expected an error on 500, got nil")
	}
}

func TestList_NextPageErrorReturnsPartialAndError(t *testing.T) {
	var server *httptest.Server
	server = withServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/page2") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, `{"values":[{"id":1,"title":"First","state":"OPEN"}],"next":%q}`, server.URL+"/page2")
	})

	prs, err := List(context.Background(), "ws", "repo", StateOpen)
	if err == nil {
		t.Fatal("expected an error when the second page fails, got nil")
	}
	if len(prs) != 1 {
		t.Errorf("expected the first page's 1 PR to be returned alongside the error, got %d", len(prs))
	}
}

func TestList_EmptyValues(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"values":[]}`)
	})

	prs, err := List(context.Background(), "ws", "repo", StateMerged)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("expected 0 PRs, got %d", len(prs))
	}
}
