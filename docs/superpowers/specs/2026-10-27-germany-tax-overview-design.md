# Germany Tax Overview — Design

Date: 2026-10-27
Status: Draft for approval

## Intent

At the start of the year, when filing the German income tax return for the
previous year, the user wants to open one page in Paisa, pick the year, and see
every posting that is relevant or potentially deductible — grouped the way the
tax return is structured, so the numbers can be read off and entered into the
tax software.

The page is a handover sheet, not an editor and not a tax calculator.

## Goals

- One annual overview page for Germany, including capital income (Anlage KAP).
- Year selector, defaulting to the previous calendar year.
- Group postings by tax-return section and category; show section and category
  totals plus the underlying postings (date, payee, account, amount).
- Detect candidates in two ways: account rules from config (covers ordinary
  bookings retroactively) and an explicit `tax_category` tag on the expense
  posting (covers exceptions and mixed accounts).
- Export the selected year as CSV.
- Make data-quality problems visible without dumping unrelated bookings onto
  the page.
- Show a compact completeness check so missing account rules are not invisible.

## Non-goals

- No tax-law calculation: no Arbeitnehmer-Pauschbetrag, no zumutbare Belastung,
  no 20 % credit, no allowance handling, no partial-deduction math. The tax
  software does that.
- No editing of postings on this page.
- No transaction-level list of untagged postings. The page shows matched
  candidates plus an aggregated coverage check by account.
- No automatic guessing from payee names.
- No automatic matching of refunds to the original expense across years.
- No support for income types other than capital income (salary is reported
  automatically, no rental income exists).

## Anti-goals

- A page that silently omits a deductible posting because a tag is missing or
  malformed.
- A page that claims an amount is deductible when it is only a candidate.
- A page that adds expense candidates and capital income into one misleading
  total.
- A page that asserts a tax-year rule for refunds or silently moves a refund into
  the wrong year.
- A model that depends on the user's current (incomplete) tagging habits.

## Decisions taken by the user

- The overview must include capital income, not just deductions.
- Structure follows the tax return sections (mockup A), because that is easiest
  to transfer into the tax software.
- No tax logic in Paisa.
- Existing `TaxRelevant` tags will be migrated manually later; the model must be
  correct independently of the current tagging state.
- `Expenses:Household:Goods` and `Expenses:Sports:Equipment` are private and must
  not be mapped as deductible by default.
- No transaction list of unclassified bookings, but a compact completeness check
  for unmatched postings grouped by account is wanted.
- Summary metrics must keep expense candidates and capital income separate.
- Refunds need a visible marker and a cross-year hint; no automatic tax handling.
- Capital income values must be labelled as raw ledger values, not as finished
  Anlage KAP field values.
- Empty sections stay visible but collapsed.

## Calls made by the implementer (review these)

1. Page content is German, because the labels are the legal terms from the
   German return and the transfer target is a German tax program. Global app
   chrome (navigation, menu label `Overview`) stays English. Visibility is gated
   by `tax_regime: germany`, not by `locale`.
2. Canonical ledger key is `tax_category`; the legacy `TaxRelevant` tag is only
   reported as a migration diagnostic, never applied.
3. Category catalog is hardcoded for the Germany regime (labels, section
   assignment, form reference). Only the account mapping is user configuration.
4. `tax_category` on the expense posting is canonical. A tag on any other
   posting in the same transaction is accepted only when the transaction has
   exactly one expense posting; otherwise it is a diagnostic.
5. The value `none` on `tax_category` explicitly excludes an account-mapped
   posting.
6. The page is self-contained: the Anlage KAP section shows raw inputs and line
   items, so it does not link to the existing Capital Income page.
7. Account matching is prefix-based on account boundaries: `Expenses:Health`
   matches `Expenses:Health` and `Expenses:Health:*`, but not
   `Expenses:HealthSomething`. Longest matching rule wins; not glob-based.
8. Route is `/more/tax/overview`, nav label `Overview`, shown first under Tax for
   the Germany regime. The `preview` route tree gets the same page.
9. Capital income uses a behavior-preserving extraction of the existing matcher.
   Fixing mixed-commodity accounts is a separate follow-up, not part of this
   change (the inspected ledger uses one account per ISIN).
10. Realized gains and losses are summed per matched buy/sell pair (positive and
    negative separately), not per account net sign. This is what Anlage KAP
    needs and may differ from the existing Capital Income summary.
