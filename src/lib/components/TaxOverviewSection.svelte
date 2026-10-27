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
  <button
    type="button"
    class="tax-overview-section-header"
    aria-expanded={expanded}
    on:click={onToggle}
  >
    <span class="is-flex is-align-items-baseline" style="gap: 0.75rem; flex-wrap: wrap;">
      <strong>{section.label}</strong>
      <span class="has-text-grey is-size-7">{section.form}</span>
    </span>
    <span class="is-flex is-align-items-center" style="gap: 1rem;">
      <span class="has-text-grey is-size-7">
        {section.posting_count === 0
          ? "Keine Buchungen"
          : section.posting_count === 1
            ? "1 Posting"
            : `${section.posting_count} Postings`}
      </span>
      <strong>{formatCurrency(section.total)}</strong>
      <span class="icon is-small" aria-hidden="true">
        <i class={expanded ? "fas fa-chevron-down" : "fas fa-chevron-right"}></i>
      </span>
    </span>
  </button>

  {#if expanded}
    {#if section.posting_count === 0}
      <p class="has-text-grey mt-3">Keine Buchungen</p>
    {:else}
      {#each section.categories as category}
        {#if category.postings.length > 0}
          <div class="mt-4">
            <div class="is-flex is-justify-content-space-between mb-2">
              <strong>{category.label}</strong>
              <span>{formatCurrency(category.total)}</span>
            </div>
            <div class="tax-overview-table-wrap">
              <table class="table is-fullwidth is-narrow is-hoverable tax-overview-table">
                <colgroup>
                  <col style="width: 7.5rem;" />
                  <col />
                  <col style="width: 32%;" />
                  <col style="width: 9rem;" />
                </colgroup>
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
                          <div class="is-size-7 has-text-grey posting-note">{posting.note}</div>
                        {/if}
                        <span class="is-size-7 has-text-grey posting-source">
                          Quelle: {sourceLabel[posting.source] ?? posting.source}
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
          </div>
        {/if}
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

  .tax-overview-table-wrap {
    overflow-x: auto;
  }

  .tax-overview-table {
    table-layout: fixed;
    min-width: 40rem;
  }

  .tax-overview-table :global(td),
  .tax-overview-table :global(th) {
    overflow-wrap: anywhere;
  }

  .posting-note {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .posting-source {
    display: block;
    margin-top: 0.25rem;
  }
</style>
