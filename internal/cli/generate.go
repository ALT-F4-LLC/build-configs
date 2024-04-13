package cli

import (
	"fmt"

	"github.com/ALT-F4-LLC/build-configs/internal/config"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generates configuration from a project config file.",

	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.New(configFile)
		if err != nil {
			return fmt.Errorf("could not load config: %v", err)
		}



		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
