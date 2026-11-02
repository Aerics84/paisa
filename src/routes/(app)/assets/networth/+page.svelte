<script lang="ts">
  import {
    ajax,
    formatCurrency,
    formatFloat,
    isMobile,
    type CurrencyNetworth,
    type Legend,
    type Networth
  } from "$lib/utils";
  import COLORS from "$lib/colors";
  import { computePeriodStats, renderNetworth } from "$lib/networth";
  import _ from "lodash";
  import { onDestroy, onMount } from "svelte";
  import { dateRange, setAllowedDateRange } from "../../../../store";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import { selectedCurrency } from "../../../../persisted_store";

  let points: Networth[] = [];
  let defaultPoints: Networth[] = [];
  let currencyNetworths: CurrencyNetworth[] = [];
  let stats = { networth: 0, investment: 0, gain: 0, xirr: 0 };
  let svg: Element;
  let destroy: () => void;
  let legends: Legend[] = [];

  $: selected = _.find(currencyNetworths, (c) => c.name === $selectedCurrency);
  $: symbol = selected?.symbol || "";
  $: points = selected ? selected.networthTimeline : defaultPoints;
  $: fmt = (v: number) =>
    symbol ? symbol + formatCurrency(v, 2, "en-US") : formatCurrency(v);
  $: stats = computePeriodStats(points, $dateRange.from, $dateRange.to);

  $: if (svg) {
    if (destroy) {
      destroy();
      destroy = null;
    }

    const inRange = _.filter(
      points,
      (p) => p.date.isSameOrBefore($dateRange.to) && p.date.isSameOrAfter($dateRange.from)
    );

    if (_.isEmpty(inRange)) {
      svg.innerHTML = "";
      legends = [];
    } else {
      ({ destroy, legends } = renderNetworth(inRange, svg, { currencySymbol: symbol }));
    }
  }

  onDestroy(async () => {
    if (destroy) {
      destroy();
    }
  });

  onMount(async () => {
    const result = await ajax("/api/networth");
    defaultPoints = result.networthTimeline;
    currencyNetworths = result.currencyNetworths || [];
    setAllowedDateRange(_.map(defaultPoints, (p) => p.date));
  });
</script>

<section class="section tab-networth">
  <div class="container is-fluid">
    <nav class="level {isMobile() && 'grid-2'}">
      <LevelItem title="Net worth" color={COLORS.primary} value={fmt(stats.networth)} />
      <LevelItem title="Net Investment" color={COLORS.secondary} value={fmt(stats.investment)} />
      <LevelItem
        title="Gain / Loss"
        color={stats.gain >= 0 ? COLORS.gainText : COLORS.lossText}
        value={fmt(stats.gain)}
      />
      <LevelItem title="XIRR" value={formatFloat(stats.xirr)} />
    </nav>
  </div>
</section>

<section class="section tab-networth">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        <div class="box overflow-x-auto">
          <ZeroState item={points}>
            <strong>Oops!</strong>
            {#if selected}
              No prices found for commodity {selected.commodity}.
            {:else}
              You have no transactions.
            {/if}
          </ZeroState>

          <LegendCard {legends} clazz="ml-4" />
          <svg id="d3-networth-timeline" height="500" bind:this={svg} />
        </div>
      </div>
    </div>
    <BoxLabel text="Networth Timeline ({selected?.name || USER_CONFIG.default_currency})" />
  </div>
</section>
