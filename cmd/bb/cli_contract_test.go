package main

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Exercise real command flag annotations and hooks with isolated Cobra parsing
// state. Restore shared flag values because production commands use globals.
func contractCommand(t *testing.T, original *cobra.Command, initialize bool) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "bb", SilenceErrors: true, SilenceUsage: true}
	child := &cobra.Command{Use: original.Use, Args: original.Args, PreRunE: original.PreRunE, RunE: original.RunE}
	copyFlags := func(dst *pflag.FlagSet, src *pflag.FlagSet) {
		src.VisitAll(func(f *pflag.Flag) {
			value := f.Value.String()
			copied := *f
			copied.Changed = false
			dst.AddFlag(&copied)
			t.Cleanup(func() { _ = f.Value.Set(value) })
		})
	}
	copyFlags(root.PersistentFlags(), rootCmd.PersistentFlags())
	copyFlags(child.Flags(), original.Flags())
	if initialize {
		root.PersistentPreRunE = rootCmd.PersistentPreRunE
	}
	root.AddCommand(child)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	oldJSON, oldCSV, oldRepo, oldFormat := jsonOutput, csvOutput, repoOverride, output.CurrentFormat
	jsonOutput, csvOutput, repoOverride = false, false, ""
	t.Cleanup(func() {
		jsonOutput, csvOutput, repoOverride, output.CurrentFormat = oldJSON, oldCSV, oldRepo, oldFormat
	})
	return root
}

func TestCLIOutputDefaultAndFlagPrecedence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"values":[{"key":"EXAMPLE","value":"hello"}]}`)
	}))
	defer server.Close()
	oldFactory := variableNewAPIClient
	variableNewAPIClient = mockAPIClientFactory(server)
	defer func() { variableNewAPIClient = oldFactory }()
	oldDefault := viper.GetString("output_format")
	defer viper.Set("output_format", oldDefault)
	for _, tc := range []struct {
		name, configured string
		flags            []string
		want             output.Format
	}{
		{"saved JSON", "json", nil, output.FormatJSON},
		{"CSV overrides JSON", "json", []string{"--csv"}, output.FormatCSV},
		{"JSON overrides table", "table", []string{"--json"}, output.FormatJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viper.Set("output_format", tc.configured)
			cli := contractCommand(t, variableListCmd, true)
			cli.SetArgs(append([]string{"list", "ws/repo"}, tc.flags...))
			var err error
			got := captureOut(func() { err = cli.Execute() })
			if err != nil {
				t.Fatal(err)
			}
			if output.CurrentFormat != tc.want {
				t.Fatalf("format=%s, want %s", output.CurrentFormat, tc.want)
			}
			if tc.want == output.FormatJSON {
				var values []map[string]interface{}
				if err := json.Unmarshal([]byte(got), &values); err != nil || len(values) != 1 {
					t.Fatalf("JSON=%q, err=%v", got, err)
				}
			}
			if tc.want == output.FormatCSV {
				rows, err := csv.NewReader(strings.NewReader(got)).ReadAll()
				if err != nil || len(rows) != 2 || rows[0][0] != "KEY" {
					t.Fatalf("CSV=%q, err=%v", got, err)
				}
			}
		})
	}
	t.Run("conflicting flags", func(t *testing.T) {
		cli := contractCommand(t, variableListCmd, true)
		cli.SetArgs([]string{"list", "ws/repo", "--json", "--csv"})
		if err := cli.Execute(); err == nil {
			t.Fatal("expected mutually exclusive flag error")
		}
	})
}

func TestCLIEmptyLists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{"values":[]}`) }))
	defer server.Close()
	oldVar, oldSource, oldIssue, oldPR := variableNewAPIClient, sourceNewAPIClient, issueNewAPIClient, prNewAPIClient
	variableNewAPIClient, sourceNewAPIClient, issueNewAPIClient, prNewAPIClient = mockAPIClientFactory(server), mockAPIClientFactory(server), mockAPIClientFactory(server), mockAPIClientFactory(server)
	oldRef := sourceRef
	sourceRef = "main"
	defer func() {
		variableNewAPIClient, sourceNewAPIClient, issueNewAPIClient, prNewAPIClient = oldVar, oldSource, oldIssue, oldPR
		sourceRef = oldRef
	}()
	for _, tc := range []struct {
		cmd    *cobra.Command
		args   []string
		header string
	}{
		{variableListCmd, []string{"ws/repo"}, "KEY"},
		{sourceListCmd, []string{"ws/repo"}, "TYPE"},
		{issueListCmd, []string{"ws/repo"}, "ID"},
		{prCommentsCmd, []string{"ws/repo", "5"}, "ID"},
		{issueCommentsCmd, []string{"ws/repo", "5"}, "ID"},
		{prThreadsCmd, []string{"ws/repo", "5"}, "ID"},
	} {
		for _, format := range []string{"json", "csv"} {
			t.Run(tc.cmd.Name()+"/"+tc.header+"/"+format, func(t *testing.T) {
				cli := contractCommand(t, tc.cmd, false)
				output.SetFormat(format)
				cli.SetArgs(append([]string{tc.cmd.Name()}, tc.args...))
				var err error
				got := captureOut(func() { err = cli.Execute() })
				if err != nil {
					t.Fatal(err)
				}
				if format == "json" && strings.TrimSpace(got) != "[]" {
					t.Fatalf("empty JSON=%q", got)
				}
				if format == "csv" {
					rows, err := csv.NewReader(strings.NewReader(got)).ReadAll()
					if err != nil || len(rows) != 1 || rows[0][0] != tc.header {
						t.Fatalf("empty CSV=%q, err=%v", got, err)
					}
				}
			})
		}
	}
}

