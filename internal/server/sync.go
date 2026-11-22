package server

import (
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/cache"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model"
	"github.com/ananthakumaran/paisa/internal/pricesync"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SyncRequest struct {
	Journal    bool `json:"journal"`
	Prices     bool `json:"prices"`
	Portfolios bool `json:"portfolios"`
	Background bool `json:"background"`
	Full       bool `json:"full"`
}

// priceSyncOutcome runs a price update and reports the outcome. CII failures
// are warnings, not price failures.
func priceSyncOutcome(db *gorm.DB, full bool) pricesync.Outcome {
	result := model.SyncCommoditiesWithOptions(db, model.SyncCommoditiesOptions{Full: full})

	warnings := []string{}
	if config.SupportsIndiaTaxFeatures() {
		if err := model.SyncCII(db); err != nil {
			warnings = append(warnings, err.Error())
		}
	}

	return pricesync.OutcomeFromResults(result.Succeeded, result.Failed, result.Errors, warnings)
}

// runSync executes every requested sync step and combines their outcomes into
// a single report.
func runSync(db *gorm.DB, request SyncRequest) pricesync.Outcome {
	outcome := pricesync.Outcome{Success: true}
	var errors []string

	if request.Journal {
		message, err := model.SyncJournal(db)
		if err != nil {
			errors = append(errors, message)
		}
	}

	if request.Prices {
		price := priceSyncOutcome(db, request.Full)
		outcome.Partial = price.Partial
		outcome.FailedCommodities = price.FailedCommodities
		outcome.Warning = price.Warning
		if price.Error != "" {
			errors = append(errors, price.Error)
		}
	}

	if request.Portfolios {
		if err := model.SyncPortfolios(db); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		outcome.Success = false
		outcome.Error = strings.Join(errors, "\n")
	}

	return outcome
}

func Sync(db *gorm.DB, request SyncRequest) gin.H {
	var outcome pricesync.Outcome
	run := func() { outcome = runSync(db, request) }
	if request.Prices {
		_ = pricesync.DefaultRunner.Run(func() error { run(); return nil })
	} else {
		run()
	}
	cache.Clear()
	cache.MarkSynced()
	return gin.H{
		"success":            outcome.Success,
		"message":            outcome.Error,
		"partial":            outcome.Partial,
		"failed_commodities": outcome.FailedCommodities,
		"warning":            outcome.Warning,
	}
}

// NewPriceManager builds a price update manager whose sync function runs a
// price update and clears the cache once it completes.
func NewPriceManager(db *gorm.DB, store pricesync.StateStore) *pricesync.Manager {
	return pricesync.New(db, config.GetConfig, store,
		func(opts pricesync.SyncOptions) pricesync.Outcome {
			outcome := priceSyncOutcome(db, opts.Full)
			cache.Clear()
			cache.MarkSynced()
			return outcome
		}, pricesync.DefaultRunner, time.Now)
}
