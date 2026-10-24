<script lang="ts">
  import {
    renderAllocation,
    renderAllocationTarget,
    renderAllocationTimeline
  } from "$lib/allocation";
  import COLORS, { generateColorScheme } from "$lib/colors";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import PreviewEmpty from "$lib/components/preview/PreviewEmpty.svelte";
  import PreviewKpi from "$lib/components/preview/PreviewKpi.svelte";
  import PreviewKpiStrip from "$lib/components/preview/PreviewKpiStrip.svelte";
  import PreviewPage from "$lib/components/preview/PreviewPage.svelte";
  import PreviewPanel from "$lib/components/preview/PreviewPanel.svelte";
  import Table from "$lib/components/Table.svelte";
  import { accountName, nonZeroCurrency } from "$lib/table_formatters";
  import {
    ajax,
    formatCurrency,
    formatPercentage,
    rem,
    type Aggregate,
    type Legend
  } from "$lib/utils";
  import _ from "lodash";
  import { onMount, tick } from "svelte";
  import type { ColumnDefinition, ProgressBarParams } from "tabulator-tables";

  let showAllocation = false;
  let depth = 2;
  let allocationTimelineLegends: Legend[] = [];
  let aggregateLeafNodes: Aggregate[] = [];
  let total = 0;
  let hasAllocationTimeline = false;
  let loadError = false;

  const columns: ColumnDefinition[] = [
    { title: "Account", field: "account", formatter: accountName },
    {
      title: "Market Value",
      field: "market_amount",
      hozAlign: "right",
      formatter: nonZeroCurrency
    },
    {
      title: "Percent",
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
        min: 0
      }
    }
  ];

  onMount(async () => {
    try {
      const {
        aggregates: aggregates,
        aggregates_timeline: aggregatesTimeline,
        allocation_targets: allocationTargets
      } = await ajax("/api/allocation");
      const accounts = _.keys(aggregates);
      aggregateLeafNodes = _.filter(_.values(aggregates), (a) => a.market_amount > 0);
      total = _.sumBy(aggregateLeafNodes, (a) => a.market_amount);
      aggregateLeafNodes = _.map(aggregateLeafNodes, (a) => {
        a.percent = (a.market_amount / total) * 100;
        return a;
      });
      const max = _.max(_.map(aggregateLeafNodes, (a) => a.percent)) || 100;
      (_.last(columns).formatterParams as ProgressBarParams).max = max;
      const color = generateColorScheme(accounts);
      depth = _.max(_.map(accounts, (account) => account.split(":").length));

      if (!_.isEmpty(allocationTargets)) {
        showAllocation = true;
      }
      hasAllocationTimeline = !_.isEmpty(aggregatesTimeline);
      await tick();

      if (showAllocation) {
        renderAllocationTarget(allocationTargets, color);
      }

      if (!_.isEmpty(aggregateLeafNodes)) {
        renderAllocation(aggregates, color);
      }

      if (hasAllocationTimeline) {
        allocationTimelineLegends = renderAllocationTimeline(aggregatesTimeline);
      }
      loadError = false;
    } catch (_error) {
      loadError = true;
    }
  });
</script>

<PreviewPage
  eyebrow="Distribution Analysis"
  title="Allocation"
  description="Compare current portfolio structure with configured targets and account-level allocation detail."
>
  <PreviewKpiStrip>
    <PreviewKpi label="Allocated value" value={formatCurrency(total)} tone="accent" />
    <PreviewKpi label="Leaf accounts" value={String(aggregateLeafNodes.length)} />
    <PreviewKpi label="Max depth" value={String(depth)} />
    <PreviewKpi label="Targets" value={showAllocation ? "Configured" : "Not configured"} />
  </PreviewKpiStrip>

  {#if loadError}
    <PreviewPanel title="Allocation data" subtle="The allocation endpoint is unavailable.">
      <PreviewEmpty
        title="Allocation data is unavailable"
        message="The backend returned an error for this endpoint. No synthetic allocation values are shown."
      />
    </PreviewPanel>
  {:else}
    <PreviewPanel
      title="Allocation targets"
      subtle="Target-versus-actual composition when targets exist."
    >
      {#if showAllocation}
        <div class="paisa-preview-chart">
          <div id="d3-allocation-target-treemap" style="width: 100%; position: relative"></div>
          <svg id="d3-allocation-target"></svg>
        </div>
      {:else}
        <PreviewEmpty
          title="No allocation targets configured"
          message="Current allocation can still be reviewed below; target comparison appears once targets exist."
        />
      {/if}
    </PreviewPanel>

    <div class="paisa-preview-grid">
      <PreviewPanel
        title="Allocation by category"
        subtle="Tree view of where value is concentrated."
      >
        {#if aggregateLeafNodes.length}
          <div class="paisa-preview-chart">
            <div id="d3-allocation-category" style="width: 100%; height: {depth * 100}px"></div>
          </div>
        {:else}
          <PreviewEmpty
            title="No allocation data"
            message="No positive allocation buckets were returned."
          />
        {/if}
      </PreviewPanel>

      <PreviewPanel
        title="Allocation by value"
        subtle="Value-weighted distribution across accounts."
      >
        {#if aggregateLeafNodes.length}
          <div class="paisa-preview-chart">
            <div id="d3-allocation-value" style="width: 100%; height: 300px"></div>
          </div>
        {:else}
          <PreviewEmpty
            title="No value breakdown"
            message="There is no live value data to chart yet."
          />
        {/if}
      </PreviewPanel>
    </div>

    <PreviewPanel title="Allocation timeline" subtle="How the allocation has moved over time.">
      {#if hasAllocationTimeline}
        <LegendCard legends={allocationTimelineLegends} clazz="mb-3" />
        <div class="paisa-preview-chart">
          <svg id="d3-allocation-timeline" width="100%" height="300"></svg>
        </div>
      {:else}
        <PreviewEmpty
          title="No allocation timeline"
          message="More historical allocation data is needed before this chart can render."
        />
      {/if}
    </PreviewPanel>

    <PreviewPanel title="Allocation table" subtle="Detailed account rows with value and percent.">
      {#if aggregateLeafNodes.length}
        <Table data={aggregateLeafNodes} tree {columns} preview />
      {:else}
        <PreviewEmpty
          title="No account rows"
          message="Allocation rows are empty for this dataset."
        />
      {/if}
    </PreviewPanel>
  {/if}
</PreviewPage>
