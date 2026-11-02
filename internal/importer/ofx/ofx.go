package ofx

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Transaction represents an extracted OFX statement transaction.
type Transaction struct {
	FITID  string    `json:"fitid"`
	Type   string    `json:"type"` // DEBIT, CREDIT, CHECK, etc.
	Date   time.Time `json:"date"`
	Amount float64   `json:"amount"` // Signed: negative for debit, positive for credit
	Payee  string    `json:"payee"`
	Memo   string    `json:"memo"`
}

// Statement contains parsed OFX metadata and transaction list.
type Statement struct {
	Currency     string        `json:"currency"`
	AccountID    string        `json:"account_id"`
	Transactions []Transaction `json:"transactions"`
}

var (
	tagRegex = regexp.MustCompile(`<([A-Za-z0-9_]+)>([^<\r\n]*)`)
	curDefRe = regexp.MustCompile(`<CURDEF>([^<\r\n]+)`)
	acctIDRe = regexp.MustCompile(`<ACCTID>([^<\r\n]+)`)
)

// Parse extracts transactions and account info from an OFX or QFX content string.
func Parse(content string) (*Statement, error) {
	stmt := &Statement{
		Currency:     "",
		Transactions: []Transaction{},
	}

	if match := curDefRe.FindStringSubmatch(content); len(match) > 1 {
		stmt.Currency = strings.TrimSpace(match[1])
	}
	if match := acctIDRe.FindStringSubmatch(content); len(match) > 1 {
		stmt.AccountID = strings.TrimSpace(match[1])
	}

	// Extract all <STMTTRN> blocks
	matches := extractSGMLBlocks(content)

	for _, block := range matches {
		tx, err := parseTxBlock(block)
		if err == nil {
			stmt.Transactions = append(stmt.Transactions, tx)
		}
	}

	if len(stmt.Transactions) == 0 {
		return nil, fmt.Errorf("no valid transactions found in OFX/QFX content")
	}

	return stmt, nil
}

func extractSGMLBlocks(content string) []string {
	var blocks []string
	parts := strings.Split(content, "<STMTTRN>")
	for i := 1; i < len(parts); i++ {
		endIdx := strings.Index(parts[i], "</STMTTRN>")
		if endIdx != -1 {
			blocks = append(blocks, parts[i][:endIdx])
		} else {
			// SGML without closing tags
			blocks = append(blocks, parts[i])
		}
	}
	return blocks
}

func parseTxBlock(block string) (Transaction, error) {
	tags := make(map[string]string)
	matches := tagRegex.FindAllStringSubmatch(block, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			tags[strings.ToUpper(m[1])] = strings.TrimSpace(m[2])
		}
	}

	trntype := tags["TRNTYPE"]
	dtposted := tags["DTPOSTED"]
	trnamt := tags["TRNAMT"]
	fitid := tags["FITID"]
	name := tags["NAME"]
	memo := tags["MEMO"]

	if dtposted == "" || trnamt == "" {
		return Transaction{}, fmt.Errorf("missing essential fields")
	}

	date, err := parseOFXDate(dtposted)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid date: %w", err)
	}

	amt, err := strconv.ParseFloat(trnamt, 64)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid amount: %w", err)
	}

	payee := name
	if payee == "" {
		payee = memo
	}

	return Transaction{
		FITID:  fitid,
		Type:   trntype,
		Date:   date,
		Amount: amt,
		Payee:  payee,
		Memo:   memo,
	}, nil
}

func parseOFXDate(dt string) (time.Time, error) {
	// OFX dates are formatted YYYYMMDDHHMMSS or YYYYMMDD
	cleaned := strings.TrimSpace(dt)
	// Strip timezone bracket suffix if any, e.g. [0:GMT]
	if idx := strings.Index(cleaned, "["); idx != -1 {
		cleaned = cleaned[:idx]
	}

	if len(cleaned) >= 8 {
		ymd := cleaned[:8]
		t, err := time.Parse("20060102", ymd)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format: %s", dt)
}

// AbsAmount returns absolute transaction value.
func (t Transaction) AbsAmount() float64 {
	return math.Abs(t.Amount)
}

// IsDebit returns true if the transaction represents an outflow.
func (t Transaction) IsDebit() bool {
	return t.Amount < 0 || strings.ToUpper(t.Type) == "DEBIT"
}
