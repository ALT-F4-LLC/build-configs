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
		if configFile == "" {
			configFile = config.FindConfigPath()
		}

		cfg, err := config.New(configFile)
		if err != nil {
			return fmt.Errorf("could not load config: %v", err)
		}

		t, err := cfg.GetTemplater()
		if err != nil {
			return fmt.Errorf("could not create templater: %v", err)
		}

		fmt.Printf("%+v\n", t)

		return t.Render()
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
