<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import _ from "lodash";
  import { getExpense } from "$lib/api/cashflow";
  import { type Posting, formatCurrency, formatPercentage, type Legend } from "$lib/utils";
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
    normalizeExpenseDetailScope,
    ROOT_EXPENSE_SCOPE
  } from "$lib/expense";
  import { t } from "$lib/i18n";

  let detailScope = writable(ROOT_EXPENSE_SCOPE);
  let timelineGroup = writable<string | null>(null);
  let z = $state<d3.ScaleOrdinal<string, string, never>>();
  let renderer = $state<((ps: Posting[], scope?: string) => void) | undefined>();
  let expenses = $state<Posting[]>();
  let grouped_expenses = $state<Record<string, Posting[]>>();
  let grouped_incomes = $state<Record<string, Posting[]>>();
  let grouped_investments = $state<Record<string, Posting[]>>();
  let grouped_taxes = $state<Record<string, Posting[]>>();
  let destroy = $state<(() => void) | undefined>();

  let legends = $state<Legend[]>([]);

  let taxRate = $state("");
  let netIncome = $state("");
  let tax = $state("");
  let expenseRate = $state("");
  let expense = $state("");
  let saving = $state("");
  let savingRate = $state("");
  let income = $state("");

  let current_month_expenses = $state<Posting[]>([]);
  let breadcrumbs = $derived(expenseBreadcrumb($detailScope));
  let monthHasExpenses = $derived((grouped_expenses?.[$month] || []).length > 0);
  let detailScopeLabel = $derived(_.last(breadcrumbs)?.label || ROOT_EXPENSE_SCOPE);

  $effect(() => {
    if (grouped_expenses && z) {
      current_month_expenses = _.chain(
        filterExpenseScope(grouped_expenses?.[$month] || [], $detailScope)
      )
        .sortBy((e) => e.date)
        .reverse()
        .value();
    }
  });

  $effect(() => {
    if (grouped_expenses && renderer) {
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
        netIncome = $t("expense.netIncome", {
          amount: formatCurrency(sum(incomes, -1) - sum(taxes))
        });
        taxRate = $t("expense.taxRateMonthly", {
          percentage: formatPercentage(sum(taxes) / sum(incomes, -1))
        });
        expenseRate = $t("expense.ofNetIncome", {
          percentage: formatPercentage(sum(expenses) / (sum(incomes, -1) - sum(taxes)))
        });
        savingRate = $t("expense.ofNetIncome", {
          percentage: formatPercentage(sum(investments) / (sum(incomes, -1) - sum(taxes)))
        });
      }

      renderer(expenses, $detailScope);
    }
  });

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
    } = await getExpense());

    setAllowedDateRange(_.map(expenses, (e) => e.date));
    ({ z, destroy, legends } = renderMonthlyExpensesTimeline(
      expenses,
      timelineGroup,
      month,
      dateRange
    ));
    renderer = renderCurrentExpensesBreakdown(z, {
      onDrilldown: (scope) => detailScope.set(normalizeExpenseDetailScope(scope))
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
                  title={$t("expense.grossIncome")}
                  value={income}
                  color={COLORS.gainText}
                  subtitle={netIncome}
                />
                <LevelItem
                  narrow
                  title={$t("expense.tax")}
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
                  title={$t("expense.netInvestment")}
                  value={saving}
                  subtitle={savingRate}
                  color={COLORS.secondary}
                />

                <LevelItem
                  narrow
                  title={$t("expense.expenses")}
                  value={expense}
                  color={COLORS.lossText}
                  subtitle={expenseRate}
                />
              </nav>
            </div>
          </div>
          <div class="column is-full">
            <nav
              class="breadcrumb has-chevron-separator mb-2 is-small"
              aria-label={$t("expense.scopeLabel")}
            >
              <ul>
                {#each breadcrumbs as crumb}
                  <li>
                    {#if crumb.scope === $detailScope}
                      <span class="is-inactive">{crumb.label}</span>
                    {:else}
                      <a
                        href={crumb.scope}
                        onclick={(event) => {
                          event.preventDefault();
                          detailScope.set(crumb.scope);
                        }}>{crumb.label}</a
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
                <div class="days"></div>
              </div>
            </div>
          </div>
          <div class="column is-8">
            <div class="px-3 box current-month-breakdown-box" style="height: 100%">
              {#if $detailScope !== ROOT_EXPENSE_SCOPE}
                <button
                  type="button"
                  class="button is-ghost is-small current-month-breakdown-reset"
                  onclick={() => detailScope.set(ROOT_EXPENSE_SCOPE)}
                  >{$t("expense.resetScope")}</button
                >
              {/if}
              <ZeroState item={current_month_expenses}>
                {#if monthHasExpenses && $detailScope !== ROOT_EXPENSE_SCOPE}
                  <strong
                    >{$t("expense.noExpensesForScope", {
                      scope: detailScopeLabel,
                      month: dayjs($month).format("MMM YYYY")
                    })}</strong
                  >
                {:else}
                  <strong>{$t("common.hurray")}</strong>
                  {$t("expense.noExpensesThisMonth")}
                {/if}
              </ZeroState>
              <svg id="d3-current-month-breakdown" width="100%" />
            </div>
          </div>
          <div class="column is-full">
            <div class="box">
              <ZeroState item={expenses}>
                <strong>{$t("common.oops")}</strong>
                {$t("expense.noExpenses")}
              </ZeroState>
              <div class="px-4 pb-1 is-size-7 has-text-grey">
                {$t("expense.trendFilterHint")}
              </div>
              <LegendCard {legends} clazz="ml-4 overflow-x-auto" />
              <svg id="d3-monthly-expense-timeline" width="100%" height="400" />
            </div>
          </div>
        </div>
        <BoxLabel text={$t("expense.monthlyExpenses")} />
      </div>
    </div>
  </div>
</section>

<style lang="scss">
  .current-month-breakdown-box {
    position: relative;
  }

  .current-month-breakdown-reset {
    position: absolute;
    top: 0.5rem;
    left: 0.5rem;
    z-index: 1;
    color: var(--bulma-link-text, inherit);
  }
</style>
