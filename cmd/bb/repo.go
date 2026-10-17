package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/bitbucket-cli/bb/generated/bitbucket"
	"github.com/bitbucket-cli/bb/internal/config"
	internalgit "github.com/bitbucket-cli/bb/internal/git"
	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	repoLimit             int
	repoAll               bool
	repoCreateDescription string
	repoCreatePrivate     bool
	repoCreatePublic      bool
	repoCreateProject     string
	repoCreateSCM         string
	repoCreateHasIssues   bool
	repoCreateNoIssues    bool
	repoCreateHasWiki     bool
	repoCreateNoWiki      bool
	repoCreateForkPolicy  string
	repoDescription       string
	repoPrivate           bool
	repoPublic            bool
	repoHasIssues         bool
	repoNoIssues          bool
	repoHasWiki           bool
	repoNoWiki            bool
	repoForkPolicy        string
)

// Mockable dependencies for testing
var (
	newAPIClient   = api.NewClient
	runGitClone    = runGitCloneReal
	getWorkspace   = config.GetWorkspace
	repoAPIBaseURL = "https://api.bitbucket.org/2.0"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories",
	Long:  `List, view, create, clone, and update Bitbucket repositories.`,
}

var repoListCmd = &cobra.Command{
	Use:   "list [workspace]",
	Short: "List repositories",
	Long: `List repositories in a workspace.

If no workspace is specified, uses the default workspace from config or environment.`,
	RunE: runRepoList,
}

var repoViewCmd = &cobra.Command{
	Use:   "view [workspace/repo]",
	Short: "View repository details",
	Long:  `Display detailed information about a repository.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRepoView,
}

var repoCloneCmd = &cobra.Command{
	Use:   "clone [workspace/repo] [directory]",
	Short: "Clone a repository",
	Long:  `Clone a Bitbucket repository to your local machine.`,
	Args:  cobra.RangeArgs(0, 2),
	RunE:  runRepoClone,
}

var repoCreateCmd = &cobra.Command{
	Use:   "create [workspace/repo]",
	Short: "Create a repository",
	Long: `Create a new Bitbucket repository.

Example:
  bb repo create myworkspace/myrepo --description "My new repo" --private`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRepoCreate,
}

var repoUpdateCmd = &cobra.Command{
	Use:   "update [workspace/repo]",
	Short: "Update repository settings",
	Long: `Update repository settings.

Example:
  bb repo update myworkspace/myrepo --description "New description" --private`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRepoUpdate,
}

var repoReviewersCmd = &cobra.Command{
	Use:   "reviewers",
	Short: "Manage default reviewers",
	Long:  `List, add, and remove default reviewers for a repository.`,
}

var repoReviewersListCmd = &cobra.Command{
	Use:   "list [workspace/repo]",
	Short: "List default reviewers",
	Long:  `List all default reviewers configured for a repository.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRepoReviewersList,
}

var repoReviewersAddCmd = &cobra.Command{
	Use:   "add [workspace/repo] <username>",
	Short: "Add a default reviewer",
	Long:  `Add a user as a default reviewer for a repository.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runRepoReviewersAdd,
}

var repoReviewersRemoveCmd = &cobra.Command{
	Use:   "remove [workspace/repo] <username>",
	Short: "Remove a default reviewer",
	Long:  `Remove a user from the default reviewers of a repository.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runRepoReviewersRemove,
}

