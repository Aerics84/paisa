package model

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/ledger"
	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/cii"
	"github.com/ananthakumaran/paisa/internal/model/commodity"
	"github.com/ananthakumaran/paisa/internal/model/import_rule"
	mutualfundModel "github.com/ananthakumaran/paisa/internal/model/mutualfund/scheme"
	npsModel "github.com/ananthakumaran/paisa/internal/model/nps/scheme"
	"github.com/ananthakumaran/paisa/internal/model/portfolio"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/scraper"
	"github.com/ananthakumaran/paisa/internal/scraper/india"
	"github.com/ananthakumaran/paisa/internal/scraper/mutualfund"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) {
	db.AutoMigrate(&npsModel.Scheme{})
	db.AutoMigrate(&mutualfundModel.Scheme{})
	db.AutoMigrate(&posting.Posting{})
	db.AutoMigrate(&price.Price{})
	db.AutoMigrate(&portfolio.Portfolio{})
	db.AutoMigrate(&price.Price{})
	db.AutoMigrate(&cii.CII{})
	db.AutoMigrate(&import_rule.Rule{})
	db.AutoMigrate(&cache.Cache{})
}

func SyncJournal(db *gorm.DB) (string, error) {
	AutoMigrate(db)
	log.Info("Syncing transactions from journal")

	errors, _, err := ledger.Cli().ValidateFile(config.GetJournalPath())
	if err != nil {

		if len(errors) == 0 {
			return err.Error(), err
		}

		var message strings.Builder
		for _, error := range errors {
			message.WriteString(error.String() + "\n\n")
		}
		return strings.TrimRight(message.String(), "\n"), err
	}

	prices, err := ledger.Cli().Prices(config.GetJournalPath())
	if err != nil {
		return err.Error(), err
	}

	if err := price.UpsertAllByType(db, config.Unknown, prices); err != nil {
		return err.Error(), err
	}

	postings, err := ledger.Cli().Parse(config.GetJournalPath(), prices)
	if err != nil {
		return err.Error(), err
	}
	posting.UpsertAll(db, postings)

	return "", nil
}

// getProvider resolves a configured provider code. It is a variable so tests
// can substitute stub providers.
var getProvider = scraper.GetProviderByCode

// SyncCommoditiesOptions controls a commodity price sync.
type SyncCommoditiesOptions struct {
	// Full forces a complete refresh instead of an incremental one.
	Full bool
	// Now bounds plausible provider dates and defaults to utils.Now().
	Now time.Time
	// Concurrency is the number of commodities fetched in parallel. It
	// defaults to 4.
	Concurrency int
}

// CommoditySyncResult reports per-commodity outcomes of a sync.
type CommoditySyncResult struct {
	Succeeded []string
	Failed    []string
	Errors    []string
}

// Err joins the per-commodity errors, or returns nil when every commodity was
// synced.
func (r CommoditySyncResult) Err() error {
	if len(r.Errors) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(r.Errors, "\n"))
}

func SyncCommodities(db *gorm.DB) error {
	return SyncCommoditiesWithOptions(db, SyncCommoditiesOptions{Full: true}).Err()
}

func SyncCommoditiesWithOptions(db *gorm.DB, opts SyncCommoditiesOptions) CommoditySyncResult {
	return syncCommodities(db, commodity.All(), opts)
}

