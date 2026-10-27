<script lang="ts">
  import { formatCurrency } from "$lib/utils";
  import dayjs from "dayjs";
  import type { TaxOverviewSection } from "$lib/utils";

  export let section: TaxOverviewSection;
  export let expanded: boolean;
  export let onToggle: () => void;

  const sourceLabel: Record<string, string> = {
    tag: "Tag",
    transaction: "Transaktion",
    account: "Konto"
  };
</script>

<div class="box mb-3">
  <button type="button" class="tax-overview-section-header" on:click={onToggle}>
    <span class="is-flex is-align-items-baseline" style="gap: 0.75rem; flex-wrap: wrap;">
      <strong>{section.label}</strong>
      <span class="has-text-grey is-size-7">{section.form}</span>
    </span>
    <span class="is-flex is-align-items-center" style="gap: 1rem;">
      <span class="has-text-grey is-size-7">
        {section.posting_count === 0 ? "Keine Buchungen" : `${section.posting_count} Postings`}
      </span>
      <strong>{formatCurrency(section.total)}</strong>
      <span class="icon is-small">
        <i class={expanded ? "fas fa-chevron-down" : "fas fa-chevron-right"}></i>
      </span>
    </span>
  </button>

  {#if expanded}
    {#if section.posting_count === 0}
      <p class="has-text-grey mt-3">Keine Buchungen</p>
    {:else}
      {#each section.categories as category}
        <div class="mt-4">
          <div class="is-flex is-justify-content-space-between mb-2">
            <strong>{category.label}</strong>
            <span>{formatCurrency(category.total)}</span>
          </div>
          <table class="table is-fullwidth is-narrow is-hoverable">
            <thead>
              <tr>
                <th>Datum</th>
                <th>Empfänger</th>
                <th>Konto</th>
                <th class="has-text-right">Betrag</th>
              </tr>
            </thead>
            <tbody>
              {#each category.postings as posting}
                <tr>
                  <td class="is-nowrap">{dayjs(posting.date).format("DD.MM.YYYY")}</td>
                  <td>
                    <div>{posting.payee}</div>
                    {#if posting.note}
                      <div class="is-size-7 has-text-grey text-truncate">{posting.note}</div>
                    {/if}
                    <span class="tag is-light is-small mt-1">
                      {sourceLabel[posting.source] ?? posting.source}
                    </span>
                    {#if posting.amount < 0}
                      <span class="tag is-warning is-small mt-1">Negativ</span>
                    {/if}
                  </td>
                  <td>{posting.account}</td>
                  <td class="has-text-right is-nowrap">{formatCurrency(posting.amount)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/each}
    {/if}
  {/if}
</div>

<style>
  .tax-overview-section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    width: 100%;
    border: 0;
    background: transparent;
    padding: 0;
    cursor: pointer;
    color: inherit;
    text-align: left;
  }
</style>
