package cli

import (
	"github.com/ALT-F4-LLC/build-configs/internal/config"
	"github.com/spf13/cobra"
)

var (
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
	rootCmd.PersistentFlags().BoolVarP(&config.Debug, "debug", "D", false, "debug mode")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config-file", "c", "", "path to the config file")
}
