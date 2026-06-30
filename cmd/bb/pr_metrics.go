package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ThomasRedstone/bb/internal/output"
	"github.com/spf13/cobra"
)

var prMetricsLimit int

var prMetricsCmd = &cobra.Command{
	Use:   "metrics [workspace/repo]",
	Short: "Show pull request metrics (time to merge)",
	Long:  `Summarise merged pull requests: count and average/median time from open to merge.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPRMetrics,
}

func init() {
	prMetricsCmd.Flags().IntVar(&prMetricsLimit, "limit", 50, "Maximum merged PRs to sample")
	prCmd.AddCommand(prMetricsCmd)
}

func runPRMetrics(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/repositories/%s/%s/pullrequests?state=MERGED&pagelen=50", repoCtx.Workspace, repoCtx.RepoSlug)
	var prs []struct {
		CreatedOn string `json:"created_on"`
		UpdatedOn string `json:"updated_on"`
	}

	for path != "" && len(prs) < prMetricsLimit {
		resp, err := client.Request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return fmt.Errorf("failed to fetch pull requests: %w", err)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return fmt.Errorf("API error: %s", resp.Status)
		}
		var page struct {
			Values []struct {
				CreatedOn string `json:"created_on"`
				UpdatedOn string `json:"updated_on"`
			} `json:"values"`
			Next string `json:"next"`
		}
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
		prs = append(prs, page.Values...)
		path = page.Next
	}
	if len(prs) > prMetricsLimit {
		prs = prs[:prMetricsLimit]
	}

	if len(prs) == 0 {
		output.Info("No merged pull requests found")
		return nil
	}

	var durations []time.Duration
	for _, pr := range prs {
		created, err1 := time.Parse(time.RFC3339, pr.CreatedOn)
		merged, err2 := time.Parse(time.RFC3339, pr.UpdatedOn)
		if err1 != nil || err2 != nil {
			continue
		}
		durations = append(durations, merged.Sub(created))
	}
	if len(durations) == 0 {
		output.Info("Could not compute timings from the sampled PRs")
		return nil
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}
	avg := total / time.Duration(len(durations))
	median := medianDuration(durations)

	headers := []string{"METRIC", "VALUE"}
	rows := [][]string{
		{"Merged PRs sampled", fmt.Sprintf("%d", len(prs))},
		{"Avg time to merge", roundDuration(avg)},
		{"Median time to merge", roundDuration(median)},
	}
	jsonData := map[string]interface{}{
		"sampled":                len(prs),
		"avg_time_to_merge":      avg.String(),
		"median_time_to_merge":   median.String(),
		"avg_time_to_merge_h":    avg.Hours(),
		"median_time_to_merge_h": median.Hours(),
	}
	return output.Print(headers, rows, jsonData)
}

func medianDuration(ds []time.Duration) time.Duration {
	// Simple insertion sort — sample sizes are small.
	s := make([]time.Duration, len(ds))
	copy(s, ds)
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// roundDuration renders a duration rounded to a sensible unit.
func roundDuration(d time.Duration) string {
	if d >= 24*time.Hour {
		return fmt.Sprintf("%.1fd", d.Hours()/24)
	}
	if d >= time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.0fm", d.Minutes())
}
