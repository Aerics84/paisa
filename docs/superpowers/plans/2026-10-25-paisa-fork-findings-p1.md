# Paisa Fork-Findings P1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Die P1-Findings aus dem Fork-Review umsetzen: Korrektheit von Datum/Währung/Kostenbasis, Gehalts- und Transferklassifikation, Yahoo-Zuverlässigkeit, Import-/Template-Robustheit sowie kleine Performance-/Sicherheits-Fixes.

**Architecture:** Ein Branch, viele kleine, unabhängig testbare Commits. Jede Task portiert gezielt einen Fork-Commit oder eine kleine Gruppe zusammengehöriger Commits, ergänzt einen Regressionstest und lässt die betroffenen Pakete laufen. Die Aufgaben sind so geschnitten, dass sie einzeln reviewt und notfalls einzeln zurückgerollt werden können.

**Tech Stack:** Go 1.26, GORM/SQLite, Gin, Svelte 5, Bulma 1, D3 7, TypeScript, Bun-Tests, npm 11, Node 24.

**Spec:** `docs/research/2026-10-25-paisa-forks-review.md`

## Global Constraints

- **Version:** Am Ende von Task 17 `VERSION` auf `8.4` setzen und `npm run sync-version` ausführen. Die daraus folgenden Änderungen an `desktop/wails.json` und `desktop/build/linux/DEBIAN/control` werden mit committet. Vorher keine Versionsänderung.
- **Lizenz:** AGPL-3.0. Beim Portieren `git cherry-pick -x <sha>` verwenden; bei manueller Übernahme die Quelle im Commit-Body als `Source: https://github.com/<fork>/commit/<sha>` vermerken.
- **Keine neuen Abhängigkeiten:** `doublestar/v4` und `bignumber.js` sind bereits vorhanden.
- **Lokal bleiben:** Keine Cloud-/Bank-Sync-Funktionen, keine automatischen Journal-Änderungen, keine LLM-/MCP-Funktionen einführen.
- **UI-Sprache:** Produktoberfläche bleibt Englisch; Referenzdokumentation bleibt Deutsch, wo sie bereits Deutsch ist.
- **Ein Task = ein Commit** (oder eine kleine, logisch zusammenhängende Commit-Serie). Nach jedem Task die genannten Tests ausführen.
- **Bestehende Monatsrückblick-Semantik nicht ändern:** TWR, Kursfrische, „not available“ und das Verbot erfundener Nullen bleiben maßgeblich.
- **Frontend-Tests:** `bun test --preload ./src/happydom.ts <pfad>`.
- **Vor dem Merge:** `go test ./...`, `npm run check`, `npm run test:prepush`, `npm run test:regression`.

## Review Focus

Diese Fehlerklassen sind in der Spec angelegt, aber nicht durch die bestehenden Tests abgedeckt. Jede Zeile wird in der genannten Task getestet:

1. **Tagesgrenzen:** Ein Posting um 00:00 Europe/Berlin darf in Browser-UTC, CSV-Export und Running-Balance nicht auf den Vortag rutschen. → Task 1.
2. **Moderne hledger-Kosten:** `acost` (`UnitCost`/`TotalCost`) muss gegenüber dem alten `aprice` Vorrang haben; sonst ist die Kostenbasis falsch. → Task 2.
3. **Gemischte Konto-Globs:** Positive und negative Muster sowie Präfixe ohne `*` müssen deterministisch filtern; leere Strings dürfen nicht panicken. → Task 3.
4. **Gehalt vs. Transfer:** `Income:SalaryOther` ist kein Gehalt, `Income:Payroll` schon; ein Transfer mit einer Ausgaben-Leg ist kein interner Transfer. → Tasks 4 und 5.
5. **Externe Daten und Leerzustände:** Yahoo liefert `GBp`/`GBX` und gelegentlich 401/429; eine leere Template-Liste darf die Import-Seite nicht crashen. → Tasks 6, 7 und 10.

---

### Task 0: Branch und Baseline

**Files:** keine Änderung.

- [ ] **Step 1: Branch anlegen**

```bash
git switch -c fix/paisa-fork-findings-p1
```

- [ ] **Step 2: Baseline prüfen**

Run:

```bash
go test ./...
npm run check
```

Expected: Beide Befehle laufen durch. Falls nicht, den Baseline-Fehler notieren und **nicht** in diesem Branch fixen.

---

### Task 1: Zeitzonen- und Datums-Korrektheit

**Files:**

- Modify: `internal/utils/utils.go` (`Now`, neue `BeginningOfDay`)
- Modify: `internal/accounting/accounting.go` (`RunningBalance`)
- Modify: `src/lib/utils.ts` (`ajax`, neue `parseApiDate`)
- Modify: `src/lib/export.ts` (`buildExportRows`, `download`)
- Test: `internal/utils/utils_test.go`
- Test: `internal/accounting/accounting_test.go` (neu)
- Test: `src/lib/date.test.ts` (neu)

**Interfaces:**

- Consumes: `config.TimeZone() *time.Location`, `utils.toDate(time.Time) time.Time`.
- Produces: `utils.BeginningOfDay(date time.Time) time.Time`; `parseApiDate(value: string): dayjs.Dayjs`; `buildExportRows(balancedPostings: BalancedPosting[]): Record<string, unknown>[]`.

**Source:** `matthiasdebernardini/paisa@ef719408`, `panuozzo77/paisa@67a2fa8a`.

- [ ] **Step 1: Go-Test für `Now` schreiben**

In `internal/utils/utils_test.go`:

```go
func TestNowUsesConfiguredTimeZone(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte(
		"journal_path: test.ledger\ndb_path: test.db\ntime_zone: Europe/Berlin\n",
	), ""))
	now = time.Time{}
	t.Cleanup(func() { now = time.Time{} })

	assert.Equal(t, "Europe/Berlin", Now().Location().String())
}
```

- [ ] **Step 2: Test rot laufen lassen**

Run: `go test ./internal/utils -run TestNowUsesConfiguredTimeZone -count=1`
Expected: FAIL, weil `Now()` `time.Now()` in der Server-Zeitzone liefert.

- [ ] **Step 3: `Now` und `BeginningOfDay` implementieren**

In `internal/utils/utils.go`:

```go
func Now() time.Time {
	if !now.Equal(time.Time{}) {
		return now
	}
	return time.Now().In(config.TimeZone())
}

func BeginningOfDay(date time.Time) time.Time {
	return toDate(date)
}
```

- [ ] **Step 4: Running-Balance-Test schreiben**

In `internal/accounting/accounting_test.go` (neue Datei, `package accounting`):

```go
func TestRunningBalanceStartsAtConfiguredMidnight(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte(
		"journal_path: test.ledger\ndb_path: test.db\ntime_zone: Europe/Berlin\n",
	), ""))
	utils.SetNow("2026-06-10")

	date := time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
	series := RunningBalance(nil, []posting.Posting{{
		Date: date, Commodity: "EUR", Quantity: decimal.NewFromInt(10),
	}})

	require.NotEmpty(t, series)
	assert.Equal(t, utils.BeginningOfDay(date), series[0].Date)
}
```

- [ ] **Step 5: Test rot laufen lassen**

Run: `go test ./internal/accounting -run TestRunningBalanceStartsAtConfiguredMidnight -count=1`
Expected: FAIL (Location/Stunde weichen ab oder `BeginningOfDay` fehlt).

- [ ] **Step 6: `RunningBalance` anpassen**

Ersetze in `internal/accounting/accounting.go`:

```go
for start := postings[0].Date; start.Before(end); start = start.AddDate(0, 0, 1) {
```

durch:

```go
for start := utils.BeginningOfDay(postings[0].Date); start.Before(end); start = utils.BeginningOfDay(start.AddDate(0, 0, 1)) {
```

- [ ] **Step 7: Frontend-Test schreiben**

In `src/lib/date.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import dayjs from "dayjs";
import { buildExportRows } from "./export";
import { parseApiDate } from "./utils";

describe("journal calendar dates", () => {
  test("parses an API instant as the journal calendar day", () => {
    const date = parseApiDate("2026-06-06T00:00:00+02:00");
    expect(date.format("YYYY-MM-DD")).toBe("2026-06-06");
    expect(date.hour()).toBe(0);
  });

  test("exports the journal calendar day, not the UTC instant", () => {
    const posting = {
      date: dayjs("2026-06-06T00:00:00+02:00"),
      transaction_id: "t1",
      payee: "Test",
      account: "Assets:Bank",
      quantity: 1,
      amount: 1,
      commodity: "EUR"
    };
    const rows = buildExportRows([{ from: posting, to: posting } as any]);
    expect(rows[0].Date).toBe("2026-06-06");
  });
});
```

- [ ] **Step 8: Test rot laufen lassen**

Run: `bun test --preload ./src/happydom.ts src/lib/date.test.ts`
Expected: FAIL (`parseApiDate`/`buildExportRows` fehlen).

- [ ] **Step 9: Frontend implementieren**

In `src/lib/utils.ts`:

```ts
export function parseApiDate(value: string) {
  return dayjs(value.slice(0, 10));
}
```

Im `JSON.parse`-Reviver `return dayjs(value);` durch `return parseApiDate(value);` ersetzen.

In `src/lib/export.ts`:

