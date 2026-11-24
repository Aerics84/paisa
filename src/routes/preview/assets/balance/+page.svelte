<script lang="ts">
  import AssetsBalance from "$lib/components/AssetsBalance.svelte";
  import { type AssetBreakdown } from "$lib/utils";
  import { getAssetBalance } from "$lib/api/assets";
  import _ from "lodash";
  import { onMount } from "svelte";

  let breakdowns: Record<string, AssetBreakdown> = {};

  onMount(async () => {
    ({ asset_breakdowns: breakdowns } = await getAssetBalance());
  });
</script>

<section class="section pb-0">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12 pb-0">
        <AssetsBalance {breakdowns} />
      </div>
    </div>
  </div>
</section>
