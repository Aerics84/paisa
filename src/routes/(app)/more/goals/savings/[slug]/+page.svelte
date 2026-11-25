<script lang="ts">
  import COLORS from "$lib/colors";
  import {
    formatCurrency,
    formatFloat,
    isMobile,
    type Forecast,
    type Point,
    type Posting,
    type AssetBreakdown,
    type GoalIntelligenceData
  } from "$lib/utils";
  import { getSavingsGoal } from "$lib/api/goals";
  import { onMount, tick, onDestroy } from "svelte";
  import ARIMAPromise from "arima/async";
  import {
    forecast,
    renderProgress,
    findBreakPoints,
    project,
    solvePMTOrNper,
    renderInvestmentTimeline,
    analyzeGoal,
    goalDelayLabel,
    goalStatusLabel,
    type GoalIntelligence
  } from "$lib/goals";
  import _ from "lodash";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import type { PageData } from "./$types";
  import PostingCard from "$lib/components/PostingCard.svelte";
  import PostingGroup from "$lib/components/PostingGroup.svelte";
  import { iconGlyph } from "$lib/icon";
  import dayjs from "dayjs";
  import ProgressWithBreakpoints from "$lib/components/ProgressWithBreakpoints.svelte";
  import AssetsBalance from "$lib/components/AssetsBalance.svelte";
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import { t } from "$lib/i18n";

  let { data }: { data: PageData } = $props();

  let svg = $state<Element>();
  let investmentTimelineSvg = $state<Element>();
  let targetDateObject = $state<dayjs.Dayjs>();
  let savingsTotal = $state(0);
  let investmentTotal = $state(0);
  let gainTotal = $state(0);
  let targetSavings = $state(0);
  let pmt = $state(0);
  let xirr = $state(0);
  let rate = $state(0);
  let paymentPerPeriod = $state(0);
  let targetDate = $state("");
  let name = $state("");
  let icon = $state("");
  let progressPercent = $state(0);
  let breakPoints = $state<Point[]>([]);
  let savingsTimeline = $state<Point[]>([]);
  let postings = $state<Posting[]>([]);
  let latestPostings = $state<Posting[]>([]);
  let balances = $state<Record<string, AssetBreakdown>>({});
  let intelligence = $state<GoalIntelligence | undefined>();
  let intelligenceData = $state<GoalIntelligenceData | undefined>();
  let statusLabel = $state("");
  let projectionDate = $state("");
  let delayLabel = $state("");
  let destroyCallback = () => {};

  onDestroy(async () => {
    destroyCallback();
  });

  onMount(async () => {
    ({
      savingsTotal,
      investmentTotal,
      gainTotal,
      savingsTimeline,
      target: targetSavings,
      rate,
      targetDate,
      postings,
      icon,
      name,
      xirr,
      paymentPerPeriod,
      balances,
      intelligence: intelligenceData
    } = await getSavingsGoal(data.name));

    latestPostings = _.chain(postings)
      .sortBy((p) => p.date)
      .reverse()
      .take(100)
      .value();

    if (targetSavings != 0) {
      progressPercent = Math.max(0, Math.min(100, (intelligenceData?.progressRatio ?? 0) * 100));
    }

    const goal = {
      type: "savings",
      name,
      id: `savings-${name}`,
      icon,
      current: savingsTotal,
      target: targetSavings,
      targetDate,
      priority: 0,
      intelligence: intelligenceData
    };
    intelligence = analyzeGoal(goal);
    statusLabel = goalStatusLabel(goal, intelligence);
    projectionDate = intelligence.projectedCompletionDate?.format("DD MMM YYYY") ?? "";
    delayLabel = goalDelayLabel(intelligence);

    ({ pmt, targetDate } = solvePMTOrNper(
      targetSavings,
      rate,
      savingsTotal,
      paymentPerPeriod,
      targetDate
    ));

    let predictionsTimeline: Forecast[] = [];
    targetDateObject = dayjs(targetDate, "YYYY-MM-DD", true);
    if (targetDateObject.isValid()) {
      predictionsTimeline = project(targetSavings, rate, targetDateObject, pmt, savingsTotal);
    } else if (savingsTotal < targetSavings) {
      const ARIMA = await ARIMAPromise;
      predictionsTimeline = forecast(savingsTimeline, targetSavings, ARIMA);
    }

    await tick();
    breakPoints = findBreakPoints(savingsTimeline.concat(predictionsTimeline), targetSavings);
    destroyCallback = renderProgress(savingsTimeline, predictionsTimeline, breakPoints, svg, {
      targetSavings
    });

    renderInvestmentTimeline(postings, investmentTimelineSvg, pmt);
  });
