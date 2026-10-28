package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

// Newest first, as Bitbucket returns it: opened at aaa, approved by Ann,
// changes requested by Bob, pushed bbb, approved by Bob, merged at bbb.
const prActivityPage1 = `{"values":[
	{"update":{"date":"2026-01-05T10:00:00.000000+00:00","state":"MERGED","author":{"display_name":"Tom","uuid":"{t}"},"source":{"commit":{"hash":"bbb"}}},"pull_request":{"id":5}},
	{"approval":{"date":"2026-01-04T10:00:00.000000+00:00","user":{"display_name":"Bob","uuid":"{b}"}},"pull_request":{"id":5}},
	{"update":{"date":"2026-01-03T10:00:00.000000+00:00","state":"OPEN","author":{"display_name":"Tom","uuid":"{t}"},"source":{"commit":{"hash":"bbb"}}},"pull_request":{"id":5}}
],"next":"%s/page2"}`

const prActivityPage2 = `{"values":[
	{"comment":{"id":9,"created_on":"2026-01-02T12:00:00+00:00","user":{"display_name":"Bob"},"content":{"raw":"please fix\nmore"}},"pull_request":{"id":5}},
	{"changes_requested":{"date":"2026-01-02T11:00:00.000000+00:00","user":{"display_name":"Bob","uuid":"{b}"}},"pull_request":{"id":5}},
	{"approval":{"date":"2026-01-02T10:00:00.000000+00:00","user":{"display_name":"Ann","uuid":"{a}"}},"pull_request":{"id":5}},
	{"update":{"date":"2026-01-01T10:00:00.000000+00:00","state":"OPEN","author":{"display_name":"Tom","uuid":"{t}"},"source":{"commit":{"hash":"aaa"}}},"pull_request":{"id":5}}
]}`

func prActivityServer(t *testing.T) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("activity is read-only, got %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Path == "/page2" {
			_, _ = io.WriteString(w, prActivityPage2)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/repositories/myws/myrepo/pullrequests/5/activity") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, strings.Replace(prActivityPage1, "%s", server.URL, 1))
	}))
	return server
}

func TestRunPRActivity_JSONAcrossPages(t *testing.T) {
	oldFactory, oldFormat := prNewAPIClient, output.CurrentFormat
	defer func() { prNewAPIClient, output.CurrentFormat = oldFactory, oldFormat }()
	server := prActivityServer(t)
	defer server.Close()
	prNewAPIClient = mockAPIClientFactory(server)
	output.CurrentFormat = output.FormatJSON

	var err error
	got := captureOut(func() { err = runPRActivity(prActivityCmd, []string{"myws/myrepo", "5"}) })
	if err != nil {
		t.Fatal(err)
	}
	var events []map[string]any
	if err := json.Unmarshal([]byte(got), &events); err != nil {
		t.Fatalf("JSON=%q, err=%v", got, err)
	}
	want := []struct {
		event, actor, commit string
		headMoved, stale     any
	}{
		{"update", "Tom", "aaa", nil, nil},
		{"approval", "Ann", "aaa", nil, true},
		{"changes_requested", "Bob", "aaa", nil, true},
		{"comment", "Bob", "", nil, nil},
		{"update", "Tom", "bbb", true, nil},
		{"approval", "Bob", "bbb", nil, false},
		{"update", "Tom", "bbb", false, nil},
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d: %s", len(events), len(want), got)
	}
	for i, w := range want {
		e := events[i]
		if e["event"] != w.event || e["actor"] != w.actor || (e["commit"] != nil || w.commit != "") && e["commit"] != w.commit ||
			e["head_moved"] != w.headMoved || e["stale"] != w.stale {
			t.Errorf("event %d = %v, want %+v", i, e, w)
		}
	}
	if events[1]["actor_uuid"] != "{a}" || events[1]["date"] != "2026-01-02T10:00:00.000000+00:00" {
		t.Errorf("approval lost actor or timestamp: %v", events[1])
	}
	if events[3]["comment_id"] != float64(9) || events[3]["detail"] != "please fix" {
		t.Errorf("comment = %v", events[3])
	}
	if events[6]["state"] != "MERGED" {
		t.Errorf("merge update = %v", events[6])
	}
}

