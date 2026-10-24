<script lang="ts">
  import { generateColorScheme, genericBarColor } from "$lib/colors";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import PreviewEmpty from "$lib/components/preview/PreviewEmpty.svelte";
  import PreviewKpi from "$lib/components/preview/PreviewKpi.svelte";
  import PreviewKpiStrip from "$lib/components/preview/PreviewKpiStrip.svelte";
  import PreviewPage from "$lib/components/preview/PreviewPage.svelte";
  import PreviewPanel from "$lib/components/preview/PreviewPanel.svelte";
  import { filterCommodityBreakdowns, renderPortfolioBreakdown } from "$lib/portfolio";
  import { ajax, type PortfolioAggregate } from "$lib/utils";
  import _ from "lodash";
  import { onMount, tick } from "svelte";

  let commodities: string[] = [];
  let selectedCommodities: string[] = [];
  let security_type: PortfolioAggregate[] = [];
  let name_and_security_type: PortfolioAggregate[] = [];
  let rating: PortfolioAggregate[] = [];
  let industry: PortfolioAggregate[] = [];
  let isEmpty = false;
  let color: any;

  let securityTypeR: any,
    portfolioR: any,
    industryR: any,
    ratingR: any = null;
  let loadError = false;

  onMount(async () => {
    try {
      ({ name_and_security_type, security_type, rating, industry, commodities } = await ajax(
        "/api/portfolio_allocation"
      ));

      if (_.isEmpty(commodities)) {
        isEmpty = true;
        loadError = false;
        return;
      } else {
        isEmpty = false;
      }

      selectedCommodities = [...commodities];
      await tick();
      securityTypeR = renderPortfolioBreakdown("#d3-portfolio-security-type", security_type);
      ratingR = renderPortfolioBreakdown("#d3-portfolio-security-rating", rating);
      industryR = renderPortfolioBreakdown("#d3-portfolio-security-industry", industry, {
        z: [genericBarColor()]
      });
      portfolioR = renderPortfolioBreakdown("#d3-portfolio", name_and_security_type);
      color = generateColorScheme(commodities);
      loadError = false;
    } catch (_error) {
      loadError = true;
      isEmpty = true;
    }
  });

  $: if (securityTypeR) {
    securityTypeR.renderer(filterCommodityBreakdowns(security_type, selectedCommodities), color);
    ratingR.renderer(filterCommodityBreakdowns(rating, selectedCommodities), color);
    industryR.renderer(filterCommodityBreakdowns(industry, selectedCommodities), color);
    portfolioR.renderer(
      filterCommodityBreakdowns(name_and_security_type, selectedCommodities),
      color
    );
  }
</script>

<PreviewPage
  eyebrow="Distribution Analysis"
  title="Portfolio Analysis"
  description="Inspect portfolio structure by security type, rating, industry, and individual security composition."
>
  <PreviewKpiStrip>
    <PreviewKpi label="Commodities" value={String(commodities.length)} tone="accent" />
    <PreviewKpi label="Security types" value={String(security_type.length)} />
    <PreviewKpi label="Ratings" value={String(rating.length)} />
    <PreviewKpi label="Industries" value={String(industry.length)} />
  </PreviewKpiStrip>

  {#if loadError}
    <PreviewPanel
      title="Portfolio metadata"
      subtle="The portfolio analysis endpoint is unavailable."
    >
      <PreviewEmpty
        title="Portfolio analysis is unavailable"
        message="The backend returned an error for this endpoint. Diagnostic breakdowns are not shown."
      />
    </PreviewPanel>
  {:else if isEmpty}
    <PreviewPanel title="Portfolio metadata" subtle="Mutual fund portfolio data is required.">
      <PreviewEmpty
        title="Portfolio data is not available"
        message="Use the Update Mutual Fund Portfolios action before these diagnostic breakdowns can render."
      />
    </PreviewPanel>
  {:else}
    <PreviewPanel
      title="Commodity filter"
      subtle="Limit the diagnostic breakdowns to selected commodities."
    >
      <div class="is-flex is-flex-wrap-wrap">
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
    </PreviewPanel>

    <div class="paisa-preview-grid">
      <PreviewPanel
        title="Security type"
        subtle="Portfolio composition grouped by instrument type."
      >
        <div class="paisa-preview-chart">
          <div
            id="d3-portfolio-security-type-treemap"
            style="width: 100%; position: relative"
          ></div>
          <svg id="d3-portfolio-security-type"></svg>
        </div>
      </PreviewPanel>

      <PreviewPanel
        title="Security rating"
        subtle="Rating distribution where metadata is available."
      >
        <div class="paisa-preview-chart">
          <div
            id="d3-portfolio-security-rating-treemap"
            style="width: 100%; position: relative"
          ></div>
          <svg id="d3-portfolio-security-rating"></svg>
        </div>
      </PreviewPanel>
    </div>

    <div class="paisa-preview-grid">
      <PreviewPanel title="Industry" subtle="Industry concentration across selected commodities.">
        <div class="paisa-preview-chart">
          <div
            id="d3-portfolio-security-industry-treemap"
            style="width: 100%; position: relative"
          ></div>
          <svg id="d3-portfolio-security-industry"></svg>
        </div>
      </PreviewPanel>

      <PreviewPanel title="Security" subtle="Individual security composition.">
        <div class="paisa-preview-chart">
          {#if portfolioR}
            <LegendCard legends={portfolioR.legends} clazz="mb-3" />
          {/if}
          <div id="d3-portfolio-treemap" style="width: 100%; position: relative"></div>
          <svg id="d3-portfolio"></svg>
        </div>
      </PreviewPanel>
    </div>
  {/if}
</PreviewPage>

<style lang="scss">
  .color-switch {
    .switch[type="checkbox"]:checked + label::before,
    .switch[type="checkbox"]:checked + label:before {
      background: var(--color);
    }
  }
</style>
