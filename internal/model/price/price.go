package price

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/google/btree"
	"github.com/shopspring/decimal"
)

type Price struct {
	ID            uint                 `gorm:"primaryKey" json:"id"`
	Date          time.Time            `gorm:"index" json:"date"`
	CommodityType config.CommodityType `json:"commodity_type"`
	CommodityID   string               `json:"commodity_id"`
	CommodityName string               `gorm:"index" json:"commodity_name"`
	Value         decimal.Decimal      `json:"value"`
}

// upsertMu serializes price writes. SQLite allows only one writer at a time,
// and all price persistence shares the same connection pool.
var upsertMu sync.Mutex

func (p Price) Less(o btree.Item) bool {
	return p.Date.Before(o.(Price).Date)
}

func DeleteAll(db *gorm.DB) error {
	err := db.Exec("DELETE FROM prices").Error
	if err != nil {
		return err
	}
	return nil
}

// LatestDate returns the most recent stored price date for a commodity.
func LatestDate(db *gorm.DB, commodityType config.CommodityType, commodityName, commodityID string) (time.Time, bool, error) {
	var latest Price
	err := db.Where("commodity_type = ? and (commodity_id = ? or commodity_name = ?)", commodityType, commodityID, commodityName).
		Order("date desc").
		Limit(1).
		Find(&latest).Error
	if err != nil {
		return time.Time{}, false, err
	}
	if latest.ID == 0 {
		return time.Time{}, false, nil
	}
	return latest.Date, true, nil
}

// UpsertMerged persists a validated price series for one commodity. When
// replaceAll is false, prices are merged into the existing history by date, so
// a partial provider response can never delete rows it omitted. An empty
// response is only rejected for a full replacement; callers that merge can
// treat it as a no-op.
func UpsertMerged(db *gorm.DB, commodityType config.CommodityType, commodityName, commodityID string, prices []*Price, replaceAll bool) error {
	upsertMu.Lock()
	defer upsertMu.Unlock()

	if len(prices) == 0 {
		if replaceAll {
			return fmt.Errorf("price: refusing to replace prices for %s with an empty response", commodityName)
		}
		return nil
	}

	merged := make(map[int64]Price, len(prices))

	if !replaceAll {
		var existing []Price
		err := db.Where("commodity_type = ? and (commodity_id = ? or commodity_name = ?)", commodityType, commodityID, commodityName).
			Find(&existing).Error
		if err != nil {
			return err
		}
		for _, p := range existing {
			merged[p.Date.UTC().UnixNano()] = p
		}
	}

	for i, p := range prices {
		if p == nil {
			return fmt.Errorf("price: nil price at index %d for %s", i, commodityName)
		}
		if p.Date.IsZero() {
			return fmt.Errorf("price: zero date at index %d for %s", i, commodityName)
		}

		row := *p
		row.ID = 0
		row.CommodityType = commodityType
		row.CommodityID = commodityID
		row.CommodityName = commodityName
		merged[row.Date.UTC().UnixNano()] = row
	}

	rows := make([]Price, 0, len(merged))
	for _, p := range merged {
		rows = append(rows, p)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Date.Before(rows[j].Date)
	})

	return db.Transaction(func(tx *gorm.DB) error {
		err := tx.Delete(&Price{}, "commodity_type = ? and (commodity_id = ? or commodity_name = ?)", commodityType, commodityID, commodityName).Error
		if err != nil {
			return err
		}

		return tx.CreateInBatches(&rows, 500).Error
	})
}

func UpsertAllByTypeNameAndID(db *gorm.DB, commodityType config.CommodityType, commodityName string, commodityID string, prices []*Price) error {
	return UpsertMerged(db, commodityType, commodityName, commodityID, prices, true)
}

// UpsertAllByType replaces every price of a commodity type. It is a full
// replacement even for an empty set: journal prices (config.Unknown) are the
// source of truth, so a journal without prices must clear the old rows.
func UpsertAllByType(db *gorm.DB, commodityType config.CommodityType, prices []Price) error {
	upsertMu.Lock()
	defer upsertMu.Unlock()

	return db.Transaction(func(tx *gorm.DB) error {
		err := tx.Delete(&Price{}, "commodity_type = ?", commodityType).Error
		if err != nil {
			return err
		}

		if len(prices) == 0 {
			return nil
		}

		return tx.CreateInBatches(&prices, 500).Error
	})
}
