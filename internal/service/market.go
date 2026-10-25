package service

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/google/btree"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type priceCache struct {
	pricesTree        map[string]*btree.BTree
	postingPricesTree map[string]*btree.BTree
}

var pcache atomic.Pointer[priceCache]
var pcacheLock sync.Mutex

func loadPriceCache(db *gorm.DB) *priceCache {
	var prices []price.Price
	result := db.Where("commodity_type != ?", config.Unknown).Find(&prices)
	if result.Error != nil {
		log.Fatal(result.Error)
	}
	cache := &priceCache{
		pricesTree:        make(map[string]*btree.BTree),
		postingPricesTree: make(map[string]*btree.BTree),
	}

	for _, price := range prices {
		if cache.pricesTree[price.CommodityName] == nil {
			cache.pricesTree[price.CommodityName] = btree.New(2)
		}

		cache.pricesTree[price.CommodityName].ReplaceOrInsert(price)
	}

	var postings []posting.Posting
	result = db.Find(&postings)
	if result.Error != nil {
		log.Fatal(result.Error)
	}

	for commodityName, postings := range lo.GroupBy(postings, func(p posting.Posting) string { return p.Commodity }) {
		if !utils.IsCurrency(postings[0].Commodity) {
			result := db.Where("commodity_type = ? and commodity_name = ?", config.Unknown, commodityName).Find(&prices)
			if result.Error != nil {
				log.Fatal(result.Error)
			}

			postingPricesTree := btree.New(2)
			for _, price := range prices {
				postingPricesTree.ReplaceOrInsert(price)
			}
			cache.postingPricesTree[commodityName] = postingPricesTree

			if cache.pricesTree[commodityName] == nil {
				cache.pricesTree[commodityName] = postingPricesTree
			}
		}
	}

	return cache
}

func ensurePriceCache(db *gorm.DB) *priceCache {
	if cache := pcache.Load(); cache != nil {
		return cache
	}

	pcacheLock.Lock()
	defer pcacheLock.Unlock()

	if cache := pcache.Load(); cache != nil {
		return cache
	}

	cache := loadPriceCache(db)
	pcache.Store(cache)
	return cache
}

func ClearPriceCache() {
	pcache.Store(nil)
}

// FindUnitPrice returns the last stored quote on or before date together with
// its actual quote date. It reports false instead of terminating when a
// commodity has no usable price series, so callers can treat missing coverage
// as unavailable.
func FindUnitPrice(db *gorm.DB, commodity string, date time.Time) (price.Price, bool) {
	cache := ensurePriceCache(db)

	pt := cache.pricesTree[commodity]
	if pt != nil {
		pc := utils.BTreeDescendFirstLessOrEqual(pt, price.Price{Date: date})
		if !pc.Value.Equal(decimal.Zero) {
			return pc, true
		}
	}

	pt = cache.postingPricesTree[commodity]
	if pt != nil {
		pc := utils.BTreeDescendFirstLessOrEqual(pt, price.Price{Date: date})
		if !pc.Value.Equal(decimal.Zero) {
			return pc, true
		}
	}

	return price.Price{}, false
}

func GetUnitPrice(db *gorm.DB, commodity string, date time.Time) price.Price {
	pc, found := FindUnitPrice(db, commodity, date)
	if !found {
		// Returning an empty price lets valuation callers fall back to the
		// posting amount instead of terminating the whole server.
		log.WithField("commodity", commodity).Warn("Price not found")
		return price.Price{}
	}
	return pc
}

func GetAllPrices(db *gorm.DB, commodity string) []price.Price {
	cache := ensurePriceCache(db)

	pmap := make(map[string]price.Price)

	pt := cache.postingPricesTree[commodity]
	if pt == nil {
		log.WithField("commodity", commodity).Warn("Posting price not found")
	} else {
		for _, price := range utils.BTreeToSlice[price.Price](pt) {
			pmap[price.Date.String()] = price
		}
	}

	pt = cache.pricesTree[commodity]
	if pt == nil {
		log.WithField("commodity", commodity).Warn("Price not found")
	} else {
		for _, price := range utils.BTreeToSlice[price.Price](pt) {
			pmap[price.Date.String()] = price
		}
	}

	prices := []price.Price{}
	keys := lo.Keys(pmap)
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, key := range keys {
		prices = append(prices, pmap[key])
	}

	return prices
}

func GetMarketPrice(db *gorm.DB, p posting.Posting, date time.Time) decimal.Decimal {
	if utils.IsCurrency(p.Commodity) {
		return p.Amount
	}

	pc := GetUnitPrice(db, p.Commodity, date)
	if !pc.Value.Equal(decimal.Zero) {
		return p.Quantity.Mul(pc.Value)
	}

	return p.Amount
}

func GetPrice(db *gorm.DB, commodity string, quantity decimal.Decimal, date time.Time) decimal.Decimal {
	if utils.IsCurrency(commodity) {
		return quantity
	}

	pc := GetUnitPrice(db, commodity, date)
	if !pc.Value.Equal(decimal.Zero) {
		return quantity.Mul(pc.Value)
	}

	return quantity
}

func PopulateMarketPrice(db *gorm.DB, ps []posting.Posting) []posting.Posting {
	date := utils.EndOfToday()
	return lo.Map(ps, func(p posting.Posting, _ int) posting.Posting {
		p.MarketAmount = GetMarketPrice(db, p, date)
		return p
	})
}
