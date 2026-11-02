package import_rule

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Rule{}))
	return db
}

func float64Ptr(value float64) *float64 {
	return &value
}

func TestRuleEvaluateMatchTypes(t *testing.T) {
	cases := []struct {
		name      string
		matchType MatchType
		pattern   string
		payee     string
		want      bool
	}{
		{"contains matches substring", MatchContains, "uber", "UBER *TRIP 1234", true},
		{"contains is case insensitive", MatchContains, "Coffee", "blue COFFEE house", true},
		{"contains rejects absent substring", MatchContains, "uber", "Lyft Ride", false},
		{"exact matches whole payee", MatchExact, "Whole Foods", "Whole Foods", true},
		{"exact is case insensitive", MatchExact, "whole foods", "WHOLE FOODS", true},
		{"exact rejects partial payee", MatchExact, "Whole Foods", "Whole Foods Market", false},
		{"regex matches alternation", MatchRegex, `AMZN|AMAZON.*PAY`, "AMZN Mktp US", true},
		{"regex matches trailing pattern", MatchRegex, `AMZN|AMAZON.*PAY`, "AMAZON RETAIL PAY", true},
		{"regex is case insensitive", MatchRegex, `amazon`, "AMAZON RETAIL", true},
		{"regex rejects non matching payee", MatchRegex, `AMZN|AMAZON.*PAY`, "WALMART", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := Rule{
				Name:           "match type",
				Enabled:        true,
				PayeePattern:   tc.pattern,
				PayeeMatchType: tc.matchType,
				TxType:         TxTypeAny,
				TargetAccount:  "Expenses:Test",
			}

			assert.Equal(t, tc.want, rule.Evaluate(CandidateTransaction{Payee: tc.payee}))
		})
	}
}

func TestRuleEvaluateTransactionType(t *testing.T) {
	debit := CandidateTransaction{Payee: "Coffee Shop", Amount: 250, IsDebit: true}
	credit := CandidateTransaction{Payee: "Coffee Shop", Amount: 250, IsDebit: false}

	cases := []struct {
		name        string
		txType      TransactionType
		transaction CandidateTransaction
		want        bool
	}{
		{"debit rule matches debit", TxTypeDebit, debit, true},
		{"debit rule rejects credit", TxTypeDebit, credit, false},
		{"credit rule matches credit", TxTypeCredit, credit, true},
		{"credit rule rejects debit", TxTypeCredit, debit, false},
		{"any rule matches debit", TxTypeAny, debit, true},
		{"any rule matches credit", TxTypeAny, credit, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := Rule{
				Name:           "transaction type",
				Enabled:        true,
				PayeePattern:   "Coffee",
				PayeeMatchType: MatchContains,
				TxType:         tc.txType,
				TargetAccount:  "Expenses:Coffee",
			}

			assert.Equal(t, tc.want, rule.Evaluate(tc.transaction))
		})
	}
}

func TestRuleEvaluateAmountRange(t *testing.T) {
	rule := Rule{
		Name:           "amount range",
		Enabled:        true,
		PayeePattern:   "Uber",
		PayeeMatchType: MatchContains,
		TxType:         TxTypeDebit,
		MinAmount:      float64Ptr(500),
		MaxAmount:      float64Ptr(1000),
		TargetAccount:  "Expenses:Transport",
	}

	cases := []struct {
		name   string
		amount float64
		want   bool
	}{
		{"below minimum", 499.99, false},
		{"at minimum", 500, true},
		{"inside range", 750, true},
		{"at maximum", 1000, true},
		{"above maximum", 1000.01, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, rule.Evaluate(CandidateTransaction{
				Payee:   "UBER *TRIP",
				Amount:  tc.amount,
				IsDebit: true,
			}))
		})
	}

	t.Run("only minimum", func(t *testing.T) {
		minOnly := rule
		minOnly.MaxAmount = nil
		assert.False(t, minOnly.Evaluate(CandidateTransaction{Payee: "UBER", Amount: 100, IsDebit: true}))
		assert.True(t, minOnly.Evaluate(CandidateTransaction{Payee: "UBER", Amount: 5000, IsDebit: true}))
	})

	t.Run("only maximum", func(t *testing.T) {
		maxOnly := rule
		maxOnly.MinAmount = nil
		assert.True(t, maxOnly.Evaluate(CandidateTransaction{Payee: "UBER", Amount: 10, IsDebit: true}))
		assert.False(t, maxOnly.Evaluate(CandidateTransaction{Payee: "UBER", Amount: 1001, IsDebit: true}))
	})
}

func TestRuleDoesNotMatchEmptyRule(t *testing.T) {
	rule := Rule{
		Name:          "empty",
		Enabled:       true,
		TxType:        TxTypeAny,
		TargetAccount: "Expenses:Test",
	}

	assert.False(t, rule.Evaluate(CandidateTransaction{Payee: "Anything", Amount: 100, IsDebit: true}))
	assert.False(t, rule.Evaluate(CandidateTransaction{}))
}

func TestRuleInvalidRegexDoesNotMatch(t *testing.T) {
	rule := Rule{
		Name:           "invalid regex",
		Enabled:        true,
		PayeePattern:   `[unclosed`,
		PayeeMatchType: MatchRegex,
		TxType:         TxTypeAny,
		TargetAccount:  "Expenses:Test",
	}

	assert.False(t, rule.Evaluate(CandidateTransaction{Payee: "[unclosed", IsDebit: true}))
}

