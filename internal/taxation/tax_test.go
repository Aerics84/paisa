package taxation

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
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
