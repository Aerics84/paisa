<script lang="ts">
  import * as d3 from "d3";
  import dayjs from "dayjs";
  import _ from "lodash";
  import { afterUpdate, onMount } from "svelte";
  import { categoryColor, categoryVisual } from "$lib/colors";
  import { renderFlow } from "$lib/cash_flow";
  import DashboardSectionHeader from "$lib/components/dashboard/DashboardSectionHeader.svelte";
  import {
    formatMonthLabel,
    hasRecordedActivity,
    isCurrentMonthlyReview,
    monthlyReviewEndpoint,
    performanceRows,
    positionLabel,
    shouldApplyMonthlyReview,
    surplusState,
    trendSeries,
    type MonthlyReviewResponse
  } from "$lib/monthly_review";
  import { createFlowRenderController, flowSignature, flowState } from "$lib/preview_flow";
  import {
    enrichTrantionSequence,
    nextUnpaidSchedule,
    scheduleIcon,
    sortTrantionSequence
  } from "$lib/transaction_sequence";
  import {
    ajax,
    formatCurrencyCrudeWithSymbol,
    formatCurrencyWithSymbol,
    formatFloat,
    formatPercentage,
    type AssetBreakdown,
    type Budget,
    type CashFlow,
    type GoalSummary,
    type LiabilityBreakdown,
    type Networth,
    type TransactionSequence,
    now
  } from "$lib/utils";
  import { dateMax, dateMin, month, refresh } from "../../store";

  type CheckingBalancePoint = {
    month: string;
    account: string;
    balance: number;
  };

  type InsightTone = "positive" | "warning" | "negative" | "neutral";

  type InsightItem = {
    title: string;
    description: string;
    tone: InsightTone;
    deltaLabel: string;
    state: "ready" | "insufficient-data";
  };

  type WidgetState = "ready" | "insufficient-data" | "not-configured";

  const heroChartHeight = 112;

  let cashFlows: CashFlow[] = [];
  let checkingBalances: Record<string, AssetBreakdown> = {};
  let checkingBalanceTimeline: CheckingBalancePoint[] = [];
  let availableMonthRange: { from: string; to: string } | null = null;
  let goalSummaries: GoalSummary[] = [];
  let transactionSequences: TransactionSequence[] = [];
  let budgetsByMonth: Record<string, Budget> = {};
  let liabilityBreakdowns: LiabilityBreakdown[] = [];
  let networthOverview: Networth;
  let networthTimeline: Networth[] = [];
  let monthlyReview: MonthlyReviewResponse | null = null;
  let monthlyReviewLoading = false;
  let monthlyReviewError = false;
  let monthlyReviewRequest = 0;
  let sankeyEmpty = true;
  let pendingFlowReview: MonthlyReviewResponse | null = null;
  let loadError = false;

  const flowRenderer = createFlowRenderController({
    render: (graph) => {
      if (!document.getElementById("d3-expense-flow")) {
        throw new Error("flow canvas missing");
      }
      renderFlow(graph, { withCurrency: true });
    },
    clear: () => d3.select("#d3-expense-flow").selectAll("*").remove()
  });
  let checkingState: WidgetState = "insufficient-data";
  let networthTrendState: WidgetState = "insufficient-data";
  let reserveState: WidgetState = "insufficient-data";
  let budgetState: WidgetState = "insufficient-data";
  let recurringState: WidgetState = "insufficient-data";
  let goalState: WidgetState = "insufficient-data";

  async function initDemo() {
    await ajax("/api/init", {
      method: "POST",
      body: JSON.stringify({
        regional_profile: USER_CONFIG?.regional_profile || "india"
      })
    });
    refresh();
  }

  async function loadMonthlyReview(selectedMonth: string) {
    const request = ++monthlyReviewRequest;
    monthlyReviewLoading = true;

    try {
      const response = (await ajax(monthlyReviewEndpoint(selectedMonth))) as MonthlyReviewResponse;
      if (!shouldApplyMonthlyReview(response, selectedMonth, request, monthlyReviewRequest)) {
        return;
      }
      monthlyReview = response;
      monthlyReviewError = false;
    } catch (_error) {
      if (request === monthlyReviewRequest) {
        monthlyReview = null;
        monthlyReviewError = true;
      }
    } finally {
      if (request === monthlyReviewRequest) {
        monthlyReviewLoading = false;
      }
    }
  }

  function applyAvailableMonthRange(range: { from: string; to: string } | null) {
    if (!range?.from || !range?.to) {
      return;
    }

    const from = dayjs(range.from);
    const to = dayjs(range.to);
    if (
      from.isValid() &&
      to.isValid() &&
      from.year() >= 1900 &&
      to.year() >= 1900 &&
      !to.isBefore(from)
    ) {
      dateMin.set(from);
      dateMax.set(to);
    }
  }

  function renderMonthlyFlow(review: MonthlyReviewResponse) {
    const signature = flowSignature(review);
    const state = flowState(review.flow_graph);
    try {
      flowRenderer.update(review.flow_graph, signature, state);
    } catch (error) {
      console.error("Failed to render monthly flow", error);
    }
  }

  afterUpdate(() => {
    const review = pendingFlowReview;
    if (!review) {
      return;
    }
    pendingFlowReview = null;
    renderMonthlyFlow(review);
  });

  onMount(async () => {
    try {
      const [dashboard, networthResponse, liabilitiesResponse] = await Promise.all([
        ajax("/api/dashboard"),
        ajax("/api/networth"),
        ajax("/api/liabilities/balance")
      ]);

      cashFlows = dashboard.cashFlows;
      goalSummaries = _.sortBy(dashboard.goalSummaries, (goal) => -goal.priority);
      budgetsByMonth = dashboard.budget.budgetsByMonth;
      transactionSequences = _.take(
        sortTrantionSequence(enrichTrantionSequence(dashboard.transactionSequences)),
        12
      );
      checkingBalances = dashboard.checkingBalances.asset_breakdowns;
      checkingBalanceTimeline = dashboard.checkingBalanceTimeline || [];
      availableMonthRange = dashboard.availableMonthRange || null;
      applyAvailableMonthRange(availableMonthRange);
      networthOverview = dashboard.networth.networth;
      networthTimeline = networthResponse.networthTimeline;
      liabilityBreakdowns = liabilitiesResponse.liability_breakdowns;
      loadError = false;
    } catch (_error) {
      loadError = true;
    }
  });

  function networthValue(point: Networth) {
    return point?.investmentAmount + point?.gainAmount - point?.withdrawalAmount;
  }

  function sampledIndices(length: number, count = 5) {
    if (length <= count) {
      return _.range(length);
    }

    return _.uniq(_.range(count).map((index) => Math.round((index * (length - 1)) / (count - 1))));
  }

  function monthLabels(months: string[], count = 5) {
    if (!months.length) {
      return [];
    }

    return sampledIndices(months.length, count).map((index) => formatMonthLabel(months[index]));
  }

  function valueTicks(min: number, max: number, count = 3, height = heroChartHeight) {
    const spread = max - min || Math.max(Math.abs(max), 1);

    return _.range(count).map((index) => {
      const ratio = count === 1 ? 0 : index / (count - 1);
      const value = max - spread * ratio;

      return {
        label: formatCurrencyCrudeWithSymbol(value, 0),
        top: ratio * 100,
        y: ratio * height
      };
    });
  }

  function pathForValues(
    values: Array<number | null>,
    min: number,
    max: number,
    width: number,
    height: number
  ) {
    const points = values.map((value, index) => ({ value, index }));
    if (points.filter((point) => Number.isFinite(point.value)).length <= 1) {
      return "";
    }

    const x = d3
      .scaleLinear()
      .domain([0, values.length - 1])
      .range([0, width]);
    const y = d3
      .scaleLinear()
      .domain([min, max === min ? max + 1 : max])
      .range([height, 0]);

    return (
      d3
        .line<{ value: number | null; index: number }>()
        .defined((point) => Number.isFinite(point.value))
        .x((point) => x(point.index))
        .y((point) => y(point.value as number))
        .curve(d3.curveMonotoneX)(points) || ""
    );
  }

  function topExpenseGroup(review: MonthlyReviewResponse | null) {
    if (!review?.flow_graph?.nodes?.length) {
      return null;
    }

    const expenses = review.flow_graph.nodes.find((node) => node.name === "Expenses");
    if (!expenses) {
      return null;
    }

    const groups = review.flow_graph.links
      .filter((link) => link.source === expenses.id)
      .map((link) => {
        const target = review.flow_graph.nodes.find((node) => node.id === link.target);
        if (!target?.name.startsWith("Expenses:") || target.name === "Expenses:Tax") {
          return null;
        }
        return {
          label: target.name.split(":").slice(1).join(" · "),
          value: link.value
        };
      })
      .filter((group): group is { label: string; value: number } => group != null);

    return _.maxBy(groups, (group) => group.value) ?? null;
  }

  $: monthKey = $month || now().format("YYYY-MM");
  $: selectedMonthLabel = dayjs(monthKey + "-01").format("MMM YYYY");
  $: if (monthKey) {
    void loadMonthlyReview(monthKey);
  }
  $: activeMonthlyReview = isCurrentMonthlyReview(monthlyReview, monthKey) ? monthlyReview : null;
  $: if (!activeMonthlyReview) {
    pendingFlowReview = null;
    flowRenderer.reset();
  }
  $: reviewTrend = trendSeries(activeMonthlyReview);
  $: reviewSurplusState = surplusState(activeMonthlyReview);
  $: reviewPerformance = performanceRows(activeMonthlyReview);
  $: reviewHasActivity = hasRecordedActivity(activeMonthlyReview);
  $: reviewLoading = monthlyReviewLoading && !activeMonthlyReview;
  $: topExpense = topExpenseGroup(activeMonthlyReview);
  $: currentBudget = budgetsByMonth[monthKey];
  $: flowTerminalCount = (activeMonthlyReview?.flow_graph?.nodes || []).filter(
    (node) =>
      !(activeMonthlyReview?.flow_graph?.links || []).some((link) => link.source === node.id)
  ).length;
  $: flowChartHeight = Math.max(440, Math.min(1100, flowTerminalCount * 30 + 120));

  $: if (
    activeMonthlyReview &&
    flowSignature(activeMonthlyReview) !== flowRenderer.renderedSignature()
  ) {
    sankeyEmpty = flowState(activeMonthlyReview.flow_graph) === "empty";
    pendingFlowReview = activeMonthlyReview;
  }

  let surplusInsightItem: InsightItem;
  let expenseInsightItem: InsightItem;
  let investmentInsightItem: InsightItem;

  $: surplusInsightItem =
    !reviewHasActivity || !activeMonthlyReview
      ? {
          title: "No recorded activity",
          description: `There are no postings for ${selectedMonthLabel}. This does not prove the ledger history is complete.`,
          tone: "neutral",
          deltaLabel: "No activity recorded",
          state: "insufficient-data"
        }
      : reviewSurplusState === "ready"
        ? {
            title: activeMonthlyReview.surplus >= 0 ? "Surplus recorded" : "Deficit recorded",
            description: "Net income after tax minus non-tax expenses.",
            tone: activeMonthlyReview.surplus >= 0 ? "positive" : "negative",
            deltaLabel: formatCurrencyWithSymbol(activeMonthlyReview.surplus),
            state: "ready"
          }
        : {
            title:
              reviewSurplusState === "income-missing"
                ? "Income not recorded"
                : reviewSurplusState === "expense-missing"
                  ? "Expenses not recorded"
                  : "No income or expenses recorded",
            description: "Surplus is not calculated from incomplete postings for this month.",
            tone: "neutral",
            deltaLabel: "Surplus not calculated",
            state: "insufficient-data"
          };

  $: expenseInsightItem =
    reviewHasActivity && topExpense
      ? {
          title: `${topExpense.label} was the largest expense group`,
          description: `Based on actual postings in ${selectedMonthLabel}.`,
          tone: "neutral",
          deltaLabel: formatCurrencyWithSymbol(topExpense.value),
          state: "ready"
        }
      : {
          title: "No expense groups recorded",
          description: reviewHasActivity
            ? `No non-tax expenses were posted in ${selectedMonthLabel}.`
            : `There are no postings for ${selectedMonthLabel}.`,
          tone: "neutral",
          deltaLabel: "No expenses posted",
          state: "insufficient-data"
        };

  $: investmentInsightItem =
    activeMonthlyReview?.performance_state === "ready" && reviewPerformance.best.length
      ? {
          title: `${positionLabel(reviewPerformance.best[0].account)} led the month`,
          description: reviewPerformance.weakest.length
            ? `Strongest ${positionLabel(reviewPerformance.best[0].account)} · weakest ${positionLabel(
                reviewPerformance.weakest[0].account
              )}. Ranked on flow-adjusted total return.`
            : "Ranked on flow-adjusted total return.",
          tone:
            reviewPerformance.best[0].total_return_percent >= 0 &&
            (reviewPerformance.weakest[0]?.total_return_percent ?? 0) >= 0
              ? "positive"
              : reviewPerformance.best[0].total_return_percent < 0
                ? "negative"
                : "warning",
          deltaLabel: `Market movement ${formatCurrencyWithSymbol(
            activeMonthlyReview.investment_market_movement
          )}`,
          state: "ready"
        }
      : {
          title: "Investment performance unavailable",
          description:
            "Historical prices or flow coverage are insufficient for a comparable monthly ranking.",
          tone: "neutral",
          deltaLabel: "Not available",
          state: "insufficient-data"
        };

  $: insights = [surplusInsightItem, expenseInsightItem, investmentInsightItem];

  $: checkingHistory = _.sortBy(checkingBalanceTimeline, ["month", "account"]);
  $: checkingMonths = _.uniq(checkingHistory.map((point) => point.month));
  $: checkingAccounts = _.uniq(checkingHistory.map((point) => point.account));
  $: checkingAccountSeries = checkingAccounts.map((account, index) => ({
    account,
    label: account.split(":").at(-1) || account,
    color: categoryColor(account, index),
    values: checkingMonths.map(
      (historyMonth) =>
        checkingHistory.find((point) => point.month === historyMonth && point.account === account)
          ?.balance ?? null
    )
  }));
  $: checkingAllValues = _.flatMap(checkingAccountSeries, (series) => series.values).filter(
    (value): value is number => value != null
  );
  $: checkingMin = _.min(checkingAllValues) ?? 0;
  $: checkingMax = _.max(checkingAllValues) ?? 0;
  $: checkingHasHistory = checkingMonths.length >= 2 && checkingAllValues.length >= 2;
  $: checkingState = checkingHasHistory ? "ready" : "insufficient-data";
  $: checkingValueTicks = checkingHasHistory ? valueTicks(checkingMin, checkingMax) : [];
  $: checkingDateTicks = checkingHasHistory ? monthLabels(checkingMonths, 5) : [];
  $: checkingTotal = _.sumBy(_.values(checkingBalances), (breakdown) => breakdown.marketAmount);

  $: monthlyNetworth = _.chain(networthTimeline)
    .groupBy((point) => dayjs(point.date).format("YYYY-MM"))
    .values()
    .map((points) => _.last(points))
    .sortBy((point) => dayjs(point.date).valueOf())
    .takeRight(12)
    .value();
  $: networthSeries = monthlyNetworth.map((point) => networthValue(point));
  $: latestNetworthPoint = _.last(networthTimeline) || networthOverview;
  $: currentNetworthValue = latestNetworthPoint ? networthValue(latestNetworthPoint) : 0;
  $: networthHistoryMonths = monthlyNetworth.length;
  $: networthHasHistory = networthSeries.length >= 2;
  $: networthTrendState = networthHasHistory ? "ready" : "insufficient-data";
  $: networthMin = _.min(networthSeries) ?? 0;
  $: networthMax = _.max(networthSeries) ?? 0;
  $: networthValueTicks = networthHasHistory ? valueTicks(networthMin, networthMax) : [];
  $: networthDateTicks = networthHasHistory
    ? monthLabels(
        monthlyNetworth.map((point) => dayjs(point.date).format("YYYY-MM")),
        5
      )
    : [];

  $: recentExpenseTotals = _.chain(cashFlows)
    .takeRight(3)
    .map((cashFlow) => cashFlow.expenses)
    .filter((value) => value > 0)
    .value();
  $: averageMonthlyExpense = recentExpenseTotals.length ? _.mean(recentExpenseTotals) : 0;
  $: reserveState = averageMonthlyExpense > 0 ? "ready" : "insufficient-data";
  $: reserveMonths = reserveState === "ready" ? checkingTotal / averageMonthlyExpense : 0;

  $: budgetPressure = _.chain(currentBudget?.accounts || [])
    .filter((account) => account.budgeted > 0)
    .map((account, index) => {
      const label = account.account.split(":").at(-1);
      const percent = Math.max(0, Math.round((account.actual / account.budgeted) * 100));

      return {
        label,
        percent,
        warning: percent >= 70,
        ...categoryVisual(label, index),
        tone:
          account.actual / account.budgeted > 0.9
            ? "negative"
            : account.actual / account.budgeted > 0.7
              ? "warning"
              : "positive"
      };
    })
    .orderBy(["percent"], ["desc"])
    .take(3)
    .value();
  $: budgetState = !currentBudget
    ? "insufficient-data"
    : !currentBudget.accounts?.some((account) => account.budgeted > 0)
      ? "not-configured"
      : budgetPressure.length
        ? "ready"
        : "insufficient-data";

  $: recurringOutlook = transactionSequences
    .map((sequence) => {
      const schedule = nextUnpaidSchedule(sequence);
      if (!schedule) {
        return null;
      }

      return {
        label: sequence.key,
        amount: schedule.amount,
        date: schedule.scheduled,
        icon: scheduleIcon(schedule)
      };
    })
    .filter((item) => item != null)
    .slice(0, 3);
  $: recurringState = recurringOutlook.length ? "ready" : "insufficient-data";

  $: goalProgress = goalSummaries.slice(0, 3).map((goal) => ({
    name: goal.name,
    percent: goal.target === 0 ? 0 : Math.round((goal.current / goal.target) * 100)
  }));
  $: goalState = goalSummaries.length
    ? goalProgress.length
      ? "ready"
      : "insufficient-data"
    : "not-configured";

  $: hasDashboardData =
    cashFlows.length > 0 ||
    !_.isEmpty(checkingBalances) ||
    checkingBalanceTimeline.length > 0 ||
    goalSummaries.length > 0 ||
    transactionSequences.length > 0 ||
    !_.isEmpty(budgetsByMonth) ||
    liabilityBreakdowns.length > 0 ||
    networthTimeline.length > 0 ||
    currentNetworthValue !== 0;
  $: isEmpty = loadError || !hasDashboardData;
