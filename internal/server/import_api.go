package server

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/importer"
	"github.com/ananthakumaran/paisa/internal/importer/manifest"
	"github.com/ananthakumaran/paisa/internal/importer/reconciliation"
	"github.com/ananthakumaran/paisa/internal/ledger"
	"github.com/ananthakumaran/paisa/internal/model/import_rule"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	maxImportRequestBytes = 10 << 20
	maxImportContentBytes = 8 << 20
)

// importSync is a seam so tests can observe that a successful commit triggers sync.
var importSync = func(deps Deps) gin.H {
	return Sync(deps, SyncRequest{Journal: true})
}

// importManifestRoot is a seam so tests can keep manifests inside a temp directory.
var importManifestRoot = func(store *config.Store) string {
	return store.ConfigDir()
}

// validateImportContent validates proposed journal content with the same
// temp-file approach used by the editor. It is a seam because release builds
// embed the ledger binary while development builds use a placeholder.
var validateImportContent = func(store *config.Store, content string) ([]ledger.LedgerFileError, string, error) {
	return validateFile(LedgerFile{Content: content}, store)
}

// matchImportAccount resolves a user supplied account name against the accounts
// already present in the ledger, falling back to the query itself.
func matchImportAccount(db *gorm.DB, query string) string {
	if strings.TrimSpace(query) == "" {
		return ""
	}

	accounts := accounting.AllAccounts(db)
	if len(accounts) == 0 {
		return query
	}

	lowerQuery := strings.ToLower(query)

	// 1. Exact match (case-insensitive)
	for _, acc := range accounts {
		if strings.ToLower(acc) == lowerQuery {
			return acc
		}
	}

	// 2. Contains match
	var matches []string
	for _, acc := range accounts {
		if strings.Contains(strings.ToLower(acc), lowerQuery) {
			matches = append(matches, acc)
		}
	}

	if len(matches) == 1 {
		return matches[0]
	}
	if len(matches) > 1 {
		best := matches[0]
		for _, m := range matches[1:] {
			if len(m) < len(best) {
				best = m
			}
		}
		return best
	}

	// 3. If no matches, treat the query as a new account
	return query
}

// HandleParseStatement parses statement uploads (CSV/OFX/QFX) and applies rules & duplicate scoring.
func HandleParseStatement(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := deps.DB
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportRequestBytes)

		var req importer.IngestionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if len(req.Content) > maxImportContentBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "statement content too large"})
			return
		}

		result, err := importer.Ingest(db, req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if result.AlreadyImported {
			log.WithFields(log.Fields{
				"source_type":   req.SourceType,
				"original_path": req.OriginalPath,
			}).Info("statement already imported")
		}

		c.JSON(http.StatusOK, ParseStatementResponse{IngestionResult: *result})
	}
}

// HandleGetReconciliation returns a read-only reconciliation report built from
// the latest import manifests. It never writes to the import store.
func HandleGetReconciliation(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		report := reconciliation.BuildReconciliationFromManifests(deps.Config.ConfigDir())
		c.JSON(http.StatusOK, ReconciliationResponse{ReconciliationReport: report})
	}
}

// HandleGetRules lists all user classification rules.
func HandleGetRules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rules, err := import_rule.All(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, RulesResponse{Rules: rules})
	}
}

// HandleSaveRule creates or updates an import rule.
func HandleSaveRule(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Config.Get().Readonly {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Readonly mode"})
			return
		}

		var r import_rule.Rule
		if err := c.ShouldBindJSON(&r); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := import_rule.Save(deps.DB, &r); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, SaveRuleResponse{Rule: r, Saved: true})
	}
}

// HandleDeleteRule deletes an import rule by ID.
func HandleDeleteRule(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Config.Get().Readonly {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Readonly mode"})
			return
		}

		idStr := c.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
			return
		}

		if err := import_rule.Delete(deps.DB, uint(id)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, DeleteRuleResponse{Success: true})
	}
}

// CommitImportRequest defines candidate items to commit directly to the ledger.
type CommitImportRequest struct {
	BaseAccount  string                       `json:"base_account"` // The account being statemented, e.g. Assets:Checking:Chase
	Commodity    string                       `json:"commodity"`
	GroupByDate  bool                         `json:"group_by_date"`
	RawContent   string                       `json:"raw_content"`
	ImportID     string                       `json:"import_id,omitempty"`
	SourceType   string                       `json:"source_type,omitempty"`
	Transactions []CommitCandidateTransaction `json:"transactions"`
}

// CommitCandidateTransaction represents an individual entry to append.
type CommitCandidateTransaction struct {
	Date            string   `json:"date" binding:"required"`
	Payee           string   `json:"payee"`
	Memo            string   `json:"memo"`
	Amount          float64  `json:"amount" binding:"required"`
	Currency        string   `json:"currency"`
	IsDebit         bool     `json:"is_debit"`
	SelectedAccount string   `json:"selected_account" binding:"required"`
	Tags            []string `json:"tags"`
}

