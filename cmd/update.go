package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/cache"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model"
	"github.com/ananthakumaran/paisa/internal/pricesync"
	"github.com/ananthakumaran/paisa/internal/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var updateJournal bool
var updateCommodities bool
var updatePortfolios bool

// recordPriceUpdate stores the outcome of a CLI price run in the state that is
// shared with the server. A load error is logged and the state is written from
// scratch; a save error is returned so the caller can report it.
func recordPriceUpdate(store pricesync.StateStore, started, finished time.Time, full bool, outcome pricesync.Outcome) error {
	state, err := store.Load()
	if err != nil {
		log.Warnf("Failed to load price update state: %v", err)
		state = pricesync.State{}
	}

	state.LastAttemptAt = started
	state.LastDurationMS = finished.Sub(started).Milliseconds()
	state.LastFull = full
	state.LastError = outcome.Error
	state.LastWarning = outcome.Warning
	state.FailedCommodities = outcome.FailedCommodities

	switch {
	case outcome.Success:
		state.LastSuccessAt = finished
		state.LastPartialSuccessAt = time.Time{}
	case outcome.Partial:
		state.LastPartialSuccessAt = finished
	}

	state.LastJobSuccess = outcome.Success
	state.LastJobPartial = outcome.Partial

	return store.Save(state)
}

// priceSyncOutcomeForCLI runs a price update and reports the outcome. CII
// failures are warnings, not price failures.
func priceSyncOutcomeForCLI(db *gorm.DB, includeCII bool) pricesync.Outcome {
	result := model.SyncCommoditiesWithOptions(db, model.SyncCommoditiesOptions{Full: true})

	warnings := []string{}
	if includeCII && config.SupportsIndiaTaxFeatures() {
		if err := model.SyncCII(db); err != nil {
			log.Warn(err)
			warnings = append(warnings, err.Error())
		}
	}

	return pricesync.OutcomeFromResults(result.Succeeded, result.Failed, result.Errors, warnings)
}

// priceFailureMessage returns the message used to exit non-zero after a failed
// price run. Per-commodity errors are preferred; when the outcome has no error
// text the failed commodity names are named instead.
func priceFailureMessage(outcome pricesync.Outcome) string {
	if outcome.Error != "" {
		return outcome.Error
	}

	if len(outcome.FailedCommodities) > 0 {
		return fmt.Sprintf("Failed to update prices for: %s", strings.Join(outcome.FailedCommodities, ", "))
	}

	return "Price update failed"
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Sync journal data",
	Run: func(cmd *cobra.Command, args []string) {
		db, err := utils.OpenDB()
		if err != nil {
			log.Fatal(err)
		}

		syncAll := !updateJournal && !updateCommodities && !updatePortfolios
		failure := ""

		if syncAll || updateJournal {
			message, err := model.SyncJournal(db)
			if err != nil {
				if strings.TrimSpace(message) == "" {
					message = err.Error()
				}
				// Prices are upserted before the postings are parsed, so a
				// journal failure can still have changed the database.
				cache.MarkSynced()
				log.Fatal(message)
			}
		}

		if syncAll || updateCommodities {
			store := pricesync.NewFileStateStore(pricesync.StatePathForDB(config.GetDBPath()))
			started := time.Now()
			outcome := pricesync.Outcome{}
			_ = pricesync.DefaultRunner.Run(func() error {
				outcome = priceSyncOutcomeForCLI(db, syncAll)
				return nil
			})

			if err := recordPriceUpdate(store, started, time.Now(), true, outcome); err != nil {
				log.Warn(err)
			}

			if !outcome.Success {
				failure = priceFailureMessage(outcome)
			}
		}

		if syncAll || updatePortfolios {
			if err := model.SyncPortfolios(db); err != nil {
				if failure == "" {
					failure = err.Error()
				} else {
					log.Error(err)
				}
			}
		}

		// Tell a running `paisa serve` that the data changed, so it drops
		// its in-memory caches without a restart or an API call.
		cache.MarkSynced()

		if failure != "" {
			log.Fatal(failure)
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVarP(&updateJournal, "journal", "j", false, "update journal")
	updateCmd.Flags().BoolVarP(&updateCommodities, "commodity", "c", false, "update commodities")
	updateCmd.Flags().BoolVarP(&updatePortfolios, "portfolio", "p", false, "update mutualfund portfolios")
}