func TestCLICommentFlagValidation(t *testing.T) {
	oldFactory := prNewAPIClient
	defer func() { prNewAPIClient = oldFactory }()
	calls := 0
	prNewAPIClient = func() (*api.Client, error) { calls++; return nil, nil }
	for _, flags := range [][]string{
		{"--reply-to", "-1"}, {"--reply-to", "0"}, {"--line", "10"},
		{"--path", "main.go", "--line", "0"},
		{"--reply-to", "12", "--path", "main.go"}, {"--reply-to", "12", "--line", "10"},
	} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			cli := contractCommand(t, prCommentCmd, false)
			cli.SetArgs(append([]string{"comment", "ws/repo", "5", "--body", "hi"}, flags...))
			if err := cli.Execute(); err == nil {
				t.Fatal("expected flag validation error")
			}
		})
	}
	if calls != 0 {
		t.Fatalf("invalid flags reached API client %d times", calls)
	}
}

func TestCLIBulkIDsUseInferredRepo(t *testing.T) {
	oldDetect := detectRepo
	detectRepo = func() (string, string, error) { return "ws", "repo", nil }
	defer func() { detectRepo = oldDetect }()
	paths := make(chan string, 6)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	oldIssue, oldPR := issueNewAPIClient, prNewAPIClient
	issueNewAPIClient, prNewAPIClient = mockAPIClientFactory(server), mockAPIClientFactory(server)
	defer func() { issueNewAPIClient, prNewAPIClient = oldIssue, oldPR }()
	for _, tc := range []struct {
		cmd   *cobra.Command
		flags []string
	}{{issueBulkCmd, []string{"--state", "resolved"}}, {prBulkApproveCmd, nil}} {
		t.Run(tc.cmd.Name(), func(t *testing.T) {
			cli := contractCommand(t, tc.cmd, false)
			cli.SetArgs(append([]string{tc.cmd.Name(), "1", "2", "3"}, tc.flags...))
			if err := cli.Execute(); err != nil {
				t.Fatal(err)
			}
		})
	}
	close(paths)
	count := 0
	for path := range paths {
		count++
		if !strings.HasPrefix(path, "/repositories/ws/repo/") {
			t.Errorf("wrong target %s", path)
		}
	}
	if count != 6 {
		t.Fatalf("requests=%d", count)
	}
}

func TestCLISourcePathAndVariableInference(t *testing.T) {
	oldDetect := detectRepo
	detectRepo = func() (string, string, error) { return "ws", "repo", nil }
	defer func() { detectRepo = oldDetect }()
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{"values":[]}`)
	}))
	defer server.Close()
	oldSource, oldVar := sourceNewAPIClient, variableNewAPIClient
	sourceNewAPIClient, variableNewAPIClient = mockAPIClientFactory(server), mockAPIClientFactory(server)
	oldRef := sourceRef
	sourceRef = "main"
	defer func() { sourceNewAPIClient, variableNewAPIClient = oldSource, oldVar; sourceRef = oldRef }()
	t.Run("source explicit path", func(t *testing.T) {
		cli := contractCommand(t, sourceListCmd, false)
		cli.SetArgs([]string{"list", "--path", "src/nested"})
		if err := cli.Execute(); err != nil {
			t.Fatal(err)
		}
		if gotPath != "/repositories/ws/repo/src/main/src/nested" {
			t.Fatalf("path=%s", gotPath)
		}
	})
	t.Run("variable inferred repo", func(t *testing.T) {
		cli := contractCommand(t, variableListCmd, false)
		cli.SetArgs([]string{"list"})
		if err := cli.Execute(); err != nil {
			t.Fatal(err)
		}
		if gotPath != "/repositories/ws/repo/pipelines_config/variables/" {
			t.Fatalf("path=%s", gotPath)
		}
	})
}

func TestPRThreadsRetainDeletedAncestors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"values":[{"id":1,"deleted":true,"content":{"raw":"old secret text"}},{"id":2,"parent":{"id":1},"content":{"raw":"live reply"}},{"id":3,"parent":{"id":99},"content":{"raw":"orphan reply"}}]}`)
	}))
	defer server.Close()
	oldFactory, oldFormat := prNewAPIClient, output.CurrentFormat
	prNewAPIClient = mockAPIClientFactory(server)
	output.SetFormat("table")
	defer func() { prNewAPIClient = oldFactory; output.CurrentFormat = oldFormat }()
	var err error
	got := captureOut(func() { err = runPRThreads(prThreadsCmd, []string{"ws/repo", "5"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[deleted]", "live reply", "orphan reply"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "old secret text") {
		t.Fatalf("deleted text printed: %q", got)
	}
}
