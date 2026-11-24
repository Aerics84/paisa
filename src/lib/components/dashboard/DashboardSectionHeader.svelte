<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    title?: string;
    subtle?: string | null;
    right?: Snippet;
  }

  let { title = "", subtle = null, right }: Props = $props();
</script>

<div class="paisa-zone__header">
  <div>
    <div class="paisa-zone__title">{title}</div>
  </div>

  {#if right}
    {@render right()}
  {:else if subtle}
    <div class="paisa-zone__subtle">
      {subtle}
    </div>
  {/if}
</div>