func TestMatchCandidateUsesPriority(t *testing.T) {
	db := openTestDB(t)

	fallback := &Rule{
		Name:           "fallback",
		Priority:       200,
		Enabled:        true,
		PayeePattern:   "Coffee",
		PayeeMatchType: MatchContains,
		TxType:         TxTypeAny,
		TargetAccount:  "Expenses:Coffee",
		Tags:           []string{"#coffee"},
	}
	specific := &Rule{
		Name:           "specific",
		Priority:       10,
		Enabled:        true,
		PayeePattern:   "Blue Bottle",
		PayeeMatchType: MatchContains,
		TxType:         TxTypeAny,
		TargetAccount:  "Expenses:Coffee:Special",
		Tags:           []string{"#coffee", "#special"},
		FlagForReview:  true,
	}
	require.NoError(t, Save(db, fallback))
	require.NoError(t, Save(db, specific))

	rules, err := ActiveRules(db)
	require.NoError(t, err)
	require.Len(t, rules, 2)
	require.Equal(t, "specific", rules[0].Name)
	require.Equal(t, "fallback", rules[1].Name)

	result := MatchCandidate(rules, CandidateTransaction{
		Payee:   "Blue Bottle Coffee",
		Amount:  350,
		IsDebit: true,
	})
	assert.True(t, result.Matched)
	assert.Equal(t, specific.ID, result.RuleID)
	assert.Equal(t, "specific", result.RuleName)
	assert.Equal(t, "Expenses:Coffee:Special", result.TargetAccount)
	assert.Equal(t, []string{"#coffee", "#special"}, result.Tags)
	assert.True(t, result.FlagForReview)

	result = MatchCandidate(rules, CandidateTransaction{
		Payee:   "Local Coffee",
		Amount:  350,
		IsDebit: true,
	})
	assert.True(t, result.Matched)
	assert.Equal(t, fallback.ID, result.RuleID)
	assert.Equal(t, "Expenses:Coffee", result.TargetAccount)

	result = MatchCandidate(rules, CandidateTransaction{
		Payee:   "Book Store",
		Amount:  350,
		IsDebit: true,
	})
	assert.False(t, result.Matched)
	assert.Zero(t, result.RuleID)
	assert.Empty(t, result.TargetAccount)
}

func TestSaveRuleAppliesDefaults(t *testing.T) {
	db := openTestDB(t)

	rule := &Rule{
		Name:          "defaults",
		TargetAccount: "Expenses:Groceries",
	}
	require.NoError(t, Save(db, rule))

	assert.Equal(t, 100, rule.Priority)
	assert.Equal(t, MatchContains, rule.PayeeMatchType)
	assert.Equal(t, MatchContains, rule.MemoMatchType)
	assert.Equal(t, TxTypeAny, rule.TxType)
	assert.NotNil(t, rule.Tags)
	assert.Empty(t, rule.Tags)

	stored, err := All(db)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, 100, stored[0].Priority)
	assert.Equal(t, MatchContains, stored[0].PayeeMatchType)
	assert.Equal(t, MatchContains, stored[0].MemoMatchType)
	assert.Equal(t, TxTypeAny, stored[0].TxType)

	t.Run("explicit values are preserved", func(t *testing.T) {
		explicit := &Rule{
			Name:           "explicit",
			Priority:       5,
			PayeeMatchType: MatchRegex,
			MemoMatchType:  MatchExact,
			TxType:         TxTypeDebit,
			TargetAccount:  "Expenses:Test",
			Tags:           []string{"#test"},
		}
		require.NoError(t, Save(db, explicit))
		assert.Equal(t, 5, explicit.Priority)
		assert.Equal(t, MatchRegex, explicit.PayeeMatchType)
		assert.Equal(t, MatchExact, explicit.MemoMatchType)
		assert.Equal(t, TxTypeDebit, explicit.TxType)
	})
}

func TestSaveRuleValidatesRequiredFields(t *testing.T) {
	db := openTestDB(t)

	assert.Error(t, Save(db, &Rule{TargetAccount: "Expenses:Test"}))
	assert.Error(t, Save(db, &Rule{Name: "missing target"}))

	all, err := All(db)
	require.NoError(t, err)
	assert.Empty(t, all)
}

func TestRulePersistence(t *testing.T) {
	db := openTestDB(t)

	rule := &Rule{
		Name:           "transport",
		Enabled:        true,
		PayeePattern:   "Uber",
		PayeeMatchType: MatchContains,
		TxType:         TxTypeDebit,
		MinAmount:      float64Ptr(100),
		TargetAccount:  "Expenses:Transport",
		Tags:           []string{"#commute", "#work"},
	}
	require.NoError(t, Save(db, rule))
	require.NotZero(t, rule.ID)

	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM import_rules").Scan(&count).Error)
	assert.Equal(t, int64(1), count)

	all, err := All(db)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, []string{"#commute", "#work"}, all[0].Tags)
	assert.Equal(t, float64(100), *all[0].MinAmount)

	all[0].Priority = 7
	require.NoError(t, Save(db, &all[0]))

	stored, err := All(db)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, 7, stored[0].Priority)

	require.NoError(t, Delete(db, rule.ID))
	all, err = All(db)
	require.NoError(t, err)
	assert.Empty(t, all)
}
