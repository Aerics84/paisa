<script lang="ts">
  import LegendCard from "$lib/components/LegendCard.svelte";
  import PreviewEmpty from "$lib/components/preview/PreviewEmpty.svelte";
  import PreviewKpi from "$lib/components/preview/PreviewKpi.svelte";
  import PreviewKpiStrip from "$lib/components/preview/PreviewKpiStrip.svelte";
  import PreviewPage from "$lib/components/preview/PreviewPage.svelte";
  import PreviewPanel from "$lib/components/preview/PreviewPanel.svelte";
  import {
    renderMonthlyInvestmentTimeline,
    renderYearlyCards,
    renderYearlyInvestmentTimeline
  } from "$lib/investment";
  import { ajax, type Legend } from "$lib/utils";
  import _ from "lodash";
  import { onMount, tick } from "svelte";

  let monthlyInvestmentTimelineLegends: Legend[] = [];
  let yearlyInvestmentTimelineLegends: Legend[] = [];
  let assetCount = 0;
  let yearlyCardCount = 0;
  let loadError = false;

  onMount(async () => {
    try {
      const { assets: assets, yearly_cards: yearlyCards } = await ajax("/api/investment");
      assetCount = _.size(assets);
      yearlyCardCount = _.size(yearlyCards);
      await tick();

      if (assetCount) {
        monthlyInvestmentTimelineLegends = renderMonthlyInvestmentTimeline(assets);
      }

      if (yearlyCardCount) {
        yearlyInvestmentTimelineLegends = renderYearlyInvestmentTimeline(yearlyCards);
        renderYearlyCards(yearlyCards);
      }
      loadError = false;
    } catch (_error) {
      loadError = true;
    }
  });
</script>

<PreviewPage
  eyebrow="Trend Analysis"
  title="Investment"
  description="Understand contribution rhythm across months and financial years before comparing it with returns."
>
  <PreviewKpiStrip>
    <PreviewKpi label="Tracked assets" value={String(assetCount)} tone="accent" />
    <PreviewKpi label="Yearly periods" value={String(yearlyCardCount)} />
    <PreviewKpi label="Primary view" value="Monthly" />
    <PreviewKpi label="Secondary view" value="Financial year" />
  </PreviewKpiStrip>

  <PreviewPanel
    title="Monthly investment timeline"
    subtle="Contribution pattern across tracked assets."
  >
    {#if loadError}
      <PreviewEmpty
        title="Investment data is unavailable"
        message="The backend returned an error for this endpoint. No synthetic investment trend is shown."
      />
    {:else if assetCount}
      <LegendCard legends={monthlyInvestmentTimelineLegends} clazz="mb-3" />
      <div class="paisa-preview-chart">
        <svg id="d3-investment-timeline" width="100%" height="500"></svg>
      </div>
    {:else}
      <PreviewEmpty
        title="No investment activity yet"
        message="This trend appears once the investment endpoint returns live asset contributions."
      />
    {/if}
  </PreviewPanel>

  <div class="paisa-preview-grid">
    <PreviewPanel title="Financial year timeline" subtle="Year-by-year contribution cadence.">
      {#if yearlyCardCount}
        <LegendCard legends={yearlyInvestmentTimelineLegends} clazz="mb-3" />
        <div class="paisa-preview-chart">
          <svg id="d3-yearly-investment-timeline" width="100%"></svg>
        </div>
      {:else}
        <PreviewEmpty
          title="No yearly history yet"
          message="More dated investment activity is needed before yearly movement can be shown."
        />
      {/if}
    </PreviewPanel>

    <PreviewPanel title="Yearly cards" subtle="Annual contribution summaries.">
      {#if yearlyCardCount}
        <div class="columns is-flex-wrap-wrap" id="d3-yearly-investment-cards"></div>
      {:else}
        <PreviewEmpty
          title="No annual summaries"
          message="Annual cards are hidden until they can be rendered from live data."
        />
      {/if}
    </PreviewPanel>
  </div>
</PreviewPage>