// HandleCommitImport writes the approved staged transactions to the journal file.
func HandleCommitImport(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := deps.DB
		if deps.Config.Get().Readonly {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Readonly mode"})
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportRequestBytes)

		var req CommitImportRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		trimmedRaw := strings.TrimSpace(req.RawContent)
		if len(req.Transactions) == 0 && trimmedRaw == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no transactions to commit"})
			return
		}

		if len(req.RawContent) > maxImportContentBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "raw content too large"})
			return
		}

		if err := validateCommitRequest(req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		defaultCommodity := req.Commodity
		if defaultCommodity == "" {
			defaultCommodity = deps.Config.Get().DefaultCurrency
		}
		if defaultCommodity == "" {
			defaultCommodity = "INR"
		}

		if strings.TrimSpace(deps.Config.Get().JournalPath) == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "journal path is not configured"})
			return
		}
		targetFile := deps.Config.JournalPath()

		var contentToAppend string
		if trimmedRaw != "" {
			contentToAppend = "\n" + trimmedRaw + "\n"
		} else {
			contentToAppend = formatTransactionsForCommit(db, req, defaultCommodity, deps.Config)
		}

		if err := os.MkdirAll(filepath.Dir(targetFile), 0750); err != nil {
			log.Warn("Failed to create destination directory: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create directory"})
			return
		}

		// Validate the existing journal plus the proposed append before touching
		// the real journal so an invalid statement can never leave it broken.
		// Validating only the fragment breaks beancount, which needs the main
		// journal's open directives to check the proposed transactions.
		existingJournal, err := readJournalForValidation(targetFile)
		if err != nil {
			log.Warn("Failed to read journal file for validation: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read journal file"})
			return
		}

		validationErrors, _, err := validateImportContent(deps.Config, combineJournalContent(existingJournal, contentToAppend))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":  "invalid ledger content",
				"errors": validationErrors,
			})
			return
		}

		f, err := os.OpenFile(targetFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Warn("Failed to open journal file for append: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open journal file"})
			return
		}

		info, err := f.Stat()
		if err != nil {
			_ = f.Close()
			log.Warn("Failed to stat journal file: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stat journal file"})
			return
		}
		preWriteSize := info.Size()

		if _, err := f.WriteString(contentToAppend); err != nil {
			_ = f.Truncate(preWriteSize)
			_ = f.Close()
			log.Warn("Failed to write to journal file: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to append to journal"})
			return
		}

		if err := f.Sync(); err != nil {
			_ = f.Truncate(preWriteSize)
			_ = f.Close()
			log.Warn("Failed to sync journal file: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sync journal"})
			return
		}

		if err := f.Close(); err != nil {
			_ = os.Truncate(targetFile, preWriteSize)
			log.Warn("Failed to close journal file: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to close journal"})
			return
		}

		syncResult := importSync(deps)
		if success, ok := syncResult["success"].(bool); !ok || !success {
			if truncateErr := os.Truncate(targetFile, preWriteSize); truncateErr != nil {
				log.Warn("Failed to rollback journal after sync failure: ", truncateErr)
			}
			message, _ := syncResult["message"].(string)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sync journal: " + message})
			return
		}

		committedCount := len(req.Transactions)
		if committedCount == 0 && trimmedRaw != "" {
			committedCount = 1
		}

		manifestCommitted := markImportManifestCommitted(req, deps.Config)

		c.JSON(http.StatusOK, CommitImportResponse{
			Success:           true,
			CommittedCount:    committedCount,
			TargetFile:        targetFile,
			ManifestCommitted: manifestCommitted,
			Errors:            convertImportValidationErrors(validationErrors),
		})
	}
}

// convertImportValidationErrors keeps a nil error slice nil so a successful
// commit still serializes "errors": null exactly as before the typed response.
func convertImportValidationErrors(errors []ledger.LedgerFileError) []ImportValidationError {
	if errors == nil {
		return nil
	}

	converted := make([]ImportValidationError, 0, len(errors))
	for _, validationError := range errors {
		converted = append(converted, ImportValidationError{
			FileName: validationError.FileName,
			LineFrom: validationError.LineFrom,
			LineTo:   validationError.LineTo,
			Error:    validationError.Error,
			Message:  validationError.Message,
		})
	}
	return converted
}