11. Dividends are shown as booked. Paisa does not gross them up. If dividend
    income exists without `Expenses:Broker:Taxes` postings, a diagnostic says the
    booked amount may be net of withholding.
12. The completeness check reports **unmatched postings grouped by account**
    (`accounts_with_unmatched_postings`, `unmatched_posting_count`), not
    transactions. It is labelled as a coverage limit, not as a list of suspected
    deductions. The account list is collapsed by default.
13. Negative matched postings are shown with a `Negative` marker and reduce the
    category total. Negative postings in the selected year get a neutral
    diagnostic that mentions refunds and cross-year treatment without asserting
    a rule. Paisa does not link the posting to an earlier expense automatically.
14. The summary strip separates `Erkannte Ausgaben`, `Kapitalerträge (gebucht)`,
    `Realisierte Gewinne/Verluste`, `Erkannte Aufwandsposten`, and `Hinweise`.
    There is no combined total. Booked capital income (dividends + interest) and
    realized gains/losses stay separate. The matched count covers expense
    postings only.
15. Sections with matches are expanded; empty sections are collapsed with a clear
    `Keine Buchungen` label. Capital income stays a separate card.
16. All Anlage KAP values carry the label `aus dem Ledger, kein
Steuererklärungswert`. The tax row is called `Gebuchte Steuern (Art nicht
klassifiziert)`, not `Quellensteuer`, because Paisa cannot distinguish
    foreign withholding from German capital gains tax.

## Data model

### Posting

Add two fields to `internal/model/posting/posting.go`:

```go
TaxCategory       string `json:"tax_category"`        // canonical tag value
TaxCategoryLegacy string `json:"tax_category_legacy"` // legacy TaxRelevant value, diagnostics only
```

Both are populated at ingest. The postings table is rebuilt on ledger reload, so
no data migration is needed; GORM auto-migration adds the columns.

### Ingest

All three ledger backends must read the new keys:

- `ledger` CLI: extend the `--csv-format` with
  `%(quoted(tag('tax_category')))` and `%(quoted(tag('TaxRelevant')))`.
- `hledger`: read `tax_category` / `TaxRelevant` from **posting** tags only
  (`ptags`), not transaction tags (`ttags`). Fix the existing bug in
  `buildHLedgerPostings` where `break` is inside the tag loop and only the first
  tag is ever examined.
- `beancount`: add `META('tax_category')` to the `bean-query` select. Use
  `META` (posting metadata), not `ANY_META` (posting or transaction metadata), so
  a transaction-level tag cannot leak onto every expense posting. Legacy
  `TaxRelevant` cannot exist as beancount metadata (uppercase keys are invalid),
  so it stays empty there.

Only the first value of a repeated tag is used. All three backends therefore
expose posting-level tags only; the sibling rule below is deterministic, and no
per-posting origin field is needed.

### Resolution order (report time)

For each `Expenses:*` posting in the selected year:

1. `TaxCategory == "none"` → excluded.
2. `TaxCategory` set and known → included with source `tag`.
3. `TaxCategory` set and unknown → diagnostic, excluded.
4. No tag on the expense posting, but a non-expense posting (bank, asset,
   liability) in the same transaction has `TaxCategory`:
   - exactly one expense posting in the transaction → include with source
     `transaction`;
   - zero or more than one → diagnostic "ambiguous tag on shared transaction",
     excluded. Zero means the transaction has no expense posting at all (for
     example a capitalized asset); the page does not do depreciation.
     This is the compatibility path for tags written on the bank line.
5. No tag → match the posting account against the configured account rules;
   longest matching prefix wins; source `account`.
6. No match → not shown.

`TaxCategoryLegacy` never includes a posting; it only produces a diagnostic with
a count and instructions to migrate to `tax_category`.

### Amounts and refunds

Use the posting's `Amount` in the default currency, as Paisa already computes it.
Negative expense postings (refunds) reduce the category total. Postings with a
zero amount produce a diagnostic.

A negative matched posting gets a `Negative` marker in the posting table (not
`Refund`, because it may be a correction). If the selected year contains at least
one negative matched posting, a compact diagnostic says:

> "Negative expense postings detected. These may represent refunds or
> corrections. The tax treatment of refunds received in a different year depends
> on the type of expense and the circumstances. Review them manually; Paisa does
> not determine the correct tax year."

