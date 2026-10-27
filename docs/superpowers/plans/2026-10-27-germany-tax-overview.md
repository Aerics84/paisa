# Germany Tax Overview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a German-language annual tax overview page that lists ledger candidates for the German income tax return, grouped by form section, with raw capital income and a CSV export.

**Architecture:** New pure Go report builder over `[]posting.Posting` plus a thin Gin handler; ledger ingestion learns a `tax_category` posting tag; a Svelte page renders the report with a year selector, collapsible sections, a coverage row and a raw Anlage KAP card. Account rules live in config; explicit tags override them.

**Tech Stack:** Go 1.24, Gin, GORM/SQLite, shopspring/decimal, SvelteKit (legacy Svelte syntax), Bulma, PapaParse, Bun tests.

**Spec:** `docs/superpowers/specs/2026-10-27-germany-tax-overview-design.md`

## Global Constraints

- Page content is German. Global navigation, API fields, code identifiers and CSV column headers stay English.
- Visibility is gated by `tax_regime: germany` via `supportsGermanyTaxFeatures()`. Never gate on `locale`.
- No tax-law calculation: no Arbeitnehmer-Pauschbetrag, no zumutbare Belastung, no 20 % credit, no allowance, no partial-deduction math.
- Canonical ledger key is `tax_category` on the expense posting. `TaxRelevant` is diagnostic only and never applied.
- `tax_category: "none"` explicitly excludes a posting.
- Account matching is whole-segment prefix matching; the longest rule wins. `Expenses:Health` matches `Expenses:Health` and `Expenses:Health:*`, never `Expenses:HealthSomething`.
- All money uses `shopspring/decimal`, never float64.
- All decimal amounts in the HTTP API are JSON numbers, never quoted strings. `decimal.MarshalJSONWithoutQuotes = true` is a **process-wide** setting and is set once in `internal/server/json.go`; it therefore applies to every endpoint in the process, not only the tax overview. This is deliberate: the Svelte client already types money as `number`, `paisa.go` already relied on this setting, and moving it to the server package makes every program that imports `internal/server` (CLI serve, desktop, server tests) use the same contract. It does **not** cover a separate program that marshals `decimal.Decimal` without importing `internal/server`; the JSON test proves the server-package contract, not every possible process. The server computes every sum; the frontend must not perform decimal arithmetic on money. JS `number` is IEEE-754, so this is a display contract for EUR amounts at two decimals, not a general decimal type.
- JSON arrays are always non-nil (`[]T{}`), never `null`.
- Anlage KAP values are raw ledger aggregates; never gross up dividends and never label booked taxes as withholding tax.
- The page has no editing actions.
- Preview route must exist at `src/routes/preview/more/tax/overview/+page.svelte`.
- No new dependencies.

## Review Focus

These are the inputs most likely to produce a wrong or misleading tax page. Each has a test in the task that owns the code.

1. **Beancount transaction-level metadata.** A `tax_category` on the transaction must not appear on every expense posting of a split transaction. Task 4 tests posting-only `META`.
2. **Sibling tag with multiple expense postings.** A tag on the bank line in a split transaction must produce a diagnostic, not silently include both expenses. Task 5 tests this.
3. **Cross-year refunds.** A negative posting must reduce the year total, carry a `Negativ` marker, and produce the neutral refund note without moving the amount to another year. Task 5 tests this.
4. **Net dividends without booked taxes.** Dividends must stay as booked and produce a diagnostic when no tax postings exist. Task 6 tests this and Task 8 tests the CSV note.
5. **Account prefix boundary.** `Expenses:HealthSomething` must not match `Expenses:Health`. Task 2 tests the schema pattern; Task 5 tests the matcher.

---

### Task 1: Tax category catalog

**Files:**

- Create: `internal/taxcategory/catalog.go`
- Create: `internal/taxcategory/catalog_test.go`

**Interfaces:**

- Consumes: nothing.
- Produces:
  - `type Section struct { Key string; Label string; Form string }`
  - `type Category struct { Key string; Label string; Section string }`
  - `const CapitalIncomeSection = "capital_income"`
  - `var Sections []Section`
  - `var Categories []Category`
  - `func IsKnownCategory(key string) bool`
  - `func CategoryByKey(key string) (Category, bool)`
  - `func CategoriesForSection(section string) []Category`
  - `func CategoryKeys() []string`

- [ ] **Step 1: Write the failing catalog test**

```go
package taxcategory

import "testing"

func TestCatalogIsCompleteAndUnique(t *testing.T) {
	sectionKeys := map[string]bool{}
	for _, s := range Sections {
		if sectionKeys[s.Key] {
			t.Fatalf("duplicate section %s", s.Key)
		}
		sectionKeys[s.Key] = true
	}

	seen := map[string]bool{}
	for _, c := range Categories {
		if seen[c.Key] {
			t.Fatalf("duplicate category %s", c.Key)
		}
		seen[c.Key] = true
		if !sectionKeys[c.Section] {
			t.Fatalf("category %s points at unknown section %s", c.Key, c.Section)
		}
	}

	if !IsKnownCategory("work_equipment") || !IsKnownCategory("health") {
		t.Fatal("expected core categories to be known")
	}
	if IsKnownCategory("none") || IsKnownCategory("Work") {
		t.Fatal("none and legacy values must not be catalog categories")
	}

	c, ok := CategoryByKey("health")
	if !ok || c.Label != "Krankheitskosten" || c.Section != "extraordinary" {
		t.Fatalf("unexpected health category: %#v", c)
	}
	if len(CategoriesForSection("work_expenses")) != 8 {
		t.Fatalf("unexpected work category count")
	}
	if len(CategoriesForSection(CapitalIncomeSection)) != 0 {
		t.Fatal("capital income is generated, not taggable")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/taxcategory -run TestCatalogIsCompleteAndUnique -v`
Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the catalog**

Create `internal/taxcategory/catalog.go`. Copy the section and category tables verbatim from the spec's “Category catalog (Germany)” section.

Section order: `work_expenses`, `insurance`, `extraordinary`, `household`, `donations`, `capital_income`.
Category order within each section as listed in the spec. Include the German `Label` and the form reference (`Anlage N`, `Anlage Vorsorgeaufwand`, `Hauptvordruck`, `Anlage KAP`).

Implement `CategoriesForSection` by filtering `Categories` in slice order so the API order is stable. `CategoryKeys` returns keys in catalog order.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/taxcategory -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/taxcategory
git commit -m "feat(tax): add Germany tax category catalog"
```

---

### Task 2: Tax deduction configuration

**Files:**

- Modify: `internal/config/config.go` (types, `Config` field, defaults, accessor)
- Modify: `internal/config/schema.json` (new `tax_deductions` object)
- Modify: `internal/config/config_test.go`
- Modify: `src/app.d.ts` (`UserConfig.tax_deductions`)

**Interfaces:**

- Consumes: `taxcategory.CategoryKeys()`.
- Produces:
  - `type TaxDeductionAccount struct { Account string; Category string }`
  - `type TaxDeductionsConfig struct { Accounts *[]TaxDeductionAccount }`
  - `Config.TaxDeductions *TaxDeductionsConfig` with `yaml:"tax_deductions,omitempty"`
  - `func TaxDeductionAccounts() []TaxDeductionAccount`

- [ ] **Step 1: Write the failing config tests**

Add to `internal/config/config_test.go`:

```go
func TestTaxDeductionAccountsDefaultsWhenUnset(t *testing.T) {
	require.NoError(t, LoadConfig([]byte("journal_path: test.ledger\ndb_path: test.db\ntax_regime: germany\n"), ""))
	rules := TaxDeductionAccounts()
	require.Len(t, rules, 4)
	assert.Equal(t, "Expenses:Health:Insurance", rules[0].Account)
	assert.Equal(t, "health_insurance", rules[0].Category)
	assert.Equal(t, "Expenses:Health", rules[1].Account)
	assert.Equal(t, "health", rules[1].Category)
	assert.Equal(t, "Expenses:Insurance", rules[2].Account)
	assert.Equal(t, "other_insurance", rules[2].Category)
	assert.Equal(t, "Expenses:Tax:Preparation", rules[3].Account)
	assert.Equal(t, "tax_advice", rules[3].Category)
}

func TestTaxDeductionAccountsExplicitEmptyDisables(t *testing.T) {
	require.NoError(t, LoadConfig([]byte("journal_path: test.ledger\ndb_path: test.db\ntax_deductions:\n  accounts: []\n"), ""))
	assert.Empty(t, TaxDeductionAccounts())
}

func TestTaxDeductionAccountsReturnsCopy(t *testing.T) {
	require.NoError(t, LoadConfig([]byte("journal_path: test.ledger\ndb_path: test.db\ntax_regime: germany\n"), ""))
	first := TaxDeductionAccounts()
	first[0].Account = "Expenses:Mutated"
	second := TaxDeductionAccounts()
	assert.Equal(t, "Expenses:Health:Insurance", second[0].Account)
}

func TestTaxDeductionAccountsExplicitListReplacesDefaults(t *testing.T) {
	require.NoError(t, LoadConfig([]byte(`
journal_path: test.ledger
db_path: test.db
tax_deductions:
  accounts:
    - account: Expenses:Health:Insurance
      category: health_insurance
`), ""))
	rules := TaxDeductionAccounts()
	require.Len(t, rules, 1)
	assert.Equal(t, "Expenses:Health:Insurance", rules[0].Account)
}

