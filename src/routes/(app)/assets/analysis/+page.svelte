<script lang="ts">
  import { generateColorScheme, genericBarColor } from "$lib/colors";
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import { filterCommodityBreakdowns, renderPortfolioBreakdown } from "$lib/portfolio";
  import { type PortfolioAggregate } from "$lib/utils";
  import { getPortfolioAllocation } from "$lib/api/assets";
  import _ from "lodash";
  import { onMount } from "svelte";
  import { t } from "$lib/i18n";

  let commodities = $state<string[]>([]);
  let selectedCommodities = $state<string[]>([]);
  let security_type = $state<PortfolioAggregate[]>([]);
  let name_and_security_type = $state<PortfolioAggregate[]>([]);
  let rating = $state<PortfolioAggregate[]>([]);
  let industry = $state<PortfolioAggregate[]>([]);
  let isEmpty = $state(false);
  let color = $state<ReturnType<typeof generateColorScheme>>();

  let securityTypeR = $state<ReturnType<typeof renderPortfolioBreakdown>>();
  let portfolioR = $state<ReturnType<typeof renderPortfolioBreakdown>>();
  let industryR = $state<ReturnType<typeof renderPortfolioBreakdown>>();
  let ratingR = $state<ReturnType<typeof renderPortfolioBreakdown>>();

  onMount(async () => {
    ({ name_and_security_type, security_type, rating, industry, commodities } =
      await getPortfolioAllocation());

    if (_.isEmpty(commodities)) {
      isEmpty = true;
      return;
    } else {
      isEmpty = false;
    }

    selectedCommodities = [...commodities];
    securityTypeR = renderPortfolioBreakdown("#d3-portfolio-security-type", security_type);
    ratingR = renderPortfolioBreakdown("#d3-portfolio-security-rating", rating);
    industryR = renderPortfolioBreakdown("#d3-portfolio-security-industry", industry, {
      z: [genericBarColor()]
    });
    portfolioR = renderPortfolioBreakdown("#d3-portfolio", name_and_security_type);
    color = generateColorScheme(commodities);
  });

  $effect(() => {
    if (securityTypeR) {
      securityTypeR.renderer(filterCommodityBreakdowns(security_type, selectedCommodities), color);
      ratingR.renderer(filterCommodityBreakdowns(rating, selectedCommodities), color);
      industryR.renderer(filterCommodityBreakdowns(industry, selectedCommodities), color);
      portfolioR.renderer(
        filterCommodityBreakdowns(name_and_security_type, selectedCommodities),
        color
      );
    }
  });
</script>

<section class="section tab-interest" class:is-hidden={!isEmpty}>
  <div class="container is-fluid">
    <div class="columns is-centered">
      <div class="column is-4 has-text-centered">
        <article class="message">
          <div class="message-body">
            <strong>{$t("common.oops")}</strong> {$t("assets.portfolioUnavailable")}<br /><br
            />
            <!-- eslint-disable-next-line svelte/no-at-html-tags -->
            {@html $t("assets.portfolioUpdateHint", { action: $t("actions.updatePortfolios") })}
          </div>
        </article>
      </div>
    </div>
  </div>
</section>

<section class="section tab-portfolio" class:is-hidden={isEmpty}>
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12 is-flex">
        {#each commodities as commodity}
          {@const name = `switch-${commodity}`}
          <div class="field mr-5 color-switch" style="--color: {color(commodity)}">
            <input
              id={name}
              type="checkbox"
              bind:group={selectedCommodities}
              name="commodities"
              class="switch is-rounded"
              value={commodity}
            />
            <label for={name}>{commodity}</label>
          </div>
        {/each}
      </div>
    </div>
    <div class="columns">
      <div class="column is-12 has-text-centered">
        <div class="box overflow-x-auto">
          <div
            id="d3-portfolio-security-type-treemap"
            style="width: 100%; position: relative"
          ></div>
          <svg id="d3-portfolio-security-type"></svg>
        </div>
      </div>
    </div>
    <BoxLabel text={$t("assets.securityType")} />

    <div class="columns">
      <div class="column is-12 has-text-centered">
        <div class="box overflow-x-auto">
          <div
            id="d3-portfolio-security-rating-treemap"
            style="width: 100%; position: relative"
          ></div>
          <svg id="d3-portfolio-security-rating"></svg>
        </div>
      </div>
    </div>
    <BoxLabel text={$t("assets.securityRating")} />

    <div class="columns">
      <div class="column is-12 has-text-centered">
        <div class="box overflow-x-auto">
          <div
            id="d3-portfolio-security-industry-treemap"
            style="width: 100%; position: relative"
          ></div>
          <svg id="d3-portfolio-security-industry"></svg>
        </div>
      </div>
    </div>
    <BoxLabel text={$t("assets.industry")} />

    <div class="columns">
      <div class="column is-12 has-text-centered">
        <div class="box overflow-x-auto">
          {#if portfolioR}
            <LegendCard legends={portfolioR.legends} clazz="ml-4" />
          {/if}
          <div id="d3-portfolio-treemap" style="width: 100%; position: relative"></div>
          <svg id="d3-portfolio"></svg>
        </div>
      </div>
    </div>
    <BoxLabel text={$t("assets.security")} />
  </div>
</section>

<style lang="scss">
  .color-switch {
    .switch[type="checkbox"]:checked + label::before,
    .switch[type="checkbox"]:checked + label:before {
      background: var(--color);
    }
  }
</style>