func init() {
	repoListCmd.Flags().IntVarP(&repoLimit, "limit", "l", 0, "Maximum number of repositories to list")
	repoListCmd.Flags().BoolVarP(&repoAll, "all", "a", false, "Fetch all repositories (may be slow for large workspaces)")

	// Create flags
	repoCreateCmd.Flags().StringVarP(&repoCreateDescription, "description", "d", "", "Repository description")
	repoCreateCmd.Flags().BoolVar(&repoCreatePrivate, "private", false, "Make repository private")
	repoCreateCmd.Flags().BoolVar(&repoCreatePublic, "public", false, "Make repository public")
	repoCreateCmd.Flags().StringVarP(&repoCreateProject, "project", "p", "", "Project key or UUID")
	repoCreateCmd.Flags().StringVar(&repoCreateSCM, "scm", "git", "Source control manager")
	repoCreateCmd.Flags().BoolVar(&repoCreateHasIssues, "enable-issues", false, "Enable issue tracker")
	repoCreateCmd.Flags().BoolVar(&repoCreateNoIssues, "disable-issues", false, "Disable issue tracker")
	repoCreateCmd.Flags().BoolVar(&repoCreateHasWiki, "enable-wiki", false, "Enable wiki")
	repoCreateCmd.Flags().BoolVar(&repoCreateNoWiki, "disable-wiki", false, "Disable wiki")
	repoCreateCmd.Flags().StringVarP(&repoCreateForkPolicy, "fork-policy", "f", "", "Fork policy (allow_forks, no_public_forks, no_forks)")

	// Update flags
	repoUpdateCmd.Flags().StringVarP(&repoDescription, "description", "d", "", "Repository description")
	repoUpdateCmd.Flags().BoolVar(&repoPrivate, "private", false, "Make repository private")
	repoUpdateCmd.Flags().BoolVar(&repoPublic, "public", false, "Make repository public")
	repoUpdateCmd.Flags().BoolVar(&repoHasIssues, "enable-issues", false, "Enable issue tracker")
	repoUpdateCmd.Flags().BoolVar(&repoNoIssues, "disable-issues", false, "Disable issue tracker")
	repoUpdateCmd.Flags().BoolVar(&repoHasWiki, "enable-wiki", false, "Enable wiki")
	repoUpdateCmd.Flags().BoolVar(&repoNoWiki, "disable-wiki", false, "Disable wiki")
	repoUpdateCmd.Flags().StringVarP(&repoForkPolicy, "fork-policy", "f", "", "Fork policy (allow_forks, no_public_forks, no_forks)")

	// Reviewers subcommands
	repoReviewersCmd.AddCommand(repoReviewersListCmd)
	repoReviewersCmd.AddCommand(repoReviewersAddCmd)
	repoReviewersCmd.AddCommand(repoReviewersRemoveCmd)

	repoCmd.AddCommand(repoListCmd)
	repoCmd.AddCommand(repoViewCmd)
	repoCmd.AddCommand(repoCreateCmd)
	repoCmd.AddCommand(repoCloneCmd)
	repoCmd.AddCommand(repoUpdateCmd)
	repoCmd.AddCommand(repoReviewersCmd)
}

