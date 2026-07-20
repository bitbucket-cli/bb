package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ThomasRedstone/bb/generated/bitbucket"
	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	pipelineBranch string
	pipelineLimit  int
)

const (
	defaultPipelineLimit = 25
	maxPipelinePageLen   = 100
)

// Mockable dependencies for testing
var (
	pipelineNewAPIClient = api.NewClient
)

var pipelineCmd = &cobra.Command{
	Use:     "pipeline",
	Aliases: []string{"pipelines"},
	Short:   "Manage pipelines",
	Long:    `List, view, trigger, and stop pipelines.`,
}

var pipelineListCmd = &cobra.Command{
	Use:   "list [workspace/repo]",
	Short: "List pipelines",
	Long:  `List recent pipelines for a repository.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPipelineList,
}

var pipelineViewCmd = &cobra.Command{
	Use:   "view [workspace/repo] <pipeline-uuid>",
	Short: "View pipeline details",
	Long:  `Display detailed information about a pipeline.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runPipelineView,
}

var pipelineTriggerCmd = &cobra.Command{
	Use:   "trigger [workspace/repo]",
	Short: "Trigger a pipeline",
	Long:  `Trigger a new pipeline run on a branch.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPipelineTrigger,
}

var pipelineStopCmd = &cobra.Command{
	Use:   "stop [workspace/repo] <pipeline-uuid>",
	Short: "Stop a pipeline",
	Long:  `Stop a running pipeline.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runPipelineStop,
}

