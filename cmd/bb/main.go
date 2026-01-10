package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tom/bb/internal/config"
	"github.com/tom/bb/internal/output"
)

var (
	version    = "dev"
	jsonOutput bool
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

		// Set output format
		if jsonOutput {
			output.SetFormat("json")
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	// Add subcommands
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(repoCmd)
	rootCmd.AddCommand(prCmd)
	rootCmd.AddCommand(branchCmd)
	rootCmd.AddCommand(pipelineCmd)
	rootCmd.AddCommand(webhookCmd)
	rootCmd.AddCommand(issueCmd)
	rootCmd.AddCommand(tagCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(configCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		osExit(1)
	}
}
