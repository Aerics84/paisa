package generator

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"

	"time"

	"strings"

	"math/rand"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/scraper/mutualfund"
	"github.com/ananthakumaran/paisa/internal/scraper/nps"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/google/btree"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
)

const START_YEAR = 2014

type GeneratorState struct {
	Balance       float64
	CreditBalance float64
	EPFBalance    float64
	Ledger        *os.File
	YearlySalary  float64
	Rent          float64
	LoanBalance   float64
	NiftyBalance  float64
}

var pricesTree map[string]*btree.BTree

func MinimalConfig(cwd string) error {
	return MinimalConfigForProfile(cwd, config.RegionalProfileGermanyEU)
}

func MinimalConfigForProfile(cwd string, profile config.RegionalProfileType) error {
	configFilePath := filepath.Join(cwd, "paisa.yaml")
	configTemplate := `
journal_path: '%s'
db_path: '%s'
regional_profile: %s
default_currency: %s
`
	log.Info("Generating config file: ", configFilePath)
	journalFilePath := filepath.Join(cwd, "main.ledger")
	dbFilePath := filepath.Join(cwd, "paisa.db")
	profile = config.NormalizeRegionalProfile(profile)
	currency := "INR"
	if profile == config.RegionalProfileGermanyEU {
		currency = "EUR"
	}
	err := os.WriteFile(configFilePath, fmt.Appendf(nil, configTemplate, filepath.Base(journalFilePath), filepath.Base(dbFilePath), profile, currency), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configFilePath, err)
	}

	// First-run auto-creation runs before the configuration exists, so it must
	// never truncate a journal the user already has. O_TRUNC is deliberately
	// absent: an absent journal is created, an existing one is left untouched.
	if _, err := os.Stat(journalFilePath); os.IsNotExist(err) {
		log.Info("Creating journal file: ", journalFilePath)
		journalFile, err := os.OpenFile(journalFilePath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to create journal file %s: %w", journalFilePath, err)
		}
		if err := journalFile.Close(); err != nil {
			return fmt.Errorf("failed to close journal file %s: %w", journalFilePath, err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to inspect journal file %s: %w", journalFilePath, err)
	}

	return nil
}

func Demo(cwd string) error {
	return DemoForProfile(cwd, config.RegionalProfileIndia)
}

func DemoForProfile(cwd string, profile config.RegionalProfileType) error {
	switch config.NormalizeRegionalProfile(profile) {
	case config.RegionalProfileGermanyEU:
		if err := generateGermanyEUConfigFile(cwd); err != nil {
			return err
		}
		if err := generateGermanyEUJournalFile(cwd); err != nil {
			return err
		}
		return removeSheetFile(cwd)
	default:
		if err := generateIndiaConfigFile(cwd); err != nil {
			return err
		}
		if err := generateIndiaJournalFile(cwd); err != nil {
			return err
		}
		return generateIndiaSheetFile(cwd)
	}
}

func generateIndiaConfigFile(cwd string) error {
	configFilePath := filepath.Join(cwd, "paisa.yaml")
	config := `
journal_path: '%s'
db_path: '%s'
ledger_cli: ledger
regional_profile: india
tax_regime: india
default_currency: INR
goals:
  retirement:
    - name: Early Retirement
      icon: mdi:palm-tree
      swr: 3
      savings:
        - Assets:Debt:*
        - Assets:Equity:*
      expenses:
        - Expenses:Rent
        - Expenses:Utilities
        - Expenses:Shopping
        - Expenses:Restaurants
        - Expenses:Food
        - Expenses:Interest:*
  savings:
    - name: Millionaire
      icon: mdi:car-sports
      target: 80000000
      target_date: "2036-01-01"
      rate: 10
      accounts:
        - '!Assets:Checking:SBI'
allocation_targets:
  - name: Debt
    target: 40
    accounts:
      - Assets:Debt:*
  - name: Equity
    target: 60
    accounts:
      - Assets:Equity:*
accounts:
  - name: Liabilities:CreditCard:Freedom
    icon: arcticons:chase
schedule_al:
  - code: bank
    accounts:
      - Assets:Checking:SBI
  - code: share
    accounts:
      - Assets:Equity:*
      - Assets:Debt:*
  - code: liability
    accounts:
      - Liabilities:Homeloan
  - code: immovable
    accounts:
      - Assets:House
commodities:
  - name: NIFTY
    type: mutualfund
    price:
      provider: in-mfapi
      code: 120716
    harvest: 365
    tax_category: equity65
  - name: PPFAS
    type: mutualfund
    price:
      provider: in-mfapi
      code: 122639
    harvest: 365
    tax_category: equity65
  - name: ABCBF
    type: mutualfund
    price:
      provider: in-mfapi
      code: 119533
    harvest: 1095
    tax_category: debt
  - name: NPS_HDFC_E
    type: nps
    price:
      provider: com-finbodhi-nps
      code: SM008001
  - name: NPS_HDFC_C
    type: nps
    price:
      provider: com-finbodhi-nps
      code: SM008002
  - name: NPS_HDFC_G
    type: nps
    price:
      provider: com-finbodhi-nps
      code: SM008003
credit_cards:
    - account: Liabilities:CreditCard:Freedom
      credit_limit: 150000
      statement_end_day: 8
      due_day: 20
      network: visa
      number: "0007"
      expiration_date: "2029-05-01"
`
	log.Info("Generating config file: ", configFilePath)
	journalFilePath := filepath.Join(cwd, "main.ledger")
	dbFilePath := filepath.Join(cwd, "paisa.db")
	err := os.WriteFile(configFilePath, fmt.Appendf(nil, config, filepath.Base(journalFilePath), filepath.Base(dbFilePath)), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configFilePath, err)
	}

	return nil
}

