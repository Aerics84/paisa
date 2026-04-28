package server

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func germanyTaxPosting(date string, account string, commodity string, quantity string, amount string) posting.Posting {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}

	return posting.Posting{
		Date:      parsed,
		Account:   account,
		Commodity: commodity,
		Quantity:  decimal.RequireFromString(quantity),
		Amount:    decimal.RequireFromString(amount),
	}
}

func TestComputeGermanyTaxByYearUsesFIFOGains(t *testing.T) {
	postings := []posting.Posting{
		germanyTaxPosting("2024-01-10", "Assets:Equity:VWCE", "VWCE", "10", "1000"),
		germanyTaxPosting("2024-02-15", "Assets:Equity:VWCE", "VWCE", "-4", "-520"),
		germanyTaxPosting("2024-03-01", "Assets:Equity:VWCE", "VWCE", "2", "260"),
		germanyTaxPosting("2025-01-05", "Assets:Equity:VWCE", "VWCE", "-3", "-390"),
	}

	byYear := computeGermanyTaxByYear("Assets:Equity:VWCE", postings)

	assert.Len(t, byYear, 2)
	assert.Equal(t, "Assets:Equity:VWCE", byYear["2024"].Account)
	assert.True(t, byYear["2024"].Units.Equal(decimal.RequireFromString("4")))
	assert.True(t, byYear["2024"].PurchasePrice.Equal(decimal.RequireFromString("400")))
	assert.True(t, byYear["2024"].SellPrice.Equal(decimal.RequireFromString("520")))
	assert.True(t, byYear["2024"].RealizedGain.Equal(decimal.RequireFromString("120")))

	assert.True(t, byYear["2025"].Units.Equal(decimal.RequireFromString("3")))
	assert.True(t, byYear["2025"].PurchasePrice.Equal(decimal.RequireFromString("300")))
	assert.True(t, byYear["2025"].SellPrice.Equal(decimal.RequireFromString("390")))
	assert.True(t, byYear["2025"].RealizedGain.Equal(decimal.RequireFromString("90")))
}
