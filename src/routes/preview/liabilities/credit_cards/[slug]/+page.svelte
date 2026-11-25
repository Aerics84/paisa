<script lang="ts">
  import { goto } from "$app/navigation";
  import COLORS from "$lib/colors";
  import BoxLabel from "$lib/components/BoxLabel.svelte";
  import CreditCardCard from "$lib/components/CreditCardCard.svelte";
  import DueDate from "$lib/components/DueDate.svelte";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import TransactionCard from "$lib/components/TransactionCard.svelte";
  import { renderYearlySpends } from "$lib/credit_cards";
  import { getCreditCard } from "$lib/api/credit_cards";
  import { iconify } from "$lib/icon";
  import {
    formatCurrency,
    formatPercentage,
    type CreditCardBill,
    type CreditCardSummary
  } from "$lib/utils";
  import MasonryGrid from "$lib/components/MasonryGrid.svelte";
  import _, { now } from "lodash";
  import { onMount } from "svelte";
  import type { PageData } from "./$types";
  import { t } from "$lib/i18n";

  let { data }: { data: PageData } = $props();
  let svg = $state<SVGElement>();

  let creditCard = $state<CreditCardSummary>();
  let currentBill = $state<CreditCardBill>();
  let small = true;
  let rendered = $state(false);

  function lastBill(creditCard: CreditCardSummary): CreditCardBill {
    return _.find(_.reverse(_.clone(creditCard.bills)), (b) => {
      return b.statementEndDate.isSameOrBefore(now());
    });
  }

  $effect(() => {
    if (creditCard && svg && !rendered) {
      renderYearlySpends(svg, creditCard.yearlySpends);
      rendered = true;
    }
  });

  onMount(async () => {
    const response = await getCreditCard(data.account);
    if (!response.found || !response.creditCard) {
      return goto("/liabilities/credit_cards");
    }

    creditCard = response.creditCard;
    currentBill = lastBill(creditCard);
  });
</script>

<section class="section">
  <div class="container is-fluid">
    <div class="columns flex-wrap">
      <div class="column is-3-widescreen is-4">
        {#if creditCard}
          <div class="flex mb-12">
            <CreditCardCard {creditCard} />
          </div>

          <nav class="level grid-2">
            <LevelItem
              narrow
              small
              title={$t("liabilities.availableCredit")}
              color={COLORS.neutral}
              value={formatCurrency(Math.max(creditCard.creditLimit - creditCard.balance, 0))}
            />

            <LevelItem
              narrow
              small
              title={$t("liabilities.creditUsage")}
              color={COLORS.neutral}
              value={formatPercentage(creditCard.balance / creditCard.creditLimit, 2)}
            />
          </nav>

          <nav class="level grid-2">
            <LevelItem
              narrow
              small
              title={$t("liabilities.statementCount")}
              color={COLORS.neutral}
              value={creditCard.bills.length.toString()}
            />
            <LevelItem
              narrow
              small
              title={$t("liabilities.transactionCount")}
              color={COLORS.neutral}
              value={_.sumBy(creditCard.bills, (b) => b.transactions.length).toString()}
            />
          </nav>

          <div class="box px-3 py-0">
            <svg bind:this={svg} width="100%" />
          </div>
          <BoxLabel text={$t("liabilities.yearWiseSpends")} />
        {/if}
      </div>
      <div class="column is-9-widescreen is-8">
        {#if currentBill}
          <div class="flex flex-wrap gap-4 mb-4">
            <div
              class="box py-2 m-0 flex-grow overflow-x-auto"
              style="border: 1px solid transparent"
            >
              <div class="is-flex mr-2 is-align-items-baseline" style="min-width: fit-content">
                <div class="ml-3 custom-icon is-size-5 whitespace-nowrap">
                  <span>{iconify(creditCard.account)}</span>
                </div>
                <div class="ml-3 whitespace-nowrap">
                  <span class="mr-1 is-size-7 has-text-grey">{$t("liabilities.payment")}</span>
                  <span>
                    <DueDate
                      dueDate={currentBill.dueDate}
                      paidDate={currentBill.paidDate}
                      amountDue={currentBill.closingBalance}
                    />
                  </span>
                </div>
              </div>
            </div>
            <div class="has-text-right">
              <div class="select is-medium">
                <select bind:value={currentBill}>
                  {#each _.reverse(_.clone(creditCard.bills)) as bill}
                    <option value={bill}
                      >{bill.statementStartDate.format("DD MMM YYYY")} — {bill.statementEndDate.format(
                        "DD MMM YYYY"
                      )}</option
                    >
                  {/each}
                </select>
              </div>
            </div>
          </div>
          <nav class="level flex gap-4 overflow-x-auto" style="justify-content: start;">
            <LevelItem
              {small}
              narrow
              title={$t("liabilities.openingBalance")}
              color={COLORS.neutral}
              value={formatCurrency(currentBill.openingBalance)}
            />
            <div class="level-item is-narrow">
              <span class="icon is-size-3">
                <i class="fas fa-plus"></i>
              </span>
            </div>
            <LevelItem
              {small}
              narrow
              title={$t("liabilities.debits")}
              color={COLORS.expenses}
              value={formatCurrency(currentBill.debits)}
            />
            <div class="level-item is-narrow">
              <span class="icon is-size-3">
                <i class="fas fa-minus"></i>
              </span>
            </div>
            <LevelItem
              {small}
              narrow
              title={$t("liabilities.credits")}
              color={COLORS.income}
              value={formatCurrency(currentBill.credits)}
            />
            <div class="level-item is-narrow">
              <span class="icon is-size-3">
                <i class="fas fa-equals"></i>
              </span>
            </div>
            <LevelItem
              {small}
              narrow
              title={$t("liabilities.amountDue")}
              color={COLORS.liabilities}
              value={formatCurrency(currentBill.closingBalance)}
            />
          </nav>

          <div>
            <MasonryGrid gap={10} maxStretchColumnSize={500} align="stretch">
              {#each currentBill.transactions as transaction}
                <div class="mr-3 is-flex-grow-1">
                  <TransactionCard t={transaction} />
                </div>
              {/each}
            </MasonryGrid>
          </div>
        {/if}
      </div>
    </div>
  </div>
</section>
