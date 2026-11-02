package importer

import (
	"math"
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/import_rule"
	"github.com/ananthakumaran/paisa/internal/model/posting"
)

// Common merchant taxonomy mapping keywords/tokens to default account categories.
var defaultMerchantTaxonomy = []struct {
	Keywords []string
	Account  string
}{
	// Food & Dining
	{
		Keywords: []string{"swiggy", "zomato", "mcdonald", "starbucks", "dominos", "pizza", "burger", "cafe", "restaurant", "eats", "diner", "subway", "kfc", "chai", "tea point", "coffee"},
		Account:  "Expenses:Food:Dining",
	},
	// Groceries
	{
		Keywords: []string{"blinkit", "zepto", "instamart", "dmart", "bigbasket", "grofers", "supermarket", "nature basket", "spencer", "reliance fresh", "more retail", "kirana", "provision"},
		Account:  "Expenses:Food:Groceries",
	},
	// Transport & Travel
	{
		Keywords: []string{"uber", "ola", "rapido", "irctc", "makemytrip", "goibibo", "yatra", "cleartrip", "indigo", "air india", "vistara", "akasa", "metro", "fastag", "toll", "hpcl", "bpcl", "ioc", "iocl", "petrol", "fuel", "shell"},
		Account:  "Expenses:Transport",
	},
	// Utilities & Bills
	{
		Keywords: []string{"bescom", "tneb", "mseb", "airtel", "jio", "vi bill", "vodafone", "tata play", "tatasky", "broadband", "act fibernet", "electricity", "water supply", "gas bill", "indane", "bharat gas", "hp gas", "mahanagar gas", "igl"},
		Account:  "Expenses:Utilities",
	},
	// Shopping & E-Commerce
	{
		Keywords: []string{"amazon", "amzn", "flipkart", "myntra", "ajio", "meesho", "nykaa", "tata cliq", "decathlon", "ikea", "zara", "h&m"},
		Account:  "Expenses:Shopping",
	},
	// Entertainment & Subscriptions
	{
		Keywords: []string{"netflix", "spotify", "prime video", "hotstar", "disney", "youtube", "apple.com/bill", "apple music", "google play", "playstore", "bookmyshow", "pvr", "inox", "cinema"},
		Account:  "Expenses:Entertainment",
	},
	// Healthcare & Pharmacy
	{
		Keywords: []string{"apollo", "pharmeasy", "1mg", "netmeds", "medplus", "hospital", "clinic", "pharmacy", "dr lal", "metropolis", "diagnostics"},
		Account:  "Expenses:Health",
	},
	// Credit Card & Bank Transfers / Repayments
	{
		Keywords: []string{"cred", "credit card payment", "cc payment", "autopay cc", "billdesk payment", "repayment"},
		Account:  "Liabilities:CreditCard",
	},
	// Investments
	{
		Keywords: []string{"zerodha", "groww", "kuvera", "indmoney", "upstox", "angelone", "camsonline", "kfintech", "mf utility", "coin"},
		Account:  "Assets:Investments",
	},
}

// HistoricalPayeeStats tracks frequency and latest usage of accounts for a normalized payee.
type HistoricalPayeeStats struct {
	TotalPostings int
	AccountCounts map[string]int
	LatestDate    time.Time
	LatestAccount string
}

// BuildHistoricalPayeeMap builds an in-memory index from existing postings mapping normalized payee names
// to their historically used offset accounts.
func BuildHistoricalPayeeMap(postings []posting.Posting) map[string]*HistoricalPayeeStats {
	result := make(map[string]*HistoricalPayeeStats)

	for _, p := range postings {
		payeeNorm := NormalizePayeeName(p.Payee)
		if payeeNorm == "" {
			continue
		}
		// Skip internal transfers or intermediate checking accounts if recorded as the payee
		acc := strings.TrimSpace(p.Account)
		if acc == "" {
			continue
		}

		stats, exists := result[payeeNorm]
		if !exists {
			stats = &HistoricalPayeeStats{
				AccountCounts: make(map[string]int),
			}
			result[payeeNorm] = stats
		}

		stats.TotalPostings++
		stats.AccountCounts[acc]++
		if stats.LatestDate.IsZero() || p.Date.After(stats.LatestDate) {
			stats.LatestDate = p.Date
			stats.LatestAccount = acc
		}
	}

	return result
}