- Neue Funktion `buildExportRows(balancedPostings: BalancedPosting[]): Record<string, unknown>[]` mit denselben Feldern wie bisher.
- Einzige inhaltliche Änderung: `Date: balancedPosting.from.date.format("YYYY-MM-DD")` statt `toISOString()`.
- `download` ruft `buildExportRows` auf; der Papa.unparse/Blob/Download-Teil bleibt unverändert.

- [ ] **Step 10: Tests grün laufen lassen**

Run:

```bash
go test ./internal/utils ./internal/accounting -run 'TestNowUsesConfiguredTimeZone|TestRunningBalanceStartsAtConfiguredMidnight' -count=1
bun test --preload ./src/happydom.ts src/lib/date.test.ts
```

Expected: PASS.

- [ ] **Step 11: Commit**

```bash
git add internal/utils/utils.go internal/utils/utils_test.go internal/accounting/accounting.go internal/accounting/accounting_test.go src/lib/utils.ts src/lib/export.ts src/lib/date.test.ts
git commit -m "fix: keep journal dates in the configured timezone"
```

---

### Task 2: hledger-`acost` unterstützen

**Files:**

- Modify: `internal/ledger/ledger.go`
- Test: `internal/ledger/ledger_test.go`

**Interfaces:**

- Produces: `hledgerPrice` struct; `HLedgerPosting.Amount[].Cost hledgerPrice` (JSON `acost`).
- Konsumiert von: `buildHLedgerPostings`.

**Source:** `nightconcept/paisa@40743ba1` – **nur** `internal/ledger/ledger.go` und `internal/ledger/ledger_test.go` übernehmen, nicht die Workflow-Änderungen.

- [ ] **Step 1: Test schreiben**

In `internal/ledger/ledger_test.go`:

```go
func TestBuildHLedgerPostingsPrefersCostOverPrice(t *testing.T) {
	date := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	pricesTree := buildPricesTree([]price.Price{{
		Date: date, CommodityName: "USD", Value: decimal.NewFromInt(1),
	}})
	var transaction HLedgerTransaction
	require.NoError(t, json.Unmarshal([]byte(`{
		"tdescription": "401(k)", "tindex": 1, "tstatus": "Cleared",
		"tsourcepos": [{"sourceLine": 1}, {"sourceLine": 2}]
	}`), &transaction))

	for name, amount := range map[string]string{
		"legacy total price": `{"paccount":"Assets:401k","pamount":[{"acommodity":"401K","aquantity":{"floatingPoint":2},"aprice":{"tag":"TotalPrice","contents":{"acommodity":"USD","aquantity":{"floatingPoint":100}}}}]}`,
		"modern total cost":  `{"paccount":"Assets:401k","pamount":[{"acommodity":"401K","aquantity":{"floatingPoint":2},"acost":{"tag":"TotalCost","contents":{"acommodity":"USD","aquantity":{"floatingPoint":100}}}}]}`,
		"modern unit cost":   `{"paccount":"Assets:401k","pamount":[{"acommodity":"401K","aquantity":{"floatingPoint":2},"acost":{"tag":"UnitCost","contents":{"acommodity":"USD","aquantity":{"floatingPoint":50}}}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			var p HLedgerPosting
			require.NoError(t, json.Unmarshal([]byte(amount), &p))
			postings, err := buildHLedgerPostings(p, transaction, pricesTree, date)
			require.NoError(t, err)
			require.Len(t, postings, 1)
			assert.True(t, postings[0].Amount.Equal(decimal.NewFromInt(100)))
		})
	}
}
```

- [ ] **Step 2: Test rot laufen lassen**

Run: `go test ./internal/ledger -run TestBuildHLedgerPostingsPrefersCostOverPrice -count=1`
Expected: FAIL, `acost` wird ignoriert (moderne Fälle liefern 0/400).

- [ ] **Step 3: Implementieren**

- `hledgerPrice`-Struct aus dem Fork-Commit übernehmen.
- `Price hledgerPrice \`json:"aprice"\``und`Cost hledgerPrice \`json:"acost"\``im`HLedgerPosting.Amount`-Struct ergänzen.
- In `buildHLedgerPostings`:

```go
amountPrice := amount.Cost
if amountPrice.Tag == "" {
	amountPrice = amount.Price
}
```

Alle bisherigen `amount.Price`-Zugriffe auf `amountPrice` umstellen; `TotalCost` wie `TotalPrice` und `UnitCost` wie `UnitPrice` behandeln.

- [ ] **Step 4: Test grün laufen lassen**

Run: `go test ./internal/ledger -run TestBuildHLedgerPostingsPrefersCostOverPrice -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ledger/ledger.go internal/ledger/ledger_test.go
git commit -m "fix(ledger): read hledger acost for cost basis"
```

---

### Task 3: `FilterByGlob` korrigieren

**Files:**

- Modify: `internal/accounting/accounting.go`
- Test: `internal/accounting/accounting_test.go`

**Interfaces:**

- Produces: `matchAccountGlob(pattern string, account string) (bool, error)` (unexportiert).
- `FilterByGlob(postings []posting.Posting, accounts []string) []posting.Posting` bleibt signaturgleich.

**Source:** `manishtanwar/paisa-app@122f9eb3` (OR/AND), `Sarath234/paisa@b5a3d091` (Präfix ohne `*`).

- [ ] **Step 1: Test schreiben**

```go
func TestFilterByGlobCombinesPositiveAndNegativePatterns(t *testing.T) {
	ps := []posting.Posting{
		{Account: "Assets:Bank:Checking"},
		{Account: "Assets:Broker"},
		{Account: "Assets:Cash"},
		{Account: "Expenses:Food"},
	}

	accounts := func(in []posting.Posting) []string {
		out := make([]string, len(in))
		for i, p := range in { out[i] = p.Account }
		return out
	}

	assert.Equal(t, []string{"Assets:Bank:Checking", "Assets:Broker"},
		accounts(FilterByGlob(ps, []string{"Assets:*", "!Assets:Cash"})))
	assert.Equal(t, []string{"Assets:Bank:Checking", "Assets:Broker"},
		accounts(FilterByGlob(ps, []string{"Assets:Bank", "Assets:Broker"})))
	assert.Equal(t, []string{"Assets:Broker", "Assets:Cash", "Expenses:Food"},
		accounts(FilterByGlob(ps, []string{"!Assets:Bank"})))
	assert.Equal(t, []string{"Assets:Bank:Checking", "Assets:Broker", "Assets:Cash", "Expenses:Food"},
		accounts(FilterByGlob(ps, nil)))
}
```

- [ ] **Step 2: Test rot laufen lassen**

Run: `go test ./internal/accounting -run TestFilterByGlobCombinesPositiveAndNegativePatterns -count=1`
Expected: FAIL: `["Assets:*", "!Assets:Cash"]` liefert nichts, `["Assets:Bank", "Assets:Broker"]` matcht keine Unterkonten.

- [ ] **Step 3: Implementieren**

- `accounts` in `positives` und `negatives` aufteilen.
- Negative Muster zuerst prüfen: Treffer → `false`.
- Ohne positive Muster → `true`.
- Positive Muster ODER-verknüpfen.
- `matchAccountGlob`: Enthält das Muster `*`, `filepath.Match`; sonst `utils.IsSameOrParent(account, pattern)`.
- Leere Strings ignorieren; ungültige Globs loggen und als Nicht-Treffer behandeln (kein `log.Fatal` in einer Filterfunktion).

- [ ] **Step 4: Test grün laufen lassen**

Run: `go test ./internal/accounting -run TestFilterByGlobCombinesPositiveAndNegativePatterns -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/accounting/accounting.go internal/accounting/accounting_test.go
git commit -m "fix(accounting): combine positive and negative account globs correctly"
```

---

### Task 4: Gehaltsklassifikation und `salary_income_accounts`

**Files:**

- Modify: `internal/config/config.go`
- Modify: `internal/config/schema.json`
- Modify: `internal/server/investment.go`
- Modify: `internal/server/monthly_review.go`
- Modify: `docs/reference/config.md`
- Test: `internal/config/config_test.go`
- Test: `internal/server/investment_test.go`

**Interfaces:**

- Produces: `Config.SalaryIncomeAccounts *[]string` (JSON/YAML `salary_income_accounts`); `isSalaryIncome(account string) bool`.

**Source:** `1Feng/paisa@df35cad3`, `1Feng/paisa@b31b02bb`.

- [ ] **Step 1: Config-Test schreiben**

```go
func TestSalaryIncomeAccountsConfig(t *testing.T) {
	base := "journal_path: test.ledger\ndb_path: test.db\n"
	require.NoError(t, LoadConfig([]byte(base+"salary_income_accounts: [Income:Employment:Wages]\n"), ""))
	require.NotNil(t, GetConfig().SalaryIncomeAccounts)
	assert.Equal(t, []string{"Income:Employment:Wages"}, *GetConfig().SalaryIncomeAccounts)

	require.NoError(t, LoadConfig([]byte(base+"salary_income_accounts: []\n"), ""))
	require.NotNil(t, GetConfig().SalaryIncomeAccounts)
	assert.Empty(t, *GetConfig().SalaryIncomeAccounts)

	assert.Error(t, LoadConfig([]byte(base+"salary_income_accounts: Income:Salary\n"), ""))
	assert.Error(t, LoadConfig([]byte(base+"salary_income_accounts: [Assets:Employment]\n"), ""))
}
```

- [ ] **Step 2: Test rot laufen lassen**

Run: `go test ./internal/config -run TestSalaryIncomeAccountsConfig -count=1`
Expected: FAIL, Feld unbekannt/Schema lehnt ab.

