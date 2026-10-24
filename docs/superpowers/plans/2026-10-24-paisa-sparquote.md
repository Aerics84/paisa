# Paisa Savings-Rate Insights Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Die jährliche Investment-Ansicht unterscheidet künftig Gesamt-Sparüberschuss/-quote von der bereits vorhandenen Investitionsquote.

**Architecture:** Die Berechnung bleibt im Go-Backend bei den vorhandenen jährlichen Investmentkarten und nutzt deren bereits berechnete Nettoeinkommen und Nettoausgaben. Die bestehende Svelte-Ansicht zeigt beide Kennzahlen mit expliziten Bezeichnungen und Formelhinweisen; Ledger und Konfiguration bleiben unverändert.

**Tech Stack:** Go, `shopspring/decimal`, Svelte/TypeScript, Bun-Tests, vorhandene Paisa-API und MkDocs.

**Spec:** `docs/superpowers/specs/2026-10-24-paisa-finanz-einblicke.md` (V1: Jahresübersicht „Gesamt sparen vs. investieren“).

## Global Constraints

- Keine verpflichtende Broker-/Bank-Cloud-Anbindung; das Ledger bleibt die Datenquelle.
- Das Finanzjahr folgt `financial_year_starting_month`.
- Gesamt-Sparüberschuss = `net_income - net_expense`; Gesamt-Sparquote = `100 * surplus / net_income`, nur wenn `net_income > 0`.
- Die bisherige Quote `net_investment / net_income` bleibt erhalten und wird als **Investitionsquote** bezeichnet.
- Bei `net_income <= 0` werden beide Quoten als nicht verfügbar angezeigt; das bestehende API-Feld `savings_rate` bleibt unverändert.
- Steuern sind in `net_income` bereits abgezogen und dürfen in `net_expense` nicht ein zweites Mal abgezogen werden.
- Der Gesamtüberschuss beschreibt Einkommen minus gebuchte Ausgaben, nicht Bankguthabenänderung oder vollständigen Vermögenszuwachs.
- `period_complete` beschreibt das kalendarische Ende des Finanzjahres; `has_activity` beschreibt vorhandene relevante Buchungen. Daraus wird keine Vollständigkeit der realen Buchführung abgeleitet.
- Einkommen-/Ausgabenbuchungen müssen Jahreskarten erzeugen, auch wenn keine Investment-Postings vorliegen.
- Negative Überschüsse werden nicht gekappt; laufende Geschäftsjahre bleiben als laufend/YTD erkennbar.
- API-Dezimalwerte werden nicht gerundet; die UI zeigt Prozentwerte mit `formatFloat(value, 2)` in der konfigurierten Locale.
- Keine neuen Abhängigkeiten, keine Anlageempfehlungen und keine Änderungen am Journalformat.
- Neue UI-Texte bleiben Englisch wie die bestehende Produktoberfläche.

## Review Focus

- **Nettoeinkommen null oder negativ:** beide Quoten sind nicht verfügbar und werden nicht als 0 % dargestellt.
- **Ausgaben über Nettoeinkommen:** der negative Überschuss und die negative Quote bleiben erhalten.
- **Steuern:** `net_tax` wird nur im Nettoeinkommen berücksichtigt und nicht erneut aus dem bereits steuerbereinigten Einkommen abgezogen.
- **Laufendes Finanzjahr:** Karte zeigt den Zeitraum als laufend/YTD und suggeriert keinen vollständigen Jahresvergleich.
- **Kein Investment, aber Einkommen/Ausgaben:** Jahreskarte wird trotzdem erstellt; `net_investment` ist 0 und die Investitionsquote ist bei positivem Nettoeinkommen 0 %.
- **Zeitraum ohne relevante Buchungen:** wird als „No recorded activity“ statt als belegtes 0-%-Jahr dargestellt; nicht erfasste Vorgänge können aus dem Ledger nicht erkannt werden.

---

## Datei- und Verantwortungsübersicht