func TestTaxDeductionAccountsRejectsUnknownCategory(t *testing.T) {
	err := LoadConfig([]byte(`
journal_path: test.ledger
db_path: test.db
tax_deductions:
  accounts:
    - account: Expenses:Health
      category: made_up
`), "")
	require.Error(t, err)
}

func TestInvalidTaxDeductionsLeavesPreviousConfigUntouched(t *testing.T) {
	require.NoError(t, LoadConfig([]byte(`
journal_path: test.ledger
db_path: test.db
tax_deductions:
  accounts:
    - account: Expenses:Health
      category: health
`), ""))
	before := TaxDeductionAccounts()

	err := LoadConfig([]byte(`
journal_path: test.ledger
db_path: test.db
tax_deductions:
  accounts:
    - account: Expenses:Health
      category: made_up
`), "")
	require.Error(t, err)
	assert.Equal(t, before, TaxDeductionAccounts(), "invalid config must not silently fall back to defaults")
}

func TestTaxDeductionAccountsRejectsWildcardOrNonExpenseAccount(t *testing.T) {
	for _, account := range []string{"Expenses:Health:*", "Assets:Checking", "Expenses"} {
		err := LoadConfig([]byte(fmt.Sprintf(`
journal_path: test.ledger
db_path: test.db
tax_deductions:
  accounts:
    - account: %s
      category: health
`, account)), "")
		assert.Error(t, err, account)
	}
}

func TestTaxDeductionSchemaCategoriesMatchCatalog(t *testing.T) {
	var root map[string]any
	require.NoError(t, json.Unmarshal([]byte(SchemaJson), &root))
	props := root["properties"].(map[string]any)
	td := props["tax_deductions"].(map[string]any)
	accounts := td["properties"].(map[string]any)["accounts"].(map[string]any)
	items := accounts["items"].(map[string]any)
	cat := items["properties"].(map[string]any)["category"].(map[string]any)
	var got []string
	for _, v := range cat["enum"].([]any) {
		got = append(got, v.(string))
	}
	assert.ElementsMatch(t, taxcategory.CategoryKeys(), got)
}

func TestSaveConfigObjectRoundTripsUnsetTaxDeductions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paisa.yaml")
	require.NoError(t, os.WriteFile(path, []byte("journal_path: test.ledger\ndb_path: test.db\n"), 0644))
	LoadConfigFile(path)
	require.Nil(t, GetConfig().TaxDeductions)
	require.NoError(t, SaveConfigObject(GetConfig()))
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "tax_deductions")
}
```

Add imports for `encoding/json`, `fmt`, `taxcategory`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config -run 'TestTaxDeduction|TestSaveConfigObjectRoundTripsUnsetTaxDeductions' -v`
Expected: FAIL because the types and schema entry are missing.

- [ ] **Step 3: Implement the config**

Validation location is not optional: `LoadConfig` already validates the raw YAML
against the embedded `schema.json` before assigning the global `config`
(`internal/config/config.go`, the `schema.Validate(configJson)` call). That path
is used by manual YAML loading, `LoadConfigFile`, and `SaveConfigObject`, so the
schema is enforced server-side, not only by the configuration UI. Do not add a
second Go validation layer; extend the schema and test through `LoadConfig`.

In `internal/config/config.go`:

```go
type TaxDeductionAccount struct {
	Account  string `json:"account" yaml:"account"`
	Category string `json:"category" yaml:"category"`
}

type TaxDeductionsConfig struct {
	Accounts *[]TaxDeductionAccount `json:"accounts,omitempty" yaml:"accounts,omitempty"`
}
```

Add to `Config` near `GermanyTax`:

```go
TaxDeductions *TaxDeductionsConfig `json:"tax_deductions,omitempty" yaml:"tax_deductions,omitempty"`
```

Add the default list and accessor:

```go
var defaultTaxDeductionAccounts = []TaxDeductionAccount{
	{Account: "Expenses:Health:Insurance", Category: "health_insurance"},
	{Account: "Expenses:Health", Category: "health"},
	{Account: "Expenses:Insurance", Category: "other_insurance"},
	{Account: "Expenses:Tax:Preparation", Category: "tax_advice"},
}

func TaxDeductionAccounts() []TaxDeductionAccount {
	rules := defaultTaxDeductionAccounts
	if config.TaxDeductions != nil && config.TaxDeductions.Accounts != nil {
		rules = *config.TaxDeductions.Accounts
	}
	return append([]TaxDeductionAccount(nil), rules...)
}
```

Do not add the defaults to `defaultConfig`; the nil pointer is how “unset” is represented. This mirrors `SalaryIncomeAccounts`.

In `internal/config/schema.json`, add a `tax_deductions` property before `schedule_al`:

```json
"tax_deductions": {
  "type": "object",
  "description": "Account rules for the Germany tax overview. When unset, built-in rules for health insurance, health costs, other insurance and tax advice are used. An empty accounts list disables account matching; an explicit list replaces the defaults. Matching is done on whole account segments; the longest prefix wins.",
  "properties": {
    "accounts": {
      "type": "array",
      "items": {
        "type": "object",
        "ui:header": "account",
        "properties": {
          "account": {
            "type": "string",
            "pattern": "^Expenses:[^:*]+(:[^:*]+)*$",
            "description": "Expense account prefix, for example Expenses:Health. Wildcards are not allowed."
          },
          "category": {
            "type": "string",
            "enum": ["work_equipment", "work_training", "work_travel", "work_commute", "work_home_office", "work_phone_internet", "tax_advice", "work_other", "health_insurance", "other_insurance", "health", "household_service", "craftsman", "donation"],
            "description": "Tax category from the Germany catalog."
          }
        },
        "required": ["account", "category"],
        "additionalProperties": false
      }
    }
  },
  "additionalProperties": false
}
```

The `enum` must match `taxcategory.CategoryKeys()` exactly; the test above enforces it.

In `src/app.d.ts`, add to `UserConfig`:

```ts
tax_deductions?: {
  accounts?: { account: string; category: string }[];
};
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config -v`
Expected: PASS, including the existing config tests.

- [ ] **Step 5: Commit**

```bash
git add internal/config src/app.d.ts
git commit -m "feat(tax): add configurable tax deduction account rules"
```

---

### Task 3: Posting fields and ledger CLI ingestion

**Files:**

- Modify: `internal/model/posting/posting.go`
- Modify: `internal/ledger/ledger.go`
- Create: `internal/ledger/tax_tag_test.go`
- Modify: `tests/regression.test.ts` (exclude the two new fields from snapshots)

**Interfaces:**

- Consumes: nothing.
- Produces: `posting.Posting.TaxCategory string`, `posting.Posting.TaxCategoryLegacy string`.

- [ ] **Step 1: Write the failing ledger integration test**

Create `internal/ledger/tax_tag_test.go`:

```go
package ledger

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLedgerCLIParsesTaxCategoryAndLegacyTag(t *testing.T) {
	if _, err := exec.LookPath("ledger"); err != nil {
		t.Skip("ledger executable is not installed")
	}

	dir := t.TempDir()
	journal := filepath.Join(dir, "main.ledger")
	require.NoError(t, os.WriteFile(journal, []byte(`commodity EUR
	format 1.000,00 EUR

2025/09/09 * PayPal ; Bürostuhl
    Assets:Checking:ING                      -699,00 EUR
    Expenses:Household:Goods                  699,00 EUR ; tax_category: work_equipment

2025/06/02 * ADAC
    Assets:Checking:ING                       -41,00 EUR ; TaxRelevant: Auto
    Expenses:Car:Memberships                   41,00 EUR
`), 0600))

	require.NoError(t, config.LoadConfig([]byte(`
journal_path: `+journal+`
db_path: `+filepath.Join(dir, "paisa.db")+`
tax_regime: germany
default_currency: EUR
time_zone: Europe/Berlin
`), journal))

	postings, err := (LedgerCLI{}).Parse(journal, nil)
	require.NoError(t, err)

	var chair, chairBank, adacBank, adacExpense *posting.Posting
	for _, p := range postings {
		switch {
		case p.Account == "Expenses:Household:Goods":
			chair = p
		case p.Account == "Assets:Checking:ING" && strings.Contains(p.Payee, "PayPal"):
			chairBank = p
		case p.Account == "Assets:Checking:ING" && p.Payee == "ADAC":
			adacBank = p
		case p.Account == "Expenses:Car:Memberships":
			adacExpense = p
		}
	}
	require.NotNil(t, chair)
	require.NotNil(t, chairBank)
	require.NotNil(t, adacBank)
	require.NotNil(t, adacExpense)

	// Posting-level extraction: the tag stays on the posting that carries it.
	assert.Equal(t, "work_equipment", chair.TaxCategory)
	assert.Empty(t, chair.TaxCategoryLegacy)
	assert.Empty(t, chairBank.TaxCategory)
	assert.Equal(t, "Auto", adacBank.TaxCategoryLegacy)
	assert.Empty(t, adacExpense.TaxCategoryLegacy)
	assert.Empty(t, adacExpense.TaxCategory)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ledger -run TestLedgerCLIParsesTaxCategoryAndLegacyTag -v`
Expected: FAIL because the fields do not exist (or are empty).

