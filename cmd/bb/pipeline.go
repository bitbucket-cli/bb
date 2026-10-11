package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bitbucket-cli/bb/generated/bitbucket"
	"github.com/bitbucket-cli/bb/internal/output"
	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	pipelineBranch      string
	pipelineLimit       int
	pipelinePattern     string
	pipelinePullRequest int
	pipelineVariables   []string
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
	Long: `Trigger a new pipeline run on a branch, or re-run a pull request's pipeline.

With --pull-request, this runs the repository's pull-requests pipeline for that
PR, exactly as opening or updating the PR would. That is the only way to clear a
"No failed builds on last commit" merge check without pushing a new commit --
and pushing one can reset existing approvals.

Note that --branch with a pull-requests pipeline is NOT equivalent: it runs the
same YAML but as a branch trigger, so Bitbucket does not set BITBUCKET_PR_ID and
any step that needs it fails.`,
	Example: `  # Re-run the pipeline for PR 197 after a transient failure
  bb pipeline trigger --pull-request 197

  # Run a branch's default pipeline
  bb pipeline trigger --branch main

  # Run a custom pipeline with a variable
  bb pipeline trigger --branch main --pattern deploy -e ENVIRONMENT=staging`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPipelineTrigger,
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
	pipelineTriggerCmd.Flags().StringVarP(&pipelineBranch, "branch", "b", "", "Branch to run pipeline on (required unless --pull-request is given)")
	pipelineTriggerCmd.Flags().IntVarP(&pipelinePullRequest, "pull-request", "P", 0, "Pull request id to run the pull-requests pipeline for (re-runs its checks)")
	pipelineTriggerCmd.MarkFlagsMutuallyExclusive("branch", "pull-request")
	pipelineTriggerCmd.MarkFlagsOneRequired("branch", "pull-request")
	pipelineTriggerCmd.Flags().StringVarP(&pipelinePattern, "pattern", "p", "", "Custom pipeline pattern name (bitbucket-pipelines.yml pipelines.custom.<pattern>); omit to run the branch's default pipeline. Not valid with --pull-request: Bitbucket cannot run custom pipelines for a pull request")
	pipelineTriggerCmd.Flags().StringArrayVarP(&pipelineVariables, "variable", "e", nil, "Pipeline variable as KEY=VALUE (repeatable)")

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

	if len(allPipelines) == 0 && output.CurrentFormat == output.FormatTable {
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

	if output.CurrentFormat == output.FormatJSON {
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
	var target map[string]interface{}
	if pipelinePullRequest > 0 {
		if pipelinePattern != "" {
			return fmt.Errorf("--pattern cannot be used with --pull-request: Bitbucket does not support running custom pipelines for a pull request")
		}
		target, err = pipelinePullRequestTarget(ctx, client, repoCtx, pipelinePullRequest)
		if err != nil {
			return err
		}
	} else {
		target = map[string]interface{}{
			"type":     "pipeline_ref_target",
			"ref_type": "branch",
			"ref_name": pipelineBranch,
		}
		if pipelinePattern != "" {
			target["selector"] = map[string]interface{}{
				"type":    "custom",
				"pattern": pipelinePattern,
			}
		}
	}
	triggerRequest := map[string]interface{}{
		"target": target,
	}

	if len(pipelineVariables) > 0 {
		variables := make([]map[string]string, 0, len(pipelineVariables))
		for _, v := range pipelineVariables {
			key, value, ok := strings.Cut(v, "=")
			if !ok {
				return fmt.Errorf("invalid --variable %q: expected KEY=VALUE", v)
			}
			variables = append(variables, map[string]string{"key": key, "value": value})
		}
		triggerRequest["variables"] = variables
	}

	body, err := json.Marshal(triggerRequest)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	var resp *http.Response
	if pipelinePullRequest > 0 {
		// Deliberately not the generated client. Its operation path is
		// /repositories/{ws}/{repo}/pipelines with no trailing slash (as the
		// OpenAPI spec declares), and Bitbucket accepts a
		// pipeline_pullrequest_target ONLY on /pipelines/ -- without the slash it
		// answers 400 "The request body contains invalid properties", which reads
		// as a malformed body and is not one. Branch targets work on either, which
		// is why this quirk stays hidden until you try a pull request.
		resp, err = client.RequestRaw(
			ctx,
			http.MethodPost,
			fmt.Sprintf("/repositories/%s/%s/pipelines/", repoCtx.Workspace, repoCtx.RepoSlug),
			"application/json",
			bytesReader(body),
		)
	} else {
		resp, err = client.CreatePipelineForRepositoryWithBody(
			ctx,
			repoCtx.Workspace,
			repoCtx.RepoSlug,
			"application/json",
			bytesReader(body),
		)
	}
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

	if pipelinePattern != "" {
		output.Success("Triggered pipeline #%d (pattern '%s') on branch '%s'", buildNum, pipelinePattern, pipelineBranch)
	} else {
		output.Success("Triggered pipeline #%d on branch '%s'", buildNum, pipelineBranch)
	}

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

	if len(steps) == 0 && output.CurrentFormat == output.FormatTable {
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

// pipelinePullRequestTarget builds the target for a pull-requests pipeline run.
//
// Bitbucket needs the PR's source and destination branches and both commit
// hashes, not just the id, so this reads the PR first. The id is sent as a
// string: the API rejects an integer here even though it returns one.
func pipelinePullRequestTarget(
	ctx context.Context,
	client *api.Client,
	repoCtx *api.RepoContext,
	prID int,
) (map[string]interface{}, error) {
	resp, err := client.Request(
		ctx,
		http.MethodGet,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%d", repoCtx.Workspace, repoCtx.RepoSlug, prID),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to look up pull request %d: %w", prID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to look up pull request %d: %s", prID, resp.Status)
	}

	var pr struct {
		State  string `json:"state"`
		Source struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
			Commit struct {
				Hash string `json:"hash"`
			} `json:"commit"`
		} `json:"source"`
		Destination struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
			Commit struct {
				Hash string `json:"hash"`
			} `json:"commit"`
		} `json:"destination"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("failed to parse pull request %d: %w", prID, err)
	}

	if pr.Source.Branch.Name == "" || pr.Source.Commit.Hash == "" {
		return nil, fmt.Errorf("pull request %d has no source branch or commit to run against", prID)
	}
	// A merged or declined PR has nothing to re-run, and Bitbucket's error for it
	// is not obvious.
	if pr.State != "" && pr.State != "OPEN" {
		return nil, fmt.Errorf("pull request %d is %s; only an open pull request can run a pipeline", prID, pr.State)
	}

	target := map[string]interface{}{
		"type":        "pipeline_pullrequest_target",
		"source":      pr.Source.Branch.Name,
		"destination": pr.Destination.Branch.Name,
		"commit":      map[string]interface{}{"type": "commit", "hash": pr.Source.Commit.Hash},
		"pullrequest": map[string]interface{}{"type": "pullrequest", "id": strconv.Itoa(prID)},
		"selector":    map[string]interface{}{"type": "pull-requests", "pattern": "**"},
	}
	if pr.Destination.Commit.Hash != "" {
		target["destination_commit"] = map[string]interface{}{
			"type": "commit",
			"hash": pr.Destination.Commit.Hash,
		}
	}
	return target, nil
}
