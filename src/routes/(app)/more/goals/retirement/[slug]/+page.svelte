<script lang="ts">
  import COLORS from "$lib/colors";
  import {
    formatCurrency,
    formatFloat,
    isMobile,
    type AssetBreakdown,
    type Point,
    type Posting,
    type GoalIntelligenceData
  } from "$lib/utils";
  import { getRetirementGoal } from "$lib/api/goals";
  import { onMount, tick, onDestroy } from "svelte";
  import ARIMAPromise from "arima/async";
  import {
    forecastRetirement,
    renderProgress,
    findBreakPoints,
    renderInvestmentTimeline,
    analyzeGoal,
    goalDelayLabel,
    goalStatusLabel,
    type ForecastMode,
    type GoalIntelligence
  } from "$lib/goals";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import type { PageData } from "./$types";
  import { iconGlyph } from "$lib/icon";
  import _ from "lodash";
  import PostingGroup from "$lib/components/PostingGroup.svelte";
  import PostingCard from "$lib/components/PostingCard.svelte";
  import ProgressWithBreakpoints from "$lib/components/ProgressWithBreakpoints.svelte";
  import AssetsBalance from "$lib/components/AssetsBalance.svelte";
  import BoxLabel from "$lib/components/BoxLabel.svelte";

  let { data }: { data: PageData } = $props();

  let svg = $state<Element>();
  let investmentTimelineSvg = $state<Element>();
  let savingsTotal = $state(0);
  let investmentTotal = $state(0);
  let gainTotal = $state(0);
  let icon = $state("");
  let name = $state("");
  let targetSavings = $state(0);
  let monthlyContribution = $state(0);
  let forecastRate = $state(0);
  let swr = $state(0);
  let xirr = $state(0);
  let yearlyExpense = $state(0);
  let progressPercent = $state(0);
  let savingsX = $state(0);
  let targetX = $state(0);
  let forecastUnavailable = $state(false);
  let forecastMode = $state<ForecastMode>("none");
  let predictedTargetDate = $state("");
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
      targetSavings,
      monthlyContribution,
      forecastRate,
      yearlyExpense,
      swr,
      xirr,
      icon,
      name,
      postings,
      balances,
      intelligence: intelligenceData
    } = await getRetirementGoal(data.name));

    latestPostings = _.chain(postings)
      .sortBy((p) => p.date)
      .reverse()
      .take(100)
      .value();

    const goal = {
      type: "retirement",
      name,
      id: `retirement-${name}`,
      icon,
      current: savingsTotal,
      target: targetSavings,
      targetDate: "",
      priority: 0,
      intelligence: intelligenceData
    };
    intelligence = analyzeGoal(goal);
    statusLabel = goalStatusLabel(goal, intelligence);
    projectionDate = intelligence.projectedCompletionDate?.format("DD MMM YYYY") ?? "";
    delayLabel = goalDelayLabel(intelligence);
    predictedTargetDate = projectionDate;

    if (yearlyExpense > 0) {
      progressPercent = Math.max(0, Math.min(100, (intelligenceData?.progressRatio ?? 0) * 100));
      savingsX = savingsTotal / yearlyExpense;
      targetX = targetSavings / yearlyExpense;
    }

    if (targetX <= 0 || savingsX <= 0 || yearlyExpense <= 0) {
      return;
    }

    const ARIMA = await ARIMAPromise;
    const { predictionsTimeline, forecastMode: mode } = forecastRetirement(
      savingsTimeline,
      targetSavings,
      savingsTotal,
      monthlyContribution,
      ARIMA,
      forecastRate
    );
    forecastMode = mode;
    forecastUnavailable = forecastMode === "none" && savingsTotal < targetSavings;
    predictedTargetDate =
      projectionDate || _.last(predictionsTimeline)?.date.format("DD MMM YYYY") || "";
    await tick();
    breakPoints = findBreakPoints(savingsTimeline.concat(predictionsTimeline), targetSavings);
    destroyCallback = renderProgress(savingsTimeline, predictionsTimeline, breakPoints, svg, {
      targetSavings
    });

    renderInvestmentTimeline(postings, investmentTimelineSvg, 0);
  });
</script>

<section class="section">
  <div class="container is-fluid">
    <nav class="level custom-icon {isMobile() && 'grid-2'}">
      <LevelItem title={name} value={iconGlyph(icon)} />
      <LevelItem
        title="Net Investment"
        value={formatCurrency(investmentTotal)}
        color={COLORS.secondary}
        subtitle={`<b>${formatCurrency(gainTotal)}</b> ${
          gainTotal >= 0 ? "gain" : "loss"
        } at <b>${formatFloat(xirr)}</b> XIRR`}
      />

      <LevelItem
        title="Current Savings"
        value={formatCurrency(savingsTotal)}
        color={COLORS.gainText}
        subtitle={`${formatFloat(savingsX, 0)}× Yearly Expenses`}
      />
      <LevelItem
        title="Yearly Expenses"
        color={COLORS.lossText}
        value={formatCurrency(yearlyExpense)}
      />

      <LevelItem
        title="Target Savings"
        value={formatCurrency(targetSavings)}
        color={COLORS.primary}
        subtitle={`${formatFloat(targetX, 0)}× Yearly Expenses${
          predictedTargetDate
            ? `<br/>Projected <b>${predictedTargetDate}</b>`
            : savingsTotal >= targetSavings
              ? "<br/>Target achieved"
              : ""
        }`}
      />
      <LevelItem
        title="Goal Status"
        value={statusLabel}
        color={intelligence?.attention === "critical"
          ? COLORS.lossText
          : intelligence?.attention === "warning"
            ? COLORS.tertiary
            : COLORS.gainText}
        subtitle={delayLabel || null}
      />
      <LevelItem title="SWR" value={formatFloat(swr)} />
    </nav>
  </div>
</section>

<section class="section">
  <div class="container is-fluid">
    <ProgressWithBreakpoints {progressPercent} {breakPoints} />
    {#if forecastUnavailable}
      <div class="notification is-light mt-4 has-text-grey">
        Paisa could not estimate when this retirement goal will be reached from the current savings
        history.
      </div>
    {/if}
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
        <BoxLabel text="{iconGlyph(icon)} {name} progress" />
        <div class="columns">
          <div class="column is-12">
            <div class="box overflow-x-auto">
              <svg height="300" width="100%" bind:this={investmentTimelineSvg} />
            </div>
          </div>
        </div>
        <BoxLabel text="Monthly Investment" />
        <div class="columns">
          <div class="column is-12 has-text-grey">
            <AssetsBalance breakdowns={balances} indent={false} />
          </div>
        </div>
        <BoxLabel text="Current Balance" />
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
