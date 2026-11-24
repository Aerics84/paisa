package main

import (
	"context"

	"github.com/ananthakumaran/paisa/cmd"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/migrations"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/ananthakumaran/paisa/internal/version"
	log "github.com/sirupsen/logrus"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

// App struct
type App struct {
	ctx context.Context
	db  gorm.DB
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	cmd.InitConfig()
	runtime.WindowMaximise(ctx)

	db, err := utils.OpenDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := migrations.Migrate(db, config.GetDBPath()); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	a.db = *db
}

func (a *App) Version() string {
	return version.String()
}
