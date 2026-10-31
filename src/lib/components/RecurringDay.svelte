<script lang="ts">
  import { isMobile, now, type TransactionSchedule } from "$lib/utils";
  import type { Dayjs } from "dayjs";
  import RecurringSchedule from "./RecurringSchedule.svelte";

  export let month: string;
  export let day: Dayjs;
  export let schedules: TransactionSchedule[];
  const isToday = day.isSame(now(), "day");
</script>

<div
  class="box m-0 p-0 recurring-day {day.format('YYYY-MM') != month &&
    'is-invisible is-hidden-mobile'}"
>
  <div class="has-text-centered has-text-weight-bold mt-1 mb-1">
    <span
      class="is-size-6 px-2 py-1 {isToday
        ? 'rounded-full is-bordered is-link has-text-link'
        : 'has-text-grey'}">{day.format(isMobile() ? "ddd D" : "D")}</span
    >
  </div>

  <div class="recurring-day__schedules">
    {#each schedules as schedule (schedule)}
      <RecurringSchedule {schedule} />
    {/each}
  </div>
</div>

<style>
  .recurring-day {
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
  }

  .recurring-day__schedules {
    min-height: 0;
    overflow-y: auto;
  }

  @media (max-width: 767px) {
    .recurring-day {
      overflow: visible;
    }

    .recurring-day__schedules {
      overflow-y: visible;
    }
  }
</style>
