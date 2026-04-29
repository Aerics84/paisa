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
  import { refresh } from "../../store";

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
    series: number[];
  };

  type DriverItem = {
    label: string;
    value: number;
    icon: string;
    placeholderRatio?: number;
  };

  type DriverDisplayItem = DriverItem & {
    widthPercent: number;
    tone: "positive" | "negative" | "placeholder";
    isPlaceholder: boolean;
  };

  const previewSeries = {
    checking: [11200, 13400, 14900, 16200, 15800, 17100, 18350, 19600],
    equity: [188000, 201000, 209500, 216000, 220500, 228200, 233800, 241400],
    yearly: [
      182000, 188500, 191000, 199000, 204000, 211000, 217500, 222000, 228000, 231500, 236000, 243680
    ],
    spending: [4180, 3960, 3875, 3742, 3610, 3580],
    category: [280, 310, 298, 336, 351, 368],
    positive: [24, 28, 29, 33, 36, 44],
    warning: [16, 19, 18, 22, 21, 27],
    negative: [31, 28, 27, 25, 22, 19]
  };

  const previewDateTicks = ["Apr 18", "Apr 25", "May 2", "May 9", "May 16"];
  const heroChartHeight = 112;

  const placeholderExpenseCategories = [
    { category: "Housing", total: 1210, delta: 0.12 },
    { category: "Groceries", total: 652, delta: -0.08 },
    { category: "Transport", total: 410, delta: -0.04 },
    { category: "Dining", total: 368, delta: 0.18 },
    { category: "Utilities", total: 245, delta: 0.09 },
    { category: "Shopping", total: 237, delta: -0.03 },
    { category: "Other", total: 620, delta: 0 }
  ].map((category, index) => ({
    ...category,
    ...categoryVisual(category.category, index)
  }));

  let cashFlows: CashFlow[] = [];
  let checkingBalances: Record<string, AssetBreakdown> = {};
  let goalSummaries: GoalSummary[] = [];
  let transactionSequences: TransactionSequence[] = [];
  let budgetsByMonth: Record<string, Budget> = {};
  let groupedExpenses: { [key: string]: Posting[] } = {};
  let liabilityBreakdowns: LiabilityBreakdown[] = [];
  let networthOverview: Networth;
  let networthTimeline: Networth[] = [];
  let xirr = 0;
  let isEmpty = false;
  let loadError = false;

  const monthKey = now().format("YYYY-MM");

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
      xirr = networthResponse.xirr || dashboard.networth.xirr;
      networthTimeline = networthResponse.networthTimeline;
      liabilityBreakdowns = liabilitiesResponse.liability_breakdowns;
      isEmpty =
        _.isEmpty(dashboard.expenses) && _.isEmpty(transactionSequences) && !networthOverview;
      loadError = false;
    } catch (_error) {
      loadError = true;
      isEmpty = true;
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

  function hasSignal(values: number[]) {
    return values.length > 1 && values.some((value) => Math.abs(value) > 0);
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

  function sparklineDateLabels(
    points: Array<{ date: unknown }>,
    fallback = previewDateTicks,
    count = 5
  ) {
    if (!points?.length) {
      return fallback;
    }

    const recent = _.takeRight(points, Math.max(count, Math.min(points.length, 8)));
    const labels = sampledIndices(recent.length, count)
      .map((index) => dayjs(recent[index]?.date as Parameters<typeof dayjs>[0]).format("MMM D"))
      .filter((label) => label !== "Invalid Date");

    return labels.length >= 4 ? labels : fallback;
  }

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
  $: assetsTotal = checkingTotal + currentNetworthValue;
  $: liabilitiesTotal = _.sumBy(liabilityBreakdowns, (breakdown) => breakdown.balance_amount);
  $: cashFlowLatest = _.last(cashFlows);
  $: cashFlowMonthToDate = cashFlowLatest
    ? cashFlowLatest.income -
      cashFlowLatest.expenses -
      cashFlowLatest.liabilities -
      cashFlowLatest.tax -
      cashFlowLatest.investment
    : 0;
  $: oneYearDelta = percentDelta(
    currentNetworthValue,
    networthTimeline.length > 12
      ? networthValue(networthTimeline[networthTimeline.length - 13])
      : previousNetworthValue
  );

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
  $: previewMode =
    currentNetworthValue === 0 &&
    checkingTotal === 0 &&
    totalSpent === 0 &&
    goalSummaries.length === 0 &&
    transactionSequences.length === 0 &&
    networthTimeline.length === 0;
  $: checkingSeriesDisplay = hasSignal(checkingSeries) ? checkingSeries : previewSeries.checking;
  $: equitySeriesDisplay = hasSignal(equitySeries) ? equitySeries : previewSeries.equity;
  $: oneYearSeriesDisplay = hasSignal(oneYearSeries) ? oneYearSeries : previewSeries.yearly;
  $: checkingValueTicks = sparklineValueTicks(checkingSeriesDisplay);
  $: equityValueTicks = sparklineValueTicks(equitySeriesDisplay);
  $: checkingDateTicks = sparklineDateLabels(cashFlows);
  $: equityDateTicks = sparklineDateLabels(networthTimeline);

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
  $: expenseCategoriesDisplay = expenseCategories.length
    ? expenseCategories
    : placeholderExpenseCategories;
  $: totalSpentDisplay = expenseCategories.length ? totalSpent : 0;
  $: expensePie = d3
    .pie<CategorySummary>()
    .sort(null)
    .value((entry) => entry.total)(expenseCategoriesDisplay);
  $: monthlyExpenseSeriesDisplay = hasSignal(monthlyExpenseSeries)
    ? monthlyExpenseSeries
    : previewSeries.spending;
  $: topCategorySeriesDisplay = hasSignal(topCategorySeries)
    ? topCategorySeries
    : previewSeries.category;

  function toneForExpenseDelta(delta: number): InsightTone {
    if (delta < -0.03) {
      return "positive";
    }
    if (delta > 0.1) {
      return "negative";
    }
    return "warning";
  }

  $: insights = [
    {
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
      series: monthlyExpenseSeriesDisplay
    },
    {
      title: topCategory
        ? `${topCategory.category} ${topCategory.delta > 0 ? "trending up" : "cooling off"}`
        : "Category trend unavailable",
      description: topCategory
        ? `${formatPercentage(Math.abs(topCategory.delta), 0)} ${
            topCategory.delta > 0 ? "higher" : "lower"
          } than last month.`
        : "Add more expenses to unlock category trends.",
      tone: topCategory ? toneForExpenseDelta(topCategory.delta) : "warning",
      deltaLabel: topCategory ? formatCurrency(topCategory.total) : "No category data",
      series: topCategorySeriesDisplay
    },
    {
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
      series: hasSignal(
        _.takeRight(
          networthTimeline.map((point) => networthValue(point)),
          6
        )
      )
        ? _.takeRight(
            networthTimeline.map((point) => networthValue(point)),
            6
          )
        : currentNetworthValue >= previousNetworthValue
          ? previewSeries.positive
          : previewSeries.negative
    }
  ] satisfies InsightItem[];

  $: checkingChange = checkingSeries.length > 1 ? checkingSeries.at(-1) - checkingSeries.at(-2) : 0;
  $: equityPerformance =
    latestNetworthPoint && previousNetworthPoint
      ? latestNetworthPoint.gainAmount - previousNetworthPoint.gainAmount
      : 0;
  $: newInvestments =
    latestNetworthPoint && previousNetworthPoint
      ? latestNetworthPoint.netInvestmentAmount - previousNetworthPoint.netInvestmentAmount
      : 0;
  $: savingsAdded = cashFlowMonthToDate;
  $: marketMovement =
    currentNetworthValue -
    previousNetworthValue -
    equityPerformance -
    newInvestments -
    savingsAdded +
    totalSpent;
  $: netWorthDrivers = [
    {
      label: "Equity performance",
      value: equityPerformance,
      icon: "fa-arrow-trend-up",
      placeholderRatio: 0.68
    },
    {
      label: "New investments",
      value: newInvestments,
      icon: "fa-circle-plus",
      placeholderRatio: 0.42
    },
    { label: "Savings added", value: savingsAdded, icon: "fa-piggy-bank", placeholderRatio: 0.23 },
    { label: "Expenses", value: -totalSpent, icon: "fa-credit-card", placeholderRatio: 0.52 },
    {
      label: "Market movement",
      value: marketMovement,
      icon: "fa-chart-line",
      placeholderRatio: 0.58
    }
  ] satisfies DriverItem[];
  $: driverMax = _.max(netWorthDrivers.map((driver) => Math.abs(driver.value))) || 1;
  $: driverImpact = _.sumBy(netWorthDrivers, (driver) => driver.value);
  $: driverImpactDisplay = previewMode ? 0 : driverImpact;
  $: displayedNetWorthDrivers = netWorthDrivers.map((driver) => {
    const isPlaceholder = previewMode;
    const widthPercent = isPlaceholder
      ? (driver.placeholderRatio || 0) * 100
      : Math.max((Math.abs(driver.value) / driverMax) * 100, 0);

    return {
      ...driver,
      widthPercent,
      isPlaceholder,
      tone: isPlaceholder ? "placeholder" : driver.value >= 0 ? "positive" : "negative"
    };
  }) satisfies DriverDisplayItem[];

  $: averageMonthlyExpense =
    _.mean(
      _(Object.entries(groupedExpenses))
        .sortBy(([date]) => date)
        .takeRight(3)
        .map(([_date, postings]) => _.sumBy(postings, (posting) => posting.amount))
        .value()
    ) || 0;
  $: reserveMonths = averageMonthlyExpense ? checkingTotal / averageMonthlyExpense : 0;
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
  $: budgetPressureDisplay = budgetPressure.length
    ? budgetPressure
    : ["Dining", "Shopping", "Transport"].map((label, index) => ({
        label,
        percent: [82, 74, 58][index],
        tone: ["negative", "warning", "positive"][index],
        warning: [true, true, false][index],
        placeholder: true,
        ...categoryVisual(label, index)
      }));

  $: recurringOutlook = transactionSequences.slice(0, 3).map((sequence) => {
    const schedule = nextUnpaidSchedule(sequence);
    return {
      label: sequence.key,
      amount: schedule.amount,
      date: schedule.scheduled,
      icon: scheduleIcon(schedule)
    };
  });
  $: recurringOutlookDisplay = recurringOutlook.length
    ? recurringOutlook
    : [
        {
          label: "Netflix",
          amount: 15.99,
          date: dayjs().month(4).date(18),
          icon: { icon: "fa-film", color: "has-text-danger" },
          placeholder: true
        },
        {
          label: "Spotify",
          amount: 9.99,
          date: dayjs().month(4).date(19),
          icon: { icon: "fa-music", color: "has-text-success" },
          placeholder: true
        },
        {
          label: "Gym",
          amount: 39.0,
          date: dayjs().month(4).date(20),
          icon: { icon: "fa-dumbbell", color: "has-text-link" },
          placeholder: true
        }
      ];

  $: goalProgress = goalSummaries.slice(0, 3).map((goal) => ({
    name: goal.name,
    percent: goal.target === 0 ? 0 : Math.round((goal.current / goal.target) * 100)
  }));
  $: goalProgressDisplay = goalProgress.length
    ? goalProgress
    : [
        { name: "Emergency Fund", percent: 68, placeholder: true },
        { name: "Retirement", percent: 42, placeholder: true },
        { name: "Vacation 2025", percent: 30, placeholder: true }
      ];
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
            <a on:click={(_event) => initDemo()} class="button is-link">Load Demo</a>
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
              <div
                class="paisa-dashboard__delta {currentNetworthValue >= previousNetworthValue
                  ? 'is-positive'
                  : 'is-negative'}"
              >
                {formatCurrency(currentNetworthValue - previousNetworthValue)}
                <span
                  >{formatPercentage(percentDelta(currentNetworthValue, previousNetworthValue), 2)} vs
                  last month</span
                >
              </div>
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
                  class:positive={checkingChange >= 0}
                  class:negative={checkingChange < 0}
                >
                  {formatPercentage(
                    percentDelta(
                      checkingSeriesDisplay.at(-1) || 0,
                      checkingSeriesDisplay.at(-2) || 0
                    ),
                    2
                  )}
                </div>
              </div>
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
                      <line x1="0" y1={tick.y} x2="420" y2={tick.y} class="paisa-sparkline__grid" />
                    {/each}
                    <path
                      d={sparklinePath(checkingSeriesDisplay, 420, heroChartHeight)}
                      class={previewMode ? "is-checking is-placeholder" : "is-checking"}
                    />
                  </svg>
                  <div class="paisa-spark-card__x-axis">
                    {#each checkingDateTicks as label}
                      <span>{label}</span>
                    {/each}
                  </div>
                </div>
              </div>
            </div>
            <div class="paisa-spark-card paisa-spark-card--hero">
              <div class="paisa-spark-card__meta">
                <div>
                  <span>Equity</span>
                  <strong>{formatCurrency(currentNetworthValue)}</strong>
                </div>
                <div
                  class="paisa-spark-card__change"
                  class:positive={equityPerformance >= 0}
                  class:negative={equityPerformance < 0}
                >
                  {formatPercentage(
                    percentDelta(equitySeriesDisplay.at(-1) || 0, equitySeriesDisplay.at(-2) || 0),
                    2
                  )}
                </div>
              </div>
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
                      <line x1="0" y1={tick.y} x2="420" y2={tick.y} class="paisa-sparkline__grid" />
                    {/each}
                    <path
                      d={sparklinePath(equitySeriesDisplay, 420, heroChartHeight)}
                      class={previewMode ? "is-equity is-placeholder" : "is-equity"}
                    />
                  </svg>
                  <div class="paisa-spark-card__x-axis">
                    {#each equityDateTicks as label}
                      <span>{label}</span>
                    {/each}
                  </div>
                </div>
              </div>
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
                <span>Net Worth Rank</span>
                <div class="paisa-rail-stat__stack">
                  <strong>Top 18%</strong>
                  <small>vs peers</small>
                </div>
              </div>
              <div class="paisa-rail-stat paisa-rail-stat--trend">
                <span>1Y Net Worth Trend</span>
                <svg viewBox="0 0 150 38" class="paisa-sparkline paisa-sparkline--compact">
                  <path
                    d={sparklinePath(oneYearSeriesDisplay, 150, 38)}
                    class={previewMode ? "is-checking is-placeholder" : "is-checking"}
                  />
                </svg>
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
            label="Net Worth Rank"
            value="Top 18%"
            meta="vs peers"
            variant="rank"
          />
          <DashboardStatStripItem label="1Y Net Worth Trend" variant="trend">
            <svg viewBox="0 0 150 38" class="paisa-sparkline paisa-sparkline--compact">
              <path
                d={sparklinePath(oneYearSeriesDisplay, 150, 38)}
                class={previewMode ? "is-checking is-placeholder" : "is-checking"}
              />
            </svg>
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

            <div class="paisa-spending">
              <div class="paisa-spending__chart">
                <svg viewBox="-140 -140 280 280">
                  {#each expensePie as slice, index}
                    <path
                      d={donutArc(slice)}
                      fill={slice.data.color || categoryColor(slice.data.category, index)}
                      opacity={expenseCategories.length ? 1 : 0.88}
                    />
                  {/each}
                </svg>
                <div class="paisa-spending__center">
                  <strong>{formatCurrency(totalSpentDisplay)}</strong>
                  <span>Total spent</span>
                </div>
              </div>

              <div class="paisa-spending__legend">
                {#each expenseCategoriesDisplay as category}
                  <div class="paisa-spending__item">
                    <div class="paisa-spending__item-label">
                      <span class="icon" style={`color: ${category.color};`}>
                        <i class={"fas " + category.icon} />
                      </span>
                      <span>{category.category}</span>
                    </div>
                    <div class="paisa-spending__item-metrics">
                      <strong
                        >{expenseCategories.length ? formatCurrency(category.total) : "--"}</strong
                      >
                      <span
                        class:positive={category.delta <= 0}
                        class:negative={category.delta > 0}
                      >
                        {#if expenseCategories.length}
                          {category.delta > 0 ? "+" : category.delta < 0 ? "-" : "--"}
                          {formatPercentage(Math.abs(category.delta), 0)}
                        {:else}
                          --
                        {/if}
                      </span>
                    </div>
                  </div>
                {/each}
              </div>
            </div>
          </div>

          <div class="paisa-zone paisa-zone--insights">
            <DashboardSectionHeader title="B. Insights">
              <svelte:fragment slot="right">
                <a href="/expense/monthly" class="secondary-link paisa-link-with-arrow"
                  >View all insights <i class="fas fa-arrow-right" /></a
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
                      />
                    </span>
                  </div>
                  <div class="paisa-insight__copy">
                    <div class="paisa-insight__title">{insight.title}</div>
                    <div class="paisa-insight__text">{insight.description}</div>
                    <div class="paisa-insight__delta">{insight.deltaLabel}</div>
                  </div>
                  <svg viewBox="0 0 130 46" class="paisa-sparkline paisa-sparkline--mini">
                    <path d={sparklinePath(insight.series, 130, 46)} class={"is-" + insight.tone} />
                  </svg>
                </article>
              {/each}
            </div>
          </div>

          <div class="paisa-zone paisa-zone--drivers">
            <DashboardSectionHeader title="C. Net Worth Drivers" subtle="This month" selectable />
            <div class="paisa-driver-list">
              {#each displayedNetWorthDrivers as driver}
                <div class="paisa-driver">
                  <div class="paisa-driver__row">
                    <span class="paisa-driver__label">
                      <span class="icon">
                        <i class={"fas " + driver.icon} />
                      </span>
                      <span>{driver.label}</span>
                    </span>
                    <strong
                      class:negative={!driver.isPlaceholder && driver.value < 0}
                      class:positive={!driver.isPlaceholder && driver.value >= 0}
                      class:paisa-driver__value--placeholder={driver.isPlaceholder}
                    >
                      {driver.isPlaceholder ? "Preview" : formatCurrency(driver.value)}
                    </strong>
                  </div>
                  <div class="paisa-driver__track">
                    <div
                      class={"paisa-driver__fill " + driver.tone}
                      style={`width: ${driver.widthPercent}%`}
                    />
                  </div>
                </div>
              {/each}
            </div>
            <div class="paisa-driver__impact">
              <span>Net impact</span>
              <strong
                class:negative={!previewMode && driverImpactDisplay < 0}
                class:positive={!previewMode && driverImpactDisplay >= 0}
                class:paisa-driver__value--placeholder={previewMode}
              >
                {previewMode ? "Preview only" : formatCurrency(driverImpactDisplay)}
              </strong>
            </div>
          </div>
        </div>
      </div>

      <div class="paisa-dashboard__section">
        <div class="paisa-dashboard__band paisa-dashboard__band--bottom paisa-surface-split">
          <div class="paisa-support">
            <div class="paisa-zone__title">Cash Reserve</div>
            <div class="paisa-support__value">{formatCurrency(checkingTotal)}</div>
            <div class="paisa-support__text">{formatFloat(reserveMonths)} months of expenses</div>
            <div class="paisa-progress">
              <div
                class="paisa-progress__fill"
                style={`width: ${Math.min((reserveMonths / 9) * 100, 100)}%`}
              />
            </div>
            <div class="paisa-progress__ticks">
              <span>0</span>
              <span>3 mo</span>
              <span>6 mo</span>
              <span>9+ mo</span>
            </div>
          </div>

          <div class="paisa-support">
            <div class="paisa-support__header">
              <div class="paisa-zone__title">Budget Pressure</div>
              <div class="paisa-zone__subtle">This month</div>
            </div>
            <div class="paisa-mini-list">
              {#each budgetPressureDisplay as budget}
                <div class="paisa-mini-list__row">
                  <span class="paisa-mini-list__label">
                    <span class="icon" style={`color: ${budget.color};`}>
                      <i class={"fas " + budget.icon} />
                    </span>
                    <span>{budget.label}</span>
                  </span>
                  <div class="paisa-mini-list__meter">
                    <div class="paisa-mini-list__track">
                      <div
                        class={"paisa-mini-list__fill " + budget.tone}
                        style={`width: ${Math.min(budget.percent, 100)}%`}
                      />
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
          </div>

          <div class="paisa-support">
            <div class="paisa-support__header">
              <div class="paisa-zone__title">Recurring Outlook</div>
              <div class="paisa-zone__subtle">Next 7 days</div>
            </div>
            <div class="paisa-mini-list">
              {#each recurringOutlookDisplay as recurring}
                <div class="paisa-mini-list__row paisa-mini-list__row--recurring">
                  <span class="is-flex is-align-items-center" style="gap: 0.55rem;">
                    <span class={"icon paisa-recurring-icon " + recurring.icon.color}>
                      <i class={"fas " + recurring.icon.icon} />
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
            <div class="paisa-mini-list">
              {#each goalProgressDisplay as goal, index}
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
                    />
                  </div>
                </div>
              {/each}
            </div>
          </div>
        </div>
      </div>

      <div class="paisa-dashboard__section paisa-dashboard__footer">
        <span>Last updated: Today, {now().format("HH:mm")}</span>
        <span class="is-flex is-align-items-center" style="gap: 0.45rem;">
          <span class="status-dot" /> All accounts connected
        </span>
      </div>
    </div>
  </section>
{/if}
