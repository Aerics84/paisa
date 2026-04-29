<script lang="ts">
  export let label = "";
  export let value = "";
  export let meta = "";
  export let variant: "metric" | "rank" | "trend" = "metric";
  export let valueClass = "";
</script>

<div class={`paisa-stat-strip__item paisa-stat-strip__item--${variant}`}>
  <div class={`paisa-stat-strip__stack paisa-stat-strip__stack--${variant}`}>
    <span>{label}</span>

    {#if $$slots.default}
      <slot />
    {:else}
      <strong class={valueClass}>{value}</strong>
      {#if meta}
        <small>{meta}</small>
      {/if}
    {/if}
  </div>
</div>

<style lang="scss">
  .paisa-stat-strip__item {
    padding: 0 1rem;
    display: flex;
    flex-direction: column;
    justify-content: center;
    min-height: 4.9rem;
    align-items: stretch;
  }

  :global(.paisa-stat-strip > .paisa-stat-strip__item + .paisa-stat-strip__item) {
    border-left: 1px solid var(--app-border);
  }

  .paisa-stat-strip__item--metric,
  .paisa-stat-strip__item--rank {
    align-items: center;
  }

  .paisa-stat-strip__item span {
    color: var(--dashboard-text-soft);
    font-size: 0.85rem;
    font-weight: 600;
  }

  .paisa-stat-strip__item strong {
    color: var(--dashboard-text-strong);
    font-size: 1.28rem;
    font-weight: 600;
  }

  .paisa-stat-strip__stack {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: flex-start;
    gap: 0.18rem;
    min-height: 100%;
    flex: 1;
  }

  .paisa-stat-strip__stack--metric {
    justify-content: center;
    align-items: center;
    text-align: center;
  }

  .paisa-stat-strip__stack--rank {
    justify-content: center;
    align-items: center;
    text-align: center;
  }

  .paisa-stat-strip__stack--trend {
    justify-content: center;
    gap: 0.34rem;
  }

  .paisa-stat-strip__item--rank .paisa-stat-strip__stack strong {
    font-size: 1.52rem;
    line-height: 1.05;
    text-align: center;
  }

  .paisa-stat-strip__item--rank {
    justify-content: center;
  }

  .paisa-stat-strip__item small {
    color: var(--dashboard-text-subtle);
    font-size: 0.8rem;
  }
</style>
