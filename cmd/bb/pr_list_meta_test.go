package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bitbucket-cli/bb/internal/output"
)

func TestPRListMetadataAcrossPages(t *testing.T) {
	oldFactory, oldAll, oldUser, oldState, oldMeta, oldFormat := prNewAPIClient, prAll, prUser, prState, prMeta, output.CurrentFormat
	t.Cleanup(func() {
		prNewAPIClient, prAll, prUser, prState, prMeta, output.CurrentFormat = oldFactory, oldAll, oldUser, oldState, oldMeta, oldFormat
	})
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/page2" {
			fmt.Fprint(w, `{"values":[{"id":2,"title":"Second","state":"OPEN","draft":false,"comment_count":3,"participants":[{"state":"changes_requested"}],"destination":{"repository":{"full_name":"ws/repo"}}}]}`)
			return
		}
		if !strings.Contains(r.URL.Path, "/workspaces/ws/pullrequests/") {
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("state") != "OPEN" {
			t.Errorf("missing state filter: %s", r.URL)
		}
		fmt.Fprintf(w, `{"size":2,"values":[{"id":1,"title":"First","state":"OPEN","draft":true,"comment_count":0,"participants":[{"state":"approved","approved":true}],"reviewers":[],"source":{"repository":{"full_name":"fork/repo"}},"destination":{"repository":{"full_name":"ws/repo"}}}],"next":%q}`, server.URL+"/page2")
	}))
	defer server.Close()
	prNewAPIClient = mockAPIClientFactory(server)
	prAll, prUser, prState, prMeta, output.CurrentFormat = true, "{self}", "OPEN", true, output.FormatJSON

	got := captureOut(func() {
		if err := runPRList(prListCmd, []string{"ws"}); err != nil {
			t.Error(err)
		}
	})
	var report struct {
		Query      prListQuery      `json:"query"`
		Pagination paginationStats  `json:"pagination"`
		Count      int              `json:"count"`
		Summary    prListSummary    `json:"summary"`
		Items      []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(got), &report); err != nil {
		t.Fatalf("metadata JSON: %q: %v", got, err)
	}
	if report.Query.Scope != "workspace" || report.Query.Workspace != "ws" || !report.Query.AllRepositories || report.Query.Author != "{self}" || report.Query.State != "OPEN" || report.Query.DraftFilter != "all" {
		t.Errorf("query metadata = %+v", report.Query)
	}
	if report.Pagination.PagesFetched != 2 || !report.Pagination.Complete || report.Pagination.HasMore || report.Pagination.APITotal == nil || *report.Pagination.APITotal != 2 || report.Count != 2 {
		t.Errorf("pagination metadata = %+v, count = %d", report.Pagination, report.Count)
	}
	if report.Items[0]["draft"] != true || report.Items[1]["comment_count"] != float64(3) {
		t.Errorf("items lost draft or comment count: %+v", report.Items)
	}
	if report.Items[0]["repo"] != "ws/repo" || report.Items[0]["url"] != "https://bitbucket.org/ws/repo/pull-requests/1" {
		t.Errorf("fork PR was counted under the wrong repository: %+v", report.Items[0])
	}
	if report.Items[0]["review_state"] != "approved" || report.Items[1]["review_state"] != "changes_requested" || report.Summary.ByReview["approved"] != 1 || report.Summary.ByReview["changes_requested"] != 1 || report.Summary.Drafts != 1 || report.Summary.NonDrafts != 1 || report.Summary.ByRepository["ws/repo"] != 2 {
		t.Errorf("review and summary = %+v, items = %+v", report.Summary, report.Items)
	}
}

func TestRepositoryPRListMetadataAndLegacyJSON(t *testing.T) {
	oldFactory, oldAll, oldUser, oldState, oldMeta, oldFormat := prNewAPIClient, prAll, prUser, prState, prMeta, output.CurrentFormat
	t.Cleanup(func() {
		prNewAPIClient, prAll, prUser, prState, prMeta, output.CurrentFormat = oldFactory, oldAll, oldUser, oldState, oldMeta, oldFormat
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != "MERGED" {
			t.Errorf("missing repository state filter: %s", r.URL)
		}
		fmt.Fprint(w, `{"values":[{"id":7,"title":"Done","state":"MERGED","source":{"branch":{"name":"feature"}}}]}`)
	}))
	defer server.Close()
	prNewAPIClient = mockAPIClientFactory(server)
	prAll, prUser, prState, output.CurrentFormat = false, "", "MERGED", output.FormatJSON

	prMeta = false
	legacy := captureOut(func() {
		if err := runPRList(prListCmd, []string{"ws/repo"}); err != nil {
			t.Error(err)
		}
	})
	var values []map[string]any
	if err := json.Unmarshal([]byte(legacy), &values); err != nil || len(values) != 1 || values[0]["branch"] != "feature" {
		t.Fatalf("legacy JSON changed: %q: %v", legacy, err)
	}

	prMeta = true
	got := captureOut(func() {
		if err := runPRList(prListCmd, []string{"ws/repo"}); err != nil {
			t.Error(err)
		}
	})
	var report prListReport
	if err := json.Unmarshal([]byte(got), &report); err != nil {
		t.Fatal(err)
	}
	if report.Query.Scope != "repository" || report.Query.Repository != "repo" || report.Query.Author != "any" || report.Query.AllRepositories || report.Count != 1 || !report.Pagination.Complete {
		t.Errorf("repository report = %+v", report)
	}
	if report.Items[0]["url"] != "https://bitbucket.org/ws/repo/pull-requests/7" {
		t.Errorf("missing repository URL fallback: %+v", report.Items[0])
	}
	if report.Items[0]["review_state"] != "unknown" || report.Summary.ByReview["unknown"] != 1 || report.Summary.DraftUnknown != 1 {
		t.Errorf("absent review or draft data presented as zero: %+v", report)
	}
}

