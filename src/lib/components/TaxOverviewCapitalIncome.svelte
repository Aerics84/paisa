<script lang="ts">
  import { formatCurrency } from "$lib/utils";
  import type { TaxOverviewCapitalIncome } from "$lib/utils";

  export let capitalIncome: TaxOverviewCapitalIncome;

  let accountsExpanded = false;
</script>

<div class="box mb-3">
  <div class="is-flex is-align-items-baseline" style="gap: 0.75rem;">
    <strong>Kapitalerträge</strong>
    <span class="has-text-grey is-size-7">Anlage KAP</span>
  </div>
  <p class="is-size-7 is-warning mt-1">aus dem Ledger, kein Steuererklärungswert</p>

  <div class="mt-4">
    <p class="has-text-grey is-size-7 is-uppercase mb-1">Berechnet</p>
    <div class="is-flex is-justify-content-space-between">
      <span>Realisierte Gewinne</span>
      <span>{formatCurrency(capitalIncome.realized_gains)}</span>
    </div>
    <div class="is-flex is-justify-content-space-between">
      <span>Realisierte Verluste</span>
      <span>{formatCurrency(capitalIncome.realized_losses)}</span>
    </div>
  </div>

  <div class="mt-4">
    <p class="has-text-grey is-size-7 is-uppercase mb-1">Gebucht</p>
    <div class="is-flex is-justify-content-space-between">
      <span>Dividenden</span>
      <span>{formatCurrency(capitalIncome.dividends)}</span>
    </div>
    <div class="is-flex is-justify-content-space-between">
      <span>Zinsen</span>
      <span>{formatCurrency(capitalIncome.interest)}</span>
    </div>
    <div class="is-flex is-justify-content-space-between">
      <span>Gebuchte Steuern (Art nicht klassifiziert)</span>
      <span>{formatCurrency(capitalIncome.taxes_booked)}</span>
    </div>
  </div>

  {#if capitalIncome.accounts.length > 0}
    <button
      type="button"
      class="button is-small is-light mt-4"
      on:click={() => (accountsExpanded = !accountsExpanded)}
    >
      {accountsExpanded ? "Konten ausblenden" : "Konten anzeigen"}
    </button>

    {#if accountsExpanded}
      <table class="table is-fullwidth is-narrow mt-3">
        <thead>
          <tr>
            <th>Konto</th>
            <th class="has-text-right">Realisierter Gewinn</th>
          </tr>
        </thead>
        <tbody>
          {#each capitalIncome.accounts as account}
            <tr>
              <td>{account.account}</td>
              <td class="has-text-right">{formatCurrency(account.realized_gain)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  {/if}
</div>