func generateGermanyEUConfigFile(cwd string) error {
	configFilePath := filepath.Join(cwd, "paisa.yaml")
	configContent := `
journal_path: '%s'
db_path: '%s'
ledger_cli: ledger
regional_profile: germany-eu
tax_regime: germany
default_currency: EUR
locale: de-DE
time_zone: Europe/Berlin
financial_year_starting_month: 1
week_starting_day: 1
display_precision: 2
goals:
  retirement:
    - name: Early Retirement
      icon: mdi:beach
      swr: 4
      savings:
        - Assets:Savings:*
        - Assets:Equity:*
      expenses:
        - Expenses:Rent
        - Expenses:Utilities
        - Expenses:Food
        - Expenses:Transport
        - Expenses:Leisure
  savings:
    - name: Emergency Fund
      icon: mdi:shield-check
      target: 18000
      accounts:
        - Assets:Checking:*
        - Assets:Savings:*
allocation_targets:
  - name: Cash
    target: 25
    accounts:
      - Assets:Checking:*
      - Assets:Savings:*
  - name: Equity
    target: 75
    accounts:
      - Assets:Equity:*
accounts:
  - name: Assets:Checking:ING
    icon: mdi:bank
  - name: Assets:Savings:Tagesgeld
    icon: mdi:piggy-bank
  - name: Assets:Equity:VWCE
    icon: mdi:chart-line
  - name: Liabilities:CreditCard:Visa
    icon: mdi:credit-card-outline
credit_cards:
  - account: Liabilities:CreditCard:Visa
    credit_limit: 5000
    statement_end_day: 28
    due_day: 15
    network: visa
    number: "4242"
    expiration_date: "2029-12-01"
`
	log.Info("Generating config file: ", configFilePath)
	journalFilePath := filepath.Join(cwd, "main.ledger")
	dbFilePath := filepath.Join(cwd, "paisa.db")
	err := os.WriteFile(configFilePath, fmt.Appendf(nil, configContent, filepath.Base(journalFilePath), filepath.Base(dbFilePath)), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configFilePath, err)
	}

	return nil
}

