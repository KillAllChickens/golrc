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
	Example: `golrc gen /path/to/music/`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]

		threads, _ := cmd.Flags().GetInt("threads")

		lrcgen.Run(file, threads)
	},
}

var allCommands = []*cobra.Command{generateCmd}

func init() {
	generateCmd.Flags().IntP("threads", "t", 10, "concurrent threads to use")

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
