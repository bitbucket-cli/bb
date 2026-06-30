package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var downloadOutput string

// Mockable dependencies for testing
var downloadNewAPIClient = api.NewClient

var downloadCmd = &cobra.Command{
	Use:     "download",
	Aliases: []string{"downloads"},
	Short:   "Manage repository download artifacts",
	Long:    `List, upload, fetch, and delete a repository's download artifacts.`,
}

var downloadListCmd = &cobra.Command{
	Use:   "list <workspace/repo>",
	Short: "List download artifacts",
	Long:  `List the download artifacts attached to a repository.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDownloadList,
}

var downloadUploadCmd = &cobra.Command{
	Use:   "upload <workspace/repo> <files...>",
	Short: "Upload download artifacts",
	Long: `Upload one or more files as repository download artifacts.

Example:
  bb download upload myworkspace/myrepo dist/app-linux dist/app-macos`,
	Args: cobra.MinimumNArgs(2),
	RunE: runDownloadUpload,
}

var downloadGetCmd = &cobra.Command{
	Use:   "get <workspace/repo> <filename>",
	Short: "Download an artifact",
	Long: `Fetch a download artifact's contents.

Writes to the artifact's filename in the current directory, or to the path
given with -o.`,
	Args: cobra.ExactArgs(2),
	RunE: runDownloadGet,
}

var downloadDeleteCmd = &cobra.Command{
	Use:   "delete <workspace/repo> <filename>",
	Short: "Delete a download artifact",
	Long:  `Delete a download artifact from a repository.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runDownloadDelete,
}

func init() {
	downloadGetCmd.Flags().StringVarP(&downloadOutput, "output", "o", "", "Write to this path instead of the artifact filename")

	downloadCmd.AddCommand(downloadListCmd)
	downloadCmd.AddCommand(downloadUploadCmd)
	downloadCmd.AddCommand(downloadGetCmd)
	downloadCmd.AddCommand(downloadDeleteCmd)
}

func runDownloadList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := downloadNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugDownloads(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	if err != nil {
		return fmt.Errorf("failed to list downloads: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []downloadArtifact `json:"values"`
		Next   string             `json:"next"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	for result.Next != "" {
		next := result.Next
		result.Next = ""
		pageResp, err := client.GetURL(ctx, next)
		if err != nil {
			return fmt.Errorf("failed to fetch next page: %w", err)
		}
		var page struct {
			Values []downloadArtifact `json:"values"`
			Next   string             `json:"next"`
		}
		err = json.NewDecoder(pageResp.Body).Decode(&page)
		pageResp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to parse page: %w", err)
		}
		result.Values = append(result.Values, page.Values...)
		result.Next = page.Next
	}

	if len(result.Values) == 0 {
		output.Info("No downloads found")
		return nil
	}

	headers := []string{"NAME", "SIZE", "DOWNLOADS", "CREATED"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, d := range result.Values {
		rows = append(rows, []string{
			d.Name,
			humanSize(d.Size),
			fmt.Sprintf("%d", d.Downloads),
			d.CreatedOn,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"name":       d.Name,
			"size":       d.Size,
			"downloads":  d.Downloads,
			"created_on": d.CreatedOn,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runDownloadUpload(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := downloadNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for _, path := range args[1:] {
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("opening %s: %w", path, err)
		}
		part, err := mw.CreateFormFile("files", filepath.Base(path))
		if err != nil {
			f.Close()
			return err
		}
		if _, err := io.Copy(part, f); err != nil {
			f.Close()
			return fmt.Errorf("reading %s: %w", path, err)
		}
		f.Close()
	}
	if err := mw.Close(); err != nil {
		return err
	}

	payload := body.Bytes()
	contentType := mw.FormDataContentType()
	withBody := func(ctx context.Context, req *http.Request) error {
		req.Body = io.NopCloser(bytes.NewReader(payload))
		req.ContentLength = int64(len(payload))
		req.Header.Set("Content-Type", contentType)
		return nil
	}

	resp, err := client.PostRepositoriesWorkspaceRepoSlugDownloads(ctx, repoCtx.Workspace, repoCtx.RepoSlug, withBody)
	if err != nil {
		return fmt.Errorf("failed to upload downloads: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Uploaded %d file(s)", len(args)-1)
	return nil
}

func runDownloadGet(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := downloadNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}
	filename := args[1]

	resp, err := client.GetRepositoriesWorkspaceRepoSlugDownloadsFilename(ctx, repoCtx.Workspace, repoCtx.RepoSlug, filename)
	if err != nil {
		return fmt.Errorf("failed to fetch download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	dest := downloadOutput
	if dest == "" {
		dest = filepath.Base(filename)
	}
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dest, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}

	output.Success("Saved %s", dest)
	return nil
}

func runDownloadDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := downloadNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}
	filename := args[1]

	resp, err := client.DeleteRepositoriesWorkspaceRepoSlugDownloadsFilename(ctx, repoCtx.Workspace, repoCtx.RepoSlug, filename)
	if err != nil {
		return fmt.Errorf("failed to delete download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Deleted download %s", filename)
	return nil
}

// downloadArtifact is the subset of a download we render.
type downloadArtifact struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Downloads int    `json:"downloads"`
	CreatedOn string `json:"created_on"`
}

// humanSize renders a byte count in a compact human-readable form.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(n)/float64(div), "KMGTPE"[exp])
}
