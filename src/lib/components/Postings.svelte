<script lang="ts">
  import { accountColorStyle } from "$lib/colors";
  import { iconText } from "$lib/icon";
  import {
    firstName,
    formatCurrency,
    formatFloatUptoPrecision,
    isMobile,
    type Posting
  } from "$lib/utils";

  const unlessDefaultCurrency = (p: Posting) => {
    if (p.commodity == USER_CONFIG.default_currency) {
      return "";
    } else {
      return `${formatFloatUptoPrecision(p.quantity, 3)} ${
        p.commodity
      } @ ${formatFloatUptoPrecision(p.amount / p.quantity, 4)}`;
    }
  };

  export let postings: Posting[];
</script>

<div style="margin: 4px 0;">
  {#each postings as p}
    {@const conversion = unlessDefaultCurrency(p)}
    {#if isMobile()}
      <div
        class="is-flex is-align-items-baseline is-hoverable mobile-posting-row"
        style="margin: 1px 0; line-height: 22px; height: 22px; overflow: hidden;"
      >
        <div
          class="truncate custom-icon mr-2"
          style="flex: 1 1 0%; min-width: 0;"
          title={p.account}
        >
          <span style={accountColorStyle(firstName(p.account))}>{iconText(p.account)}</span>
          {p.account}
        </div>
        <div class="is-flex is-align-items-baseline" style="flex: 0 0 auto; min-width: 0;">
          {#if conversion}
            <div
              class="has-text-right has-text-grey is-size-7 mr-2 truncate mobile-posting-conversion"
              style="max-width: 9rem;"
            >
              {conversion}
            </div>
          {/if}
          <div class="has-text-right" style="white-space: nowrap;">
            {formatCurrency(p.amount, 2)}
          </div>
        </div>
      </div>
    {:else}
      <div class="is-flex is-justify-content-space-between is-hoverable" style="margin: 1px 0;">
        <div class="truncate custom-icon" style="min-width: 100px;" title={p.account}>
          <span style={accountColorStyle(firstName(p.account))}>{iconText(p.account)}</span>
          {p.account}
        </div>
        <div class="is-flex is-align-items-baseline is-justify-content-right">
          <div class="has-text-right has-text-grey is-size-7 mr-2 truncate">
            {conversion}
          </div>
          <div class="has-text-right" style="min-width: 50px;">
            {formatCurrency(p.amount, 2)}
          </div>
        </div>
      </div>
    {/if}
  {/each}
</div>
