package server

import (
	"testing"

	"github.com/ananthakumaran/paisa/internal/taxation"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestComputeCapitalGainTotalsNetsAccountsAndFloorsTax(t *testing.T) {
	dec := decimal.RequireFromString
	gains := map[string]CapitalGain{
		"a": {FY: map[string]FYCapitalGain{"2024-25": {Tax: taxation.Tax{LongTerm: dec("100"), ShortTerm: dec("50"), Taxable: dec("1000")}}}},
		"b": {FY: map[string]FYCapitalGain{"2024-25": {Tax: taxation.Tax{LongTerm: dec("-150"), ShortTerm: dec("-20"), Taxable: dec("-500")}}}},
	}

	totals := ComputeCapitalGainTotals(gains)

	assert.True(t, totals["2024-25"].Tax.LongTerm.IsZero(), "long term tax floors at zero")
	assert.True(t, totals["2024-25"].Tax.ShortTerm.Equal(dec("30")), "short term tax nets across accounts")
	assert.True(t, totals["2024-25"].Tax.Taxable.Equal(dec("500")), "taxable stays signed")
}

func TestComputeCapitalGainTotalsSumsUnitsAndPrices(t *testing.T) {
	dec := decimal.RequireFromString
	gains := map[string]CapitalGain{
		"a": {FY: map[string]FYCapitalGain{"2024-25": {Units: dec("10"), PurchasePrice: dec("1000"), SellPrice: dec("1500")}}},
		"b": {FY: map[string]FYCapitalGain{"2024-25": {Units: dec("5"), PurchasePrice: dec("700"), SellPrice: dec("200")}}},
	}

	totals := ComputeCapitalGainTotals(gains)

	assert.True(t, totals["2024-25"].Units.Equal(dec("15")))
	assert.True(t, totals["2024-25"].PurchasePrice.Equal(dec("1700")))
	assert.True(t, totals["2024-25"].SellPrice.Equal(dec("1700")))
}
