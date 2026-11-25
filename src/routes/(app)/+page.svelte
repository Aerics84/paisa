<script lang="ts">
  import * as cashFlow from "$lib/cash_flow";
  import COLORS from "$lib/colors";
  import { getDashboard } from "$lib/api/dashboard";
  import LastNMonths from "$lib/components/LastNMonths.svelte";
  import TransactionCard from "$lib/components/TransactionCard.svelte";
  import * as expense from "$lib/expense/monthly";
  import {
    enrichTrantionSequence,
    nextUnpaidSchedule,
    sortTrantionSequence
  } from "$lib/transaction_sequence";
  import {
    ajax,
    formatCurrency,
    formatCurrencyWithSymbol,
    formatFloat,
    type Budget,
    type CashFlow,
    type Networth,
    type Posting,
    type Transaction,
    type TransactionSequence,
    type Legend,
    now,
    type GoalSummary,
    type AssetBreakdown,
    type ExpensePace
  } from "$lib/utils";
  import _ from "lodash";
  import { onDestroy, onMount } from "svelte";

  import BudgetCard from "$lib/components/BudgetCard.svelte";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import MasonryGrid from "$lib/components/MasonryGrid.svelte";
  import { refresh } from "../../store";
  import UpcomingCard from "$lib/components/UpcomingCard.svelte";
  import GoalSummaryCard from "$lib/components/GoalSummaryCard.svelte";
  import LegendCard from "$lib/components/LegendCard.svelte";
  import BalanceCard from "$lib/components/BalanceCard.svelte";
  import { t } from "$lib/i18n";

  let cashflowLegends = $state<Legend[]>([]);
  let month = $state(now().format("YYYY-MM"));
  let goalSummaries = $state<GoalSummary[]>([]);
  let transactionSequences = $state<TransactionSequence[]>([]);
  let cashFlows = $state<CashFlow[]>([]);
  let expenses = $state<{ [key: string]: Posting[] }>({});
  let xirr = $state(0);
  let networth = $state<Networth>();
  let renderer = $state<((data: Posting[]) => void) | undefined>();
  let totalExpense = $state(0);
  let transactions = $state<Transaction[]>([]);
  let budgetsByMonth = $state<Record<string, Budget>>({});
  let currentBudget = $state<Budget>();
  let expensePace = $state<ExpensePace | null>(null);
  let currentExpensePace = $derived(
    expensePace && expensePace.period === month ? expensePace : null
  );
  let selectedExpenses = $state<Posting[]>([]);
  let isEmpty = $state(false);
  let checkingBalances = $state<Record<string, AssetBreakdown>>({});
  let canInit = $state(false);
  let initReason = $state("");
  let initError = $state("");
  let destroyed = false;

  const NO_EXPENSES: Posting[] = [];

  onDestroy(() => {
    destroyed = true;
  });

  $effect(() => {
    if (renderer) {
      const postings = expenses[month] || NO_EXPENSES;
      selectedExpenses = postings;
      renderer(postings);
      totalExpense = _.sumBy(postings, (p) => p.amount);
    }
  });

  async function initDemo() {
    if (!window.confirm($t("dashboard.onboarding.setupDemoConfirm"))) {
      return;
    }

    initError = "";
    const response = await ajax("/api/init", {
      method: "POST",
      body: JSON.stringify({
        regional_profile: USER_CONFIG?.regional_profile || "india"
      })
    });

    if (!response?.success) {
      initError = response?.error || $t("dashboard.onboarding.setupDemoFailed");
      return;
    }

    await refresh();
  }

  onMount(async () => {
    try {
      const status = await ajax<{ can_init: boolean; reason: string }>("/api/init");
      canInit = status.can_init;
      initReason = status.reason;
    } catch {
      canInit = false;
      initReason = "";
    }

    const dashboard = await getDashboard();
    if (destroyed) {
      return;
    }
    ({
      expenses,
      cashFlows,
      goalSummaries,
      budget: { budgetsByMonth },
      networth: { networth, xirr },
      checkingBalances: { asset_breakdowns: checkingBalances },
      transactions,
      expense_pace: expensePace
    } = dashboard);

    goalSummaries = _.sortBy(goalSummaries, (g) => -g.priority);

    if (_.isEmpty(transactions)) {
      isEmpty = true;
    } else {
      isEmpty = false;
    }

    const postings = _.chain(expenses).values().flatten().value();
    const z = expense.colorScale(postings);
    renderer = expense.renderCurrentExpensesBreakdown(z);
    currentBudget = budgetsByMonth[month];

    const { renderer: cashflowRenderer, legends } = cashFlow.renderMonthlyFlow(
      "#d3-current-cash-flow",
      {
        rotate: false,
        balance: _.last(cashFlows)?.balance || 0
      }
    );
    cashflowRenderer(cashFlows);
    cashflowLegends = legends;
    transactionSequences = _.take(
      _.filter(
        sortTrantionSequence(enrichTrantionSequence(dashboard.transactionSequences)),
        (ts) => !!nextUnpaidSchedule(ts)
      ),
      16
    );
  });
