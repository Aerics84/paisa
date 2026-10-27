<script lang="ts">
  import { onMount } from "svelte";
  import { ajax, formatCurrency } from "$lib/utils";
  import type { TaxOverview } from "$lib/utils";
  import { downloadTaxOverview } from "$lib/tax_overview_export";
  import TaxOverviewSection from "./TaxOverviewSection.svelte";
  import TaxOverviewCapitalIncome from "./TaxOverviewCapitalIncome.svelte";

  let overview: TaxOverview | null = null;
  let selectedYear = "";
  let expandedSections: Record<string, boolean> = {};
  let coverageExpanded = false;
  let error = "";

  function apply(data: TaxOverview) {
    overview = data;
    selectedYear = data.selected_year;
    expandedSections = Object.fromEntries(data.sections.map((s) => [s.key, s.posting_count > 0]));
  }

  async function load(year?: string) {
    error = "";
    try {
      apply(await ajax(year ? `/api/tax_overview?year=${year}` : "/api/tax_overview"));
    } catch {
      error = "Die Steuerübersicht konnte nicht geladen werden.";
    }
  }

  function toggleSection(key: string) {
    expandedSections = { ...expandedSections, [key]: !expandedSections[key] };
  }

  onMount(() => {
    load();
  });
</script>

<section class="section tab-tax-overview">
  <div class="container">
    <div
      class="is-flex is-justify-content-space-between is-align-items-center mb-2"
      style="gap: 1rem;"
    >
      <h1 class="title is-4 mb-0">Steuerübersicht</h1>
      {#if overview}
        <div class="is-flex is-align-items-center" style="gap: 0.75rem;">
          <div class="select is-small">
            <select bind:value={selectedYear} on:change={(e) => load(e.currentTarget.value)}>
              {#each overview.years.slice().reverse() as year}
                <option value={year}>Steuerjahr {year}</option>
              {/each}
            </select>
          </div>
          <button
            type="button"
            class="button is-small is-link"
            on:click={() => downloadTaxOverview(overview)}
          >
            CSV exportieren
          </button>
        </div>
      {/if}
    </div>
    <p class="has-text-grey mb-5">Ledger-Kandidaten, keine bestätigten Absetzbeträge.</p>

    {#if error}
      <div class="notification is-warning">{error}</div>
    {/if}

    {#if overview}
      <div class="columns is-multiline mb-5">
        <div class="column is-2">
          <div class="box py-3">
            <p class="is-size-7 has-text-grey">Erkannte Ausgaben</p>
            <p class="is-size-5 has-text-weight-semibold">
              {formatCurrency(overview.summary.expense_candidates)}
            </p>
          </div>
        </div>
        <div class="column is-2">
          <div class="box py-3">
            <p class="is-size-7 has-text-grey">Kapitalerträge (gebucht)</p>
            <p class="is-size-5 has-text-weight-semibold">
              {formatCurrency(overview.summary.capital_income_booked)}
            </p>
          </div>
        </div>
        <div class="column is-3">
          <div class="box py-3">
            <p class="is-size-7 has-text-grey">Realisierte Gewinne/Verluste</p>
            <p class="is-size-5 has-text-weight-semibold">
              {formatCurrency(overview.summary.capital_income_calculated)}
            </p>
          </div>
        </div>
        <div class="column is-2">
          <div class="box py-3">
            <p class="is-size-7 has-text-grey">Erkannte Aufwandsposten</p>
            <p class="is-size-5 has-text-weight-semibold">
              {overview.summary.matched_expense_posting_count}
            </p>
          </div>
        </div>
        <div class="column is-2">
          <div class="box py-3">
            <p class="is-size-7 has-text-grey">Hinweise</p>
            <p class="is-size-5 has-text-weight-semibold">{overview.summary.diagnostic_count}</p>
          </div>
        </div>
      </div>

      {#each overview.sections as section}
        <TaxOverviewSection
          {section}
          expanded={expandedSections[section.key]}
          onToggle={() => toggleSection(section.key)}
        />
      {/each}

      <div class="box mb-3">
        <button
          type="button"
          class="is-flex is-justify-content-space-between is-align-items-center"
          style="width: 100%; border: 0; background: transparent; padding: 0; cursor: pointer; color: inherit;"
          on:click={() => (coverageExpanded = !coverageExpanded)}
        >
          <span>
            Abdeckung: {overview.coverage.unmatched_posting_count} Postings in {overview.coverage
              .accounts_with_unmatched_postings} Ausgabenkonten sind nicht zugeordnet.
          </span>
          <span class="icon is-small">
            <i class={coverageExpanded ? "fas fa-chevron-down" : "fas fa-chevron-right"}></i>
          </span>
        </button>

        {#if coverageExpanded}
          <table class="table is-fullwidth is-narrow mt-3">
            <thead>
              <tr>
                <th>Konto</th>
                <th class="has-text-right">Postings</th>
              </tr>
            </thead>
            <tbody>
              {#each overview.coverage.accounts as account}
                <tr>
                  <td>{account.account}</td>
                  <td class="has-text-right">{account.unmatched_posting_count}</td>
                </tr>
              {/each}
            </tbody>
          </table>
          <p class="is-size-7 has-text-grey mt-2">Keine Liste absetzbarer Beträge.</p>
        {/if}
      </div>

      <TaxOverviewCapitalIncome capitalIncome={overview.capital_income} />

      {#each overview.diagnostics as diagnostic}
        <article class="message {diagnostic.level === 'warning' ? 'is-warning' : 'is-info'}">
          <div class="message-body">
            <p class="has-text-weight-semibold">{diagnostic.summary}</p>
            {#if diagnostic.details}
              <p class="is-size-7 mt-1">{diagnostic.details}</p>
            {/if}
          </div>
        </article>
      {/each}
    {/if}
  </div>
</section>