- [ ] **Step 3: Implement ingestion**

In `internal/model/posting/posting.go`, add after `TagPeriod`:

```go
TaxCategory       string `json:"tax_category"`
TaxCategoryLegacy string `json:"tax_category_legacy"`
```

`internal/model.AutoMigrate` already migrates `posting.Posting`; the new columns need no migration code. The postings table is rebuilt on journal sync, so existing rows are repopulated.

In `internal/ledger/ledger.go`, extend the ledger CSV constants after `TransactionNote`:

```go
TaxCategory
TaxCategoryLegacy
```

Extend the `--csv-format` string with two trailing columns:

```go
,%(quoted(tag('tax_category'))),%(quoted(tag('TaxRelevant')))
```

Set the fields in the posting literal near line 805:

```go
TaxCategory:       record[TaxCategory],
TaxCategoryLegacy: record[TaxCategoryLegacy],
```

The values are already unquoted by the CSV reader. Keep the current note handling untouched.

Extraction is strictly posting-level: a tag stays on the posting that carries it. Do not copy `tax_category` or `TaxRelevant` onto sibling postings. The report's sibling rule (Task 5) is the only place where a bank-line tag can influence the expense posting, and only when the transaction has exactly one expense posting.

In `tests/regression.test.ts`, add `"tax_category"` and `"tax_category_legacy"` to `excludeKeys` so the existing snapshots do not need to be regenerated. The fields are covered by the dedicated Go tests.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ledger -run TestLedgerCLIParsesTaxCategoryAndLegacyTag -v`
Expected: PASS, or SKIP when `ledger` is not installed.

Run: `go test ./internal/ledger ./internal/model/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/model/posting internal/ledger tests/regression.test.ts
git commit -m "feat(ledger): read tax categories from ledger postings"
```

---

### Task 4: hledger and beancount ingestion

**Files:**

- Modify: `internal/ledger/ledger.go`
- Create: `internal/ledger/tax_tag_hledger_test.go`
- Create: `internal/ledger/tax_tag_beancount_test.go`

**Interfaces:**

- Consumes: `posting.Posting.TaxCategory`, `posting.Posting.TaxCategoryLegacy`.
- Produces: nothing new; fixes tag extraction in both backends.

- [ ] **Step 1: Write the failing hledger tag test**

Create `internal/ledger/tax_tag_hledger_test.go`:

```go
package ledger

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHLedgerReadsTaxCategoryAfterRecurringTag(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte("journal_path: main.ledger\ndefault_currency: EUR\ntime_zone: UTC\n"), ""))
	date := time.Date(2025, 9, 9, 0, 0, 0, 0, time.UTC)
	p := HLedgerPosting{
		Account: "Expenses:Household:Goods",
		Tags: [][]string{
			{"Recurring", "Office"},
			{"tax_category", "work_equipment"},
			{"TaxRelevant", "Work"},
		},
	}
	p.Amount = append(p.Amount, struct {
		Commodity string `json:"acommodity"`
		Quantity  struct {
			Value float64 `json:"floatingPoint"`
		} `json:"aquantity"`
		Price hledgerPrice `json:"aprice"`
		Cost  hledgerPrice `json:"acost"`
	}{Commodity: "EUR"})
	p.Amount[0].Quantity.Value = 699
	tx := HLedgerTransaction{Description: "PayPal | Bürostuhl", Status: "Cleared"}
	tx.TSourcePos = append(tx.TSourcePos,
		struct {
			SourceColumn uint64 `json:"sourceColumn"`
			SourceLine   uint64 `json:"sourceLine"`
			SourceName   string `json:"sourceName"`
		}{SourceName: "main.ledger", SourceLine: 1},
		struct {
			SourceColumn uint64 `json:"sourceColumn"`
			SourceLine   uint64 `json:"sourceLine"`
			SourceName   string `json:"sourceName"`
		}{SourceName: "main.ledger", SourceLine: 3},
	)

	postings, err := buildHLedgerPostings(p, tx, nil, date)
	require.NoError(t, err)
	require.Len(t, postings, 1)
	assert.Equal(t, "work_equipment", postings[0].TaxCategory)
	assert.Equal(t, "Work", postings[0].TaxCategoryLegacy)
}
```

Use the existing anonymous struct shape from `ledger.go`; if the compiler complains, define local helper types matching `HLedgerPosting.Amount` exactly.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ledger -run TestHLedgerReadsTaxCategoryAfterRecurringTag -v`
Expected: FAIL because the `break` bug leaves the second tag unread.

- [ ] **Step 3: Fix hledger extraction**

In `internal/ledger/ledger.go`, add:

```go
func tagValue(tags [][]string, key string) string {
	for _, tag := range tags {
		if len(tag) == 2 && tag[0] == key {
			return tag[1]
		}
	}
	return ""
}
```

Remove the two unconditional `break` statements in `buildHLedgerPostings`. Keep the existing recurring/period/forecast handling. After the loops, add:

```go
taxCategory := tagValue(p.Tags, "tax_category")
taxCategoryLegacy := tagValue(p.Tags, "TaxRelevant")
```

Set both fields in the posting literal.

Do **not** read `t.Tags` for tax categories; transaction-level tax metadata is intentionally ignored.

- [ ] **Step 4: Write the failing beancount test**

Create `internal/ledger/tax_tag_beancount_test.go`:

```go
package ledger

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBeancountReadsPostingMetaOnly(t *testing.T) {
	if _, err := exec.LookPath("bean-query"); err != nil {
		t.Skip("bean-query executable is not installed")
	}

	dir := t.TempDir()
	journal := filepath.Join(dir, "main.beancount")
	require.NoError(t, os.WriteFile(journal, []byte(`
2025-09-09 * "PayPal" "Bürostuhl"
  Assets:Checking:ING       -699.00 EUR
  Expenses:Household:Goods   699.00 EUR
    tax_category: "work_equipment"

2025-10-01 * "Split" "Two expenses"
  tax_category: "health"
  Assets:Checking:ING       -100.00 EUR
  Expenses:Health             60.00 EUR
  Expenses:Health:Medicine    40.00 EUR
`), 0600))

	require.NoError(t, config.LoadConfig([]byte(`
journal_path: `+journal+`
db_path: `+filepath.Join(dir, "paisa.db")+`
ledger_cli: beancount
tax_regime: germany
default_currency: EUR
time_zone: Europe/Berlin
`), journal))

	postings, err := (Beancount{}).Parse(journal, nil)
	require.NoError(t, err)

	for _, p := range postings {
		if p.Account == "Expenses:Household:Goods" {
			assert.Equal(t, "work_equipment", p.TaxCategory)
		}
		if p.Account == "Expenses:Health" || p.Account == "Expenses:Health:Medicine" {
			assert.Empty(t, p.TaxCategory, "transaction metadata must not leak onto postings")
		}
	}
}
```

- [ ] **Step 5: Implement beancount metadata**

In the `bean-query` select in `Beancount.Parse`, append `,META('tax_category')`. Add `TaxCategory` to the iota list after `TagPeriod`, and set:

```go
TaxCategory: utils.UnQuote(strings.TrimSpace(record[TaxCategory])),
```

Use `META`, not `ANY_META`. Do not add legacy support for beancount: uppercase metadata keys are invalid there.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/ledger -run 'TestHLedgerReadsTaxCategoryAfterRecurringTag|TestBeancountReadsPostingMetaOnly' -v`
Expected: PASS, or SKIP for missing external binaries.

Run: `go test ./internal/ledger -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ledger
git commit -m "feat(ledger): read tax categories from hledger and beancount"
```

---

### Task 5: Tax overview deduction builder

**Files:**

- Create: `internal/server/tax_overview.go`
- Create: `internal/server/tax_overview_test.go`

**Interfaces:**

- Consumes: `posting.Posting`, `config.TaxDeductionAccount`, `config.TaxDeductionAccounts()`, `taxcategory`.
- Produces:
  - The report types and signatures in this block:

```go
type TaxOverview struct {
	Years         []string                  `json:"years"`
	SelectedYear  string                    `json:"selected_year"`
	Summary       TaxOverviewSummary        `json:"summary"`
	Sections      []TaxOverviewSection      `json:"sections"`
	Coverage      TaxOverviewCoverage       `json:"coverage"`
	CapitalIncome TaxOverviewCapitalIncome  `json:"capital_income"`
	Diagnostics   []TaxOverviewDiagnostic   `json:"diagnostics"`
}

type TaxOverviewSummary struct {
	ExpenseCandidates       decimal.Decimal `json:"expense_candidates"`
	MatchedExpensePostings  int             `json:"matched_expense_posting_count"`
	CapitalIncomeBooked     decimal.Decimal `json:"capital_income_booked"`
	CapitalIncomeCalculated decimal.Decimal `json:"capital_income_calculated"`
	DiagnosticCount         int             `json:"diagnostic_count"`
}

type TaxOverviewSection struct {
	Key          string                `json:"key"`
	Label        string                `json:"label"`
	Form         string                `json:"form"`
	Total        decimal.Decimal       `json:"total"`
	PostingCount int                   `json:"posting_count"`
	Categories   []TaxOverviewCategory `json:"categories"`
}

type TaxOverviewCategory struct {
	Key      string               `json:"key"`
	Label    string               `json:"label"`
	Total    decimal.Decimal      `json:"total"`
	Postings []TaxOverviewPosting `json:"postings"`
}

