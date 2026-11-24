<script lang="ts">
  import type dayjs from "dayjs";
  import BoxedTabs from "./BoxedTabs.svelte";
  import { isMobile } from "$lib/utils";

  interface Props {
    value?: number;
    dateMin: dayjs.Dayjs;
    dateMax: dayjs.Dayjs;
  }

  let { value = $bindable(), dateMin, dateMax }: Props = $props();

  let options = $derived.by(() => {
    const result: { label: string; value: number }[] = [{ label: "All", value: -1 }];
    const diff = dateMax.diff(dateMin, "year");
    if (diff >= 10 && !isMobile()) {
      result.push({ label: "10 years", value: 10 });
    }

    if (diff >= 5 && !isMobile()) {
      result.push({ label: "5 years", value: 5 });
    }

    if (diff >= 3) {
      result.push({ label: "3 years", value: 3 });
    }

    if (diff >= 1) {
      result.push({ label: "1 year", value: 1 });
    }

    return result;
  });
</script>

{#if options.length > 1}
  <BoxedTabs bind:value {options} />
{/if}
