<script lang="ts">
  export let title = "";
  export let subtle: string | null = null;
  export let selectable = false;
</script>

<div class="paisa-zone__header">
  <div>
    <div class="paisa-zone__title">{title}</div>
  </div>

  {#if $$slots.right}
    <slot name="right" />
  {:else if subtle}
    <div class:paisa-zone__subtle--selectable={selectable} class="paisa-zone__subtle">
      {subtle}
      {#if selectable}
        <i class="fas fa-angle-down"></i>
      {/if}
    </div>
  {/if}
</div>