type TaxOverviewPosting struct {
	TransactionID        string          `json:"transaction_id"`
	Date                 time.Time       `json:"date"`
	Payee                string          `json:"payee"`
	Account              string          `json:"account"`
	Amount               decimal.Decimal `json:"amount"`
	Note                 string          `json:"note"`
	FileName             string          `json:"file_name"`
	TransactionBeginLine uint64          `json:"transaction_begin_line"`
	Source               string          `json:"source"`
	Category             string          `json:"category"`
}

type TaxOverviewCoverage struct {
	AccountsWithUnmatchedPostings int                          `json:"accounts_with_unmatched_postings"`
	UnmatchedPostingCount         int                          `json:"unmatched_posting_count"`
	Accounts                      []TaxOverviewCoverageAccount `json:"accounts"`
}

type TaxOverviewCoverageAccount struct {
	Account               string `json:"account"`
	UnmatchedPostingCount int    `json:"unmatched_posting_count"`
}

type TaxOverviewCapitalIncome struct {
	RealizedGains  decimal.Decimal                   `json:"realized_gains"`
	RealizedLosses decimal.Decimal                   `json:"realized_losses"`
	Dividends      decimal.Decimal                   `json:"dividends"`
	Interest       decimal.Decimal                   `json:"interest"`
	TaxesBooked    decimal.Decimal                   `json:"taxes_booked"`
	Sales          int                               `json:"sales"`
	Accounts       []TaxOverviewCapitalIncomeAccount `json:"accounts"`
}

type TaxOverviewCapitalIncomeAccount struct {
	Account      string          `json:"account"`
	RealizedGain decimal.Decimal `json:"realized_gain"`
}

type TaxOverviewDiagnostic struct {
	Level   string `json:"level"`
	Summary string `json:"summary"`
	Details string `json:"details"`
}

// Internal resolution states. These never appear in the API.
type taxResolutionStatus string

const (
	taxMatched    taxResolutionStatus = "matched"
	taxExcluded   taxResolutionStatus = "excluded"
	taxUnresolved taxResolutionStatus = "unresolved"
	taxInvalid    taxResolutionStatus = "invalid"
)

type taxResolution struct {
	Status     taxResolutionStatus
	Category   string
	Source     string
	Diagnostic *TaxOverviewDiagnostic
}

func resolveTaxPosting(p posting.Posting, siblings []posting.Posting, rules []config.TaxDeductionAccount) taxResolution
func TaxOverviewYears(postings []posting.Posting, now time.Time) []string
func BuildTaxOverview(postings []posting.Posting, year string, rules []config.TaxDeductionAccount) TaxOverview
```

- [ ] **Step 1: Write the failing builder tests**

Create `internal/server/tax_overview_test.go` with a helper:

```go
func taxOverviewPosting(txn, date, account, amount, category, legacy string) posting.Posting {
	d, _ := time.Parse("2006-01-02", date)
	return posting.Posting{
		TransactionID:    txn,
		Date:             d,
		Payee:            txn,
		Account:          account,
		Amount:           decimal.RequireFromString(amount),
		TaxCategory:      category,
		TaxCategoryLegacy: legacy,
	}
}
```

Add table-driven tests for:

- **Tag wins over account rule**: `Expenses:Health` with `TaxCategory: "work_equipment"` and rule `Expenses:Health → health` lands in `work_expenses/work_equipment`.
- **`none` excludes**: posting with `TaxCategory: "none"` is not in any section, coverage count is 0, no diagnostic.
- **Unknown category**: diagnostic summary contains `Unbekannte Steuerkategorie`, posting excluded, coverage count is 0.
- **Sibling tag, one expense**: bank posting has `tax_category: "health"`, one expense posting has no tag → included with `Source == "transaction"`.
- **Sibling tag with empty siblings**: one non-expense sibling carries `health`, another has no tag; the expense is included without an ambiguity diagnostic. Empty values are ignored.
- **Sibling `none`**: one non-expense sibling carries `none`, one expense posting exists → excluded, no coverage, no diagnostic.
- **Sibling tag, two expenses**: bank posting tagged, two expense postings → one deduplicated diagnostic containing `nicht genau einen Aufwandsposten`, both excluded, coverage count is 0.
- **Prefix boundary**: `Expenses:HealthSomething` with a rule for `Expenses:Health` is not matched; it appears in coverage.
- **Longest prefix**: rules `Expenses:Health → health` and `Expenses:Health:Insurance → health_insurance`; an insurance posting lands in `insurance/health_insurance`.
- **Legacy ignored**: `TaxCategoryLegacy: "Work"` on an expense posting without an account rule (for example `Expenses:Household:Goods`) is not included, lands in coverage, and produces a diagnostic containing `TaxRelevant`. A legacy tag must not override a configured account rule either; the rule still decides.
- **Negative posting**: amount `-45` is included, reduces the category total, and produces a diagnostic containing `Negative Aufwandsposten`.
- **Coverage**: two untagged postings on `Expenses:Household:Goods` produce `UnmatchedPostingCount == 2`, `AccountsWithUnmatchedPostings == 1`, and no amount field.
- **Zero amount**: a posting with amount 0 and a valid `tax_category` is included, increments `PostingCount` and `MatchedExpensePostings`, adds nothing to `ExpenseCandidates`, and produces a zero-amount diagnostic. It is not coverage.
- **Empty sections**: all five expense sections are present; `capital_income` is not in `Sections`.
- **Summary**: `ExpenseCandidates` equals the sum of section totals; `MatchedExpensePostings` equals the number of included postings.
- **Years**: `TaxOverviewYears` contains all valid posting years and the previous year, sorted ascending. A posting with a zero date or a year outside 1900–2100 is ignored, so `0001` cannot appear in the selector.
- **Resolution statuses**: a table-driven `TestResolveTaxPostingStatuses` asserts `matched`, `excluded` (`none`), `unresolved` (no rule), and `invalid` (unknown category or ambiguous sibling) directly, so the four states cannot collapse into one empty result.
- **Other-year diagnostics**: a 2024 negative posting and a 2024 legacy tag must not produce diagnostics in a 2025 overview. Only 2025 diagnostics and coverage appear.

Use `config.LoadConfig` in tests that need the Germany regime, and pass rules explicitly to `BuildTaxOverview` so tests do not depend on the global config defaults.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/server -run TestBuildTaxOverview -v`
Expected: FAIL because the types and function do not exist.

- [ ] **Step 3: Implement the builder**

Create `internal/server/tax_overview.go`.

Implementation outline:

1. Initialize `TaxOverview` with non-nil slices:
   ```go
   overview := TaxOverview{
       SelectedYear: year,
       Sections:     []TaxOverviewSection{},
       Coverage:     TaxOverviewCoverage{Accounts: []TaxOverviewCoverageAccount{}},
       Diagnostics:  []TaxOverviewDiagnostic{},
       Summary:      TaxOverviewSummary{},
   }
   ```
2. Build sections in `taxcategory.Sections` order, skipping `CapitalIncomeSection`. Copy categories via `CategoriesForSection`. Store a `map[string]int` from section key to index.
3. Group all postings by `TransactionID`. Filter selected-year expenses with `utils.IsSameOrParent(p.Account, "Expenses")` and `p.Date.Format("2006") == year`.
4. Resolve each expense posting through `resolveTaxPosting`, returning exactly one of `taxMatched`, `taxExcluded`, `taxUnresolved` or `taxInvalid`. Never represent “no category” and “explicitly excluded” with the same empty string. Status handling:
   - `taxMatched` → include the posting.
   - `taxExcluded` → exclude it, do not add to coverage, do not diagnose.
   - `taxUnresolved` → add to coverage.
   - `taxInvalid` → add the diagnostic, do not add to coverage and do not include.

   `resolveTaxPosting` must never read `TaxCategoryLegacy`. Legacy tags are
   collected by a separate diagnostic pass and cannot set a category or override
   an account rule.

   Precedence:
   - `TaxCategory == "none"` → excluded (classified).
   - `TaxCategory` known → included, source `tag`.
   - `TaxCategory` unknown → invalid, diagnostic.
   - No own tag: collect **distinct non-empty** `TaxCategory` values from **non-expense** siblings (`!utils.IsSameOrParent(s.Account, "Expenses")`) and count the transaction's expense postings. Empty siblings are ignored; they never create ambiguity.
     - zero sibling values: account rules.
     - one sibling value and exactly one expense posting: apply it with source `transaction`. Apply the same value rules as an own tag: `none` → `taxExcluded`; known → `taxMatched`; unknown → `taxInvalid`.
     - one sibling value and more than one expense posting: `taxInvalid` with the “ambiguous” diagnostic; do not apply.
     - more than one distinct sibling value: `taxInvalid` with the “ambiguous” diagnostic; do not apply.
   - Account rules: `account == rule.Account || strings.HasPrefix(account, rule.Account+":")`; longest `len(rule.Account)` wins.
   - No rule: `taxUnresolved` → coverage.

   This precedence is mandatory: own valid tag → own `none` → own invalid tag →
   non-empty non-expense siblings → exactly one value and one expense posting →
   account rules. A bank tag must never silently classify multiple expenses.