The wording is deliberately neutral. Paisa does not encode a German tax rule for
refunds: the correct treatment differs between expense types (for example work
expenses versus health costs treated as extraordinary burdens). The diagnostic is
expandable and lists at most five examples with date, payee, account and amount.
No automatic cross-year linking: payee/amount matching would be a heuristic and
could silently move money into the wrong year.

### Completeness check

Missing account rules must not be invisible, but a full list of private spending
would be noise. The report therefore returns an aggregated coverage object:

- expense **postings** in the selected year that were not matched by a rule, a
  posting tag, or the sibling rule;
- grouped by account, with the number of unmatched postings per account.

An account with one tagged posting and 36 untagged postings still appears with
36 unmatched postings. Coverage counts postings, not whether an account was
touched once, so the API field names talk about postings rather than accounts.

The UI shows one collapsed row, **without a total amount**:

> "Coverage: 642 postings in 44 expense accounts are unclassified. That does not
> mean they are deductible — only that Paisa cannot classify them."

The total amount is deliberately omitted: most of it is rent, groceries and
other clearly private spending, and a euro figure would invite the wrong kind of
attention. Expanding shows account names with posting counts, no individual
transactions and no amounts. This is a coverage statement, not a candidate list,
and the wording must keep that distinction visible. In the inspected ledger the
real figure is around 44 accounts / 640 postings, not a handful; that is expected
and is why the control is collapsed.

Note that legacy `TaxRelevant` tags do not count as matched coverage, because
they are not applied.

## Category catalog (Germany)

Hardcoded in Go, similar to `ScheduleAL` sections.

| Section key      | UI label (de)                  | Form                   | Categories                                                                                                                              |
| ---------------- | ------------------------------ | ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `work_expenses`  | Werbungskosten                 | Anlage N               | `work_equipment`, `work_training`, `work_travel`, `work_commute`, `work_home_office`, `work_phone_internet`, `tax_advice`, `work_other` |
| `insurance`      | Vorsorgeaufwand                | Anlage Vorsorgeaufwand | `health_insurance`, `other_insurance`                                                                                                   |
| `extraordinary`  | Außergewöhnliche Belastungen   | Hauptvordruck          | `health`                                                                                                                                |
| `household`      | Haushaltsnahe Dienstleistungen | Hauptvordruck          | `household_service`, `craftsman`                                                                                                        |
| `donations`      | Spenden und Mitgliedsbeiträge  | Hauptvordruck          | `donation`                                                                                                                              |
| `capital_income` | Kapitalerträge                 | Anlage KAP             | generated, not taggable                                                                                                                 |

Category labels (German UI):

- `work_equipment` — Arbeitsmittel
- `work_training` — Fortbildung und Fachliteratur
- `work_travel` — Reisekosten
- `work_commute` — Wege zur Arbeit
- `work_home_office` — Homeoffice
- `work_phone_internet` — Telefon und Internet
- `tax_advice` — Steuerberatung und -software
- `work_other` — Sonstige Werbungskosten
- `health_insurance` — Kranken- und Pflegeversicherung
- `other_insurance` — Sonstige Versicherungen
- `health` — Krankheitskosten
- `household_service` — Haushaltsnahe Dienstleistungen
- `craftsman` — Handwerkerleistungen
- `donation` — Spenden und Mitgliedsbeiträge

Sections are always rendered, in the order above. Sections with matches are
expanded; empty sections are collapsed and labelled `Keine Buchungen`. Capital
income is rendered last as its own card.

## Configuration

New top-level key, with defaults applied for the Germany regime:

```yaml
tax_deductions:
  accounts:
    - account: Expenses:Health:Insurance
      category: health_insurance
    - account: Expenses:Health
      category: health
    - account: Expenses:Insurance
      category: other_insurance
    - account: Expenses:Tax:Preparation
      category: tax_advice
```

Rules:

- Plain account prefix on account boundaries; `Expenses:Health` matches
  `Expenses:Health` and `Expenses:Health:*`, but not `Expenses:HealthSomething`.
- Longest matching rule wins.
- Configuration replaces the default list; an empty list disables account
  matching (tags still work).
- `TaxDeductionAccounts()` returns a copy so callers cannot mutate shared
  defaults or the active config.
- Unknown category values in config are rejected with a config validation error.