- [ ] **Step 3: Config und Schema ergänzen**

- `SalaryIncomeAccounts *[]string` in `Config` ergänzen; Default bleibt `nil`.
- `internal/config/schema.json` um `salary_income_accounts` erweitern: Array aus Strings, Pattern `^Income:[^:*]+(:[^:*]+)*$`, `uniqueItems: true`, Beschreibung wie im Fork.
- `docs/reference/config.md` um die Semantik ergänzen: `nil` = `Income:Salary` + `Income:Payroll`; `[]` = keine Gehälter; explizite Liste ersetzt die Defaults; Matching per ganzen Segmenten.

- [ ] **Step 4: Klassifikations-Test schreiben**

In `internal/server/investment_test.go`:

```go
func TestIsSalaryIncomeMatchesWholeSegments(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte(
		"journal_path: test.ledger\ndb_path: test.db\n",
	), ""))
	for account, want := range map[string]bool{
		"Income:Salary":              true,
		"Income:Salary:Employer":     true,
		"Income:Payroll:Bonus":       true,
		"Income:SalaryOther":         false,
		"Income:PayrollOther":        false,
		"Income:Bank:Interest":       false,
	} {
		assert.Equal(t, want, isSalaryIncome(account), account)
	}
}
```

- [ ] **Step 5: Test rot laufen lassen**

Run: `go test ./internal/server -run TestIsSalaryIncomeMatchesWholeSegments -count=1`
Expected: FAIL, `isSalaryIncome` fehlt.

- [ ] **Step 6: Implementieren**

In `internal/server/investment.go`:

```go
func isSalaryIncome(account string) bool {
	accounts := []string{"Income:Salary", "Income:Payroll"}
	if configured := config.GetConfig().SalaryIncomeAccounts; configured != nil {
		accounts = *configured
	}
	for _, parent := range accounts {
		if utils.IsSameOrParent(account, parent) {
			return true
		}
	}
	return false
}
```

`strings.HasPrefix`-Prüfungen in `computeInvestmentYearlyCard` durch `isSalaryIncome` ersetzen. In `internal/server/monthly_review.go` `monthlyIncome` ebenfalls `isSalaryIncome` nutzen.

- [ ] **Step 7: Karten-Test ergänzen**

```go
func TestInvestmentYearlyCardUsesConfiguredSalaryAccounts(t *testing.T) {
	configureInvestmentTestWith(t, map[string]interface{}{
		"salary_income_accounts": []string{"Income:Employment:Wages"},
	})
	date := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	assets := []posting.Posting{{Date: date, Account: "Assets:Broker", Amount: decimal.NewFromInt(100)}}
	incomes := []posting.Posting{
		{Date: date, Account: "Income:Employment:Wages", Amount: decimal.NewFromInt(-1000)},
		{Date: date, Account: "Income:Other", Amount: decimal.NewFromInt(-50)},
	}
	cards := computeInvestmentYearlyCard(date, assets, nil, incomes)
	require.NotEmpty(t, cards)
	assert.True(t, cards[0].GrossSalaryIncome.Equal(decimal.NewFromInt(1000)))
	assert.True(t, cards[0].GrossOtherIncome.Equal(decimal.NewFromInt(50)))
}
```

`configureInvestmentTestWith(t, extra map[string]interface{})` als Helfer in `investment_test.go` anlegen; der bestehende `configureInvestmentTest` baut die Basis-Map und ruft den Helfer mit `nil` auf. Die Keys aus `extra` werden in die Map gemergt.

- [ ] **Step 8: Tests grün laufen lassen**

Run:

```bash
go test ./internal/config -run TestSalaryIncomeAccountsConfig -count=1
go test ./internal/server -run 'TestIsSalaryIncomeMatchesWholeSegments|TestInvestmentYearlyCardUsesConfiguredSalaryAccounts' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/config/config.go internal/config/schema.json internal/config/config_test.go internal/server/investment.go internal/server/monthly_review.go internal/server/investment_test.go docs/reference/config.md
git commit -m "feat(config): classify payroll and configurable salary income accounts"
```

---

### Task 5: Interne Transfers ausschließen

**Files:**

- Create: `internal/accounting/transfer.go`
- Test: `internal/accounting/transfer_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/schema.json`
- Modify: `internal/server/investment.go`
- Modify: `internal/server/monthly_review.go`
- Test: `internal/server/investment_test.go`
- Test: `internal/server/monthly_review_test.go`
- Test: `internal/server/monthly_performance_test.go` (Helfer)
- Modify: `docs/reference/config.md`

**Interfaces:**

- Produces: `accounting.IsInternalTransfer(postings []posting.Posting, globs []string) bool`; `accounting.FilterOutInternalTransfers(postings []posting.Posting, globs []string) []posting.Posting`; `Config.TransferAccounts []string`.

**Source:** `1Feng/paisa@80b0ae5f`.

- [ ] **Step 1: Transfer-Test schreiben**

```go
func TestIsInternalTransfer(t *testing.T) {
	tx := []posting.Posting{
		{TransactionID: "t1", Account: "Assets:Cash"},
		{TransactionID: "t1", Account: "Assets:Broker"},
	}
	assert.True(t, IsInternalTransfer(tx, []string{"Assets:Cash"}))

	withExpense := append(append([]posting.Posting{}, tx...), posting.Posting{TransactionID: "t1", Account: "Expenses:Fee"})
	assert.False(t, IsInternalTransfer(withExpense, []string{"Assets:Cash"}))
	assert.False(t, IsInternalTransfer(tx, nil))
}

func TestFilterOutInternalTransfersRemovesWholeTransaction(t *testing.T) {
	ps := []posting.Posting{
		{TransactionID: "transfer", Account: "Assets:Cash"},
		{TransactionID: "transfer", Account: "Assets:Broker"},
		{TransactionID: "buy", Account: "Assets:Checking"},
		{TransactionID: "buy", Account: "Assets:Broker"},
	}
	filtered := FilterOutInternalTransfers(ps, []string{"Assets:Cash"})
	require.Len(t, filtered, 2)
	assert.Equal(t, "buy", filtered[0].TransactionID)
	assert.Equal(t, "buy", filtered[1].TransactionID)
}
```

- [ ] **Step 2: Test rot laufen lassen**

Run: `go test ./internal/accounting -run TestIsInternalTransfer -count=1`
Expected: FAIL, Funktion fehlt.

- [ ] **Step 3: `transfer.go` implementieren**

Portiere `matchAccountGlob`, `IsInternalTransfer` und `FilterOutInternalTransfers` aus `1Feng/paisa@80b0ae5f`. Regeln:

- `len(globs) == 0` oder weniger als 2 Postings → kein Transfer.
- **Alle** Legs müssen unter `Assets` liegen (`utils.IsSameOrParent(p.Account, "Assets")`).
- **Mindestens ein** Leg matcht einen Glob (`doublestar.Match`).
- `FilterOutInternalTransfers` gruppiert nach `TransactionID` und entfernt alle Postings klassifizierter Transaktionen.

- [ ] **Step 4: Config und Schema ergänzen**

- `TransferAccounts []string` in `Config`, Default `[]string{}`.
- Schema-Property `transfer_accounts` mit Array von Strings und Beschreibung (interne Transfers, nur Assets-zu-Assets).
- `docs/reference/config.md` mit Beispielen (`Assets:Tagesgeld`, `Assets:Depot:Verrechnung`).

- [ ] **Step 5: `GetInvestment` anpassen**

In `internal/server/investment.go`:

- `filterOutInternalTransfers(db *gorm.DB, assets []posting.Posting) []posting.Posting` wie im Fork: alle Legs der betroffenen `TransactionID`s nachladen, `accounting.IsInternalTransfer` prüfen.
- Nach dem Stock-Split-Filter aufrufen.

- [ ] **Step 6: Monatsrückblick anpassen**

In `internal/server/monthly_review.go` `monthlyInvestmentContributions`:

```go
func monthlyInvestmentContributions(postings []posting.Posting) decimal.Decimal {
	postings = accounting.FilterOutInternalTransfers(postings, config.GetConfig().TransferAccounts)
	total := decimal.Zero
	for _, p := range postings {
		if utils.FirstName(p.Account) == "Assets" && !utils.IsCheckingAccount(p.Account) {
			total = total.Add(p.Amount)
		}
	}
	return total
}
```

- [ ] **Step 7: Integrationstests schreiben**

In `internal/server/investment_test.go`:

```go
func TestGetInvestmentExcludesInternalTransfers(t *testing.T) {
	configureInvestmentTestWith(t, map[string]interface{}{
		"transfer_accounts": []string{"Assets:Cash"},
	})
	db := openInvestmentTestDB(t)
	date := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	rows := []posting.Posting{
		{TransactionID: "transfer", Date: date, Account: "Assets:Cash", Amount: decimal.NewFromInt(-100), Quantity: decimal.NewFromInt(-100), Commodity: "EUR"},
		{TransactionID: "transfer", Date: date, Account: "Assets:Broker", Amount: decimal.NewFromInt(100), Quantity: decimal.NewFromInt(100), Commodity: "EUR"},
		{TransactionID: "buy", Date: date, Account: "Assets:Checking:Bank", Amount: decimal.NewFromInt(-50), Quantity: decimal.NewFromInt(-50), Commodity: "EUR"},
		{TransactionID: "buy", Date: date, Account: "Assets:Broker", Amount: decimal.NewFromInt(50), Quantity: decimal.NewFromInt(50), Commodity: "EUR"},
	}
	require.NoError(t, db.Create(&rows).Error)

	result := GetInvestment(db)
	assets := result["assets"].([]posting.Posting)
	require.Len(t, assets, 1)
	assert.Equal(t, "buy", assets[0].TransactionID)
}
```

