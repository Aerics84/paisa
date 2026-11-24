<script lang="ts">
  import { shouldShowNoDues } from "$lib/due_date";
  import { dueDateIcon } from "$lib/utils";
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
  <span>No dues</span>
{:else}
  <span title="due on {dueDate.format('DD MMM YYYY')}">
    <span class="icon is-small {icon.color}">
      <i class="fas {icon.icon}"></i>
    </span>
    {#if paidDate}
      <span>paid on {paidDate.format("DD MMM YYYY")}</span>
    {:else}
      <span>due {dueDate.fromNow()}</span>
    {/if}
  </span>
{/if}
