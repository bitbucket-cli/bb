package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ThomasRedstone/bb/internal/config"
	"github.com/ThomasRedstone/bb/internal/output"
	"github.com/spf13/cobra"
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
	rootCmd.AddCommand(snippetCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(deployKeyCmd)
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
