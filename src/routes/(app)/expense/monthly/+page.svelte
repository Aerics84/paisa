<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import _ from "lodash";
  import { ajax, type Posting, formatCurrency, formatPercentage, type Legend } from "$lib/utils";
  import {
    renderMonthlyExpensesTimeline,
    renderCurrentExpensesBreakdown,
    renderCalendar
  } from "$lib/expense/monthly";
  import { dateRange, month, setAllowedDateRange } from "../../../../store";
  import { writable } from "svelte/store";
  import PostingCard from "$lib/components/PostingCard.svelte";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import COLORS from "$lib/colors";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import dayjs from "dayjs";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import {
    expenseBreadcrumb,
    expenseColorKey,
    filterExpenseScope,
    ROOT_EXPENSE_SCOPE
  } from "$lib/expense";

  let detailScope = writable(ROOT_EXPENSE_SCOPE);
  let timelineGroup = writable<string | null>(null);
  let z: d3.ScaleOrdinal<string, string, never>,
    renderer: (ps: Posting[], scope?: string) => void,
    expenses: Posting[],
    grouped_expenses: Record<string, Posting[]>,
    grouped_incomes: Record<string, Posting[]>,
    grouped_investments: Record<string, Posting[]>,
    grouped_taxes: Record<string, Posting[]>,
    destroy: () => void;

  let legends: Legend[] = [];

  let taxRate = "",
    netIncome = "",
    tax = "",
    expenseRate = "",
    expense = "",
    saving = "",
    savingRate = "",
    income = "";

  let current_month_expenses: Posting[] = [];
  let breadcrumbs = expenseBreadcrumb(ROOT_EXPENSE_SCOPE);
  let monthHasExpenses = false;
  let detailScopeLabel = "Expenses";

  $: {
    current_month_expenses = _.chain(
      filterExpenseScope(grouped_expenses?.[$month] || [], $detailScope)
    )
      .sortBy((e) => e.date)
      .reverse()
      .value();
  }

  $: if (grouped_expenses) {
    renderCalendar($month, grouped_expenses[$month] || [], z, $detailScope);

    const expenses = filterExpenseScope(grouped_expenses[$month] || [], $detailScope);
    const incomes = grouped_incomes[$month] || [];
    const taxes = grouped_taxes[$month] || [];
    const investments = grouped_investments[$month] || [];

    income = sumCurrency(incomes, -1);
    tax = sumCurrency(taxes);
    expense = sumCurrency(expenses);
    saving = sumCurrency(investments);

    if (_.isEmpty(incomes)) {
      taxRate = "";
      expenseRate = "";
      savingRate = "";
      netIncome = "";
    } else {
      netIncome = formatCurrency(sum(incomes, -1) - sum(taxes)) + " net income";
      taxRate = formatPercentage(sum(taxes) / sum(incomes, -1)) + " on income";
      expenseRate =
        formatPercentage(sum(expenses) / (sum(incomes, -1) - sum(taxes))) + " of net income";
      savingRate =
        formatPercentage(sum(investments) / (sum(incomes, -1) - sum(taxes))) + " of net income";
    }

    renderer(expenses, $detailScope);
  }

  $: breadcrumbs = expenseBreadcrumb($detailScope);
  $: monthHasExpenses = (grouped_expenses?.[$month] || []).length > 0;
  $: detailScopeLabel = _.last(breadcrumbs)?.label || ROOT_EXPENSE_SCOPE;

  onDestroy(async () => {
    if (destroy) {
      destroy();
    }
  });

  onMount(async () => {
    ({
      expenses: expenses,
      month_wise: {
        expenses: grouped_expenses,
        incomes: grouped_incomes,
        investments: grouped_investments,
        taxes: grouped_taxes
      }
    } = await ajax("/api/expense"));

    setAllowedDateRange(_.map(expenses, (e) => e.date));
    ({ z, destroy, legends } = renderMonthlyExpensesTimeline(
      expenses,
      timelineGroup,
      month,
      dateRange
    ));
    renderer = renderCurrentExpensesBreakdown(z, {
      onDrilldown: (scope) => detailScope.set(scope)
    });
  });

  function sum(postings: Posting[], sign = 1) {
    return sign * _.sumBy(postings, (p) => p.amount);
  }

  function sumCurrency(postings: Posting[], sign = 1) {
    return formatCurrency(sign * _.sumBy(postings, (p) => p.amount));
  }
