<script lang="ts">
  import { downloadGermanyTaxYear } from "$lib/germany_tax_export";
  import { formatCurrency, formatFloat, type GermanyTaxYear } from "$lib/utils";

  export let taxYear: GermanyTaxYear;
</script>

<div class="column is-12">
  <div class="card">
    <header class="card-header">
      <p class="card-header-title">{taxYear.tax_year}</p>
      <div class="card-header-icon">
        <button class="button is-small" on:click={() => downloadGermanyTaxYear(taxYear)}>
          Export CSV
        </button>
      </div>
    </header>

    <div class="card-content">
      <div class="content">
        <div class="columns">
          <div class="column is-4">
            <table class="table is-narrow is-fullwidth is-hoverable">
              <tbody>
                <tr>
                  <td>Gross Realized Gain</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.gross_realized_gain)}</td
                  >
                </tr>
                <tr>
                  <td>Realized Loss</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.realized_loss)}</td
                  >
                </tr>
                <tr>
                  <td>Realized Gain</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.realized_gain)}</td
                  >
                </tr>
                <tr>
                  <td>Partial Exemption Adjustment</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.partial_exemption_amount)}</td
                  >
                </tr>
                <tr>
                  <td>Taxable Base Before Allowance</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.taxable_amount_before_allowance)}</td
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
                  <td>Gross Total Tax</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.total_tax)}</td
                  >
                </tr>
                <tr>
                  <td>Withholding Tax Paid</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.withholding_tax_paid)}</td
                  >
                </tr>
                <tr>
                  <td>Tax Credit Used</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.tax_credit_used)}</td
                  >
                </tr>
                <tr>
                  <td>Net Tax Due</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(taxYear.summary.net_tax_due)}</td
                  >
                </tr>
              </tbody>
            </table>

            <p class="is-size-7 has-text-grey">
              Allowance {formatCurrency(taxYear.settings.annual_allowance)} | Capital income tax
              {formatFloat(taxYear.settings.capital_income_tax_rate * 100)}% | Soli
              {formatFloat(taxYear.settings.solidarity_surcharge_rate * 100)}% | Church tax
              {formatFloat(taxYear.settings.church_tax_rate * 100)}%
            </p>

            {#if taxYear.diagnostics.length > 0}
              <article class="message is-warning mt-3">
                <div class="message-body">
                  {#each taxYear.diagnostics as diagnostic}
                    <p class="has-text-weight-bold">{diagnostic.summary}</p>
                    <p class="is-size-7 mb-2">{diagnostic.details}</p>
                  {/each}
                </div>
              </article>
            {/if}
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
                  <th class="has-text-right">Exemption Rate</th>
                  <th class="has-text-right">Exemption Adjustment</th>
                  <th class="has-text-right">Taxable Gain</th>
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
                    <td class="has-text-right"
                      >{account.partial_exemption_rate == null
                        ? "n/a"
                        : `${formatFloat(account.partial_exemption_rate * 100)}%`}</td
                    >
                    <td class="has-text-right"
                      >{formatCurrency(account.partial_exemption_amount)}</td
                    >
                    <td class="has-text-right has-text-weight-bold"
                      >{formatCurrency(account.taxable_gain)}</td
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