- `internal/server/investment_metrics.go` – reine, testbare Berechnung von Gesamtüberschuss und Gesamt-Sparquote.
- `internal/server/investment.go` – ergänzt die API-Jahreskarte um die neuen Werte und Zeit-/Aktivitätsstatus; Jahreskarten dürfen nicht von Investment-Postings abhängen; bestehendes Feld `savings_rate` bleibt aus Kompatibilitätsgründen erhalten.
- `internal/server/investment_metrics_test.go` – Grenzfall- und Formeltests für die Berechnung.
- `internal/server/investment_test.go` – Jahreskartenintegration: Nettoeinkommen, Steuerabzug, Buchungen ohne Investment-Postings, Aktivitätsstatus und YTD-Status.
- `src/lib/utils.ts` – TypeScript-Typ für die zusätzlichen API-Felder.
- `src/lib/investment.ts` – Jahreskarten: neue Gesamt-Sparwerte, bestehende Kennzahl als Investitionsquote beschriften.
- `src/lib/investment.test.ts` – Darstellung der beiden Kennzahlen und des „nicht verfügbar“-Zustands.
- `docs/reference/investment.md` – Definitionen und Grenzen der Kennzahlen.
- `mkdocs.yml` – Eintrag der neuen Referenzseite.
- `tests/fixture/*/investment.json` – Regression-Snapshots der vorhandenen `/api/investment`-Antworten.

## Task 1: Gesamt-Sparmetriken als reine Go-Berechnung

**Files:**

- Create: `internal/server/investment_metrics.go`
- Test: `internal/server/investment_metrics_test.go`

**Interfaces:**

- Produces: `SavingsMetrics` mit `Surplus decimal.Decimal`, `OverallRate decimal.Decimal` und `OverallRateAvailable bool`.
- Produces: `calculateSavingsMetrics(netIncome, netExpense decimal.Decimal) SavingsMetrics` in `internal/server/investment_metrics.go`.
- `OverallRateAvailable` ist genau dann `true`, wenn `netIncome > 0`; `Surplus` wird unabhängig davon berechnet.

- [ ] **Step 1: Failing tests ergänzen** in `internal/server/investment_metrics_test.go`.
  - `TestCalculateSavingsMetricsReturnsSurplusAndOverallRate`: bei Nettoeinkommen 900 und Nettoausgaben 600 muss der Überschuss 300 und die Gesamtquote 33,333… % betragen.
  - `TestCalculateSavingsMetricsPreservesNegativeSurplus`: bei 900 Einkommen und 1.000 Ausgaben muss der Überschuss -100 und die Quote kleiner als 0 sein.
  - `TestCalculateSavingsMetricsMarksNonPositiveIncomeUnavailable`: bei Einkommen 0 oder -100 muss die Quote nicht verfügbar sein; die Funktion darf weder panicen noch `NaN`/`Inf` erzeugen.
- [ ] **Step 2: Tests rot ausführen.**

  Run: `go test ./internal/server -run TestCalculateSavingsMetrics -count=1`

  Expected: FAIL, weil `SavingsMetrics` und `calculateSavingsMetrics` noch nicht existieren.

- [ ] **Step 3: Minimale Berechnung implementieren** in `internal/server/investment_metrics.go`.
  - `Surplus = netIncome - netExpense`.
  - Bei `netIncome > 0`: `OverallRate = Surplus / netIncome * 100`, `OverallRateAvailable = true`.
  - Sonst `OverallRateAvailable = false`; keine künstliche 0-%-Quote erzeugen.
- [ ] **Step 4: Tests grün ausführen.**

  Run: `go test ./internal/server -run TestCalculateSavingsMetrics -count=1`

  Expected: PASS für positive, null/negative und defizitäre Fälle.

- [ ] **Step 5: Commit.**

  `git add internal/server/investment_metrics.go internal/server/investment_metrics_test.go`

  `git commit -m "feat: calculate total savings rate"`

## Task 2: API-Jahreskarten und Oberfläche ergänzen

**Files:**

- Modify: `internal/server/investment.go`
- Test: `internal/server/investment_test.go`
- Modify: `src/lib/utils.ts`
- Modify: `src/lib/investment.ts`
- Create: `src/lib/investment.test.ts`

**Interfaces:**

- Consumes: `calculateSavingsMetrics` aus Task 1.
- Produces: `overall_surplus`, `overall_savings_rate`, `overall_savings_rate_available`, `period_complete` und `has_activity` je `InvestmentYearlyCard`.
- Existing API field `savings_rate` bleibt bestehen und bedeutet weiterhin `net_investment / net_income`; die UI bezeichnet es künftig als **Investitionsquote**.

