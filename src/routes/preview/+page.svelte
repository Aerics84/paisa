<script lang="ts">
  import * as d3 from "d3";
  import dayjs from "dayjs";
  import _ from "lodash";
  import { onMount } from "svelte";
  import { byExpenseGroup } from "$lib/expense";
  import { categoryColor, categoryVisual } from "$lib/colors";
  import DashboardSectionHeader from "$lib/components/dashboard/DashboardSectionHeader.svelte";
  import DashboardStatStripItem from "$lib/components/dashboard/DashboardStatStripItem.svelte";
  import {
    enrichTrantionSequence,
    nextUnpaidSchedule,
    scheduleIcon,
    sortTrantionSequence
  } from "$lib/transaction_sequence";
  import {
    ajax,
    formatCurrency,
    formatCurrencyCrudeWithPrecision,
    formatFloat,
    formatPercentage,
    type AssetBreakdown,
    type Budget,
    type CashFlow,
    type GoalSummary,
    type LiabilityBreakdown,
    type Networth,
    type Posting,
    type TransactionSequence,
    now
  } from "$lib/utils";
  import { month, refresh } from "../../store";

  type CategorySummary = {
    category: string;
    total: number;
    delta: number;
    icon: string;
    color: string;
  };

  type InsightTone = "positive" | "warning" | "negative";

  type InsightItem = {
    title: string;
    description: string;
    tone: InsightTone;
    deltaLabel: string;
    state: "ready" | "insufficient-data";
    series: number[];
  };

  type WidgetState = "ready" | "insufficient-data" | "not-configured";

  type DriverDisplayItem = {
    label: string;
    value: number;
    icon: string;
    widthPercent: number;
    tone: "positive" | "negative";
  };
  const heroChartHeight = 112;

  let cashFlows: CashFlow[] = [];
  let checkingBalances: Record<string, AssetBreakdown> = {};
  let goalSummaries: GoalSummary[] = [];
  let transactionSequences: TransactionSequence[] = [];
  let budgetsByMonth: Record<string, Budget> = {};
  let groupedExpenses: { [key: string]: Posting[] } = {};
  let liabilityBreakdowns: LiabilityBreakdown[] = [];
  let networthOverview: Networth;
  let networthTimeline: Networth[] = [];
  let loadError = false;

  async function initDemo() {
    await ajax("/api/init", {
      method: "POST",
      body: JSON.stringify({
        regional_profile: USER_CONFIG?.regional_profile || "india"
      })
    });
    refresh();
  }

  onMount(async () => {
    try {
      const [dashboard, networthResponse, liabilitiesResponse] = await Promise.all([
        ajax("/api/dashboard"),
        ajax("/api/networth"),
        ajax("/api/liabilities/balance")
      ]);

      groupedExpenses = dashboard.expenses;
      cashFlows = dashboard.cashFlows;
      goalSummaries = _.sortBy(dashboard.goalSummaries, (goal) => -goal.priority);
      budgetsByMonth = dashboard.budget.budgetsByMonth;
      transactionSequences = _.take(
        sortTrantionSequence(enrichTrantionSequence(dashboard.transactionSequences)),
        12
      );
      checkingBalances = dashboard.checkingBalances.asset_breakdowns;
      networthOverview = dashboard.networth.networth;
      networthTimeline = networthResponse.networthTimeline;
      liabilityBreakdowns = liabilitiesResponse.liability_breakdowns;
      loadError = false;
    } catch (_error) {
      loadError = true;
    }
  });

  function previousMonthKey(value: string) {
    return dayjs(value, "YYYY-MM").subtract(1, "month").format("YYYY-MM");
  }

  function percentDelta(current: number, previous: number) {
    if (!previous) {
      return current ? 1 : 0;
    }
    return (current - previous) / Math.abs(previous);
  }

  function compactCategories(expenses: Posting[]) {
    const previousExpenses = groupedExpenses[previousMonthKey(monthKey)] || [];
    const currentGroups = _.values(byExpenseGroup(expenses));
    const previousGroups = byExpenseGroup(previousExpenses);

    const categories = _.chain(currentGroups)
      .map((group) => ({
        category: group.category,
        total: group.total,
        delta: percentDelta(group.total, previousGroups[group.category]?.total || 0)
      }))
      .orderBy(["total"], ["desc"])
      .value();

    if (categories.length <= 7) {
      return categories.map((category, index) => ({
        ...category,
        ...categoryVisual(category.category, index)
      }));
    }

    const visible = categories.slice(0, 6).map((category, index) => ({
      ...category,
      ...categoryVisual(category.category, index)
    }));
    const other = categories.slice(6);
    visible.push({
      category: "Other",
      total: _.sumBy(other, (item) => item.total),
      delta: _.meanBy(other, (item) => item.delta) || 0,
      ...categoryVisual("other", visible.length)
    });
    return visible;
  }

  function sparklinePath(values: number[], width = 220, height = 68) {
    const safe = values.filter((value) => Number.isFinite(value));
    if (safe.length <= 1) {
      return "";
    }

    const min = _.min(safe) ?? 0;
    const max = _.max(safe) ?? min + 1;
    const x = d3
      .scaleLinear()
      .domain([0, safe.length - 1])
      .range([0, width]);
    const y = d3
      .scaleLinear()
      .domain([min, max === min ? max + 1 : max])
      .range([height, 0]);

    return (
      d3
        .line<number>()
        .x((_value, index) => x(index))
        .y((value) => y(value))
        .curve(d3.curveMonotoneX)(safe) || ""
    );
  }

  function networthValue(point: Networth) {
    return point?.investmentAmount + point?.gainAmount - point?.withdrawalAmount;
  }

  function hasEnoughPoints(values: number[], min = 2) {
    return values.filter((value) => Number.isFinite(value)).length >= min;
  }

  function sparklineValueTicks(values: number[], count = 3, height = heroChartHeight) {
    const safe = values.filter((value) => Number.isFinite(value));
    if (!safe.length) {
      return [];
    }

    const min = _.min(safe) ?? 0;
    const max = _.max(safe) ?? min + 1;
    const spread = max - min || Math.max(Math.abs(max), 1);

    return _.range(count).map((index) => {
      const ratio = count === 1 ? 0 : index / (count - 1);
      const value = max - spread * ratio;

      return {
        label: formatCurrencyCrudeWithPrecision(value, 0),
        top: ratio * 100,
        y: ratio * height
      };
    });
  }

  function sampledIndices(length: number, count = 5) {
    if (length <= count) {
      return _.range(length);
    }

    return _.uniq(_.range(count).map((index) => Math.round((index * (length - 1)) / (count - 1))));
  }

  function sparklineDateLabels(points: Array<{ date: unknown }>, count = 5) {
    if (!points?.length) {
      return [];
    }

    const recent = _.takeRight(points, Math.max(count, Math.min(points.length, 8)));
    const labels = sampledIndices(recent.length, count)
      .map((index) => dayjs(recent[index]?.date as Parameters<typeof dayjs>[0]).format("MMM D"))
      .filter((label) => label !== "Invalid Date");

    return labels;
  }

  function classifyDriver(value: number) {
    return value >= 0 ? "positive" : "negative";
  }

  $: monthKey = $month || now().format("YYYY-MM");
  $: currentBudget = budgetsByMonth[monthKey];
  $: currentExpenses = groupedExpenses[monthKey] || [];
  $: previousExpenses = groupedExpenses[previousMonthKey(monthKey)] || [];
  $: totalSpent = _.sumBy(currentExpenses, (posting) => posting.amount);
  $: previousTotalSpent = _.sumBy(previousExpenses, (posting) => posting.amount);
  $: expenseCategories = compactCategories(currentExpenses);
  $: donutArc = d3.arc<d3.PieArcDatum<CategorySummary>>().innerRadius(72).outerRadius(116);

  $: checkingTotal = _.sumBy(_.values(checkingBalances), (breakdown) => breakdown.marketAmount);
  $: latestNetworthPoint = _.last(networthTimeline) || networthOverview;
  $: previousNetworthPoint =
    networthTimeline.length > 1 ? networthTimeline[networthTimeline.length - 2] : null;
  $: currentNetworthValue = latestNetworthPoint ? networthValue(latestNetworthPoint) : 0;
  $: previousNetworthValue = previousNetworthPoint
    ? networthValue(previousNetworthPoint)
    : currentNetworthValue;
  $: liabilitiesTotal = _.sumBy(liabilityBreakdowns, (breakdown) => breakdown.balance_amount);
  $: assetsTotal = currentNetworthValue + liabilitiesTotal;
  $: cashFlowLatest = _.last(cashFlows);
  $: cashFlowMonthToDate = cashFlowLatest
    ? cashFlowLatest.income -
      cashFlowLatest.expenses -
      cashFlowLatest.liabilities -
      cashFlowLatest.tax -
      cashFlowLatest.investment
    : 0;
  $: checkingSeries = _.takeRight(
    cashFlows.map((point) => point.balance || point.checking),
    8
  );
  $: equitySeries = _.takeRight(
    networthTimeline.map((point) => networthValue(point)),
    8
  );
  $: oneYearSeries = _.takeRight(
    networthTimeline.map((point) => networthValue(point)),
    12
  );
  $: hasCheckingHistory = hasEnoughPoints(checkingSeries);
  $: hasNetworthHistory = hasEnoughPoints(equitySeries);
  $: hasOneYearHistory = hasEnoughPoints(oneYearSeries);
  $: checkingState: WidgetState = hasCheckingHistory ? "ready" : "insufficient-data";
  $: networthTrendState: WidgetState = hasNetworthHistory ? "ready" : "insufficient-data";
  $: oneYearState: WidgetState = hasOneYearHistory ? "ready" : "insufficient-data";
  $: checkingValueTicks = hasCheckingHistory ? sparklineValueTicks(checkingSeries) : [];
  $: equityValueTicks = hasNetworthHistory ? sparklineValueTicks(equitySeries) : [];
  $: checkingDateTicks = hasCheckingHistory ? sparklineDateLabels(cashFlows) : [];
  $: equityDateTicks = hasNetworthHistory ? sparklineDateLabels(networthTimeline) : [];

  $: topCategory = _.maxBy(expenseCategories, (category) => Math.abs(category.delta));
  $: monthlyExpenseSeries = _(Object.entries(groupedExpenses))
    .sortBy(([date]) => date)
    .takeRight(6)
    .map(([_date, postings]) => _.sumBy(postings, (posting) => posting.amount))
    .value();
  $: topCategorySeries = topCategory
    ? _(Object.entries(groupedExpenses))
        .sortBy(([date]) => date)
        .takeRight(6)
        .map(([_date, postings]) => byExpenseGroup(postings)[topCategory.category]?.total || 0)
        .value()
    : [];
  $: hasPreviousExpenseMonth = _.has(groupedExpenses, previousMonthKey(monthKey));
  $: spendingState: WidgetState = currentExpenses.length ? "ready" : "insufficient-data";
  $: expensePie = d3
    .pie<CategorySummary>()
    .sort(null)
    .value((entry) => entry.total)(expenseCategories);

  function toneForExpenseDelta(delta: number): InsightTone {
    if (delta < -0.03) {
      return "positive";
    }
    if (delta > 0.1) {
      return "negative";
    }
    return "warning";
  }

  $: hasSpendingInsight = hasPreviousExpenseMonth && hasEnoughPoints(monthlyExpenseSeries);
  $: hasCategoryInsight = !!topCategory && hasEnoughPoints(topCategorySeries);
  $: hasNetworthInsight = !!previousNetworthPoint;
  $: insights = [
    hasSpendingInsight
      ? {
          title: totalSpent <= previousTotalSpent ? "Spending improving" : "Spending trending up",
          description:
            totalSpent <= previousTotalSpent
              ? `You spent ${formatPercentage(
                  Math.abs(percentDelta(totalSpent, previousTotalSpent)),
                  0
                )} less than last month.`
              : `${formatPercentage(
                  percentDelta(totalSpent, previousTotalSpent),
                  0
                )} more than last month.`,
          tone: toneForExpenseDelta(percentDelta(totalSpent, previousTotalSpent)),
          deltaLabel: `${formatCurrency(totalSpent - previousTotalSpent)} vs last month`,
          state: "ready",
          series: monthlyExpenseSeries
        }
      : {
          title: "Spending trend unavailable",
          description: hasPreviousExpenseMonth
            ? "Post expenses in both months to compare spending trends."
            : "Add another month of expense history to compare spending.",
          tone: "warning",
          deltaLabel: "Need at least two months of expense history",
          state: "insufficient-data",
          series: []
        },
    hasCategoryInsight
      ? {
          title: `${topCategory.category} ${topCategory.delta > 0 ? "trending up" : "cooling off"}`,
          description: `${formatPercentage(Math.abs(topCategory.delta), 0)} ${
            topCategory.delta > 0 ? "higher" : "lower"
          } than last month.`,
          tone: toneForExpenseDelta(topCategory.delta),
          deltaLabel: formatCurrency(topCategory.total),
          state: "ready",
          series: topCategorySeries
        }
      : {
          title: "Category trend unavailable",
          description: currentExpenses.length
            ? "Keep posting expenses over time to unlock category comparisons."
            : "Post expenses for the selected month to unlock category trends.",
          tone: "warning",
          deltaLabel: "Need repeated category history",
          state: "insufficient-data",
          series: []
        },
    hasNetworthInsight
      ? {
          title:
            currentNetworthValue >= previousNetworthValue
              ? "Net worth moving up"
              : "Net worth under pressure",
          description:
            currentNetworthValue >= previousNetworthValue
              ? "Net worth improved since the last snapshot."
              : "Net worth declined compared with the last snapshot.",
          tone: currentNetworthValue >= previousNetworthValue ? "positive" : "negative",
          deltaLabel: `${formatCurrency(currentNetworthValue - previousNetworthValue)} net change`,
          state: "ready",
          series: _.takeRight(
            networthTimeline.map((point) => networthValue(point)),
            6
          )
        }
      : {
          title: "Net worth trend unavailable",
          description: "Add another net worth snapshot to compare movement over time.",
          tone: "warning",
          deltaLabel: "Need at least two snapshots",
          state: "insufficient-data",
          series: []
        }
  ] satisfies InsightItem[];

  $: checkingChange = hasCheckingHistory ? checkingSeries.at(-1) - checkingSeries.at(-2) : null;
  $: checkingChangePercent = hasCheckingHistory
    ? percentDelta(checkingSeries.at(-1) || 0, checkingSeries.at(-2) || 0)
    : null;
  $: networthChange = previousNetworthPoint ? currentNetworthValue - previousNetworthValue : null;
  $: networthChangePercent = previousNetworthPoint
    ? percentDelta(currentNetworthValue, previousNetworthValue)
    : null;
  $: networthHistoryMonths = networthTimeline.length;
  $: investmentGainChange =
    latestNetworthPoint && previousNetworthPoint
      ? latestNetworthPoint.gainAmount - previousNetworthPoint.gainAmount
      : 0;
  $: netContributionChange =
    latestNetworthPoint && previousNetworthPoint
      ? latestNetworthPoint.netInvestmentAmount - previousNetworthPoint.netInvestmentAmount
      : 0;
  $: driverState: WidgetState = previousNetworthPoint ? "ready" : "insufficient-data";
  $: driverBase = previousNetworthPoint
    ? [
        {
          label: "Net contributions",
          value: netContributionChange,
          icon: "fa-circle-plus"
        },
        {
          label: "Market gain / loss",
          value: investmentGainChange,
          icon: "fa-chart-line"
        }
      ]
    : [];
  $: driverMax = _.max(driverBase.map((driver) => Math.abs(driver.value))) || 1;
  $: displayedNetWorthDrivers = driverBase.map((driver) => ({
    ...driver,
    widthPercent: Math.max((Math.abs(driver.value) / driverMax) * 100, 0),
    tone: classifyDriver(driver.value)
  })) satisfies DriverDisplayItem[];
  $: driverImpactDisplay = previousNetworthPoint ? netContributionChange + investmentGainChange : 0;

  $: recentExpenseTotals = _(Object.entries(groupedExpenses))
    .sortBy(([date]) => date)
    .takeRight(3)
    .map(([_date, postings]) => _.sumBy(postings, (posting) => posting.amount))
    .filter((value) => value > 0)
    .value();
  $: averageMonthlyExpense = recentExpenseTotals.length ? _.mean(recentExpenseTotals) : 0;
  $: reserveState: WidgetState = averageMonthlyExpense > 0 ? "ready" : "insufficient-data";
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
  $: budgetState: WidgetState = !currentBudget?.accounts?.some((account) => account.budgeted > 0)
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
  $: recurringState: WidgetState = recurringOutlook.length ? "ready" : "insufficient-data";

  $: goalProgress = goalSummaries.slice(0, 3).map((goal) => ({
    name: goal.name,
    percent: goal.target === 0 ? 0 : Math.round((goal.current / goal.target) * 100)
  }));
  $: goalState: WidgetState = goalSummaries.length
    ? goalProgress.length
      ? "ready"
      : "insufficient-data"
    : "not-configured";
  $: hasDashboardData =
    !_.isEmpty(groupedExpenses) ||
    cashFlows.length > 0 ||
    !_.isEmpty(checkingBalances) ||
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
              <div class="paisa-dashboard__amount">{formatCurrency(currentNetworthValue)}</div>
              {#if previousNetworthPoint}
                <div
                  class="paisa-dashboard__delta {currentNetworthValue >= previousNetworthValue
                    ? 'is-positive'
                    : 'is-negative'}"
                >
                  {formatCurrency(networthChange)}
                  <span>{formatPercentage(networthChangePercent, 2)} vs last month</span>
                </div>
              {:else}
                <div class="paisa-dashboard__delta">
                  <span>Need at least two net worth snapshots for a month-over-month delta.</span>
                </div>
              {/if}
            </div>
          </div>

          <div class="paisa-dashboard__trend-group">
            <div class="paisa-spark-card paisa-spark-card--hero">
              <div class="paisa-spark-card__meta">
                <div>
                  <span>Checking</span>
                  <strong>{formatCurrency(checkingTotal)}</strong>
                </div>
                <div
                  class="paisa-spark-card__change"
                  class:positive={checkingChange != null && checkingChange >= 0}
                  class:negative={checkingChange != null && checkingChange < 0}
                >
                  {#if checkingState === "ready"}
                    {formatPercentage(checkingChangePercent, 2)}
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
                      <path
                        d={sparklinePath(checkingSeries, 420, heroChartHeight)}
                        class="is-checking"
                      />
                    </svg>
                    <div class="paisa-spark-card__x-axis">
                      {#each checkingDateTicks as label}
                        <span>{label}</span>
                      {/each}
                    </div>
                  </div>
                </div>
              {:else}
                <div class="paisa-spark-card__empty">
                  Need at least two checking snapshots to plot this trend.
                </div>
              {/if}
            </div>
            <div class="paisa-spark-card paisa-spark-card--hero">
              <div class="paisa-spark-card__meta">
                <div>
                  <span>Net Worth</span>
                  <strong>{formatCurrency(currentNetworthValue)}</strong>
                </div>
                <div
                  class="paisa-spark-card__change"
                  class:positive={networthChange != null && networthChange >= 0}
                  class:negative={networthChange != null && networthChange < 0}
                >
                  {#if networthTrendState === "ready"}
                    {formatPercentage(networthChangePercent, 2)}
                  {:else}
                    Need more history
                  {/if}
                </div>
              </div>
              {#if networthTrendState === "ready"}
                <div class="paisa-spark-card__chart">
                  <div class="paisa-spark-card__y-axis">
                    {#each equityValueTicks as tick}
                      <span style={`top: calc(${tick.top}% - 0.55rem);`}>{tick.label}</span>
                    {/each}
                  </div>
                  <div class="paisa-spark-card__plot">
                    <svg
                      viewBox={`0 0 420 ${heroChartHeight}`}
                      class="paisa-sparkline paisa-sparkline--hero"
                    >
                      {#each equityValueTicks as tick}
                        <line
                          x1="0"
                          y1={tick.y}
                          x2="420"
                          y2={tick.y}
                          class="paisa-sparkline__grid"
                        />
                      {/each}
                      <path
                        d={sparklinePath(equitySeries, 420, heroChartHeight)}
                        class="is-equity"
                      />
                    </svg>
                    <div class="paisa-spark-card__x-axis">
                      {#each equityDateTicks as label}
                        <span>{label}</span>
                      {/each}
                    </div>
                  </div>
                </div>
              {:else}
                <div class="paisa-spark-card__empty">
                  Need at least two net worth snapshots to plot this trend.
                </div>
              {/if}
            </div>
          </div>

          <div class="paisa-dashboard__hero-rail">
            <div class="paisa-dashboard__rail-grid">
              <div class="paisa-rail-stat">
                <span>Assets</span>
                <strong>{formatCurrency(assetsTotal)}</strong>
              </div>
              <div class="paisa-rail-stat">
                <span>Liabilities</span>
                <strong>{formatCurrency(liabilitiesTotal)}</strong>
              </div>
              <div class="paisa-rail-stat paisa-rail-stat--rank">
                <span>Net Worth History</span>
                <div class="paisa-rail-stat__stack">
                  <strong>{networthHistoryMonths} mo</strong>
                  <small>captured</small>
                </div>
              </div>
              <div class="paisa-rail-stat paisa-rail-stat--trend">
                <span>1Y Net Worth Trend</span>
                {#if oneYearState === "ready"}
                  <svg viewBox="0 0 150 38" class="paisa-sparkline paisa-sparkline--compact">
                    <path d={sparklinePath(oneYearSeries, 150, 38)} class="is-checking" />
                  </svg>
                {:else}
                  <small>Need more than one monthly point</small>
                {/if}
              </div>
            </div>
          </div>
        </div>

        <div class="paisa-stat-strip">
          <DashboardStatStripItem label="Assets" value={formatCurrency(assetsTotal)} />
          <DashboardStatStripItem label="Liabilities" value={formatCurrency(liabilitiesTotal)} />
          <DashboardStatStripItem label="Net Worth" value={formatCurrency(currentNetworthValue)} />
          <DashboardStatStripItem
            label="Cash Flow (MTD)"
            value={formatCurrency(cashFlowMonthToDate)}
            valueClass={cashFlowMonthToDate >= 0 ? "positive" : "negative"}
          />
          <DashboardStatStripItem
            label="Net Worth History"
            value={`${networthHistoryMonths} mo`}
            meta="captured"
            variant="rank"
          />
          <DashboardStatStripItem label="1Y Net Worth Trend" variant="trend">
            {#if oneYearState === "ready"}
              <svg viewBox="0 0 150 38" class="paisa-sparkline paisa-sparkline--compact">
                <path d={sparklinePath(oneYearSeries, 150, 38)} class="is-checking" />
              </svg>
            {:else}
              <span class="paisa-stat-strip__empty">Need more history</span>
            {/if}
          </DashboardStatStripItem>
        </div>
      </div>

      <div class="paisa-dashboard__section">
        <div class="paisa-dashboard__band paisa-dashboard__band--middle">
          <div class="paisa-zone paisa-zone--spending">
            <DashboardSectionHeader
              title="A. Where Money Went This Month"
              subtle="This month"
              selectable
            />

            {#if spendingState === "ready"}
              <div class="paisa-spending">
                <div class="paisa-spending__chart">
                  <svg viewBox="-140 -140 280 280">
                    {#each expensePie as slice, index}
                      <path
                        d={donutArc(slice)}
                        fill={slice.data.color || categoryColor(slice.data.category, index)}
                      />
                    {/each}
                  </svg>
                  <div class="paisa-spending__center">
                    <strong>{formatCurrency(totalSpent)}</strong>
                    <span>Total spent</span>
                  </div>
                </div>

                <div class="paisa-spending__legend">
                  {#each expenseCategories as category}
                    <div class="paisa-spending__item">
                      <div class="paisa-spending__item-label">
                        <span class="icon" style={`color: ${category.color};`}>
                          <i class={"fas " + category.icon}></i>
                        </span>
                        <span>{category.category}</span>
                      </div>
                      <div class="paisa-spending__item-metrics">
                        <strong>{formatCurrency(category.total)}</strong>
                        <span
                          class:positive={category.delta <= 0}
                          class:negative={category.delta > 0}
                        >
                          {category.delta > 0 ? "+" : category.delta < 0 ? "-" : "--"}
                          {formatPercentage(Math.abs(category.delta), 0)}
                        </span>
                      </div>
                    </div>
                  {/each}
                </div>
              </div>
            {:else}
              <div class="paisa-zone__empty">
                <strong>No expenses posted for this month.</strong>
                <p>Post expenses in the selected month to unlock the spending breakdown.</p>
              </div>
            {/if}
          </div>

          <div class="paisa-zone paisa-zone--insights">
            <DashboardSectionHeader title="B. Insights">
              <svelte:fragment slot="right">
                <a href="/expense/monthly" class="secondary-link paisa-link-with-arrow"
                  >View all insights <i class="fas fa-arrow-right"></i></a
                >
              </svelte:fragment>
            </DashboardSectionHeader>

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
                              : "fa-arrow-trend-down")}
                      ></i>
                    </span>
                  </div>
                  <div class="paisa-insight__copy">
                    <div class="paisa-insight__title">{insight.title}</div>
                    <div class="paisa-insight__text">{insight.description}</div>
                    <div class="paisa-insight__delta">{insight.deltaLabel}</div>
                  </div>
                  {#if insight.state === "ready"}
                    <svg viewBox="0 0 130 46" class="paisa-sparkline paisa-sparkline--mini">
                      <path
                        d={sparklinePath(insight.series, 130, 46)}
                        class={"is-" + insight.tone}
                      />
                    </svg>
                  {/if}
                </article>
              {/each}
            </div>
          </div>

          <div class="paisa-zone paisa-zone--drivers">
            <DashboardSectionHeader title="C. Net Worth Drivers" subtle="This month" selectable />
            {#if driverState === "ready"}
              <div class="paisa-driver-list">
                {#each displayedNetWorthDrivers as driver}
                  <div class="paisa-driver">
                    <div class="paisa-driver__row">
                      <span class="paisa-driver__label">
                        <span class="icon">
                          <i class={"fas " + driver.icon}></i>
                        </span>
                        <span>{driver.label}</span>
                      </span>
                      <strong class:negative={driver.value < 0} class:positive={driver.value >= 0}>
                        {formatCurrency(driver.value)}
                      </strong>
                    </div>
                    <div class="paisa-driver__track">
                      <div
                        class={"paisa-driver__fill " + driver.tone}
                        style={`width: ${driver.widthPercent}%`}
                      ></div>
                    </div>
                  </div>
                {/each}
              </div>
              <div class="paisa-driver__impact">
                <span>Net impact</span>
                <strong
                  class:negative={driverImpactDisplay < 0}
                  class:positive={driverImpactDisplay >= 0}
                >
                  {formatCurrency(driverImpactDisplay)}
                </strong>
              </div>
            {:else}
              <div class="paisa-zone__empty">
                <strong>Net worth drivers need more history.</strong>
                <p>Add another snapshot so contributions and market movement can be compared.</p>
              </div>
            {/if}
          </div>
        </div>
      </div>

      <div class="paisa-dashboard__section">
        <div class="paisa-dashboard__band paisa-dashboard__band--bottom paisa-surface-split">
          <div class="paisa-support">
            <div class="paisa-zone__title">Cash Reserve</div>
            <div class="paisa-support__value">{formatCurrency(checkingTotal)}</div>
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
              <div class="paisa-zone__subtle">This month</div>
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
                <strong>Budget data is not ready yet.</strong>
                <p>Keep posting expenses this month to compare actuals against your budget.</p>
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
                      <strong>{formatCurrency(recurring.amount)}</strong>
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

          <div class="paisa-support">
            <div class="paisa-support__header">
              <div class="paisa-zone__title">Goal Progress</div>
              <a
                href="/more/goals"
                class="secondary-link paisa-mini-list__link paisa-mini-list__link--header"
                >View all goals</a
              >
            </div>
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

      <div class="paisa-dashboard__section paisa-dashboard__footer">
        <span>Last updated: Today, {now().format("HH:mm")}</span>
        <span>Classic and preview dashboards remain available in parallel.</span>
      </div>
    </div>
  </section>
{/if}
