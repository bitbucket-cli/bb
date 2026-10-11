package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	generated "github.com/bitbucket-cli/bb/generated/bitbucket"
	"github.com/bitbucket-cli/bb/internal/config"
	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	snippetTitle    string
	snippetPrivate  bool
	snippetFilename string
	snippetRole     string
)

// Mockable dependencies for testing
var (
	snippetNewAPIClient           = api.NewClient
	snippetStdin        io.Reader = os.Stdin
)

var snippetCmd = &cobra.Command{
	Use:     "snippet",
	Aliases: []string{"snippets"},
	Short:   "Manage snippets",
	Long:    `List, view, create, and delete workspace snippets.`,
}

var snippetListCmd = &cobra.Command{
	Use:   "list [workspace]",
	Short: "List snippets",
	Long: `List snippets in a workspace.

The workspace defaults to the configured workspace (bb config / BB_WORKSPACE)
when omitted. Use --role to filter by your relationship to the snippet.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSnippetList,
}

var snippetViewCmd = &cobra.Command{
	Use:   "view <workspace> <snippet-id>",
	Short: "View snippet details",
	Long:  `Display a snippet's metadata and the files it contains.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runSnippetView,
}

var snippetCreateCmd = &cobra.Command{
	Use:   "create [workspace] [files...]",
	Short: "Create a snippet",
	Long: `Create a snippet from one or more files, or from stdin.

Examples:
  bb snippet create myworkspace main.go util.go --title "helpers"
  cat notes.txt | bb snippet create myworkspace --filename notes.txt --private

The workspace defaults to the configured workspace when the first argument is a
file that exists. Pass --private to make the snippet private.`,
	Args: cobra.ArbitraryArgs,
	RunE: runSnippetCreate,
}

var snippetUpdateCmd = &cobra.Command{
	Use:   "update <workspace> <snippet-id> [files...]",
	Short: "Update a snippet",
	Long: `Update a snippet's files and/or title.

Replaces the snippet's contents with the given files, or with stdin via
--filename. Use --title to rename it.`,
	Args: cobra.MinimumNArgs(2),
	RunE: runSnippetUpdate,
}

