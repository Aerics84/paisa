<script lang="ts">
  import * as d3 from "d3";
  import dayjs from "dayjs";
  import _ from "lodash";
  import { onMount, untrack } from "svelte";
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
  import { getDashboard } from "$lib/api/dashboard";
  import { getLiabilityBalance } from "$lib/api/liabilities";
  import { getNetworth } from "$lib/api/assets";
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
  import { t } from "$lib/i18n";
  import PreviewDashboardSettings from "$lib/components/PreviewDashboardSettings.svelte";
  import PreviewRecentTransactions from "$lib/components/PreviewRecentTransactions.svelte";
  import {
    latestTransactions,
    loadLayout,
    saveLayout,
    type PreviewDashboardLayout
  } from "$lib/preview_dashboard";
  import type { Transaction } from "$lib/api/ledger";

  // The app is client-only (ssr = false), so a synchronous read avoids a layout flash.
  let dashboardLayout = $state<PreviewDashboardLayout>(loadLayout());
  let showDashboardSettings = $state(false);
  let customizeButtonEl = $state<HTMLButtonElement | null>(null);

  function updateDashboardLayout(next: PreviewDashboardLayout) {
    dashboardLayout = next;
    saveLayout(next);
  }

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

  let cashFlows = $state<CashFlow[]>([]);
  let checkingBalances = $state<Record<string, AssetBreakdown>>({});
  let checkingBalanceTimeline = $state<CheckingBalancePoint[]>([]);
  let availableMonthRange = $state<{ from: dayjs.Dayjs; to: dayjs.Dayjs } | null>(null);
  let goalSummaries = $state<GoalSummary[]>([]);
  let transactionSequences = $state<TransactionSequence[]>([]);
  let transactions = $state<Transaction[]>([]);
  let budgetsByMonth = $state<Record<string, Budget>>({});
  let liabilityBreakdowns = $state<Record<string, LiabilityBreakdown>>({});
  let networthOverview = $state<Networth>();
  let networthTimeline = $state<Networth[]>([]);
  let monthlyReview = $state<MonthlyReviewResponse | null>(null);
  let monthlyReviewLoading = $state(false);
  let monthlyReviewError = $state(false);
  let monthlyReviewRequest = 0;
  let sankeyEmpty = $state(true);
  let loadError = $state(false);

  const flowRenderer = createFlowRenderController({
    render: (graph) => {
      if (!document.getElementById("d3-expense-flow")) {
        throw new Error("flow canvas missing");
      }
      renderFlow(graph, { withCurrency: true });
    },
    clear: () => d3.select("#d3-expense-flow").selectAll("*").remove()
  });

  async function initDemo() {
    await ajax("/api/init", {
      method: "POST",
      body: JSON.stringify({
        regional_profile: USER_CONFIG?.regional_profile || "india"
      })
    });
    await refresh();
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
    } catch {
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

  function applyAvailableMonthRange(range: { from: dayjs.Dayjs; to: dayjs.Dayjs } | null) {
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

  function flowCanvas(_node: SVGElement, review: MonthlyReviewResponse | null) {
    let current = review;
    let frame = 0;

    const draw = () => {
      frame = 0;
      if (!current) {
        return;
      }
      renderMonthlyFlow(current);
    };

    const schedule = () => {
      if (frame) {
        cancelAnimationFrame(frame);
      }
      frame = requestAnimationFrame(draw);
    };

    schedule();

    return {
      update(next: MonthlyReviewResponse | null) {
        current = next;
        schedule();
      },
      destroy() {
        if (frame) {
          cancelAnimationFrame(frame);
        }
      }
    };
  }

  onMount(async () => {
    try {
      const [dashboard, networthResponse, liabilitiesResponse] = await Promise.all([
        getDashboard(),
        getNetworth(),
        getLiabilityBalance()
      ]);

      cashFlows = dashboard.cashFlows;
      goalSummaries = _.sortBy(dashboard.goalSummaries, (goal) => -goal.priority);
      budgetsByMonth = dashboard.budget.budgetsByMonth;
      transactionSequences = _.take(
        sortTrantionSequence(enrichTrantionSequence(dashboard.transactionSequences)),
        12
      );
      transactions = latestTransactions(dashboard.transactions || [], 6);
      checkingBalances = dashboard.checkingBalances.asset_breakdowns;
      checkingBalanceTimeline = dashboard.checkingBalanceTimeline || [];
      availableMonthRange = dashboard.availableMonthRange || null;
      applyAvailableMonthRange(availableMonthRange);
      networthOverview = dashboard.networth.networth;
      networthTimeline = networthResponse.networthTimeline;
      liabilityBreakdowns = liabilitiesResponse.liability_breakdowns;
      loadError = false;
    } catch {
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

  let monthKey = $derived($month || now().format("YYYY-MM"));
  let selectedMonthLabel = $derived(dayjs(monthKey + "-01").format("MMM YYYY"));
  let activeMonthlyReview = $derived(
    isCurrentMonthlyReview(monthlyReview, monthKey) ? monthlyReview : null
  );
  let reviewTrend = $derived(trendSeries(activeMonthlyReview));
  let reviewSurplusState = $derived(surplusState(activeMonthlyReview));
  let reviewPerformance = $derived(performanceRows(activeMonthlyReview));
  let reviewHasActivity = $derived(hasRecordedActivity(activeMonthlyReview));
  let reviewLoading = $derived(monthlyReviewLoading && !activeMonthlyReview);
  let topExpense = $derived(topExpenseGroup(activeMonthlyReview));
  let currentBudget = $derived(budgetsByMonth[monthKey]);
  let flowTerminalCount = $derived(
    (activeMonthlyReview?.flow_graph?.nodes || []).filter(
      (node) =>
        !(activeMonthlyReview?.flow_graph?.links || []).some((link) => link.source === node.id)
    ).length
  );
  let flowChartHeight = $derived(Math.max(440, Math.min(1100, flowTerminalCount * 30 + 120)));

  $effect(() => {
    const selectedMonth = monthKey;
    untrack(() => loadMonthlyReview(selectedMonth));
  });

  $effect(() => {
    if (!activeMonthlyReview) {
      flowRenderer.reset();
    }
  });

  $effect(() => {
    if (activeMonthlyReview) {
      sankeyEmpty = flowState(activeMonthlyReview.flow_graph) === "empty";
    }
  });

  $effect(() => {
    if (activeMonthlyReview && sankeyEmpty) {
      flowRenderer.update(
        activeMonthlyReview.flow_graph,
        flowSignature(activeMonthlyReview),
        "empty"
      );
    }
  });

  let surplusInsightItem: InsightItem = $derived(
    !reviewHasActivity || !activeMonthlyReview
      ? {
          title: $t("dashboard.noRecordedActivity"),
          description: $t("dashboard.noPostingsForMonth", { month: selectedMonthLabel }),
          tone: "neutral",
          deltaLabel: $t("dashboard.insight.noActivityDelta"),
          state: "insufficient-data"
        }
      : reviewSurplusState === "ready"
        ? {
            title:
              activeMonthlyReview.surplus >= 0
                ? $t("dashboard.insight.surplusRecorded")
                : $t("dashboard.insight.deficitRecorded"),
            description: $t("dashboard.insight.surplusDefinition"),
            tone: activeMonthlyReview.surplus >= 0 ? "positive" : "negative",
            deltaLabel: formatCurrencyWithSymbol(activeMonthlyReview.surplus),
            state: "ready"
          }
        : {
            title:
              reviewSurplusState === "income-missing"
                ? $t("dashboard.insight.incomeNotRecorded")
                : reviewSurplusState === "expense-missing"
                  ? $t("dashboard.insight.expensesNotRecorded")
                  : $t("dashboard.insight.noIncomeOrExpenses"),
            description: $t("dashboard.insight.surplusIncomplete"),
            tone: "neutral",
            deltaLabel: $t("dashboard.insight.surplusNotCalculated"),
            state: "insufficient-data"
          }
  );

  let expenseInsightItem: InsightItem = $derived(
    reviewHasActivity && topExpense
      ? {
          title: $t("dashboard.insight.largestExpense", { group: topExpense.label }),
          description: $t("dashboard.insight.basedOnPostings", { month: selectedMonthLabel }),
          tone: "neutral",
          deltaLabel: formatCurrencyWithSymbol(topExpense.value),
          state: "ready"
        }
      : {
          title: $t("dashboard.insight.noExpenseGroups"),
          description: reviewHasActivity
            ? $t("dashboard.insight.noNonTaxExpenses", { month: selectedMonthLabel })
            : $t("dashboard.noPostingsForMonth", { month: selectedMonthLabel }),
          tone: "neutral",
          deltaLabel: $t("dashboard.insight.noExpensesPosted"),
          state: "insufficient-data"
        }
  );

  let investmentInsightItem: InsightItem = $derived(
    activeMonthlyReview?.performance_state === "ready" && reviewPerformance.best.length
      ? {
          title: $t("dashboard.insight.positionLedMonth", {
            position: positionLabel(reviewPerformance.best[0].account)
          }),
          description: reviewPerformance.weakest.length
            ? $t("dashboard.insight.strongestWeakest", {
                best: positionLabel(reviewPerformance.best[0].account),
                weakest: positionLabel(reviewPerformance.weakest[0].account)
              })
            : $t("dashboard.insight.rankedOnReturn"),
          tone:
            reviewPerformance.best[0].total_return_percent >= 0 &&
            (reviewPerformance.weakest[0]?.total_return_percent ?? 0) >= 0
              ? "positive"
              : reviewPerformance.best[0].total_return_percent < 0
                ? "negative"
                : "warning",
          deltaLabel: $t("dashboard.insight.marketMovementAmount", {
            amount: formatCurrencyWithSymbol(activeMonthlyReview.investment_market_movement)
          }),
          state: "ready"
        }
      : {
          title: $t("dashboard.insight.performanceUnavailable"),
          description: $t("dashboard.insight.insufficientRanking"),
          tone: "neutral",
          deltaLabel: $t("common.notAvailable"),
          state: "insufficient-data"
        }
  );

  let insights = $derived([surplusInsightItem, expenseInsightItem, investmentInsightItem]);

  let checkingHistory = $derived(_.sortBy(checkingBalanceTimeline, ["month", "account"]));
  let checkingMonths = $derived(_.uniq(checkingHistory.map((point) => point.month)));
  let checkingAccounts = $derived(_.uniq(checkingHistory.map((point) => point.account)));
  let checkingAccountSeries = $derived(
    checkingAccounts.map((account, index) => ({
      account,
      label: account.split(":").at(-1) || account,
      color: categoryColor(account, index),
      values: checkingMonths.map(
        (historyMonth) =>
          checkingHistory.find((point) => point.month === historyMonth && point.account === account)
            ?.balance ?? null
      )
    }))
  );
  let checkingAllValues = $derived(
    _.flatMap(checkingAccountSeries, (series) => series.values).filter(
      (value): value is number => value != null
    )
  );
  let checkingMin = $derived(_.min(checkingAllValues) ?? 0);
  let checkingMax = $derived(_.max(checkingAllValues) ?? 0);
  let checkingHasHistory = $derived(checkingMonths.length >= 2 && checkingAllValues.length >= 2);
  let checkingState: WidgetState = $derived(checkingHasHistory ? "ready" : "insufficient-data");
  let checkingValueTicks = $derived(checkingHasHistory ? valueTicks(checkingMin, checkingMax) : []);
  let checkingDateTicks = $derived(checkingHasHistory ? monthLabels(checkingMonths, 5) : []);
  let checkingTotal = $derived(
    _.sumBy(_.values(checkingBalances), (breakdown) => breakdown.marketAmount)
  );

  let monthlyNetworth = $derived(
    _.chain(networthTimeline)
      .groupBy((point) => dayjs(point.date).format("YYYY-MM"))
      .values()
      .map((points) => _.last(points))
      .sortBy((point) => dayjs(point.date).valueOf())
      .takeRight(12)
      .value()
  );
  let networthSeries = $derived(monthlyNetworth.map((point) => networthValue(point)));
  let latestNetworthPoint = $derived(_.last(networthTimeline) || networthOverview);
  let currentNetworthValue = $derived(latestNetworthPoint ? networthValue(latestNetworthPoint) : 0);
  let networthHistoryMonths = $derived(monthlyNetworth.length);
  let networthHasHistory = $derived(networthSeries.length >= 2);
  let networthTrendState: WidgetState = $derived(
    networthHasHistory ? "ready" : "insufficient-data"
  );
  let networthMin = $derived(_.min(networthSeries) ?? 0);
  let networthMax = $derived(_.max(networthSeries) ?? 0);
  let networthValueTicks = $derived(networthHasHistory ? valueTicks(networthMin, networthMax) : []);
  let networthDateTicks = $derived(
    networthHasHistory
      ? monthLabels(
          monthlyNetworth.map((point) => dayjs(point.date).format("YYYY-MM")),
          5
        )
      : []
  );

  let recentExpenseTotals = $derived(
    _.chain(cashFlows)
      .takeRight(3)
      .map((cashFlow) => cashFlow.expenses)
      .filter((value) => value > 0)
      .value()
  );
  let averageMonthlyExpense = $derived(
    recentExpenseTotals.length ? _.mean(recentExpenseTotals) : 0
  );
  let reserveState: WidgetState = $derived(
    averageMonthlyExpense > 0 ? "ready" : "insufficient-data"
  );
  let reserveMonths = $derived(
    reserveState === "ready" ? checkingTotal / averageMonthlyExpense : 0
  );

  let budgetPressure = $derived(
    _.chain(currentBudget?.accounts || [])
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
      .value()
  );
  let budgetState: WidgetState = $derived(
    !currentBudget
      ? "insufficient-data"
      : !currentBudget.accounts?.some((account) => account.budgeted > 0)
        ? "not-configured"
        : budgetPressure.length
          ? "ready"
          : "insufficient-data"
  );

  let recurringOutlook = $derived(
    transactionSequences
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
      .slice(0, 3)
  );
  let recurringState: WidgetState = $derived(
    recurringOutlook.length ? "ready" : "insufficient-data"
  );

  let goalProgress = $derived(
    goalSummaries.slice(0, 3).map((goal) => ({
      name: goal.name,
      percent: goal.target === 0 ? 0 : Math.round((goal.current / goal.target) * 100)
    }))
  );
  let goalState: WidgetState = $derived(
    goalSummaries.length ? (goalProgress.length ? "ready" : "insufficient-data") : "not-configured"
  );

  let hasDashboardData = $derived(
    cashFlows.length > 0 ||
      !_.isEmpty(checkingBalances) ||
      checkingBalanceTimeline.length > 0 ||
      goalSummaries.length > 0 ||
      transactionSequences.length > 0 ||
      !_.isEmpty(budgetsByMonth) ||
      !_.isEmpty(liabilityBreakdowns) ||
      networthTimeline.length > 0 ||
      currentNetworthValue !== 0 ||
      transactions.length > 0
  );
  let isEmpty = $derived(loadError || !hasDashboardData);
</script>

{#if isEmpty}
  <section class="section">
    <div class="paisa-empty-state">
      <div class="paisa-empty-state__copy">
        <p class="paisa-kicker">{$t("nav.overview")}</p>
        <h2>
          {loadError
            ? $t("dashboard.preview.serverErrorTitle")
            : $t("dashboard.preview.firstJournalTitle")}
        </h2>
        <p>
          {#if loadError}
            <!-- eslint-disable-next-line svelte/no-at-html-tags -->
            {@html $t("dashboard.preview.apiUnreachable", { url: "http://localhost:7500" })}
          {:else}
            {$t("dashboard.preview.cockpit")}
          {/if}
        </p>
        <ol>
          {#if loadError}
            <li>
              <!-- eslint-disable-next-line svelte/no-at-html-tags -->
              {@html $t("dashboard.preview.startBackend", { command: ".\\paisa.exe serve" })}
            </li>
            <li>
              <!-- eslint-disable-next-line svelte/no-at-html-tags -->
              {@html $t("dashboard.preview.keepDev", { command: "npm run dev" })}
            </li>
            <li>{$t("dashboard.preview.reload")}</li>
          {:else}
            <li>{$t("dashboard.preview.configure")}</li>
            <li>{$t("dashboard.preview.addEntries")}</li>
            <li>{$t("dashboard.preview.loadDemo")}</li>
          {/if}
        </ol>
        <div class="buttons">
          <a href="/more/config" class="button is-light"
            >{$t("dashboard.preview.openConfiguration")}</a
          >
          <a href="/ledger/editor" class="button is-light"
            >{$t("dashboard.preview.openLedgerEditor")}</a
          >
          {#if !loadError}
            <button onclick={(_event) => initDemo()} class="button is-link" type="button">
              {$t("dashboard.preview.loadDemoButton")}
            </button>
          {/if}
        </div>
      </div>
    </div>
  </section>
{:else}
  <section class="section pt-0 px-0">
    <div class="paisa-dashboard__toolbar">
      <button
        bind:this={customizeButtonEl}
        class="button is-small is-light"
        type="button"
        data-testid="preview-customize"
        onclick={() => (showDashboardSettings = true)}
      >
        <span class="icon is-small"><i class="fas fa-gear"></i></span>
        <span>{$t("dashboard.customize")}</span>
      </button>
    </div>
    <div class="paisa-dashboard" data-testid="preview-dashboard">
      {#if dashboardLayout.visible.hero}
        <div
          class="paisa-dashboard__section paisa-dashboard__hero"
          data-testid="preview-section-hero"
        >
          <div class="paisa-dashboard__hero-top">
            <div class="paisa-dashboard__hero-summary">
              <div>
                <div class="paisa-dashboard__label">{$t("dashboard.totalNetWorth")}</div>
                <div class="paisa-dashboard__amount">
                  {formatCurrencyWithSymbol(currentNetworthValue)}
                </div>
              </div>
            </div>

            <div class="paisa-dashboard__trend-group">
              <div class="paisa-spark-card paisa-spark-card--hero">
                <div class="paisa-spark-card__meta">
                  <div>
                    <span>{$t("dashboard.netWorth")}</span>
                    <strong>{formatCurrencyWithSymbol(currentNetworthValue)}</strong>
                  </div>
                  <div class="paisa-spark-card__change">
                    {#if networthHasHistory}
                      {$t("dashboard.months", { count: networthHistoryMonths })}
                    {:else}
                      {$t("common.needMoreHistory")}
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
                    {$t("dashboard.networthTrendInsufficient")}
                  </div>
                {/if}
              </div>

              <div class="paisa-spark-card paisa-spark-card--hero">
                <div class="paisa-spark-card__meta">
                  <div>
                    <span>{$t("dashboard.checking")}</span>
                    <strong>{formatCurrencyWithSymbol(checkingTotal)}</strong>
                  </div>
                  <div class="paisa-spark-card__change">
                    {#if checkingHasHistory}
                      {$t("dashboard.accounts", { count: checkingAccounts.length })}
                    {:else}
                      {$t("common.needMoreHistory")}
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
                    {$t("dashboard.checkingTrendInsufficient")}
                  </div>
                {/if}
              </div>
            </div>
          </div>
        </div>
      {/if}

      {#if dashboardLayout.visible.monthlyReview}
        <div class="paisa-dashboard__section" data-testid="preview-section-monthly-review">
          <div class="paisa-review">
            <div class="paisa-review__header">
              <div>
                <div class="paisa-zone__title">{$t("dashboard.monthlyReview")}</div>
                <div class="paisa-zone__subtle">
                  {selectedMonthLabel}{activeMonthlyReview && !activeMonthlyReview.period_complete
                    ? ` · ${$t("dashboard.inProgress")}`
                    : ""}
                </div>
              </div>
              {#if activeMonthlyReview}
                <div class="paisa-review__status">
                  {activeMonthlyReview.period_complete
                    ? $t("dashboard.periodComplete")
                    : $t("dashboard.monthToDate")}
                </div>
              {/if}
            </div>

            {#if reviewLoading}
              <div class="paisa-zone__empty">
                <strong>{$t("dashboard.loadingMonth", { month: selectedMonthLabel })}</strong>
              </div>
            {:else if monthlyReviewError}
              <div class="paisa-zone__empty">
                <strong>{$t("dashboard.reviewLoadError")}</strong>
                <p>{$t("dashboard.reviewLoadErrorHint")}</p>
              </div>
            {:else if !activeMonthlyReview}
              <div class="paisa-zone__empty">
                <strong>{$t("dashboard.reviewUnavailable")}</strong>
              </div>
            {:else}
              {#if reviewHasActivity}
                <div class="paisa-review__metrics">
                  <div class="paisa-review__metric">
                    <span>{$t("dashboard.netIncomeAfterTax")}</span>
                    <strong
                      >{formatCurrencyWithSymbol(activeMonthlyReview.net_income_after_tax)}</strong
                    >
                    <small>
                      {activeMonthlyReview.has_income_postings
                        ? $t("dashboard.taxAmount", {
                            amount: formatCurrencyWithSymbol(activeMonthlyReview.net_tax)
                          })
                        : $t("dashboard.noRecordedIncome")}
                    </small>
                  </div>
                  <div class="paisa-review__metric">
                    <span>{$t("dashboard.surplusDeficit")}</span>
                    {#if reviewSurplusState === "ready"}
                      <strong class:is-negative={activeMonthlyReview.surplus < 0}
                        >{formatCurrencyWithSymbol(activeMonthlyReview.surplus)}</strong
                      >
                      <small>{$t("dashboard.netIncomeMinusExpenses")}</small>
                    {:else}
                      <strong class="is-muted">{$t("common.notAvailable")}</strong>
                      <small>
                        {reviewSurplusState === "income-missing"
                          ? $t("dashboard.noIncomeThisMonth")
                          : reviewSurplusState === "expense-missing"
                            ? $t("dashboard.noExpensesThisMonth")
                            : $t("dashboard.noIncomeOrExpensesThisMonth")}
                      </small>
                    {/if}
                  </div>
                  <div class="paisa-review__metric">
                    <span>{$t("dashboard.netExpense")}</span>
                    <strong
                      >{activeMonthlyReview.has_expense_postings
                        ? formatCurrencyWithSymbol(activeMonthlyReview.net_expense)
                        : $t("common.notAvailable")}</strong
                    >
                    <small
                      >{activeMonthlyReview.has_expense_postings
                        ? $t("dashboard.excludesTax")
                        : $t("dashboard.noRecordedExpenses")}</small
                    >
                  </div>
                </div>

                <div class="paisa-review__trend">
                  <div class="paisa-zone__subtle">{$t("dashboard.netIncomeLastThreeMonths")}</div>
                  <div class="paisa-review__trend-values">
                    {#each reviewTrend as point, index}
                      {@const previous = index > 0 ? reviewTrend[index - 1] : null}
                      <div class="paisa-review__trend-point" class:is-mtd={point.isMonthToDate}>
                        <span>{dayjs(point.month + "-01").format("MMM YYYY")}</span>
                        <strong
                          >{point.value != null
                            ? formatCurrencyWithSymbol(point.value)
                            : $t("common.notAvailable")}</strong
                        >
                        {#if point.isMonthToDate}
                          <small>{$t("dashboard.monthToDate")}</small>
                        {:else if point.value != null && previous?.value != null}
                          <small
                            class:is-positive={point.value >= previous.value}
                            class:is-negative={point.value < previous.value}
                          >
                            {point.value >= previous.value ? "↑" : "↓"}
                            {formatCurrencyWithSymbol(Math.abs(point.value - previous.value))}
                          </small>
                        {:else}
                          <small>{$t("dashboard.noComparableMonth")}</small>
                        {/if}
                      </div>
                    {/each}
                  </div>
                </div>

                <div class="paisa-review__flow">
                  <div class="paisa-review__performance-header">
                    <div class="paisa-zone__subtle">{$t("dashboard.postedMoneyFlow")}</div>
                    <a href="/cash_flow/yearly" class="secondary-link paisa-link-with-arrow"
                      >{$t("dashboard.openYearlyFlow")} <i class="fas fa-arrow-right"></i></a
                    >
                  </div>
                  {#if sankeyEmpty}
                    <div class="paisa-zone__empty">
                      <strong>{$t("dashboard.noPostedFlows")}</strong>
                      <p>{$t("dashboard.noPostingsToConnect")}</p>
                    </div>
                  {:else}
                    <div class="paisa-review__flow-canvas">
                      <svg
                        id="d3-expense-flow"
                        height={flowChartHeight}
                        use:flowCanvas={activeMonthlyReview}
                      ></svg>
                    </div>
                  {/if}
                </div>
              {:else}
                <div class="paisa-review__no-activity">
                  <strong>{$t("dashboard.noRecordedActivity")}</strong>
                  <p>
                    {$t("dashboard.noPostingsForMonth", { month: selectedMonthLabel })}
                  </p>
                </div>
              {/if}

              <div class="paisa-review__performance">
                <div class="paisa-review__performance-header">
                  <div class="paisa-zone__subtle">{$t("nav.investments")}</div>
                  {#if reviewPerformance.asOf}
                    <div class="paisa-zone__subtle">
                      {$t("dashboard.pricesAsOf", {
                        date: dayjs(reviewPerformance.asOf).format("MMM D, YYYY")
                      })}
                    </div>
                  {/if}
                </div>

                <div class="paisa-review__investment-drivers">
                  <div>
                    <span>{$t("dashboard.netContributions")}</span><strong
                      >{formatCurrencyWithSymbol(
                        activeMonthlyReview.investment_contributions
                      )}</strong
                    ><small>{$t("dashboard.nonCheckingAssets")}</small>
                  </div>
                  <div>
                    <span>{$t("dashboard.marketMovement")}</span><strong
                      >{activeMonthlyReview.performance_state === "ready"
                        ? formatCurrencyWithSymbol(activeMonthlyReview.investment_market_movement)
                        : $t("common.notAvailable")}</strong
                    ><small>{$t("dashboard.rankedPositions")}</small>
                  </div>
                  <div>
                    <span>{$t("dashboard.dividends")}</span><strong
                      >{formatCurrencyWithSymbol(activeMonthlyReview.dividend_income)}</strong
                    ><small>{$t("dashboard.totalReturnComponent")}</small>
                  </div>
                </div>

                {#if activeMonthlyReview.performance_state === "ready" && reviewPerformance.best.length}
                  <div class="paisa-review__performers">
                    <div class="paisa-review__performer-group">
                      <div class="paisa-zone__subtle">{$t("dashboard.best")}</div>
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
                              >{$t("dashboard.dividendsAmount", {
                                amount: formatCurrencyWithSymbol(performer.dividend_income)
                              })}</small
                            >
                          {/if}
                        </div>
                      {/each}
                    </div>
                    <div class="paisa-review__performer-group">
                      <div class="paisa-zone__subtle">{$t("dashboard.weakest")}</div>
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
                              >{$t("dashboard.dividendsAmount", {
                                amount: formatCurrencyWithSymbol(performer.dividend_income)
                              })}</small
                            >
                          {/if}
                        </div>
                      {/each}
                    </div>
                  </div>
                {:else}
                  <div class="paisa-zone__empty">
                    <strong
                      >{$t("dashboard.investmentPerformanceUnavailable", {
                        month: selectedMonthLabel
                      })}</strong
                    >
                    <p>
                      {$t("dashboard.insufficientPrices")}
                    </p>
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        </div>
      {/if}

      {#if dashboardLayout.visible.insights}
        <div class="paisa-dashboard__section" data-testid="preview-section-insights">
          <div class="paisa-dashboard__band paisa-dashboard__band--review">
            <div class="paisa-zone paisa-zone--insights">
              <DashboardSectionHeader title={$t("nav.insights")} subtle={selectedMonthLabel}>
                {#snippet right()}
                  <a href="/expense/monthly" class="secondary-link paisa-link-with-arrow"
                    >{$t("dashboard.viewAllInsights")} <i class="fas fa-arrow-right"></i></a
                  >
                {/snippet}
              </DashboardSectionHeader>

              {#if reviewLoading}
                <div class="paisa-zone__empty">
                  <strong>{$t("dashboard.loadingMonth", { month: selectedMonthLabel })}</strong>
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
              <DashboardSectionHeader
                title={$t("dashboard.goalProgress")}
                subtle={$t("dashboard.currentStatus")}
              >
                {#snippet right()}
                  <a href="/more/goals" class="secondary-link paisa-link-with-arrow"
                    >{$t("dashboard.viewAllGoals")} <i class="fas fa-arrow-right"></i></a
                  >
                {/snippet}
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
                  <strong>{$t("dashboard.noGoalsConfigured")}</strong>
                  <p>{$t("dashboard.createGoals")}</p>
                </div>
              {:else}
                <div class="paisa-support__empty">
                  <strong>{$t("dashboard.goalProgressNotReady")}</strong>
                  <p>{$t("dashboard.goalNeedsData")}</p>
                </div>
              {/if}
            </div>
          </div>
        </div>
      {/if}

      {#if dashboardLayout.visible.support}
        <div class="paisa-dashboard__section" data-testid="preview-section-support">
          <div class="paisa-dashboard__band paisa-dashboard__band--support paisa-surface-split">
            <div class="paisa-support">
              <div class="paisa-support__header">
                <div class="paisa-zone__title">{$t("dashboard.cashReserve")}</div>
                <div class="paisa-zone__subtle">{$t("dashboard.current")}</div>
              </div>
              <div class="paisa-support__value">{formatCurrencyWithSymbol(checkingTotal)}</div>
              {#if reserveState === "ready"}
                <div class="paisa-support__text">
                  {$t("dashboard.reserveMonths", {
                    count: reserveMonths,
                    value: formatFloat(reserveMonths)
                  })}
                </div>
                <div class="paisa-progress">
                  <div
                    class="paisa-progress__fill"
                    style={`width: ${Math.min((reserveMonths / 9) * 100, 100)}%`}
                  ></div>
                </div>
                <div class="paisa-progress__ticks">
                  <span>0</span>
                  <span>{$t("dashboard.months3")}</span>
                  <span>{$t("dashboard.months6")}</span>
                  <span>{$t("dashboard.months9plus")}</span>
                </div>
              {:else}
                <div class="paisa-support__empty">
                  <strong>{$t("dashboard.needExpenseHistory")}</strong>
                  <p>{$t("dashboard.needExpenseHistoryHint")}</p>
                </div>
              {/if}
            </div>

            <div class="paisa-support">
              <div class="paisa-support__header">
                <div class="paisa-zone__title">{$t("dashboard.budgetPressure")}</div>
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
                  >{$t("dashboard.viewAllBudgets")}</a
                >
              {:else if budgetState === "not-configured"}
                <div class="paisa-support__empty">
                  <strong>{$t("dashboard.noBudgetConfigured")}</strong>
                  <p>{$t("dashboard.createBudgetLines")}</p>
                  <a
                    href="/expense/budget"
                    class="secondary-link paisa-mini-list__link paisa-zone__empty-link"
                    >{$t("dashboard.setUpBudget")}</a
                  >
                </div>
              {:else}
                <div class="paisa-support__empty">
                  <strong>{$t("dashboard.noBudgetData", { month: selectedMonthLabel })}</strong>
                  <p>{$t("dashboard.budgetPressureHint")}</p>
                </div>
              {/if}
            </div>

            <div class="paisa-support">
              <div class="paisa-support__header">
                <div class="paisa-zone__title">{$t("dashboard.recurringOutlook")}</div>
                <div class="paisa-zone__subtle">{$t("dashboard.next7Days")}</div>
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
                  >{$t("dashboard.viewAllRecurring")}</a
                >
              {:else}
                <div class="paisa-support__empty">
                  <strong>{$t("dashboard.noRecurringOutlook")}</strong>
                  <p>{$t("dashboard.recurringHint")}</p>
                </div>
              {/if}
            </div>
          </div>
        </div>
      {/if}

      {#if dashboardLayout.visible.recentTransactions && transactions.length > 0}
        <PreviewRecentTransactions {transactions} />
      {/if}

      <div class="paisa-dashboard__section paisa-dashboard__footer">
        <span>{$t("dashboard.liveSnapshot")}</span>
        <span>{$t("dashboard.lastUpdated", { time: now().format("HH:mm") })}</span>
      </div>
    </div>
  </section>
{/if}

<PreviewDashboardSettings
  bind:open={showDashboardSettings}
  layout={dashboardLayout}
  onchange={updateDashboardLayout}
  returnFocusTo={customizeButtonEl}
/>
