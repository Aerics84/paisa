<script lang="ts">
  import {
    renderAllocation,
    renderAllocationTarget,
    renderAllocationTimeline
  } from "$lib/allocation";
  import COLORS, { generateColorScheme } from "$lib/colors";
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import Table from "$lib/components/Table.svelte";
  import { accountName, nonZeroCurrency } from "$lib/table_formatters";
  import { formatPercentage, rem, type Aggregate, type Legend } from "$lib/utils";
  import { getAllocation } from "$lib/api/assets";
  import _ from "lodash";
  import { onMount, tick } from "svelte";
  import type { ColumnDefinition } from "tabulator-tables";
  import { t } from "$lib/i18n";

  let showAllocation = $state(false);
  let depth = $state(2);
  let allocationTimelineLegends = $state<Legend[]>([]);
  let aggregateLeafNodes = $state<Aggregate[]>([]);
  let total = $state(0);
  let maxPercent = $state(100);

  let columns = $derived<ColumnDefinition[]>([
    { title: $t("assets.account"), field: "account", formatter: accountName },
    {
      title: $t("assets.marketValue"),
      field: "market_amount",
      hozAlign: "right",
      formatter: nonZeroCurrency
    },
    {
      title: $t("assets.percent"),
      field: "percent",
      hozAlign: "right",
      formatter: (cell) => formatPercentage(cell.getValue() / 100, 2)
    },
    {
      title: "%",
      field: "percent",
      hozAlign: "right",
      formatter: "progress",
      cssClass: "has-text-left",
      minWidth: rem(250),
      formatterParams: {
        color: COLORS.assets,
        min: 0,
        max: maxPercent
      }
    }
  ]);

  onMount(async () => {
    const {
      aggregates: aggregates,
      aggregates_timeline: aggregatesTimeline,
      allocation_targets: allocationTargets
    } = await getAllocation();
    const accounts = _.keys(aggregates);
    aggregateLeafNodes = _.filter(_.values(aggregates), (a) => a.market_amount > 0);
    total = _.sumBy(aggregateLeafNodes, (a) => a.market_amount);
    aggregateLeafNodes = _.map(aggregateLeafNodes, (a) => {
      a.percent = (a.market_amount / total) * 100;
      return a;
    });
    const max = _.max(_.map(aggregateLeafNodes, (a) => a.percent)) || 100;
    maxPercent = max;
    const color = generateColorScheme(accounts);
    depth = _.max(_.map(accounts, (account) => account.split(":").length));

    if (!_.isEmpty(allocationTargets)) {
      showAllocation = true;
    }
    await tick();

    renderAllocationTarget(allocationTargets, color);
    renderAllocation(aggregates, color);
    allocationTimelineLegends = renderAllocationTimeline(aggregatesTimeline);
  });
</script>

<section class="section tab-allocation" style={showAllocation ? "" : "display: none"}>
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12 has-text-centered">
        <div class="box overflow-x-auto">
          <div id="d3-allocation-target-treemap" style="width: 100%; position: relative"></div>
          <svg id="d3-allocation-target"></svg>
        </div>
      </div>
    </div>
    <BoxLabel text={$t("assets.allocationTargets")} />
  </div>
</section>
<section class="section tab-allocation">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12 has-text-centered">
        <div id="d3-allocation-category" style="width: 100%; height: {depth * 100}px"></div>
      </div>
    </div>
    <BoxLabel text={$t("assets.allocationByCategory")} />
  </div>
</section>
<section class="section tab-allocation">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12 has-text-centered">
        <div id="d3-allocation-value" style="width: 100%; height: 300px"></div>
      </div>
    </div>
    <BoxLabel text={$t("assets.allocationByValue")} />
  </div>
</section>
<section class="section tab-allocation">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        <div class="box">
          <LegendCard legends={allocationTimelineLegends} clazz="ml-4" />
          <svg id="d3-allocation-timeline" width="100%" height="300" />
        </div>
      </div>
    </div>
    <BoxLabel text={$t("assets.allocationTimeline")} />
  </div>
</section>
<section class="section tab-allocation">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        <Table data={aggregateLeafNodes} tree {columns} />
      </div>
    </div>
    <BoxLabel text={$t("assets.allocationTable")} />
  </div>
</section>