Deliberately not mapped by default: `Expenses:Household:*`,
`Expenses:Sports:*`, `Expenses:Car:*`, `Expenses:Transportation`,
`Expenses:Phone`, `Expenses:Utilities:*`, `Expenses:Travel:*`. These need an
explicit tag when relevant. Phone/internet/commute/travel amounts usually require
a share or a different calculation in the tax software, so tagging them is a
conscious act.

## Ledger tagging reference

ledger / hledger (posting tag, canonical placement):

```ledger
2025/09/09 * PayPal ; Bürostuhl
    ; PayPal Europe S.a.r.l. et Cie S.C.A
    ; 1044657181450/PP.6635.PP/. aeris GmbH, Ihr Einkauf bei aeris GmbH
    Assets:Checking:ING                      -699,00 EUR
    Expenses:Household:Goods                  699,00 EUR ; tax_category: work_equipment
```

beancount:

```beancount
2025-09-09 * "PayPal" "Bürostuhl"
  Assets:Checking:ING       -699.00 EUR
  Expenses:Household:Goods   699.00 EUR
    tax_category: "work_equipment"
```

## API

`GET /api/tax_overview?year=2025`

Response (abridged):

```json
{
  "years": ["2020", "2021", "2022", "2023", "2024", "2025", "2026"],
  "selected_year": "2025",
  "sections": [
    {
      "key": "work_expenses",
      "label": "Werbungskosten",
      "form": "Anlage N",
      "total": 722.79,
      "categories": [
        {
          "key": "work_equipment",
          "label": "Arbeitsmittel",
          "total": 699,
          "postings": [
            {
              "transaction_id": "…",
              "date": "2025-09-09T00:00:00+02:00",
              "payee": "PayPal | Bürostuhl",
              "account": "Expenses:Household:Goods",
              "amount": 699,
              "note": "1044657181450/PP.6635.PP/. aeris GmbH, Ihr Einkauf bei aeris GmbH",
              "file_name": "main.ledger",
              "line": 2012,
              "source": "tag"
            }
          ]
        }
      ]
    }
  ],
  "summary": {
    "expense_candidates": 722.79,
    "matched_expense_posting_count": 3,
    "capital_income_booked": 118.06,
    "capital_income_calculated": 0,
    "diagnostic_count": 1
  },
  "coverage": {
    "accounts_with_unmatched_postings": 44,
    "unmatched_posting_count": 642,
    "accounts": [{ "account": "Expenses:Household:Goods", "unmatched_posting_count": 36 }]
  },
  "capital_income": {
    "realized_gains": 0,
    "realized_losses": 0,
    "dividends": 118.06,
    "interest": 0,
    "taxes_booked": 0,
    "sales": 0,
    "accounts": []
  },
  "diagnostics": [
    {
      "level": "warning",
      "summary": "6 Postings verwenden noch das Legacy-Tag TaxRelevant",
      "details": "Bitte auf tax_category umstellen. Legacy-Tags werden nicht angewendet."
    }
  ]
}
```

All monetary fields are JSON numbers, not strings. `paisa.go` currently sets
`decimal.MarshalJSONWithoutQuotes = true`; this change moves that setting into
`internal/server` so every program that imports the server package (CLI serve,
desktop, server tests) shares the contract, and adds an API-level JSON test. The
test proves the server-package contract; it does not claim that unrelated
packages marshaling `decimal.Decimal` get the same setting.

- `years` comes from distinct valid posting years in the database (1900–2100,
  ignoring zero dates), plus the previous calendar year if missing, sorted
  ascending. The database has no non-zero date constraint, so `0001` must never
  reach the selector.
- `selected_year` defaults to the previous calendar year when the query
  parameter is absent. A four-digit year in 1900–2100 is accepted, even when it
  has no postings; it returns all sections with zero totals and an empty capital
  income block. Anything else (`abc`, `25`, `20255`) returns HTTP 400 with an
  error body, rather than a plausible-looking empty overview for a nonsense year.
- The UI only sends years from `years`, so the 400 path is an API guard, not a
  normal user flow.
- `source` is one of `tag`, `transaction`, `account`. `transaction` means the tag
  was found on a non-expense posting of the same transaction and resolved through
  the single-expense rule.
- `summary` is computed server-side so the frontend never adds expense candidates
  and capital income together. `matched_expense_posting_count` counts matched
  **expense** postings only; capital income postings are not included.
