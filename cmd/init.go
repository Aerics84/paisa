package cmd

import (
	"os"
	"path/filepath"

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

		journalPath := filepath.Join(cwd, "main.ledger")
		if info, err := os.Stat(journalPath); err == nil {
			if info.Size() > 0 {
				log.Fatalf("%s already contains data; refusing to overwrite it. Delete or move the file first if you really want to regenerate the demo.", journalPath)
			}
		} else if !os.IsNotExist(err) {
			log.Fatal(err)
		}

		if err := generator.DemoForProfile(cwd, profile); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	initCmd.Flags().StringVar(&initRegionalProfile, "regional-profile", string(config.RegionalProfileGermanyEU), "Regional profile for generated starter content")
	rootCmd.AddCommand(initCmd)
}
