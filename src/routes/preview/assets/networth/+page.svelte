<script lang="ts">
  import {
    ajax,
    formatCurrency,
    formatFloat,
    isMobile,
    type Legend,
    type Networth
  } from "$lib/utils";
  import { renderNetworth } from "$lib/networth";
  import _ from "lodash";
  import { onDestroy, onMount } from "svelte";
  import { dateRange, setAllowedDateRange } from "../../../../store";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import PreviewEmpty from "$lib/components/preview/PreviewEmpty.svelte";
  import PreviewKpi from "$lib/components/preview/PreviewKpi.svelte";
  import PreviewKpiStrip from "$lib/components/preview/PreviewKpiStrip.svelte";
  import PreviewPage from "$lib/components/preview/PreviewPage.svelte";
  import PreviewPanel from "$lib/components/preview/PreviewPanel.svelte";

  let networth = 0;
  let investment = 0;
  let gain = 0;
  let xirr = 0;
  let svg: Element;
  let destroy: () => void;
  let points: Networth[] = [];
  let legends: Legend[] = [];
  let loadError = false;

  $: if (!_.isEmpty(points) && svg) {
    if (destroy) {
      destroy();
    }

    ({ destroy, legends } = renderNetworth(
      _.filter(
        points,
        (p) => p.date.isSameOrBefore($dateRange.to) && p.date.isSameOrAfter($dateRange.from)
      ),
      svg
    ));
  }

  onDestroy(async () => {
    if (destroy) {
      destroy();
    }
  });

  onMount(async () => {
    try {
      const result = await ajax("/api/networth");
      points = result.networthTimeline;
      setAllowedDateRange(_.map(points, (p) => p.date));

      const current = _.last(points);
      if (current) {
        networth = current.investmentAmount + current.gainAmount - current.withdrawalAmount;
        investment = current.investmentAmount - current.withdrawalAmount;
        gain = current.gainAmount;
      }
      xirr = result.xirr;
      loadError = false;
    } catch (_error) {
      loadError = true;
    }
  });
</script>

<PreviewPage
  eyebrow="Trend Analysis"
  title="Net Worth"
  description="Track the shape of total wealth, invested capital, gains, and return over the selected period."
>
  <PreviewKpiStrip>
    <PreviewKpi label="Net worth" value={formatCurrency(networth)} tone="accent" />
    <PreviewKpi label="Net investment" value={formatCurrency(investment)} />
    <PreviewKpi
      label="Gain / Loss"
      value={formatCurrency(gain)}
      tone={gain >= 0 ? "positive" : "negative"}
    />
    <PreviewKpi label="XIRR" value={formatFloat(xirr)} />
  </PreviewKpiStrip>

  <PreviewPanel
    title="Net worth timeline"
    subtle="Investment, withdrawal, and market movement in one long-running view."
  >
    {#if loadError}
      <PreviewEmpty
        title="Net-worth data is unavailable"
        message="The backend returned an error for this endpoint. The rest of the preview shell remains available."
      />
    {:else if points.length}
      <LegendCard {legends} clazz="mb-3" />
      <div class="paisa-preview-chart">
        <svg id="d3-networth-timeline" height="500" bind:this={svg}></svg>
      </div>
    {:else}
      <PreviewEmpty
        title="No net-worth history yet"
        message="Add transactions or import a journal before this trend can be calculated."
      />
    {/if}
  </PreviewPanel>

  <div class="paisa-preview-grid">
    <PreviewPanel title="Current signal" subtle="What this view can explain from live data.">
      <div class="paisa-preview-list">
        <div class="paisa-preview-list__item">
          <span>Observed snapshots</span>
          <strong>{points.length}</strong>
        </div>
        <div class="paisa-preview-list__item">
          <span>Visible range</span>
          <strong>{isMobile() ? "Compact" : "Full timeline"}</strong>
        </div>
      </div>
    </PreviewPanel>

    <PreviewPanel
      title="Drill deeper"
      subtle="Use adjacent asset views to separate allocation and returns."
    >
      <div class="paisa-preview-list">
        <a class="paisa-preview-list__item secondary-link" href="/preview/assets/investment">
          <span>Contribution rhythm</span>
          <strong>Investment</strong>
        </a>
        <a class="paisa-preview-list__item secondary-link" href="/preview/assets/gain">
          <span>Return drivers</span>
          <strong>Gain</strong>
        </a>
      </div>
    </PreviewPanel>
  </div>
</PreviewPage>