func emitTransaction(file *os.File, date time.Time, payee string, from string, to string, amount any) error {
	amountString := ""
	switch amount.(type) {
	case string:
		amountString = amount.(string)
	case float64:
		amountString = formatFloat(amount.(float64))
	}

	_, err := file.WriteString(fmt.Sprintf(`
%s %s
    %s                                %s INR
    %s
`, date.Format("2006/01/02"), payee, to, amountString, from))
	if err != nil {
		return fmt.Errorf("failed to write transaction %q: %w", payee, err)
	}

	return nil
}

// commodityPrice returns the most recent known price for a commodity on or
// before date. The second result is false when no price history is available,
// which happens when the demo price download failed or was skipped offline.
func commodityPrice(commodity string, date time.Time) (price.Price, bool) {
	tree := pricesTree[commodity]
	if tree == nil || tree.Len() == 0 {
		return price.Price{}, false
	}

	pc := utils.BTreeDescendFirstLessOrEqual(tree, price.Price{Date: date})
	if pc.Value.IsZero() {
		return price.Price{}, false
	}

	return pc, true
}

func emitCommodityBuy(file *os.File, date time.Time, commodity string, from string, to string, amount float64) (float64, error) {
	pc, ok := commodityPrice(commodity, date)
	if !ok {
		log.Warnf("Skipping %s transaction in demo journal: no price history available", commodity)
		return 0, nil
	}

	units := amount / pc.Value.InexactFloat64()
	_, err := file.WriteString(fmt.Sprintf(`
%s Investment
    %s                      %s %s @    %s INR
    %s
`, date.Format("2006/01/02"), to, formatFloat(units), commodity, formatFloat(pc.Value.InexactFloat64()), from))
	if err != nil {
		return 0, fmt.Errorf("failed to write %s investment transaction: %w", commodity, err)
	}

	return units, nil
}

func emitCommoditySell(file *os.File, date time.Time, commodity string, from string, to string, amount float64, availableUnits float64) (float64, float64, error) {
	pc, ok := commodityPrice(commodity, date)
	if !ok {
		log.Warnf("Skipping %s sale in demo journal: no price history available", commodity)
		return 0, 0, nil
	}

	requiredUnits := amount / pc.Value.InexactFloat64()
	units := math.Min(availableUnits, requiredUnits)
	bought, err := emitCommodityBuy(file, date, commodity, from, to, -units*pc.Value.InexactFloat64())
	if err != nil {
		return 0, 0, err
	}

	return bought, units * pc.Value.InexactFloat64(), nil
}

// demoHTTPClient bounds the price-history download so an offline demo
// generation cannot hang the server.
var demoHTTPClient = &http.Client{Timeout: 10 * time.Second}

func loadPrices(schemeCode string, commodityType config.CommodityType, commodityName string, pricesTree map[string]*btree.BTree) {
	var prices []*price.Price
	var err error

	switch commodityType {
	case config.MutualFund:
		prices, err = mutualfund.GetNavWithClient(demoHTTPClient, schemeCode, commodityName)
	case config.NPS:
		prices, err = nps.GetNavWithClient(demoHTTPClient, schemeCode, commodityName)
	}

	if err != nil {
		// Demo generation must still work offline. Commodity transactions that
		// need a price are skipped, the rest of the demo journal is generated.
		log.Warnf("Skipping %s price history for demo journal: %v", commodityName, err)
		return
	}

	pricesTree[commodityName] = btree.New(2)
	for _, price := range prices {
		pricesTree[commodityName].ReplaceOrInsert(*price)
	}
}

func formatFloat(num float64) string {
	s := fmt.Sprintf("%.4f", num)
	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}

func roundToK(amount float64) float64 {
	if amount < 20000 {
		return float64(int(amount/100) * 100)
	}
	return float64(int(amount/1000) * 1000)
}

func incrementByPercentRange(amount float64, min int, max int) float64 {
	return roundToK(amount + amount*percentRange(min, max))
}

