package taxation

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestCalculateIndiaEquityLongTermTax(t *testing.T) {
	tax := Calculate(nil,
		decimal.NewFromInt(10),
		config.Commodity{TaxCategory: config.Equity65},
		decimal.NewFromInt(100),
		time.Date(2022, time.January, 10, 0, 0, 0, 0, time.UTC),
		decimal.NewFromInt(120),
		time.Date(2023, time.March, 15, 0, 0, 0, 0, time.UTC),
	)

	assert.True(t, tax.Gain.Equal(decimal.NewFromInt(200)))
	assert.True(t, tax.Taxable.Equal(decimal.NewFromInt(200)))
	assert.True(t, tax.LongTerm.Equal(decimal.NewFromInt(20)))
	assert.True(t, tax.ShortTerm.Equal(decimal.Zero))
	assert.True(t, tax.Slab.Equal(decimal.Zero))
}

func TestCalculateIndiaDebtShortTermUsesSlab(t *testing.T) {
	tax := Calculate(nil,
		decimal.NewFromInt(5),
		config.Commodity{TaxCategory: config.Debt},
		decimal.NewFromInt(100),
		time.Date(2024, time.January, 10, 0, 0, 0, 0, time.UTC),
		decimal.NewFromInt(130),
		time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC),
	)

	assert.True(t, tax.Gain.Equal(decimal.NewFromInt(150)))
	assert.True(t, tax.Taxable.Equal(decimal.NewFromInt(150)))
	assert.True(t, tax.Slab.Equal(decimal.NewFromInt(150)))
	assert.True(t, tax.LongTerm.Equal(decimal.Zero))
	assert.True(t, tax.ShortTerm.Equal(decimal.Zero))
}

func equityTax(db *gorm.DB, quantity, purchasePrice decimal.Decimal, purchaseDate time.Time, sellPrice decimal.Decimal, sellDate time.Time) Tax {
	return Calculate(db,
		quantity,
		config.Commodity{Name: "TEST_EQUITY", TaxCategory: config.Equity},
		purchasePrice,
		purchaseDate,
		sellPrice,
		sellDate,
	)
}

func TestEquityLongTermExemptThroughMarch2018(t *testing.T) {
	db := taxTestDB(t, "TEST_EQUITY", "100")

	tax := equityTax(db,
		decimal.NewFromInt(10),
		decimal.NewFromInt(100), taxTestDate(2016, time.January, 1),
		decimal.NewFromInt(150), taxTestDate(2018, time.March, 31),
	)

	assert.True(t, tax.Taxable.IsZero(), "sale before 2018-04-01 must stay exempt")
	assert.True(t, tax.LongTerm.IsZero())
}

func TestEquityLongTermTaxedFromApril2018(t *testing.T) {
	db := taxTestDB(t, "TEST_EQUITY", "100")

	tax := equityTax(db,
		decimal.NewFromInt(10),
		decimal.NewFromInt(100), taxTestDate(2016, time.January, 1),
		decimal.NewFromInt(150), taxTestDate(2018, time.April, 1),
	)

	assert.True(t, tax.Taxable.Equal(decimal.NewFromInt(500)))
	assert.True(t, tax.LongTerm.Equal(decimal.NewFromInt(50)))
}

func TestEquityRateChange(t *testing.T) {
	tax := equityTax(nil,
		decimal.NewFromInt(10),
		decimal.NewFromInt(100), taxTestDate(2020, time.January, 1),
		decimal.NewFromInt(200), taxTestDate(2024, time.July, 22),
	)

	assert.True(t, tax.LongTerm.Equal(decimal.NewFromInt(100)))
}

func TestEquityRateChangeOnDate(t *testing.T) {
	tax := equityTax(nil,
		decimal.NewFromInt(10),
		decimal.NewFromInt(100), taxTestDate(2020, time.January, 1),
		decimal.NewFromInt(200), taxTestDate(2024, time.July, 23),
	)

	assert.True(t, tax.LongTerm.Equal(decimal.NewFromInt(125)))
}

func TestEquityShortTermBeforeFebruary2018IsTaxed(t *testing.T) {
	db := taxTestDB(t, "TEST_EQUITY", "100")

	tax := equityTax(db,
		decimal.NewFromInt(10),
		decimal.NewFromInt(100), taxTestDate(2017, time.December, 1),
		decimal.NewFromInt(120), taxTestDate(2018, time.January, 15),
	)

	assert.True(t, tax.ShortTerm.Equal(decimal.NewFromInt(30)))
}

func TestGrandfatheringCapsAtSalePrice(t *testing.T) {
	db := taxTestDB(t, "TEST_EQUITY", "150")

	tax := equityTax(db,
		decimal.NewFromInt(10),
		decimal.NewFromInt(100), taxTestDate(2010, time.January, 1),
		decimal.NewFromInt(120), taxTestDate(2020, time.January, 1),
	)

	assert.True(t, tax.Taxable.IsZero(), "capped grandfathering must not create an artificial loss")
}

func TestDebtBoughtAfterApril2023IsNotIndexed(t *testing.T) {
	tax := Calculate(nil,
		decimal.NewFromInt(10),
		config.Commodity{TaxCategory: config.Debt},
		decimal.NewFromInt(100), taxTestDate(2023, time.April, 1),
		decimal.NewFromInt(200), taxTestDate(2026, time.April, 2),
	)

	assert.True(t, tax.Taxable.Equal(decimal.NewFromInt(1000)))
	assert.True(t, tax.Slab.Equal(decimal.NewFromInt(1000)))
}

func TestCalendarYearsExceedUsesCalendarYears(t *testing.T) {
	assert.False(t, calendarYearsExceed(
		taxTestDate(2020, time.January, 1),
		taxTestDate(2021, time.January, 1),
		1,
	))
}

func TestCalendarYearsExceedNextDay(t *testing.T) {
	assert.True(t, calendarYearsExceed(
		taxTestDate(2020, time.January, 1),
		taxTestDate(2021, time.January, 2),
		1,
	))
}

func TestGrandfatheredPurchasePrice(t *testing.T) {
	cases := []struct {
		name           string
		actual, fmv    decimal.Decimal
		sale, expected decimal.Decimal
	}{
		{"caps at sale price above actual", decimal.NewFromInt(100), decimal.NewFromInt(150), decimal.NewFromInt(120), decimal.NewFromInt(120)},
		{"keeps actual when fmv is lower", decimal.NewFromInt(100), decimal.NewFromInt(80), decimal.NewFromInt(70), decimal.NewFromInt(100)},
		{"keeps actual when fmv is missing", decimal.NewFromInt(100), decimal.Zero, decimal.NewFromInt(120), decimal.NewFromInt(100)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, grandfatheredPurchasePrice(tc.actual, tc.fmv, tc.sale).Equal(tc.expected))
		})
	}
}
