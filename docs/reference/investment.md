---
description: "The investment page yearly cards show the total surplus and savings rate next to the investment rate, with their formulas and limits."
---

# Investment

The investment page shows your holdings, their market value and a set of
yearly cards. Each yearly card summarizes the financial year that starts
at `financial_year_starting_month` and reports how much of your income you
did not spend and how much of it you invested.

## The numbers on a yearly card

| Row                 | Meaning                                                                      |
| ------------------- | ---------------------------------------------------------------------------- |
| Gross Salary Income | Postings under `Income:Salary`                                               |
| Gross Other Income  | Postings under any other `Income:` account                                   |
| Tax                 | Postings under `Expenses:Tax`                                                |
| Net Income          | `Gross Salary Income + Gross Other Income - Tax`                             |
| Net Expense         | Postings under `Expenses:` except `Expenses:Tax`                             |
| Investment          | Postings under `Assets:` (excluding cash, loan-funded buys and stock splits) |
| Total Surplus       | `Net Income - Net Expense`                                                   |
| Total Savings Rate  | `100 * Total Surplus / Net Income`                                           |
| Investment Rate     | `100 * Investment / Net Income`                                              |

`Net Income` is Paisa's existing income definition: salary plus every other
income posting, minus taxes. "Other income" is not limited to earned
income; it can also contain interest, dividends or capital gains, so the
card must not be read as a purely employment-income statement.

`Expenses:Tax` is already subtracted from `Net Income` and is therefore
excluded from `Net Expense`. Taxes are counted exactly once.

## How to read the rates

`Total Savings Rate` describes the share of net income that was not spent.
`Investment Rate` describes the share of net income that was moved into
investment accounts. The two answers are different questions: money can be
saved without being invested, and money can be invested without being
saved (for example by moving an existing cash balance into an investment
account).

Negative surpluses and rates above 100% are shown as they are. They are not
clipped, because clipping would hide a real overspend or a real drawdown of
savings. When `Net Income` is zero or negative, both rates are shown as
`N/A` instead of `0%`, because the division would be meaningless.

`Total Surplus` and `Total Savings Rate` are also shown as `N/A` when the
period contains no recorded income posting or no recorded expense posting.
A period that only records investments, or only one side of the
income/expense pair, cannot state what was saved: "spent nothing" and
"expenses not recorded" look identical in the ledger. Paisa does not
present either case as a confident figure.

The API reports this through `has_income_postings` and
`has_expense_postings`. `overall_savings_rate_available` mirrors whether
`Net Income` is positive; the numeric `overall_savings_rate` is `0` when it
is not available, so consumers must check the flag before displaying it.

The card colors follow the common guideline of saving 20 % of net income: a
`Total Savings Rate` of 20 % or more is green, a positive rate below 20 % is
amber, and a negative rate is red. `Total Surplus` is green when positive and
red when negative, and negative amounts are shown in red. `N/A` values stay
neutral, so a period without recorded data does not look like a bad year.
`Investment Rate` is not colored: investing is not good or bad by itself and
the 20 % guideline does not apply to it. The colors describe recorded postings
only; they are not investment advice.

## What the surplus is not

The surplus is a calculated value derived from the postings recorded in your
ledger. It is **not** the change in your bank balance and **not** a complete
reconciliation to your net worth:

- Postings that never made it into the ledger are not visible to Paisa.
- Transfers between your own asset accounts, loan repayments and changes in
  the market value of your holdings are not split out in this version.
- The same income and expense postings can already be reflected elsewhere
  in Paisa's reports; the yearly card is a summary of recorded postings, not
  a second set of books.

The information tooltip on the card states this as well: the value is
calculated from recorded ledger postings and is not the change in bank
balance.

## Complete periods, partial periods and missing activity

Two flags describe the data behind a card:

- `period_complete` only says whether the end of the financial year is
  already over; a year that ends today already counts as complete. A running
  financial year is labeled `(YTD)` and should be read as a year-to-date
  figure, not as a finished year.
- `has_activity` only says whether at least one income, expense, tax or
  investment posting was processed for that period. A period without any such
  posting is shown as `No recorded activity` instead of a confirmed zero
  surplus.

Neither flag proves that the ledger contains every real-world transaction.
Paisa can only report what was recorded.

## Historical figures are not a forecast

Past cards describe past recorded postings. They are not a prediction of
future income, spending or returns, and they do not carry any investment
recommendation.
