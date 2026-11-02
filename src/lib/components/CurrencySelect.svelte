<script lang="ts">
  import _ from "lodash";
  import { selectedCurrency } from "../../persisted_store";

  // additional currencies configured in networth_currencies
  const currencies = _.map(USER_CONFIG.networth_currencies || [], (c) => c.name);

  // forget a stored selection that is no longer configured
  $: if ($selectedCurrency && !_.includes(currencies, $selectedCurrency)) {
    $selectedCurrency = "";
  }
</script>

{#if !_.isEmpty(currencies)}
  <div class="select is-small">
    <select bind:value={$selectedCurrency} aria-label="Currency">
      <option value="">{USER_CONFIG.default_currency}</option>
      {#each currencies as currency}
        <option value={currency}>{currency}</option>
      {/each}
    </select>
  </div>
{/if}
