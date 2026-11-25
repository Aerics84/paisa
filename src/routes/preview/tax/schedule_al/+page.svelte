<script lang="ts">
  import { renderBreakdowns } from "$lib/schedule_al";
  import _ from "lodash";
  import { getScheduleAL } from "$lib/api/tax";
  import type { ScheduleAL } from "$lib/utils";
  import { onMount } from "svelte";
  import { setAllowedDateRange, year } from "../../../../store";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import { t } from "$lib/i18n";

  let scheduleAls = $state<Record<string, ScheduleAL>>();
  let selectedScheduleAl = $derived(scheduleAls ? scheduleAls[$year] : undefined);
  let loaded = $state(false);

  $effect(() => {
    if (selectedScheduleAl) {
      renderBreakdowns(selectedScheduleAl.entries);
    }
  });

  onMount(async () => {
    ({ schedule_als: scheduleAls } = await getScheduleAL());
    loaded = true;

    setAllowedDateRange(_.map(Object.values(scheduleAls), (e) => e.date));
  });
</script>

<section class="section tab-schedule_al">
  <div class="container is-fluid">
    {#if selectedScheduleAl}
      <div class="columns">
        <div class="column is-12">
          <p class="subtitle is-12">
            {$t("tax.scheduleAlAsOn")}
            <span class="has-text-weight-bold">{selectedScheduleAl.date.format("DD MMM YYYY")}</span
            >
          </p>
        </div>
      </div>
    {/if}

    {#if loaded && !selectedScheduleAl}
      <div class="columns">
        <div class="column is-12">
          <div class="box" style="max-width: 1024px;">
            <ZeroState item={false}>
              <strong>{$t("tax.noScheduleAlData")}</strong>
              {$t("tax.noScheduleAlEntries")}
            </ZeroState>
          </div>
        </div>
      </div>
    {/if}

    <div class="columns" style:display={selectedScheduleAl ? null : "none"}>
      <div class="column is-12">
        <div class="box px-3" style="max-width: 1024px;">
          <table class="table is-narrow is-fullwidth is-hoverable">
            <thead>
              <tr>
                <th>{$t("tax.code")}</th>
                <th>{$t("tax.section")}</th>
                <th>{$t("tax.details")}</th>
                <th class="has-text-right">{$t("tax.amount")}</th>
              </tr>
            </thead>
            <tbody class="d3-schedule-al has-text-grey-dark"></tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</section>
