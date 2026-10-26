package main

import (
	"fmt"
	"os"

	covers "golrc/coverpull"
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

		// threads, _ := cmd.Flags().GetInt("threads")

		lrcgen.Run(cmd, file)
	},
}

// var coverCmd = &cobra.Command{
// 	Use: "cover",
// 	Short: "manipulate cover.jpg files",
// 	Example: "golrc cover gen /path/to/music",
// 	Args: cobra.ExactArgs(1),
// 	Run: func(cmd *cobra.Command, args []string) {
// 		file := args[0]
// 		// threads, _ := cmd.Flags().GetInt("threads")
//
// 		covers.Run(cmd, file)
// 	},
// }
//
var coverCmd = &cobra.Command{
	Use:   "cover",
	Short: "Cover art tools",
}

var coverGenCmd = &cobra.Command{
	Use:   "gen [dir]",
	Short: "Extract embedded album art to cover.jpg",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return covers.Run(cmd, args[0])
	},
}

var allCommands = []*cobra.Command{generateCmd, coverCmd}

func init() {
	generateCmd.Flags().IntP("threads", "t", 10, "concurrent threads to use")
	generateCmd.Flags().BoolP("rename", "r", false, "rename the file: \"XX - Title\"")

	coverGenCmd.Flags().IntP("threads", "t", 10, "concurrent threads to use")
	coverGenCmd.Flags().BoolP("replace", "r", false, "replace existing cover.jpg")

	coverCmd.AddCommand(coverGenCmd)
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
