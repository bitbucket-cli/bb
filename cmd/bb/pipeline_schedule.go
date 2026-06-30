package main

import (
	"encoding/json"
	"fmt"

	generated "github.com/ThomasRedstone/bb/generated/bitbucket"
	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var pipelineScheduleCmd = &cobra.Command{
	Use:     "schedule",
	Aliases: []string{"schedules"},
	Short:   "Manage pipeline schedules",
	Long:    `List, view, and delete a repository's pipeline schedules.`,
}

var pipelineScheduleListCmd = &cobra.Command{
	Use:   "list <workspace/repo>",
	Short: "List pipeline schedules",
	Args:  cobra.ExactArgs(1),
	RunE:  runPipelineScheduleList,
}

var pipelineScheduleViewCmd = &cobra.Command{
	Use:   "view <workspace/repo> <schedule-uuid>",
	Short: "View a pipeline schedule",
	Args:  cobra.ExactArgs(2),
	RunE:  runPipelineScheduleView,
}

var pipelineScheduleDeleteCmd = &cobra.Command{
	Use:   "delete <workspace/repo> <schedule-uuid>",
	Short: "Delete a pipeline schedule",
	Args:  cobra.ExactArgs(2),
	RunE:  runPipelineScheduleDelete,
}

var pipelineCacheCmd = &cobra.Command{
	Use:     "cache",
	Aliases: []string{"caches"},
	Short:   "Manage pipeline caches",
	Long:    `List and delete a repository's pipeline caches.`,
}

var pipelineCacheListCmd = &cobra.Command{
	Use:   "list <workspace/repo>",
	Short: "List pipeline caches",
	Args:  cobra.ExactArgs(1),
	RunE:  runPipelineCacheList,
}

var pipelineCacheDeleteCmd = &cobra.Command{
	Use:   "delete <workspace/repo> <cache-name>",
	Short: "Delete a pipeline cache by name",
	Args:  cobra.ExactArgs(2),
	RunE:  runPipelineCacheDelete,
}

func init() {
	pipelineScheduleCmd.AddCommand(pipelineScheduleListCmd)
	pipelineScheduleCmd.AddCommand(pipelineScheduleViewCmd)
	pipelineScheduleCmd.AddCommand(pipelineScheduleDeleteCmd)

	pipelineCacheCmd.AddCommand(pipelineCacheListCmd)
	pipelineCacheCmd.AddCommand(pipelineCacheDeleteCmd)

	pipelineCmd.AddCommand(pipelineScheduleCmd)
	pipelineCmd.AddCommand(pipelineCacheCmd)
}

func runPipelineScheduleList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := pipelineNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoryPipelineSchedules(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	if err != nil {
		return fmt.Errorf("failed to list schedules: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []pipelineSchedule `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Values) == 0 {
		output.Info("No schedules found")
		return nil
	}

	headers := []string{"UUID", "ENABLED", "CRON", "REF"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, s := range result.Values {
		enabled := "No"
		if s.Enabled {
			enabled = "Yes"
		}
		rows = append(rows, []string{s.UUID, enabled, s.CronPattern, s.Target.RefName})
		jsonData = append(jsonData, map[string]interface{}{
			"uuid":         s.UUID,
			"enabled":      s.Enabled,
			"cron_pattern": s.CronPattern,
			"ref_name":     s.Target.RefName,
		})
	}
	return output.Print(headers, rows, jsonData)
}

func runPipelineScheduleView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := pipelineNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoryPipelineSchedule(ctx, repoCtx.Workspace, repoCtx.RepoSlug, args[1])
	if err != nil {
		return fmt.Errorf("failed to get schedule: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var s pipelineSchedule
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if jsonOutput {
		return output.Print(nil, nil, s)
	}
	fmt.Printf("UUID:    %s\n", s.UUID)
	fmt.Printf("Enabled: %t\n", s.Enabled)
	fmt.Printf("Cron:    %s\n", s.CronPattern)
	fmt.Printf("Ref:     %s\n", s.Target.RefName)
	fmt.Printf("Created: %s\n", s.CreatedOn)
	return nil
}

func runPipelineScheduleDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := pipelineNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	resp, err := client.DeleteRepositoryPipelineSchedule(ctx, repoCtx.Workspace, repoCtx.RepoSlug, args[1])
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	output.Success("Deleted schedule %s", args[1])
	return nil
}

func runPipelineCacheList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := pipelineNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	resp, err := client.GetRepositoryPipelineCaches(ctx, repoCtx.Workspace, repoCtx.RepoSlug)
	if err != nil {
		return fmt.Errorf("failed to list caches: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Values []pipelineCache `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Values) == 0 {
		output.Info("No caches found")
		return nil
	}

	headers := []string{"NAME", "PATH", "SIZE", "CREATED"}
	var rows [][]string
	var jsonData []map[string]interface{}
	for _, c := range result.Values {
		rows = append(rows, []string{c.Name, output.Truncate(c.Path, 40), humanSize(c.FileSizeBytes), c.CreatedOn})
		jsonData = append(jsonData, map[string]interface{}{
			"uuid":            c.UUID,
			"name":            c.Name,
			"path":            c.Path,
			"file_size_bytes": c.FileSizeBytes,
		})
	}
	return output.Print(headers, rows, jsonData)
}

func runPipelineCacheDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := pipelineNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	params := &generated.DeleteRepositoryPipelineCachesParams{Name: args[1]}
	resp, err := client.DeleteRepositoryPipelineCaches(ctx, repoCtx.Workspace, repoCtx.RepoSlug, params)
	if err != nil {
		return fmt.Errorf("failed to delete cache: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	output.Success("Deleted cache %s", args[1])
	return nil
}

// pipelineSchedule is the subset of a schedule we render.
type pipelineSchedule struct {
	UUID        string `json:"uuid"`
	Enabled     bool   `json:"enabled"`
	CronPattern string `json:"cron_pattern"`
	Target      struct {
		RefName string `json:"ref_name"`
	} `json:"target"`
	CreatedOn string `json:"created_on"`
}

// pipelineCache is the subset of a cache we render.
type pipelineCache struct {
	UUID          string `json:"uuid"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	CreatedOn     string `json:"created_on"`
}