func percentRange(min int, max int) float64 {
	if min == max {
		return float64(min) * 0.01
	}
	return float64(randRange(min, max)) * 0.01
}

func randRange(min int, max int) int {
	return rand.Intn(max-min) + min
}

func taxRate(amount float64) float64 {
	if amount < 500000 {
		return 0
	} else if amount < 750000 {
		return 0.10
	} else if amount < 1000000 {
		return 0.15
	} else if amount < 1250000 {
		return 0.20
	} else if amount < 1500000 {
		return 0.25
	}
	return 0.30
}

func emitChitFund(state *GeneratorState) error {
	start, _ := time.Parse("02-01-2006", "01-01-2016")
	end, _ := time.Parse("02-01-2006", "01-11-2016")

	for ; start.Before(end); start = start.AddDate(0, 1, 0) {
		price := 10000 - ((time.November - start.Month()) * 100)
		amount := fmt.Sprintf("1 CHIT @ %d", price)

		if start.Month() >= time.June {
			if err := emitTransaction(state.Ledger, start, "Chit installment", "Assets:Checking:SBI", "Liabilities:Chit", amount); err != nil {
				return err
			}
		} else {
			if err := emitTransaction(state.Ledger, start, "Chit installment", "Assets:Checking:SBI", "Assets:Debt:Chit", amount); err != nil {
				return err
			}
		}

		if start.Month() == time.June {
			amount = fmt.Sprintf("-5 CHIT @ %d", price)
			if err := emitTransaction(state.Ledger, start, "Chit withdraw", "Assets:Checking:SBI", "Assets:Debt:Chit", amount); err != nil {
				return err
			}
			amount = fmt.Sprintf("-5 CHIT @ %d", price)
			if err := emitTransaction(state.Ledger, start, "Chit withdraw", "Assets:Checking:SBI", "Liabilities:Chit", amount); err != nil {
				return err
			}
		}

	}

	return nil
}

func emitSalary(state *GeneratorState, start time.Time) error {
	if start.Month() == time.April {
		state.YearlySalary = incrementByPercentRange(state.YearlySalary, 10, 15)
	}

	var salary float64 = state.YearlySalary / 12
	var company string
	if start.Year() > 2017 {
		company = "Globex"
	} else {
		company = "Acme"
	}

	tax := salary * taxRate(state.YearlySalary)
	epf := salary * 0.12
	nps := salary * 0.10
	state.EPFBalance += epf
	netSalary := salary - tax - epf - nps
	state.Balance += netSalary

	salaryAccount := fmt.Sprintf("Income:Salary:%s", company)
	if err := emitTransaction(state.Ledger, start, "Salary", salaryAccount, "Assets:Checking:SBI", netSalary); err != nil {
		return err
	}
	if err := emitTransaction(state.Ledger, start, "Salary EPF", salaryAccount, "Assets:Debt:EPF", epf); err != nil {
		return err
	}
	if err := emitTransaction(state.Ledger, start, "Salary Tax", salaryAccount, "Expenses:Tax", tax); err != nil {
		return err
	}
	if _, err := emitCommodityBuy(state.Ledger, start, "NPS_HDFC_E", salaryAccount, "Assets:Debt:NPS:HDFC:E", nps*0.75); err != nil {
		return err
	}
	if _, err := emitCommodityBuy(state.Ledger, start, "NPS_HDFC_C", salaryAccount, "Assets:Equity:NPS:HDFC:C", nps*0.15); err != nil {
		return err
	}
	if _, err := emitCommodityBuy(state.Ledger, start, "NPS_HDFC_G", salaryAccount, "Assets:Equity:NPS:HDFC:G", nps*0.10); err != nil {
		return err
	}

	return nil
}

