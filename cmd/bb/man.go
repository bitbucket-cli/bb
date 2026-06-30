package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docsFormat string

var manCmd = &cobra.Command{
	Use:    "man <output-dir>",
	Short:  "Generate command documentation into a directory",
	Long:   `Generate reference documentation for every bb command — man pages (default) or markdown (--format markdown).`,
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch docsFormat {
		case "markdown", "md":
			return doc.GenMarkdownTree(rootCmd, args[0])
		case "man", "":
			header := &doc.GenManHeader{
				Title:   "BB",
				Section: "1",
				Source:  "bb",
				Manual:  "bb Manual",
			}
			return doc.GenManTree(rootCmd, header, args[0])
		default:
			return fmt.Errorf("unknown --format %q (want man or markdown)", docsFormat)
		}
	},
}

func init() {
	manCmd.Flags().StringVar(&docsFormat, "format", "man", "Output format: man or markdown")
	rootCmd.AddCommand(manCmd)
}