In `internal/server/monthly_review_test.go`:

```go
func TestMonthlyReviewExcludesInternalTransfersFromContributions(t *testing.T) {
	setMonthlyReviewNow(t, "2026-06-15")
	db := openMonthlyReviewTestDB(t)
	loadMonthlyReviewConfigWith(t, "transfer_accounts: [Assets:Cash]\n")

	addMonthlyReviewCurrencyPosting(t, db, "transfer", "2026-06-01", "Assets:Cash", "-100")
	addMonthlyReviewCurrencyPosting(t, db, "transfer", "2026-06-01", "Assets:Broker", "100")
	addMonthlyReviewCurrencyPosting(t, db, "buy", "2026-06-02", "Assets:Checking:Bank", "-50")
	addMonthlyReviewCurrencyPosting(t, db, "buy", "2026-06-02", "Assets:Broker", "50")

	review := GetMonthlyReview(db, monthlyReviewTestDate("2026-06-01"))
	requireDecimalEqual(t, "50", review.InvestmentContributions)
}
```

`loadMonthlyReviewConfigWith(t, extra string)` in `internal/server/monthly_performance_test.go` ergänzen; bestehendes `loadMonthlyReviewConfig` ruft es mit `""` auf:

```go
func loadMonthlyReviewConfigWith(t *testing.T, extra string) {
	t.Helper()
	err := config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
`+extra), "")
	require.NoError(t, err)
}

func loadMonthlyReviewConfig(t *testing.T) {
	loadMonthlyReviewConfigWith(t, "")
}
```

- [ ] **Step 8: Tests grün laufen lassen**

Run:

```bash
go test ./internal/accounting -run 'TestIsInternalTransfer|TestFilterOutInternalTransfers' -count=1
go test ./internal/server -run 'TestGetInvestmentExcludesInternalTransfers|TestMonthlyReviewExcludesInternalTransfersFromContributions' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/accounting/transfer.go internal/accounting/transfer_test.go internal/config/config.go internal/config/schema.json internal/server/investment.go internal/server/monthly_review.go internal/server/investment_test.go internal/server/monthly_review_test.go internal/server/monthly_performance_test.go docs/reference/config.md
git commit -m "feat: exclude internal asset transfers from savings rate"
```

---

### Task 6: Yahoo `GBp`/`GBX` normalisieren

**Files:**

- Modify: `internal/scraper/stock/yahoo.go`
- Test: `internal/scraper/stock/yahoo_test.go`

**Interfaces:**

- Produces: `normalizeYahooCurrency(currency string) (string, float64)`; `normalizeYahooPrice(value float64, currency string) (float64, string)`.

**Source:** `mrkstwrt/paisa-source@7f4bd850` (nur die Normalisierung; unsere FX-/Nullwert-Logik bleibt).

- [ ] **Step 1: Test schreiben**

```go
func TestNormalizeYahooCurrency(t *testing.T) {
	for _, tc := range []struct {
		in     string
		want   string
		scale  float64
	}{
		{"GBp", "GBP", 0.01},
		{"GBX", "GBP", 0.01},
		{"GBP", "GBP", 1},
		{"USD", "USD", 1},
	} {
		got, scale := normalizeYahooCurrency(tc.in)
		assert.Equal(t, tc.want, got)
		assert.Equal(t, tc.scale, scale)
	}
}

