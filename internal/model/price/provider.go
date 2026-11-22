package price

import (
	"time"

	"gorm.io/gorm"
)

type AutoCompleteItem struct {
	Label string `json:"label"`
	ID    string `json:"id"`
}

type AutoCompleteField struct {
	Label     string `json:"label"`
	ID        string `json:"id"`
	Help      string `json:"help"`
	InputType string `json:"inputType"`
}

type PriceProvider interface {
	Code() string
	Label() string
	Description() string
	AutoCompleteFields() []AutoCompleteField
	AutoComplete(db *gorm.DB, field string, filter map[string]string) []AutoCompleteItem
	ClearCache(db *gorm.DB)
	GetPrices(code string, commodityName string) ([]*Price, error)
}

// RangeProvider is an optional capability for providers that can fetch only
// the prices on or after a given date. The sync engine uses it for incremental
// updates; providers without it fall back to GetPrices.
type RangeProvider interface {
	GetPricesSince(code string, commodityName string, since time.Time) ([]*Price, error)
}
