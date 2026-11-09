package server

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func creditCardTestDate(t *testing.T, date string) time.Time {
	t.Helper()

	parsed, err := time.ParseInLocation("2006-01-02", date, config.TimeZone())
	require.NoError(t, err, "test date %s must be valid", date)
	return parsed
}

func TestComputeBillsTreatsRoundedBillRemainderAsPaid(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
`), ""))

	creditCard := config.CreditCard{
		Account:         "Liabilities:CreditCard:Visa",
		StatementEndDay: 15,
		DueDay:          25,
	}

	postings := []posting.Posting{
		{
			TransactionID: "purchase",
			Date:          creditCardTestDate(t, "2022-01-10"),
			Account:       creditCard.Account,
			Commodity:     "EUR",
			Quantity:      decimal.RequireFromString("-1000"),
			Amount:        decimal.RequireFromString("-1000"),
		},
		{
			TransactionID: "payment",
			Date:          creditCardTestDate(t, "2022-02-10"),
			Account:       creditCard.Account,
			Commodity:     "EUR",
			Quantity:      decimal.RequireFromString("999.60"),
			Amount:        decimal.RequireFromString("999.60"),
		},
	}

	bills := computeBills(nil, creditCard, postings, false)
	require.Len(t, bills, 2)
	require.NotNil(t, bills[0].PaidDate, "a rounded payment within one currency unit must mark the bill as paid")
	require.True(t, bills[0].PaidDate.Equal(creditCardTestDate(t, "2022-02-10")))
}
