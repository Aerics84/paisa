// Package reconciliation builds a read-only reconciliation report from
// imported finance data.
//
// The package deliberately has no data store of its own. Callers pass a
// ReconciliationInput, typically assembled from existing import manifests.
// BuildReconciliationFromManifests is the convenience adapter used by the
// HTTP API: it reads the latest staged statement manifests below
// <root>/imports and maps their rows into bank statements. The current
// statement manifests do not carry holdings, manual balances or tax records,
// so those lists stay empty unless a caller supplies them explicitly.
package reconciliation

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/importer/manifest"
)

// BankTransaction is one row of an imported bank statement. Amounts are kept
// as strings so malformed source data can be reported instead of silently
// becoming zero.
type BankTransaction struct {
	Date     string `json:"date,omitempty"`
	Debit    string `json:"debit,omitempty"`
	Credit   string `json:"credit,omitempty"`
	Balance  string `json:"balance,omitempty"`
	Currency string `json:"currency,omitempty"`
	Source   string `json:"source,omitempty"`
}

// BankStatement groups the transactions belonging to one imported source file.
type BankStatement struct {
	Source       string            `json:"source"`
	Transactions []BankTransaction `json:"transactions"`
}

// Security identifies a holding.
type Security struct {
	Identifier string `json:"identifier,omitempty"`
	ISIN       string `json:"isin,omitempty"`
	Symbol     string `json:"symbol,omitempty"`
	Name       string `json:"name,omitempty"`
}

// HoldingSource records where a holding was imported from.
type HoldingSource struct {
	OriginalPath  string `json:"original_path,omitempty"`
	RecordLocator string `json:"record_locator,omitempty"`
}

// Holding is one imported portfolio position.
type Holding struct {
	CurrentValue string        `json:"current_value,omitempty"`
	Currency     string        `json:"currency,omitempty"`
	OverlapKey   string        `json:"overlap_key,omitempty"`
	Security     Security      `json:"security"`
	Source       HoldingSource `json:"source"`
}

// ManualBalance is an imported asset or liability balance. A balance_type of
// "cash_flow" is ignored because it is not a stock value.
type ManualBalance struct {
	Amount      string `json:"amount,omitempty"`
	Currency    string `json:"currency,omitempty"`
	BalanceType string `json:"balance_type,omitempty"`
}

// TaxRecord is one filing/source present for a financial year. Supported
// source values mirror the source material: itr, ais, form16, form26as or
// annual_tax_statement.
type TaxRecord struct {
	Source        string `json:"source"`
	FinancialYear string `json:"financial_year"`
}

// ReconciliationInput is the complete input to BuildReconciliation.
type ReconciliationInput struct {
	BankStatements []BankStatement `json:"bank_statements"`
	Holdings       []Holding       `json:"holdings"`
	Balances       []ManualBalance `json:"balances"`
	TaxRecords     []TaxRecord     `json:"tax_records"`
	Issues         []string        `json:"issues,omitempty"`
}

// BankBalanceCheck summarizes balance continuity for one imported statement.
type BankBalanceCheck struct {
	Source           string  `json:"source"`
	Currency         string  `json:"currency"`
	TransactionCount int     `json:"transaction_count"`
	Direction        string  `json:"direction"`
	OpeningBalance   *string `json:"opening_balance"`
	ClosingBalance   *string `json:"closing_balance"`
	MismatchCount    int     `json:"mismatch_count"`
	Status           string  `json:"status"`
}

// DuplicateHolding reports one security identifier found more than once.
type DuplicateHolding struct {
	Key         string   `json:"key"`
	Occurrences int      `json:"occurrences"`
	Sources     []string `json:"sources"`
}

// CurrencyTotal aggregates holdings and manual balances for one currency.
type CurrencyTotal struct {
	Currency       string `json:"currency"`
	Holdings       string `json:"holdings"`
	Assets         string `json:"assets"`
	Liabilities    string `json:"liabilities"`
	Net            string `json:"net"`
	InvalidRecords int    `json:"invalid_records"`
}

// TaxYearCheck reports whether a financial year has the expected source
// families: ITR, AIS and a tax statement (Form 26AS or annual tax statement).
type TaxYearCheck struct {
	FinancialYear   string   `json:"financial_year"`
	SourcesPresent  []string `json:"sources_present"`
	MissingRequired []string `json:"missing_required"`
	Status          string   `json:"status"`
}

// ReconciliationReport is the serialized result of BuildReconciliation.
type ReconciliationReport struct {
	GeneratedAt         string             `json:"generated_at"`
	Status              string             `json:"status"`
	BankBalances        []BankBalanceCheck `json:"bank_balances"`
	DuplicateHoldings   []DuplicateHolding `json:"duplicate_holdings"`
	PortfolioTotals     []CurrencyTotal    `json:"portfolio_totals"`
	TaxYearCompleteness []TaxYearCheck     `json:"tax_year_completeness"`
	Issues              []string           `json:"issues"`
}