- `capital_income_booked` is the sum of **booked** dividend and interest
  postings only. `capital_income_calculated` is realized gains minus realized
  losses from matched buy/sell pairs. They are deliberately separate: one is
  booked, the other is derived. Do not add them into a single summary figure.
- `coverage` is computed from `Expenses:*` postings in the selected year that are
  not matched by a rule, a tag, or the sibling rule. It is a coverage metric, not
  a candidate list. The account list is sorted by unmatched posting count,
  descending. No amounts are returned, to keep it from reading like a deduction
  list.
- `taxes_booked` replaces `withholding_tax`; the API name must not imply a tax
  type.

### Capital income data

Reuse the existing buy/sell matching from `internal/server/germany_tax.go`, but
surface raw inputs only:

- realized gains: sum of positive `RealizedGain` values across matched buy/sell
  pairs, before partial exemption and allowance;
- realized losses: sum of negative `RealizedGain` values across matched pairs,
  shown as a positive number;
- dividends from `Income:Dividend*` (booked amount);
- interest from `Income:Interest*` (booked amount);
- taxes booked from `Expenses:Broker:Taxes*` (type not classified);
- number of realized sales: distinct non-empty sell `TransactionID` values; a sell
  with an empty ID counts individually, so empty IDs never collapse into one sale.

Gains and losses are split per matched pair, not by the net sign of an account.
That matches the two lines of Anlage KAP and may differ from the existing Capital
Income summary, which nets per account.

No rates, no allowance, no tax amounts. Extract the shared purchase/sale matching
into a helper so `GetGermanyTax` and the overview do not diverge. The extraction
is behavior-preserving: it keeps today's per-account matching and output. The
existing matcher assumes one commodity per account, which holds for the inspected
ledger (`Assets:Equity:<ISIN>`). Mixed-commodity accounts are a known limitation
and a separate follow-up, not part of this change.

Note: the existing Capital Income page only creates year cards for years with
realized sales. That is why only 2021 appears in the current ledger. The overview
must include years with only dividends/interest; do not copy that limitation.

Dividends are not grossed up. The inspected Trading212 imports book the cash
amount received and no withholding posting, so "Dividends" means "as booked". If
dividend income exists for a year without any `Expenses:Broker:Taxes` postings,
emit a diagnostic that the amount may be net of foreign withholding tax and
should be checked against the broker statement.

All five values are raw ledger aggregates. None of them is guaranteed to be the
value of a specific Anlage KAP line. The UI must say so at card level: realized
gains are not automatically "Gewinne aus Kapitalanlagen", dividends may be net,
and booked taxes may be foreign withholding, German capital gains tax, or
something else. Paisa does not classify tax types and does not model foreign tax
credits.

## Language and visibility

Paisa has no UI localization: `locale` only controls number and date formatting.
The new page is therefore a deliberate, page-local exception.

- **German:** page title, section and category labels, buttons, empty states,
  source badges and diagnostics. These are the terms the user sees in WISO/Elster
  and in the tax form, so translating them adds a mental translation step.
- **English:** global navigation and menu structure (`Tax → Overview`), because
  those belong to the app shell, plus API fields and code identifiers.
- **Visibility:** gated by `tax_regime: germany` through the existing
  `supportsGermanyTaxFeatures()` check, exactly like the Capital Income page.
  Do **not** gate on `locale`. `locale: de-DE` vs `locale: en-GB` is a formatting
  choice; a German tax return is still a German tax return. The repository's
  `region-aware-taxation` spec requires tax behavior to be selected by the
  explicit tax regime, not inferred from formatting or currency.
- **Future:** if English labels are ever needed for the Germany regime, add a
  real translation layer. Do not hide the page based on `locale`.

## UI

Route: `src/routes/(app)/more/tax/overview/+page.svelte` (plus the duplicate under
`src/routes/preview/more/tax/overview/`). All user-facing text on the page is
German; the labels below are the source of truth for the plan.

Layout:

