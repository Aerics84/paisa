<script lang="ts">
  import CreditCardCard from "$lib/components/CreditCardCard.svelte";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import { getCreditCards } from "$lib/api/credit_cards";
  import { helpUrl, type CreditCardSummary } from "$lib/utils";
  import _ from "lodash";
  import { onMount } from "svelte";
  import { t } from "$lib/i18n";

  let isEmpty = false;
  let creditCards: CreditCardSummary[] = [];

  onMount(async () => {
    ({ creditCards } = await getCreditCards());
    if (_.isEmpty(creditCards)) {
      isEmpty = true;
    }
  });
</script>

<section class="section">
  <div class="container is-fluid">
    <div class="columns flex-wrap">
      <div class="column is-12">
        <div class="credit-card-container">
          {#each creditCards as creditCard}
            <CreditCardCard {creditCard} />
          {/each}
        </div>
      </div>
    </div>
    <div class="columns flex-wrap">
      <div class="column is-12">
        <ZeroState item={!isEmpty}>
          <strong>{$t("common.oops")}</strong>
          <!-- eslint-disable-next-line svelte/no-at-html-tags -->
          {@html $t("liabilities.noCreditCards", {
            link: `<a href="${helpUrl("credit-card")}">${$t("common.docs")}</a>`
          })}
        </ZeroState>
      </div>
    </div>
  </div>
</section>