func emitExpense(state *GeneratorState, start time.Time) error {
	if start.Month() == time.April {
		state.Rent = incrementByPercentRange(state.Rent, 5, 10)
	}

	emit := func(payee string, account string, amount float64, fuzz float64) error {
		var actualAmount float64
		if fuzz == 1 {
			actualAmount = amount
		} else {
			actualAmount = roundToK(percentRange(int(fuzz*100), 100) * amount)
		}
		start = start.AddDate(0, 0, 1)
		if err := emitTransaction(state.Ledger, start, payee, "Assets:Checking:SBI", account, actualAmount); err != nil {
			return err
		}
		state.Balance -= actualAmount
		return nil
	}

	emitExpense := func(payee string, account string, amount float64, fuzz float64) error {
		actualAmount := roundToK(percentRange(int(fuzz*100), 100) * amount)
		start = start.AddDate(0, 0, 1)
		if err := emitTransaction(state.Ledger, start, payee, "Liabilities:CreditCard:Freedom", account, actualAmount); err != nil {
			return err
		}
		state.CreditBalance -= actualAmount
		return nil
	}

	expenses := []struct {
		payee   string
		account string
		amount  float64
		fuzz    float64
	}{
		{"Rent", "Expenses:Rent", state.Rent, 1.0},
		{"Internet", "Expenses:Utilities", 1500, 1.0},
		{"Mobile", "Expenses:Utilities", 430, 1.0},
		{"Shopping", "Expenses:Shopping", 3000, 0.5},
		{"Eat out", "Expenses:Restaurants", 2500, 0.5},
		{"Groceries", "Expenses:Food", 5000, 0.9},
	}
	for _, expense := range expenses {
		if err := emitExpense(expense.payee, expense.account, expense.amount, expense.fuzz); err != nil {
			return err
		}
	}

	if state.LoanBalance > 0 {
		emi := math.Min(state.Balance-10000, 30000.0)
		interest := (state.LoanBalance * 0.08 / 12)
		principal := emi - interest
		state.LoanBalance -= principal
		if err := emit("EMI", "Expenses:Interest:Homeloan", interest, 1.0); err != nil {
			return err
		}
		if err := emit("EMI", "Liabilities:Homeloan", principal, 1.0); err != nil {
			return err
		}
	}

	if state.Balance < 10000 {
		if err := emit("Pay Credit Card Bill", "Liabilities:CreditCard:Freedom", -state.CreditBalance, 1.0); err != nil {
			return err
		}
		state.CreditBalance = 0
		return nil
	}

	if lo.Contains([]time.Month{time.January, time.April, time.November, time.December}, start.Month()) {
		if err := emit("Dress", "Expenses:Clothing", 5000, 0.5); err != nil {
			return err
		}
	}

	if err := emit("Pay Credit Card Bill", "Liabilities:CreditCard:Freedom", -state.CreditBalance, 1.0); err != nil {
		return err
	}
	state.CreditBalance = 0

	return nil
}
func emitInvestment(state *GeneratorState, start time.Time) error {
	if start.Month() == time.April {
		epfInterest := state.EPFBalance * 0.08
		if err := emitTransaction(state.Ledger, start, "EPF Interest", "Income:Interest:EPF", "Assets:Debt:EPF", epfInterest); err != nil {
			return err
		}
		state.EPFBalance += epfInterest
	}

	if state.Balance < 10000 {
		return nil
	}

	equity1 := roundToK(state.Balance * 0.5)
	equity2 := roundToK(state.Balance * 0.2)
	debt := roundToK(state.Balance * 0.3)

	units, err := emitCommodityBuy(state.Ledger, start, "NIFTY", "Assets:Checking:SBI", "Assets:Equity:NIFTY", equity1)
	if err != nil {
		return err
	}
	if units > 0 {
		// A skipped commodity transaction (no price history) must not reduce
		// the notional cash balance; otherwise the demo journal drifts.
		state.Balance -= equity1
	}
	state.NiftyBalance += units

	units, err = emitCommodityBuy(state.Ledger, start, "PPFAS", "Assets:Checking:SBI", "Assets:Equity:PPFAS", equity2)
	if err != nil {
		return err
	}
	if units > 0 {
		state.Balance -= equity2
	}

	units, err = emitCommodityBuy(state.Ledger, start, "ABCBF", "Assets:Checking:SBI", "Assets:Debt:ABCBF", debt)
	if err != nil {
		return err
	}
	if units > 0 {
		state.Balance -= debt
	}

	if start.Month() == time.March {
		soldUnits, amount, err := emitCommoditySell(state.Ledger, start.AddDate(0, 0, 15), "NIFTY", "Assets:Checking:SBI", "Assets:Equity:NIFTY", 75000, state.NiftyBalance)
		if err != nil {
			return err
		}
		state.NiftyBalance += soldUnits
		state.Balance += amount
	}

	return nil
}

