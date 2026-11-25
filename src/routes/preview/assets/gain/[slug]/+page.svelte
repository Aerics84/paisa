<script lang="ts">
  import COLORS, { generateColorScheme, genericBarColor } from "$lib/colors";
  import { renderAccountOverview, buildLegends } from "$lib/gain";
  import { filterCommodityBreakdowns, renderPortfolioBreakdown } from "$lib/portfolio";
  import {
    type Posting,
    formatCurrency,
    formatFloat,
    type AccountGain,
    type Networth,
    type PortfolioAggregate,
    type AssetBreakdown,
    formatPercentage,
    formatFloatUptoPrecision
  } from "$lib/utils";
  import { getAccountGain } from "$lib/api/assets";
  import _ from "lodash";
  import { onMount, onDestroy } from "svelte";
  import type { PageData } from "./$types";
  import PostingCard from "$lib/components/PostingCard.svelte";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import { iconify } from "$lib/icon";
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import { t } from "$lib/i18n";

  let commodities = $state<string[]>([]);
  let selectedCommodities = $state<string[]>([]);
  let security_type = $state<PortfolioAggregate[]>([]);
  let name_and_security_type = $state<PortfolioAggregate[]>([]);
  let rating = $state<PortfolioAggregate[]>([]);
  let industry = $state<PortfolioAggregate[]>([]);
  let color = $state<ReturnType<typeof generateColorScheme>>();

  let securityTypeEmpty = $state(false);
  let nameAndSecurityTypeEmpty = $state(false);
  let ratingEmpty = $state(false);
  let industryEmpty = $state(false);

  let { data }: { data: PageData } = $props();
  let gain = $state<AccountGain>();
  let overview = $state<Networth>();
  let assetBreakdown = $state<AssetBreakdown>();
  let legends = buildLegends();

  let destroyCallback = () => {};
  let postings = $state<Posting[]>([]);

  let securityTypeR = $state<ReturnType<typeof renderPortfolioBreakdown>["renderer"]>();
  let portfolioR = $state<ReturnType<typeof renderPortfolioBreakdown>["renderer"]>();
  let industryR = $state<ReturnType<typeof renderPortfolioBreakdown>["renderer"]>();
  let ratingR = $state<ReturnType<typeof renderPortfolioBreakdown>["renderer"]>();

  onDestroy(async () => {
    destroyCallback();
  });

  onMount(async () => {
    ({
      gain_timeline_breakdown: gain,
      asset_breakdown: assetBreakdown,
      portfolio_allocation: { name_and_security_type, security_type, rating, industry, commodities }
    } = await getAccountGain(data.name));

    overview = _.last(gain.networthTimeline);
    postings = _.chain(gain.postings)
      .sortBy((p) => p.date)
      .reverse()
      .take(100)
      .value();
    destroyCallback = renderAccountOverview(
      gain.networthTimeline,
      gain.postings,
      "d3-account-timeline-breakdown"
    );

    selectedCommodities = [...commodities];
    ({ renderer: securityTypeR } = renderPortfolioBreakdown(
      "#d3-portfolio-security-type",
      security_type,
      {
        small: true
      }
    ));
    ({ renderer: ratingR } = renderPortfolioBreakdown("#d3-portfolio-security-rating", rating, {
      small: true
    }));
    ({ renderer: industryR } = renderPortfolioBreakdown(
      "#d3-portfolio-security-industry",
      industry,
      {
        small: true,
        z: [genericBarColor()]
      }
    ));
    ({ renderer: portfolioR } = renderPortfolioBreakdown("#d3-portfolio", name_and_security_type, {
      small: true
    }));

    if (commodities.length !== 0) {
      color = generateColorScheme(commodities);
    }

    securityTypeEmpty = security_type.length === 0;
    nameAndSecurityTypeEmpty = name_and_security_type.length === 0;
    ratingEmpty = rating.length === 0;
    industryEmpty = industry.length === 0;
  });

  $effect(() => {
    if (securityTypeR) {
      securityTypeR(filterCommodityBreakdowns(security_type, selectedCommodities), color);
      ratingR(filterCommodityBreakdowns(rating, selectedCommodities), color);
      industryR(filterCommodityBreakdowns(industry, selectedCommodities), color);
      portfolioR(filterCommodityBreakdowns(name_and_security_type, selectedCommodities), color);
    }
  });
</script>

