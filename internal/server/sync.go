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

// runSync executes the requested sync steps in order. A failure stops the
// remaining steps, preserving the legacy sync contract: a broken journal must
// not trigger a price or portfolio run.
func runSync(db *gorm.DB, request SyncRequest) pricesync.Outcome {
	warning := ""

	if request.Journal {
		message, err := model.SyncJournal(db)
		if err != nil {
			if strings.TrimSpace(message) == "" {
				message = err.Error()
			}
			return pricesync.Outcome{Success: false, Error: message}
		}
	}

	if request.Prices {
		price := priceSyncOutcome(db, request.Full)
		if !price.Success {
			return price
		}
		warning = price.Warning
	}

	if request.Portfolios {
		if err := model.SyncPortfolios(db); err != nil {
			return pricesync.Outcome{Success: false, Error: err.Error()}
		}
	}

	return pricesync.Outcome{Success: true, Warning: warning}
}

func syncResponse(outcome pricesync.Outcome) gin.H {
	response := gin.H{"success": outcome.Success}
	if outcome.Error != "" {
		response["message"] = outcome.Error
	}
	if outcome.Partial {
		response["partial"] = true
	}
	if len(outcome.FailedCommodities) > 0 {
		response["failed_commodities"] = outcome.FailedCommodities
	}
	if outcome.Warning != "" {
		response["warning"] = outcome.Warning
	}
	return response
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
	return syncResponse(outcome)
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
