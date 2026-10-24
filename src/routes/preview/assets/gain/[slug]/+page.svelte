<script lang="ts">
  import COLORS, { generateColorScheme, genericBarColor } from "$lib/colors";
  import { renderAccountOverview, buildLegends } from "$lib/gain";
  import { filterCommodityBreakdowns, renderPortfolioBreakdown } from "$lib/portfolio";
  import {
    ajax,
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
  import _ from "lodash";
  import { onMount, onDestroy, tick } from "svelte";
  import type { PageData } from "./$types";
  import PostingCard from "$lib/components/PostingCard.svelte";
  import { iconify } from "$lib/icon";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import PreviewEmpty from "$lib/components/preview/PreviewEmpty.svelte";
  import PreviewKpi from "$lib/components/preview/PreviewKpi.svelte";
  import PreviewKpiStrip from "$lib/components/preview/PreviewKpiStrip.svelte";
  import PreviewPage from "$lib/components/preview/PreviewPage.svelte";
  import PreviewPanel from "$lib/components/preview/PreviewPanel.svelte";

  let commodities: string[] = [];
  let selectedCommodities: string[] = [];
  let security_type: PortfolioAggregate[] = [];
  let name_and_security_type: PortfolioAggregate[] = [];
  let rating: PortfolioAggregate[] = [];
  let industry: PortfolioAggregate[] = [];
  let color: any;

  let securityTypeEmpty: boolean = false;
  let nameAndSecurityTypeEmpty: boolean = false;
  let ratingEmpty: boolean = false;
  let industryEmpty: boolean = false;

  export let data: PageData;
  let gain: AccountGain;
  let overview: Networth;
  let assetBreakdown: AssetBreakdown;
  let legends = buildLegends();

  let destroyCallback = () => {};
  let postings: Posting[] = [];
  let loadError = false;

  let securityTypeR: any,
    portfolioR: any,
    industryR: any,
    ratingR: any = null;

  onDestroy(async () => {
    destroyCallback();
  });

  onMount(async () => {
    try {
      ({
        gain_timeline_breakdown: gain,
        asset_breakdown: assetBreakdown,
        portfolio_allocation: {
          name_and_security_type,
          security_type,
          rating,
          industry,
          commodities
        }
      } = await ajax("/api/gain/:name", null, data));

      overview = _.last(gain.networthTimeline);
      postings = _.chain(gain.postings)
        .sortBy((p) => p.date)
        .reverse()
        .take(100)
        .value();
      if (commodities.length !== 0) {
        color = generateColorScheme(commodities);
      }
      selectedCommodities = [...commodities];

      securityTypeEmpty = security_type.length === 0;
      nameAndSecurityTypeEmpty = name_and_security_type.length === 0;
      ratingEmpty = rating.length === 0;
      industryEmpty = industry.length === 0;
      await tick();

      destroyCallback = renderAccountOverview(
        gain.networthTimeline,
        gain.postings,
        "d3-account-timeline-breakdown"
      );

      if (!securityTypeEmpty) {
        ({ renderer: securityTypeR } = renderPortfolioBreakdown(
          "#d3-portfolio-security-type",
          security_type,
          {
            small: true
          }
        ));
      }

      if (!ratingEmpty) {
        ({ renderer: ratingR } = renderPortfolioBreakdown("#d3-portfolio-security-rating", rating, {
          small: true
        }));
      }

      if (!industryEmpty) {
        ({ renderer: industryR } = renderPortfolioBreakdown(
          "#d3-portfolio-security-industry",
          industry,
          {
            small: true,
            z: [genericBarColor()]
          }
        ));
      }

      if (!nameAndSecurityTypeEmpty) {
        ({ renderer: portfolioR } = renderPortfolioBreakdown(
          "#d3-portfolio",
          name_and_security_type,
          {
            small: true
          }
        ));
      }
      loadError = false;
    } catch (_error) {
      loadError = true;
    }
  });

  $: if (securityTypeR) {
    securityTypeR(filterCommodityBreakdowns(security_type, selectedCommodities), color);
  }

  $: if (ratingR) {
    ratingR(filterCommodityBreakdowns(rating, selectedCommodities), color);
  }

  $: if (industryR) {
    industryR(filterCommodityBreakdowns(industry, selectedCommodities), color);
  }

  $: if (portfolioR) {
    portfolioR(filterCommodityBreakdowns(name_and_security_type, selectedCommodities), color);
  }
</script>

<PreviewPage
  eyebrow="Detail"
  title={data.name}
  description="Position-level performance, timeline events, portfolio metadata, and supporting postings."
>
  {#if loadError}
    <PreviewPanel title="Position unavailable" subtle="The gain detail endpoint is unavailable.">
      <PreviewEmpty
        title="Position data is unavailable"
        message="The backend returned an error for this endpoint. No synthetic position values are shown."
      />
    </PreviewPanel>
  {:else if overview}
    <PreviewKpiStrip>
      <PreviewKpi label="Balance" value={formatCurrency(overview.balanceAmount)} tone="accent" />
      <PreviewKpi label="Net investment" value={formatCurrency(overview.netInvestmentAmount)} />
      <PreviewKpi
        label="Gain / Loss"
        value={formatCurrency(overview.gainAmount)}
        tone={overview.gainAmount >= 0 ? "positive" : "negative"}
      />
      <PreviewKpi
        label="XIRR"
        value={formatFloat(gain.xirr)}
        detail={`${formatPercentage(assetBreakdown.absoluteReturn, 2)} absolute return`}
      />
    </PreviewKpiStrip>

    <div class="paisa-preview-grid paisa-preview-grid--sidebar">
      <PreviewPanel title="Timeline story" subtle="Investment, withdrawal, and return events.">
        <LegendCard {legends} clazz="mb-3" />
        <div class="paisa-preview-chart">
          <svg id="d3-account-timeline-breakdown" width="100%" height="450"></svg>
        </div>
      </PreviewPanel>

      <PreviewPanel title="Fact panel" subtle="Latest live position state.">
        <div class="paisa-preview-list">
          <div class="paisa-preview-list__item">
            <span>Instrument</span>
            <strong><span>{iconify(data.name)}</span> {data.name}</strong>
          </div>
          <div class="paisa-preview-list__item">
            <span>Investment</span>
            <strong>{formatCurrency(overview.investmentAmount)}</strong>
          </div>
          <div class="paisa-preview-list__item">
            <span>Withdrawal</span>
            <strong>{formatCurrency(overview.withdrawalAmount)}</strong>
          </div>
          {#if overview.balanceUnits > 0}
            <div class="paisa-preview-list__item">
              <span>Balance units</span>
              <strong>{formatFloatUptoPrecision(overview.balanceUnits, 4)}</strong>
            </div>
          {/if}
        </div>
      </PreviewPanel>
    </div>

    <div class="paisa-preview-grid">
      <PreviewPanel title="Security type" subtle="Metadata composition for this position.">
        {#if !securityTypeEmpty}
          <div class="paisa-preview-chart">
            <div
              id="d3-portfolio-security-type-treemap"
              style="width: 100%; position: relative"
            ></div>
            <svg id="d3-portfolio-security-type" width="100%"></svg>
          </div>
        {:else}
          <PreviewEmpty
            title="No security type data"
            message="This position has no security type metadata."
          />
        {/if}
      </PreviewPanel>

      <PreviewPanel title="Security rating" subtle="Rating breakdown when metadata is available.">
        {#if !ratingEmpty}
          <div class="paisa-preview-chart">
            <div
              id="d3-portfolio-security-rating-treemap"
              style="width: 100%; position: relative"
            ></div>
            <svg id="d3-portfolio-security-rating" width="100%"></svg>
          </div>
        {:else}
          <PreviewEmpty title="No rating data" message="This position has no rating metadata." />
        {/if}
      </PreviewPanel>
    </div>

    <div class="paisa-preview-grid">
      <PreviewPanel title="Industry" subtle="Industry exposure for this position.">
        {#if !industryEmpty}
          <div class="paisa-preview-chart">
            <div
              id="d3-portfolio-security-industry-treemap"
              style="width: 100%; position: relative"
            ></div>
            <svg id="d3-portfolio-security-industry" width="100%"></svg>
          </div>
        {:else}
          <PreviewEmpty
            title="No industry data"
            message="This position has no industry metadata."
          />
        {/if}
      </PreviewPanel>

      <PreviewPanel title="Security" subtle="Security composition within this position.">
        {#if !nameAndSecurityTypeEmpty}
          <div class="paisa-preview-chart">
            <div id="d3-portfolio-treemap" style="width: 100%; position: relative"></div>
            <svg id="d3-portfolio" width="100%"></svg>
          </div>
        {:else}
          <PreviewEmpty
            title="No security composition"
            message="No security composition data is available."
          />
        {/if}
      </PreviewPanel>
    </div>

    <PreviewPanel title="Postings" subtle="Recent entries supporting the position story.">
      {#if postings.length}
        <div class="columns is-flex-wrap-wrap">
          {#each postings as posting}
            <div class="column is-full-tablet is-half-desktop is-one-third-widescreen">
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
            </div>
          {/each}
        </div>
      {:else}
        <PreviewEmpty
          title="No postings"
          message="No recent postings were returned for this position."
        />
      {/if}
    </PreviewPanel>
  {:else}
    <PreviewPanel title="Position unavailable" subtle="No live overview was returned.">
      <PreviewEmpty
        title="No position data"
        message="This detail page needs gain timeline data before it can render."
      />
    </PreviewPanel>
  {/if}
</PreviewPage>
