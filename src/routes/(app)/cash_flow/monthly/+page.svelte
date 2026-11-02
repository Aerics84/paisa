<script lang="ts">
  import _ from "lodash";
  import { renderMonthlyFlow } from "$lib/cash_flow";
  import { ajax, formatCurrency, type CashFlow, type Legend } from "$lib/utils";
  import { onMount } from "svelte";
  import { dateRange, setAllowedDateRange } from "../../../../store";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import LegendCard from "$lib/components/LegendCard.svelte";

  let legends: Legend[] = [];
  let cashFlows: CashFlow[] = [];
  let renderer: (cashflows: CashFlow[]) => void;

  $: visibleCashFlows = _.filter(
    cashFlows,
    (c) => c.date.isSameOrBefore($dateRange.to) && c.date.isSameOrAfter($dateRange.from)
  );

  $: if (!_.isEmpty(cashFlows) && renderer) {
    renderer(visibleCashFlows);
  }

  $: cashFlowSummary = _.isEmpty(visibleCashFlows)
    ? []
    : (() => {
        const income = _.sumBy(visibleCashFlows, "income");
        const expenses = _.sumBy(visibleCashFlows, "expenses");
        const tax = _.sumBy(visibleCashFlows, "tax");
        return [
          { label: "Income", value: income },
          { label: "Expenses", value: expenses + tax },
          { label: "Net", value: income - expenses - tax }
        ];
      })();

  onMount(async () => {
    ({ cash_flows: cashFlows } = await ajax("/api/cash_flow"));
    setAllowedDateRange(_.map(cashFlows, (c) => c.date));
    ({ renderer, legends } = renderMonthlyFlow("#d3-monthly-cash-flow", {
      rotate: true,
      balance: _.last(cashFlows)?.balance || 0
    }));
  });
</script>

<section class="section">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        <div class="box cash-flow-monthly">
          <ZeroState item={cashFlows}>
            <strong>Oops!</strong> You have not made any transactions.
          </ZeroState>

          {#if !_.isEmpty(cashFlowSummary)}
            <div class="cash-flow-monthly__summary" aria-label="Cash flow totals">
              {#each cashFlowSummary as item}
                <div class="cash-flow-monthly__summary-card">
                  <span class="is-size-7 has-text-grey">{item.label}</span>
                  <span
                    class="cash-flow-monthly__summary-value"
                    class:has-text-danger={item.value < 0}
                  >
                    {formatCurrency(item.value)}
                  </span>
                </div>
              {/each}
            </div>
          {/if}

          <LegendCard {legends} clazz="ml-5 mb-2 cash-flow-monthly__legend" />
          <div class="cash-flow-monthly__chart-scroll">
            <div class="cash-flow-monthly__chart-canvas">
              <svg
                class:is-not-visible={_.isEmpty(cashFlows)}
                id="d3-monthly-cash-flow"
                width="100%"
                height="500"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</section>

<style lang="scss">
  .cash-flow-monthly__summary {
    display: none;
  }

  .cash-flow-monthly__chart-scroll {
    max-width: 100%;
  }

  @media screen and (max-width: 768px) {
    .cash-flow-monthly {
      padding: 0.75rem;
    }

    .cash-flow-monthly__summary {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 0.5rem;
      margin-bottom: 0.75rem;
    }

    .cash-flow-monthly__summary-card {
      display: flex;
      flex-direction: column;
      gap: 0.2rem;
      min-width: 0;
      padding: 0.5rem 0.6rem;
      border: 1px solid var(--app-border, rgba(127, 127, 127, 0.25));
      border-radius: 0.5rem;
    }

    .cash-flow-monthly__summary-value {
      font-size: 1rem;
      font-weight: 600;
      overflow-wrap: anywhere;
    }

    :global(.cash-flow-monthly__legend) {
      margin-left: 0 !important;
    }

    .cash-flow-monthly__chart-scroll {
      overflow-x: auto;
      -webkit-overflow-scrolling: touch;
    }

    .cash-flow-monthly__chart-canvas {
      min-width: 640px;
    }
  }
</style>
