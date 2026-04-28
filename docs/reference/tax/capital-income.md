# Capital Income

This page is available for the `germany` tax regime.

Paisa provides a Germany-focused capital income report for realized
investment gains. The report summarizes calendar-year realized gains
and applies the configured Germany tax inputs from `germany_tax`.

## What the report shows

- gross realized gains and realized losses from matched buy and sell lots
- partial exemption adjustment for commodities that declare
  `germany_partial_exemption_rate`
- taxable amount before and after annual allowance
- annual allowance used
- capital income tax
- solidarity surcharge
- optional church tax
- withholding tax paid through broker tax postings
- tax credit used against the computed annual tax
- net tax due after available tax credits

The report also surfaces diagnostics when Paisa sees Germany tax
activity that may require manual review. Current diagnostics focus on:

- mutual funds or ETFs with Germany tax activity but no configured
  `germany_partial_exemption_rate`
- dividend or interest income without matching
  `Expenses:Broker:Taxes` inputs for the same tax year

The current implementation is intended as a practical first-pass
personal investor report. It does not yet model every Germany tax edge
case.

## Configuration

The following configuration values control the report:

```yaml
tax_regime: germany
germany_tax:
  annual_allowance: 1000
  capital_income_tax_rate: 0.25
  solidarity_surcharge_rate: 0.055
  church_tax_rate: 0

commodities:
  - name: VWCE
    type: mutualfund
    price:
      provider: com-yahoo
      code: VWCE
    germany_partial_exemption_rate: 0.3
```

## Notes

- The report uses calendar years, which aligns with Germany tax
  reporting.
- The allowance is applied at the annual summary level, not per account.
- `Expenses:Broker:Taxes` postings are treated as withholding-tax-aware
  inputs and reduce the remaining annual tax due up to the computed
  gross tax amount.
- If `germany_partial_exemption_rate` is not configured for a mutual
  fund or ETF, Paisa currently treats the position as fully taxable and
  emits a warning.
- India-only workflows such as Tax Harvesting and Schedule AL are not
  shown in the Germany regime.
