<script lang="ts">
  import {
    formatCurrency,
    formatFloat,
    isMobile,
    type CurrencyNetworth,
    type Legend,
    type Networth
  } from "$lib/utils";
  import { getNetworth } from "$lib/api/assets";
  import COLORS from "$lib/colors";
  import { computePeriodStats, hasNetworthKpiData, renderNetworth } from "$lib/networth";
  import _ from "lodash";
  import { onDestroy, onMount } from "svelte";
  import { dateRange, setAllowedDateRange } from "../../../../store";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import { selectedCurrency } from "../../../../persisted_store";
  import { t } from "$lib/i18n";

  let defaultPoints = $state<Networth[]>([]);
  let currencyNetworths = $state<CurrencyNetworth[]>([]);
  let svg = $state<Element>();
  let destroy: (() => void) | undefined;
  let legends = $state<Legend[]>([]);

  let selected = $derived(_.find(currencyNetworths, (c) => c.name === $selectedCurrency));
  let symbol = $derived(selected?.symbol || "");
  let points = $derived(selected ? selected.networthTimeline : defaultPoints);
  let fmt = $derived((v: number) =>
    symbol ? symbol + formatCurrency(v, 2, "en-US") : formatCurrency(v)
  );
  let stats = $derived(computePeriodStats(points, $dateRange.from, $dateRange.to));
  let showKpis = $derived(hasNetworthKpiData(selected, points));

  $effect(() => {
    if (svg) {
      if (destroy) {
        destroy();
        destroy = undefined;
      }

      const inRange = _.filter(
        points,
        (p) => p.date.isSameOrBefore($dateRange.to) && p.date.isSameOrAfter($dateRange.from)
      );

      if (_.isEmpty(inRange)) {
        // D3 owns this container; Svelte only supplies the element.
        // eslint-disable-next-line svelte/no-dom-manipulating
        svg.innerHTML = "";
        legends = [];
      } else {
        ({ destroy, legends } = renderNetworth(inRange, svg, { currencySymbol: symbol }));
      }
    }
  });

  onDestroy(async () => {
    if (destroy) {
      destroy();
    }
  });

  onMount(async () => {
    const result = await getNetworth();
    defaultPoints = result.networthTimeline;
    currencyNetworths = result.currencyNetworths || [];
    setAllowedDateRange(_.map(defaultPoints, (p) => p.date));
  });
</script>

<section class="section tab-networth">
  <div class="container is-fluid">
    <nav class="level {isMobile() && 'grid-2'}">
      {#if showKpis}
        <LevelItem
          title={$t("dashboard.netWorth")}
          color={COLORS.primary}
          value={fmt(stats.networth)}
        />
        <LevelItem
          title={$t("dashboard.netInvestment")}
          color={COLORS.secondary}
          value={fmt(stats.investment)}
        />
        <LevelItem
          title={$t("dashboard.gainLoss")}
          color={stats.gain >= 0 ? COLORS.gainText : COLORS.lossText}
          value={fmt(stats.gain)}
        />
        <LevelItem title="XIRR" value={formatFloat(stats.xirr)} />
      {:else}
        <p class="is-size-6 has-text-grey">
          {$t("assets.noFxRates", {
            currency: selected?.name || $t("assets.selectedCurrency")
          })}
        </p>
      {/if}
    </nav>
  </div>
</section>

<section class="section tab-networth">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        <div class="box overflow-x-auto">
          <ZeroState item={points}>
            <strong>{$t("common.oops")}</strong>
            {#if selected}
              {$t("assets.noPricesForCommodity", { commodity: selected.commodity })}
            {:else}
              {$t("assets.noTransactions")}
            {/if}
          </ZeroState>

          <LegendCard {legends} clazz="ml-4" />
          <svg id="d3-networth-timeline" height="500" bind:this={svg} />
        </div>
      </div>
    </div>
    <BoxLabel
      text={$t("assets.networthTimelineWithCurrency", {
        currency: selected?.name || USER_CONFIG.default_currency
      })}
    />
  </div>
</section>
