package importer

import (
	"encoding/csv"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/importer/ofx"
	"github.com/ananthakumaran/paisa/internal/model/import_rule"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"gorm.io/gorm"
)

// IngestionFormat denotes the input file type.
type IngestionFormat string

const (
	FormatCSV   IngestionFormat = "csv"
	FormatOFX   IngestionFormat = "ofx"
	FormatQFX   IngestionFormat = "qfx"
	FormatExcel IngestionFormat = "excel"
	FormatPDF   IngestionFormat = "pdf"
	FormatCAS   IngestionFormat = "cas"
)

// StagedTransaction represents an ingested row enriched with classification and duplicate analysis.
type StagedTransaction struct {
	Index            int      `json:"index"`
	Date             string   `json:"date"`
	Payee            string   `json:"payee"`
	Memo             string   `json:"memo"`
	Amount           float64  `json:"amount"` // Absolute amount
	Currency         string   `json:"currency"`
	IsDebit          bool     `json:"is_debit"`
	SelectedAccount  string   `json:"selected_account"`
	SuggestedAccount string   `json:"suggested_account,omitempty"`
	Confidence       string   `json:"confidence,omitempty"` // "rule", "preset", "manual"
	MatchedRuleName  string   `json:"matched_rule_name,omitempty"`
	MatchedRuleID    uint     `json:"matched_rule_id,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	FlagForReview    bool     `json:"flag_for_review"`

	// Duplicate Detection
	IsDuplicate      bool   `json:"is_duplicate"`
	DuplicateScore   int    `json:"duplicate_score"` // 0-100
	DuplicateMatchID uint   `json:"duplicate_match_id,omitempty"`
	DuplicateReason  string `json:"duplicate_reason,omitempty"`
}

// IngestionRequest specifies payload for statement parsing.
type IngestionRequest struct {
	Format        IngestionFormat `json:"format"`
	Content       string          `json:"content"`
	Delimiter     string          `json:"delimiter,omitempty"`
	PresetName    string          `json:"preset_name,omitempty"`
	BaseAccount   string          `json:"base_account,omitempty"` // e.g., Assets:Checking:Chase
	SourceAccount string          `json:"source_account,omitempty"`
}

// IngestionResult returns the staged candidate list.
type IngestionResult struct {
	TotalParsed        int                 `json:"total_parsed"`
	AutoCategorized    int                 `json:"auto_categorized"`
	PossibleDuplicates int                 `json:"possible_duplicates"`
	Transactions       []StagedTransaction `json:"transactions"`
}

// Ingest parses input and enriches every transaction with rules and duplicate checks.
func Ingest(db *gorm.DB, req IngestionRequest) (*IngestionResult, error) {
	var candidates []import_rule.CandidateTransaction

	switch strings.ToLower(string(req.Format)) {
	case string(FormatOFX), string(FormatQFX):
		stmt, err := ofx.Parse(req.Content)
		if err != nil {
			return nil, fmt.Errorf("OFX parse error: %w", err)
		}
		for _, tx := range stmt.Transactions {
			candidates = append(candidates, import_rule.CandidateTransaction{
				Date:     tx.Date.Format("2006-01-02"),
				Payee:    tx.Payee,
				Memo:     tx.Memo,
				Amount:   tx.AbsAmount(),
				Currency: stmt.Currency,
				IsDebit:  tx.IsDebit(),
			})
		}

	case string(FormatCSV), string(FormatExcel), string(FormatPDF), "":
		parsed, err := parseCSVRows(req.Content, req.Delimiter, req.PresetName, db)
		if err != nil {
			return nil, err
		}
		candidates = parsed

	default:
		return nil, fmt.Errorf("unsupported format %q", req.Format)
	}

	// Fetch active user rules
	rules, _ := import_rule.ActiveRules(db)

	// Fetch existing ledger postings for duplicate detection and historical payee analysis.
	// The query is bounded to the statement's date range so large journals do not
	// require scanning the whole postings table.
	existingPostings := loadPostingsForIngestion(db, candidates)

	// Build historical payee mapping index
	historyMap := BuildHistoricalPayeeMap(existingPostings)

	// Fetch existing chart of accounts for account resolution
	existingAccounts := accounting.AllAccounts(db)

	defaultCurr := config.DefaultCurrency()
	if defaultCurr == "" {
		defaultCurr = "INR"
	}

	staged := make([]StagedTransaction, 0, len(candidates))
	autoCatCount := 0
	duplicateCount := 0

	for i, c := range candidates {
		curr := strings.TrimSpace(c.Currency)
		if curr == "" {
			curr = defaultCurr
		}

		item := StagedTransaction{
			Index:    i,
			Date:     c.Date,
			Payee:    c.Payee,
			Memo:     c.Memo,
			Amount:   c.Amount,
			Currency: curr,
			IsDebit:  c.IsDebit,
			Tags:     []string{},
		}

		// 1. Evaluate Rule Match
		match := import_rule.MatchCandidate(rules, c)
		if match.Matched {
			item.SuggestedAccount = match.TargetAccount
			item.SelectedAccount = match.TargetAccount
			item.MatchedRuleName = match.RuleName
			item.MatchedRuleID = match.RuleID
			item.Tags = match.Tags
			item.FlagForReview = match.FlagForReview
			item.Confidence = "rule"
			autoCatCount++
		} else {
			// Suggest offset account using historical postings, merchant taxonomy, or heuristics
			suggestedAcc, conf := SuggestOffsetAccount(c, existingAccounts, historyMap)
			if suggestedAcc != "" {
				item.SuggestedAccount = suggestedAcc
				item.Confidence = conf
			}
		}

		// 2. Duplicate Detection Scoring
		dupMatch, score, reason := findBestDuplicateMatch(c, existingPostings)
		if score >= 70 {
			item.IsDuplicate = true
			item.DuplicateScore = score
			item.DuplicateReason = reason
			if dupMatch != nil {
				item.DuplicateMatchID = dupMatch.ID
			}
			duplicateCount++
		}

		staged = append(staged, item)
	}

	return &IngestionResult{
		TotalParsed:        len(staged),
		AutoCategorized:    autoCatCount,
		PossibleDuplicates: duplicateCount,
		Transactions:       staged,
	}, nil
}

const ingestionDateWindowDays = 7

// loadPostingsForIngestion loads only postings that can be relevant for duplicate
// detection and historical suggestions around the statement date range.
func loadPostingsForIngestion(db *gorm.DB, candidates []import_rule.CandidateTransaction) []posting.Posting {
	minDate, maxDate, ok := candidateDateRange(candidates)
	if !ok {
		return nil
	}

	var existing []posting.Posting
	db.Select("id, transaction_id, date, payee, account, amount, note").
		Where("date >= ? AND date <= ?",
			minDate.AddDate(0, 0, -ingestionDateWindowDays),
			maxDate.AddDate(0, 0, ingestionDateWindowDays)).
		Find(&existing)

	return existing
}

func candidateDateRange(candidates []import_rule.CandidateTransaction) (time.Time, time.Time, bool) {
	var minDate, maxDate time.Time
	found := false

	for _, candidate := range candidates {
		date, err := time.Parse("2006-01-02", candidate.Date)
		if err != nil {
			continue
		}

		if !found {
			minDate = date
			maxDate = date
			found = true
			continue
		}

		if date.Before(minDate) {
			minDate = date
		}
		if date.After(maxDate) {
			maxDate = date
		}
	}

	return minDate, maxDate, found
}

func parseCSVRows(content, delimiter, presetName string, db *gorm.DB) ([]import_rule.CandidateTransaction, error) {
	reader := csv.NewReader(strings.NewReader(content))
	reader.FieldsPerRecord = -1
	if delimiter != "" {
		runes := []rune(delimiter)
		if len(runes) == 1 {
			reader.Comma = runes[0]
		}
	}

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("statement must have at least a header and one data row")
	}

	headerIdx, dateIdx, descIdx, debitIdx, creditIdx, amountIdx, drCrIdx, currIdx, fallbackDateIdx, stmtCurrency := detectHeaderRow(records)
	if headerIdx < 0 || dateIdx < 0 || (debitIdx < 0 && creditIdx < 0 && amountIdx < 0) {
		return nil, fmt.Errorf("could not detect valid transaction table header in statement")
	}

	var candidates []import_rule.CandidateTransaction

	for _, row := range records[headerIdx+1:] {
		if len(row) == 0 || allEmpty(row) {
			continue
		}

		dateRaw := ""
		if dateIdx >= 0 && dateIdx < len(row) {
			dateRaw = strings.TrimSpace(row[dateIdx])
		}
		dateStr, ok := normalizeDateString(dateRaw)
		if !ok && fallbackDateIdx >= 0 && fallbackDateIdx < len(row) {
			// Some statements annotate the transaction date with a timestamp while
			// keeping the plain value date in a separate column.
			dateRaw = strings.TrimSpace(row[fallbackDateIdx])
			dateStr, ok = normalizeDateString(dateRaw)
		}
		if !ok || dateStr == "" {
			// Skip rows that don't have a valid date in date column (e.g., summary rows, disclaimers)
			continue
		}

		desc := ""
		if descIdx >= 0 && descIdx < len(row) {
			desc = strings.TrimSpace(row[descIdx])
		}

		var amt float64
		var isDebit bool
		var rowCurr string

		if currIdx >= 0 && currIdx < len(row) {
			rowCurr = normalizeCurrencyToken(row[currIdx])
		}

		if debitIdx >= 0 && debitIdx < len(row) && strings.TrimSpace(row[debitIdx]) != "" {
			val, _, isCr, parsedCurr := parseAmountFloat(row[debitIdx])
			if val > 0 {
				amt = val
				isDebit = !isCr // Default true for debit column unless explicitly marked Cr
				if rowCurr == "" && parsedCurr != "" {
					rowCurr = parsedCurr
				}
			}
		}

		if amt == 0 && creditIdx >= 0 && creditIdx < len(row) && strings.TrimSpace(row[creditIdx]) != "" {
			val, isDr, _, parsedCurr := parseAmountFloat(row[creditIdx])
			if val > 0 {
				amt = val
				isDebit = isDr // Default false for credit column unless explicitly marked Dr
				if rowCurr == "" && parsedCurr != "" {
					rowCurr = parsedCurr
				}
			}
		}

		if amt == 0 && amountIdx >= 0 && amountIdx < len(row) && strings.TrimSpace(row[amountIdx]) != "" {
			val, d, isCr, parsedCurr := parseAmountFloat(row[amountIdx])
			if val > 0 {
				amt = val
				if rowCurr == "" && parsedCurr != "" {
					rowCurr = parsedCurr
				}
				if drCrIdx >= 0 && drCrIdx < len(row) {
					indicator := strings.ToUpper(strings.TrimSpace(row[drCrIdx]))
					if strings.HasPrefix(indicator, "DR") || indicator == "D" || indicator == "DEBIT" {
						isDebit = true
					} else if strings.HasPrefix(indicator, "CR") || indicator == "C" || indicator == "CREDIT" {
						isDebit = false
					} else if isCr {
						isDebit = false
					} else if d {
						isDebit = true
					} else {
						isDebit = true
					}
				} else {
					if isCr {
						isDebit = false
					} else if d {
						isDebit = true
					} else {
						// Fallback: positive amount in single amount column usually treated as debit unless sign indicates otherwise
						isDebit = true
					}
				}
			}
		}

		if rowCurr == "" {
			rowCurr = stmtCurrency
		}

		if amt > 0 {
			candidates = append(candidates, import_rule.CandidateTransaction{
				Date:     dateStr,
				Payee:    desc,
				Memo:     desc,
				Amount:   amt,
				Currency: rowCurr,
				IsDebit:  isDebit,
			})
		}
	}

	return candidates, nil
}

var headerCurrencyRe = regexp.MustCompile(`(?i)\(([A-Z]{3})\)|\b(INR|USD|EUR|GBP|CAD|AUD|SGD|JPY|CHF|AED)\b`)

// detectHeaderRow scans the top rows of a statement (up to 50 rows) to find where the actual transaction table begins.
func detectHeaderRow(records [][]string) (headerIdx, dateIdx, descIdx, debitIdx, creditIdx, amountIdx, drCrIdx, currIdx, fallbackDateIdx int, stmtCurrency string) {
	maxScan := len(records)
	if maxScan > 50 {
		maxScan = 50
	}

	bestHeaderIdx := -1
	bestScore := -1
	bDate, bDesc, bDebit, bCredit, bAmount, bDrCr, bCurr, bFallbackDate := -1, -1, -1, -1, -1, -1, -1, -1

	for i := 0; i < maxScan; i++ {
		row := records[i]
		if len(row) == 0 || allEmpty(row) {
			continue
		}

		dIdx := findColumnIndex(row, []string{"transaction date", "txn date", "value date", "trans date", "date"})
		deIdx := findColumnIndex(row, []string{"narration", "description", "particulars", "memo", "payee", "details", "name"})
		drIdx := findColumnIndex(row, []string{"debit amount", "withdrawal", "debit", "dr"})
		crIdx := findColumnIndex(row, []string{"credit amount", "deposit", "credit", "cr"})
		aIdx := findColumnIndex(row, []string{"amount (inr)", "net amount", "trans amount", "amount"})
		cIdx := findColumnIndex(row, []string{"currency", "curr", "ccy", "iso currency"})
		drCr := findDrCrIndicatorIndex(row, aIdx)

		var fbDate int = -1
		if dIdx >= 0 {
			for idx, cell := range row {
				if idx == dIdx {
					continue
				}
				clean := strings.ToLower(strings.TrimSpace(cell))
				if strings.Contains(clean, "date") {
					fbDate = idx
					break
				}
			}
		}

		score := 0
		if dIdx >= 0 {
			score += 4
		}
		if deIdx >= 0 {
			score += 3
		}
		if drIdx >= 0 {
			score += 3
		}
		if crIdx >= 0 {
			score += 3
		}
		if aIdx >= 0 {
			score += 3
		}
		if drCr >= 0 {
			score += 2
		}

		// Must have date and at least one amount column
		if dIdx >= 0 && (drIdx >= 0 || crIdx >= 0 || aIdx >= 0) && score > bestScore {
			bestScore = score
			bestHeaderIdx = i
			bDate = dIdx
			bDesc = deIdx
			bDebit = drIdx
			bCredit = crIdx
			bAmount = aIdx
			bDrCr = drCr
			bCurr = cIdx
			bFallbackDate = fbDate
		}
	}

	// Extract currency hint from header row or top metadata section
	if bestHeaderIdx >= 0 {
		for _, cell := range records[bestHeaderIdx] {
			if m := headerCurrencyRe.FindStringSubmatch(cell); len(m) > 0 {
				if m[1] != "" {
					stmtCurrency = strings.ToUpper(m[1])
					break
				} else if m[2] != "" {
					stmtCurrency = strings.ToUpper(m[2])
					break
				}
			}
		}
		if stmtCurrency == "" {
			for i := 0; i < bestHeaderIdx; i++ {
				for _, cell := range records[i] {
					if m := headerCurrencyRe.FindStringSubmatch(cell); len(m) > 0 {
						if m[1] != "" {
							stmtCurrency = strings.ToUpper(m[1])
							break
						} else if m[2] != "" {
							stmtCurrency = strings.ToUpper(m[2])
							break
						}
					}
				}
				if stmtCurrency != "" {
					break
				}
			}
		}
	}

	return bestHeaderIdx, bDate, bDesc, bDebit, bCredit, bAmount, bDrCr, bCurr, bFallbackDate, stmtCurrency
}

func isDrCrIndicatorHeader(h string) bool {
	clean := strings.ToLower(strings.TrimSpace(h))
	switch clean {
	case "dr / cr", "dr/cr", "cr / dr", "cr/dr", "dr cr", "cr dr", "dr", "cr", "d/c", "type", "txn type", "transaction type", "dr/cr indicator", "cr/dr indicator":
		return true
	}
	return false
}

func findDrCrIndicatorIndex(header []string, amountIdx int) int {
	// If amount column is detected, prefer adjacent column (e.g. column right after Amount)
	if amountIdx >= 0 {
		if amountIdx+1 < len(header) && isDrCrIndicatorHeader(header[amountIdx+1]) {
			return amountIdx + 1
		}
		if amountIdx-1 >= 0 && isDrCrIndicatorHeader(header[amountIdx-1]) {
			return amountIdx - 1
		}
	}
	// Otherwise look for any matching column
	for i, h := range header {
		if isDrCrIndicatorHeader(h) {
			return i
		}
	}
	return -1
}

func findColumnIndex(header []string, targets []string) int {
	// First pass: exact match (case-insensitive)
	for i, h := range header {
		clean := strings.ToLower(strings.TrimSpace(h))
		for _, target := range targets {
			if clean == target {
				return i
			}
		}
	}
	// Second pass: substring match for targets with length > 2
	for i, h := range header {
		clean := strings.ToLower(strings.TrimSpace(h))
		for _, target := range targets {
			if len(target) > 2 && strings.Contains(clean, target) {
				return i
			}
		}
	}
	return -1
}

func allEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func normalizeCurrencyToken(raw string) string {
	clean := strings.ToUpper(strings.TrimSpace(raw))
	switch clean {
	case "$", "US$", "USD":
		return "USD"
	case "₹", "RS", "RS.", "INR":
		return "INR"
	case "€", "EUR":
		return "EUR"
	case "£", "GBP":
		return "GBP"
	default:
		if len(clean) == 3 {
			return clean
		}
		return ""
	}
}

func parseAmountFloat(val string) (amount float64, isDebit bool, isCredit bool, currency string) {
	cleaned := strings.TrimSpace(val)
	upper := strings.ToUpper(cleaned)

	if strings.HasSuffix(upper, "DR.") || strings.HasSuffix(upper, "DR") {
		isDebit = true
		cleaned = strings.TrimSuffix(cleaned, "Dr.")
		cleaned = strings.TrimSuffix(cleaned, "dr.")
		cleaned = strings.TrimSuffix(cleaned, "Dr")
		cleaned = strings.TrimSuffix(cleaned, "dr")
		cleaned = strings.TrimSuffix(cleaned, "DR")
	} else if strings.HasSuffix(upper, "CR.") || strings.HasSuffix(upper, "CR") {
		isCredit = true
		cleaned = strings.TrimSuffix(cleaned, "Cr.")
		cleaned = strings.TrimSuffix(cleaned, "cr.")
		cleaned = strings.TrimSuffix(cleaned, "Cr")
		cleaned = strings.TrimSuffix(cleaned, "cr")
		cleaned = strings.TrimSuffix(cleaned, "CR")
	}

	upperCleaned := strings.ToUpper(cleaned)
	switch {
	case strings.Contains(upperCleaned, "INR") || strings.Contains(cleaned, "₹") || strings.Contains(upperCleaned, "RS."):
		currency = "INR"
	case strings.Contains(upperCleaned, "USD") || strings.Contains(cleaned, "$"):
		currency = "USD"
	case strings.Contains(upperCleaned, "EUR") || strings.Contains(cleaned, "€"):
		currency = "EUR"
	case strings.Contains(upperCleaned, "GBP") || strings.Contains(cleaned, "£"):
		currency = "GBP"
	}

	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.ReplaceAll(cleaned, "$", "")
	cleaned = strings.ReplaceAll(cleaned, "₹", "")
	cleaned = strings.ReplaceAll(cleaned, "€", "")
	cleaned = strings.ReplaceAll(cleaned, "£", "")
	for _, code := range []string{"INR", "USD", "EUR", "GBP", "CAD", "AUD", "SGD", "Rs.", "RS.", "rs."} {
		cleaned = strings.ReplaceAll(cleaned, code, "")
	}
	cleaned = strings.TrimSpace(cleaned)

	parsed, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, false, false, ""
	}

	if parsed < 0 {
		return math.Abs(parsed), true, false, currency
	}
	if isDebit {
		return parsed, true, false, currency
	}
	if isCredit {
		return parsed, false, true, currency
	}
	return parsed, false, false, currency
}

func normalizeDateString(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	// Try parsing standard date formats
	formats := []string{
		"2006-01-02", "02/01/2006", "01/02/2006", "02-01-2006", "01-02-2006",
		"02-Jan-2006", "02 Jan 2006", "2006/01/02", "2/1/2006", "02/01/06", "02-01-06",
		"02-Jan-06", "02 Jan 06", "2/1/06", "2-1-06", "2-1-2006",
		"2006-01-02 15:04:05", "02/01/2006 15:04:05", "02-01-2006 15:04:05",
		"2006-01-02 15:04", "02/01/2006 15:04", "02-01-2006 15:04",
		"02-01-06 15:04", "02-01-06 15:04:05", "02/01/06 15:04", "02/01/06 15:04:05",
		"2/1/2006 15:04", "2/1/2006 15:04:05", "2/1/06 15:04",
		"2006-01-02T15:04:05", "2006-01-02T15:04:05Z",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, raw); err == nil {
			return t.Format("2006-01-02"), true
		}
	}

	// If there's a space-separated time component, strip it and try date part
	fields := strings.Fields(raw)
	if len(fields) >= 2 {
		if strings.Contains(fields[1], ":") {
			for _, f := range formats {
				if t, err := time.Parse(f, fields[0]); err == nil {
					return t.Format("2006-01-02"), true
				}
			}
		}
		lastIdx := len(fields) - 1
		if strings.Contains(fields[lastIdx], ":") {
			datePart := strings.Join(fields[:lastIdx], " ")
			for _, f := range formats {
				if t, err := time.Parse(f, datePart); err == nil {
					return t.Format("2006-01-02"), true
				}
			}
		}
	}

	return "", false
}

func findBestDuplicateMatch(c import_rule.CandidateTransaction, existing []posting.Posting) (*posting.Posting, int, string) {
	cTime, err := time.Parse("2006-01-02", c.Date)
	if err != nil {
		cTime = time.Now()
	}

	var bestMatch *posting.Posting
	highestScore := 0
	bestReason := ""

	for i := range existing {
		p := &existing[i]
		score := 0
		reasons := []string{}

		// A matching amount and date alone is not enough to flag a duplicate:
		// recurring amounts with a different merchant must stay distinct.
		pPayee := normalizeDuplicatePayee(p.Payee + " " + p.Note)
		cPayee := normalizeDuplicatePayee(c.Payee + " " + c.Memo)
		if pPayee == "" || cPayee == "" {
			continue
		}
		if !strings.Contains(pPayee, cPayee) && !strings.Contains(cPayee, pPayee) {
			continue
		}

		// 1. Amount match
		pAmt := math.Abs(p.Amount.InexactFloat64())

		diff := math.Abs(pAmt - c.Amount)
		if diff < 0.01 {
			score += 50
			reasons = append(reasons, "exact amount match")
		} else if diff <= 0.05 {
			score += 30
		} else {
			continue // Non-matching amounts are unlikely duplicates
		}

		// 2. Date proximity
		daysDiff := math.Abs(p.Date.Sub(cTime).Hours() / 24.0)
		if daysDiff == 0 {
			score += 30
			reasons = append(reasons, "same day")
		} else if daysDiff <= 3 {
			score += 20
			reasons = append(reasons, fmt.Sprintf("within %.0f days", daysDiff))
		} else if daysDiff <= 7 {
			score += 10
		} else {
			continue // More than 7 days apart is usually distinct
		}

		// 3. Payee / Note similarity
		score += 20
		reasons = append(reasons, "payee text match")

		if score > highestScore {
			highestScore = score
			bestMatch = p
			bestReason = strings.Join(reasons, ", ")
		}
	}

	return bestMatch, highestScore, bestReason
}

func normalizeDuplicatePayee(payee string) string {
	return strings.Join(strings.Fields(strings.ToLower(payee)), " ")
}
