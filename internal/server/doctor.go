package server

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Level string

const (
	WARN  Level = "warning"
	ERROR Level = "danger"
)

type Issue struct {
	Level       Level  `json:"level"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Details     string `json:"details"`
}

type Rule struct {
	Issue     Issue
	Predicate func(db *gorm.DB) []error
}

const DATE_FORMAT string = "02 Jan 2006"

var rules []Rule

func init() {
	rules = []Rule{
		{
			Issue: Issue{
				Level:       ERROR,
				Summary:     "Negative Balance",
				Description: "The running balance of an <b>asset</b> account is not supposed to go negative at any time. This issue typically happens due to incorrect transaction entries."},
			Predicate: ruleAssetRegisterNonNegative},
		{
			Issue: Issue{
				Level:       ERROR,
				Summary:     "Credit Entry",
				Description: "Income account should never have credit entry."},
			Predicate: ruleNonCreditAccount},
		{
			Issue: Issue{
				Level:       WARN,
				Summary:     "Debit Entry",
				Description: "Expense accounts have a debit entry. Refunds, insurance payouts and reimbursements are usually booked this way on purpose, so this is only a hint to review them."},
			Predicate: ruleNonDebitAccount},
		{
			Issue: Issue{
				Level:       ERROR,
				Summary:     "Exchange Price Missing",
				Description: "Exchange price is missing for the commodity."},
			Predicate: ruleExchangePriceMissing},
		{
			Issue: Issue{
				Level:       WARN,
				Summary:     "Unit Price Mismatch",
				Description: "Unit price used in the journal doesn't match the price fetched from external system."},
			Predicate: ruleJournalPriceMismatch},
		{
			Issue: Issue{
				Level:       WARN,
				Summary:     "Asset Accounts missing from Allocation Target",
				Description: "Asset accounts are not part of any allocation target."},
			Predicate: ruleAllocationTargetMissingAssetAccounts}}
}

func GetDiagnosis(db *gorm.DB) gin.H {
	issues := make([]Issue, 0)
	for _, rule := range rules {
		for _, error := range rule.Predicate(db) {
			issue := rule.Issue
			issue.Details = error.Error()
			issues = append(issues, issue)
		}
	}
	return gin.H{"issues": issues}
}

func ruleAssetRegisterNonNegative(db *gorm.DB) []error {
	errs := make([]error, 0)
	assets := query.Init(db).Like("Assets:%").All()
	for account, ps := range lo.GroupBy(assets, func(posting posting.Posting) string { return posting.Account }) {
		for _, balance := range accounting.Register(ps) {
			if balance.Quantity.LessThan(decimal.NewFromFloat(0.01).Neg()) {
				errs = append(errs, errors.New(fmt.Sprintf("<b>%s</b> account went negative (%.2f) on %s", account, balance.Quantity.InexactFloat64(), balance.Date.Format(DATE_FORMAT))))
				break
			}
		}
	}
	return errs
}

func ruleNonCreditAccount(db *gorm.DB) []error {
	errs := make([]error, 0)
	incomes := query.Init(db).Like("Income:%").NotLike("Income:CapitalGains:%").All()
	for _, p := range incomes {
		if p.Amount.GreaterThan(decimal.NewFromFloat(0.01)) {
			errs = append(errs, errors.New(fmt.Sprintf("<b>%.4f</b> got credited to <b>%s</b> on %s", p.Amount.InexactFloat64(), p.Account, p.Date.Format(DATE_FORMAT))))
		}
	}
	return errs
}

func ruleNonDebitAccount(db *gorm.DB) []error {
	postings := query.Init(db).Like("Expenses:%").All()

	refunds := lo.Filter(postings, func(p posting.Posting, _ int) bool {
		return p.Amount.LessThan(decimal.NewFromFloat(0.01).Neg())
	})

	if len(refunds) == 0 {
		return []error{}
	}

	grouped := lo.GroupBy(refunds, func(p posting.Posting) string { return p.Account })
	accounts := lo.Keys(grouped)
	sort.Strings(accounts)

	total := decimal.Zero
	lines := make([]string, 0, len(accounts))
	for _, account := range accounts {
		sum := decimal.Zero
		for _, p := range grouped[account] {
			sum = sum.Add(p.Amount)
		}
		total = total.Add(sum)
		lines = append(lines, fmt.Sprintf("<b>%s</b>: %d %s, %.2f %s", account, len(grouped[account]), pluralize("entry", "entries", len(grouped[account])), sum.InexactFloat64(), config.DefaultCurrency()))
	}

	// One aggregated hint instead of one entry per posting: refunds are a
	// legitimate pattern and should not drown the actual problems.
	return []error{errors.New(fmt.Sprintf("%d %s with a debit entry, %.2f %s in total.<br/>%s", len(refunds), pluralize("posting", "postings", len(refunds)), total.InexactFloat64(), config.DefaultCurrency(), strings.Join(lines, "<br/>")))}
}

func pluralize(singular string, plural string, count int) string {
	if count == 1 {
		return singular
	}
	return plural
}

func ruleExchangePriceMissing(db *gorm.DB) []error {
	errs := make([]error, 0)
	postings := query.Init(db).Desc().All()

	for _, p := range postings {
		if !utils.IsCurrency(p.Commodity) {
			externalPrice := service.GetUnitPrice(db, p.Commodity, p.Date)
			if externalPrice.CommodityName != "" && externalPrice.CommodityName != p.Commodity {
				errs = append(errs, errors.New(fmt.Sprintf("Exchange price from <b>%s</b> to your default currency <b>%s</b> is not specified for posting %s", p.Commodity, config.DefaultCurrency(), formatPosting(p))))
			}
		}
	}
	return errs
}

// priceMismatchTolerance is the relative difference tolerated between the price
// recorded in the journal and the quote fetched from the price provider.
// Fractional orders, savings plans and limit orders execute at prices that
// differ from the daily close, so an exact match can never be expected.
const priceMismatchTolerance = 0.05

func ruleJournalPriceMismatch(db *gorm.DB) []error {
	errs := make([]error, 0)

	type candidate struct {
		posting posting.Posting
		quote   price.Price
	}

	candidates := make([]candidate, 0)
	ratios := make(map[string][]float64)

	for _, p := range query.Init(db).Desc().All() {
		if utils.IsCurrency(p.Commodity) || p.Quantity.IsZero() || service.IsSellWithCapitalGains(db, p) {
			continue
		}

		quote := service.GetUnitPrice(db, p.Commodity, p.Date)
		if quote.CommodityName != p.Commodity || quote.CommodityType == config.Unknown || !quote.Value.IsPositive() {
			continue
		}

		candidates = append(candidates, candidate{posting: p, quote: quote})
		ratios[p.Commodity] = append(ratios[p.Commodity], p.Price().Div(quote.Value).InexactFloat64())
	}

	// Yahoo restates historical closes for corporate actions (share splits,
	// rights issues). Such a series is scaled against the execution prices
	// recorded in the journal, so comparing them per posting only produces
	// noise. Detect the systematic offset through the median ratio and skip
	// that commodity entirely. A single posting is never enough evidence,
	// otherwise one wrong price would hide itself.
	adjusted := make(map[string]bool, len(ratios))
	for commodity, rs := range ratios {
		if len(rs) >= 3 && math.Abs(median(rs)-1) > priceMismatchTolerance {
			adjusted[commodity] = true
		}
	}

	for _, c := range candidates {
		if adjusted[c.posting.Commodity] {
			continue
		}

		diff := c.quote.Value.Sub(c.posting.Price()).Abs()
		if diff.Div(c.quote.Value).GreaterThan(decimal.NewFromFloat(priceMismatchTolerance)) {
			errs = append(errs, errors.New(fmt.Sprintf("The price specified in your posting %s doesn't match the price <b>%.4f</b> (%s) fetched from external system", formatPosting(c.posting), c.quote.Value.InexactFloat64(), c.quote.Date.Format(DATE_FORMAT))))
		}
	}

	return errs
}

func median(values []float64) float64 {
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}

	return (sorted[middle-1] + sorted[middle]) / 2
}

func formatPosting(p posting.Posting) string {
	var price string
	if p.Quantity.Equal(p.Amount) {
		price = fmt.Sprintf("%.4f %s", p.Quantity.InexactFloat64(), p.Commodity)
	} else {
		price = fmt.Sprintf("%.4f %s @ %.4f %s", p.Quantity.InexactFloat64(), p.Commodity, p.Price().InexactFloat64(), config.DefaultCurrency())
	}

	postingUrl := fmt.Sprintf("/ledger/editor/%s#%d", url.PathEscape(p.FileName), p.TransactionBeginLine)
	return fmt.Sprintf("<a href=\"%s\"> %s\t%s\t%s</a>", postingUrl, p.Date.Format(DATE_FORMAT), p.Account, price)
}

func ruleAllocationTargetMissingAssetAccounts(db *gorm.DB) []error {
	errs := make([]error, 0)

	if len(config.GetConfig().AllocationTargets) == 0 {
		return errs
	}

	var accounts []string
	db.Model(&posting.Posting{}).Where("account like ?", "Assets:%").Distinct().Pluck("Account", &accounts)

	ignoredAccounts := make([]string, 0)
	for _, account := range accounts {
		found := false
		for _, target := range config.GetConfig().AllocationTargets {
			for _, targetAccount := range target.Accounts {
				match, _ := filepath.Match(targetAccount, account)
				if match {
					found = true
					break
				}
			}

			if found {
				break
			}
		}

		if !found {
			ignoredAccounts = append(ignoredAccounts, account)
		}
	}

	if len(ignoredAccounts) > 0 {
		errs = append(errs, errors.New(fmt.Sprintf("The following asset accounts are not part of any asset allocation target: <b>%s</b>", strings.Join(ignoredAccounts, ", "))))
	}

	return errs
}