func runRepoList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := newAPIClient()
	if err != nil {
		return err
	}

	workspace := ""
	if len(args) > 0 {
		workspace = args[0]
	} else {
		workspace = getWorkspace()
	}

	if workspace == "" {
		return fmt.Errorf("workspace required: specify as argument or set BB_WORKSPACE")
	}

	// Collect repositories
	var allRepos []bitbucket.Repository

	// First request - use pagelen=100 via query string in the URL
	firstURL := fmt.Sprintf("%s/repositories/%s?pagelen=100", repoAPIBaseURL, workspace)
	resp, err := fetchPage(ctx, client, firstURL)
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result bitbucket.PaginatedRepositories
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		resp.Body.Close()
		return fmt.Errorf("failed to parse response: %w", err)
	}
	resp.Body.Close()

	// Get total count if available
	totalCount := 0
	if result.Size != nil {
		totalCount = *result.Size
	}

	if result.Values != nil {
		allRepos = append(allRepos, *result.Values...)
	}

	// Determine if we should fetch more pages
	shouldPaginate := repoAll || repoLimit > 0
	hasMoreResults := result.Next != nil && *result.Next != ""

	if shouldPaginate && hasMoreResults {
		// Determine target count
		targetCount := totalCount
		if repoLimit > 0 && repoLimit < totalCount {
			targetCount = repoLimit
		}

		// Show progress if we need to fetch more pages
		if totalCount > 0 && len(allRepos) < targetCount && result.Next != nil {
			fmt.Fprintf(os.Stderr, "Fetching repositories: %d/%d\r", len(allRepos), targetCount)
		}

		// Follow pagination links
		for result.Next != nil && *result.Next != "" {
			// Check if we've hit the limit
			if repoLimit > 0 && len(allRepos) >= repoLimit {
				break
			}

			nextURL := *result.Next
			nextResp, err := fetchPage(ctx, client, nextURL)
			if err != nil {
				return fmt.Errorf("failed to fetch next page: %w", err)
			}
			if nextResp.StatusCode != http.StatusOK {
				nextResp.Body.Close()
				return fmt.Errorf("API error fetching next page: %s", nextResp.Status)
			}

			// Reset result before decoding - JSON decoder doesn't zero fields
			// that are absent in the response, which would cause infinite loop
			// if the last page doesn't include a "next" field
			result = bitbucket.PaginatedRepositories{}
			if err := json.NewDecoder(nextResp.Body).Decode(&result); err != nil {
				nextResp.Body.Close()
				return fmt.Errorf("failed to parse response: %w", err)
			}
			nextResp.Body.Close()

			if result.Values != nil {
				allRepos = append(allRepos, *result.Values...)
			}

			// Update progress
			if targetCount > 0 {
				current := len(allRepos)
				if current > targetCount {
					current = targetCount
				}
				fmt.Fprintf(os.Stderr, "Fetching repositories: %d/%d\r", current, targetCount)
			}
		}

		// Clear progress line
		if totalCount > 0 && len(allRepos) > 100 {
			fmt.Fprintf(os.Stderr, "\033[2K\r") // Clear line
		}

	}
	if repoLimit > 0 && len(allRepos) > repoLimit {
		allRepos = allRepos[:repoLimit]
	}

	if len(allRepos) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No repositories found in workspace '%s'", workspace)
		return nil
	}

	// Build output
	headers := []string{"NAME", "VISIBILITY", "LANGUAGE", "UPDATED"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, repo := range allRepos {
		name := ""
		if repo.FullName != nil {
			name = *repo.FullName
		}

		visibility := "public"
		if repo.IsPrivate != nil && *repo.IsPrivate {
			visibility = "private"
		}

		language := ""
		if repo.Language != nil {
			language = *repo.Language
		}

		updated := ""
		if repo.UpdatedOn != nil {
			updated = repo.UpdatedOn.Format("2006-01-02")
		}

		rows = append(rows, []string{name, visibility, language, updated})
		jsonData = append(jsonData, map[string]interface{}{
			"full_name":  name,
			"is_private": repo.IsPrivate,
			"language":   language,
			"updated_on": updated,
			"uuid":       repo.Uuid,
		})
	}

	if err := output.Print(headers, rows, jsonData); err != nil {
		return err
	}

	// Show message if there are more results not displayed
	if !shouldPaginate && hasMoreResults {
		if totalCount > 0 {
			fmt.Fprintf(os.Stderr, "\nShowing %d of %d repositories. Use --all to fetch all.\n", len(allRepos), totalCount)
		} else {
			fmt.Fprintf(os.Stderr, "\nShowing the first %d repositories. Use --all to fetch all.\n", len(allRepos))
		}
	}

	return nil
}

// fetchPage fetches a page of results using the provided URL (with auth).
func fetchPage(ctx context.Context, client *api.Client, pageURL string) (*http.Response, error) {
	return client.GetURL(ctx, pageURL)
}