```text
Steuerübersicht
[Steuerjahr 2025 ▾]                         [CSV exportieren]
Ledger-Kandidaten, keine bestätigten Absetzbeträge.

Erkannte Ausgaben   Kapitalerträge (gebucht)   Realisierte Gewinne/Verluste   Erkannte Aufwandsposten   Hinweise
1.121,58 €          118,06 €                   0,00 €                         22                        3

Werbungskosten — Anlage N                        722,79 €  3 Postings  ▾
  Arbeitsmittel   699,00 €
    Datum | Empfänger | Konto | Betrag        [Tag]
  Steuerberatung und -software  22,79 €
  Sonstige Werbungskosten  1,00 €

Außergewöhnliche Belastungen — Hauptvordruck     398,79 €  17 Postings ▸
Vorsorgeaufwand — Anlage Vorsorgeaufwand            0,00 €  0 Postings  ▸  (eingeklappt)
Haushaltsnahe Dienstleistungen — Hauptvordruck      0,00 €  0 Postings  ▸  (eingeklappt)
Spenden und Mitgliedsbeiträge — Hauptvordruck       0,00 €  0 Postings  ▸  (eingeklappt)

Abdeckung: 642 Postings in 44 Ausgabenkonten sind nicht zugeordnet. ▸

Kapitalerträge — Anlage KAP
  aus dem Ledger, kein Steuererklärungswert
  Berechnet:
    Realisierte Gewinne   0,00 €
    Realisierte Verluste  0,00 €
  Gebucht:
    Dividenden           118,06 €
    Zinsen                0,00 €
    Gebuchte Steuern      0,00 €  (Art nicht klassifiziert)

[Hinweise, falls vorhanden]
```

Behavior:

- Summary metrics are five separate figures; no combined total. `Erkannte
Ausgaben` is labelled as the sum of matched expense postings, not as a
  deductible amount. `Erkannte Aufwandsposten` counts only expense postings,
  never capital income postings. Booked capital income (dividends + interest)
  and realized gains/losses are separate tiles; never add them. The tile label
  is `Realisierte Gewinne/Verluste`, not `Kapitalerträge (berechnet)`, because
  the value is not a tax calculation.
- Sections are cards/rows with total and posting count; click toggles the
  posting table. Sections with matches are expanded by default. Empty sections
  stay present but collapsed and marked `Keine Buchungen`.
- Posting table columns: Datum, Empfänger (transaction note as secondary line,
  truncated), Konto, Betrag. A small source badge shows `Tag`, `Transaktion` or
  `Konto`, matching the three resolution sources; negative amounts carry a
  `Negativ` badge. `Transaktion` means the tag sat on another posting of the same
  transaction (for example the bank line) and was resolved through the
  single-expense rule, so the user can see why the posting was found.
- Category rows are shown inside an expanded section with their own subtotal.
- The coverage row is collapsed by default and expands to account names and
  posting counts. It never lists amounts or individual transactions, so it does
  not read as a deduction list.
- Capital income is a separate card with two labelled groups — `Berechnet`
  (realized gains/losses) and `Gebucht` (dividends, interest, booked taxes) —
  plus the persistent label `aus dem Ledger, kein Steuererklärungswert`. It has
  no mixed total. Account-level realized gain rows are expandable when sales
  exist.
- Diagnostics render as warning boxes at the bottom: unknown `tax_category`
  values, ambiguous tags, legacy `TaxRelevant` counts, zero-amount matches,
  negative postings (neutral refund note), dividends without booked taxes.
- No editing, no Pauschbetrag comparison, no legal hints beyond the form
  reference.

### CSV export

Client-side, following `germany_tax_export.ts` (Papaparse). Columns:

`section,category,date,payee,account,amount,source,note,file,line`

Capital income rows use `section=capital_income`, `category` = `realized_gains`,
`realized_losses`, `dividends`, `interest`, `taxes_booked`, and carry the same
raw-value note in the `note` column: "aus dem Ledger, kein Steuererklärungswert".
Include a UTF-8 BOM so Excel opens umlauts correctly. `source` is one of `tag`,
`transaction`, `account`, or empty for capital-income rows. The CSV contains the
same raw values as the page, not tax-return field values.

## Diagnostics

