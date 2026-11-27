<script lang="ts">
  import BudgetCard from "$lib/components/BudgetCard.svelte";
  import { getBudget } from "$lib/api/cashflow";
  import { formatCurrency, type Budget, helpUrl, now, isMobile } from "$lib/utils";
  import _ from "lodash";
  import { onMount } from "svelte";
  import { month, setAllowedDateRange } from "../../../../store";
  import COLORS from "$lib/colors";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import { t } from "$lib/i18n";

  const monthStart = now().startOf("month");
  let budgetsByMonth = $state<Record<string, Budget>>({});
  let currentMonthAccountBudgets = $derived(budgetsByMonth[$month]?.accounts || []);
  let currentMonthBudget = $derived(budgetsByMonth[$month]);
  let checkingBalance = $state<number>();
  let availableForBudgeting = $state<number>();
  let isEmpty = $state(false);

  onMount(async () => {
    ({ budgetsByMonth, checkingBalance, availableForBudgeting } = await getBudget());
    setAllowedDateRange(
      _.chain(budgetsByMonth)
        .values()
        .flatten()
        .map((b) => b.date)
        .value()
    );

    if (_.isEmpty(budgetsByMonth)) {
      isEmpty = true;
    }
  });
</script>

<section class="section">
  <div class="container is-fluid">
    <div class="columns is-flex-wrap-wrap is-centered">
      {#if currentMonthBudget}
        <div class="column is-12">
          <nav class="level {isMobile() && 'grid-2'}">
            <LevelItem
              title={$t("expense.checkingCurrentBalance")}
              value={formatCurrency(checkingBalance)}
            />
            <LevelItem
              title={availableForBudgeting >= 0
                ? $t("expense.availableForBudgeting")
                : $t("expense.budgetDeficit")}
              color={availableForBudgeting >= 0 ? COLORS.gainText : COLORS.lossText}
              value={formatCurrency(Math.abs(availableForBudgeting))}
            />

            {#if currentMonthBudget.date.isSameOrAfter(monthStart)}
              <LevelItem
                title={$t("expense.availableForSpending")}
                value={formatCurrency(currentMonthBudget.availableThisMonth)}
                subtitle={$t("expense.outOfBudgeted", {
                  amount: formatCurrency(currentMonthBudget.forecast)
                })}
              />

              <LevelItem
                title={$t("expense.projectedMonthEndBalance")}
                value={formatCurrency(currentMonthBudget.endOfMonthBalance)}
              />
            {/if}
          </nav>
        </div>
      {/if}
      <div class="column">
        <div class="is-flex">
          <div style="max-width: 800px; min-width: 350px; width: 100%; margin: auto;">
            <ZeroState item={!isEmpty}>
              <strong>{$t("common.oops")}</strong>
              <!-- eslint-disable-next-line svelte/no-at-html-tags -->
              {@html $t("expense.noBudget", {
                link: `<a href="${helpUrl("budget")}">${$t("common.docs")}</a>`
              })}
            </ZeroState>

            {#each currentMonthAccountBudgets as accountBudget (accountBudget)}
              <BudgetCard {accountBudget} />
            {/each}
          </div>
        </div>
      </div>
    </div>
  </div>
</section>