var pipelineStepsCmd = &cobra.Command{
	Use:   "steps [workspace/repo] <pipeline-uuid>",
	Short: "List pipeline steps",
	Long:  `List all steps in a pipeline.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runPipelineSteps,
}

var pipelineLogsCmd = &cobra.Command{
	Use:   "logs [workspace/repo] <pipeline-uuid> <step-uuid>",
	Short: "View step logs",
	Long:  `Display logs for a specific pipeline step.`,
	Args:  cobra.RangeArgs(2, 3),
	RunE:  runPipelineLogs,
}

func init() {
	pipelineListCmd.Flags().IntVarP(&pipelineLimit, "limit", "l", defaultPipelineLimit, "Maximum number of pipelines to show (0 = all)")
	pipelineTriggerCmd.Flags().StringVarP(&pipelineBranch, "branch", "b", "", "Branch to run pipeline on (required)")
	pipelineTriggerCmd.MarkFlagRequired("branch")

	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineViewCmd)
	pipelineCmd.AddCommand(pipelineTriggerCmd)
	pipelineCmd.AddCommand(pipelineStopCmd)
	pipelineCmd.AddCommand(pipelineStepsCmd)
	pipelineCmd.AddCommand(pipelineLogsCmd)
}

func runPipelineList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := pipelineNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	sort := bitbucket.GetPipelinesForRepositoryParamsSort("-created_on")
	pagelen := pipelinePageLen(pipelineLimit)
	params := &bitbucket.GetPipelinesForRepositoryParams{
		Sort:    &sort,
		Pagelen: &pagelen,
	}

	resp, err := client.GetPipelinesForRepository(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		params,
	)
	if err != nil {
		return fmt.Errorf("failed to list pipelines: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result bitbucket.PaginatedPipelines
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Collect pipelines, following pagination up to the requested limit.
	allPipelines := make([]bitbucket.Pipeline, 0)
	if result.Values != nil {
		allPipelines = append(allPipelines, *result.Values...)
	}
	for result.Next != nil && *result.Next != "" {
		if pipelineLimit > 0 && len(allPipelines) >= pipelineLimit {
			break
		}
		nextResp, err := client.GetURL(ctx, *result.Next)
		if err != nil {
			return fmt.Errorf("failed to fetch next page: %w", err)
		}
		if nextResp.StatusCode != 200 {
			status := nextResp.Status
			nextResp.Body.Close()
			return fmt.Errorf("API error: %s", status)
		}
		result = bitbucket.PaginatedPipelines{}
		if err := json.NewDecoder(nextResp.Body).Decode(&result); err != nil {
			nextResp.Body.Close()
			return fmt.Errorf("failed to parse response: %w", err)
		}
		nextResp.Body.Close()
		if result.Values != nil {
			allPipelines = append(allPipelines, *result.Values...)
		}
	}

	if pipelineLimit > 0 && len(allPipelines) > pipelineLimit {
		allPipelines = allPipelines[:pipelineLimit]
	}

	if len(allPipelines) == 0 {
		output.Info("No pipelines found")
		return nil
	}

	headers := []string{"#", "STATUS", "BRANCH", "CREATED", "DURATION"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, pipeline := range allPipelines {
		buildNum := ""
		if pipeline.BuildNumber != nil {
			buildNum = strconv.Itoa(*pipeline.BuildNumber)
		}

		status := getPipelineStatus(&pipeline)
		branch := getPipelineTarget(&pipeline)

		created := ""
		if pipeline.CreatedOn != nil {
			created = pipeline.CreatedOn.Format("2006-01-02 15:04")
		}

		duration := ""
		if pipeline.BuildSecondsUsed != nil {
			duration = formatDuration(*pipeline.BuildSecondsUsed)
		}

		rows = append(rows, []string{buildNum, status, branch, created, duration})
		jsonData = append(jsonData, map[string]interface{}{
			"build_number": pipeline.BuildNumber,
			"uuid":         pipeline.Uuid,
			"status":       status,
			"branch":       branch,
			"created_on":   created,
			"duration":     pipeline.BuildSecondsUsed,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func pipelinePageLen(limit int) int32 {
	if limit > 0 && limit < maxPipelinePageLen {
		return int32(limit)
	}
	return maxPipelinePageLen
}

func runPipelineView(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := pipelineNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	pipelineUUID := rest[0]
	// Ensure UUID is wrapped in braces
	if !strings.HasPrefix(pipelineUUID, "{") {
		pipelineUUID = "{" + pipelineUUID + "}"
	}

	resp, err := client.GetPipelineForRepository(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		pipelineUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to get pipeline: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var pipeline bitbucket.Pipeline
	if err := json.NewDecoder(resp.Body).Decode(&pipeline); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if jsonOutput {
		return output.JSON(pipeline)
	}

	// Display pipeline details
	buildNum := ""
	if pipeline.BuildNumber != nil {
		buildNum = strconv.Itoa(*pipeline.BuildNumber)
	}

	uuid := ""
	if pipeline.Uuid != nil {
		uuid = *pipeline.Uuid
	}

	status := getPipelineStatus(&pipeline)
	branch := getPipelineTarget(&pipeline)

	fmt.Printf("Pipeline #%s\n", buildNum)
	fmt.Printf("UUID:     %s\n", uuid)
	fmt.Printf("Status:   %s\n", status)
	fmt.Printf("Branch:   %s\n", branch)

	if pipeline.CreatedOn != nil {
		fmt.Printf("Created:  %s\n", pipeline.CreatedOn.Format("2006-01-02 15:04:05"))
	}
	if pipeline.CompletedOn != nil {
		fmt.Printf("Completed: %s\n", pipeline.CompletedOn.Format("2006-01-02 15:04:05"))
	}
	if pipeline.BuildSecondsUsed != nil {
		fmt.Printf("Duration: %s\n", formatDuration(*pipeline.BuildSecondsUsed))
	}

	if pipeline.Creator != nil && pipeline.Creator.DisplayName != nil {
		fmt.Printf("Creator:  %s\n", *pipeline.Creator.DisplayName)
	}

	return nil
}

func runPipelineTrigger(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := pipelineNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	// Build pipeline trigger request
	triggerRequest := map[string]interface{}{
		"target": map[string]interface{}{
			"type":     "pipeline_ref_target",
			"ref_type": "branch",
			"ref_name": pipelineBranch,
		},
	}

	body, err := json.Marshal(triggerRequest)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	resp, err := client.CreatePipelineForRepositoryWithBody(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		"application/json",
		bytesReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to trigger pipeline: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	var pipeline bitbucket.Pipeline
	if err := json.NewDecoder(resp.Body).Decode(&pipeline); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	buildNum := 0
	if pipeline.BuildNumber != nil {
		buildNum = *pipeline.BuildNumber
	}

	output.Success("Triggered pipeline #%d on branch '%s'", buildNum, pipelineBranch)

	if pipeline.Uuid != nil {
		output.Info("UUID: %s", *pipeline.Uuid)
	}

	return nil
}

func runPipelineStop(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := pipelineNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	pipelineUUID := rest[0]
	// Ensure UUID is wrapped in braces
	if !strings.HasPrefix(pipelineUUID, "{") {
		pipelineUUID = "{" + pipelineUUID + "}"
	}

	resp, err := client.StopPipeline(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		pipelineUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to stop pipeline: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Stopped pipeline %s", rest[0])
	return nil
}

// Helper functions

func getPipelineStatus(pipeline *bitbucket.Pipeline) string {
	if pipeline.State == nil {
		return "unknown"
	}

	// The state type uses AdditionalProperties for the actual state info
	// Common states: PENDING, IN_PROGRESS, COMPLETED (with result: SUCCESSFUL, FAILED, STOPPED, EXPIRED)
	stateType := pipeline.State.Type
	if stateType == "" {
		return "unknown"
	}

	// Check additional properties for result if completed
	if result, ok := pipeline.State.AdditionalProperties["result"]; ok {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if resultName, ok := resultMap["name"].(string); ok {
				return resultName
			}
		}
	}

	// Check for name in additional properties
	if name, ok := pipeline.State.AdditionalProperties["name"].(string); ok {
		return name
	}

	return stateType
}

func getPipelineTarget(pipeline *bitbucket.Pipeline) string {
	if pipeline.Target == nil {
		return ""
	}

	// Check additional properties for ref_name (branch name)
	if refName, ok := pipeline.Target.AdditionalProperties["ref_name"].(string); ok {
		return refName
	}

	return ""
}

func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	secs := seconds % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	hours := minutes / 60
	mins := minutes % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func runPipelineSteps(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := pipelineNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	pipelineUUID := rest[0]
	if !strings.HasPrefix(pipelineUUID, "{") {
		pipelineUUID = "{" + pipelineUUID + "}"
	}

	resp, err := client.GetPipelineStepsForRepository(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		pipelineUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to list pipeline steps: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	type pipelineStep struct {
		UUID  string `json:"uuid"`
		Name  string `json:"name"`
		State struct {
			Name   string `json:"name"`
			Result *struct {
				Name string `json:"name"`
			} `json:"result"`
		} `json:"state"`
		DurationInSeconds *int `json:"duration_in_seconds"`
	}

	steps, err := fetchAllPages[pipelineStep](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(steps) == 0 {
		output.Info("No steps found")
		return nil
	}

	headers := []string{"UUID", "NAME", "STATUS", "DURATION"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, step := range steps {
		status := step.State.Name
		if step.State.Result != nil {
			status = step.State.Result.Name
		}

		duration := ""
		if step.DurationInSeconds != nil {
			duration = formatDuration(*step.DurationInSeconds)
		}

		uuid := step.UUID
		if strings.HasPrefix(uuid, "{") && strings.HasSuffix(uuid, "}") {
			uuid = uuid[1 : len(uuid)-1]
		}

		rows = append(rows, []string{
			uuid,
			step.Name,
			status,
			duration,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"uuid":     step.UUID,
			"name":     step.Name,
			"status":   status,
			"duration": step.DurationInSeconds,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runPipelineLogs(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := pipelineNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 2)
	if err != nil {
		return err
	}

	pipelineUUID := rest[0]
	if !strings.HasPrefix(pipelineUUID, "{") {
		pipelineUUID = "{" + pipelineUUID + "}"
	}

	stepUUID := rest[1]
	if !strings.HasPrefix(stepUUID, "{") {
		stepUUID = "{" + stepUUID + "}"
	}

	resp, err := client.GetPipelineStepLogForRepository(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		pipelineUUID,
		stepUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to get step logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	// The log is returned as plain text or octet-stream
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return fmt.Errorf("failed to read logs: %w", err)
	}

	fmt.Print(buf.String())
	return nil
}
