package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ThomasRedstone/bb/internal/output"
	api "github.com/ThomasRedstone/bb/pkg/bitbucket"
	"github.com/spf13/cobra"
)

var (
	envType string
)

// Mockable dependencies for testing
var (
	deployNewAPIClient = api.NewClient
)

var deployCmd = &cobra.Command{
	Use:     "deploy",
	Aliases: []string{"deployment", "deployments"},
	Short:   "Manage deployments",
	Long:    `List deployments and manage deployment environments.`,
}

var deployListCmd = &cobra.Command{
	Use:   "list [workspace/repo]",
	Short: "List deployments",
	Long:  `List all deployments for a repository.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDeployList,
}

var deployEnvCmd = &cobra.Command{
	Use:     "env",
	Aliases: []string{"environment", "environments"},
	Short:   "Manage deployment environments",
	Long:    `List, create, and delete deployment environments.`,
}

var deployEnvListCmd = &cobra.Command{
	Use:   "list [workspace/repo]",
	Short: "List environments",
	Long:  `List all deployment environments for a repository.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDeployEnvList,
}

var deployEnvCreateCmd = &cobra.Command{
	Use:   "create [workspace/repo] <env-name>",
	Short: "Create an environment",
	Long: `Create a new deployment environment.

Example:
  bb deploy env create myworkspace/myrepo production --type Production
  bb deploy env create myworkspace/myrepo staging --type Staging`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runDeployEnvCreate,
}

var deployEnvDeleteCmd = &cobra.Command{
	Use:   "delete [workspace/repo] <env-uuid>",
	Short: "Delete an environment",
	Long:  `Delete a deployment environment.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runDeployEnvDelete,
}

func init() {
	deployEnvCreateCmd.Flags().StringVarP(&envType, "type", "t", "Test", "Environment type (Test, Staging, Production)")

	// Environment subcommands
	deployEnvCmd.AddCommand(deployEnvListCmd)
	deployEnvCmd.AddCommand(deployEnvCreateCmd)
	deployEnvCmd.AddCommand(deployEnvDeleteCmd)

	deployCmd.AddCommand(deployListCmd)
	deployCmd.AddCommand(deployEnvCmd)
}

func runDeployList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := deployNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	resp, err := client.GetDeploymentsForRepository(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
	)
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	type deploymentRow struct {
		UUID        string `json:"uuid"`
		Environment *struct {
			Name string `json:"name"`
			UUID string `json:"uuid"`
		} `json:"environment"`
		Release *struct {
			Name string `json:"name"`
		} `json:"release"`
		State *struct {
			Name string `json:"name"`
		} `json:"state"`
	}

	deployments, err := fetchAllPages[deploymentRow](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(deployments) == 0 {
		output.Info("No deployments found")
		return nil
	}

	headers := []string{"UUID", "ENVIRONMENT", "RELEASE", "STATE"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, deployment := range deployments {
		envName := ""
		if deployment.Environment != nil {
			envName = deployment.Environment.Name
		}

		releaseName := ""
		if deployment.Release != nil {
			releaseName = deployment.Release.Name
		}

		stateName := ""
		if deployment.State != nil {
			stateName = deployment.State.Name
		}

		uuid := deployment.UUID
		if len(uuid) > 8 {
			uuid = uuid[:8] + "..."
		}

		rows = append(rows, []string{
			uuid,
			envName,
			releaseName,
			stateName,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"uuid":        deployment.UUID,
			"environment": envName,
			"release":     releaseName,
			"state":       stateName,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runDeployEnvList(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := deployNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, err := repoArg(args)
	if err != nil {
		return err
	}

	resp, err := client.GetEnvironmentsForRepository(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
	)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	type envRow struct {
		UUID            string `json:"uuid"`
		Name            string `json:"name"`
		EnvironmentType *struct {
			Name string `json:"name"`
		} `json:"environment_type"`
	}

	environments, err := fetchAllPages[envRow](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(environments) == 0 {
		output.Info("No environments found")
		return nil
	}

	headers := []string{"UUID", "NAME", "TYPE"}
	var rows [][]string
	var jsonData []map[string]interface{}

	for _, env := range environments {
		envTypeName := ""
		if env.EnvironmentType != nil {
			envTypeName = env.EnvironmentType.Name
		}

		rows = append(rows, []string{
			env.UUID,
			env.Name,
			envTypeName,
		})
		jsonData = append(jsonData, map[string]interface{}{
			"uuid": env.UUID,
			"name": env.Name,
			"type": envTypeName,
		})
	}

	return output.Print(headers, rows, jsonData)
}

func runDeployEnvCreate(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := deployNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	envName := rest[0]

	envReq := map[string]interface{}{
		"name": envName,
		"environment_type": map[string]interface{}{
			"name": envType,
		},
	}

	body, err := json.Marshal(envReq)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	withBody := func(ctx context.Context, req *http.Request) error {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/json")
		return nil
	}

	resp, err := client.CreateEnvironmentWithBody(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		"application/json",
		bytes.NewReader(body),
		withBody,
	)
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	var result struct {
		UUID string `json:"uuid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	output.Success("Created environment '%s' (UUID: %s)", envName, result.UUID)
	return nil
}

func runDeployEnvDelete(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := deployNewAPIClient()
	if err != nil {
		return err
	}

	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}

	envUUID := rest[0]

	resp, err := client.DeleteEnvironmentForRepository(
		ctx,
		repoCtx.Workspace,
		repoCtx.RepoSlug,
		envUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("API error: %s - %v", resp.Status, errResp)
	}

	output.Success("Deleted environment %s", envUUID)
	return nil
}
