package cmd

import (
	"os"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/generator"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var initRegionalProfile string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generates a sample config and journal file",
	Run: func(cmd *cobra.Command, args []string) {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}

		profile := config.RegionalProfileType(initRegionalProfile)
		if !config.IsSupportedRegionalProfile(profile) {
			log.Fatalf("Unsupported regional profile: %s", initRegionalProfile)
		}

		generator.DemoForProfile(cwd, profile)
	},
}

func init() {
	initCmd.Flags().StringVar(&initRegionalProfile, "regional-profile", string(config.RegionalProfileIndia), "Regional profile for generated starter content")
	rootCmd.AddCommand(initCmd)
}
