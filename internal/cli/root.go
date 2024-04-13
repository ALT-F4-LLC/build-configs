package cli

import "github.com/spf13/cobra"

var (
	debug      bool
	configFile string
)

var rootCmd = &cobra.Command{
	Use:   "build-configs",
	Short: "build-configs is a CLI for generating build configurations.",
	Long: `build-configs is an easy-to-use, standardised configuration generator
built to ease development overhead when bootstrapping and updating
configuration in ALT-F4 projects.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "D", false, "debug mode")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config-file", "c", "build-configs.json", "path to the config file")
}
