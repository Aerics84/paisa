<script lang="ts">
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import { buildLegends, renderOverview } from "$lib/gain";
  import { type Legend } from "$lib/utils";
  import { getGain } from "$lib/api/assets";
  import _ from "lodash";
  import { onMount } from "svelte";
  import { t } from "$lib/i18n";

  let legends: Legend[] = [];

  onMount(async () => {
    const { gain_breakdown: gains } = await getGain();

    legends = buildLegends();
    renderOverview(gains);
  });
</script>

<section class="section tab-gain">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        <!-- WCAG 2.1.1: horizontal scrolling needs keyboard access. -->
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div
          class="box overflow-x-auto"
          tabindex="0"
          role="region"
          aria-label={$t("assets.gainOverviewChart")}
        >
          <LegendCard {legends} clazz="ml-4" />
          <svg id="d3-gain-overview" />
        </div>
      </div>
    </div>
    <BoxLabel text={$t("assets.gainOverview")} />
  </div>
</section>
<section class="section tab-gain">
  <div class="container is-fluid d3-gain-timeline-breakdown">
    <div class="columns">
      <div id="d3-gain-timeline-breakdown" class="column is-12"></div>
    </div>
  </div>
</section>
