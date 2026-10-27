# Germany Tax Overview

This page is available for the `germany` tax regime. Open it from
**Tax → Overview**; the preview tree has the same page at
`/preview/more/tax/overview`.

The page is a handover sheet for the German income tax return. It lists
ledger **candidates** grouped by the form section they belong to. It is
not a tax calculator and not a confirmed deduction list.

## What the page is and is not

- It shows postings that match an account rule or carry a `tax_category`
  tag.
- It does not apply tax law: no Arbeitnehmer-Pauschbetrag, no zumutbare
  Belastung, no 20 % credit, no allowance and no partial-deduction math.
  Your tax software does that.
- It does not edit postings.
- It does not guess from payee names.

## Year selection

The selector lists every year that has postings in the ledger, plus the
previous calendar year. It defaults to the previous calendar year. A
valid four-digit year between 1900 and 2100 is accepted even when it has
no postings; the page then shows empty sections.

## Sections and categories

Sections are always shown in this order. Empty sections are collapsed and
labelled `Keine Buchungen`.

| Section                        | Form                   | Categories                                                                                                                                                          |
| ------------------------------ | ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Werbungskosten                 | Anlage N               | Arbeitsmittel, Fortbildung und Fachliteratur, Reisekosten, Wege zur Arbeit, Homeoffice, Telefon und Internet, Steuerberatung und -software, Sonstige Werbungskosten |
| Vorsorgeaufwand                | Anlage Vorsorgeaufwand | Kranken- und Pflegeversicherung, Sonstige Versicherungen                                                                                                            |
| Außergewöhnliche Belastungen   | Hauptvordruck          | Krankheitskosten                                                                                                                                                    |
| Haushaltsnahe Dienstleistungen | Hauptvordruck          | Haushaltsnahe Dienstleistungen, Handwerkerleistungen                                                                                                                |
| Spenden und Mitgliedsbeiträge  | Hauptvordruck          | Spenden und Mitgliedsbeiträge                                                                                                                                       |
| Kapitalerträge                 | Anlage KAP             | generated, not taggable                                                                                                                                             |

## Tagging postings

`tax_category` on the expense posting is the canonical form.

ledger and hledger:

```ledger
2025/09/09 * PayPal ; Bürostuhl
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

A tag on another posting in the same transaction (for example the bank
line) is accepted only when the transaction has exactly one expense
posting. With zero or several expense postings the page emits a
diagnostic instead of guessing. A tag on a split transaction must
therefore sit on the relevant expense posting.

To exclude a posting that an account rule would match, tag it with
`tax_category: "none"`. The posting is then shown in neither the
sections nor the coverage row.

The legacy `TaxRelevant` tag is never applied. Postings that still carry
it are reported in a diagnostic and must be migrated to `tax_category`
manually.

## Account rules

Account matching is configured with `tax_deductions.accounts`:

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

- Omitting `tax_deductions` uses the built-in rules shown above.
- An explicit list replaces the defaults.
- An empty list disables account matching; tags still work.
- Matching is whole-segment prefix matching: `Expenses:Health` matches
  `Expenses:Health` and `Expenses:Health:*`, but never
  `Expenses:HealthSomething`. The longest matching rule wins.
- Only `Expenses:` accounts and concrete account names are accepted;
  wildcards are rejected by configuration validation.
- Unknown category values are rejected when the configuration is loaded.

These accounts are deliberately not mapped by default:
`Expenses:Household:*`, `Expenses:Sports:*`, `Expenses:Car:*`,
`Expenses:Transportation`, `Expenses:Phone`, `Expenses:Utilities:*` and
`Expenses:Travel:*`. Tag them explicitly when they are relevant.

## Coverage

The collapsed coverage row counts expense postings in the selected year
that were not matched by a tag, the sibling rule or an account rule. It
groups them by account. It never shows amounts or individual
transactions: most of those postings are private spending, and the row
describes the limit of automatic classification, not a list of suspected
deductions.

## Negative postings

A negative expense posting (a refund or correction) is included and
reduces the category total. It carries a `Negativ` marker. Paisa does not
move it to another year and does not link it to an earlier expense.
When the selected year contains negative matched postings, a neutral
diagnostic asks you to review the correct tax-year treatment manually.

## Capital income (Anlage KAP)

The capital income card is separate from the expense sections. Every
value is a raw ledger aggregate: `aus dem Ledger, kein
Steuererklärungswert`. None of the values is guaranteed to be a specific
Anlage KAP line.

- **Berechnet**: realized gains and realized losses from matched
  buy/sell pairs, split per pair. Losses are shown as a positive number.
- **Gebucht**: dividends from `Income:Dividend*`, interest from
  `Income:Interest*`, and booked taxes from `Expenses:Broker:Taxes*`.
  The label is `Gebuchte Steuern (Art nicht klassifiziert)` because Paisa
  cannot distinguish foreign withholding from German capital gains tax.

Dividends are not grossed up. If a year has dividend or interest income
but no `Expenses:Broker:Taxes` postings, a diagnostic notes that the
booked amount may be net of withholding tax. The card never renders a
combined capital-income total, and the summary keeps booked capital
income and realized gains/losses as separate figures.

## CSV export

`CSV exportieren` downloads the selected year as UTF-8 CSV with a BOM.
Columns:

`section,category,date,payee,account,amount,source,note,file,line`

`source` is `tag`, `transaction` or `account`, and empty for capital
income rows. Capital income rows use `section=capital_income` and keep
the raw-value note in the `note` column. The CSV contains the same raw
values as the page, not tax-return field values.

## Visibility

The page and the menu entry are gated by `tax_regime: germany`. They are
independent of `locale`; `locale` only controls number and date
formatting. Page content is German because the labels are the legal terms
used in the German return. Navigation, API fields and code identifiers
stay English.