var numericAmountPattern = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
var holdingKeyPattern = regexp.MustCompile(`[^A-Z0-9]`)

// BuildReconciliation derives all checks from input. It does not read from or
// write to any external store.
func BuildReconciliation(input ReconciliationInput) ReconciliationReport {
	bankBalances := make([]BankBalanceCheck, 0, len(input.BankStatements))
	for _, statement := range input.BankStatements {
		bankBalances = append(bankBalances, reconcileBank(statement))
	}

	duplicateHoldings := findDuplicateHoldings(input.Holdings)
	portfolioTotals := buildPortfolioTotals(input.Holdings, input.Balances)
	taxYearCompleteness := buildTaxYearCompleteness(input.TaxRecords)

	issues := append([]string{}, input.Issues...)
	if len(duplicateHoldings) > 0 {
		issues = append(issues, fmt.Sprintf("%d holding identifier(s) appear more than once.", len(duplicateHoldings)))
	}
	hasInvalidPortfolioRecords := false
	for _, total := range portfolioTotals {
		if total.InvalidRecords > 0 {
			hasInvalidPortfolioRecords = true
			break
		}
	}
	if hasInvalidPortfolioRecords {
		issues = append(issues, "Some portfolio records contain invalid amounts.")
	}
	for _, year := range taxYearCompleteness {
		if year.Status == "incomplete" {
			issues = append(issues, "One or more tax years are missing a required source family.")
			break
		}
	}

	hasBankWarning := false
	hasBankError := false
	for _, bank := range bankBalances {
		switch bank.Status {
		case "error":
			hasBankError = true
		case "warning":
			hasBankWarning = true
		}
	}

	status := "ok"
	if hasBankError || hasInvalidPortfolioRecords {
		status = "error"
	} else if len(issues) > 0 || hasBankWarning {
		status = "warning"
	}

	return ReconciliationReport{
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
		Status:              status,
		BankBalances:        bankBalances,
		DuplicateHoldings:   duplicateHoldings,
		PortfolioTotals:     portfolioTotals,
		TaxYearCompleteness: taxYearCompleteness,
		Issues:              issues,
	}
}

func reconcileBank(statement BankStatement) BankBalanceCheck {
	rows := make([]BankTransaction, 0, len(statement.Transactions))
	for _, transaction := range statement.Transactions {
		if _, ok := parseAmount(transaction.Balance); ok {
			rows = append(rows, transaction)
		}
	}

	currency := "INR"
	for _, row := range rows {
		if strings.TrimSpace(row.Currency) != "" {
			currency = row.Currency
			break
		}
	}

	if len(rows) == 0 {
		return BankBalanceCheck{
			Source:           statement.Source,
			Currency:         currency,
			TransactionCount: len(statement.Transactions),
			Direction:        "unknown",
			OpeningBalance:   nil,
			ClosingBalance:   nil,
			MismatchCount:    0,
			Status:           "warning",
		}
	}

	ascendingMismatches := 0
	descendingMismatches := 0
	for index := 1; index < len(rows); index++ {
		previous, _ := parseAmount(rows[index-1].Balance)
		current, _ := parseAmount(rows[index].Balance)
		if amountsDiffer(current, previous+transactionDelta(rows[index])) {
			ascendingMismatches++
		}
		if amountsDiffer(previous, current+transactionDelta(rows[index-1])) {
			descendingMismatches++
		}
	}

	ascending := ascendingMismatches <= descendingMismatches
	mismatches := ascendingMismatches
	if !ascending {
		mismatches = descendingMismatches
	}

	first, _ := parseAmount(rows[0].Balance)
	last, _ := parseAmount(rows[len(rows)-1].Balance)
	var opening float64
	var closing float64
	if ascending {
		opening = first - transactionDelta(rows[0])
		closing = last
	} else {
		opening = last - transactionDelta(rows[len(rows)-1])
		closing = first
	}

	direction := "ascending"
	if len(rows) < 2 {
		direction = "unknown"
	} else if !ascending {
		direction = "descending"
	}

	status := "ok"
	if len(rows) != len(statement.Transactions) {
		status = "warning"
	} else if mismatches > 0 {
		status = "error"
	}

	return BankBalanceCheck{
		Source:           statement.Source,
		Currency:         currency,
		TransactionCount: len(statement.Transactions),
		Direction:        direction,
		OpeningBalance:   formatAmount(opening),
		ClosingBalance:   formatAmount(closing),
		MismatchCount:    mismatches,
		Status:           status,
	}
}

