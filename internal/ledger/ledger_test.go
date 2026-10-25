package ledger

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertPriceEqual(t *testing.T, actual price.Price, date string, commodityName string, value float64) {
	assert.Equal(t, commodityName, actual.CommodityName, "they should be equal")
	assert.Equal(t, date, actual.Date.Format("2006/01/02"), "they should be equal")
	assert.Equal(t, value, actual.Value.InexactFloat64(), "they should be equal")
}

func TestParseLegerPrices(t *testing.T) {
	parsedPrices, _ := parseLedgerPrices("P 2023/05/01 00:00:00 USD 0.9 EUR\n", "EUR")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "USD", 0.9)
	parsedPrices, _ = parseLedgerPrices("P 2023/05/01 00:00:00 EUR $1.1\n", "$")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", 1.1)
	parsedPrices, _ = parseLedgerPrices("P 2023/05/01 00:00:00 EUR $-1.1\n", "$")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", -1.1)
	parsedPrices, _ = parseLedgerPrices("P 2023/05/01 00:00:00 EUR ₹70\n", "₹")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", 70)

	parsedPrices, _ = parseLedgerPrices("P 2023/05/01 00:00:00 USD 0.9 EUR\n", "INR")
	assert.Len(t, parsedPrices, 0)
	parsedPrices, _ = parseLedgerPrices("P 2023/05/01 00:00:00 USD $0.9\n", "INR")
	assert.Len(t, parsedPrices, 0)

	parsedPrices, _ = parseLedgerPrices("P 2022/01/29 00:50:00 UAH 0.026 EUR\n", "EUR")
	assertPriceEqual(t, parsedPrices[0], "2022/01/29", "UAH", 0.026)
}

func TestParseHLegerPrices(t *testing.T) {
	parsedPrices, _ := parseHLedgerPrices("P 2023-05-01 USD 0.9 EUR\n", "EUR")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "USD", 0.9)
	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 EUR $1.1\n", "$")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", 1.1)

	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 EUR USD 1.1\n", "USD")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", 1.1)

	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 EUR 1.1$\n", "$")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", 1.1)

	parsedPrices, _ = parseHLedgerPrices(utils.Dos2Unix("P 2023-05-01 EUR 1.1$\r\n"), "$")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", 1.1)

	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 \"AAPL0\" \"USD0\" 45.5\n", "USD0")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "AAPL0", 45.5)

	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 USD 0.9 EUR\n", "INR")
	assert.Len(t, parsedPrices, 0)

	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 USD $0.9\n", "INR")
	assert.Len(t, parsedPrices, 0)

	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 USD $0.9\r\n", "INR")
	assert.Len(t, parsedPrices, 0)
}

func TestParseAmount(t *testing.T) {
	commodity, amount, _ := parseAmount("0.9 USD")
	assert.Equal(t, "USD", commodity)
	assert.Equal(t, 0.9, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("$0.9")
	assert.Equal(t, "$", commodity)
	assert.Equal(t, 0.9, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("0.9$")
	assert.Equal(t, "$", commodity)
	assert.Equal(t, 0.9, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("$-0.9")
	assert.Equal(t, "$", commodity)
	assert.Equal(t, -0.9, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("-0.9$")
	assert.Equal(t, "$", commodity)
	assert.Equal(t, -0.9, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("100,000 EUR")
	assert.Equal(t, "EUR", commodity)
	assert.Equal(t, 100000.0, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("1,00,000 INR")
	assert.Equal(t, "INR", commodity)
	assert.Equal(t, 100000.0, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("92,33 EUR")
	assert.Equal(t, "EUR", commodity)
	assert.Equal(t, 92.33, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("2,707819 \"EWG2LD\"")
	assert.Equal(t, "EWG2LD", commodity)
	assert.Equal(t, 2.707819, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("10.005,05 EUR")
	assert.Equal(t, "EUR", commodity)
	assert.Equal(t, 10005.05, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("100,000.00 \"EUR0-0\"")
	assert.Equal(t, "EUR0-0", commodity)
	assert.Equal(t, 100000.0, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("-100,000.00 \"EUR0-0\"")
	assert.Equal(t, "EUR0-0", commodity)
	assert.Equal(t, -100000.0, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("\"EUR0-0\" -100,000.00")
	assert.Equal(t, "EUR0-0", commodity)
	assert.Equal(t, -100000.0, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("INR 70.0099")
	assert.Equal(t, "INR", commodity)
	assert.Equal(t, 70.0099, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("1E-8 BTC")
	assert.Equal(t, "BTC", commodity)
	assert.Equal(t, 1e-08, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("100E-8    BTC")
	assert.Equal(t, "BTC", commodity)
	assert.Equal(t, 1e-06, amount.InexactFloat64())
}

func TestBuildHLedgerPostingsPrefersCostOverPrice(t *testing.T) {
	date := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	pricesTree := buildPricesTree([]price.Price{{
		Date: date, CommodityName: "USD", Value: decimal.NewFromInt(1),
	}})
	var transaction HLedgerTransaction
	require.NoError(t, json.Unmarshal([]byte(`{
		"tdescription": "401(k)", "tindex": 1, "tstatus": "Cleared",
		"tsourcepos": [{"sourceLine": 1}, {"sourceLine": 2}]
	}`), &transaction))

	for name, amount := range map[string]string{
		"legacy total price": `{"paccount":"Assets:401k","pamount":[{"acommodity":"401K","aquantity":{"floatingPoint":2},"aprice":{"tag":"TotalPrice","contents":{"acommodity":"USD","aquantity":{"floatingPoint":100}}}}]}`,
		"modern total cost":  `{"paccount":"Assets:401k","pamount":[{"acommodity":"401K","aquantity":{"floatingPoint":2},"acost":{"tag":"TotalCost","contents":{"acommodity":"USD","aquantity":{"floatingPoint":100}}}}]}`,
		"modern unit cost":   `{"paccount":"Assets:401k","pamount":[{"acommodity":"401K","aquantity":{"floatingPoint":2},"acost":{"tag":"UnitCost","contents":{"acommodity":"USD","aquantity":{"floatingPoint":50}}}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			var p HLedgerPosting
			require.NoError(t, json.Unmarshal([]byte(amount), &p))
			postings, err := buildHLedgerPostings(p, transaction, pricesTree, date)
			require.NoError(t, err)
			require.Len(t, postings, 1)
			assert.True(t, postings[0].Amount.Equal(decimal.NewFromInt(100)))
		})
	}
}
