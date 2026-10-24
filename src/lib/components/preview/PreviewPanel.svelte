<script lang="ts">
  export let title = "";
  export let subtle = "";
  export let padded = true;
</script>

<section class="paisa-preview-panel" class:paisa-preview-panel--flush={!padded}>
  {#if title || subtle || $$slots.headerRight}
    <header class="paisa-preview-panel__header">
      <div>
        {#if title}
          <h2>{title}</h2>
        {/if}
        {#if subtle}
          <p>{subtle}</p>
        {/if}
      </div>

      {#if $$slots.headerRight}
        <slot name="headerRight" />
      {/if}
    </header>
  {/if}

  <div class="paisa-preview-panel__body">
    <slot />
  </div>
</section>