func findDuplicateHoldings(holdings []Holding) []DuplicateHolding {
	grouped := make(map[string][]Holding)
	for _, holding := range holdings {
		key := holdingKey(holding)
		if key == "" {
			continue
		}
		grouped[key] = append(grouped[key], holding)
	}

	duplicates := make([]DuplicateHolding, 0)
	for key, rows := range grouped {
		if len(rows) < 2 {
			continue
		}
		sourceSet := make(map[string]struct{})
		for _, row := range rows {
			source := strings.TrimSpace(row.Source.OriginalPath)
			if source == "" {
				source = "unknown"
			}
			sourceSet[source] = struct{}{}
		}
		sources := make([]string, 0, len(sourceSet))
		for source := range sourceSet {
			sources = append(sources, source)
		}
		sort.Strings(sources)
		duplicates = append(duplicates, DuplicateHolding{
			Key:         key,
			Occurrences: len(rows),
			Sources:     sources,
		})
	}

	sort.Slice(duplicates, func(i, j int) bool {
		return duplicates[i].Key < duplicates[j].Key
	})
	return duplicates
}

func holdingKey(holding Holding) string {
	raw := strings.TrimSpace(holding.OverlapKey)
	if raw == "" {
		raw = strings.TrimSpace(holding.Security.Identifier)
	}
	if raw == "" {
		raw = strings.TrimSpace(holding.Security.ISIN)
	}
	if raw == "" {
		raw = strings.TrimSpace(holding.Security.Symbol)
	}
	if raw == "" {
		raw = strings.TrimSpace(holding.Security.Name)
	}
	return holdingKeyPattern.ReplaceAllString(strings.ToUpper(raw), "")
}

type currencyAccumulator struct {
	holdings       float64
	assets         float64
	liabilities    float64
	invalidRecords int
}

func buildPortfolioTotals(holdings []Holding, balances []ManualBalance) []CurrencyTotal {
	currencies := make(map[string]*currencyAccumulator)

	accumulator := func(currency string) *currencyAccumulator {
		key := strings.TrimSpace(currency)
		if key == "" {
			key = "UNKNOWN"
		}
		current, ok := currencies[key]
		if !ok {
			current = &currencyAccumulator{}
			currencies[key] = current
		}
		return current
	}

	for _, holding := range holdings {
		current := accumulator(holding.Currency)
		value, ok := parseAmount(holding.CurrentValue)
		if !ok {
			current.invalidRecords++
			continue
		}
		current.holdings += value
	}

	for _, balance := range balances {
		if strings.EqualFold(strings.TrimSpace(balance.BalanceType), "cash_flow") {
			continue
		}
		current := accumulator(balance.Currency)
		value, ok := parseAmount(balance.Amount)
		if !ok {
			current.invalidRecords++
			continue
		}
		if strings.EqualFold(strings.TrimSpace(balance.BalanceType), "liability") {
			current.liabilities += math.Abs(value)
		} else {
			current.assets += value
		}
	}

	totals := make([]CurrencyTotal, 0, len(currencies))
	for currency, value := range currencies {
		totals = append(totals, CurrencyTotal{
			Currency:       currency,
			Holdings:       formatAmountString(value.holdings),
			Assets:         formatAmountString(value.assets),
			Liabilities:    formatAmountString(value.liabilities),
			Net:            formatAmountString(value.holdings + value.assets - value.liabilities),
			InvalidRecords: value.invalidRecords,
		})
	}
	sort.Slice(totals, func(i, j int) bool {
		return totals[i].Currency < totals[j].Currency
	})
	return totals
}

func buildTaxYearCompleteness(records []TaxRecord) []TaxYearCheck {
	years := make(map[string]map[string]struct{})
	for _, record := range records {
		year := strings.TrimSpace(record.FinancialYear)
		if year == "" {
			continue
		}
		sources, ok := years[year]
		if !ok {
			sources = make(map[string]struct{})
			years[year] = sources
		}
		sources[strings.TrimSpace(record.Source)] = struct{}{}
	}

	financialYears := make([]string, 0, len(years))
	for year := range years {
		financialYears = append(financialYears, year)
	}
	sort.Strings(financialYears)

	checks := make([]TaxYearCheck, 0, len(financialYears))
	for _, year := range financialYears {
		sourceSet := years[year]
		sourcesPresent := make([]string, 0, len(sourceSet))
		for source := range sourceSet {
			if source != "" {
				sourcesPresent = append(sourcesPresent, source)
			}
		}
		sort.Strings(sourcesPresent)

		missingRequired := make([]string, 0, 3)
		if _, ok := sourceSet["itr"]; !ok {
			missingRequired = append(missingRequired, "itr")
		}
		if _, ok := sourceSet["ais"]; !ok {
			missingRequired = append(missingRequired, "ais")
		}
		_, hasForm26AS := sourceSet["form26as"]
		_, hasAnnualStatement := sourceSet["annual_tax_statement"]
		if !hasForm26AS && !hasAnnualStatement {
			missingRequired = append(missingRequired, "tax_statement")
		}

		status := "complete"
		if len(missingRequired) > 0 {
			status = "incomplete"
		}

		checks = append(checks, TaxYearCheck{
			FinancialYear:   year,
			SourcesPresent:  sourcesPresent,
			MissingRequired: missingRequired,
			Status:          status,
		})
	}
	return checks
}