- [ ] **Step 1: Failing Jahreskarten- und Darstellungstests schreiben.**
  - In `internal/server/investment_test.go`, `TestComputeInvestmentYearlyCardSubtractsTaxesOnce`: bei 1.000 Bruttoeinkommen, 100 Steuer und 600 Nettoausgaben muss `net_income` 900, `overall_surplus` 300 und `overall_savings_rate` 33,333… % sein.
  - In `internal/server/investment_test.go`, `TestComputeInvestmentYearlyCardKeepsInvestmentRateSeparate`: bei 200 Nettoinvestition und 900 Nettoeinkommen muss `savings_rate` weiterhin 22,222… % sein.
  - In `internal/server/investment_test.go`, `TestComputeInvestmentYearlyCardMarksCurrentFinancialYearIncomplete`: mit kontrollierter Uhr innerhalb eines Finanzjahres muss `period_complete` false sein; abgeschlossene Vorjahre müssen true sein.
  - In `internal/server/investment_test.go`, `TestComputeInvestmentYearlyCardWorksWithoutInvestmentPostings`: vorhandene Income-/Expense-Postings erzeugen eine Karte auch bei leerer Investment-Posting-Liste.
  - In `internal/server/investment_test.go`, `TestComputeInvestmentYearlyCardMarksEmptyPeriodAsNoActivity`: ein Zeitraum ohne relevante Postings setzt `has_activity` false; ein belegtes Defizit setzt es true.
  - In `src/lib/investment.test.ts`, rendere eine Jahreskarte und prüfe, dass „Total Surplus“, „Total Savings Rate“ und „Investment Rate“ erscheinen.
  - Bei `net_income <= 0` muss die Gesamt- und Investitionsquote als „N/A“ erscheinen, nicht als `0 %`.
  - Bei `period_complete === false` muss die Karte den Zeitraum als laufend/YTD kennzeichnen.
  - Bei `has_activity === false` muss die Karte „No recorded activity“ zeigen, nicht einen scheinbar bestätigten Nullüberschuss.
  - Mit `en-US`-Locale muss ein Wert von 33,333… % als `33.33%` erscheinen; API-/Backendwerte bleiben ungerundet.
- [ ] **Step 2: Gezielt rot ausführen.**

  Run: `go test ./internal/server -run 'Test(CalculateSavingsMetrics|ComputeInvestmentYearlyCard)' -count=1`

  Run: `bun test --preload ./src/happydom.ts src/lib/investment.test.ts`

  Expected: FAIL, weil Berechnung, API-Felder und UI-Ausgabe noch fehlen.

- [ ] **Step 3: Jahreskarte erweitern** in `internal/server/investment.go`.
  - `InvestmentYearlyCard` erhält die neuen JSON-Felder.
  - `period_complete` ist false, wenn das Finanzjahrende nach `utils.EndOfToday()` liegt; sonst true.
  - `has_activity` ist true, wenn im Finanzjahr mindestens eine Income-, Expense-/Tax- oder Investment-Buchung verarbeitet wurde.
  - `computeInvestmentYearlyCard` übergibt vorhandenes `netIncome` und `netExpense` an `calculateSavingsMetrics`.
  - Entferne die frühe Rückgabe bei `len(assets) == 0`; Einkommen und Ausgaben reichen aus, um Jahreskarten zu berechnen. Die bestehende Rückgabe bei vollständig leerem Journal bleibt erhalten.
  - Bestehendes `SavingsRate` nicht umdeuten oder entfernen.
- [ ] **Step 4: TypeScript-Typ und Jahreskartenanzeige ergänzen.**
  - `InvestmentYearlyCard` in `src/lib/utils.ts` um die neuen Felder ergänzen.
  - In `src/lib/investment.ts` die bisherige Beschriftung „Savings Rate“ in „Investment Rate“ ändern.
  - „Total Surplus“ und „Total Savings Rate“ zusätzlich anzeigen; bei `net_income <= 0` beide Quoten als „N/A“ ausgeben.
  - Bei `period_complete === false` die Karte als laufendes Jahr/YTD kennzeichnen; bei `has_activity === false` „No recorded activity“ anzeigen.
  - Einen kompakten Info-Tooltip statt eines dauerhaften Absatzes verwenden. Er erklärt die Formel und sagt ausdrücklich: „Calculated from recorded ledger postings; this is not the change in bank balance.“
  - UI-Rundung mit vorhandener `formatFloat(value, 2)` umsetzen; Backend/API-Werte bleiben ungerundet.
