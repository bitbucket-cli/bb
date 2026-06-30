package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var manCmd = &cobra.Command{
	Use:    "man <output-dir>",
	Short:  "Generate man pages for bb into a directory",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		header := &doc.GenManHeader{
			Title:   "BB",
			Section: "1",
			Source:  "bb",
			Manual:  "bb Manual",
		}
		return doc.GenManTree(rootCmd, header, args[0])
	},
}

func init() {
	rootCmd.AddCommand(manCmd)
}