</script>

<section class="section tab-expense">
  <div class="container is-fluid">
    <div class="columns is-flex-wrap-wrap">
      <div class="column is-3">
        <div class="columns is-flex-wrap-wrap">
          <div class="column is-full">
            <div>
              <nav class="level grid-2">
                <LevelItem
                  narrow
                  title="Gross Income"
                  value={income}
                  color={COLORS.gainText}
                  subtitle={netIncome}
                />
                <LevelItem
                  narrow
                  title="Tax"
                  value={tax}
                  subtitle={taxRate}
                  color={COLORS.lossText}
                />
              </nav>
            </div>
          </div>
          <div class="column is-full">
            <div>
              <nav class="level grid-2">
                <LevelItem
                  narrow
                  title="Net Investment"
                  value={saving}
                  subtitle={savingRate}
                  color={COLORS.secondary}
                />

                <LevelItem
                  narrow
                  title="Expenses"
                  value={expense}
                  color={COLORS.lossText}
                  subtitle={expenseRate}
                />
              </nav>
            </div>
          </div>
          <div class="column is-full">
            <nav class="breadcrumb has-chevron-separator mb-2 is-small" aria-label="expense scope">
              <ul>
                {#each breadcrumbs as crumb}
                  <li>
                    {#if crumb.scope === $detailScope}
                      <a class="is-inactive">{crumb.label}</a>
                    {:else}
                      <a
                        href={crumb.scope}
                        on:click|preventDefault={() => detailScope.set(crumb.scope)}
                        >{crumb.label}</a
                      >
                    {/if}
                  </li>
                {/each}
              </ul>
            </nav>
            {#each current_month_expenses as expense}
              <PostingCard
                posting={expense}
                color={z(expenseColorKey(expense, $detailScope))}
                icon={true}
              />
            {/each}
          </div>
        </div>
      </div>
      <div class="column is-9">
        <div class="columns is-flex-wrap-wrap">
          <div class="column is-4">
            <div class="p-3 box">
              <div id="d3-current-month-expense-calendar" class="d3-calendar">
                <div class="weekdays">
                  {#each dayjs.weekdaysShort(true) as day}
                    <div>{day}</div>
                  {/each}
                </div>
                <div class="days" />
              </div>
            </div>
          </div>
          <div class="column is-8">
            <div class="px-3 box" style="height: 100%">
              <ZeroState item={current_month_expenses}>
                {#if monthHasExpenses && $detailScope !== ROOT_EXPENSE_SCOPE}
                  <strong>No expenses for {detailScopeLabel}</strong> in {dayjs($month).format(
                    "MMM YYYY"
                  )}.
                {:else}
                  <strong>Hurray!</strong> You have no expenses this month.
                {/if}
              </ZeroState>
              <svg id="d3-current-month-breakdown" width="100%" />
            </div>
          </div>
          <div class="column is-full">
            <div class="box">
              <ZeroState item={expenses}>
                <strong>Oops!</strong> You have no expenses.
              </ZeroState>
              <div class="px-4 pb-1 is-size-7 has-text-grey">
                Trend filter only affects this 12-month chart.
              </div>
              <LegendCard {legends} clazz="ml-4 overflow-x-auto" />
              <svg id="d3-monthly-expense-timeline" width="100%" height="400" />
            </div>
          </div>
        </div>
        <BoxLabel text="Monthly Expenses" />
      </div>
    </div>
  </div>
</section>