func generateIndiaJournalFile(cwd string) error {
	journalFilePath := filepath.Join(cwd, "main.ledger")
	log.Info("Generating journal file: ", journalFilePath)
	ledgerFile, err := os.OpenFile(journalFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create journal file %s: %w", journalFilePath, err)
	}

	writeJournal := func(content string) error {
		if _, err := ledgerFile.WriteString(content); err != nil {
			return fmt.Errorf("failed to write journal file %s: %w", journalFilePath, err)
		}
		return nil
	}

	startMonth := utils.BeginningOfMonth(utils.EndOfToday())
	endMonth := startMonth.AddDate(0, 2, 0)

	if err := writeJournal(`
= Expenses:Rent
    ; Recurring: Rent
    ; Period: 1 * ?

= expr payee=~/Internet/
    ; Recurring: Internet
    ; Period: 1 * ?

= expr payee=~/EPF/
    ; Recurring: EPF

= Liabilities:Homeloan
    ; Recurring: EMI Principle

= Expenses:Interest:Homeloan
    ; Recurring: EMI Interest

~ Monthly from `); err != nil {
		_ = ledgerFile.Close()
		return err
	}

	if err := writeJournal(fmt.Sprintf("%s to %s", startMonth.Format("2006-01-02"), endMonth.Format("2006-01-02"))); err != nil {
		_ = ledgerFile.Close()
		return err
	}

	if err := writeJournal(`
    Expenses:Rent                              15000 INR
    Expenses:Interest:Homeloan                  6000 INR
    Expenses:Food                               5000 INR
    Expenses:Utilities                          2000 INR
    Expenses:Shopping                           3000 INR
    Expenses:Clothing                           1000 INR
    Assets:Checking:SBI

`); err != nil {
		_ = ledgerFile.Close()
		return err
	}

	end := utils.EndOfToday()
	start, err := time.Parse("02-01-2006", fmt.Sprintf("01-01-%d", START_YEAR))
	if err != nil {
		_ = ledgerFile.Close()
		return err
	}

	pricesTree = make(map[string]*btree.BTree)
	loadPrices("120716", config.MutualFund, "NIFTY", pricesTree)
	loadPrices("122639", config.MutualFund, "PPFAS", pricesTree)
	loadPrices("119533", config.MutualFund, "ABCBF", pricesTree)
	loadPrices("SM008001", config.NPS, "NPS_HDFC_E", pricesTree)
	loadPrices("SM008002", config.NPS, "NPS_HDFC_C", pricesTree)
	loadPrices("SM008003", config.NPS, "NPS_HDFC_G", pricesTree)

	state := GeneratorState{Balance: 0, Ledger: ledgerFile, YearlySalary: 1000000, Rent: 10000, LoanBalance: 2500000}

	generate := func() error {
		if err := emitTransaction(state.Ledger, start, "Home purchase", "Liabilities:Homeloan", "Assets:House", "1 APT @ 2500000"); err != nil {
			return err
		}

		for ; start.Before(end); start = start.AddDate(0, 1, 0) {
			if err := emitSalary(&state, start); err != nil {
				return err
			}
			if err := emitExpense(&state, start); err != nil {
				return err
			}
			if err := emitInvestment(&state, start); err != nil {
				return err
			}
		}

		return emitChitFund(&state)
	}

	if err := generate(); err != nil {
		_ = ledgerFile.Close()
		return err
	}

	if err := ledgerFile.Close(); err != nil {
		return fmt.Errorf("failed to close journal file %s: %w", journalFilePath, err)
	}

	return nil
}

