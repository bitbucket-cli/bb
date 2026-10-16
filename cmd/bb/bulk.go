package main

import (
	"fmt"
	"strconv"

	"github.com/bitbucket-cli/bb/internal/output"
)

// Validate the entire batch before authenticating or starting any mutations.
func parseBulkIDs(ids []string, resource string) ([]int, error) {
	numbers := make([]int, len(ids))
	seen := make(map[int]bool, len(ids))
	for i, id := range ids {
		n, err := strconv.Atoi(id)
		decimal := id != ""
		for _, ch := range id {
			decimal = decimal && ch >= '0' && ch <= '9'
		}
		if err != nil || n <= 0 || !decimal {
			return nil, fmt.Errorf("invalid %s ID %q: expected a positive decimal integer", resource, id)
		}
		if seen[n] {
			return nil, fmt.Errorf("duplicate %s ID %q", resource, id)
		}
		seen[n] = true
		numbers[i] = n
	}
	return numbers, nil
}

type bulkResult struct {
	Repository        string `json:"repository"`
	Resource          string `json:"resource"`
	ID                string `json:"id"`
	Action            string `json:"action"`
	Status            string `json:"status"`
	State             string `json:"state,omitempty"`
	Strategy          string `json:"strategy,omitempty"`
	CloseSourceBranch *bool  `json:"close_source_branch,omitempty"`
	Error             string `json:"error,omitempty"`
}

func bulkPlan(repository, resource, action string, ids []int) []bulkResult {
	results := make([]bulkResult, len(ids))
	for i, id := range ids {
		results[i] = bulkResult{Repository: repository, Resource: resource, ID: strconv.Itoa(id), Action: action, Status: "planned"}
	}
	return results
}

func completeBulk(results []bulkResult, errs []error) {
	for i, err := range errs {
		results[i].Status = "success"
		if err != nil {
			results[i].Status = "failed"
			results[i].Error = err.Error()
		}
	}
}

func reportBulk(results []bulkResult) error {
	rows := make([][]string, len(results))
	failures := 0
	for i, result := range results {
		closeSource := ""
		if result.CloseSourceBranch != nil {
			closeSource = strconv.FormatBool(*result.CloseSourceBranch)
		}
		rows[i] = []string{result.Repository, result.Resource, result.ID, result.Action, result.Status, result.State, result.Strategy, closeSource, result.Error}
		if result.Status == "failed" {
			failures++
		}
	}
	if err := output.Print([]string{"Repository", "Resource", "ID", "Action", "Status", "State", "Strategy", "Close source", "Error"}, rows, results); err != nil {
		return err
	}
	if failures > 0 {
		return fmt.Errorf("%d of %d operations failed", failures, len(results))
	}
	return nil
}