</script>

{#if isEmpty}
  <section class="section">
    <div class="paisa-empty-state">
      <div class="paisa-empty-state__copy">
        <p class="paisa-kicker">Overview</p>
        <h2>
          {loadError
            ? "Start the local Paisa server to preview the dashboard."
            : "Start with your first journal or load a guided demo."}
        </h2>
        <p>
          {#if loadError}
            The Vite frontend is running, but the local API server is not reachable on
            <code>http://localhost:7500</code>.
          {:else}
            Paisa will turn your ledger into a financial cockpit once transactions, prices, and
            budgets are available.
          {/if}
        </p>
        <ol>
          {#if loadError}
            <li>Start the backend with <code>.\paisa.exe serve</code>.</li>
            <li>Keep <code>npm run dev</code> running in a second terminal.</li>
            <li>Reload this page once the API responds.</li>
          {:else}
            <li>Set your regional profile, locale, and default currency in Configuration.</li>
            <li>Add your first journal entries in the Ledger editor.</li>
            <li>Or load the demo dataset to preview the redesigned dashboard immediately.</li>
          {/if}
        </ol>
        <div class="buttons">
          <a href="/more/config" class="button is-light">Open Configuration</a>
          <a href="/ledger/editor" class="button is-light">Open Ledger Editor</a>
          {#if !loadError}
            <button on:click={(_event) => initDemo()} class="button is-link" type="button">
              Load Demo
            </button>
          {/if}
        </div>
      </div>
    </div>
  </section>
{:else}
  <section class="section pt-0 px-0">
    <div class="paisa-dashboard">
      <div class="paisa-dashboard__section paisa-dashboard__hero">
        <div class="paisa-dashboard__hero-top">
          <div class="paisa-dashboard__hero-summary">
            <div>
              <div class="paisa-dashboard__label">Total Net Worth</div>
              <div class="paisa-dashboard__amount">
                {formatCurrencyWithSymbol(currentNetworthValue)}
              </div>
            </div>
          </div>

          <div class="paisa-dashboard__trend-group">
            <div class="paisa-spark-card paisa-spark-card--hero">
              <div class="paisa-spark-card__meta">
                <div>
                  <span>Net Worth</span>
                  <strong>{formatCurrencyWithSymbol(currentNetworthValue)}</strong>
                </div>
                <div class="paisa-spark-card__change">
                  {#if networthHasHistory}
                    {networthHistoryMonths} months
                  {:else}
                    Need more history
                  {/if}
                </div>
              </div>
              {#if networthTrendState === "ready"}
                <div class="paisa-spark-card__chart">
                  <div class="paisa-spark-card__y-axis">
                    {#each networthValueTicks as tick}
                      <span style={`top: calc(${tick.top}% - 0.55rem);`}>{tick.label}</span>
                    {/each}
                  </div>
                  <div class="paisa-spark-card__plot">
                    <svg
                      viewBox={`0 0 420 ${heroChartHeight}`}
                      class="paisa-sparkline paisa-sparkline--hero"
                    >
                      {#each networthValueTicks as tick}
                        <line
                          x1="0"
                          y1={tick.y}
                          x2="420"
                          y2={tick.y}
                          class="paisa-sparkline__grid"
                        />
                      {/each}
                      <path
                        d={pathForValues(
                          networthSeries,
                          networthMin,
                          networthMax,
                          420,
                          heroChartHeight
                        )}
                        class="is-equity"
                        vector-effect="non-scaling-stroke"
                      />
                    </svg>
                    <div class="paisa-spark-card__x-axis">
                      {#each networthDateTicks as label}
                        <span>{label}</span>
                      {/each}
                    </div>
                  </div>
                </div>
              {:else}
                <div class="paisa-spark-card__empty">
                  Need at least two monthly net worth snapshots to plot this trend.
                </div>
              {/if}
            </div>

            <div class="paisa-spark-card paisa-spark-card--hero">
              <div class="paisa-spark-card__meta">
                <div>
                  <span>Checking</span>
                  <strong>{formatCurrencyWithSymbol(checkingTotal)}</strong>
                </div>
                <div class="paisa-spark-card__change">
                  {#if checkingHasHistory}
                    {checkingAccounts.length} account{checkingAccounts.length === 1 ? "" : "s"}
                  {:else}
                    Need more history
                  {/if}
                </div>
              </div>
              {#if checkingState === "ready"}
                <div class="paisa-spark-card__chart">
                  <div class="paisa-spark-card__y-axis">
                    {#each checkingValueTicks as tick}
                      <span style={`top: calc(${tick.top}% - 0.55rem);`}>{tick.label}</span>
                    {/each}
                  </div>
                  <div class="paisa-spark-card__plot">
                    <svg
                      viewBox={`0 0 420 ${heroChartHeight}`}
                      class="paisa-sparkline paisa-sparkline--hero"
                    >
                      {#each checkingValueTicks as tick}
                        <line
                          x1="0"
                          y1={tick.y}
                          x2="420"
                          y2={tick.y}
                          class="paisa-sparkline__grid"
                        />
                      {/each}
                      {#each checkingAccountSeries as series}
                        <path
                          d={pathForValues(
                            series.values,
                            checkingMin,
                            checkingMax,
                            420,
                            heroChartHeight
                          )}
                          style={`stroke: ${series.color}`}
                          vector-effect="non-scaling-stroke"
                        />
                      {/each}
                    </svg>
                    <div class="paisa-spark-card__x-axis">
                      {#each checkingDateTicks as label}
                        <span>{label}</span>
                      {/each}
                    </div>
                    {#if checkingAccountSeries.length > 1}
                      <div class="paisa-spark-card__legend">
                        {#each checkingAccountSeries as series}
                          <span><i style={`background: ${series.color}`}></i>{series.label}</span>
                        {/each}
                      </div>
                    {/if}
                  </div>
                </div>
              {:else}
                <div class="paisa-spark-card__empty">
                  Need at least two recorded checking months to plot this trend.
                </div>
              {/if}
            </div>
          </div>
        </div>
      </div>

      <div class="paisa-dashboard__section">
        <div class="paisa-review">
          <div class="paisa-review__header">
            <div>
              <div class="paisa-zone__title">Monthly Review</div>
              <div class="paisa-zone__subtle">
                {selectedMonthLabel}{activeMonthlyReview && !activeMonthlyReview.period_complete
                  ? " · in progress"
                  : ""}
              </div>
            </div>
            {#if activeMonthlyReview}
              <div class="paisa-review__status">
                {activeMonthlyReview.period_complete ? "Complete calendar month" : "Month to date"}
              </div>
            {/if}
          </div>

          {#if reviewLoading}
            <div class="paisa-zone__empty">
              <strong>Loading {selectedMonthLabel}…</strong>
            </div>
          {:else if monthlyReviewError}
            <div class="paisa-zone__empty">
              <strong>Monthly review could not be loaded.</strong>
              <p>Select another month or reload the page.</p>
            </div>
          {:else if !activeMonthlyReview}
            <div class="paisa-zone__empty">
              <strong>Monthly review is not available yet.</strong>
            </div>
          {:else}
            {#if reviewHasActivity}
              <div class="paisa-review__metrics">
                <div class="paisa-review__metric">
                  <span>Net income after tax</span>
                  <strong
                    >{formatCurrencyWithSymbol(activeMonthlyReview.net_income_after_tax)}</strong
                  >
                  <small>
                    {activeMonthlyReview.has_income_postings
                      ? `Tax ${formatCurrencyWithSymbol(activeMonthlyReview.net_tax)}`
                      : "No recorded income"}
                  </small>
                </div>
                <div class="paisa-review__metric">
                  <span>Surplus / Deficit</span>
                  {#if reviewSurplusState === "ready"}
                    <strong class:is-negative={activeMonthlyReview.surplus < 0}
                      >{formatCurrencyWithSymbol(activeMonthlyReview.surplus)}</strong
                    >
                    <small>net income − expenses</small>
                  {:else}
                    <strong class="is-muted">Not available</strong>
                    <small>
                      {reviewSurplusState === "income-missing"
                        ? "No recorded income this month."
                        : reviewSurplusState === "expense-missing"
                          ? "No recorded expenses this month."
                          : "No recorded income or expenses this month."}
                    </small>
                  {/if}
                </div>
                <div class="paisa-review__metric">
                  <span>Net expense</span>
                  <strong
                    >{activeMonthlyReview.has_expense_postings
                      ? formatCurrencyWithSymbol(activeMonthlyReview.net_expense)
                      : "Not available"}</strong
                  >
                  <small
                    >{activeMonthlyReview.has_expense_postings
                      ? "Excludes tax"
                      : "No recorded expenses"}</small
                  >
                </div>
              </div>

              <div class="paisa-review__trend">
                <div class="paisa-zone__subtle">Net income · last three months</div>
                <div class="paisa-review__trend-values">
                  {#each reviewTrend as point, index}
                    {@const previous = index > 0 ? reviewTrend[index - 1] : null}
                    <div class="paisa-review__trend-point" class:is-mtd={point.isMonthToDate}>
                      <span>{dayjs(point.month + "-01").format("MMM YYYY")}</span>
                      <strong
                        >{point.value != null
                          ? formatCurrencyWithSymbol(point.value)
                          : "Not available"}</strong
                      >
                      {#if point.isMonthToDate}
                        <small>Month to date</small>
                      {:else if point.value != null && previous?.value != null}
                        <small
                          class:is-positive={point.value >= previous.value}
                          class:is-negative={point.value < previous.value}
                        >
                          {point.value >= previous.value ? "↑" : "↓"}
                          {formatCurrencyWithSymbol(Math.abs(point.value - previous.value))}
                        </small>
                      {:else}
                        <small>No comparable month</small>
                      {/if}
                    </div>
                  {/each}
                </div>
              </div>

              <div class="paisa-review__flow">
                <div class="paisa-review__performance-header">
                  <div class="paisa-zone__subtle">Posted money flow</div>
                  <a href="/cash_flow/yearly" class="secondary-link paisa-link-with-arrow"
                    >Open yearly flow <i class="fas fa-arrow-right"></i></a
                  >
                </div>
                {#if sankeyEmpty}
                  <div class="paisa-zone__empty">
                    <strong>No posted flows to chart.</strong>
                    <p>The selected month contains no postings to connect.</p>
                  </div>
                {:else}
                  <div class="paisa-review__flow-canvas">
                    <svg id="d3-expense-flow" height={flowChartHeight}></svg>
                  </div>
                {/if}
              </div>
            {:else}
              <div class="paisa-review__no-activity">
                <strong>No recorded activity</strong>
                <p>
                  There are no postings for {selectedMonthLabel}. This does not prove the ledger
                  history is complete.
                </p>
              </div>
            {/if}

            <div class="paisa-review__performance">
              <div class="paisa-review__performance-header">
                <div class="paisa-zone__subtle">Investments</div>
                {#if reviewPerformance.asOf}
                  <div class="paisa-zone__subtle">
                    Prices as of {dayjs(reviewPerformance.asOf).format("MMM D, YYYY")}
                  </div>
                {/if}
              </div>

              <div class="paisa-review__investment-drivers">
                <div>
                  <span>Net contributions</span><strong
                    >{formatCurrencyWithSymbol(
                      activeMonthlyReview.investment_contributions
                    )}</strong
                  ><small>Non-checking assets</small>
                </div>
                <div>
                  <span>Market movement</span><strong
                    >{activeMonthlyReview.performance_state === "ready"
                      ? formatCurrencyWithSymbol(activeMonthlyReview.investment_market_movement)
                      : "Not available"}</strong
                  ><small>Ranked positions</small>
                </div>
                <div>
                  <span>Dividends</span><strong
                    >{formatCurrencyWithSymbol(activeMonthlyReview.dividend_income)}</strong
                  ><small>Total return component</small>
                </div>
              </div>

              {#if activeMonthlyReview.performance_state === "ready" && reviewPerformance.best.length}
                <div class="paisa-review__performers">
                  <div class="paisa-review__performer-group">
                    <div class="paisa-zone__subtle">Best</div>
                    {#each reviewPerformance.best as performer}
                      <div class="paisa-review__performer">
                        <span>{positionLabel(performer.account)}</span>
                        <strong
                          class:is-positive={performer.total_return_percent >= 0}
                          class:is-negative={performer.total_return_percent < 0}
                        >
                          {formatPercentage(performer.total_return_percent / 100, 2)}
                        </strong>
                        {#if performer.dividend_income}
                          <small
                            >Dividends {formatCurrencyWithSymbol(performer.dividend_income)}</small
                          >
                        {/if}
                      </div>
                    {/each}
                  </div>
                  <div class="paisa-review__performer-group">
                    <div class="paisa-zone__subtle">Weakest</div>
                    {#each reviewPerformance.weakest as performer}
                      <div class="paisa-review__performer">
                        <span>{positionLabel(performer.account)}</span>
                        <strong
                          class:is-positive={performer.total_return_percent >= 0}
                          class:is-negative={performer.total_return_percent < 0}
                        >
                          {formatPercentage(performer.total_return_percent / 100, 2)}
                        </strong>
                        {#if performer.dividend_income}
                          <small
                            >Dividends {formatCurrencyWithSymbol(performer.dividend_income)}</small
                          >
                        {/if}
                      </div>
                    {/each}
                  </div>
                </div>
              {:else}
                <div class="paisa-zone__empty">
                  <strong>Investment performance is not available for {selectedMonthLabel}.</strong>
                  <p>
                    Historical prices or flow coverage are insufficient for a comparable ranking.
                  </p>
                </div>
              {/if}
            </div>
          {/if}
        </div>
      </div>

      <div class="paisa-dashboard__section">
        <div class="paisa-dashboard__band paisa-dashboard__band--review">
          <div class="paisa-zone paisa-zone--insights">
            <DashboardSectionHeader title="Insights" subtle={selectedMonthLabel}>
              <svelte:fragment slot="right">
                <a href="/expense/monthly" class="secondary-link paisa-link-with-arrow"
                  >View all insights <i class="fas fa-arrow-right"></i></a
                >
              </svelte:fragment>
            </DashboardSectionHeader>

            {#if reviewLoading}
              <div class="paisa-zone__empty">
                <strong>Loading {selectedMonthLabel}…</strong>
              </div>
            {:else}
              <div class="paisa-insights">
                {#each insights as insight}
                  <article class={"paisa-insight paisa-insight--" + insight.tone}>
                    <div class="paisa-insight__icon">
                      <span class="icon">
                        <i
                          class={"fas " +
                            (insight.tone === "positive"
                              ? "fa-arrow-trend-up"
                              : insight.tone === "warning"
                                ? "fa-triangle-exclamation"
                                : insight.tone === "negative"
                                  ? "fa-arrow-trend-down"
                                  : "fa-circle-info")}
                        ></i>
                      </span>
                    </div>
                    <div class="paisa-insight__copy">
                      <div class="paisa-insight__title">{insight.title}</div>
                      <div class="paisa-insight__text">{insight.description}</div>
                      <div class="paisa-insight__delta">{insight.deltaLabel}</div>
                    </div>
                  </article>
                {/each}
              </div>
            {/if}
          </div>

          <div class="paisa-zone paisa-zone--goals">
            <DashboardSectionHeader title="Goal Progress" subtle="Current status">
              <svelte:fragment slot="right">
                <a href="/more/goals" class="secondary-link paisa-link-with-arrow"
                  >View all goals <i class="fas fa-arrow-right"></i></a
                >
              </svelte:fragment>
            </DashboardSectionHeader>
            {#if goalState === "ready"}
              <div class="paisa-mini-list">
                {#each goalProgress as goal, index}
                  <div class="paisa-mini-list__goal">
                    <div class="is-flex is-justify-content-space-between">
                      <span>{goal.name}</span>
                      <strong>{goal.percent}%</strong>
                    </div>
                    <div class="paisa-mini-list__track">
                      <div
                        class="paisa-mini-list__fill positive"
                        style={`width: ${Math.min(goal.percent, 100)}%; background: ${categoryColor(
                          goal.name,
                          index
                        )};`}
                      ></div>
                    </div>
                  </div>
                {/each}
              </div>
            {:else if goalState === "not-configured"}
              <div class="paisa-support__empty">
                <strong>No goals configured.</strong>
                <p>Create savings or retirement goals to track progress here.</p>
              </div>
            {:else}
              <div class="paisa-support__empty">
                <strong>Goal progress is not ready yet.</strong>
                <p>Goal targets exist, but Paisa needs more qualifying data to chart progress.</p>
              </div>
            {/if}
          </div>
        </div>
      </div>

      <div class="paisa-dashboard__section">
        <div class="paisa-dashboard__band paisa-dashboard__band--support paisa-surface-split">
          <div class="paisa-support">
            <div class="paisa-support__header">
              <div class="paisa-zone__title">Cash Reserve</div>
              <div class="paisa-zone__subtle">Current</div>
            </div>
            <div class="paisa-support__value">{formatCurrencyWithSymbol(checkingTotal)}</div>
            {#if reserveState === "ready"}
              <div class="paisa-support__text">{formatFloat(reserveMonths)} months of expenses</div>
              <div class="paisa-progress">
                <div
                  class="paisa-progress__fill"
                  style={`width: ${Math.min((reserveMonths / 9) * 100, 100)}%`}
                ></div>
              </div>
              <div class="paisa-progress__ticks">
                <span>0</span>
                <span>3 mo</span>
                <span>6 mo</span>
                <span>9+ mo</span>
              </div>
            {:else}
              <div class="paisa-support__empty">
                <strong>Need expense history.</strong>
                <p>Post expenses in at least one recent month to estimate your reserve runway.</p>
              </div>
            {/if}
          </div>

          <div class="paisa-support">
            <div class="paisa-support__header">
              <div class="paisa-zone__title">Budget Pressure</div>
              <div class="paisa-zone__subtle">{selectedMonthLabel}</div>
            </div>
            {#if budgetState === "ready"}
              <div class="paisa-mini-list">
                {#each budgetPressure as budget}
                  <div class="paisa-mini-list__row">
                    <span class="paisa-mini-list__label">
                      <span class="icon" style={`color: ${budget.color};`}>
                        <i class={"fas " + budget.icon}></i>
                      </span>
                      <span>{budget.label}</span>
                    </span>
                    <div class="paisa-mini-list__meter">
                      <div class="paisa-mini-list__track">
                        <div
                          class={"paisa-mini-list__fill " + budget.tone}
                          style={`width: ${Math.min(budget.percent, 100)}%`}
                        ></div>
                      </div>
                      <strong>{budget.percent}%</strong>
                      {#if budget.warning}
                        <span class="paisa-mini-list__alert">!</span>
                      {/if}
                    </div>
                  </div>
                {/each}
              </div>
              <a href="/expense/budget" class="secondary-link paisa-mini-list__link"
                >View all budgets</a
              >
            {:else if budgetState === "not-configured"}
              <div class="paisa-support__empty">
                <strong>No budget configured.</strong>
                <p>Create budget lines to monitor category pressure here.</p>
                <a
                  href="/expense/budget"
                  class="secondary-link paisa-mini-list__link paisa-zone__empty-link"
                  >Set up budget</a
                >
              </div>
            {:else}
              <div class="paisa-support__empty">
                <strong>No budget data for {selectedMonthLabel}.</strong>
                <p>Budget pressure appears for months with configured budget lines.</p>
              </div>
            {/if}
          </div>

          <div class="paisa-support">
            <div class="paisa-support__header">
              <div class="paisa-zone__title">Recurring Outlook</div>
              <div class="paisa-zone__subtle">Next 7 days</div>
            </div>
            {#if recurringState === "ready"}
              <div class="paisa-mini-list">
                {#each recurringOutlook as recurring}
                  <div class="paisa-mini-list__row paisa-mini-list__row--recurring">
                    <span class="is-flex is-align-items-center" style="gap: 0.55rem;">
                      <span class={"icon paisa-recurring-icon " + recurring.icon.color}>
                        <i class={"fas " + recurring.icon.icon}></i>
                      </span>
                      <span>{recurring.label}</span>
                    </span>
                    <div class="is-flex is-align-items-center" style="gap: 0.85rem;">
                      <strong>{formatCurrencyWithSymbol(recurring.amount)}</strong>
                      <span class="paisa-zone__subtle">{recurring.date.format("MMM D")}</span>
                    </div>
                  </div>
                {/each}
              </div>
              <a href="/cash_flow/recurring" class="secondary-link paisa-mini-list__link"
                >View all recurring</a
              >
            {:else}
              <div class="paisa-support__empty">
                <strong>No recurring outlook yet.</strong>
                <p>Recurring items appear after Paisa can detect repeating transactions.</p>
              </div>
            {/if}
          </div>
        </div>
      </div>

      <div class="paisa-dashboard__section paisa-dashboard__footer">
        <span>Live snapshot stays independent of the selected review month.</span>
        <span>Last updated: Today, {now().format("HH:mm")}</span>
      </div>
    </div>
  </section>
{/if}