func runRepoView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := newAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlug(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var repo bitbucket.Repository
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract clone URLs
	var cloneSSH, cloneHTTPS string
	if repo.Links != nil && repo.Links.Clone != nil {
		for _, link := range *repo.Links.Clone {
			if link.Name != nil && link.Href != nil {
				switch *link.Name {
				case "ssh":
					cloneSSH = *link.Href
				case "https":
					cloneHTTPS = *link.Href
				}
			}
		}
	}

	mainBranch := ""
	if repo.Mainbranch != nil && repo.Mainbranch.Name != nil {
		mainBranch = *repo.Mainbranch.Name
	}

	if output.CurrentFormat == output.FormatJSON {
		return output.JSON(repo)
	}

	// Table output
	name := ""
	if repo.FullName != nil {
		name = *repo.FullName
	}

	description := ""
	if repo.Description != nil {
		description = *repo.Description
	}

	visibility := "public"
	if repo.IsPrivate != nil && *repo.IsPrivate {
		visibility = "private"
	}

	language := ""
	if repo.Language != nil {
		language = *repo.Language
	}

	fmt.Printf("Name:        %s\n", name)
	if description != "" {
		fmt.Printf("Description: %s\n", description)
	}
	fmt.Printf("Visibility:  %s\n", visibility)
	if language != "" {
		fmt.Printf("Language:    %s\n", language)
	}
	if mainBranch != "" {
		fmt.Printf("Main branch: %s\n", mainBranch)
	}
	if cloneSSH != "" {
		fmt.Printf("Clone SSH:   %s\n", cloneSSH)
	}
	if cloneHTTPS != "" {
		fmt.Printf("Clone HTTPS: %s\n", cloneHTTPS)
	}
	if repo.UpdatedOn != nil {
		fmt.Printf("Updated:     %s\n", repo.UpdatedOn.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func runRepoClone(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := newAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 0)
	if err != nil {
		return err
	}

	// Get repository to find clone URL
	resp, err := client.GetRepositoriesWorkspaceRepoSlug(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var repo bitbucket.Repository
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Find SSH clone URL (prefer SSH over HTTPS)
	var cloneURL string
	if repo.Links != nil && repo.Links.Clone != nil {
		for _, link := range *repo.Links.Clone {
			if link.Name != nil && link.Href != nil {
				if *link.Name == "ssh" {
					cloneURL = *link.Href
					break
				}
				if *link.Name == "https" && cloneURL == "" {
					cloneURL = *link.Href
				}
			}
		}
	}

	if cloneURL == "" {
		return fmt.Errorf("no clone URL found for repository")
	}

	// Determine target directory
	targetDir := ""
	if len(rest) > 0 {
		targetDir = rest[0]
	} else {
		targetDir = repoCtx.RepoSlug
	}

	output.Info("Cloning %s/%s into %s...", repoCtx.Workspace, repoCtx.RepoSlug, targetDir)

	if err := runGitClone(cloneURL, targetDir); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	output.Success("Repository cloned to %s", targetDir)
	return nil
}

func runRepoCreate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := newAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	if repoCreatePrivate && repoCreatePublic {
		return fmt.Errorf("cannot specify both --private and --public")
	}

	if repoCreateHasIssues && repoCreateNoIssues {
		return fmt.Errorf("cannot specify both --enable-issues and --disable-issues")
	}

	if repoCreateHasWiki && repoCreateNoWiki {
		return fmt.Errorf("cannot specify both --enable-wiki and --disable-wiki")
	}

	createReq := map[string]interface{}{
		"scm": repoCreateSCM,
	}

	if repoCreateDescription != "" {
		createReq["description"] = repoCreateDescription
	}

	if repoCreatePrivate {
		createReq["is_private"] = true
	} else if repoCreatePublic {
		createReq["is_private"] = false
	}

	if repoCreateHasIssues {
		createReq["has_issues"] = true
	} else if repoCreateNoIssues {
		createReq["has_issues"] = false
	}

	if repoCreateHasWiki {
		createReq["has_wiki"] = true
	} else if repoCreateNoWiki {
		createReq["has_wiki"] = false
	}

	if repoCreateProject != "" {
		projectField := "key"
		if strings.HasPrefix(repoCreateProject, "{") {
			projectField = "uuid"
		}
		createReq["project"] = map[string]interface{}{projectField: repoCreateProject}
	}

	if repoCreateForkPolicy != "" {
		createReq["fork_policy"] = repoCreateForkPolicy
	}

	body, err := json.Marshal(createReq)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugWithBody(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	var createdRepo bitbucket.Repository
	if err := json.NewDecoder(resp.Body).Decode(&createdRepo); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fullName := repoCtx.Workspace + "/" + repoCtx.RepoSlug
	if createdRepo.FullName != nil && *createdRepo.FullName != "" {
		fullName = *createdRepo.FullName
	}

	output.Success("Created repository %s", fullName)
	return nil
}

func runGitCloneReal(cloneURL, targetDir string) error {
	if err := internalgit.ValidateCloneURL(cloneURL); err != nil {
		return err
	}
	gitCmd := exec.Command("git", "clone", "--", cloneURL, targetDir)
	gitCmd.Stdout = os.Stdout
	gitCmd.Stderr = os.Stderr
	return gitCmd.Run()
}

func runRepoUpdate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := newAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	// Build update request
	updateReq := make(map[string]interface{})

	if repoDescription != "" {
		updateReq["description"] = repoDescription
	}

	if repoPrivate {
		updateReq["is_private"] = true
	} else if repoPublic {
		updateReq["is_private"] = false
	}

	if repoHasIssues {
		updateReq["has_issues"] = true
	} else if repoNoIssues {
		updateReq["has_issues"] = false
	}

	if repoHasWiki {
		updateReq["has_wiki"] = true
	} else if repoNoWiki {
		updateReq["has_wiki"] = false
	}

	if repoForkPolicy != "" {
		updateReq["fork_policy"] = repoForkPolicy
	}

	if len(updateReq) == 0 {
		return fmt.Errorf("no updates specified")
	}

	body, err := json.Marshal(updateReq)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	resp, err := client.PutRepositoriesWorkspaceRepoSlugWithBody(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to update repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Updated repository %s/%s", repoCtx.Workspace, repoCtx.RepoSlug)
	return nil
}

func runRepoReviewersList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := newAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugDefaultReviewers(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
	)
	if err != nil {
		return fmt.Errorf("failed to list default reviewers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	type defaultReviewer struct {
		UUID        string `json:"uuid"`
		DisplayName string `json:"display_name"`
		Nickname    string `json:"nickname"`
		AccountID   string `json:"account_id"`
	}

	reviewers, err := fetchAllPages[defaultReviewer](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(reviewers) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No default reviewers configured")
		return nil
	}

	headers := []string{"NAME", "NICKNAME", "ACCOUNT ID"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, reviewer := range reviewers {
		rows = append(rows, []string{
			reviewer.DisplayName,
			reviewer.Nickname,
			reviewer.AccountID,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"uuid":         reviewer.UUID,
			"display_name": reviewer.DisplayName,
			"nickname":     reviewer.Nickname,
			"account_id":   reviewer.AccountID,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runRepoReviewersAdd(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := newAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	username := rest[0]

	resp, err := client.PutRepositoriesWorkspaceRepoSlugDefaultReviewersTargetUsername(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		username,
	)
	if err != nil {
		return fmt.Errorf("failed to add default reviewer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Added %s as default reviewer", username)
	return nil
}

func runRepoReviewersRemove(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := newAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	username := rest[0]

	resp, err := client.DeleteRepositoriesWorkspaceRepoSlugDefaultReviewersTargetUsername(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		username,
	)
	if err != nil {
		return fmt.Errorf("failed to remove default reviewer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Removed %s from default reviewers", username)
	return nil
}
