package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "git"

var versionCmd = &cobra.Command{
	Use: "version",
	Short: "Shows the version number of build-configs, then exits.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("build-configs version '%s'\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