</script>

<section class="section">
  <div class="container is-fluid">
    <nav class="level custom-icon {isMobile() && 'grid-2'}">
      <LevelItem title={name} value={iconGlyph(icon)} />
      <LevelItem
        title={$t("dashboard.netInvestment")}
        value={formatCurrency(investmentTotal)}
        color={COLORS.secondary}
        subtitle={$t(gainTotal >= 0 ? "more.goal.gain" : "more.goal.loss", {
          amount: formatCurrency(gainTotal)
        })}
      />

      <LevelItem
        title={$t("more.goal.currentSavings")}
        value={formatCurrency(savingsTotal)}
        color={COLORS.gainText}
        subtitle={$t("more.goal.xirr", { xirr: formatFloat(xirr) })}
      />

      <LevelItem
        title={$t("more.goal.targetSavings")}
        value={formatCurrency(targetSavings)}
        color={COLORS.primary}
        subtitle={targetDateObject?.isValid() ? targetDateObject.format("DD MMM YYYY") : null}
      />

      {#if intelligence?.requiredMonthlyContribution !== undefined}
        <LevelItem
          title={$t("more.goal.monthlyInvestmentNeeded")}
          value={formatCurrency(intelligence.requiredMonthlyContribution)}
          color={COLORS.secondary}
          subtitle={rate > 0
            ? $t("more.goal.expectedReturn", { rate: formatFloat(rate, 2) })
            : null}
        />
      {/if}
      <LevelItem
        title={$t("more.goal.goalStatus")}
        value={statusLabel}
        color={intelligence?.attention === "critical"
          ? COLORS.lossText
          : intelligence?.attention === "warning"
            ? COLORS.tertiary
            : COLORS.gainText}
        subtitle={delayLabel || null}
      />
      {#if projectionDate}
        <LevelItem
          title={$t("more.goal.projection")}
          value={projectionDate}
          color={COLORS.primary}
          subtitle={delayLabel || null}
        />
      {/if}
    </nav>
  </div>
</section>

<section class="section">
  <div class="container is-fluid">
    <ProgressWithBreakpoints {progressPercent} {breakPoints} />
  </div>
</section>

<section class="section tab-retirement-progress">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-9">
        <div class="columns flex-wrap">
          <div class="column is-12">
            <div class="box overflow-x-auto">
              <svg height="400" bind:this={svg} />
            </div>
          </div>
        </div>
        <BoxLabel text={`${iconGlyph(icon)} ${$t("more.goal.progress", { name })}`} />
        <div class="columns">
          <div class="column is-12">
            <div class="box overflow-x-auto">
              <svg height="300" width="100%" bind:this={investmentTimelineSvg} />
            </div>
          </div>
        </div>
        <BoxLabel text={$t("more.goal.monthlyInvestment")} />
        <div class="columns">
          <div class="column is-12 has-text-grey">
            <AssetsBalance breakdowns={balances} indent={false} />
          </div>
        </div>
        <BoxLabel text={$t("more.goal.currentBalance")} />
      </div>
      <div class="column is-3">
        <PostingGroup postings={latestPostings} groupFormat="MMM YYYY" let:groupedPostings>
          <div>
            {#each groupedPostings as posting}
              <PostingCard
                {posting}
                color={posting.amount >= 0
                  ? posting.account.startsWith("Income:CapitalGains")
                    ? COLORS.tertiary
                    : COLORS.secondary
                  : posting.account.startsWith("Income:CapitalGains")
                    ? COLORS.secondary
                    : COLORS.tertiary}
              />
            {/each}
          </div>
        </PostingGroup>
      </div>
    </div>
  </div>
</section>
