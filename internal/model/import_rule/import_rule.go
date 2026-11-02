package import_rule

import (
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

// MatchType defines how condition fields are evaluated.
type MatchType string

const (
	MatchContains MatchType = "contains"
	MatchExact    MatchType = "exact"
	MatchRegex    MatchType = "regex"
)

// TransactionType defines debit, credit, or any.
type TransactionType string

const (
	TxTypeAny    TransactionType = "any"
	TxTypeDebit  TransactionType = "debit"
	TxTypeCredit TransactionType = "credit"
)

// Rule stores user-defined transaction classification rules.
type Rule struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `gorm:"not null" json:"name"`
	Priority int    `gorm:"default:100;index" json:"priority"`
	Enabled  bool   `gorm:"default:true;index" json:"enabled"`

	// Matching Criteria
	PayeePattern   string          `gorm:"type:text" json:"payee_pattern"`
	PayeeMatchType MatchType       `gorm:"default:'contains'" json:"payee_match_type"`
	MemoPattern    string          `gorm:"type:text" json:"memo_pattern"`
	MemoMatchType  MatchType       `gorm:"default:'contains'" json:"memo_match_type"`
	TxType         TransactionType `gorm:"default:'any'" json:"tx_type"`
	MinAmount      *float64        `json:"min_amount,omitempty"`
	MaxAmount      *float64        `json:"max_amount,omitempty"`

	// Target Action
	TargetAccount string   `gorm:"not null" json:"target_account"`
	Tags          []string `gorm:"serializer:json;type:text" json:"tags"`
	FlagForReview bool     `gorm:"default:false" json:"flag_for_review"`
}

// TableName overrides the table name for GORM.
func (Rule) TableName() string {
	return "import_rules"
}

// CandidateTransaction represents a parsed statement entry to evaluate.
type CandidateTransaction struct {
	Date     string  `json:"date"`
	Payee    string  `json:"payee"`
	Memo     string  `json:"memo"`
	Amount   float64 `json:"amount"` // Absolute amount
	Currency string  `json:"currency,omitempty"`
	IsDebit  bool    `json:"is_debit"`
}

// RuleMatchResult contains the decision from applying a rule.
type RuleMatchResult struct {
	Matched       bool     `json:"matched"`
	RuleID        uint     `json:"rule_id,omitempty"`
	RuleName      string   `json:"rule_name,omitempty"`
	TargetAccount string   `json:"target_account,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	FlagForReview bool     `json:"flag_for_review"`
}

// All retrieves all rules ordered by priority ascending (lower number = higher priority).
func All(db *gorm.DB) ([]Rule, error) {
	var rules []Rule
	err := db.Order("priority asc, id asc").Find(&rules).Error
	return rules, err
}

// ActiveRules retrieves all enabled rules.
func ActiveRules(db *gorm.DB) ([]Rule, error) {
	var rules []Rule
	err := db.Where("enabled = ?", true).Order("priority asc, id asc").Find(&rules).Error
	return rules, err
}

// Save creates or updates a rule.
func Save(db *gorm.DB, rule *Rule) error {
	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if rule.TargetAccount == "" {
		return fmt.Errorf("target account is required")
	}
	if rule.Priority == 0 {
		rule.Priority = 100
	}
	if rule.PayeeMatchType == "" {
		rule.PayeeMatchType = MatchContains
	}
	if rule.MemoMatchType == "" {
		rule.MemoMatchType = MatchContains
	}
	if rule.TxType == "" {
		rule.TxType = TxTypeAny
	}
	if rule.Tags == nil {
		rule.Tags = []string{}
	}

	return db.Save(rule).Error
}

// Delete removes a rule by ID.
func Delete(db *gorm.DB, id uint) error {
	return db.Delete(&Rule{}, id).Error
}

// Evaluate matches a transaction against a single rule.
func (r *Rule) Evaluate(tx CandidateTransaction) bool {
	if !r.Enabled {
		return false
	}

	// Transaction Type check
	if r.TxType == TxTypeDebit && !tx.IsDebit {
		return false
	}
	if r.TxType == TxTypeCredit && tx.IsDebit {
		return false
	}

	// Amount range checks
	if r.MinAmount != nil && tx.Amount < *r.MinAmount {
		return false
	}
	if r.MaxAmount != nil && tx.Amount > *r.MaxAmount {
		return false
	}

	// Payee match check
	if r.PayeePattern != "" {
		if !matchesPattern(tx.Payee, r.PayeePattern, r.PayeeMatchType) {
			return false
		}
	}

	// Memo / Narration match check
	if r.MemoPattern != "" {
		if !matchesPattern(tx.Memo, r.MemoPattern, r.MemoMatchType) {
			return false
		}
	}

	// If neither pattern was specified and neither amount/type matched, avoid matching everything
	if r.PayeePattern == "" && r.MemoPattern == "" && r.MinAmount == nil && r.MaxAmount == nil && r.TxType == TxTypeAny {
		return false
	}

	return true
}

func matchesPattern(text, pattern string, matchType MatchType) bool {
	if pattern == "" {
		return true
	}
	textLower := strings.ToLower(strings.TrimSpace(text))
	patLower := strings.ToLower(strings.TrimSpace(pattern))

	switch matchType {
	case MatchExact:
		return textLower == patLower
	case MatchRegex:
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return false
		}
		return re.MatchString(text)
	case MatchContains:
		fallthrough
	default:
		return strings.Contains(textLower, patLower)
	}
}

// MatchCandidate runs through rules in priority order and returns the first match.
func MatchCandidate(rules []Rule, tx CandidateTransaction) RuleMatchResult {
	for _, r := range rules {
		if r.Evaluate(tx) {
			return RuleMatchResult{
				Matched:       true,
				RuleID:        r.ID,
				RuleName:      r.Name,
				TargetAccount: r.TargetAccount,
				Tags:          r.Tags,
				FlagForReview: r.FlagForReview,
			}
		}
	}
	return RuleMatchResult{Matched: false}
}