func transactionDelta(transaction BankTransaction) float64 {
	credit, _ := parseAmount(transaction.Credit)
	debit, _ := parseAmount(transaction.Debit)
	return credit - debit
}

func parseAmount(value string) (float64, bool) {
	text := strings.ReplaceAll(strings.TrimSpace(value), ",", "")
	if !numericAmountPattern.MatchString(text) {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, false
	}
	return parsed, true
}

func amountsDiffer(actual, expected float64) bool {
	return math.Abs(actual-expected) > 0.011
}

func formatAmount(value float64) *string {
	formatted := formatAmountString(value)
	return &formatted
}

func formatAmountString(value float64) string {
	if value == 0 {
		// Normalize negative zero so JSON never contains "-0.00".
		value = 0
	}
	return strconv.FormatFloat(value, 'f', 2, 64)
}

// BuildReconciliationFromManifests reads the latest import manifests below
// <root>/imports and maps their records into bank statements. It is read-only
// and never creates or updates a manifest. Current statement manifests do not
// contain balances, holdings, manual balances or tax records; those parts of
// the report remain empty until callers provide them via ReconciliationInput.
func BuildReconciliationFromManifests(root string) ReconciliationReport {
	input, issues := loadManifestInput(root)
	input.Issues = append(issues, input.Issues...)
	return BuildReconciliation(input)
}

type manifestRecord struct {
	Date     string   `json:"date"`
	Payee    string   `json:"payee"`
	Memo     string   `json:"memo"`
	Amount   *float64 `json:"amount"`
	Currency string   `json:"currency"`
	IsDebit  bool     `json:"is_debit"`
	Debit    string   `json:"debit"`
	Credit   string   `json:"credit"`
	Balance  string   `json:"balance"`
}

func loadManifestInput(root string) (ReconciliationInput, []string) {
	input := ReconciliationInput{
		BankStatements: []BankStatement{},
		Holdings:       []Holding{},
		Balances:       []ManualBalance{},
		TaxRecords:     []TaxRecord{},
	}
	issues := []string{}

	if strings.TrimSpace(root) == "" {
		return input, []string{"Import root is not configured."}
	}

	importRoot := filepath.Join(root, "imports")
	entries, err := os.ReadDir(importRoot)
	if os.IsNotExist(err) {
		return input, issues
	}
	if err != nil {
		return input, []string{"Could not read the import directory."}
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sourceType := entry.Name()
		manifests, err := manifest.ReadLatest(root, sourceType)
		if err != nil {
			issues = append(issues, fmt.Sprintf("Could not read %s manifests.", sourceType))
			continue
		}
		for _, imported := range manifests {
			recordsPath := filepath.Join(importRoot, sourceType, imported.ImportID, "records.json")
			data, err := os.ReadFile(recordsPath)
			if err != nil {
				issues = append(issues, fmt.Sprintf("Could not read records for %s.", imported.OriginalPath))
				continue
			}
			var records []manifestRecord
			if err := json.Unmarshal(data, &records); err != nil {
				issues = append(issues, fmt.Sprintf("Could not parse records for %s.", imported.OriginalPath))
				continue
			}

			transactions := make([]BankTransaction, 0, len(records))
			for _, record := range records {
				transaction := BankTransaction{
					Date:     record.Date,
					Debit:    record.Debit,
					Credit:   record.Credit,
					Balance:  record.Balance,
					Currency: record.Currency,
				}
				if transaction.Debit == "" && transaction.Credit == "" && record.Amount != nil {
					amount := strconv.FormatFloat(math.Abs(*record.Amount), 'f', 2, 64)
					if record.IsDebit {
						transaction.Debit = amount
					} else {
						transaction.Credit = amount
					}
				}
				transactions = append(transactions, transaction)
			}
			input.BankStatements = append(input.BankStatements, BankStatement{
				Source:       imported.OriginalPath,
				Transactions: transactions,
			})
		}
	}

	return input, issues
}
