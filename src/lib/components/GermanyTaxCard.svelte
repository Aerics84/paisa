<script lang="ts">
  import { formatCurrency, formatFloat, type GermanyTaxYear } from "$lib/utils";

  export let taxYear: GermanyTaxYear;
</script>

<div class="column is-12">
  <div class="card">
    <header class="card-header">
      <p class="card-header-title">{taxYear.tax_year}</p>
    </header>

    <div class="card-content">
      <div class="content">
        <div class="columns">
          <div class="column is-4">
            <table class="table is-narrow is-fullwidth is-hoverable">
              <tbody>
                <tr>
                  <td>Realized Gain</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.realized_gain)}</td
                  >
                </tr>
                <tr>
                  <td>Allowance Used</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.allowance_used)}</td
                  >
                </tr>
                <tr>
                  <td>Taxable Amount</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.taxable_amount)}</td
                  >
                </tr>
                <tr>
                  <td>Capital Income Tax</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.capital_income_tax)}</td
                  >
                </tr>
                <tr>
                  <td>Solidarity Surcharge</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.solidarity_surcharge)}</td
                  >
                </tr>
                <tr>
                  <td>Church Tax</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.church_tax)}</td
                  >
                </tr>
                <tr>
                  <td>Total Tax</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.total_tax)}</td
                  >
                </tr>
              </tbody>
            </table>

            <p class="is-size-7 has-text-grey">
              Allowance {formatCurrency(taxYear.settings.annual_allowance)} • Capital income tax
              {formatFloat(taxYear.settings.capital_income_tax_rate * 100)}% • Soli
              {formatFloat(taxYear.settings.solidarity_surcharge_rate * 100)}% • Church tax
              {formatFloat(taxYear.settings.church_tax_rate * 100)}%
            </p>
          </div>

          <div class="column is-8 overflow-x-auto">
            <table class="table is-narrow is-fullwidth is-hoverable">
              <thead>
                <tr>
                  <th>Account</th>
                  <th class="has-text-right">Sold Units</th>
                  <th class="has-text-right">Purchase Price</th>
                  <th class="has-text-right">Sell Price</th>
                  <th class="has-text-right">Realized Gain</th>
                </tr>
              </thead>
              <tbody>
                {#each taxYear.accounts as account}
                  <tr>
                    <td>{account.account}</td>
                    <td class="has-text-right">{formatFloat(account.units)}</td>
                    <td class="has-text-right">{formatCurrency(account.purchase_price)}</td>
                    <td class="has-text-right">{formatCurrency(account.sell_price)}</td>
                    <td class="has-text-right has-text-weight-bold"
                      >{formatCurrency(account.realized_gain)}</td
                    >
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>
</div>
