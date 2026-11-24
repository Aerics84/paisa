package cmd

import (
	"os"
	"time"

	"github.com/ananthakumaran/paisa/internal/auth"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/migrations"
	"github.com/ananthakumaran/paisa/internal/server"
	"github.com/ananthakumaran/paisa/internal/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var port int
var host string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "serve the WEB UI",
	Run: func(cmd *cobra.Command, args []string) {
		db, err := utils.OpenDB()
		if err != nil {
			log.Fatal(err)
		}

		if err := migrations.Migrate(db, config.GetDBPath()); err != nil {
			log.Fatalf("Failed to migrate database: %v", err)
		}

		if os.Getenv("PAISA_DEBUG") == "true" {
			db = db.Debug()
		}

		server.Listen(server.Deps{DB: db, Config: config.DefaultStore(), Auth: auth.NewStore(24 * time.Hour)}, host, port)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVarP(&port, "port", "p", 7500, "port to listen on")
	serveCmd.Flags().StringVar(&host, "host", "", "host to listen on (default is all interfaces)")
}
