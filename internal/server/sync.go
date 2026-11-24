package server

import (
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/cache"
	"github.com/ananthakumaran/paisa/internal/model"
	"github.com/ananthakumaran/paisa/internal/pricesync"
	"github.com/gin-gonic/gin"
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
func priceSyncOutcome(deps Deps, full bool) pricesync.Outcome {
	db := deps.DB
	result := model.SyncCommoditiesWithOptions(db, model.SyncCommoditiesOptions{Full: full})

	warnings := []string{}
	if deps.Config.SupportsIndiaTaxFeatures() {
		if err := model.SyncCII(db); err != nil {
			warnings = append(warnings, err.Error())
		}
	}

	return pricesync.OutcomeFromResults(result.Succeeded, result.Failed, result.Errors, warnings)
}

// runSync executes the requested sync steps in order. A failure stops the
// remaining steps, preserving the legacy sync contract: a broken journal must
// not trigger a price or portfolio run.
func runSync(deps Deps, request SyncRequest) pricesync.Outcome {
	warning := ""

	if request.Journal {
		message, err := model.SyncJournal(deps.DB)
		if err != nil {
			if strings.TrimSpace(message) == "" {
				message = err.Error()
			}
			return pricesync.Outcome{Success: false, Error: message}
		}
	}

	if request.Prices {
		price := priceSyncOutcome(deps, request.Full)
		if !price.Success {
			return price
		}
		warning = price.Warning
	}

	if request.Portfolios {
		if err := model.SyncPortfolios(deps.DB); err != nil {
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

func Sync(deps Deps, request SyncRequest) gin.H {
	var outcome pricesync.Outcome
	run := func() { outcome = runSync(deps, request) }
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
func NewPriceManager(deps Deps, stateStore pricesync.StateStore) *pricesync.Manager {
	return pricesync.New(deps.DB, deps.Config.Get, stateStore,
		func(opts pricesync.SyncOptions) pricesync.Outcome {
			outcome := priceSyncOutcome(deps, opts.Full)
			cache.Clear()
			cache.MarkSynced()
			return outcome
		}, pricesync.DefaultRunner, time.Now)
}