5. Append included postings to the section/category, add amounts and counts.
6. Add one additional diagnostic pass for selected-year transactions whose tagged non-expense postings have **zero** expense postings (for example a capitalized asset). The per-posting resolution above cannot see those because it only iterates expense postings. Deduplicate this diagnostic per transaction. Legacy, negative and zero-amount diagnostics are also limited to the selected year.
7. Build `Coverage.Accounts` from the unresolved postings grouped by account. Sort by `UnmatchedPostingCount` descending, then account name ascending. Set `AccountsWithUnmatchedPostings` to the number of groups and `UnmatchedPostingCount` to the total.
8. Aggregate diagnostics by condition, not per posting:
   - unknown category values: one diagnostic per value, details with count and up to 5 examples.
   - ambiguous sibling tags: one diagnostic, count and up to 5 examples.
   - legacy tags: one diagnostic with count.
   - negative postings: one diagnostic with the exact neutral German text from the spec.
   - zero amounts: one diagnostic. A zero-amount posting with a valid category is still `matched`: include it, count it, add 0 and diagnose it. It is not coverage.
   - coverage is not a diagnostic; it is the `Coverage` object.
9. `TaxOverviewYears`: distinct valid posting years plus `now.AddDate(-1,0,0).Format("2006")`, sorted with `utils.SortedKeys`. Skip `p.Date.IsZero()` and years outside 1900–2100; the database has no non-zero date constraint, so `0001` must not reach the selector.

Use a helper:

```go
func taxPostingExamples(ps []posting.Posting, limit int) string
```

Returning `2025-09-09 PayPal | Bürostuhl (Expenses:Household:Goods); …`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server -run TestBuildTaxOverview -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/tax_overview.go internal/server/tax_overview_test.go
git commit -m "feat(tax): build Germany tax overview report"
```

---

### Task 6: Raw capital income and shared extraction

**Files:**

- Create: `internal/server/tax_overview_capital.go`
- Create: `internal/server/tax_overview_capital_test.go`
- Modify: `internal/server/germany_tax.go` (extract `collectGermanyTaxPostings`)
- Modify: `internal/server/germany_tax_test.go` only if the extraction changes a helper signature

**Interfaces:**

- Consumes: `computeGermanyTaxByYear`, `posting.Posting`, `utils.IsSameOrParent`.
- Produces:
  - `func collectGermanyTaxPostings(db *gorm.DB) (investmentPostings []posting.Posting, incomePostings []posting.Posting, withholdingTaxPostings []posting.Posting)`
  - `func ComputeTaxOverviewCapitalIncome(investmentPostings, incomePostings, withholdingTaxPostings []posting.Posting, year string) (TaxOverviewCapitalIncome, []TaxOverviewDiagnostic)`

- [ ] **Step 1: Write the failing capital-income tests**

Create `internal/server/tax_overview_capital_test.go`:

```go
func TestCapitalIncomeSplitsGainsAndLossesPerPair(t *testing.T) {
	// Buy 10 at 100, sell 2 at 150 (gain 100), sell 2 at 50 (loss 100).
	// Same account, same year. Call with year "2025".
	// Expect gains 100, losses 100, not a net 0.
}

func TestCapitalIncomeIncludesDividendsOnlyYear(t *testing.T) {
	// No investment postings, one Income:Dividend posting of -118.06 in 2025.
	// Call with year "2025"; expect Dividends 118.06, no sales.
}

func TestCapitalIncomeCountsDistinctSalesAndTaxes(t *testing.T) {
	// Two sells in the same transaction count once; Expenses:Broker:Taxes sums.
}

func TestCapitalIncomeCountsEmptyTransactionIDsAsSeparateSales(t *testing.T) {
	// Two matched sell pairs with empty TransactionID -> Sales 2, not 1.
	// Empty IDs must not collapse into a single sale.
}

func TestCapitalIncomeDoesNotGrossUpDividends(t *testing.T) {
	// Dividends 100 and taxes 25 stay 100 and 25; no 125.
}

func TestCapitalIncomeWarnsWhenIncomeHasNoBookedTaxes(t *testing.T) {
	// Dividend postings, no Expenses:Broker:Taxes -> diagnostic contains
	// "Dividendeneingänge ohne gebuchte Steuern".
}

func TestCapitalIncomeIgnoresOtherYears(t *testing.T) {
	// A 2024 dividend and a 2025 dividend must not leak into the 2025 result.
	// With no 2025 tax postings, exactly one 2025 diagnostic is returned;
	// a 2024 "no booked taxes" diagnostic must not be present.
}
```

Use `germanyTaxPosting` from `germany_tax_test.go` for investment postings. Set distinct `TransactionID` values on sell postings so the sales count is testable. For income/withholding, construct postings with negative/positive amounts and account names.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/server -run TestCapitalIncome -v`
Expected: FAIL because the function does not exist.

- [ ] **Step 3: Implement the capital-income aggregation**

Create `internal/server/tax_overview_capital.go`:

1. Initialize `TaxOverviewCapitalIncome{Accounts: []TaxOverviewCapitalIncomeAccount{}}`.
2. Group investment postings by account. For each account call `computeGermanyTaxByYear(account, ps)` and use only `byYear[year]`.
   - Sum positive `pair.RealizedGain` into `RealizedGains`; sum negative `pair.RealizedGain.Neg()` into `RealizedLosses`.
   - Add `TaxOverviewCapitalIncomeAccount{Account: account, RealizedGain: acct.RealizedGain}`.
   - Count distinct non-empty `pair.Sell.TransactionID` values into `Sales`. If a sell's `TransactionID` is empty, count that pair individually instead of merging all empty IDs into one sale. The parsers populate IDs, but the pure helper must not depend on that for a truthful count.
3. Filter income postings to `year`:
   - `Income:Dividend*` → `Dividends += -p.Amount`.
   - `Income:Interest*` → `Interest += -p.Amount`.
   - Track `hasIncome`.
4. Filter withholding postings to `year`: `TaxesBooked += p.Amount`.
5. Add one diagnostic when `hasIncome` and `TaxesBooked.IsZero()`, with the German text from the spec. The text must explicitly say `ohne gebuchte Steuern`; it must not claim that no tax was withheld, and it must not net taxes against dividends. Do not shorten it to “keine Steuern”.

Sort account rows by account name before returning. Do not include another year's income, taxes or diagnostics.

Refactor `GetGermanyTax` to use the extracted `collectGermanyTaxPostings` query helper; keep its output and tests unchanged.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server -run 'TestCapitalIncome|TestComputeGermanyTax|TestBuildGermanyTaxYear' -v`
Expected: PASS.

Run: `go test ./internal/server -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server
git commit -m "feat(tax): add raw capital income to tax overview"
```

---

### Task 7: Tax overview API handler

**Files:**

- Modify: `internal/server/tax_overview.go` (add handler)
- Create: `internal/server/json.go` (decimal JSON contract)
- Modify: `paisa.go` (move the decimal setting to the server package)
- Modify: `internal/server/server.go`
- Create: `internal/server/tax_overview_handler_test.go`

**Interfaces:**

- Consumes: `BuildTaxOverview`, `TaxOverviewYears`, `ComputeTaxOverviewCapitalIncome`, `collectGermanyTaxPostings`, `utils.Now`, `config.SupportsGermanyTaxFeatures`, `config.TaxDeductionAccounts`.
- Produces:
  - `func GetTaxOverview(db *gorm.DB, requestedYear string) (TaxOverview, error)`
  - `func taxOverviewYear(requested string, now time.Time) (string, error)` — empty means previous calendar year; a four-digit year in 1900–2100 is accepted even if it has no postings; anything else is an error.

- [ ] **Step 1: Write the failing handler test**

Create `internal/server/tax_overview_handler_test.go`:

```go
func TestGetTaxOverviewBuildsSelectedYear(t *testing.T) {
	utils.SetNow("2026-10-27")
	t.Cleanup(func() { utils.SetNow(time.Now().Format("2006-01-02")) })
	loadTaxOverviewConfig(t)
	db := openTaxOverviewTestDB(t)
	// Insert postings with distinct TransactionID values:
	// 2025 Expenses:Health 100 (account rule health)
	// 2025 Expenses:Household:Goods 699 with tax_category work_equipment
	// 2025 Expenses:Household:Goods 50 untagged (coverage)
	// 2025 Income:Dividend:VWCE -118.06
	// 2024 Income:Interest -10
	// 2026 Expenses:Health 30 (different year)

	overview, err := GetTaxOverview(db, "2025")
	require.NoError(t, err)

	assert.Equal(t, "2025", overview.SelectedYear)
	assert.Contains(t, overview.Years, "2024")
	assert.Contains(t, overview.Years, "2025")
	assert.Contains(t, overview.Years, "2026")
	assert.True(t, overview.Summary.ExpenseCandidates.Equal(decimal.RequireFromString("799")))
	assert.Equal(t, 2, overview.Summary.MatchedExpensePostings)
	assert.Equal(t, 1, overview.Coverage.UnmatchedPostingCount)
	assert.Equal(t, 1, overview.Summary.DiagnosticCount)
	assert.True(t, overview.Summary.CapitalIncomeBooked.Equal(decimal.RequireFromString("118.06")))
	assert.True(t, overview.Summary.CapitalIncomeCalculated.Equal(decimal.Zero))
}

