<script lang="ts">
  import * as d3 from "d3";
  import { onMount } from "svelte";
  import _ from "lodash";
  import { getExpense } from "$lib/api/cashflow";
  import { formatCurrency, formatPercentage, type Legend, type Posting } from "$lib/utils";
  import {
    renderYearlyExpensesTimeline,
    renderCurrentExpensesBreakdown,
    renderCalendar
  } from "$lib/expense/yearly";
  import { dateMin, dateMax, year } from "../../../../store";
  import { writable } from "svelte/store";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import COLORS from "$lib/colors";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import { t } from "$lib/i18n";

  let groups = writable([]);
  let z = $state<d3.ScaleOrdinal<string, string, never>>();
  let renderer = $state<((ps: Posting[]) => void) | undefined>();
  let expenses = $state<Posting[]>();
  let grouped_expenses = $state<Record<string, Posting[]>>();
  let grouped_incomes = $state<Record<string, Posting[]>>();
  let grouped_investments = $state<Record<string, Posting[]>>();
  let grouped_taxes = $state<Record<string, Posting[]>>();

  let currentYearExpenses = $state<Posting[]>([]);

  let legends = $state<Legend[]>([]);

  let income = $state("");
  let netIncome = $state("");
  let taxRate = $state("");
  let tax = $state("");
  let expenseRate = $state("");
  let expense = $state("");
  let investment = $state("");
  let savingRate = $state("");

  $effect(() => {
    if (grouped_expenses && renderer) {
      currentYearExpenses = grouped_expenses[$year];
      renderCalendar(currentYearExpenses, z, $groups);

      const expenses = grouped_expenses[$year] || [];
      const incomes = grouped_incomes[$year] || [];
      const taxes = grouped_taxes[$year] || [];
      const investments = grouped_investments[$year] || [];

      income = sumCurrency(incomes, -1);

      tax = sumCurrency(taxes);
      expense = sumCurrency(expenses);
      investment = sumCurrency(investments);

      if (_.isEmpty(incomes)) {
        expenseRate = "";
        taxRate = "";
        savingRate = "";
        netIncome = "";
      } else {
        netIncome = $t("expense.netIncome", {
          amount: formatCurrency(sum(incomes, -1) - sum(taxes))
        });
        taxRate = $t("expense.taxRateYearly", {
          percentage: formatPercentage(sum(taxes) / sum(incomes, -1))
        });
        expenseRate = $t("expense.ofNetIncome", {
          percentage: formatPercentage(sum(expenses) / (sum(incomes, -1) - sum(taxes)))
        });
        savingRate = $t("expense.ofNetIncome", {
          percentage: formatPercentage(sum(investments) / (sum(incomes, -1) - sum(taxes)))
        });
      }

      renderer(expenses);
    }
  });

  onMount(async () => {
    ({
      expenses: expenses,
      year_wise: {
        expenses: grouped_expenses,
        incomes: grouped_incomes,
        investments: grouped_investments,
        taxes: grouped_taxes
      }
    } = await getExpense());

    const [start, end] = d3.extent(_.map(expenses, (e) => e.date));
    if (start) {
      dateMin.set(start);
      dateMax.set(end);
    }

    ({ z, legends } = renderYearlyExpensesTimeline(expenses, groups, year));

    renderer = renderCurrentExpensesBreakdown(z);
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
                  title={$t("expense.grossIncome")}
                  value={income}
                  color={COLORS.gainText}
                  subtitle={netIncome}
                />
                <LevelItem
                  title={$t("expense.tax")}
                  value={tax}
                  color={COLORS.lossText}
                  subtitle={taxRate}
                />
              </nav>
            </div>
          </div>
          <div class="column is-full">
            <div>
              <nav class="level grid-2">
                <LevelItem
                  title={$t("expense.netInvestment")}
                  value={investment}
                  color={COLORS.secondary}
                  subtitle={savingRate}
                />

                <LevelItem
                  title={$t("expense.expenses")}
                  value={expense}
                  color={COLORS.lossText}
                  subtitle={expenseRate}
                />
              </nav>
            </div>
          </div>
        </div>
      </div>
      <div class="column is-3">
        <div class="px-3 box">
          <div id="d3-current-year-expense-calendar" class="d3-calendar">
            <div class="months"></div>
          </div>
        </div>
      </div>
      <div class="column is-full-tablet is-half-fullhd">
        <div class="px-3 box" style="height: 100%">
          <ZeroState item={currentYearExpenses}>
            <strong>{$t("common.hurray")}</strong>
            {$t("expense.noExpensesThisYear")}
          </ZeroState>
          <svg id="d3-current-year-breakdown" width="100%" />
        </div>
      </div>
      <div class="column is-12">
        <div class="box">
          <ZeroState item={expenses}>
            <strong>{$t("common.oops")}</strong>
            {$t("expense.noExpenses")}
          </ZeroState>

          <LegendCard {legends} clazz="ml-4" />
          <svg id="d3-yearly-expense-timeline" width="100%" height="500" />
        </div>
      </div>
    </div>
    <BoxLabel text={$t("expense.yearlyExpenses")} />
  </div>
</section>
