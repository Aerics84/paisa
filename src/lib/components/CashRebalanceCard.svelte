<script lang="ts">
  import {
    ajax,
    formatCurrency,
    formatPercentage,
    type AllocationTarget,
    type RebalancePlan
  } from "$lib/utils";

  let {
    targets = [],
    initialPlan = null
  }: {
    targets: AllocationTarget[];
    initialPlan?: RebalancePlan | null;
  } = $props();

  let mode = $state<"cash_injection" | "full_rebalance">("cash_injection");
  let cashAmount = $state<number>(0);
  let plan = $state<RebalancePlan | null>(null);
  let loading = $state(false);

  // React to the plan passed by the page, including after targets are saved.
  $effect(() => {
    if (initialPlan) {
      plan = initialPlan;
      mode = initialPlan.mode;
    }
  });

  async function updatePlan() {
    loading = true;
    try {
      const res = await ajax("/api/allocation/rebalance", {
        method: "POST",
        body: JSON.stringify({
          cash: Number(cashAmount) || 0,
          mode
        })
      });
      plan = res;
    } catch (e) {
      console.error("Failed to calculate rebalance:", e);
    } finally {
      loading = false;
    }
  }

  function handleModeChange(newMode: "cash_injection" | "full_rebalance") {
    mode = newMode;
    updatePlan();
  }

  function handleCashInput(e: Event) {
    const val = Number((e.target as HTMLInputElement).value);
    cashAmount = isNaN(val) || val < 0 ? 0 : val;
    updatePlan();
  }

  function addCash(amount: number) {
    cashAmount = (Number(cashAmount) || 0) + amount;
    updatePlan();
  }

  $effect(() => {
    if (targets && targets.length > 0 && !plan) {
      updatePlan();
    }
  });
</script>

<div class="box p-5 mt-4">
  <div
    class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-base-200 pb-4 mb-4"
  >
    <div>
      <h3 class="text-lg font-bold flex items-center gap-2">
        <span>⚖️ Portfolio Rebalancing Engine</span>
      </h3>
      <p class="text-sm text-base-content/70">
        {#if mode === "cash_injection"}
          Calculates tax-free cash deployment targeting underweighted assets without selling
          existing holdings.
        {:else}
          Calculates buy and sell transactions required to restore exact target weights.
        {/if}
      </p>
    </div>

    <!-- Mode Selector Tabs -->
    <div class="join">
      <button
        type="button"
        class="btn btn-sm join-item"
        class:btn-primary={mode === "cash_injection"}
        onclick={() => handleModeChange("cash_injection")}
      >
        💧 Cash Deployment (Buy Only)
      </button>
      <button
        type="button"
        class="btn btn-sm join-item"
        class:btn-primary={mode === "full_rebalance"}
        onclick={() => handleModeChange("full_rebalance")}
      >
        🔄 Full Rebalance (Buy & Sell)
      </button>
    </div>
  </div>

  {#if mode === "cash_injection"}
    <div class="bg-base-200/50 rounded-lg p-4 mb-5">
      <div class="flex flex-col sm:flex-row sm:items-center gap-3">
        <label for="fresh-cash-input" class="text-sm font-semibold whitespace-nowrap">
          Fresh Investable Cash:
        </label>
        <div class="flex items-center gap-2 flex-grow max-w-xs">
          <input
            id="fresh-cash-input"
            type="number"
            min="0"
            step="1000"
            value={cashAmount}
            oninput={handleCashInput}
            class="input input-bordered input-sm w-full font-mono font-semibold"
            placeholder="0"
          />
        </div>
        <div class="flex items-center gap-1.5 flex-wrap">
          <button type="button" class="btn btn-xs btn-outline" onclick={() => addCash(10000)}
            >+10k</button
          >
          <button type="button" class="btn btn-xs btn-outline" onclick={() => addCash(50000)}
            >+50k</button
          >
          <button type="button" class="btn btn-xs btn-outline" onclick={() => addCash(100000)}
            >+100k</button
          >
          <button
            type="button"
            class="btn btn-xs btn-ghost text-xs"
            onclick={() => {
              cashAmount = 0;
              updatePlan();
            }}>Reset</button
          >
        </div>
      </div>
    </div>
  {/if}

  {#if plan && plan.items && plan.items.length > 0}
    <div class="overflow-x-auto">
      <table class="table table-sm w-full">
        <thead>
          <tr class="text-base-content/70 border-b border-base-200 text-xs uppercase">
            <th>Asset Class</th>
            <th class="text-right">Target</th>
            <th class="text-right">Current %</th>
            <th class="text-right">Current Value</th>
            <th class="text-center">Action Required</th>
            <th class="text-right">Projected %</th>
          </tr>
        </thead>
        <tbody>
          {#each plan.items as item}
            <tr class="hover:bg-base-200/40 transition-colors">
              <td class="font-semibold">{item.name}</td>
              <td class="text-right font-mono">{formatPercentage(Number(item.target) / 100, 1)}</td>
              <td class="text-right font-mono">{formatPercentage(Number(item.current) / 100, 1)}</td
              >
              <td class="text-right font-mono">{formatCurrency(Number(item.market_amount))}</td>
              <td class="text-center">
                {#if item.action === "BUY"}
                  <span
                    class="badge badge-success badge-sm font-semibold gap-1 text-white py-2.5 px-3"
                  >
                    BUY {formatCurrency(Number(item.amount))}
                  </span>
                {:else if item.action === "SELL"}
                  <span
                    class="badge badge-error badge-sm font-semibold gap-1 text-white py-2.5 px-3"
                  >
                    SELL {formatCurrency(Number(item.amount))}
                  </span>
                {:else}
                  <span class="badge badge-ghost badge-sm text-base-content/60"> HOLD </span>
                {/if}
              </td>
              <td class="text-right font-mono font-semibold">
                {formatPercentage(Number(item.new_percent) / 100, 1)}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div
      class="flex items-center justify-between text-xs text-base-content/60 mt-4 pt-3 border-t border-base-200"
    >
      <div>
        Portfolio Total: <span class="font-mono font-semibold text-base-content"
          >{formatCurrency(Number(plan.total_before))}</span
        >
        {#if Number(plan.cash_amount) > 0}
          &rarr; Projected: <span class="font-mono font-semibold text-base-content"
            >{formatCurrency(Number(plan.total_after))}</span
          >
        {/if}
      </div>
      <div>
        {#if loading}
          <span class="loading loading-spinner loading-xs"></span> Calculating...
        {/if}
      </div>
    </div>
  {:else}
    <div class="text-center py-6 text-sm text-base-content/60">
      Configure allocation targets above to view rebalancing recommendations.
    </div>
  {/if}
</div>
