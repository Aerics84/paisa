<script lang="ts">
  import AssetsBalance from "$lib/components/AssetsBalance.svelte";
  import PreviewEmpty from "$lib/components/preview/PreviewEmpty.svelte";
  import PreviewKpi from "$lib/components/preview/PreviewKpi.svelte";
  import PreviewKpiStrip from "$lib/components/preview/PreviewKpiStrip.svelte";
  import PreviewPage from "$lib/components/preview/PreviewPage.svelte";
  import PreviewPanel from "$lib/components/preview/PreviewPanel.svelte";
  import { ajax, formatCurrency, type AssetBreakdown } from "$lib/utils";
  import _ from "lodash";
  import { onMount } from "svelte";

  let breakdowns: Record<string, AssetBreakdown> = {};
  let loadError = false;

  $: rows = _.values(breakdowns);
  $: totalMarket = _.sumBy(rows, (breakdown) => breakdown.marketAmount);
  $: largest = _.maxBy(rows, (breakdown) => breakdown.marketAmount);
  $: largestShare =
    totalMarket && largest ? Math.round((largest.marketAmount / totalMarket) * 100) : 0;

  onMount(async () => {
    try {
      ({ asset_breakdowns: breakdowns } = await ajax("/api/assets/balance"));
      loadError = false;
    } catch (_error) {
      loadError = true;
    }
  });
</script>

<PreviewPage
  eyebrow="Distribution Analysis"
  title="Balance"
  description="See where money is sitting today before opening account-level detail."
>
  <PreviewKpiStrip>
    <PreviewKpi label="Total balance" value={formatCurrency(totalMarket)} tone="accent" />
    <PreviewKpi label="Accounts" value={String(rows.length)} />
    <PreviewKpi label="Largest bucket" value={largest?.group || "None"} />
    <PreviewKpi label="Concentration" value={`${largestShare}%`} />
  </PreviewKpiStrip>

  <PreviewPanel
    title="Account balance table"
    subtle="Live account hierarchy with market value, gains, and return metrics."
  >
    {#if loadError}
      <PreviewEmpty
        title="Balance data is unavailable"
        message="The backend returned an error for this endpoint. Account rows are not shown."
      />
    {:else if rows.length}
      <AssetsBalance {breakdowns} preview />
    {:else}
      <PreviewEmpty
        title="No account balances"
        message="Import or enter asset postings before balance distribution can be shown."
      />
    {/if}
  </PreviewPanel>
</PreviewPage>
