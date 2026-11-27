<script lang="ts">
  import { t } from "$lib/i18n";
  import type { Transaction } from "$lib/api/ledger";
  import DashboardSectionHeader from "./dashboard/DashboardSectionHeader.svelte";
  import TransactionCard from "./TransactionCard.svelte";

  interface Props {
    transactions?: Transaction[];
  }

  let { transactions = [] }: Props = $props();
</script>

{#if transactions.length > 0}
  <section class="paisa-dashboard__section" data-testid="preview-section-recent-transactions">
    <DashboardSectionHeader title={$t("dashboard.recentTransactions")}>
      {#snippet right()}
        <a href="/preview/ledger/transaction" class="secondary-link paisa-link-with-arrow"
          >{$t("dashboard.viewAllTransactions")} <i class="fas fa-arrow-right"></i></a
        >
      {/snippet}
    </DashboardSectionHeader>

    <div class="paisa-recent-grid">
      {#each transactions as t (t.id)}
        <TransactionCard {t} linkPrefix="/preview" />
      {/each}
    </div>
  </section>
{/if}