func syncCommodities(db *gorm.DB, all []config.Commodity, opts SyncCommoditiesOptions) CommoditySyncResult {
	AutoMigrate(db)
	log.Info("Fetching commodities price history")

	now := opts.Now
	if now.IsZero() {
		now = utils.Now()
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}

	commodities := lo.Shuffle(all)
	jobs := make(chan config.Commodity)
	var wg sync.WaitGroup
	var mu sync.Mutex
	result := CommoditySyncResult{}

	worker := func() {
		defer wg.Done()
		for commodity := range jobs {
			name, err := syncCommodity(db, commodity, opts.Full, now)
			mu.Lock()
			if err != nil {
				log.Error(err)
				result.Failed = append(result.Failed, commodity.Name)
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.Succeeded = append(result.Succeeded, name)
			}
			mu.Unlock()
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}

	for _, commodity := range commodities {
		jobs <- commodity
	}
	close(jobs)
	wg.Wait()

	return result
}

func syncCommodity(db *gorm.DB, commodity config.Commodity, full bool, now time.Time) (string, error) {
	name := commodity.Name
	code := commodity.Price.Code
	log.Info("Fetching commodity ", name)

	provider := getProvider(commodity.Price.Provider)
	if provider == nil {
		return "", fmt.Errorf("Failed to fetch price for %s: unknown price provider %q", name, commodity.Price.Provider)
	}

	prices, replaceAll, err := fetchCommodityPrices(db, commodity, provider, full, now)
	if err != nil {
		return "", fmt.Errorf("Failed to fetch price for %s: %w", name, err)
	}

	if err := price.UpsertMerged(db, commodity.Type, name, code, prices, replaceAll); err != nil {
		return "", fmt.Errorf("Failed to persist price for %s: %w", name, err)
	}

	return name, nil
}

// fetchCommodityPrices returns the validated prices to persist and whether
// they replace the stored history. Incremental syncs merge so a partial
// provider response can never delete rows it omitted.
func fetchCommodityPrices(db *gorm.DB, commodity config.Commodity, provider price.PriceProvider, full bool, now time.Time) ([]*price.Price, bool, error) {
	name := commodity.Name
	code := commodity.Price.Code

	if full {
		prices, err := provider.GetPrices(code, name)
		if err != nil {
			return nil, false, err
		}
		validated, err := price.ValidatePrices(prices, price.ValidateOptions{Now: now})
		if err != nil {
			return nil, false, err
		}
		return validated, true, nil
	}

	latest, ok, err := price.LatestDate(db, commodity.Type, name, code)
	if err != nil {
		return nil, false, err
	}

	if !ok {
		// Nothing stored yet, so there is no incremental window. Fetch the
		// available history and merge it.
		prices, err := provider.GetPrices(code, name)
		if err != nil {
			return nil, false, err
		}
		validated, err := price.ValidatePrices(prices, price.ValidateOptions{Now: now})
		if err != nil {
			return nil, false, err
		}
		return validated, false, nil
	}

	since := latest.AddDate(0, 0, -7)

	prices, err := fetchSince(provider, code, name, since)
	if err != nil {
		return nil, false, err
	}

	validated, err := price.ValidatePrices(prices, price.ValidateOptions{Since: since, Now: now, RequireSince: true})
	if err == nil {
		return lo.Filter(validated, func(p *price.Price, _ int) bool {
			return !p.Date.Before(since)
		}), false, nil
	}

	// The provider returned nothing usable for the window, for example
	// because it does not support range requests. Fall back to a full fetch,
	// but only accept it if it still covers the requested window.
	log.Infof("Incremental fetch for %s failed (%s), falling back to full history", name, err)
	prices, err = provider.GetPrices(code, name)
	if err != nil {
		return nil, false, err
	}
	validated, err = price.ValidatePrices(prices, price.ValidateOptions{Since: since, Now: now, RequireSince: true})
	if err != nil {
		return nil, false, err
	}
	return validated, false, nil
}

func fetchSince(provider price.PriceProvider, code string, name string, since time.Time) ([]*price.Price, error) {
	if rangeProvider, ok := provider.(price.RangeProvider); ok {
		return rangeProvider.GetPricesSince(code, name, since)
	}
	return provider.GetPrices(code, name)
}

func SyncCII(db *gorm.DB) error {
	AutoMigrate(db)
	log.Info("Fetching taxation related info")
	ciis, err := india.GetCostInflationIndex()
	if err != nil {
		log.Error(err)
		return fmt.Errorf("Failed to fetch CII: %w", err)
	}
	cii.UpsertAll(db, ciis)
	return nil
}

func SyncPortfolios(db *gorm.DB) error {
	db.AutoMigrate(&portfolio.Portfolio{})
	log.Info("Fetching commodities portfolio")
	commodities := commodity.FindByType(config.MutualFund)
	for _, commodity := range commodities {
		if commodity.Price.Provider != "in-mfapi" {
			continue
		}

		name := commodity.Name
		log.Info("Fetching portfolio for ", name)
		portfolios, err := mutualfund.GetPortfolio(commodity.Price.Code, commodity.Name)

		if err != nil {
			log.Error(err)
			return fmt.Errorf("Failed to fetch portfolio for %s: %w", name, err)
		}

		portfolio.UpsertAll(db, commodity.Type, commodity.Price.Code, portfolios)
	}
	return nil
}
