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
  import AllocationTargetModal from "$lib/components/AllocationTargetModal.svelte";
  import CashRebalanceCard from "$lib/components/CashRebalanceCard.svelte";
  import { accountName, nonZeroCurrency } from "$lib/table_formatters";
  import {
    ajax,
    formatCurrency,
    formatPercentage,
    rem,
    type Aggregate,
    type AllocationTarget,
    type Legend,
    type RebalancePlan
  } from "$lib/utils";
  import _ from "lodash";
  import { onMount, tick } from "svelte";
  import type { ColumnDefinition, ProgressBarParams } from "tabulator-tables";

  let showAllocation = false;
  let depth = 2;
  let allocationTimelineLegends: Legend[] = [];
  let aggregateLeafNodes: Aggregate[] = [];
  let total = 0;
  let allocationTargets: AllocationTarget[] = [];
  let rebalancePlan: RebalancePlan | null = null;
  let availableAccounts: string[] = [];
  let showTargetModal = false;

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

  async function loadData() {
    const [allocationResult, configResult] = await Promise.all([
      ajax("/api/allocation"),
      ajax("/api/config")
    ]);
    const {
      aggregates: aggregates,
      aggregates_timeline: aggregatesTimeline,
      allocation_targets: targets,
      rebalance: rebalance
    } = allocationResult;
    allocationTargets = targets || [];
    rebalancePlan = rebalance || null;
    availableAccounts = (configResult.accounts || []).filter((a: string) =>
      a.startsWith("Assets:")
    );
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

    showAllocation = !_.isEmpty(allocationTargets);
    await tick();

    if (showAllocation) {
      renderAllocationTarget(allocationTargets, color);
    }
    renderAllocation(aggregates, color);
    allocationTimelineLegends = renderAllocationTimeline(aggregatesTimeline);
  }

  onMount(loadData);
</script>

<section class="section tab-allocation">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        <div class="box p-5">
          <div class="flex items-center justify-between pb-3 border-b border-base-200 mb-4">
            <div>
              <h2 class="text-base font-bold flex items-center gap-2">
                <span>🎯 Target Asset Allocation</span>
                {#if showAllocation}
                  <span class="badge badge-sm badge-ghost">{allocationTargets.length} Targets</span>
                {/if}
              </h2>
            </div>
            <div>
              <button
                type="button"
                class="btn btn-sm btn-primary gap-1"
                onclick={() => (showTargetModal = true)}
              >
                <span>⚙</span>
                {showAllocation ? "Manage Targets" : "Set Up Targets"}
              </button>
            </div>
          </div>

          {#if showAllocation}
            <!-- Drift Badges Overview Grid -->
            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 mb-5">
              {#each allocationTargets as target}
                <div class="bg-base-200/50 rounded-xl p-3 border border-base-300">
                  <div class="flex items-center justify-between mb-1">
                    <span class="font-bold text-sm truncate">{target.name}</span>
                    {#if target.status === "overweight"}
                      <span class="badge badge-xs badge-warning font-semibold text-[10px] py-1 px-2"
                        >Overweight</span
                      >
                    {:else if target.status === "underweight"}
                      <span class="badge badge-xs badge-info font-semibold text-[10px] py-1 px-2"
                        >Underweight</span
                      >
                    {:else}
                      <span
                        class="badge badge-xs badge-success font-semibold text-white text-[10px] py-1 px-2"
                        >In Band</span
                      >
                    {/if}
                  </div>
                  <div class="flex items-baseline justify-between text-xs mt-1">
                    <span class="text-base-content/70"
                      >Current: <strong class="font-mono text-base-content"
                        >{formatPercentage(Number(target.current) / 100, 1)}</strong
                      ></span
                    >
                    <span class="text-base-content/70"
                      >Target: <span class="font-mono text-base-content/90"
                        >{formatPercentage(Number(target.target) / 100, 1)}</span
                      ></span
                    >
                  </div>
                  <div
                    class="flex items-baseline justify-between text-xs mt-1 pt-1 border-t border-base-300/50"
                  >
                    <span class="text-base-content/60"
                      >{formatCurrency(Number(target.market_amount || 0))}</span
                    >
                    <span
                      class="font-mono text-[11px] font-semibold"
                      class:text-warning={target.status === "overweight"}
                      class:text-info={target.status === "underweight"}
                      class:text-success={target.status === "in_band"}
                    >
                      {Number(target.drift) > 0 ? "+" : ""}{formatPercentage(
                        Number(target.drift || 0) / 100,
                        1
                      )} drift
                    </span>
                  </div>
                </div>
              {/each}
            </div>

            <div class="overflow-x-auto text-center">
              <div id="d3-allocation-target-treemap" style="width: 100%; position: relative"></div>
              <svg id="d3-allocation-target" />
            </div>

            <!-- Smart Rebalancing Calculator -->
            <CashRebalanceCard targets={allocationTargets} initialPlan={rebalancePlan} />
          {:else}
            <!-- Zero State: Prompt to configure targets -->
            <div class="has-text-centered p-6 bg-base-200/30 rounded-xl my-2">
              <div class="text-4xl mb-2">🎯</div>
              <h3 class="text-base font-bold mb-1">Target Asset Allocation Not Configured</h3>
              <p class="text-sm text-base-content/70 max-w-md mx-auto mb-4">
                Define your desired asset allocation weights (e.g. 60% Equity, 40% Debt) to monitor
                portfolio drift and get automated buy/sell rebalancing recommendations without
                editing config files.
              </p>
              <button
                type="button"
                class="btn btn-primary btn-sm gap-2"
                onclick={() => (showTargetModal = true)}
              >
                <span>＋</span> Set Up Asset Targets
              </button>
            </div>
          {/if}
        </div>
      </div>
    </div>
    <BoxLabel text="Allocation Targets" />
  </div>
</section>
<section class="section tab-allocation">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12 has-text-centered">
        <div id="d3-allocation-category" style="width: 100%; height: {depth * 100}px"></div>
      </div>
    </div>
    <BoxLabel text="Allocation by category" />
  </div>
</section>
<section class="section tab-allocation">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12 has-text-centered">
        <div id="d3-allocation-value" style="width: 100%; height: 300px"></div>
      </div>
    </div>
    <BoxLabel text="Allocation by value" />
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
    <BoxLabel text="Allocation Timeline" />
  </div>
</section>
<section class="section tab-allocation">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        <Table data={aggregateLeafNodes} tree {columns} />
      </div>
    </div>
    <BoxLabel text="Allocation Table" />
  </div>
</section>

<AllocationTargetModal
  bind:active={showTargetModal}
  targets={allocationTargets}
  {availableAccounts}
  onSave={loadData}
/>
