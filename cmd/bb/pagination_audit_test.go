package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

// Large JSON results can exceed pipe capacity; spool to a temporary file.
func capturePagedOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	old := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = old }()
	runErr := fn()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), runErr
}

func TestWorkspacePRListBeyondOneHundredAndPageFailure(t *testing.T) {
	oldFactory, oldUser, oldState, oldFormat := prNewAPIClient, prUser, prState, output.CurrentFormat
	defer func() {
		prNewAPIClient, prUser, prState, output.CurrentFormat = oldFactory, oldUser, oldState, oldFormat
	}()
	prUser, prState, output.CurrentFormat = "{self}", "MERGED", output.FormatJSON
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("second-page-fails=%v", fail), func(t *testing.T) {
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/page2" {
					if fail {
						w.WriteHeader(http.StatusForbidden)
						_, _ = io.WriteString(w, `{}`)
						return
					}
					_, _ = io.WriteString(w, `{"values":[{"id":101,"state":"MERGED"}]}`)
					return
				}
				if r.URL.Query().Get("state") != "MERGED" {
					t.Errorf("lost state filter: %s", r.URL)
				}
				values := make([]map[string]any, 100)
				for i := range values {
					values[i] = map[string]any{"id": i + 1, "state": "MERGED"}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"values": values, "next": server.URL + "/page2", "size": 101})
			}))
			defer server.Close()
			prNewAPIClient = mockAPIClientFactory(server)
			text, err := capturePagedOutput(t, func() error { return runPRListAll(prListCmd, []string{"ws"}) })
			if fail {
				if err == nil || text != "" {
					t.Fatalf("reported partial success: %v %q", err, text)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var values []map[string]any
			if err := json.Unmarshal([]byte(text), &values); err != nil {
				t.Fatal(err)
			}
			if len(values) != 101 || values[100]["id"] != float64(101) {
				t.Fatalf("truncated result: %d", len(values))
			}
		})
	}
}

func TestPageAdvertisedTotalCannotSilentlyTruncate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"values":[{"id":1}],"size":101}`)
	}))
	defer server.Close()
	client, err := mockAPIClientFactory(server)()
	if err != nil {
		t.Fatal(err)
	}
	first, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Body.Close()
	if _, err := fetchAllPages[pageItem](t.Context(), client, first); err == nil {
		t.Fatal("accepted a truncated response with an advertised larger total")
	}
}

func TestAuditedListCommandsFollowPagesAndFailWithoutPartialOutput(t *testing.T) {
	oldFormat := output.CurrentFormat
	output.CurrentFormat = output.FormatJSON
	defer func() { output.CurrentFormat = oldFormat }()
	factories := []*func() (*api.Client, error){&sshKeyNewAPIClient, &gpgKeyNewAPIClient, &projectNewAPIClient, &workspaceNewAPIClient, &deployNewAPIClient, &prNewAPIClient, &commitNewAPIClient}
	for _, ptr := range factories {
		old := *ptr
		defer func(p *func() (*api.Client, error), f func() (*api.Client, error)) { *p = f }(ptr, old)
	}
	cases := []struct {
		name string
		fn   func(*cobra.Command, []string) error
		cmd  *cobra.Command
		args []string
	}{
		{"ssh keys", runSSHKeyList, sshKeyListCmd, nil}, {"gpg keys", runGPGKeyList, gpgKeyListCmd, nil},
		{"project reviewers", runProjectReviewersList, projectReviewersListCmd, []string{"ws", "PROJ"}},
		{"project deploy keys", runProjectDeployKeyList, projectDeployKeyListCmd, []string{"ws", "PROJ"}},
		{"project permissions", runProjectPermissions, projectPermissionsCmd, []string{"ws", "PROJ"}},
		{"workspace hooks", runWorkspaceHookList, workspaceHookListCmd, []string{"ws"}},
		{"workspace variables", runWorkspaceVarList, workspaceVarListCmd, []string{"ws"}},
		{"deployment variables", runDeployVarList, deployVarListCmd, []string{"ws/repo", "env"}},
		{"PR tasks", runPRTasksList, prTasksListCmd, []string{"ws/repo", "1"}},
		{"commit reports", runCommitReport, commitReportCmd, []string{"ws/repo", "hash"}},
	}
	for _, tc := range cases {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fail=%v", tc.name, fail), func(t *testing.T) {
				var server *httptest.Server
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/user" {
						_, _ = io.WriteString(w, `{"uuid":"{self}"}`)
						return
					}
					if r.URL.Path == "/page2" {
						if fail {
							w.WriteHeader(http.StatusForbidden)
							_, _ = io.WriteString(w, `{}`)
							return
						}
						_, _ = io.WriteString(w, `{"values":[{"id":2,"uuid":"two","name":"two"}]}`)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"values": []map[string]any{{"id": 1, "uuid": "one", "name": "one"}}, "next": server.URL + "/page2"})
				}))
				defer server.Close()
				for _, ptr := range factories {
					*ptr = mockAPIClientFactory(server)
				}
				text, err := capturePagedOutput(t, func() error { return tc.fn(tc.cmd, tc.args) })
				if fail {
					if err == nil || text != "" {
						t.Fatalf("partial success: %v %q", err, text)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				var values []map[string]any
				if err := json.Unmarshal([]byte(text), &values); err != nil {
					t.Fatal(err)
				}
				if len(values) != 2 {
					t.Fatalf("truncated result: %d", len(values))
				}
			})
		}
	}
}
