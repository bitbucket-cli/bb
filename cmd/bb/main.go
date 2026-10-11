package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bitbucket-cli/bb/internal/config"
	"github.com/bitbucket-cli/bb/internal/output"
	"github.com/bitbucket-cli/bb/pkg/auth"
	"github.com/spf13/cobra"
)

var (
	version    = "dev"
	jsonOutput bool
	csvOutput  bool
	osExit     = os.Exit // Mockable for testing
)

var rootCmd = &cobra.Command{
	Use:   "bb",
	Short: "Bitbucket CLI - A command-line interface for Bitbucket Cloud",
	Long: `bb is a CLI for working with Bitbucket Cloud repositories,
pull requests, branches, and pipelines.

To get started, run:
  bb auth login`,
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize config
		if err := config.Init(); err != nil {
			return err
		}

		// Apply the configured token storage mode (auto/keyring/file).
		auth.SetTokenStoreMode(config.GetTokenStore())

		// Apply the configured default, then explicit flags.
		output.SetFormat(config.Get().OutputFormat)

		// Set output format
		if jsonOutput {
			output.SetFormat("json")
		} else if csvOutput {
			output.SetFormat("csv")
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&csvOutput, "csv", false, "Output in CSV format")
	rootCmd.MarkFlagsMutuallyExclusive("json", "csv")
	rootCmd.PersistentFlags().StringVarP(&repoOverride, "repo", "R", "", "Target repository as workspace/repo (default: detected from the current git remote)")

	// Add subcommands
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(repoCmd)
	rootCmd.AddCommand(prCmd)
	rootCmd.AddCommand(branchCmd)
	rootCmd.AddCommand(pipelineCmd)
	rootCmd.AddCommand(webhookCmd)
	rootCmd.AddCommand(snippetCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(deployKeyCmd)
	rootCmd.AddCommand(variableCmd)
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(sourceCmd)
	rootCmd.AddCommand(issueCmd)
	rootCmd.AddCommand(tagCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(configCmd)
}

// cmdCtx returns the command's context, falling back to context.Background()
// when the command was not started via Execute() (e.g. in tests).
func cmdCtx(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		osExit(1)
	}
}