func TestTaxOverviewJSONUsesNumbersForDecimals(t *testing.T) {
	utils.SetNow("2026-10-27")
	t.Cleanup(func() { utils.SetNow(time.Now().Format("2006-01-02")) })
	loadTaxOverviewConfig(t)
	db := openTaxOverviewTestDB(t)
	require.NoError(t, db.Create(&posting.Posting{
		TransactionID: "health", Date: time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC),
		Account: "Expenses:Health", Amount: decimal.RequireFromString("100"),
	}))

	overview, err := GetTaxOverview(db, "2025")
	require.NoError(t, err)
	body, err := json.Marshal(overview)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"expense_candidates":100`)
	assert.NotContains(t, string(body), `"expense_candidates":"100"`)
	assert.Contains(t, string(body), `"capital_income_calculated":0`)
}

func TestServerDecimalJSONIsNumber(t *testing.T) {
	body, err := json.Marshal(struct {
		Amount decimal.Decimal `json:"amount"`
	}{Amount: decimal.RequireFromString("1.23")})
	require.NoError(t, err)
	assert.JSONEq(t, `{"amount":1.23}`, string(body))
}
```

Add a second test for the default year:

```go
func TestGetTaxOverviewDefaultsToPreviousYear(t *testing.T) {
	utils.SetNow("2026-10-27")
	// No ?year= parameter.
	overview, err := GetTaxOverview(db, "")
	require.NoError(t, err)
	assert.Equal(t, "2025", overview.SelectedYear)
}

func TestTaxOverviewYearValidation(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	for _, requested := range []string{"", "2025", "2019"} {
		got, err := taxOverviewYear(requested, now)
		require.NoError(t, err, requested)
		if requested == "" {
			assert.Equal(t, "2025", got)
		} else {
			assert.Equal(t, requested, got)
		}
	}
	for _, requested := range []string{"abc", "25", "20255", "20x5"} {
		_, err := taxOverviewYear(requested, now)
		assert.Error(t, err, requested)
	}
}

func TestGetTaxOverviewAllowsValidYearWithoutPostings(t *testing.T) {
	loadTaxOverviewConfig(t)
	db := openTaxOverviewTestDB(t)
	overview, err := GetTaxOverview(db, "2019")
	require.NoError(t, err)
	assert.Equal(t, "2019", overview.SelectedYear)
	assert.NotEmpty(t, overview.Sections)
	assert.Equal(t, 0, overview.Coverage.UnmatchedPostingCount)
}

func TestTaxOverviewRouteRejectsInvalidYear(t *testing.T) {
	loadTaxOverviewConfig(t)
	db := openTaxOverviewTestDB(t)
	router := Build(db, false)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tax_overview?year=abc", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

Add a test for the India regime returning an empty overview with no sections? Optional; gate is in the handler.

Use an in-memory SQLite helper modeled on `openInvestmentTestDB`:

```go
func loadTaxOverviewConfig(t *testing.T) {
	t.Helper()
	require.NoError(t, config.LoadConfig([]byte(`
journal_path: test.ledger
db_path: test.db
tax_regime: germany
default_currency: EUR
time_zone: Europe/Berlin
`), ""))
}

func openTaxOverviewTestDB(t *testing.T) *gorm.DB {
	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}))
	return db
}
```

`loadTaxOverviewConfig` should call `config.LoadConfig` with `tax_regime: germany`, `default_currency: EUR`, `time_zone: Europe/Berlin`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server -run TestGetTaxOverview -v`
Expected: FAIL because `GetTaxOverview` does not exist.

- [ ] **Step 3: Implement the handler and the decimal JSON contract**

Create `internal/server/json.go`:

```go
package server

import "github.com/shopspring/decimal"

func init() {
	// The HTTP API emits decimal amounts as JSON numbers. The Svelte client
	// relies on numbers, not quoted strings. This is process-wide and covers
	// every program that imports internal/server (CLI serve, desktop, tests),
	// not decimal marshaling in unrelated packages.
	decimal.MarshalJSONWithoutQuotes = true
}
```

Remove the now-redundant assignment and `decimal` import from `paisa.go`.

In `internal/server/tax_overview.go`:

```go
func taxOverviewYear(requested string, now time.Time) (string, error) {
	if requested == "" {
		return now.AddDate(-1, 0, 0).Format("2006"), nil
	}
	if len(requested) != 4 {
		return "", fmt.Errorf("invalid year: %q", requested)
	}
	value, err := strconv.Atoi(requested)
	if err != nil || value < 1900 || value > 2100 {
		return "", fmt.Errorf("invalid year: %q", requested)
	}
	return requested, nil
}

func GetTaxOverview(db *gorm.DB, requestedYear string) (TaxOverview, error) {
	if !config.SupportsGermanyTaxFeatures() {
		return TaxOverview{
			Years:         []string{},
			Sections:      []TaxOverviewSection{},
			Coverage:      TaxOverviewCoverage{Accounts: []TaxOverviewCoverageAccount{}},
			Diagnostics:   []TaxOverviewDiagnostic{},
			CapitalIncome: TaxOverviewCapitalIncome{Accounts: []TaxOverviewCapitalIncomeAccount{}},
		}, nil
	}

	postings := query.Init(db).All()
	now := utils.Now()
	years := TaxOverviewYears(postings, now)
	year, err := taxOverviewYear(requestedYear, now)
	if err != nil {
		return TaxOverview{}, err
	}

	overview := BuildTaxOverview(postings, year, config.TaxDeductionAccounts())
	overview.Years = years
	overview.SelectedYear = year

	investment, income, withholding := collectGermanyTaxPostings(db)
	capitalIncome, capitalDiagnostics := ComputeTaxOverviewCapitalIncome(investment, income, withholding, year)
	overview.CapitalIncome = capitalIncome
	overview.Diagnostics = append(overview.Diagnostics, capitalDiagnostics...)
	overview.Summary.CapitalIncomeBooked = overview.CapitalIncome.Dividends.
		Add(overview.CapitalIncome.Interest)
	overview.Summary.CapitalIncomeCalculated = overview.CapitalIncome.RealizedGains.
		Sub(overview.CapitalIncome.RealizedLosses)
	overview.Summary.DiagnosticCount = len(overview.Diagnostics)
	return overview, nil
}
```

Register the route in `internal/server/server.go` after `capital_income_tax`:

```go
router.GET("/api/tax_overview", func(c *gin.Context) {
	overview, err := GetTaxOverview(db, c.Query("year"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, overview)
})
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server -run 'TestGetTaxOverview|TestTaxOverviewYear|TestTaxOverviewRoute|TestServerDecimal' -v`
Expected: PASS.

Run: `go test ./internal/server -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server paisa.go
git commit -m "feat(api): expose Germany tax overview endpoint"
```

---

### Task 8: Frontend types, API client and CSV export

**Files:**

- Modify: `src/lib/utils.ts`
- Create: `src/lib/tax_overview_export.ts`
- Create: `src/lib/tax_overview_export.test.ts`

**Interfaces:**

- Consumes: the JSON shape from Task 7.
- Produces:
  - TypeScript interfaces `TaxOverview`, `TaxOverviewSummary`, `TaxOverviewSection`, `TaxOverviewCategory`, `TaxOverviewPosting`, `TaxOverviewCoverage`, `TaxOverviewCoverageAccount`, `TaxOverviewCapitalIncome`, `TaxOverviewCapitalIncomeAccount`, `TaxOverviewDiagnostic`.
  - `export const TAX_OVERVIEW_CSV_COLUMNS: string[]`
  - `export function taxOverviewCSVRows(overview: TaxOverview): Array<Record<string, string | number>>`
  - `export function taxOverviewCSV(overview: TaxOverview): string` (includes the UTF-8 BOM)
  - `export function downloadTaxOverview(overview: TaxOverview): void`

- [ ] **Step 1: Write the failing CSV test**

Create `src/lib/tax_overview_export.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import dayjs from "dayjs";
import {
  TAX_OVERVIEW_CSV_COLUMNS,
  taxOverviewCSV,
  taxOverviewCSVRows
} from "./tax_overview_export";
import type { TaxOverview } from "./utils";

const overview: TaxOverview = {
  years: ["2025"],
  selected_year: "2025",
  summary: {
    expense_candidates: 699,
    matched_expense_posting_count: 1,
    capital_income_booked: 118.06,
    capital_income_calculated: 0,
    diagnostic_count: 1
  },
  sections: [
    {
      key: "work_expenses",
      label: "Werbungskosten",
      form: "Anlage N",
      total: 699,
      posting_count: 1,
      categories: [
        {
          key: "work_equipment",
          label: "Arbeitsmittel",
          total: 699,
          postings: [
            {
              transaction_id: "txn-1",
              date: dayjs("2025-09-09"),
              payee: "PayPal | Bürostuhl",
              account: "Expenses:Household:Goods",
              amount: 699,
              note: "",
              file_name: "main.ledger",
              transaction_begin_line: 10,
              source: "transaction",
              category: "work_equipment"
            }
          ]
        }
      ]
    }
  ],
  coverage: { accounts_with_unmatched_postings: 1, unmatched_posting_count: 2, accounts: [] },
  capital_income: {
    realized_gains: 0,
    realized_losses: 0,
    dividends: 118.06,
    interest: 0,
    taxes_booked: 0,
    sales: 0,
    accounts: []
  },
  diagnostics: []
};

describe("tax overview CSV", () => {
  test("keeps the fixed column order", () => {
    expect(TAX_OVERVIEW_CSV_COLUMNS).toEqual([
      "section",
      "category",
      "date",
      "payee",
      "account",
      "amount",
      "source",
      "note",
      "file",
      "line"
    ]);
  });

  test("starts with a UTF-8 BOM", () => {
    expect(taxOverviewCSV(overview).startsWith("\uFEFF")).toBe(true);
  });

  test("keeps raw capital income note and source", () => {
    const rows = taxOverviewCSVRows(overview);
    expect(rows[0].source).toBe("transaction");
    const dividend = rows.find((r) => r.category === "dividends");
    expect(dividend?.section).toBe("capital_income");
    expect(dividend?.note).toBe("aus dem Ledger, kein Steuererklärungswert");
    expect(dividend?.source).toBe("");
  });

  test("keeps negative amounts negative", () => {
    const negative = taxOverviewCSVRows({
      ...overview,
      sections: [
        {
          ...overview.sections[0],
          categories: [
            {
              ...overview.sections[0].categories[0],
              postings: [{ ...overview.sections[0].categories[0].postings[0], amount: -45 }]
            }
          ]
        }
      ]
    });
    expect(Number(negative[0].amount)).toBe(-45);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bun test --preload ./src/happydom.ts src/lib/tax_overview_export.test.ts`