- [ ] **Step 5: Tests grün ausführen.**

  Run: `go test ./internal/server -run 'Test(CalculateSavingsMetrics|ComputeInvestmentYearlyCard)' -count=1`

  Expected: PASS für Berechnung, Steuerabzug, Investitionsquote und Finanzjahrstatus.

  Run: `bun test --preload ./src/happydom.ts src/lib/investment.test.ts`

  Expected: PASS; beide Quoten sind unterscheidbar und fehlende Quoten werden nicht als Null dargestellt.

- [ ] **Step 6: Commit.**

  `git add internal/server/investment.go src/lib/utils.ts src/lib/investment.ts src/lib/investment.test.ts`

  `git commit -m "feat: show total and invested savings rates"`

## Task 3: Kennzahlen dokumentieren und API-Regression absichern

**Files:**

- Create: `docs/reference/investment.md`
- Modify: `mkdocs.yml`
- Update: `tests/fixture/*/investment.json` – nur Fixtures, deren Antwort sich geändert hat.

- [ ] **Step 1: Referenzdokument verfassen.**
  - Formeln und Vorzeichen erklären.
  - Festhalten, dass `net_income` Paisas bestehende Definition nutzt (Salary + sonstige Income-Postings minus `Expenses:Tax`); „sonstige Income“ kann auch Anlageerträge enthalten.
  - Festhalten, dass der Überschuss ein rechnerischer Wert aus erfassten Postings ist, keine Bankguthabenänderung und keine vollständige Vermögensüberleitung; V1 zerlegt ihn nicht in Cashbestand, Tilgung und andere Vermögensbewegungen.
  - Erklären, dass `period_complete` nur das Kalender-/Finanzjahrende beschreibt und `has_activity` nur vorhandene Ledger-Postings; Paisa kann nie erfasste Buchungen nicht erkennen.
  - Hinweis: laufende Finanzjahre sind YTD; historische Kennzahlen sind keine Prognose.
- [ ] **Step 2: Seite in `mkdocs.yml` unter Reference eintragen.**
- [ ] **Step 3: Investment-Regression-Snapshots aktualisieren und diff prüfen.**

  Run (PowerShell): `$env:REGENERATE="true"; bun test tests/regression.test.ts`

  Expected: `/api/investment`-Snapshots enthalten ausschließlich die erwarteten Zusatzfelder; keine unbeabsichtigten Änderungen an anderen Endpunkten.

- [ ] **Step 4: Gesamte relevante Prüfung ausführen.**

  Run: `go test ./...`

  Run: `bun test --preload ./src/happydom.ts src/lib/investment.test.ts`

  Run: `bun test tests/regression.test.ts`

  Run: `npm run check`

  Run: `npm run build`

  Expected: alle Tests, Typprüfung und Produktionsbuild erfolgreich.

- [ ] **Step 5: Commit.**

  `git add docs/reference/investment.md mkdocs.yml tests/fixture`

  `git commit -m "docs: explain investment and total savings rates"`

## Folgepläne – bewusst getrennte Arbeitspakete

Nach Freigabe und Review von V1 sollten für diese unabhängigen Bereiche separate technische Pläne entstehen:

1. Geldfluss-/Sparüberschussansicht im `/preview`-Dashboard.
2. Positionsübersicht mit TER, Marktwert, Einstand, XIRR, Dividenden und Datenfrische.
3. Datenfeasibility-Audit für historische Bewertungen, Zahlungsstromdaten, Kurslücken, Währungen und Granularität; erst danach TWROR, Benchmarkvergleich, monatliche Heatmap und Drawdown planen.
4. Fonds-Look-through nach Land und Unternehmensgröße mit Abdeckungsgrad.
5. Deutsche Vorabpauschale nach eigener steuerlicher Fachspezifikation.

RSI/MACD-Scanner, Buy/Sell-Marker und verpflichtender Broker-Sync sind kein Bestandteil dieser Roadmap.
