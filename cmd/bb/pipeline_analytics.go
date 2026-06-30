package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var pipelineAnalyticsLimit int

var pipelineAnalyticsCmd = &cobra.Command{
	Use:   "analytics <workspace/repo>",
	Short: "Show pipeline success/failure statistics",
	Long:  `Summarise recent pipeline runs: counts by result and the success rate.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPipelineAnalytics,
}

func init() {
	pipelineAnalyticsCmd.Flags().IntVar(&pipelineAnalyticsLimit, "limit", 50, "Number of recent pipelines to sample")
	pipelineCmd.AddCommand(pipelineAnalyticsCmd)
}

func runPipelineAnalytics(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := pipelineNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := api.ParseRepoContext(args[0])
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/repositories/%s/%s/pipelines/?pagelen=50&sort=-created_on", repoCtx.Workspace, repoCtx.RepoSlug)
	type pipelineState struct {
		State struct {
			Name   string `json:"name"`
			Result *struct {
				Name string `json:"name"`
			} `json:"result"`
		} `json:"state"`
	}
	var pipelines []pipelineState

	for path != "" && len(pipelines) < pipelineAnalyticsLimit {
		resp, err := client.Request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return fmt.Errorf("failed to fetch pipelines: %w", err)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return fmt.Errorf("API error: %s", resp.Status)
		}
		var page struct {
			Values []pipelineState `json:"values"`
			Next   string          `json:"next"`
		}
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
		pipelines = append(pipelines, page.Values...)
		path = page.Next
	}
	if len(pipelines) > pipelineAnalyticsLimit {
		pipelines = pipelines[:pipelineAnalyticsLimit]
	}

	if len(pipelines) == 0 {
		output.Info("No pipelines found")
		return nil
	}

	byResult := map[string]int{}
	completed, successful := 0, 0
	for _, p := range pipelines {
		if p.State.Result != nil && p.State.Result.Name != "" {
			byResult[p.State.Result.Name]++
			completed++
			if p.State.Result.Name == "SUCCESSFUL" {
				successful++
			}
		} else {
			byResult[p.State.Name]++ // IN_PROGRESS / PENDING etc.
		}
	}

	rate := "n/a"
	if completed > 0 {
		rate = fmt.Sprintf("%.0f%%", float64(successful)/float64(completed)*100)
	}

	headers := []string{"METRIC", "VALUE"}
	rows := [][]string{
		{"Sampled", fmt.Sprintf("%d", len(pipelines))},
		{"Success rate", rate},
	}
	for name, n := range byResult {
		rows = append(rows, []string{name, fmt.Sprintf("%d", n)})
	}
	jsonData := map[string]interface{}{
		"sampled":      len(pipelines),
		"success_rate": rate,
		"by_result":    byResult,
	}
	return output.Print(headers, rows, jsonData)
}