Expected: FAIL because the module does not exist.

- [ ] **Step 3: Implement the types and export**

In `src/lib/utils.ts`, add the interfaces near `GermanyTaxYear`:

```ts
export interface TaxOverviewPosting {
  transaction_id: string;
  date: dayjs.Dayjs;
  payee: string;
  account: string;
  amount: number;
  note: string;
  file_name: string;
  transaction_begin_line: number;
  source: "tag" | "transaction" | "account";
  category: string;
}

export interface TaxOverviewCategory {
  key: string;
  label: string;
  total: number;
  postings: TaxOverviewPosting[];
}

export interface TaxOverviewSection {
  key: string;
  label: string;
  form: string;
  total: number;
  posting_count: number;
  categories: TaxOverviewCategory[];
}

export interface TaxOverviewSummary {
  expense_candidates: number;
  matched_expense_posting_count: number;
  capital_income_booked: number;
  capital_income_calculated: number;
  diagnostic_count: number;
}

export interface TaxOverviewCoverageAccount {
  account: string;
  unmatched_posting_count: number;
}

export interface TaxOverviewCoverage {
  accounts_with_unmatched_postings: number;
  unmatched_posting_count: number;
  accounts: TaxOverviewCoverageAccount[];
}

export interface TaxOverviewCapitalIncomeAccount {
  account: string;
  realized_gain: number;
}

export interface TaxOverviewCapitalIncome {
  realized_gains: number;
  realized_losses: number;
  dividends: number;
  interest: number;
  taxes_booked: number;
  sales: number;
  accounts: TaxOverviewCapitalIncomeAccount[];
}

export interface TaxOverviewDiagnostic {
  level: string;
  summary: string;
  details: string;
}

export interface TaxOverview {
  years: string[];
  selected_year: string;
  summary: TaxOverviewSummary;
  sections: TaxOverviewSection[];
  coverage: TaxOverviewCoverage;
  capital_income: TaxOverviewCapitalIncome;
  diagnostics: TaxOverviewDiagnostic[];
}
```

Add the overload near `/api/capital_income_tax`:

```ts
export function ajax(route: "/api/tax_overview"): Promise<TaxOverview>;
export function ajax(route: `/api/tax_overview?year=${string}`): Promise<TaxOverview>;
```

Use `number` for decimal fields (the API emits JSON numbers) and `string` for dates; existing `ajax` converts date strings to dayjs, so type the date as `dayjs.Dayjs` as other interfaces do.

Create `src/lib/tax_overview_export.ts`:

```ts
import Papa from "papaparse";
import dayjs from "dayjs";
import { parseApiDate } from "./utils";
import type { TaxOverview } from "./utils";

export const TAX_OVERVIEW_CSV_COLUMNS = [
  "section",
  "category",
  "date",
  "payee",
  "account",
  "amount",
  "source",
  "note",
  "file",
  "line"
];

export function taxOverviewCSVRows(overview: TaxOverview): Array<Record<string, string | number>> {
  const rows: Array<Record<string, string | number>> = [];
  for (const section of overview.sections) {
    for (const category of section.categories) {
      for (const posting of category.postings) {
        rows.push({
          section: section.key,
          category: category.key,
          date: dayjs(posting.date).format("YYYY-MM-DD"),
          payee: posting.payee,
          account: posting.account,
          amount: posting.amount,
          source: posting.source,
          note: posting.note ?? "",
          file: posting.file_name,
          line: posting.transaction_begin_line
        });
      }
    }
  }
  const capital = overview.capital_income;
  const capitalRows: Array<[string, number]> = [
    ["realized_gains", capital.realized_gains],
    ["realized_losses", capital.realized_losses],
    ["dividends", capital.dividends],
    ["interest", capital.interest],
    ["taxes_booked", capital.taxes_booked]
  ];
  for (const [category, amount] of capitalRows) {
    rows.push({
      section: "capital_income",
      category,
      date: "",
      payee: "",
      account: "",
      amount,
      source: "",
      note: "aus dem Ledger, kein Steuererklärungswert",
      file: "",
      line: ""
    });
  }
  return rows;
}

export function taxOverviewCSV(overview: TaxOverview): string {
  return (
    "\uFEFF" + Papa.unparse(taxOverviewCSVRows(overview), { columns: TAX_OVERVIEW_CSV_COLUMNS })
  );
}

export function downloadTaxOverview(overview: TaxOverview) {
  const blob = new Blob([taxOverviewCSV(overview)], { type: "text/csv;charset=utf-8;" });
  const link = document.createElement("a");
  link.href = window.URL.createObjectURL(blob);
  link.download = `paisa-steueruebersicht-${overview.selected_year}.csv`;
  link.click();
}
```

Remove unused imports (`parseApiDate` if not used).

- [ ] **Step 4: Run tests to verify they pass**

Run: `bun test --preload ./src/happydom.ts src/lib/tax_overview_export.test.ts`
Expected: PASS.

Run: `npm run check`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/lib/utils.ts src/lib/tax_overview_export.ts src/lib/tax_overview_export.test.ts
git commit -m "feat(tax): add tax overview API types and CSV export"
```

---

### Task 9: Tax overview UI

**Files:**

- Create: `src/lib/components/TaxOverviewSection.svelte`
- Create: `src/lib/components/TaxOverviewCapitalIncome.svelte`
- Create: `src/lib/components/TaxOverviewPage.svelte`
- Create: `src/routes/(app)/more/tax/overview/+page.svelte`
- Create: `src/routes/preview/more/tax/overview/+page.svelte`
- Modify: `src/lib/components/Navbar.svelte`
- Modify: `src/lib/components/PreviewNavbar.svelte`

**Interfaces:**

- Consumes: `TaxOverview`, `TaxOverviewSection`, `TaxOverviewCapitalIncome`, `ajax`, `formatCurrency`, `downloadTaxOverview`, `parseApiDate`.
- Produces: the page at `/more/tax/overview` and `/preview/more/tax/overview`.

- [ ] **Step 1: Add the navigation entries**

In `Navbar.svelte` inside the `supportsGermanyTaxFeatures` block, push `Overview` before `Capital Income`:

```ts
tax.children.push(
  { label: "Overview", href: "/overview", help: "tax" },
  { label: "Capital Income", href: "/capital_income", help: "tax" }
);
```

In `PreviewNavbar.svelte`, add the same `Overview` entry before `Capital Income`.

- [ ] **Step 2: Implement the section component**

Create `TaxOverviewSection.svelte`:

```svelte
<script lang="ts">
  import { formatCurrency } from "$lib/utils";
  import dayjs from "dayjs";
  import type { TaxOverviewSection } from "$lib/utils";

  export let section: TaxOverviewSection;
  export let expanded: boolean;
  export let onToggle: () => void;

  const sourceLabel: Record<string, string> = {
    tag: "Tag",
    transaction: "Transaktion",
    account: "Konto"
  };
