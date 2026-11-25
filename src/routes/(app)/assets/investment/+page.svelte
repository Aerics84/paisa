<script lang="ts">
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import {
    renderMonthlyInvestmentTimeline,
    renderYearlyCards,
    renderYearlyInvestmentTimeline
  } from "$lib/investment";
  import { type Legend } from "$lib/utils";
  import { getInvestment } from "$lib/api/assets";
  import _ from "lodash";
  import { onMount } from "svelte";
  import { t } from "$lib/i18n";

  let monthlyInvestmentTimelineLegends: Legend[] = [];
  let yearlyInvestmentTimelineLegends: Legend[] = [];

  onMount(async () => {
    const { assets: assets, yearly_cards: yearlyCards } = await getInvestment();
    monthlyInvestmentTimelineLegends = renderMonthlyInvestmentTimeline(assets);
    yearlyInvestmentTimelineLegends = renderYearlyInvestmentTimeline(yearlyCards);
    renderYearlyCards(yearlyCards);
  });
</script>

<section class="section tab-investment">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        <div class="box">
          <LegendCard legends={monthlyInvestmentTimelineLegends} clazz="ml-4" />
          <svg id="d3-investment-timeline" width="100%" height="500"></svg>
        </div>
      </div>
    </div>
    <BoxLabel text={$t("assets.monthlyInvestmentTimeline")} />
  </div>
</section>
<section class="section tab-investment">
  <div class="container is-fluid">
    <div class="columns is-flex-wrap-wrap">
      <div class="column is-full-tablet is-half-fullhd">
        <div class="box px-2">
          <LegendCard legends={yearlyInvestmentTimelineLegends} clazz="ml-4" />
          <svg id="d3-yearly-investment-timeline" width="100%"></svg>
        </div>
        <BoxLabel text={$t("assets.financialYearInvestmentTimeline")} />
      </div>
      <div class="column is-full-tablet is-half-fullhd">
        <div class="columns is-flex-wrap-wrap" id="d3-yearly-investment-cards"></div>
      </div>
    </div>
  </div>
</section>