</script>

<div data-testid="dashboard-page">
  <section class="section" class:is-hidden={!isEmpty}>
    <div class="container is-fluid">
      <div class="columns">
        <div class="column is-12">
          <ZeroState item={!isEmpty}>
            <div class="has-text-left" style="max-width: 640px;">
              <p class="mb-2">
                {$t("dashboard.onboarding.intro")}
              </p>
              <div>
                <p class="is-size-4">{$t("dashboard.onboarding.getStarted")}</p>
                <ol class="ml-5 mt-2 mb-4">
                  <li>
                    <!-- eslint-disable-next-line svelte/no-at-html-tags -->
                    {@html $t("dashboard.onboarding.stepConfig", {
                      link: `<a href="/more/config">${$t("dashboard.onboarding.configurationLink")}</a>`
                    })}
                  </li>
                  <li>
                    <!-- eslint-disable-next-line svelte/no-at-html-tags -->
                    {@html $t("dashboard.onboarding.stepEditor", {
                      link: `<a href="/ledger/editor">${$t("dashboard.onboarding.editorLink")}</a>`
                    })}
                  </li>
                </ol>
                <p class="is-size-4">{$t("dashboard.onboarding.viewDemo")}</p>
                <p class="ml-3"></p>
                <ol class="ml-5 mt-2 mb-4">
                  <li>
                    {$t("dashboard.onboarding.demoIntro")}
                  </li>
                  <li>
                    <!-- eslint-disable-next-line svelte/no-at-html-tags -->
                    {@html $t("dashboard.onboarding.demoCleanup", {
                      link: `<a href="/ledger/editor">${$t("dashboard.onboarding.editorLink")}</a>`
                    })}
                  </li>
                  <li>
                    <!-- eslint-disable-next-line svelte/no-at-html-tags -->
                    {@html $t("dashboard.onboarding.demoReset", {
                      link: `<a href="/more/config">${$t("dashboard.onboarding.configurationLink")}</a>`
                    })}
                  </li>
                </ol>

                <button
                  type="button"
                  onclick={(_e) => initDemo()}
                  class="button is-link"
                  disabled={!canInit}
                >
                  {$t("dashboard.onboarding.setupDemo")}
                </button>
                {#if !canInit && initReason}
                  <p class="help is-danger mt-2">
                    {$t("dashboard.onboarding.setupDemoUnavailable", { reason: initReason })}
                  </p>
                {/if}
                {#if initError}
                  <p class="help is-danger mt-2">{initError}</p>
                {/if}
              </div>
            </div>
          </ZeroState>
        </div>
      </div>
    </div>
  </section>

  <section class="section tab-networth" class:is-hidden={isEmpty}>
    <div class="container is-fluid">
      <div class="tile is-ancestor is-align-items-start">
        <div class="tile is-4 is-vertical">
          <div class="tile is-parent">
            <div class="tile is-child">
              <div class="content">
                <p class="subtitle">
                  <a class="secondary-link has-text-grey" href="/assets/networth"
                    >{$t("nav.assets")}</a
                  >
                </p>
                <div class="content">
                  <div>
                    {#if networth}
                      <nav class="level grid-2">
                        <LevelItem
                          narrow
                          title={$t("dashboard.netWorth")}
                          color={COLORS.primary}
                          value={formatCurrencyWithSymbol(networth.balanceAmount)}
                        />

                        <LevelItem
                          narrow
                          title={$t("dashboard.netInvestment")}
                          color={COLORS.secondary}
                          value={formatCurrencyWithSymbol(networth.netInvestmentAmount)}
                        />
                      </nav>
                      <nav class="level grid-2">
                        <LevelItem
                          narrow
                          title={$t("dashboard.gainLoss")}
                          color={networth.gainAmount >= 0 ? COLORS.gainText : COLORS.lossText}
                          value={formatCurrencyWithSymbol(networth.gainAmount)}
                        />

                        <LevelItem narrow title="XIRR" value={formatFloat(xirr)} />
                      </nav>
                    {/if}
                  </div>
                </div>
              </div>
            </div>
          </div>

          {#if !_.isEmpty(checkingBalances)}
            <div class="tile is-parent">
              <article class="tile is-child">
                <div class="content">
                  <p class="subtitle">
                    <a class="secondary-link has-text-grey" href="/assets/balance"
                      >{$t("dashboard.checkingBalance")}</a
                    >
                  </p>
                  <div class="content">
                    <MasonryGrid gap={10} maxStretchColumnSize={400} align="stretch">
                      {#each _.values(checkingBalances) as assetBreakdown}
                        <div class="is-flex-grow-1">
                          <BalanceCard {assetBreakdown} />
                        </div>
                      {/each}
                    </MasonryGrid>
                  </div>
                </div>
              </article>
            </div>
          {/if}

          <div class="tile is-parent">
            <article class="tile is-child min-w-0">
              <p class="subtitle">
                <a class="secondary-link has-text-grey" href="/cash_flow/monthly"
                  >{$t("nav.cashFlow")}</a
                >
              </p>
              <div class="content box px-2 pb-0">
                <ZeroState item={cashFlows}>
                  <strong>{$t("common.oops")}</strong>
                  {$t("dashboard.cashFlowEmpty")}
                </ZeroState>

                <LegendCard legends={cashflowLegends} clazz="mb-2 overflow-x-auto" />

                <svg
                  class:is-not-visible={_.isEmpty(cashFlows)}
                  id="d3-current-cash-flow"
                  height="250"
                  width="100%"
                />
              </div>
            </article>
          </div>
          {#if currentBudget}
            <div class="tile is-parent">
              <div class="tile is-child">
                <div class="content">
                  <p class="subtitle">
                    <a class="secondary-link has-text-grey" href="/expense/budget"
                      >{$t("nav.budget")}</a
                    >
                  </p>
                  <div class="content">
                    <div>
                      {#each currentBudget.accounts as accountBudget (accountBudget)}
                        <BudgetCard compact {accountBudget} />
                      {/each}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          {/if}
          {#if currentExpensePace}
            <div class="tile is-parent">
              <div class="tile is-child">
                <div class="content">
                  <p class="subtitle">
                    <a class="secondary-link has-text-grey" href="/expense/budget"
                      >{$t("dashboard.projectedMonthEnd")}</a
                    >
                  </p>
                  <div class="content box px-3 py-2">
                    <span class="has-text-weight-bold"
                      >{formatCurrencyWithSymbol(currentExpensePace.projectedExpenses)}</span
                    >
                    {#if currentExpensePace.overBudget != null}
                      <span class="tag is-danger is-light ml-2"
                        >{$t("dashboard.overBudget", {
                          amount: formatCurrencyWithSymbol(currentExpensePace.overBudget)
                        })}</span
                      >
                    {/if}
                  </div>
                </div>
              </div>
            </div>
          {/if}
          {#if !_.isEmpty(goalSummaries)}
            <div class="tile">
              <div class="tile is-parent is-12">
                <article class="tile is-child">
                  <div class="content">
                    <p class="subtitle">
                      <a class="secondary-link has-text-grey" href="/more/goals"
                        >{$t("nav.goals")}</a
                      >
                    </p>
                    <div class="content">
                      {#each goalSummaries as goal}
                        <GoalSummaryCard {goal} small />
                      {/each}
                    </div>
                  </div>
                </article>
              </div>
            </div>
          {/if}
        </div>
        <div class="tile is-vertical">
          <div class="tile is-parent is-12">
            <article class="tile is-child">
              <p class="subtitle is-flex is-justify-content-space-between is-align-items-end">
                <span
                  ><a class="secondary-link has-text-grey" href="/expense/monthly"
                    >{$t("nav.expenses")}</a
                  >
                  <span class="is-size-5 has-text-weight-bold px-2" style="color: {COLORS.expenses}"
                    >{formatCurrency(totalExpense)}</span
                  ></span
                >
                <LastNMonths n={3} bind:value={month} />
              </p>
              <div class="content box px-3">
                <ZeroState item={selectedExpenses}>
                  <strong>{$t("common.hurray")}</strong>
                  {$t("dashboard.expensesEmpty")}
                </ZeroState>
                <svg id="d3-current-month-breakdown" width="100%" />
              </div>
            </article>
          </div>
          {#if !_.isEmpty(transactionSequences)}
            <div class="tile">
              <div class="tile is-parent is-12">
                <article class="tile is-child">
                  <div class="content">
                    <p class="subtitle">
                      <a class="secondary-link has-text-grey" href="/cash_flow/recurring"
                        >{$t("nav.recurring")}</a
                      >
                    </p>
                    <div class="content box">
                      <div
                        class="grid grid-rows-1 overflow-hidden"
                        style="grid-auto-rows: 0px; grid-template-columns: repeat(auto-fit, minmax(130px, 150px));"
                      >
                        {#each transactionSequences as ts (ts)}
                          <UpcomingCard transactionSequece={ts} />
                        {/each}
                      </div>
                    </div>
                  </div>
                </article>
              </div>
            </div>
          {/if}
          {#if !_.isEmpty(transactions)}
            <div class="tile">
              <div class="tile is-parent is-12">
                <article class="tile is-child">
                  <div class="content">
                    <p class="subtitle">
                      <a class="secondary-link has-text-grey" href="/ledger/transaction"
                        >{$t("dashboard.recentTransactions")}</a
                      >
                    </p>
                    <div>
                      <MasonryGrid gap={10} maxStretchColumnSize={500} align="stretch">
                        {#each _.take(transactions, 20) as t}
                          <div class="mr-3 is-flex-grow-1">
                            <TransactionCard {t} />
                          </div>
                        {/each}
                      </MasonryGrid>
                    </div>
                  </div>
                </article>
              </div>
            </div>
          {/if}
        </div>
      </div>
    </div>
  </section>
</div>

<style lang="scss">
  p.subtitle {
    margin-bottom: 0.5rem !important;
  }

  p.subtitle a.secondary-link {
    text-transform: uppercase;
    font-size: 1rem;
  }
</style>