</script>
```

Render:

- A clickable header `button` with `section.label`, `section.form` in grey, `section.posting_count` + "Postings" (or "Keine Buchungen"), the total via `formatCurrency(section.total)`, and a chevron (`fas fa-chevron-down` / `fas fa-chevron-right`).
- If `expanded` and `section.posting_count === 0`, a paragraph "Keine Buchungen".
- If `expanded`, for each category: a subtotal row with `category.label` and `formatCurrency(category.total)`, then a table with headers `Datum`, `Empfänger`, `Konto`, `Betrag`.
  - Date: `dayjs(posting.date).format("DD.MM.YYYY")`.
  - Payee: `posting.payee`, with `posting.note` as a second truncated line when non-empty.
  - Account: `posting.account`.
  - Source badge: `sourceLabel[posting.source] ?? posting.source`.
  - Negative badge when `posting.amount < 0`: `Negativ`.
  - Amount: `formatCurrency(posting.amount)`.
- Use `class:is-active` / Bulma classes consistent with the rest of the app. Use `<button class="..." on:click={onToggle}>` for accessibility.

- [ ] **Step 3: Implement the capital income component**

Create `TaxOverviewCapitalIncome.svelte` with `export let capitalIncome: TaxOverviewCapitalIncome;`. Render a card:

- Header: `Kapitalerträge`, `Anlage KAP`. No mixed total in the header; the values are only shown in their groups.
- Persistent note: `aus dem Ledger, kein Steuererklärungswert` in warning color.
- Group `Berechnet`: `Realisierte Gewinne`, `Realisierte Verluste`.
- Group `Gebucht`: `Dividenden`, `Zinsen`, `Gebuchte Steuern (Art nicht klassifiziert)`.
- Do not render a combined capital-income sum anywhere in this card.
- If `capitalIncome.accounts.length > 0`, a toggle to show account rows: `Konto`, `Realisierter Gewinn`.

- [ ] **Step 4: Implement the page component**

Create `TaxOverviewPage.svelte`:

```svelte
<script lang="ts">
  import { onMount } from "svelte";
  import { ajax, formatCurrency } from "$lib/utils";
  import type { TaxOverview } from "$lib/utils";
  import { downloadTaxOverview } from "$lib/tax_overview_export";
  import TaxOverviewSection from "./TaxOverviewSection.svelte";
  import TaxOverviewCapitalIncome from "./TaxOverviewCapitalIncome.svelte";

  let overview: TaxOverview | null = null;
  let selectedYear = "";
  let expandedSections: Record<string, boolean> = {};
  let coverageExpanded = false;
  let error = "";

  function apply(data: TaxOverview) {
    overview = data;
    selectedYear = data.selected_year;
    expandedSections = Object.fromEntries(data.sections.map((s) => [s.key, s.posting_count > 0]));
  }

  async function load(year?: string) {
    error = "";
    try {
      apply(await ajax(year ? `/api/tax_overview?year=${year}` : "/api/tax_overview"));
    } catch (e) {
      error = "Die Steuerübersicht konnte nicht geladen werden.";
    }
  }

  onMount(() => {
    load();
  });
</script>
```

Render:

- `<section class="section tab-tax-overview">` + container.
- Header row: `<h1 class="title is-4">Steuerübersicht</h1>`, a `<select bind:value={selectedYear} on:change={(e) => load(e.currentTarget.value)}>` with `overview.years` reversed, and button `CSV exportieren` calling `downloadTaxOverview(overview)`.
- Subtitle: `Ledger-Kandidaten, keine bestätigten Absetzbeträge.`
- If `error`, a warning box.
- If `overview`, the five summary tiles: `Erkannte Ausgaben`, `Kapitalerträge (gebucht)`, `Realisierte Gewinne/Verluste`, `Erkannte Aufwandsposten`, `Hinweise` (use `formatCurrency` for the first three; counts for the last two). Booked capital income and realized gains/losses are separate; never render their sum. The third label is deliberately not `Kapitalerträge (berechnet)`, because the value is not a tax calculation.
- Sections: `{#each overview.sections as section}` → `<TaxOverviewSection>` with `expanded={expandedSections[section.key]}`, `onToggle`.
- Coverage row: a button with `Abdeckung: {overview.coverage.unmatched_posting_count} Postings in {overview.coverage.accounts_with_unmatched_postings} Ausgabenkonten sind nicht zugeordnet.` When expanded, a table of `overview.coverage.accounts` with `Konto`, `Postings` and the note `Keine Liste absetzbarer Beträge.`
- `<TaxOverviewCapitalIncome capitalIncome={overview.capital_income} />`.
- Diagnostics: for each `overview.diagnostics`, an `article.message` with class `is-warning` for `level == "warning"` else `is-info`, showing `summary` and `details`.

- [ ] **Step 5: Create the route files**

`src/routes/(app)/more/tax/overview/+page.svelte`:

```svelte
<script lang="ts">
  import TaxOverviewPage from "$lib/components/TaxOverviewPage.svelte";
</script>

<TaxOverviewPage />
```

Copy the same file to `src/routes/preview/more/tax/overview/+page.svelte`.

- [ ] **Step 6: Verify the UI**

Run: `npm run check`
Expected: PASS.

Run: `npm run build`
Expected: PASS.

Manual:

- Start the app against a Germany-regime ledger and open `/more/tax/overview`.
- Confirm the year selector defaults to the previous year.
- Confirm non-empty sections are expanded, empty sections are collapsed and say `Keine Buchungen`.
- Confirm the coverage row expands to account names without amounts.
- Confirm the summary shows `Kapitalerträge (gebucht)` and `Realisierte Gewinne/Verluste` separately, and the KAP card shows `Berechnet` and `Gebucht` with the raw note and `Gebuchte Steuern (Art nicht klassifiziert)`, with no mixed total.
- Confirm the CSV downloads with a BOM and includes the KAP note.

- [ ] **Step 7: Commit**

```bash
git add src/lib/components/TaxOverviewSection.svelte src/lib/components/TaxOverviewCapitalIncome.svelte src/lib/components/TaxOverviewPage.svelte "src/routes/(app)/more/tax/overview" src/routes/preview/more/tax/overview src/lib/components/Navbar.svelte src/lib/components/PreviewNavbar.svelte
git commit -m "feat(ui): add Germany tax overview page"
```

---

### Task 10: CI parser job, documentation and final verification

**Files:**

- Create: `.github/workflows/ledger-parsers.yml`
- Create: `docs/reference/tax/deductions.md`
- Modify: `docs/reference/tax/index.md`
- Modify: `mkdocs.yml`

**Interfaces:**

- Consumes: everything above.
- Produces: a CI job that actually executes the external-parser tests, plus user documentation.

- [ ] **Step 1: Add the dedicated parser CI job**

Create `.github/workflows/ledger-parsers.yml`:

```yaml
name: Ledger Parsers

on:
  - push
  - pull_request

jobs:
  parsers:
    runs-on: ubuntu-22.04
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
      - run: sudo apt-get update && sudo apt-get install -y ledger hledger pipx
      - run: pipx install beancount==2.3.6
      - run: echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - run: ledger --version && hledger --version && bean-query --version
      - run: go test ./internal/ledger -run 'TestLedgerCLIParsesTaxCategoryAndLegacyTag|TestBeancountReadsPostingMetaOnly|TestHLedgerReadsTaxCategoryAfterRecurringTag' -count=1 -v
```

The version step is intentional: a skipped test must not look like a pass.
`ubuntu-22.04` keeps the Python version compatible with beancount 2.3.6; if the
runner or package changes, adjust the base image, not the test assertion.

- [ ] **Step 2: Write the reference page**

Create `docs/reference/tax/deductions.md` covering:

- What the page is and is not (candidates, not confirmed deductions; no tax math).
- Year selection and default previous year.
- Section/category catalog with German labels and form references.
- Tagging syntax for ledger/hledger and beancount, using the Bürostuhl example.
- `tax_category: "none"` exclusion.
- `tax_deductions.accounts` config: omitted = built-in defaults; explicit list replaces; empty list disables; whole-segment longest-prefix matching.
- Coverage row: counts unmatched postings by account, no amounts, not a deduction list.
- Negative postings and the neutral refund note.
- Anlage KAP: raw values, booked versus calculated values kept separate, booked dividends, booked taxes not classified, no gross-up and no combined total.
- CSV export columns and the raw-value note.
- Visibility: `tax_regime: germany`, independent of `locale`.

- [ ] **Step 3: Add it to the docs navigation**

In `mkdocs.yml`, add `reference/tax/deductions.md` under the Tax section. In `docs/reference/tax/index.md`, add Germany deductions to the supported paths.

- [ ] **Step 4: Run the full verification**

Run:

```bash
go test ./...
bun test --preload ./src/happydom.ts src
npm run check
npm run build
```

Expected: all PASS. If `ledger`, `hledger`, or `bean-query` are missing, the parser integration tests skip; note that in the final report.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/ledger-parsers.yml docs/reference/tax/deductions.md docs/reference/tax/index.md mkdocs.yml
git commit -m "docs(tax): document Germany tax overview"
```

---

## Self-Review Notes

- **Spec coverage:** catalog, config, all three ledger backends, deduction resolution, sibling rule, `none`, coverage, negative/refund note, raw KAP, German/English split, gating, UI, CSV, diagnostics, docs and tests each have a task.
- **Type consistency:** `TaxOverview` types are defined once in Task 5 and consumed unchanged in Tasks 6–9. `TaxDeductionAccount` comes from Task 2. `collectGermanyTaxPostings` is introduced in Task 6 and used in Task 7.
- **Contract decisions pinned by tests:** decimal amounts are JSON numbers via the server-package init; invalid `tax_deductions` fails `LoadConfig` and leaves the previous config; booked and realized capital income are separate fields; resolution states are explicit; zero amounts are included and diagnosed; year filtering covers diagnostics; invalid year values return 400 while valid empty years return an empty overview; zero-date postings are ignored; legacy is posting-level only; sibling resolution ignores empty tags and honors `none`; empty sell IDs count individually; a dedicated CI job runs the external parser tests.
- **Known limitation:** the existing capital-income matcher is reused behavior-preserving; mixed-commodity accounts are not fixed here, per the spec.
- **Snapshot impact:** the new Posting fields are excluded from the regression JSON snapshots to avoid requiring external ledger tooling for a shape-only change.