<section class="section">
  <div class="container is-fluid">
    <div class="columns is-flex-wrap-wrap">
      <div class="column is-3">
        <div class="columns is-flex-wrap-wrap">
          {#if overview}
            <div class="column is-full">
              <div>
                <nav class="level grid-2">
                  <LevelItem
                    narrow
                    title={$t("nav.balance")}
                    color={COLORS.primary}
                    value={formatCurrency(overview.balanceAmount)}
                  />
                  <LevelItem
                    narrow
                    title={$t("dashboard.netInvestment")}
                    color={COLORS.secondary}
                    value={formatCurrency(overview.netInvestmentAmount)}
                  />
                </nav>
              </div>
            </div>
            <div class="column is-full">
              <div>
                <nav class="level grid-2">
                  <LevelItem
                    narrow
                    title={$t("dashboard.gainLoss")}
                    color={overview.gainAmount >= 0 ? COLORS.gainText : COLORS.lossText}
                    value={formatCurrency(overview.gainAmount)}
                  />

                  <LevelItem
                    narrow
                    title="XIRR"
                    value={formatFloat(gain.xirr)}
                    subtitle={$t("assets.absoluteReturn", {
                      percentage: formatPercentage(assetBreakdown.absoluteReturn, 2)
                    })}
                  />
                </nav>
              </div>
            </div>
          {/if}

          <div class="column is-full">
            {$t("nav.postings")}
            {#each postings as posting}
              <PostingCard
                {posting}
                color={posting.amount >= 0
                  ? posting.account.startsWith("Income:CapitalGains")
                    ? COLORS.loss
                    : COLORS.secondary
                  : posting.account.startsWith("Income:CapitalGains")
                    ? COLORS.gain
                    : COLORS.tertiary}
              />
            {/each}
          </div>
        </div>
      </div>
      <div class="column is-9">
        {#if overview}
          <div class="box py-2 mb-4 mt-0">
            <div class="is-flex mr-2 is-align-items-baseline" style="min-width: fit-content">
              <div class="ml-3 custom-icon is-size-5">
                <span>{iconify(data.name)}</span>
              </div>
              <div class="ml-3">
                <span class="mr-1 is-size-7 has-text-grey">{$t("nav.investment")}</span>
                <span class="has-text-weight-bold">{formatCurrency(overview.investmentAmount)}</span
                >
              </div>
              <div class="ml-3">
                <span class="mr-1 is-size-7 has-text-grey">{$t("assets.withdrawal")}</span>
                <span class="has-text-weight-bold">{formatCurrency(overview.withdrawalAmount)}</span
                >
              </div>
              {#if overview.balanceUnits > 0}
                <div class="ml-3">
                  <span class="mr-1 is-size-7 has-text-grey">{$t("assets.balanceUnits")}</span>
                  <span class="has-text-weight-bold"
                    >{formatFloatUptoPrecision(overview.balanceUnits, 4)}</span
                  >
                </div>
              {/if}
            </div>
          </div>
        {/if}
        <LegendCard {legends} clazz="mb-2" />
        <div class="box">
          <svg id="d3-account-timeline-breakdown" width="100%" height="450" />
        </div>
        <BoxLabel text={$t("assets.timeline")} />

        <div class="columns">
          <div class="column is-6">
            <div class="mt-5" class:is-hidden={securityTypeEmpty}>
              <div class="box overflow-x-auto">
                <div
                  id="d3-portfolio-security-type-treemap"
                  style="width: 100%; position: relative"
                ></div>
                <svg id="d3-portfolio-security-type" width="100%"></svg>
              </div>
              <BoxLabel text={$t("assets.securityType")} />
            </div>

            <div class="mt-5" class:is-hidden={ratingEmpty}>
              <div class="box overflow-x-auto">
                <div
                  id="d3-portfolio-security-rating-treemap"
                  style="width: 100%; position: relative"
                ></div>
                <svg id="d3-portfolio-security-rating" width="100%"></svg>
              </div>
              <BoxLabel text={$t("assets.securityRating")} />
            </div>

            <div class="mt-5" class:is-hidden={industryEmpty}>
              <div class="box overflow-x-auto">
                <div
                  id="d3-portfolio-security-industry-treemap"
                  style="width: 100%; position: relative"
                ></div>
                <svg id="d3-portfolio-security-industry" width="100%"></svg>
              </div>
              <BoxLabel text={$t("assets.industry")} />
            </div>
          </div>
          <div class="column is-6 mt-5">
            <div class:is-hidden={nameAndSecurityTypeEmpty}>
              <div class="box overflow-x-auto">
                <div id="d3-portfolio-treemap" style="width: 100%; position: relative"></div>
                <svg id="d3-portfolio" width="100%"></svg>
              </div>
              <BoxLabel text={$t("assets.security")} />
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</section>
