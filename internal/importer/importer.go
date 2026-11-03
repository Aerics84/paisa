package importer

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/importer/manifest"
	"github.com/ananthakumaran/paisa/internal/importer/ofx"
	"github.com/ananthakumaran/paisa/internal/model/import_rule"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"gorm.io/gorm"
)

const (
	// IngestionParserName identifies the statement parser in import manifests.
	IngestionParserName = "paisa-import"
	// IngestionParserVersion changes whenever parsing semantics change enough
	// that a previously imported statement must be parsed again.
	IngestionParserVersion = "1"
)

// ingestConfigDir is a seam so tests can keep manifests inside a temp directory.
var ingestConfigDir = config.GetConfigDir

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

	// Balance is the running statement balance for rows that carry one.
	Balance *float64 `json:"balance,omitempty"`

	// Duplicate Detection
	IsDuplicate      bool   `json:"is_duplicate"`
	DuplicateScore   int    `json:"duplicate_score"` // 0-100
	DuplicateMatchID uint   `json:"duplicate_match_id,omitempty"`
	DuplicateReason  string `json:"duplicate_reason,omitempty"`
}

// IngestionColumnMapping explicitly maps statement columns. Indexes are
// zero-based; -1 means the column is absent. HeaderRow is the zero-based row
// containing the column names. When supplied, this bypasses the language-
// specific header heuristics.
type IngestionColumnMapping struct {
	HeaderRow   int `json:"header_row"`
	Date        int `json:"date"`
	Description int `json:"description"`
	Debit       int `json:"debit"`
	Credit      int `json:"credit"`
	Amount      int `json:"amount"`
	Balance     int `json:"balance"`
	Currency    int `json:"currency"`
	DrCr        int `json:"dr_cr"`
}

// IngestionRequest specifies payload for statement parsing.
type IngestionRequest struct {
	Format        IngestionFormat         `json:"format"`
	Content       string                  `json:"content"`
	Delimiter     string                  `json:"delimiter,omitempty"`
	PresetName    string                  `json:"preset_name,omitempty"`
	BaseAccount   string                  `json:"base_account,omitempty"` // e.g., Assets:Checking:Chase
	SourceAccount string                  `json:"source_account,omitempty"`
	ColumnMapping *IngestionColumnMapping `json:"column_mapping,omitempty"`

	// Provenance is optional. When SourceType and OriginalPath are supplied,
	// ContentSHA256 is derived from Content if the caller did not provide it.
	SourceType    string `json:"source_type,omitempty"`
	OriginalPath  string `json:"original_path,omitempty"`
	ContentSHA256 string `json:"content_sha256,omitempty"`
	Locale        string `json:"locale,omitempty"`
}

// IngestionResult returns the staged candidate list.
type IngestionResult struct {
	TotalParsed        int                 `json:"total_parsed"`
	AutoCategorized    int                 `json:"auto_categorized"`
	PossibleDuplicates int                 `json:"possible_duplicates"`
	AlreadyImported    bool                `json:"already_imported"`
	ImportID           string              `json:"import_id,omitempty"`
	Transactions       []StagedTransaction `json:"transactions"`
}

type ingestionProvenance struct {
	present      bool
	sourceType   string
	originalPath string
	contentHash  string
}

func resolveIngestionProvenance(req IngestionRequest) (ingestionProvenance, error) {
	sourceType := strings.TrimSpace(req.SourceType)
	originalPath := strings.TrimSpace(req.OriginalPath)
	contentHash := strings.ToLower(strings.TrimSpace(req.ContentSHA256))

	if sourceType == "" && originalPath == "" && contentHash == "" {
		return ingestionProvenance{}, nil
	}
	if sourceType == "" || originalPath == "" {
		return ingestionProvenance{}, fmt.Errorf("source_type and original_path must be provided together")
	}

	if contentHash == "" {
		sum := sha256.Sum256([]byte(req.Content))
		contentHash = hex.EncodeToString(sum[:])
	} else if len(contentHash) != 64 {
		return ingestionProvenance{}, fmt.Errorf("content_sha256 must be a 64 character SHA-256 hex string")
	} else if _, err := hex.DecodeString(contentHash); err != nil {
		return ingestionProvenance{}, fmt.Errorf("content_sha256 must be a SHA-256 hex string")
	}

	return ingestionProvenance{
		present:      true,
		sourceType:   sourceType,
		originalPath: originalPath,
		contentHash:  contentHash,
	}, nil
}

