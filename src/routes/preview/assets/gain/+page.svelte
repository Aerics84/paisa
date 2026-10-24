<script lang="ts">
  import LegendCard from "$lib/components/LegendCard.svelte";
  import { buildLegends, renderOverview } from "$lib/gain";
  import { ajax, type Legend } from "$lib/utils";
  import PreviewEmpty from "$lib/components/preview/PreviewEmpty.svelte";
  import PreviewKpi from "$lib/components/preview/PreviewKpi.svelte";
  import PreviewKpiStrip from "$lib/components/preview/PreviewKpiStrip.svelte";
  import PreviewPage from "$lib/components/preview/PreviewPage.svelte";
  import PreviewPanel from "$lib/components/preview/PreviewPanel.svelte";
  import _ from "lodash";
  import { onMount, tick } from "svelte";

  let legends: Legend[] = [];
  let gainCount = 0;
  let loadError = false;

  onMount(async () => {
    try {
      const { gain_breakdown: gains } = await ajax("/api/gain");

      gainCount = _.size(gains);
      await tick();
      if (gainCount) {
        legends = buildLegends();
        renderOverview(gains);
      }
      loadError = false;
    } catch (_error) {
      loadError = true;
    }
  });
</script>

<PreviewPage
  eyebrow="Trend Analysis"
  title="Gain"
  description="Separate gains and losses by position so return drivers are visible before drilling into an instrument."
>
  <PreviewKpiStrip>
    <PreviewKpi label="Tracked positions" value={String(gainCount)} tone="accent" />
    <PreviewKpi label="Primary view" value="Overview" />
    <PreviewKpi label="Detail route" value="Per position" />
    <PreviewKpi label="Data source" value="Live gain API" />
  </PreviewKpiStrip>

  <PreviewPanel title="Gain overview" subtle="Winning and losing positions in one comparison view.">
    {#if loadError}
      <PreviewEmpty
        title="Gain data is unavailable"
        message="The backend returned an error for this endpoint. No fabricated gain values are shown."
      />
    {:else if gainCount}
      <LegendCard {legends} clazz="mb-3" />
      <div class="paisa-preview-chart">
        <svg id="d3-gain-overview"></svg>
      </div>
    {:else}
      <PreviewEmpty
        title="No gain data yet"
        message="This overview appears once Paisa can compute gain breakdowns from your journal."
      />
    {/if}
  </PreviewPanel>

  <PreviewPanel title="Timeline breakdown" subtle="Position-level movement and drilldown context.">
    {#if gainCount}
      <div class="paisa-preview-chart d3-gain-timeline-breakdown">
        <div id="d3-gain-timeline-breakdown"></div>
      </div>
    {:else}
      <PreviewEmpty
        title="No timeline breakdown"
        message="There are no live gain entries to explain yet."
      />
    {/if}
  </PreviewPanel>
</PreviewPage>