// NormalizePayeeName cleans raw payees into a canonical search key.
func NormalizePayeeName(raw string) string {
	clean := CleanPayee(raw)
	if clean != "" {
		return strings.ToLower(clean)
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

// SuggestOffsetAccount combines:
// 1. Historical ledger transaction matching (highest priority if history exists)
// 2. Built-in merchant taxonomy matching best aligned with user's chart of accounts
// 3. Amount & polarity heuristics
func SuggestOffsetAccount(
	tx import_rule.CandidateTransaction,
	existingAccounts []string,
	historyMap map[string]*HistoricalPayeeStats,
) (suggestedAccount string, confidence string) {
	queryText := strings.ToLower(strings.Join([]string{tx.Payee, tx.Memo}, " "))
	normPayee := NormalizePayeeName(tx.Payee)

	// --- 1. Historical Ledger Posting Lookup ---
	if historyMap != nil && normPayee != "" {
		// Check exact normalized match
		if stats, ok := historyMap[normPayee]; ok && stats.TotalPostings > 0 {
			bestAcc := pickBestAccount(stats.AccountCounts, tx.IsDebit)
			if bestAcc != "" {
				return bestAcc, "historical"
			}
		}

		// Check partial token match against history keys
		for histPayee, stats := range historyMap {
			if len(histPayee) >= 3 && (strings.Contains(normPayee, histPayee) || strings.Contains(histPayee, normPayee)) {
				bestAcc := pickBestAccount(stats.AccountCounts, tx.IsDebit)
				if bestAcc != "" {
					return bestAcc, "historical"
				}
			}
		}
	}

	// --- 2. Built-in Category & Keyword Taxonomy ---
	for _, item := range defaultMerchantTaxonomy {
		matched := false
		for _, kw := range item.Keywords {
			if strings.Contains(queryText, kw) {
				matched = true
				break
			}
		}
		if matched {
			// Find closest matching account in user's chart of accounts
			resolved := ResolveBestMatchingAccount(item.Account, existingAccounts, tx.IsDebit)
			if resolved != "" {
				return resolved, "taxonomy"
			}
		}
	}

	// --- 3. Amount & Polarity Context Heuristics ---
	amt := tx.Amount
	if tx.IsDebit {
		// Micro debits (< 500) with generic terms often Food/Groceries/Transport
		if amt <= 500 && (strings.Contains(queryText, "food") || strings.Contains(queryText, "tea") || strings.Contains(queryText, "bakery") || strings.Contains(queryText, "snack") || strings.Contains(queryText, "sweet")) {
			resolved := ResolveBestMatchingAccount("Expenses:Food", existingAccounts, true)
			if resolved != "" {
				return resolved, "heuristic"
			}
		}
		// Round large debits (e.g., 5000, 10000, 25000, 50000) with payment/transfer terms
		if isRoundAmount(amt) && amt >= 5000 && (strings.Contains(queryText, "transfer") || strings.Contains(queryText, "tfr") || strings.Contains(queryText, "self")) {
			resolved := ResolveBestMatchingAccount("Assets:Transfer", existingAccounts, true)
			if resolved != "" {
				return resolved, "heuristic"
			}
		}
	} else {
		// Credits with salary / interest / refund keywords
		if strings.Contains(queryText, "salary") || strings.Contains(queryText, "payroll") {
			resolved := ResolveBestMatchingAccount("Income:Salary", existingAccounts, false)
			if resolved != "" {
				return resolved, "heuristic"
			}
		}
		if strings.Contains(queryText, "interest") || strings.Contains(queryText, "int.pd") {
			resolved := ResolveBestMatchingAccount("Income:Interest", existingAccounts, false)
			if resolved != "" {
				return resolved, "heuristic"
			}
		}
		if strings.Contains(queryText, "dividend") {
			resolved := ResolveBestMatchingAccount("Income:Dividend", existingAccounts, false)
			if resolved != "" {
				return resolved, "heuristic"
			}
		}
		if strings.Contains(queryText, "refund") || strings.Contains(queryText, "reversal") || strings.Contains(queryText, "cashback") {
			resolved := ResolveBestMatchingAccount("Expenses:Rebate", existingAccounts, false)
			if resolved != "" {
				return resolved, "heuristic"
			}
		}
	}

	return "", ""
}

func isRoundAmount(amt float64) bool {
	if amt <= 0 {
		return false
	}
	intPart := int(amt)
	if math.Abs(float64(intPart)-amt) > 0.001 {
		return false
	}
	return intPart%1000 == 0
}

// pickBestAccount selects the most frequent account compatible with debit/credit direction.
func pickBestAccount(counts map[string]int, isDebit bool) string {
	best := ""
	maxCount := -1

	for acc, count := range counts {
		if isDebit {
			// Debits should favor Expenses, Liabilities, Assets (exclude Equity/Income unless only option)
			if strings.HasPrefix(acc, "Equity:") || strings.HasPrefix(acc, "Income:") {
				continue
			}
		} else {
			// Credits should favor Income, Assets, Liabilities (exclude Expenses/Equity unless only option)
			if strings.HasPrefix(acc, "Equity:") || strings.HasPrefix(acc, "Expenses:") {
				continue
			}
		}

		if count > maxCount {
			maxCount = count
			best = acc
		}
	}

	// Fallback to absolute highest if no polarity match
	if best == "" {
		for acc, count := range counts {
			if count > maxCount {
				maxCount = count
				best = acc
			}
		}
	}
	return best
}

// ResolveBestMatchingAccount maps a target taxonomy path (e.g. "Expenses:Food:Dining") to the user's
// existing chart of accounts by walking upwards (e.g. Expenses:Food:Dining -> Expenses:Food -> Expenses:Dining -> Expenses).
func ResolveBestMatchingAccount(target string, existingAccounts []string, isDebit bool) string {
	if len(existingAccounts) == 0 {
		return target
	}

	targetLower := strings.ToLower(target)
	for _, acc := range existingAccounts {
		if strings.ToLower(acc) == targetLower {
			return acc
		}
	}

	// Try sub-tree or parent matching:
	// 1. Prefix match (e.g., target is Expenses:Food, user has Expenses:Food:Groceries)
	for _, acc := range existingAccounts {
		if strings.HasPrefix(strings.ToLower(acc), targetLower+":") {
			return acc
		}
	}

	// 2. Walk up parent branches (e.g., target is Expenses:Food:Dining -> try Expenses:Food, then Expenses:Dining)
	parts := strings.Split(target, ":")
	if len(parts) > 2 {
		parent := strings.Join(parts[:len(parts)-1], ":")
		for _, acc := range existingAccounts {
			if strings.EqualFold(acc, parent) {
				return acc
			}
		}
		// Also check root + leaf (e.g., Expenses:Dining)
		rootLeaf := parts[0] + ":" + parts[len(parts)-1]
		for _, acc := range existingAccounts {
			if strings.EqualFold(acc, rootLeaf) {
				return acc
			}
		}
	}

	// 3. Match any account containing the leaf token
	if len(parts) > 1 {
		leaf := strings.ToLower(parts[len(parts)-1])
		for _, acc := range existingAccounts {
			accLower := strings.ToLower(acc)
			if isDebit && !strings.HasPrefix(accLower, "expenses:") {
				continue
			}
			if !isDebit && !strings.HasPrefix(accLower, "income:") {
				continue
			}
			if strings.Contains(accLower, leaf) {
				return acc
			}
		}
	}

	// 4. Return target as-is if no closer match found
	return target
}