func manifestIsCommitted(m manifest.Manifest) bool {
	status := strings.ToLower(strings.TrimSpace(m.Status))
	return status == "committed" || status == ""
}

// readByID loads a manifest directly from its deterministic import directory.
// Unlike ReadLatest, this keeps idempotency working after a newer version of
// the same path has been imported.
func readByID(root, sourceType, importID string) (manifest.Manifest, bool, error) {
	if strings.TrimSpace(root) == "" {
		return manifest.Manifest{}, false, fmt.Errorf("import root is required")
	}
	if sourceType == "" || sourceType == "." || sourceType == ".." ||
		filepath.Base(sourceType) != sourceType ||
		strings.ContainsAny(sourceType, `/\`) ||
		strings.Contains(sourceType, ":") {
		return manifest.Manifest{}, false, fmt.Errorf("source_type must be a single path segment")
	}
	if len(importID) != 64 {
		return manifest.Manifest{}, false, fmt.Errorf("import_id must be a 64 character SHA-256 hex string")
	}

	data, err := os.ReadFile(filepath.Join(root, "imports", sourceType, importID, "manifest.json"))
	if os.IsNotExist(err) {
		return manifest.Manifest{}, false, nil
	}
	if err != nil {
		return manifest.Manifest{}, false, err
	}

	var m manifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest.Manifest{}, false, err
	}
	return m, true, nil
}

// Ingest parses input and enriches every transaction with rules and duplicate checks.
func Ingest(db *gorm.DB, req IngestionRequest) (*IngestionResult, error) {
	provenance, err := resolveIngestionProvenance(req)
	if err != nil {
		return nil, err
	}

	importID := ""
	if provenance.present {
		importID = manifest.ImportID(provenance.sourceType, provenance.originalPath, provenance.contentHash)
		existing, found, err := readByID(ingestConfigDir(), provenance.sourceType, importID)
		if err != nil {
			return nil, fmt.Errorf("read import manifest: %w", err)
		}
		if found &&
			strings.EqualFold(existing.ContentSHA256, provenance.contentHash) &&
			existing.Parser.Version == IngestionParserVersion &&
			manifestIsCommitted(existing) {
			return &IngestionResult{
				AlreadyImported: true,
				ImportID:        importID,
				Transactions:    []StagedTransaction{},
			}, nil
		}
	}

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
		parsed, err := parseCSVRows(req.Content, req.Delimiter, req.PresetName, db, req.Locale, req.ColumnMapping)
		if err != nil {
			return nil, err
		}
		candidates = parsed

	default:
		return nil, fmt.Errorf("unsupported format %q", req.Format)
	}

	if len(candidates) == 0 && strings.TrimSpace(req.Content) != "" {
		return nil, fmt.Errorf("no transaction rows detected; check the delimiter (comma, semicolon or tab), locale, and column headers")
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
	seenRows := make(map[string]struct{}, len(candidates))

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
			Balance:  c.Balance,
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
			// Suggest offset account using historical postings, merchant taxonomy, or heuristics.
			suggestedAcc, conf := SuggestOffsetAccount(c, existingAccounts, historyMap)
			if suggestedAcc == "" {
				fallback := "Expenses:Unknown"
				if !c.IsDebit {
					fallback = "Income:Unknown"
				}
				suggestedAcc = ResolveBestMatchingAccount(fallback, existingAccounts, c.IsDebit)
				conf = "fallback"
			}
			if suggestedAcc != "" {
				item.SuggestedAccount = suggestedAcc
				// Pre-select suggestions so the staging table is committable
				// immediately; the user can still override every row.
				item.SelectedAccount = suggestedAcc
				item.Confidence = conf
			}
		}

		// 2. Duplicate Detection Scoring against the ledger.
		dupMatch, score, reason := findBestDuplicateMatch(c, existingPostings)
		dbDuplicate := false
		if score >= 70 {
			item.IsDuplicate = true
			item.DuplicateScore = score
			item.DuplicateReason = reason
			if dupMatch != nil {
				item.DuplicateMatchID = dupMatch.ID
			}
			dbDuplicate = true
			duplicateCount++
		}

		// 3. Duplicate Detection within the uploaded statement itself.
		rowKey := statementRowKey(c, item.SelectedAccount)
		if _, seen := seenRows[rowKey]; seen {
			item.IsDuplicate = true
			if item.DuplicateScore < 100 {
				item.DuplicateScore = 100
			}
			if item.DuplicateReason == "" {
				item.DuplicateReason = "duplicate row within the same statement"
			} else {
				item.DuplicateReason += ", duplicate row within the same statement"
			}
			if !dbDuplicate {
				duplicateCount++
			}
		} else {
			seenRows[rowKey] = struct{}{}
		}

		staged = append(staged, item)
	}

	result := &IngestionResult{
		TotalParsed:        len(staged),
		AutoCategorized:    autoCatCount,
		PossibleDuplicates: duplicateCount,
		ImportID:           importID,
		Transactions:       staged,
	}

	if provenance.present && !config.GetConfig().Readonly {
		importManifest := manifest.Manifest{
			RecordType:    manifest.RecordTypeImportManifest,
			ImportID:      importID,
			SourceType:    provenance.sourceType,
			OriginalPath:  provenance.originalPath,
			ContentSHA256: provenance.contentHash,
			ImportedAt:    time.Now().UTC().Format(time.RFC3339Nano),
			Status:        "staged",
			Parser:        manifest.ParserInfo{Name: IngestionParserName, Version: IngestionParserVersion},
			RowCounts: manifest.RowCounts{
				Read:     len(candidates),
				Accepted: len(staged),
			},
			Errors: []manifest.ImportError{},
		}
		if _, err := manifest.Write(ingestConfigDir(), importManifest, staged); err != nil {
			return nil, fmt.Errorf("write import manifest: %w", err)
		}
	}

	return result, nil
}

func statementRowKey(c import_rule.CandidateTransaction, account string) string {
	payee := normalizeDuplicatePayee(c.Payee)
	if payee == "" {
		payee = normalizeDuplicatePayee(c.Memo)
	}
	direction := "credit"
	if c.IsDebit {
		direction = "debit"
	}
	return strings.Join([]string{
		c.Date,
		payee,
		fmt.Sprintf("%.2f", c.Amount),
		direction,
		strings.ToLower(strings.TrimSpace(account)),
	}, "|")
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

func detectCSVDelimiter(content, requested string) rune {
	// A non-default delimiter chosen by the user is authoritative. The UI
	// defaults to comma, so detect the real delimiter for comma requests.
	if runes := []rune(requested); len(runes) == 1 && runes[0] != ',' {
		return runes[0]
	}

	// Statements often start with metadata lines that contain no delimiter.
	// Look at several non-empty lines and use the delimiter that appears most
	// often on a single line.
	delimiters := []rune{',', ';', '\t', '|'}
	counts := map[rune]int{',': 0, ';': 0, '\t': 0, '|': 0}
	lines := 0
	for line := range strings.SplitSeq(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines++
		for _, delimiter := range delimiters {
			if count := strings.Count(line, string(delimiter)); count > counts[delimiter] {
				counts[delimiter] = count
			}
		}
		if lines >= 20 {
			break
		}
	}

	best := ','
	bestCount := 0
	for _, delimiter := range delimiters {
		if counts[delimiter] > bestCount {
			best = delimiter
			bestCount = counts[delimiter]
		}
	}
	return best
}

func parseCSVRows(content, delimiter, presetName string, db *gorm.DB, locale string, mapping *IngestionColumnMapping) ([]import_rule.CandidateTransaction, error) {
	reader := csv.NewReader(strings.NewReader(content))
	reader.FieldsPerRecord = -1
	reader.Comma = detectCSVDelimiter(content, delimiter)

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("statement must have at least a header and one data row")
	}

	headerIdx, dateIdx, descIdx, debitIdx, creditIdx, amountIdx, drCrIdx, currIdx, fallbackDateIdx, stmtCurrency := detectHeaderRow(records)
	balanceIdx := -1
	merchantIdx := -1
	textIdx := -1

	if mapping != nil {
		if mapping.HeaderRow < 0 || mapping.HeaderRow >= len(records) {
			return nil, fmt.Errorf("column mapping: header row %d is outside the statement", mapping.HeaderRow)
		}
		headerIdx = mapping.HeaderRow
		headerWidth := len(records[headerIdx])
		normalize := func(value int) int {
			if value < 0 {
				return -1
			}
			return value
		}
		dateIdx = normalize(mapping.Date)
		descIdx = normalize(mapping.Description)
		debitIdx = normalize(mapping.Debit)
		creditIdx = normalize(mapping.Credit)
		amountIdx = normalize(mapping.Amount)
		balanceIdx = normalize(mapping.Balance)
		currIdx = normalize(mapping.Currency)
		drCrIdx = normalize(mapping.DrCr)
		fallbackDateIdx = -1
		merchantIdx = descIdx
		textIdx = descIdx

		indexes := []struct {
			name  string
			value int
		}{
			{"date", dateIdx},
			{"description", descIdx},
			{"debit", debitIdx},
			{"credit", creditIdx},
			{"amount", amountIdx},
			{"balance", balanceIdx},
			{"currency", currIdx},
			{"dr_cr", drCrIdx},
		}
		for _, item := range indexes {
			if item.value >= headerWidth {
				return nil, fmt.Errorf("column mapping: %s index %d is outside the header row", item.name, item.value)
			}
		}
		if dateIdx < 0 || (debitIdx < 0 && creditIdx < 0 && amountIdx < 0) {
			return nil, fmt.Errorf("column mapping must include a date column and an amount, debit or credit column")
		}
	} else {
		if headerIdx < 0 || dateIdx < 0 || (debitIdx < 0 && creditIdx < 0 && amountIdx < 0) {
			return nil, fmt.Errorf("could not detect valid transaction table header in statement; open Column Mapping to choose the columns manually")
		}
		balanceIdx = findColumnIndex(records[headerIdx], []string{"balance", "closing balance", "running balance", "saldo", "kontostand", "neuer saldo", "saldo nach buchung"})
		merchantIdx = findColumnIndex(records[headerIdx], []string{"auftraggeber", "empfänger", "empfaenger", "payee", "name", "narration"})
		textIdx = findColumnIndex(records[headerIdx], []string{"buchungstext", "verwendungszweck", "description", "particulars", "details", "memo", "text"})
	}

	// A single amount column with both positive and negative values is a signed
	// column: positive means credit, negative means debit. Without a negative
	// value it is more likely a charges-only statement, where the existing
	// debit default is safer.
	signedAmountColumn := false
	if amountIdx >= 0 {
		for _, row := range records[headerIdx+1:] {
			if amountIdx >= len(row) {
				continue
			}
			rawAmount := strings.TrimSpace(row[amountIdx])
			if strings.HasPrefix(rawAmount, "-") ||
				(strings.HasPrefix(rawAmount, "(") && strings.HasSuffix(rawAmount, ")")) {
				signedAmountColumn = true
				break
			}
		}
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

		cellAt := func(index int) string {
			if index >= 0 && index < len(row) {
				return strings.TrimSpace(row[index])
			}
			return ""
		}

		merchant := cellAt(merchantIdx)
		text := cellAt(textIdx)
		desc := merchant
		if desc == "" {
			desc = text
		}
		if desc == "" {
			desc = cellAt(descIdx)
		}
		memo := text
		if memo == "" {
			memo = desc
		}

		var amt float64
		var isDebit bool
		var rowCurr string

		if currIdx >= 0 && currIdx < len(row) {
			rowCurr = normalizeCurrencyToken(row[currIdx])
		}

		if debitIdx >= 0 && debitIdx < len(row) && strings.TrimSpace(row[debitIdx]) != "" {
			val, _, isCr, parsedCurr := parseAmountFloat(row[debitIdx], locale)
			if val > 0 {
				amt = val
				isDebit = !isCr // Default true for debit column unless explicitly marked Cr
				if rowCurr == "" && parsedCurr != "" {
					rowCurr = parsedCurr
				}
			}
		}

		if amt == 0 && creditIdx >= 0 && creditIdx < len(row) && strings.TrimSpace(row[creditIdx]) != "" {
			val, isDr, _, parsedCurr := parseAmountFloat(row[creditIdx], locale)
			if val > 0 {
				amt = val
				isDebit = isDr // Default false for credit column unless explicitly marked Dr
				if rowCurr == "" && parsedCurr != "" {
					rowCurr = parsedCurr
				}
			}
		}

		if amt == 0 && amountIdx >= 0 && amountIdx < len(row) && strings.TrimSpace(row[amountIdx]) != "" {
			val, d, isCr, parsedCurr := parseAmountFloat(row[amountIdx], locale)
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
					} else if signedAmountColumn {
						isDebit = false
					} else {
						// Fallback: positive amount in a charges-only statement is
						// treated as debit unless a sign indicates otherwise.
						isDebit = true
					}
				}
			}
		}

		if rowCurr == "" {
			rowCurr = stmtCurrency
		}

		var balance *float64
		if balanceIdx >= 0 && balanceIdx < len(row) && strings.TrimSpace(row[balanceIdx]) != "" {
			if parsed, ok := parseLocaleAmount(row[balanceIdx], locale); ok {
				balance = &parsed
			}
		}

		if amt > 0 {
			candidates = append(candidates, import_rule.CandidateTransaction{
				Date:     dateStr,
				Payee:    desc,
				Memo:     memo,
				Amount:   amt,
				Currency: rowCurr,
				IsDebit:  isDebit,
				Balance:  balance,
			})
		}
	}

	return candidates, nil
}

var headerCurrencyRe = regexp.MustCompile(`(?i)\(([A-Z]{3})\)|\b(INR|USD|EUR|GBP|CAD|AUD|SGD|JPY|CHF|AED)\b`)

// detectHeaderRow scans the top rows of a statement (up to 50 rows) to find where the actual transaction table begins.
func detectHeaderRow(records [][]string) (headerIdx, dateIdx, descIdx, debitIdx, creditIdx, amountIdx, drCrIdx, currIdx, fallbackDateIdx int, stmtCurrency string) {
	maxScan := min(len(records), 50)

	bestHeaderIdx := -1
	bestScore := -1
	bDate, bDesc, bDebit, bCredit, bAmount, bDrCr, bCurr, bFallbackDate := -1, -1, -1, -1, -1, -1, -1, -1

	for i := range maxScan {
		row := records[i]
		if len(row) == 0 || allEmpty(row) {
			continue
		}

		dIdx := findColumnIndex(row, []string{"transaction date", "txn date", "value date", "trans date", "date", "datum", "buchungstag", "buchung", "valuta", "valutadatum"})
		deIdx := findColumnIndex(row, []string{"narration", "description", "particulars", "memo", "payee", "details", "name", "beschreibung", "buchungstext", "verwendungszweck", "auftraggeber", "empfänger", "empfaenger", "transaction text", "text"})
		drIdx := findColumnIndex(row, []string{"debit amount", "withdrawal", "debit", "dr", "soll"})
		crIdx := findColumnIndex(row, []string{"credit amount", "deposit", "credit", "cr", "haben"})
		aIdx := findColumnIndex(row, []string{"amount (inr)", "net amount", "trans amount", "amount", "betrag", "umsatz"})
		cIdx := findColumnIndex(row, []string{"currency", "curr", "ccy", "iso currency", "währung", "waehrung"})
		drCr := findDrCrIndicatorIndex(row, aIdx)

		var fbDate int = -1
		if dIdx >= 0 {
			for idx, cell := range row {
				if idx == dIdx {
					continue
				}
				clean := strings.ToLower(strings.TrimSpace(cell))
				if strings.Contains(clean, "date") || strings.Contains(clean, "wertstellung") {
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
	// Respect target priority: more specific names come before generic ones.
	// First pass: exact match (case-insensitive).
	for _, target := range targets {
		for i, h := range header {
			if strings.ToLower(strings.TrimSpace(h)) == target {
				return i
			}
		}
	}
	// Second pass: substring match for targets with length > 2, in priority order.
	for _, target := range targets {
		if len(target) <= 2 {
			continue
		}
		for i, h := range header {
			if strings.Contains(strings.ToLower(strings.TrimSpace(h)), target) {
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

var nonNumericAmountRe = regexp.MustCompile(`[^0-9.,'+-]`)

func parseAmountFloat(val string, locale string) (amount float64, isDebit bool, isCredit bool, currency string) {
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

	for _, code := range []string{"INR", "USD", "EUR", "GBP", "CAD", "AUD", "SGD", "JPY", "CHF", "Rs.", "RS.", "rs.", "Rs", "RS", "rs"} {
		cleaned = strings.ReplaceAll(cleaned, code, "")
	}
	cleaned = strings.ReplaceAll(cleaned, "₹", "")
	cleaned = strings.ReplaceAll(cleaned, "€", "")
	cleaned = strings.ReplaceAll(cleaned, "£", "")
	cleaned = strings.ReplaceAll(cleaned, "$", "")

	parsed, ok := parseLocaleAmount(cleaned, locale)
	if !ok {
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

// parseLocaleAmount parses a signed amount using the decimal separator implied
// by locale. When both separators are present the right-most one wins, which
// handles en-IN grouping (1,23,456.78) and German grouping (1.234,56).
func parseLocaleAmount(raw string, locale string) (float64, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, false
	}

	negative := false
	if strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		negative = true
		text = strings.TrimSpace(text[1 : len(text)-1])
	}

	text = nonNumericAmountRe.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "'", "")
	if text == "" {
		return 0, false
	}

	if strings.HasPrefix(text, "-") {
		negative = !negative
		text = text[1:]
	} else if strings.HasPrefix(text, "+") {
		text = text[1:]
	}
	if text == "" {
		return 0, false
	}

	dotCount := strings.Count(text, ".")
	commaCount := strings.Count(text, ",")
	commaDecimal := localeUsesCommaDecimal(locale)

	switch {
	case dotCount > 0 && commaCount > 0:
		if strings.LastIndex(text, ".") > strings.LastIndex(text, ",") {
			text = strings.ReplaceAll(text, ",", "")
		} else {
			text = strings.ReplaceAll(text, ".", "")
			text = strings.ReplaceAll(text, ",", ".")
		}
	case commaCount > 0:
		if commaDecimal {
			if commaCount > 1 {
				return 0, false
			}
			text = strings.ReplaceAll(text, ",", ".")
		} else {
			text = strings.ReplaceAll(text, ",", "")
		}
	case dotCount > 0:
		if commaDecimal {
			if dotCount > 1 {
				parts := strings.Split(text, ".")
				if !allThreeDigitGroups(parts[1:]) {
					return 0, false
				}
				text = strings.Join(parts, "")
			} else if dotLooksLikeGrouping(text) {
				text = strings.ReplaceAll(text, ".", "")
			}
		} else if dotCount > 1 {
			return 0, false
		}
	}

	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, false
	}
	if negative {
		parsed = -parsed
	}
	return parsed, true
}

func localeUsesCommaDecimal(locale string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(locale), "_", "-"))
	parts := strings.SplitN(normalized, "-", 2)
	language := parts[0]
	region := ""
	if len(parts) > 1 {
		region = parts[1]
	}

	// Swiss German and Swiss Italian use a dot decimal separator.
	if (language == "de" || language == "it") && (region == "ch" || region == "li") {
		return false
	}

	switch language {
	case "de", "fr", "es", "it", "nl", "pt", "da", "nb", "nn", "sv", "fi", "pl", "ru", "tr", "cs", "sk", "hu", "id", "vi", "ro", "hr", "sl", "bg", "el":
		return true
	}
	return false
}

func dotLooksLikeGrouping(text string) bool {
	index := strings.Index(text, ".")
	if index < 0 {
		return false
	}
	left := text[:index]
	right := text[index+1:]
	return len(left) >= 1 && len(left) <= 3 && len(right) == 3 && isDigits(left) && isDigits(right)
}

func allThreeDigitGroups(groups []string) bool {
	for _, group := range groups {
		if len(group) != 3 || !isDigits(group) {
			return false
		}
	}
	return true
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
		"02.01.2006", "2.1.2006", "02.01.06", "2.1.06",
		"02-Jan-06", "02 Jan 06", "2/1/06", "2-1-06", "2-1-2006",
		"2006-01-02 15:04:05", "02/01/2006 15:04:05", "02-01-2006 15:04:05",
		"02.01.2006 15:04:05", "02.01.2006 15:04",
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