// markImportManifestCommitted promotes the staged manifest belonging to a
// successful commit to status "committed". A missing or unreadable manifest is
// reported to the caller as a boolean instead of failing an already-applied
// journal append.
func markImportManifestCommitted(req CommitImportRequest, store *config.Store) bool {
	importID := strings.TrimSpace(req.ImportID)
	if importID == "" {
		return false
	}

	root := importManifestRoot(store)
	sourceType := strings.TrimSpace(req.SourceType)
	if sourceType == "" {
		found, err := manifest.FindSourceType(root, importID)
		if err != nil {
			log.Warn("Failed to locate import manifest: ", err)
			return false
		}
		sourceType = found
	}

	if _, err := manifest.MarkCommitted(root, sourceType, importID); err != nil {
		log.Warn("Failed to mark import manifest as committed: ", err)
		return false
	}
	return true
}

func validateCommitRequest(req CommitImportRequest) error {
	if strings.TrimSpace(req.BaseAccount) != "" {
		if err := validateLedgerAccount("base_account", req.BaseAccount); err != nil {
			return err
		}
	} else if len(req.Transactions) > 0 {
		return fmt.Errorf("base_account is required")
	}

	for i, tx := range req.Transactions {
		if _, err := time.Parse("2006-01-02", tx.Date); err != nil {
			return fmt.Errorf("transaction %d: invalid date %q", i, tx.Date)
		}
		if tx.Amount <= 0 || math.IsNaN(tx.Amount) || math.IsInf(tx.Amount, 0) {
			return fmt.Errorf("transaction %d: amount must be greater than zero", i)
		}
		if err := validateLedgerAccount("selected_account", tx.SelectedAccount); err != nil {
			return fmt.Errorf("transaction %d: %w", i, err)
		}
	}

	return nil
}

func validateLedgerAccount(field string, account string) error {
	if strings.TrimSpace(account) == "" {
		return fmt.Errorf("%s is required", field)
	}
	if strings.ContainsAny(account, "\r\n;") {
		return fmt.Errorf("%s contains invalid ledger characters", field)
	}
	for _, r := range account {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s contains invalid control characters", field)
		}
	}
	return nil
}

func readJournalForValidation(path string) (string, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func combineJournalContent(existing string, appendContent string) string {
	if existing == "" {
		return appendContent
	}
	if !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	return existing + appendContent
}

func formatTransactionsForCommit(db *gorm.DB, req CommitImportRequest, defaultCommodity string, store *config.Store) string {
	var sb strings.Builder
	dialect := store.Get().LedgerCli
	// Read the journal once per commit; every transaction reuses this map.
	separators := journalDecimalSeparators(store, dialect)
	baseAcc := matchImportAccount(db, req.BaseAccount)
	if baseAcc == "" {
		baseAcc = req.BaseAccount
	}

	if req.GroupByDate && dialect != "beancount" {
		// Group debit entries by (Date, Currency) while preserving chronological appearance
		type groupKey struct {
			date     string
			currency string
		}
		var debitKeys []groupKey
		debitGroups := make(map[groupKey][]CommitCandidateTransaction)
		var creditTxs []CommitCandidateTransaction

		for _, tx := range req.Transactions {
			curr := strings.TrimSpace(tx.Currency)
			if curr == "" {
				curr = defaultCommodity
			}
			tx.Currency = curr

			if tx.IsDebit {
				k := groupKey{date: tx.Date, currency: curr}
				if _, exists := debitGroups[k]; !exists {
					debitKeys = append(debitKeys, k)
				}
				debitGroups[k] = append(debitGroups[k], tx)
			} else {
				creditTxs = append(creditTxs, tx)
			}
		}

		for _, k := range debitKeys {
			txs := debitGroups[k]
			dateStr := strings.ReplaceAll(k.date, "/", "-")
			if len(txs) == 1 {
				writeSingleLedgerEntry(&sb, db, baseAcc, txs[0], k.currency, dialect, separators)
				continue
			}

			desc := fmt.Sprintf("Consolidated Debits (%d items)", len(txs))
			sb.WriteString(fmt.Sprintf("%s * %q\n", dateStr, desc))
			var total float64
			for _, tx := range txs {
				offsetAcc := matchImportAccount(db, tx.SelectedAccount)
				if offsetAcc == "" {
					offsetAcc = tx.SelectedAccount
				}
				amtStr := formatCommitAmount(tx.Amount, k.currency, separators)
				total += tx.Amount
				note := extractShortNote(tx.Payee, tx.Memo)
				if note != "" {
					sb.WriteString(fmt.Sprintf("  %-40s   %s %s  ; %s\n", offsetAcc, amtStr, k.currency, note))
				} else {
					sb.WriteString(fmt.Sprintf("  %-40s   %s %s\n", offsetAcc, amtStr, k.currency))
				}
			}
			sb.WriteString(fmt.Sprintf("  %-40s  -%s %s\n\n", baseAcc, formatCommitAmount(total, k.currency, separators), k.currency))
		}

		for _, tx := range creditTxs {
			writeSingleLedgerEntry(&sb, db, baseAcc, tx, tx.Currency, dialect, separators)
		}
		return sb.String()
	}

	for _, tx := range req.Transactions {
		curr := strings.TrimSpace(tx.Currency)
		if curr == "" {
			curr = defaultCommodity
		}
		writeSingleLedgerEntry(&sb, db, baseAcc, tx, curr, dialect, separators)
	}

	return sb.String()
}

// extractShortNote cleans raw statement payee/memo to produce a concise note without random bank IDs or hashes.
func extractShortNote(payee, memo string) string {
	raw := strings.TrimSpace(payee)
	if raw == "" {
		raw = strings.TrimSpace(memo)
	}
	if raw == "" {
		return ""
	}
	return importer.CleanPayee(raw)
}

// formatCommitAmount writes amounts using the decimal separator declared by
// the journal's commodity style. German/EU journals typically declare
// `format 1.000,00 EUR`, where a dot is a thousands separator; writing 12.50
// there changes the value to 1250.00. Beancount always uses a dot. When the
// journal has no explicit format, Ledger's standard dot is used: the journal is
// the source of truth, so the UI locale must not override it.
func formatCommitAmount(amount float64, commodity string, separators map[string]string) string {
	formatted := fmt.Sprintf("%.2f", amount)
	if separators[commodity] == "," {
		return strings.Replace(formatted, ".", ",", 1)
	}
	return formatted
}

// journalDecimalSeparators reads the active journal once and maps every
// commodity that declares a `format` directive to its decimal separator. A
// journal can use dots, commas or Indian grouping (`9,99,99,999.00`). Beancount
// always uses dots, so no journal read is needed there.
func journalDecimalSeparators(store *config.Store, dialect string) map[string]string {
	if dialect == "beancount" {
		return nil
	}

	data, err := os.ReadFile(store.JournalPath())
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	separators := make(map[string]string)
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "commodity" {
			continue
		}

		commodity := strings.Trim(fields[1], `"'`)
		separator, ok := formatDirectiveSeparator(lines, i)
		if ok {
			separators[commodity] = separator
		}
	}

	return separators
}

