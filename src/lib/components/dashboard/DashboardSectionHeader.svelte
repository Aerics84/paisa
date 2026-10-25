<script lang="ts">
  export let title = "";
  export let subtle: string | null = null;
</script>

<div class="paisa-zone__header">
  <div>
    <div class="paisa-zone__title">{title}</div>
  </div>

  {#if $$slots.right}
    <slot name="right" />
  {:else if subtle}
    <div class="paisa-zone__subtle">
      {subtle}
    </div>
  {/if}
</div>