var snippetDeleteCmd = &cobra.Command{
	Use:   "delete <workspace> <snippet-id>",
	Short: "Delete a snippet",
	Long:  `Delete a snippet from a workspace.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runSnippetDelete,
}

func init() {
	snippetListCmd.Flags().StringVar(&snippetRole, "role", "", "Filter by your role: owner, contributor, or member")
	snippetCreateCmd.Flags().StringVarP(&snippetTitle, "title", "t", "", "Snippet title")
	snippetCreateCmd.Flags().BoolVar(&snippetPrivate, "private", false, "Make the snippet private")
	snippetCreateCmd.Flags().StringVar(&snippetFilename, "filename", "", "Filename to use when reading content from stdin")
	snippetUpdateCmd.Flags().StringVarP(&snippetTitle, "title", "t", "", "New snippet title")
	snippetUpdateCmd.Flags().BoolVar(&snippetPrivate, "private", false, "Make the snippet private")
	snippetUpdateCmd.Flags().StringVar(&snippetFilename, "filename", "", "Filename to use when reading content from stdin")

	snippetCmd.AddCommand(snippetListCmd)
	snippetCmd.AddCommand(snippetViewCmd)
	snippetCmd.AddCommand(snippetCreateCmd)
	snippetCmd.AddCommand(snippetUpdateCmd)
	snippetCmd.AddCommand(snippetDeleteCmd)
}

// buildSnippetMultipart assembles the multipart body shared by snippet create
// and update, from the given file paths plus the title/private/filename flags.
func buildSnippetMultipart(files []string) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	if snippetTitle != "" {
		_ = mw.WriteField("title", snippetTitle)
	}
	_ = mw.WriteField("is_private", strconv.FormatBool(snippetPrivate))

	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return nil, "", fmt.Errorf("opening %s: %w", path, err)
		}
		part, err := mw.CreateFormFile("file", filepath.Base(path))
		if err != nil {
			f.Close()
			return nil, "", err
		}
		if _, err := io.Copy(part, f); err != nil {
			f.Close()
			return nil, "", fmt.Errorf("reading %s: %w", path, err)
		}
		f.Close()
	}

	if snippetFilename != "" {
		part, err := mw.CreateFormFile("file", snippetFilename)
		if err != nil {
			return nil, "", err
		}
		if _, err := io.Copy(part, snippetStdin); err != nil {
			return nil, "", fmt.Errorf("reading stdin: %w", err)
		}
	}

	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return body, mw.FormDataContentType(), nil
}

func runSnippetUpdate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := snippetNewAPIClient()
	if err != nil {
		return err
	}

	workspace, snippetID := args[0], args[1]
	files := args[2:]
	if len(files) == 0 && snippetFilename == "" {
		return fmt.Errorf("provide file paths, or --filename to read content from stdin")
	}

	body, contentType, err := buildSnippetMultipart(files)
	if err != nil {
		return err
	}

	resp, err := client.RequestRaw(ctx, http.MethodPut, fmt.Sprintf("/snippets/%s/%s", workspace, snippetID), contentType, body)
	if err != nil {
		return fmt.Errorf("failed to update snippet: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Updated snippet %s", snippetID)
	return nil
}

// resolveWorkspace returns the explicit workspace argument or falls back to the
// configured default, erroring when neither is available.
func resolveWorkspace(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if ws := config.GetWorkspace(); ws != "" {
		return ws, nil
	}
	return "", fmt.Errorf("no workspace given and none configured (set one with `bb config` or BB_WORKSPACE)")
}

func runSnippetList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := snippetNewAPIClient()
	if err != nil {
		return err
	}

	var explicit string
	if len(args) == 1 {
		explicit = args[0]
	}
	workspace, err := resolveWorkspace(explicit)
	if err != nil {
		return err
	}

	var params *generated.GetSnippetsWorkspaceParams
	if snippetRole != "" {
		role := generated.GetSnippetsWorkspaceParamsRole(snippetRole)
		params = &generated.GetSnippetsWorkspaceParams{Role: &role}
	}

	resp, err := client.GetSnippetsWorkspace(ctx, workspace, params)
	if err != nil {
		return fmt.Errorf("failed to list snippets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []snippetSummary `json:"values"`
		Next   string           `json:"next"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Follow pagination links so the full set is returned.
	for result.Next != "" {
		next := result.Next
		result.Next = ""
		pageResp, err := client.GetURL(ctx, next)
		if err != nil {
			return fmt.Errorf("failed to fetch next page: %w", err)
		}
		var page struct {
			Values []snippetSummary `json:"values"`
			Next   string           `json:"next"`
		}
		err = json.NewDecoder(pageResp.Body).Decode(&page)
		pageResp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to parse page: %w", err)
		}
		result.Values = append(result.Values, page.Values...)
		result.Next = page.Next
	}

	if len(result.Values) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No snippets found")
		return nil
	}

	headers := []string{"ID", "TITLE", "PRIVATE", "OWNER", "UPDATED"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, s := range result.Values {
		priv := "No"
		if s.IsPrivate {
			priv = "Yes"
		}
		rows = append(rows, []string{
			s.ID,
			output.Truncate(s.Title, 40),
			priv,
			s.Owner.DisplayName,
			s.UpdatedOn,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"id":         s.ID,
			"title":      s.Title,
			"is_private": s.IsPrivate,
			"owner":      s.Owner.DisplayName,
			"updated_on": s.UpdatedOn,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runSnippetView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := snippetNewAPIClient()
	if err != nil {
		return err
	}

	workspace, snippetID := args[0], args[1]

	resp, err := client.GetSnippetsWorkspaceEncodedId(ctx, workspace, snippetID)
	if err != nil {
		return fmt.Errorf("failed to get snippet: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var snippet struct {
		snippetSummary
		Files map[string]interface{} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&snippet); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if output.CurrentFormat == output.FormatJSON {
		return output.Print(nil, nil, snippet)
	}

	fmt.Printf("ID:      %s\n", snippet.ID)
	fmt.Printf("Title:   %s\n", snippet.Title)
	fmt.Printf("Private: %t\n", snippet.IsPrivate)
	fmt.Printf("Owner:   %s\n", snippet.Owner.DisplayName)
	fmt.Printf("Created: %s\n", snippet.CreatedOn)
	fmt.Printf("Updated: %s\n", snippet.UpdatedOn)
	fmt.Printf("Files:\n")
	for name := range snippet.Files {
		fmt.Printf("  - %s\n", name)
	}

	return nil
}

func runSnippetCreate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := snippetNewAPIClient()
	if err != nil {
		return err
	}

	// First arg is the workspace when it isn't an existing file; remaining args
	// are file paths. Content can also come from stdin with --filename.
	var explicit string
	files := args
	if len(args) > 0 {
		if _, statErr := os.Stat(args[0]); statErr != nil {
			explicit = args[0]
			files = args[1:]
		}
	}
	workspace, err := resolveWorkspace(explicit)
	if err != nil {
		return err
	}

	if len(files) == 0 && snippetFilename == "" {
		return fmt.Errorf("provide file paths, or --filename to read content from stdin")
	}

	body, contentType, err := buildSnippetMultipart(files)
	if err != nil {
		return err
	}

	resp, err := client.PostSnippetsWorkspaceWithBody(ctx, workspace, contentType, body)
	if err != nil {
		return fmt.Errorf("failed to create snippet: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	var result snippetSummary
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	output.Success("Created snippet %s", result.ID)
	return nil
}

func runSnippetDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := snippetNewAPIClient()
	if err != nil {
		return err
	}

	workspace, snippetID := args[0], args[1]

	resp, err := client.DeleteSnippetsWorkspaceEncodedId(ctx, workspace, snippetID)
	if err != nil {
		return fmt.Errorf("failed to delete snippet: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Deleted snippet %s", snippetID)
	return nil
}

// snippetSummary is the subset of a snippet we render.
type snippetSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	IsPrivate bool   `json:"is_private"`
	CreatedOn string `json:"created_on"`
	UpdatedOn string `json:"updated_on"`
	Owner     struct {
		DisplayName string `json:"display_name"`
	} `json:"owner"`
}