// formatDirectiveSeparator returns the last decimal separator of the commodity
// block that starts at index. It returns false when no unambiguous format is
// found.
func formatDirectiveSeparator(lines []string, index int) (string, bool) {
	for j := index + 1; j < len(lines) && j <= index+5; j++ {
		formatFields := strings.Fields(lines[j])
		if len(formatFields) < 2 || formatFields[0] != "format" {
			continue
		}
		pattern := formatFields[1]
		position := strings.LastIndexAny(pattern, ".,")
		if position < 0 || position == len(pattern)-1 {
			continue
		}
		if !strings.ContainsAny(pattern[position+1:], "0123456789") {
			continue
		}
		return string(pattern[position]), true
	}

	return "", false
}

func writeSingleLedgerEntry(sb *strings.Builder, db *gorm.DB, baseAcc string, tx CommitCandidateTransaction, commodity string, dialect string, separators map[string]string) {
	offsetAcc := matchImportAccount(db, tx.SelectedAccount)
	if offsetAcc == "" {
		offsetAcc = tx.SelectedAccount
	}

	shortNote := extractShortNote(tx.Payee, tx.Memo)
	if shortNote == "" {
		shortNote = strings.TrimSpace(tx.Payee)
	}
	amtStr := formatCommitAmount(tx.Amount, commodity, separators)
	dateStr := strings.ReplaceAll(tx.Date, "/", "-")

	if dialect == "beancount" {
		sb.WriteString(dateStr)
		sb.WriteString(" * ")
		if shortNote != "" {
			sb.WriteString(fmt.Sprintf("%q", shortNote))
		}
		for _, tag := range tx.Tags {
			cleanTag := strings.TrimPrefix(tag, "#")
			sb.WriteString(" #" + cleanTag)
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString(dateStr)
		sb.WriteString(" * ")
		if shortNote != "" {
			sb.WriteString(fmt.Sprintf("%q", shortNote))
		}
		if len(tx.Tags) > 0 {
			sb.WriteString("  ; " + strings.Join(tx.Tags, " "))
		}
		sb.WriteString("\n")
	}

	if tx.IsDebit {
		sb.WriteString(fmt.Sprintf("  %-40s  -%s %s\n", baseAcc, amtStr, commodity))
		sb.WriteString(fmt.Sprintf("  %-40s   %s %s\n\n", offsetAcc, amtStr, commodity))
	} else {
		sb.WriteString(fmt.Sprintf("  %-40s   %s %s\n", baseAcc, amtStr, commodity))
		sb.WriteString(fmt.Sprintf("  %-40s  -%s %s\n\n", offsetAcc, amtStr, commodity))
	}
}
