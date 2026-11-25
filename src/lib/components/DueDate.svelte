<script lang="ts">
  import { shouldShowNoDues } from "$lib/due_date";
  import { dueDateIcon } from "$lib/utils";
  import { t } from "$lib/i18n";
  import dayjs from "dayjs";

  interface Props {
    dueDate: dayjs.Dayjs;
    paidDate: dayjs.Dayjs;
    amountDue: number;
  }

  let { dueDate, paidDate, amountDue }: Props = $props();

  let icon = $derived(dueDateIcon(dueDate, paidDate));
</script>

{#if shouldShowNoDues(paidDate, amountDue)}
  <span>{$t("liabilities.noDues")}</span>
{:else}
  <span title={$t("liabilities.dueOn", { date: dueDate.format("DD MMM YYYY") })}>
    <span class="icon is-small {icon.color}">
      <i class="fas {icon.icon}"></i>
    </span>
    {#if paidDate}
      <span>{$t("liabilities.paidOn", { date: paidDate.format("DD MMM YYYY") })}</span>
    {:else}
      <span>{$t("liabilities.due", { relative: dueDate.fromNow() })}</span>
    {/if}
  </span>
{/if}
