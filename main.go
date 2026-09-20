package main

import (
	"fmt"
	"os"

	"golrc/lrcgen"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "golrc",
	Short: "Generate .lrc files quickly",
}

var generateCmd = &cobra.Command{
	Use:     "gen",
	Short:   "Generate lrc files recursevly",
	Example: `urly gen file.flac`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]

		lrcgen.Run(file, 10)
	},
}

var allCommands = []*cobra.Command{generateCmd}

func init() {
	for _, comm := range allCommands {
		rootCmd.AddCommand(comm)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
