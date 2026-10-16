package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

func TestBulkInvalidBatchNeverAuthenticates(t *testing.T) {
	oldPR, oldIssue, oldState := prNewAPIClient, issueNewAPIClient, issueBulkState
	defer func() { prNewAPIClient, issueNewAPIClient, issueBulkState = oldPR, oldIssue, oldState }()
	calls := 0
	factory := func() (*api.Client, error) {
		calls++
		return nil, fmt.Errorf("unexpected authentication")
	}
	prNewAPIClient, issueNewAPIClient, issueBulkState = factory, factory, "resolved"
	for _, ids := range [][]string{
		{"1", "bad", "3"}, {"1", "0"}, {"1", "-2"}, {"1", "+2"},
		{"1", "999999999999999999999999"}, {"1", "01"}, {"1", "1"},
	} {
		args := append([]string{"ws/repo"}, ids...)
		for _, action := range []string{"approve", "decline", "merge"} {
			if err := runPRBulk(prBulkMergeCmd, args, action); err == nil {
				t.Errorf("accepted %s batch %v", action, ids)
			}
		}
		if err := runIssueBulk(issueBulkCmd, args); err == nil {
			t.Errorf("accepted issue batch %v", ids)
		}
	}
	issueBulkState = " "
	if err := runIssueBulk(issueBulkCmd, []string{"ws/repo", "1"}); err == nil {
		t.Error("accepted empty state")
	}
	if err := runPRBulk(prBulkApproveCmd, []string{"ws/repo", "1"}, "unknown"); err == nil {
		t.Error("accepted unknown action")
	}
	if calls != 0 {
		t.Fatalf("invalid batches authenticated %d times", calls)
	}
}

func TestBulkDryRunPlansWithoutAuthentication(t *testing.T) {
	oldPR, oldIssue := prNewAPIClient, issueNewAPIClient
	defer func() { prNewAPIClient, issueNewAPIClient = oldPR, oldIssue }()
	prNewAPIClient = func() (*api.Client, error) {
		t.Error("dry run authenticated")
		return nil, fmt.Errorf("unexpected authentication")
	}
	issueNewAPIClient = prNewAPIClient
	for _, tc := range []struct {
		cmd   *cobra.Command
		flags []string
	}{
		{prBulkApproveCmd, nil}, {prBulkDeclineCmd, nil},
		{prBulkMergeCmd, []string{"--strategy", "squash", "--close-source-branch=false"}},
		{issueBulkCmd, []string{"--state", "resolved"}},
	} {
		t.Run(tc.cmd.Name(), func(t *testing.T) {
			cli := contractCommand(t, tc.cmd, false)
			output.CurrentFormat = output.FormatJSON
			cli.SetArgs(append([]string{tc.cmd.Name(), "ws/repo", "1", "2", "--dry-run"}, tc.flags...))
			var err error
			text := captureOut(func() { err = cli.Execute() })
			if err != nil {
				t.Fatal(err)
			}
			var results []bulkResult
			if err := json.Unmarshal([]byte(text), &results); err != nil {
				t.Fatalf("invalid JSON %q: %v", text, err)
			}
			if len(results) != 2 || results[0].ID != "1" || results[1].ID != "2" {
				t.Fatalf("wrong targets: %+v", results)
			}
			for _, result := range results {
				if result.Status != "planned" || result.Repository != "ws/repo" || result.Error != "" {
					t.Fatalf("wrong dry-run result %+v", result)
				}
				if tc.cmd == prBulkMergeCmd && (result.Strategy != "squash" || result.CloseSourceBranch == nil || *result.CloseSourceBranch) {
					t.Fatalf("lost merge options: %+v", result)
				}
				if tc.cmd == issueBulkCmd && result.State != "resolved" {
					t.Fatalf("lost issue state: %+v", result)
				}
			}
		})
	}
}

func TestBulkPartialFailureStructuredOutput(t *testing.T) {
	oldPR, oldIssue, oldState, oldFormat := prNewAPIClient, issueNewAPIClient, issueBulkState, output.CurrentFormat
	defer func() {
		prNewAPIClient, issueNewAPIClient, issueBulkState, output.CurrentFormat = oldPR, oldIssue, oldState, oldFormat
	}()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if strings.Contains(r.URL.Path, "/2/") || strings.HasSuffix(r.URL.Path, "/2") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	prNewAPIClient, issueNewAPIClient, issueBulkState = mockAPIClientFactory(server), mockAPIClientFactory(server), "resolved"
	for _, format := range []output.Format{output.FormatJSON, output.FormatCSV} {
		for _, resource := range []string{"pr", "issue"} {
			t.Run(string(format)+"/"+resource, func(t *testing.T) {
				requests.Store(0)
				output.CurrentFormat = format
				var err error
				text := captureOut(func() {
					if resource == "pr" {
						err = runPRBulk(prBulkApproveCmd, []string{"ws/repo", "1", "2", "3"}, "approve")
					} else {
						err = runIssueBulk(issueBulkCmd, []string{"ws/repo", "1", "2", "3"})
					}
				})
				if err == nil || !strings.Contains(err.Error(), "1 of 3") || requests.Load() != 3 {
					t.Fatalf("partial failure: err=%v requests=%d", err, requests.Load())
				}
				if format == output.FormatJSON {
					var results []bulkResult
					if err := json.Unmarshal([]byte(text), &results); err != nil {
						t.Fatal(err)
					}
					if len(results) != 3 || results[0].ID != "1" || results[1].ID != "2" || results[2].ID != "3" || results[0].Status != "success" || results[1].Status != "failed" || results[2].Status != "success" || results[1].Error == "" {
						t.Fatalf("incorrect outcomes: %+v", results)
					}
				} else {
					rows, err := csv.NewReader(strings.NewReader(text)).ReadAll()
					if err != nil || len(rows) != 4 || rows[1][4] != "success" || rows[2][4] != "failed" || rows[3][4] != "success" || rows[2][8] == "" {
						t.Fatalf("incorrect CSV: %v %v", rows, err)
					}
				}
			})
		}
	}
}
