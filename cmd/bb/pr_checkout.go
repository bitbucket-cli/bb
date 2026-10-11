package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/bitbucket-cli/bb/internal/git"
	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

// prTemplatePaths are searched (relative to the working directory) for a default
// pull request description when --body is not given.
var prTemplatePaths = []string{
	".bitbucket/pull_request_template.md",
	"pull_request_template.md",
	".github/pull_request_template.md",
	"docs/pull_request_template.md",
}

// loadPRTemplate returns the contents of the first PR template found under dir,
// or "" when none exists.
func loadPRTemplate(dir string) string {
	for _, p := range prTemplatePaths {
		if data, err := os.ReadFile(filepath.Join(dir, p)); err == nil {
			return string(data)
		}
	}
	return ""
}

// Mockable git operations for testing.
var (
	prCheckoutDetect   = git.DetectRepo
	prCheckoutFetch    = git.Fetch
	prCheckoutCheckout = git.Checkout
	prCurrentBranch    = git.CurrentBranch
)

var prCheckoutCmd = &cobra.Command{
	Use:   "checkout <pr-number> [workspace/repo]",
	Short: "Check out a pull request branch locally",
	Long: `Fetch and check out the source branch of a pull request.

The repository is auto-detected from the current git remote (origin) when the
workspace/repo argument is omitted.

Example:
  bb pr checkout 42`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runPrCheckout,
}

func init() {
	prCmd.AddCommand(prCheckoutCmd)
}

func runPrCheckout(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	prNum, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid pr-number %q", args[0])
	}

	repoCtx, err := resolvePrRepo(args)
	if err != nil {
		return err
	}

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugPullrequestsPullRequestId(ctx, repoCtx.Workspace, repoCtx.RepoSlug, prNum)
	if err != nil {
		return fmt.Errorf("failed to fetch pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var pr struct {
		Source struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
		} `json:"source"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	branch := pr.Source.Branch.Name
	if branch == "" {
		return fmt.Errorf("could not determine the source branch for PR %d", prNum)
	}

	if err := prCheckoutFetch("origin", branch); err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}
	if err := prCheckoutCheckout(branch); err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}

	output.Success("Checked out %s (PR #%d)", branch, prNum)
	return nil
}

// resolvePrRepo resolves the repo context from the optional positional argument,
// falling back to auto-detecting it from the current git remote.
func resolvePrRepo(args []string) (*api.RepoContext, error) {
	if len(args) >= 2 {
		return api.ParseRepoContext(args[1])
	}
	ws, repo, err := prCheckoutDetect()
	if err != nil {
		return nil, err
	}
	return &api.RepoContext{Workspace: ws, RepoSlug: repo}, nil
}