func formatEuroAmount(amount float64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}

	s := fmt.Sprintf("%.2f", amount)
	parts := strings.Split(s, ".")
	whole := parts[0]
	fraction := parts[1]

	chunks := make([]string, 0, len(whole)/3+1)
	for len(whole) > 3 {
		chunks = append([]string{whole[len(whole)-3:]}, chunks...)
		whole = whole[:len(whole)-3]
	}
	chunks = append([]string{whole}, chunks...)

	return sign + strings.Join(chunks, ".") + "," + fraction
}

func emitGermanyEUTransaction(writer io.StringWriter, date time.Time, payee string, lines ...string) error {
	_, err := writer.WriteString(fmt.Sprintf("%s %s\n", date.Format("2006/01/02"), payee))
	if err != nil {
		return fmt.Errorf("failed to write transaction %q: %w", payee, err)
	}

	for _, line := range lines {
		_, err = writer.WriteString("    " + line + "\n")
		if err != nil {
			return fmt.Errorf("failed to write transaction %q: %w", payee, err)
		}
	}

	_, err = writer.WriteString("\n")
	if err != nil {
		return fmt.Errorf("failed to write transaction %q: %w", payee, err)
	}

	return nil
}

func generateGermanyEUJournalFile(cwd string) error {
	journalFilePath := filepath.Join(cwd, "main.ledger")
	log.Info("Generating journal file: ", journalFilePath)

	startMonth := utils.BeginningOfMonth(utils.EndOfToday()).AddDate(0, -2, 0)
	endMonth := utils.BeginningOfMonth(utils.EndOfToday())

	var journal strings.Builder
	journal.WriteString("commodity EUR\n")
	journal.WriteString("\tformat 1.000,00 EUR\n\n")

	priceDates := []time.Time{
		startMonth.AddDate(0, 0, 10),
		startMonth.AddDate(0, 1, 10),
		endMonth.AddDate(0, 0, 10),
		utils.EndOfToday(),
	}
	priceValues := []string{"118,00 EUR", "119,50 EUR", "121,00 EUR", "123,40 EUR"}
	for index, date := range priceDates {
		journal.WriteString(fmt.Sprintf("P %s 00:00:00 VWCE %s\n", date.Format("2006/01/02"), priceValues[index]))
	}
	journal.WriteString("\n")

	var emitErr error
	emit := func(date time.Time, payee string, lines ...string) {
		if emitErr != nil {
			return
		}
		emitErr = emitGermanyEUTransaction(&journal, date, payee, lines...)
	}

	for offset := range 3 {
		month := startMonth.AddDate(0, offset, 0)

		salary := 4800.0
		tagesgeld := 600.0
		rent := 1450.0
		utilities := 210.0
		groceries := 260.0
		transport := 95.0
		leisure := 140.0
		cardBill := groceries + transport + leisure
		etfUnits := 4 + offset
		etfPrice := 118.0 + float64(offset)*1.5
		etfAmount := float64(etfUnits) * etfPrice

		emit(month, "Salary",
			fmt.Sprintf("Income:Salary:Acme GmbH                -%s EUR", formatEuroAmount(salary)),
			fmt.Sprintf("Assets:Checking:ING                      %s EUR", formatEuroAmount(salary)),
		)
		emit(month.AddDate(0, 0, 1), "Transfer to Tagesgeld",
			fmt.Sprintf("Assets:Savings:Tagesgeld                  %s EUR", formatEuroAmount(tagesgeld)),
			fmt.Sprintf("Assets:Checking:ING                      -%s EUR", formatEuroAmount(tagesgeld)),
		)
		emit(month.AddDate(0, 0, 3), "Rent",
			fmt.Sprintf("Expenses:Rent                             %s EUR", formatEuroAmount(rent)),
			fmt.Sprintf("Assets:Checking:ING                      -%s EUR", formatEuroAmount(rent)),
		)
		emit(month.AddDate(0, 0, 5), "Utilities",
			fmt.Sprintf("Expenses:Utilities                        %s EUR", formatEuroAmount(utilities)),
			fmt.Sprintf("Assets:Checking:ING                      -%s EUR", formatEuroAmount(utilities)),
		)
		emit(month.AddDate(0, 0, 10), "ETF Purchase",
			fmt.Sprintf("Assets:Equity:VWCE                        %d VWCE @ %s EUR", etfUnits, formatEuroAmount(etfPrice)),
			fmt.Sprintf("Assets:Checking:ING                      -%s EUR", formatEuroAmount(etfAmount)),
		)
		emit(month.AddDate(0, 0, 12), "Groceries",
			fmt.Sprintf("Expenses:Food                             %s EUR", formatEuroAmount(groceries)),
			fmt.Sprintf("Liabilities:CreditCard:Visa              -%s EUR", formatEuroAmount(groceries)),
		)
		emit(month.AddDate(0, 0, 16), "Deutschlandticket",
			fmt.Sprintf("Expenses:Transport                        %s EUR", formatEuroAmount(transport)),
			fmt.Sprintf("Liabilities:CreditCard:Visa              -%s EUR", formatEuroAmount(transport)),
		)
		emit(month.AddDate(0, 0, 20), "Weekend Trip",
			fmt.Sprintf("Expenses:Leisure                          %s EUR", formatEuroAmount(leisure)),
			fmt.Sprintf("Liabilities:CreditCard:Visa              -%s EUR", formatEuroAmount(leisure)),
		)
		emit(month.AddDate(0, 0, 27), "Pay Credit Card Bill",
			fmt.Sprintf("Liabilities:CreditCard:Visa               %s EUR", formatEuroAmount(cardBill)),
			fmt.Sprintf("Assets:Checking:ING                      -%s EUR", formatEuroAmount(cardBill)),
		)
	}

	if emitErr != nil {
		return emitErr
	}

	err := os.WriteFile(journalFilePath, []byte(journal.String()), 0644)
	if err != nil {
		return fmt.Errorf("failed to write journal file %s: %w", journalFilePath, err)
	}

	return nil
}

