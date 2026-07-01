package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
)

func TestRunIssueNamedList(t *testing.T) {
	defer func() { issueNewAPIClient = api.NewClient }()

	for _, kind := range []string{"milestones", "versions", "components"} {
		var gotPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"values":[{"id":1,"name":"v1"}]}`))
		}))

		issueNewAPIClient = mockAPIClientFactory(server)
		if err := runIssueNamedList(issueMilestonesCmd, []string{"myws/myrepo"}, kind); err != nil {
			t.Errorf("runIssueNamedList(%s) error: %v", kind, err)
		}
		if !strings.HasSuffix(gotPath, "/"+kind) {
			t.Errorf("%s: path = %q, want suffix /%s", kind, gotPath, kind)
		}
		server.Close()
	}
}

func TestIssueMetaCommandsRegistered(t *testing.T) {
	for _, name := range []string{"milestones", "versions", "components"} {
		found := false
		for _, c := range issueCmd.Commands() {
			if c.Name() == name {
				found = true
			}
		}
		if !found {
			t.Errorf("issue missing subcommand %q", name)
		}
	}
}