func TestGetHistoryConvertsGBpToGBP(t *testing.T) {
	loadYahooTestConfig(t) // DefaultCurrency EUR
	previous := yahooHTTPClient
	t.Cleanup(func() { yahooHTTPClient = previous })
	yahooHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.String(), "/VOD.L?"):
			return yahooJSONResponse(yahooChartJSON("GBp", []string{"2024-01-02"}, []float64{100})), nil
		case strings.Contains(req.URL.String(), "/GBPEUR=X?"):
			return yahooJSONResponse(yahooChartJSON("EUR", []string{"2024-01-02"}, []float64{1.2})), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	prices, err := GetHistory("VOD.L", "Vodafone")
	require.NoError(t, err)
	require.Len(t, prices, 1)
	assert.True(t, prices[0].Value.Equal(decimal.RequireFromString("1.2")))
}
```

- [ ] **Step 2: Test rot laufen lassen**

Run: `go test ./internal/scraper/stock -run 'TestNormalizeYahooCurrency|TestGetHistoryConvertsGBpToGBP' -count=1`
Expected: FAIL, `GBpEUR=X` wird angefragt bzw. Wert bleibt 100.

- [ ] **Step 3: Implementieren**

- `normalizeYahooCurrency`: `GBp`/`GBX` → `GBP`, Faktor `0.01`; sonst unverändert, `1`.
- In `GetHistory`: `quoteCurrency, scale := normalizeYahooCurrency(result.Meta.Currency)`; `needExchangePrice := !utils.IsCurrency(quoteCurrency)`; Exchange-Ticker mit `quoteCurrency` bauen; `value = value * scale` **vor** der FX-Multiplikation.
- Bestehende `math.IsNaN`/`value == 0`-Guards und FX-Abdeckungslogik nicht verändern.

- [ ] **Step 4: Tests grün laufen lassen**

Run: `go test ./internal/scraper/stock -run 'TestNormalizeYahooCurrency|TestGetHistoryConvertsGBpToGBP' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/scraper/stock/yahoo.go internal/scraper/stock/yahoo_test.go
git commit -m "fix(scraper): normalize Yahoo GBp/GBX prices to GBP"
```

---

### Task 7: Yahoo-Cookie/Crumb-Session

**Files:**

- Modify: `internal/scraper/stock/yahoo.go`
- Test: `internal/scraper/stock/yahoo_test.go`

**Interfaces:**

- Produces: `yahooSession`, `newYahooSession() *yahooSession`, `yahooSessionState`, `yahooCookieURL`, `yahooCrumbURL`, `(*yahooSession).getCrumb(refresh bool) (string, error)`, `statusError`, `retryable(error) bool`.
- `getTicker(ticker string) (*Response, error)` bleibt signaturgleich.

**Source:** `stan-ely/paisa@25859f7d` (Session/Crumb/Retry), angepasst an unseren injizierbaren `yahooHTTPClient`.

- [ ] **Step 1: Session-Test schreiben**

```go
func TestGetTickerUsesCrumb(t *testing.T) {
	loadYahooTestConfig(t)
	previousClient := yahooHTTPClient
	previousCookieURL, previousCrumbURL := yahooCookieURL, yahooCrumbURL
	yahooCookieURL, yahooCrumbURL = "https://example.test/cookie", "https://example.test/crumb"
	yahooSessionState = newYahooSession()
	t.Cleanup(func() {
		yahooHTTPClient = previousClient
		yahooCookieURL, yahooCrumbURL = previousCookieURL, previousCrumbURL
		yahooSessionState = newYahooSession()
	})

	var sawCrumb bool
	yahooHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.String() == yahooCookieURL:
			return &http.Response{StatusCode: 200, Header: http.Header{"Set-Cookie": []string{"A=1; Path=/"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		case req.URL.String() == yahooCrumbURL:
			return yahooJSONResponse("crumb123"), nil
		case strings.Contains(req.URL.String(), "/AAPL?"):
			sawCrumb = strings.Contains(req.URL.RawQuery, "crumb=crumb123")
			return yahooJSONResponse(yahooChartJSON("USD", []string{"2024-01-02"}, []float64{100})), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	_, err := getTicker("AAPL")
	require.NoError(t, err)
	assert.True(t, sawCrumb)
}
```

- [ ] **Step 2: Retry-Test schreiben**

```go
func TestGetTickerRefreshesCrumbOnUnauthorized(t *testing.T) {
	// Fake-Transport: erster Chart-Request -> 401, zweiter -> 200.
	// Crumb-Endpoint zählt Aufrufe; erwartet 2 (initial + refresh).
	// getTicker("AAPL") muss erfolgreich sein und crumbCalls == 2.
}
```

- [ ] **Step 3: Tests rot laufen lassen**

Run: `go test ./internal/scraper/stock -run 'TestGetTickerUsesCrumb|TestGetTickerRefreshesCrumbOnUnauthorized' -count=1`
Expected: FAIL, `yahooSessionState`/`yahooCookieURL` fehlen.

- [ ] **Step 4: Implementieren**

- `yahooCookieURL = "https://fc.yahoo.com"` und `yahooCrumbURL = "https://query1.finance.yahoo.com/v1/test/getcrumb"` als Paketvariablen (für Tests überschreibbar).
- `yahooSession` mit `sync.Mutex`, `http.CookieJar` und gecachtem `crumb`.
- `newYahooSession()` legt einen Cookie-Jar an.
- `getCrumb(refresh bool)`: Cookie holen (Body ignorieren, `Set-Cookie` zählt), Crumb holen, trimmen, bei leerem/HTML-Body Fehler.
- `client()`: `&http.Client{Jar: s.jar, Transport: yahooHTTPClient.Transport, Timeout: 30 * time.Second}`; so bleiben Test-Transport und Produktions-Client vereinbar.
- `fetchTicker(ticker, refresh bool)`: Crumb holen, `crumb` als Query-Parameter anhängen, Status prüfen, JSON parsen.
- `getTicker`: bei `401/403/429` (`*statusError`) einmal mit `refresh=true` wiederholen.
- `firstYahooResult` weiterverwenden; keine Panics bei leeren Results.

- [ ] **Step 5: Tests grün laufen lassen**

Run: `go test ./internal/scraper/stock -run 'TestGetTickerUsesCrumb|TestGetTickerRefreshesCrumbOnUnauthorized' -count=1`
Expected: PASS.

- [ ] **Step 6: Bestehende Yahoo-Tests laufen lassen**

Run: `go test ./internal/scraper/stock -count=1`
Expected: PASS; insbesondere die bisherigen Null-/FX-Tests dürfen nicht regressieren.

- [ ] **Step 7: Commit**

```bash
git add internal/scraper/stock/yahoo.go internal/scraper/stock/yahoo_test.go
git commit -m "fix(scraper): authenticate Yahoo requests with cookie and crumb"
```

---

### Task 8: Tooltip-Kürzung

**Files:**

- Create: `src/lib/tooltip.ts`
- Create: `src/lib/tooltip.test.ts`
- Modify: `src/lib/utils.ts` (Re-Export statt Implementierung)

**Interfaces:**

- Produces: `tooltip(rows: Array<Array<string | string[]>>, options?: { header?: string; total?: string }): string`.
- `src/lib/utils.ts` re-exportiert `tooltip`, damit bestehende Importe unverändert bleiben.

**Source:** `bellussi1/paisa@5ad11187` + `4ad755ff` (eine Variante; nicht zusätzlich `savinmikhail` übernehmen).

- [ ] **Step 1: Test schreiben**

```ts
import { describe, expect, test } from "bun:test";
import { tooltip } from "./tooltip";

describe("tooltip", () => {
  test("truncates long lists but keeps the total", () => {
    const rows = Array.from({ length: 30 }, (_, i) => [`row ${i}`, `${i}`]);
    const html = tooltip(rows, { total: "123" });
    expect(html).toContain("earlier rows omitted");
    expect(html).toContain("Total");
    expect(html).toContain("123");
    expect(rows).toHaveLength(30);
  });

  test("keeps short lists intact", () => {
    const html = tooltip([["A", "1"]], { header: "Rows", total: "1" });
    expect(html).toContain("Rows");
    expect(html).toContain("Total");
    expect(html).not.toContain("omitted");
  });
});
```

- [ ] **Step 2: Test rot laufen lassen**

Run: `bun test --preload ./src/happydom.ts src/lib/tooltip.test.ts`
Expected: FAIL, Modul fehlt.

- [ ] **Step 3: Implementieren**

- Implementierung aus `src/lib/utils.ts` nach `src/lib/tooltip.ts` verschieben.
- Viewport-basierte Kürzung (`window.innerHeight`, Zeilenhöhe ~24px, vertikaler Abstand ~32px), Summenzeile immer erhalten, Header nur wenn genug Platz.
- In `utils.ts`:

```ts
export { tooltip } from "./tooltip";
```

- [ ] **Step 4: Tests grün laufen lassen**

Run:

```bash
bun test --preload ./src/happydom.ts src/lib/tooltip.test.ts
npm run check
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/lib/tooltip.ts src/lib/tooltip.test.ts src/lib/utils.ts
git commit -m "fix(ui): truncate long chart tooltips without hiding totals"
```

---

### Task 9: Navbar- und Error-Handler-Stabilität

**Files:**

- Modify: `src/lib/components/Navbar.svelte`
- Modify: `src/hooks.client.ts`
- Create: `src/lib/error.ts`
- Create: `src/lib/error.test.ts`
- Create: `src/lib/components/Navbar.guard.test.ts`

**Interfaces:**

- Produces: `formatError(error: unknown): string`; `isResizeObserverLoopError(message: unknown): boolean`.

**Source:** `manishtanwar/paisa-app@4295944c`, `0220739b`.

- [ ] **Step 1: Error-Test schreiben**

```ts
import { describe, expect, test } from "bun:test";
import { formatError, isResizeObserverLoopError } from "./error";

describe("error helpers", () => {
  test("does not crash on null", () => {
    expect(formatError(null)).toBe("Unknown error");
    expect(formatError(undefined)).toBe("Unknown error");
  });
  test("recognizes ResizeObserver loop warnings", () => {
    expect(isResizeObserverLoopError("ResizeObserver loop limit exceeded")).toBe(true);
    expect(isResizeObserverLoopError("Something else")).toBe(false);
  });
});
```

- [ ] **Step 2: Test rot laufen lassen**

Run: `bun test --preload ./src/happydom.ts src/lib/error.test.ts`
Expected: FAIL, Modul fehlt.

- [ ] **Step 3: Implementieren**

- `formatError` und `isResizeObserverLoopError` aus `hooks.client.ts` in `src/lib/error.ts` verschieben; `hooks.client.ts` importiert sie.
- `formatError`: bei `!error` → `"Unknown error"`; sonst Stack/Message/String.
- Error-Listener: ResizeObserver-Loop-Warnungen ignorieren.
- Navbar: `if (selectedSubLink && !_.isEmpty(selectedSubLink.children))`.

- [ ] **Step 4: Navbar-Guard-Test schreiben**

```ts
import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("Navbar sub-link guard", () => {
  test("checks selectedSubLink before reading children", () => {
    const source = readFileSync(join(__dirname, "Navbar.svelte"), "utf-8");
    expect(source).toMatch(
      /if\s*\(\s*selectedSubLink\s*&&\s*!_\.isEmpty\(\s*selectedSubLink\.children\s*\)\s*\)/
    );
  });
});
```

- [ ] **Step 5: Tests grün laufen lassen**

Run:

```bash
bun test --preload ./src/happydom.ts src/lib/error.test.ts src/lib/components/Navbar.guard.test.ts
npm run check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add src/hooks.client.ts src/lib/error.ts src/lib/error.test.ts src/lib/components/Navbar.svelte src/lib/components/Navbar.guard.test.ts
git commit -m "fix(ui): guard navbar sub-links and benign browser errors"
```

---

### Task 10: Import-/Template-Robustheit

**Files:**

- Modify: `internal/model/template/template.go`
- Test: `internal/model/template/template_test.go`
- Modify: `src/routes/(app)/ledger/import/+page.svelte`
- Create: `src/routes/(app)/ledger/import/page.guard.test.ts`
- Modify: `src/lib/journal.ts`
- Modify: `src/lib/journal.test.ts`
- Modify: `src/lib/template_helpers.ts`
- Modify: `src/lib/import.test.ts`

**Interfaces:**

- `Template.All() []Template` liefert nie `nil`.
- Produces: `all(custom []config.ImportTemplate, builtins fs.FS) []Template` (unexportiert, testbar).
- `negate(value: string | number): string | undefined` nutzt `BigNumber`.

**Source:** `1Feng/paisa@0ca8d474`, `3ea8aab0`, `63b10690`; `St-Gonne/paisa@f046d740`.

- [ ] **Step 1: Template-Test schreiben**

```go
func TestAllReturnsEmptySliceNotNil(t *testing.T) {
	templates := all(nil, fstest.MapFS{})
	assert.NotNil(t, templates)
	payload, err := json.Marshal(map[string]any{"templates": templates})
	require.NoError(t, err)
	assert.Equal(t, `{"templates":[]}`, string(payload))
}

func TestAllIncludesCustomTemplates(t *testing.T) {
	templates := all([]config.ImportTemplate{{Name: "my-bank", Content: "{{date}}"}}, fstest.MapFS{})
	require.Len(t, templates, 1)
	assert.Equal(t, "my-bank", templates[0].Name)
}
```

- [ ] **Step 2: Test rot laufen lassen**

Run: `go test ./internal/model/template -run 'TestAllReturnsEmptySliceNotNil|TestAllIncludesCustomTemplates' -count=1`
Expected: FAIL, `all` fehlt.

- [ ] **Step 3: `All`/`all` implementieren**

Portiere die Struktur aus `1Feng/paisa@0ca8d474`, angepasst an `fs.FS`:

```go
func All() []Template {
	return all(config.GetConfig().ImportTemplates, BuiltinTemplates)
}

func all(custom []config.ImportTemplate, builtins fs.FS) []Template
```

Anforderungen an `all`:

- Startet mit `templates := []Template{}` (nie `nil`).
- Übernimmt Custom-Templates unverändert wie bisher.
- Liest `templates/` aus `builtins`; `fs.ErrNotExist` ist ein gültiger Leerzustand, andere Fehler bleiben `log.Fatal`.
- Überspringt Dateien, deren Endung nicht `.handlebars` ist.
- Entfernt die Endung für `ID`/`Name` wie im aktuellen Code.
- `BuiltinTemplates` ist `embed.FS` und erfüllt `fs.FS`; Imports `errors`, `io/fs`, `path` ergänzen.

- [ ] **Step 4: Import-Page-Guard testen**

```ts
import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("ledger import page guards", () => {
  test("normalises a null template response", () => {
    const source = readFileSync(join(__dirname, "+page.svelte"), "utf-8");
    expect(source).toMatch(/templates\s*=\s*templates\s*(\|\||\?\?)\s*\[\s*\]/);
    expect(source).toMatch(/if\s*\(\s*templates\.length\s*>\s*0\s*\)/);
  });
});
```

- [ ] **Step 5: `+page.svelte` anpassen**

- Nach `({ templates } = await ajax("/api/templates"));` → `templates = templates || [];`.
- `templates[0]` nur innerhalb `if (templates.length > 0)` lesen; sonst leeren Editor seeden (`saveAsName = ""`).
- Beim Löschen des letzten Templates `selectedTemplate = undefined` setzen.

- [ ] **Step 6: Journal-Alignment-Test schreiben**

```ts
test("falls back when amount_alignment_column is missing or zero", () => {
  const previous = (globalThis as any).USER_CONFIG;
  (globalThis as any).USER_CONFIG = { amount_alignment_column: 0 };
  try {
    expect(() => format(readFixture("unformatted.ledger"))).not.toThrow();
  } finally {
    (globalThis as any).USER_CONFIG = previous;
  }
});
```

- [ ] **Step 7: `journal.ts` anpassen**

```ts
if (
  typeof USER_CONFIG !== "undefined" &&
  typeof USER_CONFIG.amount_alignment_column === "number" &&
  USER_CONFIG.amount_alignment_column > 0
) {
  amountAlignmentColumn = USER_CONFIG.amount_alignment_column;
}
```

- [ ] **Step 8: `negate`-Test schreiben**

In `src/lib/import.test.ts`:

```ts
test("negate preserves decimal notation", () => {
  assert.strictEqual(helpers.negate("0.00000001"), "-0.00000001");
  assert.strictEqual(helpers.negate(0.00000001), "-0.00000001");
  assert.strictEqual(helpers.negate("-0.00000001"), "0.00000001");
  assert.strictEqual(helpers.negate("₹ 1,234.56"), "-1234.56");
  assert.strictEqual(helpers.negate(0), "0");
});
```

- [ ] **Step 9: `negateAmount` implementieren**

```ts
import { BigNumber } from "bignumber.js";

function negateAmount(value: string | number) {
  const amount = _.isNumber(value) ? value.toString() : scrubAmount(value);
  if (amount) {
    return new BigNumber(amount).negated().toFixed();
  }
}
```

`negate: negateAmount` im Helper-Objekt.

- [ ] **Step 10: Tests grün laufen lassen**

Run:

```bash
go test ./internal/model/template -count=1
bun test --preload ./src/happydom.ts src/lib/journal.test.ts src/lib/import.test.ts 'src/routes/(app)/ledger/import/page.guard.test.ts'
```

Expected: PASS.

- [ ] **Step 11: Commit**

```bash
git add internal/model/template/template.go internal/model/template/template_test.go 'src/routes/(app)/ledger/import/+page.svelte' 'src/routes/(app)/ledger/import/page.guard.test.ts' src/lib/journal.ts src/lib/journal.test.ts src/lib/template_helpers.ts src/lib/import.test.ts
git commit -m "fix(import): handle empty templates and preserve decimal precision"
```

---

### Task 11: SQLite-WAL und Backup-Pruning

**Files:**

- Modify: `internal/utils/utils.go` (`OpenDB`)
- Modify: `internal/server/editor.go`
- Modify: `internal/server/sheet.go` (optional, gleiches Muster)
- Test: `internal/utils/utils_test.go`
- Test: `internal/server/editor_test.go`

**Interfaces:**

- `OpenDB() (*gorm.DB, error)` bleibt signaturgleich und setzt WAL/busy_timeout.
- Produces: `pruneOldBackups(filePath string, maxBackups int)` (unexportiert, `package server`).

**Source:** `Sarath234/paisa@22abf928`, `43b52794`.

- [ ] **Step 1: WAL-Test schreiben**

```go
func TestOpenDBUsesWALAndBusyTimeout(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.LoadConfig([]byte(fmt.Sprintf(
		"journal_path: %s/main.ledger\ndb_path: %s/test.db\n", dir, dir,
	)), ""))
	db, err := OpenDB()
	require.NoError(t, err)
	var mode string
	require.NoError(t, db.Raw("PRAGMA journal_mode").Scan(&mode).Error)
	assert.Equal(t, "wal", mode)
	var timeout int
	require.NoError(t, db.Raw("PRAGMA busy_timeout").Scan(&timeout).Error)
	assert.Equal(t, 5000, timeout)
}
```

- [ ] **Step 2: Test rot laufen lassen**

Run: `go test ./internal/utils -run TestOpenDBUsesWALAndBusyTimeout -count=1`
Expected: FAIL, `mode` ist `delete`, `timeout` 0.

- [ ] **Step 3: `OpenDB` anpassen**

```go
func OpenDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(config.GetDBPath()), &gorm.Config{Logger: gorm_logrus.New()})
	if err != nil {
		return db, err
	}
	if err := db.Exec("PRAGMA journal_mode = WAL").Error; err != nil {
		return db, err
	}
	if err := db.Exec("PRAGMA busy_timeout = 5000").Error; err != nil {
		return db, err
	}
	return db, nil
}
```

- [ ] **Step 4: Prune-Test schreiben**

```go
func TestPruneOldBackupsKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	journal := filepath.Join(dir, "main.ledger")
	for i := 0; i < 15; i++ {
		name := fmt.Sprintf("%s.backup.2026-01-%02d-00-00-00.000", journal, i+1)
		require.NoError(t, os.WriteFile(name, []byte("x"), 0644))
	}
	pruneOldBackups(journal, 10)
	matches, err := filepath.Glob(journal + ".backup.*")
	require.NoError(t, err)
	assert.Len(t, matches, 10)
}
```

- [ ] **Step 5: Test rot laufen lassen**

Run: `go test ./internal/server -run TestPruneOldBackupsKeepsNewest -count=1`
Expected: FAIL, Funktion fehlt.

- [ ] **Step 6: Implementieren**

- `pruneOldBackups` in `editor.go`: `filepath.Glob(filePath + ".backup.*")`, sortieren, die ältesten löschen, bis nur `maxBackups` übrig sind.
- Nach erfolgreichem Schreiben in `SaveFile` aufrufen.
- Optional gleiches in `SaveSheet`; wenn übernommen, mit eigenem Test.
- Keine Löschung bei fehlgeschlagenem Schreiben.

- [ ] **Step 7: Tests grün laufen lassen**

Run:

```bash
go test ./internal/utils -run TestOpenDBUsesWALAndBusyTimeout -count=1
go test ./internal/server -run TestPruneOldBackupsKeepsNewest -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/utils/utils.go internal/utils/utils_test.go internal/server/editor.go internal/server/editor_test.go internal/server/sheet.go
git commit -m "fix(db): enable WAL and prune old editor backups"
```

---

### Task 12: XSS-Flächen und Rate-Limit-Key

**Files:**

- Modify: `src/lib/components/LevelItem.svelte`
- Modify: `src/lib/components/PriceCodeSearchModal.svelte`
- Create: `src/lib/components/html.guard.test.ts`
- Modify: `internal/server/server.go`
- Test: `internal/server/server_test.go` (neu oder bestehend)

**Interfaces:**

- Produces: `rateLimitKey(c *gin.Context) string`.

**Source:** `Sarath234/paisa@6b730f51` (nur XSS- und Rate-Limit-Teil).

- [ ] **Step 1: XSS-Guard-Test schreiben**

```ts
import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("untrusted HTML", () => {
  test("provider/help/subtitle fields are not rendered with @html", () => {
    for (const file of ["LevelItem.svelte", "PriceCodeSearchModal.svelte"]) {
      const source = readFileSync(join(__dirname, file), "utf-8");
      expect(source).not.toContain("{@html subtitle}");
      expect(source).not.toContain("{@html selectedProvider.description}");
      expect(source).not.toContain("{@html field.help}");
    }
  });
});
```

- [ ] **Step 2: Test rot laufen lassen**

Run: `bun test --preload ./src/happydom.ts src/lib/components/html.guard.test.ts`
Expected: FAIL.

- [ ] **Step 3: Komponenten anpassen**

`{@html subtitle}` → `{subtitle}`, `{@html selectedProvider.description}` → `{selectedProvider.description}`, `{@html field.help}` → `{field.help}`. Prüfen, ob die Werte weiterhin als Text korrekt aussehen; keine vorsätzlichen HTML-Formatierungen verlieren (falls doch, sanitize statt `@html`).

- [ ] **Step 4: Rate-Limit-Test schreiben**

In `internal/server/server_test.go`:

```go
func TestRateLimitKeyUsesClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/ping", nil)
	ctx.Request.RemoteAddr = "203.0.113.9:1234"
	assert.Equal(t, "203.0.113.9", rateLimitKey(ctx))
}
```

- [ ] **Step 5: Test rot laufen lassen**

Run: `go test ./internal/server -run TestRateLimitKeyUsesClientIP -count=1`
Expected: FAIL, Funktion fehlt.

- [ ] **Step 6: Implementieren**

```go
func rateLimitKey(c *gin.Context) string {
	return c.ClientIP()
}
```

In `TokenAuthMiddleware` beide `RateLimitCtx(..., "user", ...)` durch `RateLimitCtx(..., rateLimitKey(c), ...)` ersetzen.

- [ ] **Step 7: Tests grün laufen lassen**

Run:

```bash
bun test --preload ./src/happydom.ts src/lib/components/html.guard.test.ts
go test ./internal/server -run TestRateLimitKeyUsesClientIP -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add src/lib/components/LevelItem.svelte src/lib/components/PriceCodeSearchModal.svelte src/lib/components/html.guard.test.ts internal/server/server.go internal/server/server_test.go
git commit -m "fix(security): remove untrusted html and rate-limit per client IP"
```

---

### Task 13: FIFO-Reslicing und DB-Indizes

**Files:**

- Modify: `internal/accounting/accounting.go` (`FIFO`)
- Modify: `internal/model/posting/posting.go`
- Modify: `internal/model/price/price.go`
- Test: `internal/accounting/accounting_test.go`
- Test: `internal/model/model_test.go` (neu oder bestehend)

**Interfaces:**

- `FIFO(postings []posting.Posting) []posting.Posting` bleibt signaturgleich.
- GORM-Tags: `Date`, `Account`, `Commodity` auf `posting.Posting`; `Date`, `CommodityName` auf `price.Price`.

**Source:** `Sarath234/paisa@6b730f51` (Performance-/Index-Teil).

- [ ] **Step 1: FIFO-Test schreiben**

```go
func TestFIFOConsumesLotsInOrder(t *testing.T) {
	ps := []posting.Posting{
		{Commodity: "AAPL", Quantity: decimal.NewFromInt(10), Amount: decimal.NewFromInt(100)},
		{Commodity: "AAPL", Quantity: decimal.NewFromInt(10), Amount: decimal.NewFromInt(110)},
		{Commodity: "AAPL", Quantity: decimal.NewFromInt(-15), Amount: decimal.NewFromInt(-180)},
	}
	remaining := FIFO(ps)
	require.Len(t, remaining, 1)
	assert.True(t, remaining[0].Quantity.Equal(decimal.NewFromInt(5)))
	assert.True(t, remaining[0].Amount.Equal(decimal.NewFromInt(55)))
}
```

- [ ] **Step 2: Test laufen lassen (Charakterisierung)**

Run: `go test ./internal/accounting -run TestFIFOConsumesLotsInOrder -count=1`
Expected: PASS. Der Test beschreibt das bestehende Verhalten und muss den Refactor überleben.

- [ ] **Step 3: `FIFO` auf Head-Index umstellen**

Portiere die `head`-Variable aus `6b730f51`: `available := make([]posting.Posting, 0, len(postings))`, `head := 0`, Zugriffe auf `available[head]`, `head++` statt `available = available[1:]`, Rückgabe `available[head:]`.

- [ ] **Step 4: Index-Test schreiben**

In `internal/model/model_test.go`:

```go
func TestPostingAndPriceIndexesExist(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:indexes?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &price.Price{}))

	names := func(table string) []string {
		var indexes []struct{ Name string }
		require.NoError(t, db.Raw("PRAGMA index_list('"+table+"')").Scan(&indexes).Error)
		out := make([]string, 0, len(indexes))
		for _, index := range indexes {
			out = append(out, index.Name)
		}
		return out
	}

	assert.Contains(t, names("postings"), "idx_postings_date")
	assert.Contains(t, names("postings"), "idx_postings_account")
	assert.Contains(t, names("postings"), "idx_postings_commodity")
	assert.Contains(t, names("prices"), "idx_prices_date")
	assert.Contains(t, names("prices"), "idx_prices_commodity_name")
}
```

- [ ] **Step 5: Test rot laufen lassen**

Run: `go test ./internal/model -run TestPostingAndPriceIndexesExist -count=1`
Expected: FAIL, keine Indizes.

- [ ] **Step 6: Indizes ergänzen**

`Date`, `Account`, `Commodity` in `posting.Posting` mit `gorm:"index"`; `Date`, `CommodityName` in `price.Price` mit `gorm:"index"`.

- [ ] **Step 7: Tests grün laufen lassen**

Run:

```bash
go test ./internal/accounting -run TestFIFOConsumesLotsInOrder -count=1
go test ./internal/model -run TestPostingAndPriceIndexesExist -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/accounting/accounting.go internal/accounting/accounting_test.go internal/model/posting/posting.go internal/model/price/price.go internal/model/model_test.go
git commit -m "perf: index postings and prices and avoid FIFO reslicing"
```

---

### Task 14: Cashflow mit einer DB-Abfrage

**Files:**

- Modify: `internal/server/cash_flow.go`
- Test: `internal/server/cash_flow_test.go` (neu)

**Interfaces:**

- `computeCashFlow(db *gorm.DB, q *query.Query, balance decimal.Decimal) []CashFlow` bleibt signaturgleich.

**Source:** `Sarath234/paisa@6b730f51` + Folgefix `fdb5c1eb` (nur `computeCashFlow`).

- [ ] **Step 1: Verhaltens-Test schreiben**

In `internal/server/cash_flow_test.go`:

```go
func TestComputeCashFlowClassifiesPostings(t *testing.T) {
	loadMonthlyReviewConfig(t)
	utils.SetNow("2022-03-15")
	db := openMonthlyReviewTestDB(t)

	date := monthlyReviewTestDate("2022-01-15")
	rows := []posting.Posting{
		{TransactionID: "salary", Date: date, Account: "Income:Salary:Acme", Commodity: "EUR", Amount: decimal.NewFromInt(-100)},
		{TransactionID: "salary", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(100)},
		{TransactionID: "food", Date: date, Account: "Expenses:Food:Groceries", Commodity: "EUR", Amount: decimal.NewFromInt(30)},
		{TransactionID: "food", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(-30)},
		{TransactionID: "tax", Date: date, Account: "Expenses:Tax:Income", Commodity: "EUR", Amount: decimal.NewFromInt(10)},
		{TransactionID: "tax", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(-10)},
		{TransactionID: "buy", Date: date, Account: "Assets:Broker", Commodity: "EUR", Amount: decimal.NewFromInt(50)},
		{TransactionID: "buy", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(-50)},
		{TransactionID: "loan", Date: date, Account: "Liabilities:Loan", Commodity: "EUR", Amount: decimal.NewFromInt(-20)},
		{TransactionID: "loan", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(20)},
	}
	require.NoError(t, db.Create(&rows).Error)

	flows := computeCashFlow(db, query.Init(db), decimal.Zero)
	require.NotEmpty(t, flows)

	jan := flows[0]
	require.Equal(t, "2022-01-01", jan.Date.Format("2006-01-02"))
	requireDecimalEqual(t, "100", jan.Income)
	requireDecimalEqual(t, "30", jan.Expenses)
	requireDecimalEqual(t, "10", jan.Tax)
	requireDecimalEqual(t, "50", jan.Investment)
	requireDecimalEqual(t, "20", jan.Liabilities)
	requireDecimalEqual(t, "30", jan.Checking)
	requireDecimalEqual(t, "30", jan.Balance)
}
```

- [ ] **Step 2: Test grün laufen lassen (Baseline)**

Run: `go test ./internal/server -run TestComputeCashFlow -count=1`
Expected: PASS. Der Test sichert das Verhalten vor der Optimierung.

- [ ] **Step 3: Implementieren**

- Entferne die doppelte `var cashFlows []CashFlow`-Deklaration, falls sie im Arbeitsstand vorhanden ist.
- `all := q.Clone().All()` einmal laden.
- In-memory in `expenseList`, `incomeList`, `liabilityList`, `investmentList`, `taxList`, `checkingList` aufteilen; `Expenses:Tax` vor `Expenses:` prüfen; `Assets:Checking` vor `Assets:` prüfen.
- `utils.GroupByMonth` auf die Teillisten anwenden.
- `end := utils.MaxTime(utils.EndOfToday(), all[len(all)-1].Date)`.
- **Nicht** `postings[0]` verwenden – der Folgefix `fdb5c1eb` behebt genau diesen Fehler.

- [ ] **Step 4: Test erneut laufen lassen**

Run: `go test ./internal/server -run TestComputeCashFlow -count=1`
Expected: PASS, identische Werte.

- [ ] **Step 5: Commit**

```bash
git add internal/server/cash_flow.go internal/server/cash_flow_test.go
git commit -m "perf: compute cash flow from a single posting query"
```

---

### Task 15: Preis-Cache thread-sicher und ohne `log.Fatal`

**Files:**

- Modify: `internal/service/market.go`
- Test: `internal/service/market_test.go`

**Interfaces:**

- Produces: `ensurePriceCache(db *gorm.DB) *priceCache`.
- `GetUnitPrice` liefert bei fehlendem Kurs `price.Price{}` und loggt eine Warnung.

**Source:** `Sarath234/paisa@6b730f51` (Cache-Teil), `nextxm/paisa@2bc1e5c6` (Fatal → Warn).

- [ ] **Step 1: Test schreiben**

```go
func TestGetUnitPriceReturnsZeroWithoutPanic(t *testing.T) {
	if os.Getenv("PAISA_TEST_MISSING_PRICE") == "1" {
		price := GetUnitPrice(openTestDB(t), "MISSING", time.Now())
		if !price.Value.IsZero() {
			os.Exit(2)
		}
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestGetUnitPriceReturnsZeroWithoutPanic")
	cmd.Env = append(os.Environ(), "PAISA_TEST_MISSING_PRICE=1")
	require.NoError(t, cmd.Run())
}

func TestPriceCacheConcurrentClearAndRead(t *testing.T) {
	db := openTestDB(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _ = FindUnitPrice(db, "EUR", time.Now()) }()
		go func() { defer wg.Done(); ClearPriceCache() }()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Tests rot laufen lassen**

Run: `go test -race ./internal/service -run 'TestGetUnitPriceReturnsZeroWithoutPanic|TestPriceCacheConcurrentClearAndRead' -count=1`
Expected: FAIL/Panic (log.Fatal beendet den Testprozess; Race im Cache).

- [ ] **Step 3: Implementieren**

- `priceCache` ohne eingebettetes `sync.Once`; stattdessen `atomic.Pointer[priceCache]` + `sync.Mutex` für den Aufbau.
- `ensurePriceCache(db)` mit Double-Checked Locking.
- `ClearPriceCache` setzt den Pointer auf `nil`.
- `FindUnitPrice`/`GetAllPrices`/`GetUnitPrice` nutzen `ensurePriceCache`.
- `GetUnitPrice`: bei `!found` `log.WithField("commodity", commodity).Warn("Price not found")` und `price.Price{}` zurückgeben.
- `GetAllPrices`: bei fehlendem Tree `[]price.Price{}` und Warnung.

- [ ] **Step 4: Tests grün laufen lassen**

Run: `go test -race ./internal/service -run 'TestGetUnitPriceReturnsZeroWithoutPanic|TestPriceCacheConcurrentClearAndRead' -count=1`
Expected: PASS.

- [ ] **Step 5: Bestehende Service-Tests laufen lassen**

Run: `go test ./internal/service -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/service/market.go internal/service/market_test.go
git commit -m "fix(service): make price cache concurrency-safe and avoid fatal on missing prices"
```

---

### Task 16: Retirement-Zins nicht als Investment-Basis

**Files:**

- Modify: `internal/server/goal/retirement.go`
- Test: `internal/server/goal/retirement_test.go` (neu)

**Interfaces:**

- `getRetirementDetail(db *gorm.DB, conf config.RetirementGoal) gin.H` bleibt signaturgleich.

**Source:** `sean0407/paisa@6b12ddf0`.

- [ ] **Step 1: Test schreiben**

Portiere `TestRetirementDetailExcludesInterestFromInvestmentTotal` aus `6b12ddf0`:

- In-Memory-SQLite mit `posting.Posting`, `cache.Cache`, `price.Price`.
- Postings: 100 Assets:Test, 2 Interest Assets:Test, -2 Income:Interest:Test.
- `detail := getRetirementDetail(db, config.RetirementGoal{Name: "Test", Savings: []string{"Assets:TestAccount"}, YearlyExpenses: 45000, SWR: 4})`.
- `investmentTotal == 100`, `gainTotal == 2`.

- [ ] **Step 2: Test rot laufen lassen**

Run: `go test ./internal/server/goal -run TestRetirementDetailExcludesInterestFromInvestmentTotal -count=1`
Expected: FAIL, `investmentTotal == 102`, `gainTotal == 0`.

- [ ] **Step 3: Implementieren**

In `getRetirementDetail`:

```go
investmentPostings := make([]posting.Posting, 0, len(savings))
for _, p := range savings {
	if !service.IsInterest(db, p) {
		investmentPostings = append(investmentPostings, p)
	}
}
investmentTotal := accounting.CostBalance(investmentPostings)
```

`service.ClearInterestCache()` und `ClearPriceCache()` im Test aufrufen, damit die Zins-Erkennung frisch lädt.

- [ ] **Step 4: Test grün laufen lassen**

Run: `go test ./internal/server/goal -run TestRetirementDetailExcludesInterestFromInvestmentTotal -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/goal/retirement.go internal/server/goal/retirement_test.go
git commit -m "fix(goals): exclude interest from retirement investment basis"
```

---

### Task 17: Abschlussprüfung, Dokumentation und Version 8.4

**Files:**

- Modify: `docs/research/2026-10-25-paisa-forks-review.md` (Status/Abweichungen, nur wenn nötig)
- Modify: `docs/reference/config.md` (falls in Task 4/5 noch nicht vollständig)
- Modify: `VERSION`
- Modify: `desktop/wails.json` (via `npm run sync-version`)
- Modify: `desktop/build/linux/DEBIAN/control` (via `npm run sync-version`)
- Keine Produktionscode-Änderung.

- [ ] **Step 1: Doku prüfen**

Sicherstellen, dass `salary_income_accounts` und `transfer_accounts` in `docs/reference/config.md` mit Defaults, `[]`-Semantik und Beispielen dokumentiert sind.

- [ ] **Step 2: Gezielte Pakettests**

Run:

```bash
go test ./internal/utils ./internal/accounting ./internal/ledger ./internal/server ./internal/service ./internal/model ./internal/scraper/stock ./internal/config ./internal/server/goal -count=1
```

Expected: PASS.

- [ ] **Step 3: Frontend-Checks**

Run:

```bash
npm run check
bun test --preload ./src/happydom.ts src/lib/date.test.ts src/lib/tooltip.test.ts src/lib/error.test.ts src/lib/journal.test.ts src/lib/import.test.ts src/lib/components/Navbar.guard.test.ts src/lib/components/html.guard.test.ts 'src/routes/(app)/ledger/import/page.guard.test.ts'
```

Expected: PASS.

- [ ] **Step 4: Voller Testlauf**

Run:

```bash
go test ./...
npm run test:prepush
npm run test:regression
```

Expected: PASS.

- [ ] **Step 5: Manueller Smoke-Test**

Run:

```bash
TZ=UTC go run . serve --now 2022-02-07
```

Dann `http://localhost:7500` öffnen und prüfen:

- Import-Seite öffnen (kein Crash bei leerer/gefilterter Template-Liste).
- `/preview` mit einem Monat öffnen; Nettovermögen, Checking und Monatsrückblick zeigen plausible Werte.
- CSV-Export eines Postings: Datum entspricht dem Journal-Tag (kein Vortag).
- Retirement-Ziel: Gain bleibt nach Zinsbuchung plausibel.
- Bei fehlendem Yahoo-Kurs: keine Server-Panic, nur Warnung/leerer Kurs.

- [ ] **Step 6: Version auf 8.4 erhöhen**

Erst nachdem alle Tests und der Smoke-Test grün sind:

```bash
printf '8.4\n' > VERSION
npm run sync-version
```

Prüfen:

```bash
cat VERSION
grep -n '"productVersion"' desktop/wails.json
grep -n '^Version:' desktop/build/linux/DEBIAN/control
```

Expected:

```text
8.4
"productVersion": "8.4",
Version: 8.4
```

Dann separat committen:

```bash
git add VERSION desktop/wails.json desktop/build/linux/DEBIAN/control
git commit -m "chore(release): bump version to 8.4"
```

- [ ] **Step 7: Offene Doku-Änderungen committen (nur falls vorhanden)**

```bash
git add docs/reference/config.md docs/research/2026-10-25-paisa-forks-review.md
git diff --cached --quiet || git commit -m "docs: document fork-findings config and verification"
```

Falls Task 4/5 die Dokumentation bereits vollständig committet haben, ist dieser Schritt ein No-op.

---

## Follow-up Plans (nicht in diesem Branch)

Diese P2-Themen sind eigenständige Subsysteme und brauchen jeweils einen eigenen Plan, nachdem die P1-Änderungen gemergt und die Monatsrückblick-Arbeit abgeschlossen ist:

1. **Monatsrückblick-Insights** – Sarath `427a8b94`, `4c00145b`, `e96e314b`, `3606a5aa`, `c37e1d84`; nextxm `910c0309`, `7c21e9de`, `2f42d50f`.
2. **Net-Worth/FX-Konsistenz** – nextxm `0de4ec5d`, `c1f2cb61`, `40c23d8c`, `38b6ce71`; 1Feng `59f7ff6b`, `1da0a8fb`; vivek `6c44eb6f`.
3. **Projektion/FIRE** – Sarath `1fd9353a`, `31d86c3a`, `4d67ae3e`, `48f697c3`; nextxm `434223c4`.
4. **Rebalancing** – Sarath `d0aae4ad`, `2658958e`, `6e5b9940`, `9b68574d`.
5. **Import-Framework** – 1Feng `e7636237`; swapnil `98cdce40`, `08673844`.
6. **Doctor/Datenqualität** – 1Feng `c3de5245`; Sarath `ba2f9456`; coffmant `bee64418` + `911d094c`.
7. **Git-Historie/Verschlüsselung** – d093w1z `989cee37`; gvijayakumar `c46598ad`.
8. **PWA/MCP/LLM** – nur nach Produktentscheidung; derzeit nicht empfohlen.

### Zurückgestellte P1-Politur

Diese Fixes aus dem Review sind bewusst **nicht** Teil des Korrektheits-Branches, weil sie rein kosmetisch bzw. import-spezifisch sind:

- **Gain-Chart-Margin** – `1Feng/paisa@4de04c31`: linken Rand an die längste Kontobezeichnung anpassen. Kleiner, separater UI-Fix; Ziel `src/lib/gain.ts:38` und neue `src/lib/gain_margin.ts`.
- **Chart-Width-Null-Guards** – `1Feng/paisa@1404e357` + `257b131d`: `chartWidth`-Helper für Chart-Container. Nur nötig, wenn Charts in versteckten Tabs weiterhin crashen.
- **Import-Dezimalen/Sonderfälle** – `matthiasdebernardini/paisa@7c89e0be`: exakte Dezimalen, wissenschaftliche Notation, Spalten jenseits Z, Template-Lint. Größerer Eingriff in `internal/ledger/ledger.go` und `src/lib/spreadsheet.ts`; erst nach Task 10 separat planen.
