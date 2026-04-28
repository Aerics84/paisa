# Capital Income

This page is available for the `germany` tax regime.

Paisa provides a Germany-focused capital income report for realized
investment gains. The report summarizes calendar-year realized gains
and applies the configured Germany tax inputs from `germany_tax`.

## What the report shows

- realized gain from matched buy and sell lots
- annual allowance used
- taxable amount after allowance
- capital income tax
- solidarity surcharge
- optional church tax

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
```

## Notes

- The report uses calendar years, which aligns with Germany tax
  reporting.
- The allowance is applied at the annual summary level, not per
  account.
- India-only workflows such as Tax Harvesting and Schedule AL are not
  shown in the Germany regime.
