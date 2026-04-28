package cmd

import (
	"fmt"

	"github.com/ananthakumaran/paisa/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Version:", version.String())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