func TestRunPRActivity_CSV(t *testing.T) {
	oldFactory, oldFormat := prNewAPIClient, output.CurrentFormat
	defer func() { prNewAPIClient, output.CurrentFormat = oldFactory, oldFormat }()
	server := prActivityServer(t)
	defer server.Close()
	prNewAPIClient = mockAPIClientFactory(server)
	output.CurrentFormat = output.FormatCSV

	var err error
	got := captureOut(func() { err = runPRActivity(prActivityCmd, []string{"myws/myrepo", "5"}) })
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(got)).ReadAll()
	if err != nil || len(rows) != 8 {
		t.Fatalf("CSV=%q, err=%v", got, err)
	}
	if strings.Join(rows[0], ",") != "DATE,EVENT,ACTOR,COMMIT,HEAD_MOVED,STALE,DETAIL" {
		t.Errorf("header = %v", rows[0])
	}
	if strings.Join(rows[2][1:], ",") != "approval,Ann,aaa,,true," {
		t.Errorf("stale approval row = %v", rows[2])
	}
	if strings.Join(rows[5][1:], ",") != "update,Tom,bbb,true,,OPEN" {
		t.Errorf("head move row = %v", rows[5])
	}
}

func TestRunPRActivity_Table(t *testing.T) {
	oldFactory, oldFormat := prNewAPIClient, output.CurrentFormat
	defer func() { prNewAPIClient, output.CurrentFormat = oldFactory, oldFormat }()
	server := prActivityServer(t)
	defer server.Close()
	prNewAPIClient = mockAPIClientFactory(server)
	output.CurrentFormat = output.FormatTable

	var err error
	got := captureOut(func() { err = runPRActivity(prActivityCmd, []string{"myws/myrepo", "5"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"HEAD MOVED", "changes_requested", "please fix"} {
		if !strings.Contains(got, s) {
			t.Errorf("table missing %q:\n%s", s, got)
		}
	}
}

func TestRunPRActivity_PageFailure(t *testing.T) {
	oldFactory, oldFormat := prNewAPIClient, output.CurrentFormat
	defer func() { prNewAPIClient, output.CurrentFormat = oldFactory, oldFormat }()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/page2" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = io.WriteString(w, strings.Replace(prActivityPage1, "%s", server.URL, 1))
	}))
	defer server.Close()
	prNewAPIClient = mockAPIClientFactory(server)
	output.CurrentFormat = output.FormatJSON

	var err error
	got := captureOut(func() { err = runPRActivity(prActivityCmd, []string{"myws/myrepo", "5"}) })
	if err == nil || got != "" {
		t.Fatalf("reported partial history: %v %q", err, got)
	}
}

func TestRunPRActivity_NoClient(t *testing.T) {
	defer func() { prNewAPIClient = api.NewClient }()
	prNewAPIClient = func() (*api.Client, error) { return nil, errors.New("auth required") }
	if err := runPRActivity(prActivityCmd, []string{"myws/myrepo", "5"}); err == nil {
		t.Error("expected error when client creation fails")
	}
}

func TestRunPRActivity_BadNumber(t *testing.T) {
	defer func() { prNewAPIClient = api.NewClient }()
	prNewAPIClient = func() (*api.Client, error) { return &api.Client{}, nil }
	if err := runPRActivity(prActivityCmd, []string{"myws/myrepo", "abc"}); err == nil {
		t.Error("expected error for non-numeric pr-number")
	}
}

func TestBuildPRActivity_UnknownAndHeadless(t *testing.T) {
	var entries []prActivityEntry
	if err := json.Unmarshal([]byte(`[
		{"approval":{"date":"2026-01-02T00:00:00Z","user":{"display_name":"Ann"}},"pull_request":{"id":1}},
		{"future_event":{"date":"2026-01-01T00:00:00Z","user":{"display_name":"Bob"}},"pull_request":{"id":1}}
	]`), &entries); err != nil {
		t.Fatal(err)
	}
	events := buildPRActivity(entries)
	if len(events) != 2 {
		t.Fatalf("events = %+v", events)
	}
	if e := events[0]; e.Event != "future_event" || e.Actor != "Bob" || e.Date != "2026-01-01T00:00:00Z" {
		t.Errorf("unknown entry = %+v", e)
	}
	// With no update to establish a head, staleness is unknown rather than false.
	if e := events[1]; e.Event != "approval" || e.Commit != "" || e.Stale != nil {
		t.Errorf("headless approval = %+v", e)
	}
}