| Condition                      | Message (German UI)                                                                                                                                                                                                                                                           |
| ------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TaxCategory` unknown          | "Unbekannte Steuerkategorie `<value>` bei N Postings; nicht angezeigt."                                                                                                                                                                                                       |
| Legacy tag present             | "N Postings verwenden noch das Legacy-Tag `TaxRelevant`; bitte auf `tax_category` umstellen."                                                                                                                                                                                 |
| Tag on shared transaction      | "N Transaktionen haben ein Steuer-Tag, aber nicht genau einen Aufwandsposten; Transaktion aufteilen oder den Aufwandsposten taggen."                                                                                                                                          |
| Zero amount                    | "N erkannte Postings haben einen Betrag von 0."                                                                                                                                                                                                                               |
| Negative posting               | "Negative Aufwandsposten erkannt. Diese können Erstattungen oder Korrekturen sein. Die steuerliche Behandlung von Erstattungen in einem anderen Jahr hängt von der Art der Ausgabe und den Umständen ab. Bitte manuell prüfen; Paisa bestimmt nicht das richtige Steuerjahr." |
| Dividends without booked taxes | "Dividendeneingänge ohne gebuchte Steuern; der Betrag kann netto sein. Bitte Broker-Abrechnung prüfen."                                                                                                                                                                       |
| Unmatched postings             | Coverage row: "M Postings in N Ausgabenkonten sind nicht zugeordnet. Keine Liste absetzbarer Beträge."                                                                                                                                                                        |
| Config category unknown        | Config validation error at startup/save (can stay English, it is a config error).                                                                                                                                                                                             |

Diagnostics never list hundreds of unrelated postings; they aggregate by
condition and show at most a few example transaction links. Their body text is
German; config validation errors can remain English because they are developer-
facing.

## Testing

Go:

- resolution precedence: tag over account; `none` excludes; longest prefix wins;
  `Expenses:Health` does not match `Expenses:HealthSomething`; unknown value
  diagnostically excluded; legacy `TaxRelevant` not applied; sibling tag with one
  expense posting applies; with two does not.
- API: year list includes income-only years and previous year; empty year;
  category and section totals; KAP raw totals for dividends-only years and for
  the single 2021 sale fixture; gains and losses split per matched pair even when
  an account nets to zero; dividends-without-booked-taxes diagnostic.
- Coverage: counts unmatched postings, including untagged postings in a
  partially tagged account; excludes matched postings; returns no amounts; never
  lists transactions; API uses the postings-based names. Negative postings are
  counted and marked.
- Zero amounts: a validly tagged zero posting is included, counted, adds 0 € and
  produces a diagnostic; it is not coverage.
- Resolution states: `matched`, `excluded`, `unresolved`, `invalid` are tested
  separately; `none` and “no rule” must not collapse into one empty value.
- Year filter: diagnostics too, not just amounts, are limited to the selected
  year; a 2024 legacy/negative posting must not appear in 2025.
- Year validation: empty defaults to previous year; a valid four-digit year
  without postings returns an empty overview; an invalid value returns 400.
- Summary: expense candidates, booked capital income and calculated capital
  income are separate fields; no combined total is produced; matched count
  excludes capital income postings.
- Decimal JSON: the API emits decimal amounts as JSON numbers, tested both on a
  generic decimal value and on the tax overview response. The setting is
  process-wide; the plan states that explicitly.
- Config: invalid `tax_deductions` fails `LoadConfig` and leaves the previous
  config untouched; the embedded JSON schema is the server-side validator.
- Source badges: all three sources (`tag`, `transaction`, `account`) round-trip
  from API to UI and CSV.
- CI: a dedicated workflow installs `ledger`, `hledger` and `beancount` and runs
  the parser integration tests, so a local machine without those binaries cannot
  turn a skip into an apparent pass.
- Parser tests for all three backends with `tax_category` metadata.
- hledger multi-tag regression (the `break` bug).

Frontend:

- CSV export unit test (columns, BOM, KAP rows).
- Page smoke test if the existing test setup supports it.

Manual:

- Run against the real ledger: 2025 shows Health (17 postings), Tax advice
  (WISO), tagged work equipment; 2021 KAP shows the Coinbase sale; 2025 KAP shows
  dividends even without sales.

## Documentation

- New `docs/reference/tax/deductions.md`: what the page shows, tagging syntax for
  ledger/hledger/beancount, category catalog, account mapping config, explicit
  statement that no tax calculation happens.
- Update `docs/reference/tax/index.md` supported paths.
- Add the nav entry documentation if the tax pages are listed.

## Risks and edge cases

- **Umlauts in the ledger.** The inspected file already contains broken
  characters (`B�rostuhl`). The feature must not re-encode the journal; only read
  it. CSV export must still emit UTF-8.
- **Foreign-currency expenses.** `Posting.Amount` is already converted to the
  default currency; no extra handling.
- **Split receipts.** The correct pattern is two expense postings, tag only the
  relevant one. A transaction-level tag with two expense postings is a
  diagnostic, not a guess.
- **Tags on refunds.** A negative amount reduces the category total, so the page
  shows the net of expenses and refunds booked in that year. Whether that net is
  the right tax figure is for the user and the tax software to decide. For a
  negative posting in a later year, the diagnostic and the `Negative` badge flag
  the ambiguity; Paisa does not reassign the amount to another year.
- **Account mapping too broad.** `Expenses:Insurance` may contain non-deductible
  policies. The page presents it as a candidate; `tax_category: none` lets the
  user exclude a specific posting.
- **Coverage noise.** Most private accounts will appear in the coverage count.
  The wording must make clear that this is a limit of automatic classification,
  not a to-do list. Do not sort it in a way that implies priority.
- **Capital income labels.** "Taxes booked" is deliberately vague. Any UI copy
  that calls it withholding tax or German capital gains tax would be wrong for
  some brokers.
- **Performance.** The year query touches only postings; no market prices or
  network calls. Capital income reuses the existing in-memory matching over
  investment postings.

## Revision history

- 2026-10-27 — Initial draft.
- 2026-10-27 — Review feedback incorporated:
  - compact coverage check for unmatched postings grouped by account (posting
    counts only, no amounts, no transactions); API names describe postings;
  - `Negative` badge and neutral refund diagnostic for negative postings, without
    asserting a tax-year rule;
  - Anlage KAP values explicitly labelled as raw ledger values, `Taxes booked`
    instead of `Withholding tax`;
  - summary metrics separated (expense candidates, capital income, matched
    expense postings, diagnostics), no combined total; matched count excludes
    capital income postings;
  - source badges cover `Tag`, `Transaction` and `Account`;
  - CSV keeps the raw-value note on capital income rows;
  - account prefix matching bounded at account separators;
  - beancount reads posting-level `META` only, so transaction metadata cannot
    leak onto sibling postings;
  - empty sections collapsed and labelled;
  - tests added for prefix boundaries, coverage, refunds and separated summary.
- 2026-10-27 — Second review feedback incorporated:
  - refund diagnostic reworded neutrally; no tax-year rule asserted;
  - `Refund` badge renamed to `Negative`, since a negative posting can also be a
    correction;
  - `Transaction` added to the source badges and CSV `source` values;
  - coverage API names changed to `accounts_with_unmatched_postings` and
    `unmatched_posting_count`;
  - `Matched expense postings` explicitly excludes capital income postings;
  - CSV capital income rows keep the raw-value note.
- 2026-10-27 — Language decision:
  - page content is German (legal terms from the return); global navigation and
    API/code stay English;
  - visibility is gated by `tax_regime: germany`, never by `locale`.
- 2026-10-27 — Plan review refinements:
  - booked (`dividends + interest`) and realized (`realized gains − losses`)
    capital income are separate summary fields and tiles; the realized tile is
    labelled `Realisierte Gewinne/Verluste`; no mixed KAP total;
  - decimal JSON output is a number in the API contract and is tested;
  - config schema validation is explicitly enforced in `LoadConfig`, and invalid
    loads must leave the previous config untouched;
  - internal resolution states are `matched`, `excluded`, `unresolved`,
    `invalid`;
  - zero-amount postings are included, counted, add 0 € and produce a
    diagnostic;
  - year filtering covers diagnostics as well as amounts;
  - the decimal JSON setting is explicitly process-wide, not endpoint-local;
  - invalid `year` values return 400; valid years without postings return an
    empty overview;
  - legacy tags are never read by the classifier, only by the diagnostic pass;
  - the KAP diagnostic says "ohne gebuchte Steuern" and is not shortened;
  - legacy extraction is posting-level; the test asserts the tag on the posting
    that carries it, not on its sibling;
  - sibling resolution ignores empty tags and treats a sibling `none` as an
    explicit exclusion;
  - `sales` counts distinct non-empty transaction IDs and counts empty-ID pairs
    individually;
  - the decimal test only claims the server-package contract;
  - `TaxDeductionAccounts()` returns a copy;
  - a dedicated CI job installs ledger, hledger and beancount and runs the
    parser tests instead of skipping them;
  - zero-date postings never reach the year selector;
  - the realized tile is labelled `Realisierte Gewinne/Verluste`.