func TestPRListTableShowsQueryAndCompleteness(t *testing.T) {
	oldFactory, oldAll, oldUser, oldState, oldMeta, oldFormat := prNewAPIClient, prAll, prUser, prState, prMeta, output.CurrentFormat
	t.Cleanup(func() {
		prNewAPIClient, prAll, prUser, prState, prMeta, output.CurrentFormat = oldFactory, oldAll, oldUser, oldState, oldMeta, oldFormat
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"values":[]}`)
	}))
	defer server.Close()
	prNewAPIClient = mockAPIClientFactory(server)
	prAll, prUser, prState, prMeta, output.CurrentFormat = false, "", "OPEN", false, output.FormatTable
	got := captureOut(func() {
		if err := runPRList(prListCmd, []string{"ws/repo"}); err != nil {
			t.Error(err)
		}
	})
	for _, want := range []string{"scope=ws/repo", "author=any", "state=OPEN", "drafts=all", "pages=1", "complete=true", "No pull requests found"} {
		if !strings.Contains(got, want) {
			t.Errorf("table output missing %q: %s", want, got)
		}
	}
}

func TestPRListTimeFiltersAreSentToBitbucket(t *testing.T) {
	oldFactory, oldAll, oldUser, oldState, oldMeta, oldCreated, oldUpdated, oldFormat := prNewAPIClient, prAll, prUser, prState, prMeta, prCreatedSince, prUpdatedSince, output.CurrentFormat
	t.Cleanup(func() {
		prNewAPIClient, prAll, prUser, prState, prMeta, prCreatedSince, prUpdatedSince, output.CurrentFormat = oldFactory, oldAll, oldUser, oldState, oldMeta, oldCreated, oldUpdated, oldFormat
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		states := r.URL.Query()["state"]
		if strings.Join(states, ",") != "OPEN,MERGED,DECLINED" {
			t.Errorf("state selection = %v", states)
		}
		wantQ := "created_on >= 2026-10-01T00:00:00Z AND updated_on >= 2026-10-31T12:00:00Z"
		if got := r.URL.Query().Get("q"); got != wantQ {
			t.Errorf("server-side filter = %q, want %q", got, wantQ)
		}
		fmt.Fprint(w, `{"values":[]}`)
	}))
	defer server.Close()
	prNewAPIClient = mockAPIClientFactory(server)
	prAll, prUser, prState, prMeta, output.CurrentFormat = false, "", "ALL", true, output.FormatJSON
	prCreatedSince, prUpdatedSince = "2026-10-01T01:00:00+01:00", "2026-10-31T12:00:00Z"
	got := captureOut(func() {
		if err := runPRList(prListCmd, []string{"ws/repo"}); err != nil {
			t.Error(err)
		}
	})
	var report prListReport
	if err := json.Unmarshal([]byte(got), &report); err != nil {
		t.Fatal(err)
	}
	if report.Query.State != "ALL" || report.Query.CreatedSince != "2026-10-01T00:00:00Z" || report.Query.UpdatedSince != "2026-10-31T12:00:00Z" {
		t.Errorf("filter metadata = %+v", report.Query)
	}
}

func TestParsePRSince(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	got, err := parsePRSince("36h", now)
	if err != nil || got != "2026-11-09T00:00:00Z" {
		t.Errorf("relative bound = %q, err = %v", got, err)
	}
	for _, bad := range []string{"0h", "-1h", "yesterday", "2026-11-10"} {
		if _, err := parsePRSince(bad, now); err == nil {
			t.Errorf("accepted invalid bound %q", bad)
		}
	}
}

func TestRepositoryPRListDoesNotPrintPartialReport(t *testing.T) {
	oldFactory, oldAll, oldUser, oldState, oldMeta, oldFormat := prNewAPIClient, prAll, prUser, prState, prMeta, output.CurrentFormat
	t.Cleanup(func() {
		prNewAPIClient, prAll, prUser, prState, prMeta, output.CurrentFormat = oldFactory, oldAll, oldUser, oldState, oldMeta, oldFormat
	})
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/page2" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		fmt.Fprintf(w, `{"size":2,"values":[{"id":1,"state":"OPEN"}],"next":%q}`, server.URL+"/page2")
	}))
	defer server.Close()
	prNewAPIClient = mockAPIClientFactory(server)
	prAll, prUser, prState, prMeta, output.CurrentFormat = false, "", "OPEN", true, output.FormatJSON
	var runErr error
	got := captureOut(func() { runErr = runPRList(prListCmd, []string{"ws/repo"}) })
	if runErr == nil || got != "" {
		t.Fatalf("printed partial result: error=%v output=%q", runErr, got)
	}
}
