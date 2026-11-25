<script lang="ts">
  import { formatCurrency, formatFloat, isMobile, type Legend, type Networth } from "$lib/utils";
  import { getNetworth } from "$lib/api/assets";
  import COLORS from "$lib/colors";
  import { renderNetworth } from "$lib/networth";
  import _ from "lodash";
  import { onDestroy, onMount } from "svelte";
  import { dateRange, setAllowedDateRange } from "../../../../store";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import { t } from "$lib/i18n";

  let networth = $state(0);
  let investment = $state(0);
  let gain = $state(0);
  let xirr = $state(0);
  let svg = $state<Element>();
  let destroy: (() => void) | undefined;
  let points = $state<Networth[]>([]);
  let legends = $state<Legend[]>([]);

  $effect(() => {
    if (!_.isEmpty(points)) {
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
  });

  onDestroy(async () => {
    if (destroy) {
      destroy();
    }
  });

  onMount(async () => {
    const result = await getNetworth();
    points = result.networthTimeline;
    setAllowedDateRange(_.map(points, (p) => p.date));

    const current = _.last(points);
    if (current) {
      networth = current.investmentAmount + current.gainAmount - current.withdrawalAmount;
      investment = current.investmentAmount - current.withdrawalAmount;
      gain = current.gainAmount;
    }
    xirr = result.xirr;
  });
</script>

<section class="section tab-networth">
  <div class="container is-fluid">
    <nav class="level {isMobile() && 'grid-2'}">
      <LevelItem
        title={$t("dashboard.netWorth")}
        color={COLORS.primary}
        value={formatCurrency(networth)}
      />
      <LevelItem
        title={$t("dashboard.netInvestment")}
        color={COLORS.secondary}
        value={formatCurrency(investment)}
      />
      <LevelItem
        title={$t("dashboard.gainLoss")}
        color={gain >= 0 ? COLORS.gainText : COLORS.lossText}
        value={formatCurrency(gain)}
      />
      <LevelItem title="XIRR" value={formatFloat(xirr)} />
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
            {$t("assets.noTransactions")}
          </ZeroState>

          <LegendCard {legends} clazz="ml-4" />
          <svg id="d3-networth-timeline" height="500" bind:this={svg} />
        </div>
      </div>
    </div>
    <BoxLabel text={$t("assets.networthTimelineLabel")} />
  </div>
</section>
