package ledger

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadBeancountTestConfig(t *testing.T, journalPath, defaultCurrency string) {
	t.Helper()
	require.NoError(t, config.LoadConfig([]byte(`
journal_path: `+journalPath+`
db_path: `+filepath.Join(filepath.Dir(journalPath), "paisa.db")+`
ledger_cli: beancount
default_currency: `+defaultCurrency+`
time_zone: Europe/Berlin
`), ""))
}

func requireBeancount3(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bean-query"); err != nil {
		t.Skip("bean-query executable is not installed")
	}
	if _, err := exec.LookPath("bean-check"); err != nil {
		t.Skip("bean-check executable is not installed")
	}
}

func TestParseBeancountPricesCSV(t *testing.T) {
	output := `date,currency,amount.number,amount.currency
2022-01-07,NIFTY,100,INR
2022-01-08,USD,80.442048,INR
2022-01-09,EUR,0.5,USD
2022-01-10,INR,0.02,USD
2022-01-11,GBP,0,INR
`
	prices, err := parseBeancountPrices(output, "INR")
	require.NoError(t, err)
	require.Len(t, prices, 4)

	assertPriceEqual(t, prices[0], "2022/01/07", "NIFTY", 100)
	assertPriceEqual(t, prices[1], "2022/01/08", "USD", 80.442048)
	assertPriceEqual(t, prices[2], "2022/01/10", "USD", 50)
	assertPriceEqual(t, prices[3], "2022/01/11", "GBP", 0)
}

func TestBeancountParsePreservesPrecision(t *testing.T) {
	requireBeancount3(t)

	dir := t.TempDir()
	journal := filepath.Join(dir, "main.beancount")
	require.NoError(t, os.WriteFile(journal, []byte(`
1970-01-01 open Assets:Checking
1970-01-01 open Assets:Equity:ABNB
1970-01-01 commodity ABNB
1970-01-01 commodity USD

2022-01-08 txn "Buy ABNB"
  Assets:Equity:ABNB   0.76 ABNB {131.27 USD}
  Assets:Checking
`), 0600))
	loadBeancountTestConfig(t, journal, "USD")

	postings, err := (Beancount{}).Parse(journal, nil)
	require.NoError(t, err)

	byAccount := map[string]*posting.Posting{}
	for _, p := range postings {
		byAccount[p.Account] = p
	}

	equity, ok := byAccount["Assets:Equity:ABNB"]
	require.True(t, ok)
	assert.True(t, equity.Quantity.Equal(decimal.RequireFromString("0.76")), "quantity: %s", equity.Quantity)
	assert.True(t, equity.Amount.Equal(decimal.RequireFromString("99.7652")), "cost amount: %s", equity.Amount)

	checking, ok := byAccount["Assets:Checking"]
	require.True(t, ok)
	assert.True(t, checking.Amount.Equal(decimal.RequireFromString("-99.7652")), "cost amount: %s", checking.Amount)
}

func TestBeancountPrices(t *testing.T) {
	requireBeancount3(t)

	dir := t.TempDir()
	journal := filepath.Join(dir, "main.beancount")
	require.NoError(t, os.WriteFile(journal, []byte(`
1970-01-01 commodity NIFTY
1970-01-01 commodity USD
1970-01-01 commodity EUR

2022-01-07 price NIFTY 100 INR
2022-01-08 price USD 80.442048 INR
2022-01-09 price EUR 0.5 USD
2022-01-10 price INR 0.02 USD
`), 0600))
	loadBeancountTestConfig(t, journal, "INR")

	prices, err := (Beancount{}).Prices(journal)
	require.NoError(t, err)
	require.Len(t, prices, 3)

	assertPriceEqual(t, prices[0], "2022/01/07", "NIFTY", 100)
	assertPriceEqual(t, prices[1], "2022/01/08", "USD", 80.442048)
	assertPriceEqual(t, prices[2], "2022/01/10", "USD", 50)
}

func TestBeancountValidateFileOutput(t *testing.T) {
	requireBeancount3(t)

	dir := t.TempDir()
	journal := filepath.Join(dir, "main.beancount")
	require.NoError(t, os.WriteFile(journal, []byte(`
1970-01-01 open Assets:Checking
1970-01-01 open Equity:OpeningBalances

2022-01-01 txn "Opening"
  Assets:Checking            1000 INR
  Equity:OpeningBalances
`), 0600))
	loadBeancountTestConfig(t, journal, "INR")

	errors, output, err := (Beancount{}).ValidateFile(journal)
	require.NoError(t, err)
	assert.Empty(t, errors)
	assert.Contains(t, output, "Assets:Checking")
	assert.Contains(t, output, "1000")
}
