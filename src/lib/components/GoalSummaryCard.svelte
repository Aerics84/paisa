<script lang="ts">
  import { iconGlyph } from "$lib/icon";
  import { t } from "$lib/i18n";
  import { formatCurrency, formatPercentage, type GoalSummary } from "$lib/utils";
  import { analyzeGoal, goalDelayLabel, goalStatusShortLabel } from "$lib/goals";
  import _ from "lodash";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import Progress from "$lib/components/Progress.svelte";
  import COLORS from "$lib/colors";
  import dayjs from "dayjs";
  import type { Action } from "svelte/action";

  interface Props {
    goal: GoalSummary;
    small?: boolean;
    action?: Action | null;
  }

  let { goal, small = false, action = null }: Props = $props();

  function formatDate(date: string) {
    const d = dayjs(date, "YYYY-MM-DD", true);
    if (d.isValid()) {
      return d.fromNow();
    }
    return "";
  }

  let analysis = $derived(analyzeGoal(goal));
  let completed = $derived(analysis.progressRatio * 100);
  let status = $derived(goalStatusShortLabel(goal, analysis));
  let required = $derived(
    analysis.state === "active" && (analysis.requiredMonthlyContribution ?? 0) > 0
      ? analysis.requiredMonthlyContribution
      : undefined
  );
  let projection = $derived(analysis.projectedCompletionDate);
  let delay = $derived(goalDelayLabel(analysis));
</script>

<div class="box p-3 goal-summary-card" class:mb-3={small}>
  <div class="flex justify-between mb-4">
    <div class="flex">
      {#if action}
        <span use:action class="icon is-size-4 mr-1 mt-1 has-text-grey-light">
          <i class="fas fa-grip-vertical"></i>
        </span>
      {/if}
      <a
        class="secondary-link has-text-grey"
        href="/more/goals/{goal.type}/{encodeURIComponent(goal.name)}"
      >
        <h4 class="is-size-4 has-text-grey">{goal.name}</h4>
      </a>
    </div>
    {#if !_.isEmpty(goal.icon)}
      <span class="{small ? 'is-size-3' : 'is-size-2'} custom-icon">{iconGlyph(goal.icon)}</span>
    {/if}
  </div>
  <nav class="level grid-2">
    <LevelItem
      {small}
      narrow
      title={$t("assets.current")}
      color={COLORS.gainText}
      value={formatCurrency(goal.current)}
    />

    <LevelItem
      {small}
      narrow
      title={$t("assets.target")}
      color={COLORS.primary}
      value={formatCurrency(goal.target)}
    />
  </nav>
  <Progress small showPercent={false} progressPercent={completed} />
  <div class="flex justify-between has-text-grey">
    <div>{formatPercentage(completed / 100, 2)}</div>
    <div>{formatDate(goal.targetDate)}</div>
  </div>
  <div class="is-size-7 has-text-grey mt-2">
    <div class="flex justify-between">
      <span>{status}</span>
      {#if required !== undefined}
        <span
          >{$t("components.goalSummary.monthlyNeeded", { amount: formatCurrency(required) })}</span
        >
      {/if}
    </div>
    {#if projection}
      <div class="flex justify-between">
        <span
          >{$t("components.goalSummary.projected", {
            date: projection.format("DD MMM YYYY")
          })}</span
        >
        {#if delay}
          <span>{delay}</span>
        {/if}
      </div>
    {/if}
  </div>
</div>
