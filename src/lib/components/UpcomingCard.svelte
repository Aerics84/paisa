<script lang="ts">
  import {
    intervalText,
    nextUnpaidSchedule,
    scheduleIcon,
    totalRecurring
  } from "$lib/transaction_sequence";
  import {
    formatCurrencyCrude,
    now,
    type TransactionSchedule,
    type TransactionSequence
  } from "$lib/utils";

  interface Props {
    transactionSequece: TransactionSequence;
  }

  let { transactionSequece }: Props = $props();

  let schedule = $derived(nextUnpaidSchedule(transactionSequece));

  function progress(scheduled: TransactionSchedule["scheduled"]) {
    if (scheduled.isBefore(now()) || !transactionSequece.interval) {
      return 0;
    }

    const percent = (scheduled.diff(now(), "day") / transactionSequece.interval) * 100;
    return Math.max(0, Math.min(100, percent));
  }
</script>

{#if schedule}
  {@const n = schedule.scheduled}
  {@const icon = scheduleIcon(schedule)}
  {@const percent = progress(n)}
  <div class="has-text-centered mb-0 mr-3 max-w-[200px]">
    <div class="is-size-7 truncate">{transactionSequece.key}</div>
    <div class="my-1">
      <span class="tag">{intervalText(transactionSequece)}</span>
    </div>
    <div class="has-text-grey is-size-7">
      <span class="icon has-text-grey-light">
        <i class="fas fa-calendar"></i>
      </span>
      {schedule.scheduled.format("DD MMM YYYY")}
    </div>
    <div class="grid justify-center">
      <div
        class="radial-progress mx-3 mt-3 is-size-7 has-text-grey-lighter opacity-20"
        style="--value: 100; --thickness: 3px; --size: 100px; grid-area: 1/1"
      ></div>
      <div
        class="radial-progress mx-3 mt-3 is-size-7 {icon.color}"
        style="--value: {percent}; --thickness: 3px; --size: 100px; grid-area: 1/1"
        role="progressbar"
        aria-label="{transactionSequece.key} recurring payment"
        aria-valuemin="0"
        aria-valuemax="100"
        aria-valuenow={percent}
        aria-valuetext="due {n.fromNow()}"
      >
        <div class="is-size-6">
          <span class="icon">
            <i class="fas {icon.icon}"></i>
          </span>
        </div>
        <span>{formatCurrencyCrude(totalRecurring(transactionSequece))}</span>
        <span>due {n.fromNow()}</span>
      </div>
    </div>
  </div>
{/if}