func generateIndiaSheetFile(cwd string) error {
	sheetFilePath := filepath.Join(cwd, "Schedule AL.paisa")
	sheet := `
date_query = {date <= [2023-03-31]}
cost_basis(x) = cost(fifo(x AND date_query))
cost_basis_negative(x) = cost(fifo(negate(x AND date_query)))

# Immovable
immovable = cost_basis({account = Assets:House})

# Movable
metal = 0
art = 0
vehicle = 0
bank = cost_basis({account =~ /^Assets:Checking:SBI/})
share = cost_basis({account =~ /^Assets:Equity:.*/ OR
                    account =~ /^Assets:Debt:.*/})
insurance = 0
loan = 0
cash = 0

# Liability
liability = cost_basis_negative({account =~ /^Liabilities:Homeloan/})

# Total
total = immovable + metal + art + vehicle + bank + share + insurance + loan + cash - liability
`
	log.Info("Generating sheet file: ", sheetFilePath)
	err := os.WriteFile(sheetFilePath, []byte(sheet), 0644)
	if err != nil {
		return fmt.Errorf("failed to write sheet file %s: %w", sheetFilePath, err)
	}

	return nil
}

func removeSheetFile(cwd string) error {
	sheetFilePath := filepath.Join(cwd, "Schedule AL.paisa")
	if err := os.Remove(sheetFilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove sheet file %s: %w", sheetFilePath, err)
	}

	return nil
}
